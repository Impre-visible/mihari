package tools

import (
	"context"
	"errors"
	"strings"

	"github.com/Impre-visible/mihari/internal/agent"
)

type subagentArgs struct {
	Task string `json:"task" desc:"Ce que le sous-agent doit trouver ou vérifier, avec tout le contexte utile : il ne voit pas ta conversation"`
}

func init() {
	agent.Register(agent.Define("subagent", "Confie une recherche à un sous-agent au contexte neuf : explorer une partie du code, comprendre un mécanisme, vérifier une hypothèse. Il lit, cherche et lance des commandes sans rien modifier, puis ne te rend que sa conclusion. Garde ton propre contexte léger.",
		func(ctx context.Context, env *agent.Env, a subagentArgs) (string, error) {
			if strings.TrimSpace(a.Task) == "" {
				return "", errors.New("tâche vide")
			}
			return env.Subagent(ctx, a.Task)
		}, agent.When(func(env *agent.Env) bool { return env.Subagent != nil })))
}
