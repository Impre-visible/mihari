// Package sandbox runs an agent's commands in a throwaway container, with the
// job's repository mounted at /workspace and no GitLab credentials inside.
package sandbox

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"
)

// Workdir is where the repository appears inside the container.
const Workdir = "/workspace"

// How much of a command's output an agent gets back, per stream.
const outputLimit = 64 << 10

type Kind string

const (
	Docker Kind = "docker"
	// Apple's container CLI, which runs each container in its own lightweight VM.
	Apple Kind = "apple"
)

// Info describes the container runtime of this machine for the Status page.
type Info struct {
	Kind      Kind   `json:"kind"`
	Label     string `json:"label"`
	Installed bool   `json:"installed"`
	// Installed and answering: the Docker daemon runs and is reachable.
	Ready   bool   `json:"ready"`
	Error   string `json:"error"`
	Install string `json:"install"`
}

type runtimeCLI struct {
	kind  Kind
	label string
	path  string
}

// detect picks Apple's runtime on a Mac that has it, and Docker otherwise.
func detect() (runtimeCLI, Info) {
	if runtime.GOOS == "darwin" {
		if path := lookPath("container"); path != "" {
			return runtimeCLI{Apple, "Apple container", path}, Info{Kind: Apple, Label: "Apple container", Installed: true}
		}
	}
	install := "Installe Docker Engine (docs.docker.com/engine/install)."
	switch runtime.GOOS {
	case "darwin":
		install = "Installe container d'Apple (github.com/apple/container/releases), ou Docker Desktop / OrbStack."
	case "windows":
		install = "Installe Docker Desktop (docs.docker.com/desktop)."
	}
	path := lookPath("docker")
	return runtimeCLI{Docker, "Docker", path}, Info{Kind: Docker, Label: "Docker", Installed: path != "", Install: install}
}

func lookPath(name string) string {
	if path, err := exec.LookPath(name); err == nil {
		return path
	}
	// A GUI app's PATH often lacks these.
	for _, dir := range []string{"/usr/local/bin", "/opt/homebrew/bin", "/opt/orbstack/bin"} {
		path := filepath.Join(dir, name)
		if _, err := os.Stat(path); err == nil {
			return path
		}
	}
	return ""
}

// Detect reports which runtime jobs would use and whether it is ready.
func Detect(ctx context.Context) Info {
	cli, info := detect()
	if !info.Installed {
		info.Error = info.Label + " n'est pas installé."
		return info
	}
	if err := cli.ready(ctx); err != nil {
		info.Error = err.Error()
		return info
	}
	info.Ready = true
	return info
}

func (r runtimeCLI) run(ctx context.Context, stdin []byte, args ...string) (string, string, error) {
	cmd := exec.CommandContext(ctx, r.path, args...)
	if stdin != nil {
		cmd.Stdin = bytes.NewReader(stdin)
	}
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	err := cmd.Run()
	return stdout.String(), stderr.String(), err
}

func (r runtimeCLI) ready(ctx context.Context) error {
	switch r.kind {
	case Apple:
		if _, _, err := r.run(ctx, nil, "system", "status"); err != nil {
			// The services start on demand; the default kernel installs without a prompt.
			if _, stderr, err := r.run(ctx, nil, "system", "start", "--enable-kernel-install"); err != nil {
				return fmt.Errorf("container system start : %s", strings.TrimSpace(stderr))
			}
		}
		return nil
	default:
		_, stderr, err := r.run(ctx, nil, "info", "--format", "{{.ServerVersion}}")
		if err == nil {
			return nil
		}
		switch {
		case strings.Contains(stderr, "permission denied"):
			return errors.New("Docker refuse l'accès : ajoute ton utilisateur au groupe docker (sudo usermod -aG docker $USER), puis reconnecte-toi")
		case strings.Contains(stderr, "Cannot connect") || strings.Contains(stderr, "Is the docker daemon running"):
			return errors.New("le démon Docker ne tourne pas : lance Docker (sudo systemctl start docker, ou ouvre Docker Desktop)")
		}
		return fmt.Errorf("docker info : %s", strings.TrimSpace(stderr))
	}
}

// Spec describes the container a job gets.
type Spec struct {
	Image string
	// Host directory mounted read-write at Workdir.
	Workspace string
	Env       []string
}

// Container is a running sandbox; Close must always be called.
type Container struct {
	cli  runtimeCLI
	name string
}

