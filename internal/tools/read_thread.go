package tools

import (
	"context"

	"github.com/Impre-visible/mihari/internal/agent"
)

func init() {
	agent.Register(agent.Define("read_thread", "Relit le fil de commentaires qui a lancé le job.",
		func(ctx context.Context, env *agent.Env, _ agent.NoArgs) (string, error) {
			if env.ThreadID == "" {
				return "Ce job n'a pas de fil de commentaires : il vient d'une demande de review ou d'un label.", nil
			}
			d, err := env.GitLab.Discussion(ctx, env.Target, env.ThreadID)
			if err != nil {
				return "", err
			}
			return formatNotes(d.Notes), nil
		}, agent.When(hasTarget)))
}
