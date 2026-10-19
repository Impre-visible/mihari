package tools

import (
	"context"
	"errors"
	"strings"

	"github.com/Impre-visible/mihari/internal/agent"
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
			body := env.ClosePlan(ctx) + a.Body
			if !env.Keep(agent.Post{Kind: "reply", Body: body}) {
				if _, err := env.Respond(ctx, body); err != nil {
					return "", err
				}
			}
			env.Replied = true
			return "Réponse postée.", nil
		}, agent.When(hasTarget)))
}