// Start pulls the image if needed and starts an idle container to exec into.
func Start(ctx context.Context, spec Spec) (*Container, error) {
	cli, info := detect()
	if !info.Installed {
		return nil, fmt.Errorf("%s n'est pas installé. %s", info.Label, info.Install)
	}
	if err := cli.ready(ctx); err != nil {
		return nil, err
	}
	workspace, err := filepath.Abs(spec.Workspace)
	if err != nil {
		return nil, err
	}

	name := "mihari-" + strconv.FormatInt(time.Now().UnixNano(), 36)
	args := []string{"run", "--detach", "--rm", "--name", name,
		"--volume", workspace + ":" + Workdir, "--workdir", Workdir}
	if cli.kind == Docker {
		// --init reaps the zombies agents' commands leave; no-new-privileges blocks setuid escalation.
		args = append(args, "--init", "--security-opt", "no-new-privileges", "--pids-limit", "4096")
	}
	for _, env := range spec.Env {
		args = append(args, "--env", env)
	}
	// The container idles until Close; every command is an exec.
	args = append(args, spec.Image, "sleep", "infinity")

	startCtx, cancel := context.WithTimeout(ctx, 10*time.Minute)
	defer cancel()
	if _, stderr, err := cli.run(startCtx, nil, args...); err != nil {
		msg := strings.TrimSpace(stderr)
		if strings.Contains(msg, "sleep") && strings.Contains(msg, "not found") {
			return nil, fmt.Errorf("l'image %s n'a pas de shell utilisable (sleep introuvable)", spec.Image)
		}
		return nil, fmt.Errorf("démarrage du container (%s) : %s", spec.Image, msg)
	}
	return &Container{cli: cli, name: name}, nil
}

// Result is what a command printed and how it ended.
type Result struct {
	Stdout   string `json:"stdout"`
	Stderr   string `json:"stderr"`
	ExitCode int    `json:"exitCode"`
	TimedOut bool   `json:"timedOut"`
	// Output beyond the limit was cut from the middle.
	Truncated bool  `json:"truncated"`
	Duration  int64 `json:"durationMs"`
}

// Exec runs command with sh -c in Workdir. timeout is enforced inside the
// container too, so a killed client never leaves the command running.
func (c *Container) Exec(ctx context.Context, command string, stdin []byte, timeout time.Duration) (Result, error) {
	seconds := strconv.Itoa(max(1, int(timeout.Seconds())))
	args := []string{"exec"}
	if stdin != nil {
		args = append(args, "--interactive")
	}
	args = append(args, "--workdir", Workdir, c.name, "timeout", "-s", "KILL", seconds, "sh", "-c", command)

	// A little longer than the inner timeout, so that one reports first.
	ctx, cancel := context.WithTimeout(ctx, timeout+15*time.Second)
	defer cancel()
	begin := time.Now()
	stdout, stderr, err := c.cli.run(ctx, stdin, args...)

	res := Result{Duration: time.Since(begin).Milliseconds()}
	res.Stdout, res.Truncated = clip(stdout)
	var cut bool
	res.Stderr, cut = clip(stderr)
	res.Truncated = res.Truncated || cut

	var exitErr *exec.ExitError
	switch {
	case err == nil:
	case errors.As(err, &exitErr):
		res.ExitCode = exitErr.ExitCode()
		// timeout -s KILL exits 137 when it had to kill the command.
		res.TimedOut = res.ExitCode == 137 && time.Since(begin) >= timeout
	case ctx.Err() != nil:
		res.ExitCode, res.TimedOut = -1, true
	default:
		return res, err
	}
	return res, nil
}

// clip keeps the start and the end of long output, where errors usually are.
func clip(s string) (string, bool) {
	if len(s) <= outputLimit {
		return s, false
	}
	half := outputLimit / 2
	return s[:half] + "\n… [" + strconv.Itoa(len(s)-outputLimit) + " octets coupés] …\n" + s[len(s)-half:], true
}

// Chown hands the workspace back to the user. Docker on Linux writes the
// mounted files as root, which Mihari must commit, push and delete.
func (c *Container) Chown(ctx context.Context) {
	if c.cli.kind == Docker && runtime.GOOS == "linux" {
		owner := strconv.Itoa(os.Getuid()) + ":" + strconv.Itoa(os.Getgid())
		_, _, _ = c.cli.run(ctx, nil, "exec", c.name, "chown", "-R", owner, Workdir)
	}
}

// Close stops and removes the container.
func (c *Container) Close() error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	c.Chown(ctx)
	args := []string{"rm", "--force", c.name}
	if c.cli.kind == Apple {
		args = []string{"delete", "--force", c.name}
	}
	if _, stderr, err := c.cli.run(ctx, nil, args...); err != nil {
		return fmt.Errorf("suppression du container %s : %s", c.name, strings.TrimSpace(stderr))
	}
	return nil
}
