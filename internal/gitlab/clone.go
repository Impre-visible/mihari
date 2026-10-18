package gitlab

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"strconv"
	"strings"
)

// gitAuth gives git the token through its environment only, so it is
// written neither in .git/config nor on the command line.
func gitAuth(base *url.URL, token string) []string {
	auth := base64.StdEncoding.EncodeToString([]byte("oauth2:" + token))
	return append(os.Environ(),
		"GIT_TERMINAL_PROMPT=0",
		"GIT_CONFIG_COUNT=1",
		"GIT_CONFIG_KEY_0=http."+base.String()+"/.extraHeader",
		"GIT_CONFIG_VALUE_0=Authorization: Basic "+auth,
	)
}

func git(ctx context.Context, dir string, env []string, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Dir, cmd.Env = dir, env
	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("git %s : %s", args[0], strings.TrimSpace(string(out)))
	}
	return strings.TrimSpace(string(out)), nil
}

// Clone checks out ref of the project at path into dir; the clone holds no
// credentials once done.
func Clone(ctx context.Context, path, ref, dir string) error {
	base, token, _, err := connection()
	if err != nil {
		return err
	}
	args := []string{"clone", "--quiet", "--depth", "50"}
	if ref != "" {
		args = append(args, "--branch", ref)
	}
	_, err = git(ctx, "", gitAuth(base, token), append(args, base.JoinPath(path+".git").String(), dir)...)
	if err != nil {
		return fmt.Errorf("clone de %s (%s) : %w", path, ref, err)
	}
	return nil
}

// CheckoutMergeRequest checks out the head of a merge request in a clone of its
// target project, which works for a fork's branch the bot cannot read directly.
func CheckoutMergeRequest(ctx context.Context, dir string, iid int) error {
	base, token, _, err := connection()
	if err != nil {
		return err
	}
	ref := "refs/merge-requests/" + strconv.Itoa(iid) + "/head"
	if _, err := git(ctx, dir, gitAuth(base, token), "fetch", "--quiet", "--depth", "50", "origin", ref+":mr"); err != nil {
		return err
	}
	_, err = git(ctx, dir, nil, "checkout", "--quiet", "mr")
	return err
}

// Push commits every change in dir as author and pushes it to branch. It never
// forces: a branch that moved meanwhile makes it fail.
func Push(ctx context.Context, dir, branch, message string, author Identity) (string, error) {
	base, token, _, err := connection()
	if err != nil {
		return "", err
	}
	if _, err := git(ctx, dir, nil, "add", "--all"); err != nil {
		return "", err
	}
	status, err := git(ctx, dir, nil, "status", "--porcelain")
	if err != nil {
		return "", err
	}
	if status == "" {
		return "", errors.New("aucun changement à pousser")
	}
	if _, err := git(ctx, dir, nil, "-c", "user.name="+author.Name, "-c", "user.email="+author.Email,
		"commit", "--quiet", "--no-verify", "-m", message); err != nil {
		return "", err
	}
	if _, err := git(ctx, dir, gitAuth(base, token), "push", "--quiet", "origin", "HEAD:refs/heads/"+branch); err != nil {
		// Leave the commit out of the next push attempt's way.
		_, _ = git(ctx, dir, nil, "reset", "--quiet", "--soft", "HEAD~1")
		return "", err
	}
	return git(ctx, dir, nil, "rev-parse", "--short", "HEAD")
}
