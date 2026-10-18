package gitlab

import (
	"context"
	"encoding/base64"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

// Clone checks out ref of the project at path into dir. The token goes to git
// through its environment only, so it is written neither in .git/config nor
// on the command line, and the clone holds no credentials once done.
func Clone(ctx context.Context, path, ref, dir string) error {
	base, token, _, err := connection()
	if err != nil {
		return err
	}
	remote := base.JoinPath(path + ".git").String()
	auth := base64.StdEncoding.EncodeToString([]byte("oauth2:" + token))

	args := []string{"clone", "--quiet", "--depth", "50"}
	if ref != "" {
		args = append(args, "--branch", ref)
	}
	cmd := exec.CommandContext(ctx, "git", append(args, remote, dir)...)
	cmd.Env = append(os.Environ(),
		"GIT_TERMINAL_PROMPT=0",
		"GIT_CONFIG_COUNT=1",
		"GIT_CONFIG_KEY_0=http."+base.String()+"/.extraHeader",
		"GIT_CONFIG_VALUE_0=Authorization: Basic "+auth,
	)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("git clone %s (%s) : %s", path, ref, strings.TrimSpace(string(out)))
	}
	return nil
}
