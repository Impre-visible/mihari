package tools

import (
	"context"

	"github.com/Impre-visible/mihari/internal/agent"
)

type sessionSearchArgs struct {
	Query string `json:"query,omitempty" desc:"Mots qui doivent tous apparaître dans le job (demande, échanges, résultat) ; vide pour les derniers jobs"`
	JobID int    `json:"job_id,omitempty" desc:"Relit ce job en entier au lieu de chercher"`
}

func init() {
	agent.Register(agent.Define("session_search", "Cherche dans tes jobs passés sur ce projet : ce qui a déjà été fait ou demandé, les erreurs rencontrées, les réponses données. Ce n'est que l'historique : vérifie dans le code avant de conclure.",
		func(_ context.Context, env *agent.Env, a sessionSearchArgs) (string, error) {
			return env.PastJobs(a.Query, a.JobID)
		}, agent.When(func(env *agent.Env) bool { return env.PastJobs != nil })))
}
