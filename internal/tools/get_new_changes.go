package tools

import (
	"context"
	"fmt"
	"strings"

	"github.com/Impre-visible/mihari/internal/agent"
)

func init() {
	agent.Register(agent.Define("get_new_changes", "Ce qui a changé dans la MR depuis ta dernière review : les nouveaux commits et leur diff. Pour une review incrémentale, c'est tout ce que tu relis.",
		func(ctx context.Context, env *agent.Env, _ agent.NoArgs) (string, error) {
			commits, diffs, err := env.GitLab.Compare(ctx, env.ReviewedSHA, env.MR.HeadSHA)
			if err != nil {
				return "", err
			}
			if len(commits) == 0 {
				return "Aucun nouveau commit depuis ta dernière review.", nil
			}
			var b strings.Builder
			b.WriteString("Nouveaux commits :\n")
			for _, c := range commits {
				fmt.Fprintf(&b, "- %s %s (%s)\n", short(c.SHA), c.Title, c.Author)
			}
			for _, d := range diffs {
				fmt.Fprintf(&b, "\n=== %s\n%s\n", d.NewPath, d.Diff)
			}
			return b.String(), nil
		}, agent.When(func(env *agent.Env) bool { return reviewing(env) && env.ReviewedSHA != "" })))
}
