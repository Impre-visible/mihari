package tools

import (
	"context"
	"errors"
	"strings"

	"github.com/Impre-visible/mihari/internal/agent"
	"github.com/Impre-visible/mihari/internal/gitlab"
)

type replyArgs struct {
	Body string `json:"body" desc:"La réponse, en Markdown"`
}

func init() {
	agent.Register(agent.Define("reply", "Répond à la personne qui t'a sollicité : ton résumé, ta réponse ou ta question. Ton plan (todo_write) y est repris en tête. Appelle-le avant finish.",
		func(ctx context.Context, env *agent.Env, a replyArgs) (string, error) {
			if strings.TrimSpace(a.Body) == "" {
				return "", errors.New("réponse vide")
			}
			// The plan's own comment gives way to the answer, which notifies.
			plan := env.ClosePlan(ctx)
			var note gitlab.Note
			if !env.Keep(agent.Post{Kind: "reply", Body: plan + a.Body}) {
				var err error
				if note, err = env.Respond(ctx, plan+a.Body); err != nil {
					return "", err
				}
			}
			if plan != "" {
				env.PlanTakenIn(note.ID, a.Body)
			}
			env.Replied = true
			return "Réponse postée.", nil
		}, agent.When(hasTarget), agent.Writes()))
}
