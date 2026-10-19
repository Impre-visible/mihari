package tools

import (
	"context"
	"time"

	"github.com/Impre-visible/mihari/internal/agent"
)

func init() {
	agent.Register(agent.Define("git_status", "Branche courante et fichiers modifiés, ajoutés ou non suivis du repo : ce que push enverrait.",
		func(ctx context.Context, env *agent.Env, _ agent.NoArgs) (string, error) {
			res, err := env.Runner.Exec(ctx, needs("git")+git+" status --short --branch", nil, 30*time.Second)
			if err != nil {
				return "", err
			}
			return format(res), nil
		}))
}
