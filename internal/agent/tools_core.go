package agent

import (
	"context"
	"strings"
)

type finishArgs struct {
	Success bool   `json:"success" desc:"true si la demande est traitée, false si tu n'as pas pu"`
	Summary string `json:"summary" desc:"Ce que tu as fait, ou pourquoi tu n'as pas pu, en 1 à 3 phrases"`
}

type rememberArgs struct {
	Note string `json:"note" desc:"Ce qu'un prochain job sur ce projet doit savoir, en une phrase"`
}

func init() {
	Register(Define("remember", "Garde une note sur ce projet pour tes prochains jobs : convention, commande de test, piège, préférence de l'équipe. Pas de note sur la tâche en cours.",
		func(_ context.Context, env *Env, a rememberArgs) (string, error) {
			if err := env.Remember(a.Note); err != nil {
				return "", err
			}
			return "Noté pour les prochains jobs.", nil
		}, When(func(env *Env) bool { return env.Remember != nil })))

	Register(Define("finish", "Termine le job. Appelle-le une seule fois, quand tout est fait (ou impossible).",
		func(_ context.Context, env *Env, a finishArgs) (string, error) {
			env.Finished = &Finish{Success: a.Success, Summary: strings.TrimSpace(a.Summary)}
			return "Job terminé.", nil
		}))
}
