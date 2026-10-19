package tools

import (
	"context"

	"github.com/Impre-visible/mihari/internal/agent"
)

type rememberArgs struct {
	Note string `json:"note" desc:"Ce qu'un prochain job sur ce projet doit savoir, en une phrase"`
}

func init() {
	agent.Register(agent.Define("remember", "Garde une note sur ce projet pour tes prochains jobs : convention, commande de test, piège, préférence de l'équipe. Pas de note sur la tâche en cours.",
		func(_ context.Context, env *agent.Env, a rememberArgs) (string, error) {
			if err := env.Remember(a.Note); err != nil {
				return "", err
			}
			return "Noté pour les prochains jobs.", nil
		}, agent.When(func(env *agent.Env) bool { return env.Remember != nil })))
}
