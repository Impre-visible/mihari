package tools

import (
	"context"
	"fmt"
	"strings"

	"github.com/Impre-visible/mihari/internal/agent"
	"github.com/Impre-visible/mihari/internal/gitlab"
)

func formatNotes(notes []gitlab.Note) string {
	var b strings.Builder
	for _, n := range notes {
		where := ""
		if n.Path != "" {
			where = fmt.Sprintf(" sur %s:%d", n.Path, n.Line)
		}
		fmt.Fprintf(&b, "@%s%s, %s :\n%s\n\n", n.Author, where, n.CreatedAt.Format("02/01 15:04"), strings.TrimSpace(n.Body))
	}
	return b.String()
}

// diffPaths finds the old and new names of path in the merge request's diff,
// which a line comment needs.
func diffPaths(ctx context.Context, env *agent.Env, path string) (oldPath, newPath string, err error) {
	diffs, err := env.GitLab.Diffs(ctx, env.MR.IID)
	if err != nil {
		return "", "", err
	}
	oldPath, newPath = path, path
	for _, d := range diffs {
		if d.NewPath == path || d.OldPath == path {
			oldPath, newPath = d.OldPath, d.NewPath
		}
	}
	return oldPath, newPath, nil
}

func short(sha string) string {
	return sha[:min(8, len(sha))]
}
