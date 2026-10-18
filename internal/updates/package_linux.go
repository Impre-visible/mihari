package updates

import (
	"bytes"
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"syscall"

	"github.com/wailsapp/wails/v3/pkg/application"
	"github.com/wailsapp/wails/v3/pkg/updater"
)

// packageMethod tells whether pacman or dpkg owns the running binary, and
// whether pkexec can raise the rights to replace it.
func packageMethod() Method {
	exe, err := os.Executable()
	if err != nil {
		return MethodManual
	}
	if _, err := exec.LookPath("pkexec"); err != nil {
		return MethodManual
	}
	if _, err := exec.LookPath("pacman"); err == nil && exec.Command("pacman", "-Qo", exe).Run() == nil {
		return MethodPacman
	}
	if _, err := exec.LookPath("dpkg"); err == nil && exec.Command("dpkg", "-S", exe).Run() == nil {
		return MethodApt
	}
	return MethodManual
}

// installPackage downloads the release's package, checks it against the
// published SHA-256, installs it through pkexec and restarts on it.
func (s *Service) installPackage(ctx context.Context, rel *updater.Release, m Method) error {
	if rel.Verification == nil || len(rel.Verification.Digest) == 0 {
		// Root installs it: an unverified file is never handed over.
		return errors.New("la release ne publie pas de somme de contrôle pour ce paquet")
	}
	suffix := ".pkg.tar.zst"
	if m == MethodApt {
		suffix = ".deb"
	}
	file, err := os.CreateTemp("", "mihari-update-*"+suffix)
	if err != nil {
		return err
	}
	defer os.Remove(file.Name())

	hash := sha256.New()
	err = s.provider.Download(ctx, rel, io.MultiWriter(file, hash), func(written, total int64) {
		s.mu.Lock()
		s.info.Downloaded, s.info.Total = written, total
		s.mu.Unlock()
		s.publish()
	})
	if cerr := file.Close(); err == nil {
		err = cerr
	}
	if err != nil {
		return err
	}
	if !bytes.Equal(hash.Sum(nil), rel.Verification.Digest) {
		return errors.New("le paquet téléchargé ne correspond pas à sa somme de contrôle")
	}
	// The package manager runs as root and must be able to read it.
	if err := os.Chmod(file.Name(), 0o644); err != nil {
		return err
	}

	args := []string{"pacman", "-U", "--noconfirm", file.Name()}
	if m == MethodApt {
		args = []string{"apt-get", "install", "-y", file.Name()}
	}
	out, err := exec.CommandContext(ctx, "pkexec", args...).CombinedOutput()
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) && (exitErr.ExitCode() == 126 || exitErr.ExitCode() == 127) {
		return errors.New("installation annulée : le mot de passe n'a pas été donné")
	}
	if err != nil {
		return fmt.Errorf("%s : %s", args[0], strings.TrimSpace(string(out)))
	}

	// The package replaced the binary at the same path; start it once this one is gone.
	exe, err := os.Executable()
	if err != nil {
		return err
	}
	if err := relaunchAfterExit(strings.TrimSuffix(exe, " (deleted)")); err != nil {
		return err
	}
	application.Get().Quit()
	return nil
}

// relaunchAfterExit starts path once this process has exited, so that the
// two versions never run side by side.
func relaunchAfterExit(path string) error {
	return relaunchWhenGone(os.Getpid(), path)
}

func relaunchWhenGone(pid int, path string) error {
	script := `while kill -0 "$1" 2>/dev/null; do sleep 0.2; done; exec "$2"`
	args := []string{"sh", "-c", script, "sh", strconv.Itoa(pid), path}
	// Desktop launchers run Mihari in a systemd unit, which kills every
	// process of its cgroup once Mihari exits, a new session or not. A unit
	// of its own survives that.
	if _, err := exec.LookPath("systemd-run"); err == nil && os.Getenv("XDG_RUNTIME_DIR") != "" {
		args = append([]string{"systemd-run", "--user", "--quiet", "--collect", "--unit", "mihari-relaunch-" + strconv.Itoa(pid), "--"}, args...)
	}
	cmd := exec.Command(args[0], args[1:]...)
	// Its own session too, for systems without systemd.
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	cmd.Stdin, cmd.Stdout, cmd.Stderr = nil, io.Discard, io.Discard
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("relance de Mihari : %w", err)
	}
	// Not waited for: it must outlive us.
	return cmd.Process.Release()
}
