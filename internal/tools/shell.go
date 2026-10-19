package tools

import (
	"context"
	"errors"
	"fmt"
	"path"
	"strings"
	"time"

	"github.com/Impre-visible/mihari/internal/agent"
	"github.com/Impre-visible/mihari/internal/sandbox"
)

// quote makes s a single shell word.
func quote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}

// inWorkspace resolves p against /workspace; the container is the boundary,
// so absolute paths elsewhere are allowed.
func inWorkspace(p string) string {
	if p == "" {
		return sandbox.Workdir
	}
	if path.IsAbs(p) {
		return path.Clean(p)
	}
	return path.Join(sandbox.Workdir, p)
}

// format shows a command's output the way the model and the chat read it,
// ending with its exit code.
func format(res sandbox.Result) string {
	var b strings.Builder
	if res.Stdout != "" {
		b.WriteString(res.Stdout)
	}
	if res.Stderr != "" {
		if b.Len() > 0 && !strings.HasSuffix(b.String(), "\n") {
			b.WriteString("\n")
		}
		b.WriteString("[stderr]\n" + res.Stderr)
	}
	if b.Len() == 0 {
		b.WriteString("(aucune sortie)")
	}
	fmt.Fprintf(&b, "\n[code de sortie %d", res.ExitCode)
	if res.TimedOut {
		b.WriteString(", délai dépassé")
	}
	if res.Truncated {
		b.WriteString(", sortie tronquée")
	}
	b.WriteString("]")
	return b.String()
}

// output runs command and returns its stdout, or its stderr as the error
// when it fails.
func output(ctx context.Context, env *agent.Env, command string, stdin []byte, timeout time.Duration) (string, error) {
	res, err := env.Runner.Exec(ctx, command, stdin, timeout)
	if err != nil {
		return "", err
	}
	if res.ExitCode != 0 {
		msg := strings.TrimSpace(res.Stderr)
		if msg == "" {
			msg = strings.TrimSpace(res.Stdout)
		}
		if msg == "" {
			msg = fmt.Sprintf("code de sortie %d", res.ExitCode)
		}
		return "", errors.New(msg)
	}
	return res.Stdout, nil
}

// Command output is cut past 64 KiB; readFile reads a file in pieces below that.
const chunk = 48 << 10

// readFile returns up to limit bytes of a file in the container, whole and
// byte for byte.
func readFile(ctx context.Context, env *agent.Env, file string, limit int) (string, error) {
	var b strings.Builder
	for b.Len() < limit {
		cmd := fmt.Sprintf("test -f %[1]s || { echo 'fichier introuvable : %[2]s' >&2; exit 1; }; tail -c +%[3]d %[1]s | head -c %[4]d",
			quote(file), strings.ReplaceAll(file, "'", ""), b.Len()+1, min(chunk, limit-b.Len()))
		res, err := env.Runner.Exec(ctx, cmd, nil, 30*time.Second)
		if err != nil {
			return "", err
		}
		if res.ExitCode != 0 {
			return "", errors.New(strings.TrimSpace(res.Stderr))
		}
		b.WriteString(res.Stdout)
		if len(res.Stdout) < chunk {
			break
		}
	}
	return b.String(), nil
}

// git runs git in the workspace, which the container sees owned by someone else.
const git = "git -c safe.directory='*' -c core.pager=cat -c color.ui=never"

// needs makes a command fail with a hint when program is missing from the
// job's image, which is often the project's CI image.
func needs(program string) string {
	return fmt.Sprintf("command -v %[1]s >/dev/null 2>&1 || { echo '%[1]s absent de l'\\''image : installe-le avec run_command (apt-get install -y %[1]s, apk add %[1]s…) puis réessaie.' >&2; exit 127; }; ", program)
}
