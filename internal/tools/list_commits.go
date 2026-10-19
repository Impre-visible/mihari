package tools

import (
	"context"
	"fmt"
	"strings"

	"github.com/Impre-visible/mihari/internal/agent"
)

func init() {
	agent.Register(agent.Define("list_commits", "Liste les commits de la MR, du plus ancien au plus récent.",
		func(ctx context.Context, env *agent.Env, _ agent.NoArgs) (string, error) {
			commits, err := env.GitLab.Commits(ctx, env.MR.IID)
			if err != nil {
				return "", err
			}
			var b strings.Builder
			for _, c := range commits {
				fmt.Fprintf(&b, "%s %s (%s, %s)\n", short(c.SHA), c.Title, c.Author, c.Created.Format("02/01 15:04"))
			}
			if b.Len() == 0 {
				return "Aucun commit.", nil
			}
			return b.String(), nil
		}, agent.When(hasMR)))
}
