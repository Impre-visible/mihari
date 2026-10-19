package sandbox

import (
	"archive/tar"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"strings"
)

// A Docker daemon on another machine (ssh://, tcp://) cannot mount a folder
// of this one: it mounts an empty folder of its own in its place. There, the
// repository is copied into the container instead, and the agent's work is
// copied back before Mihari commits it.

// remoteDocker reports whether Docker runs elsewhere, from DOCKER_HOST or the
// current docker context.
func (r runtimeCLI) remoteDocker(ctx context.Context) bool {
	if r.kind != Docker {
		return false
	}
	host := os.Getenv("DOCKER_HOST")
	if host == "" {
		out, _, err := r.run(ctx, nil, "context", "inspect", "--format", "{{.Endpoints.docker.Host}}")
		if err != nil {
			return false
		}
		host = strings.TrimSpace(out)
	}
	return isRemote(host)
}

func isRemote(host string) bool {
	return host != "" && !strings.HasPrefix(host, "unix://") && !strings.HasPrefix(host, "npipe://")
}

// stream runs the runtime's CLI and hands its output to consume as it comes.
func (r runtimeCLI) stream(ctx context.Context, consume func(io.Reader) error, args ...string) error {
	cmd := exec.CommandContext(ctx, r.path, args...)
	out, err := cmd.StdoutPipe()
	if err != nil {
		return err
	}
	var stderr strings.Builder
	cmd.Stderr = &stderr
	if err := cmd.Start(); err != nil {
		return err
	}
	consumed := consume(out)
	// Whatever consume left unread must not block the command.
	_, _ = io.Copy(io.Discard, out)
	if err := cmd.Wait(); err != nil {
		if msg := strings.TrimSpace(stderr.String()); msg != "" {
			return fmt.Errorf("%w : %s", err, msg)
		}
		return err
	}
	return consumed
}

// copyIn sends the host workspace into the container.
func (c *Container) copyIn(ctx context.Context) error {
	if _, stderr, err := c.cli.run(ctx, nil, "cp", c.workspace+string(filepath.Separator)+".", c.name+":"+Workdir); err != nil {
		return fmt.Errorf("copie du repo dans le container : %s", strings.TrimSpace(stderr))
	}
	return nil
}

// Files git would see, as a tar stream; exit 99 when git or a tar that reads
// a list of names is missing.
const listedTar = `command -v git >/dev/null 2>&1 && tar --version 2>/dev/null | grep -qiE 'gnu|bsdtar' || exit 99
git -c safe.directory='*' ls-files -z --cached --others --exclude-standard | tar -cf - --null --no-recursion -T -`

// pull replaces the host workspace's files, .git aside, with the
// container's. With git and tar in the image, only what git would see
// travels, not node_modules and the like; otherwise everything does.
func (c *Container) pull(ctx context.Context) error {
	staging, err := os.MkdirTemp(filepath.Dir(c.workspace), "pull-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(staging)

	err = c.cli.stream(ctx, func(r io.Reader) error { return extract(r, staging, "") },
		"exec", "--workdir", Workdir, c.name, "sh", "-c", listedTar)
	if err != nil {
		// Start over with the whole folder; docker cp names entries after it.
		if err := os.RemoveAll(staging); err != nil {
			return err
		}
		if err := os.Mkdir(staging, 0o700); err != nil {
			return err
		}
		err = c.cli.stream(ctx, func(r io.Reader) error { return extract(r, staging, path.Base(Workdir)) },
			"cp", c.name+":"+Workdir, "-")
		if err != nil {
			return fmt.Errorf("copie du travail depuis le container : %w", err)
		}
	}

	entries, err := os.ReadDir(c.workspace)
	if err != nil {
		return err
	}
	for _, e := range entries {
		if e.Name() != ".git" {
			if err := os.RemoveAll(filepath.Join(c.workspace, e.Name())); err != nil {
				return err
			}
		}
	}
	entries, err = os.ReadDir(staging)
	if err != nil {
		return err
	}
	for _, e := range entries {
		if err := os.Rename(filepath.Join(staging, e.Name()), filepath.Join(c.workspace, e.Name())); err != nil {
			return err
		}
	}
	return nil
}

// pushGit hands the host's .git, which now holds Mihari's commit, back to
// the container, so that git there agrees with what was pushed.
func (c *Container) pushGit(ctx context.Context) error {
	if _, stderr, err := c.cli.run(ctx, nil, "exec", c.name, "rm", "-rf", Workdir+"/.git"); err != nil {
		return fmt.Errorf("mise à jour de .git dans le container : %s", strings.TrimSpace(stderr))
	}
	if _, stderr, err := c.cli.run(ctx, nil, "cp", filepath.Join(c.workspace, ".git"), c.name+":"+Workdir+"/.git"); err != nil {
		return fmt.Errorf("mise à jour de .git dans le container : %s", strings.TrimSpace(stderr))
	}
	return nil
}

// extract writes a tar stream into dir, leaving .git out and refusing any
// path that would land outside dir: the container is not trusted.
func extract(r io.Reader, dir, strip string) error {
	root, err := os.OpenRoot(dir)
	if err != nil {
		return err
	}
	defer root.Close()
	tr := tar.NewReader(r)
	for {
		h, err := tr.Next()
		if errors.Is(err, io.EOF) {
			return nil
		}
		if err != nil {
			return err
		}
		name := strings.TrimPrefix(path.Clean(h.Name), "./")
		if strip != "" {
			if name == strip {
				continue
			}
			name = strings.TrimPrefix(name, strip+"/")
		}
		if name == "." || name == ".git" || strings.HasPrefix(name, ".git/") {
			continue
		}
		if !filepath.IsLocal(name) {
			return fmt.Errorf("chemin refusé dans l'archive : %q", h.Name)
		}
		mode := h.FileInfo().Mode().Perm() | 0o600
		switch h.Typeflag {
		case tar.TypeDir:
			err = root.MkdirAll(name, mode|0o700)
		case tar.TypeReg:
			if err = root.MkdirAll(path.Dir(name), 0o755); err == nil {
				err = writeEntry(root, name, mode, tr)
			}
		case tar.TypeSymlink:
			if err = root.MkdirAll(path.Dir(name), 0o755); err == nil {
				err = root.Symlink(h.Linkname, name)
			}
		}
		if err != nil {
			return err
		}
	}
}

func writeEntry(root *os.Root, name string, mode os.FileMode, r io.Reader) error {
	f, err := root.OpenFile(name, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, mode)
	if err != nil {
		return err
	}
	if _, err := io.Copy(f, r); err != nil {
		f.Close()
		return err
	}
	return f.Close()
}
