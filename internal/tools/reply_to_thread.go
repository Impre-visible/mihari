package tools

import (
	"context"
	"errors"
	"strings"

	"github.com/Impre-visible/mihari/internal/agent"
)

type replyThreadArgs struct {
	ThreadID string `json:"thread_id" desc:"Identifiant du fil, tel que read_comments l'affiche"`
	Body     string `json:"body" desc:"La réponse, en Markdown"`
}

func init() {
	agent.Register(agent.Define("reply_to_thread", "Répond dans un fil précis, par exemple pour dire qu'une de tes remarques est corrigée. Ne résout jamais le fil : c'est à l'humain.",
		func(ctx context.Context, env *agent.Env, a replyThreadArgs) (string, error) {
			if strings.TrimSpace(a.Body) == "" || strings.TrimSpace(a.ThreadID) == "" {
				return "", errors.New("thread_id et body sont obligatoires")
			}
			if !env.Keep(agent.Post{Kind: "reply", Body: a.Body}) {
				if _, err := env.GitLab.Reply(ctx, env.Target, a.ThreadID, a.Body); err != nil {
					return "", err
				}
			}
			return "Réponse postée dans le fil.", nil
		}, agent.When(hasTarget)))
}
