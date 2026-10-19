package tools

import (
	"context"
	"fmt"
	"strings"

	"github.com/Impre-visible/mihari/internal/agent"
)

func init() {
	agent.Register(agent.Define("read_comments", "Relit tous les commentaires, y compris les anciens échanges avec toi.",
		func(ctx context.Context, env *agent.Env, _ agent.NoArgs) (string, error) {
			discussions, err := env.GitLab.Discussions(ctx, env.Target)
			if err != nil {
				return "", err
			}
			var b strings.Builder
			for _, d := range discussions {
				state := ""
				if d.Open {
					state = ", ouvert"
				}
				fmt.Fprintf(&b, "[fil %s%s]\n%s\n", d.ID, state, formatNotes(d.Notes))
			}
			if b.Len() == 0 {
				return "Aucun commentaire.", nil
			}
			return b.String(), nil
		}, agent.When(hasTarget)))
}
