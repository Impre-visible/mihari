package tools

import (
	"context"
	"errors"
	"strings"

	"github.com/Impre-visible/mihari/internal/agent"
)

type pushArgs struct {
	Message string `json:"message" desc:"Message de commit, première ligne courte"`
}

func init() {
	agent.Register(agent.Define("push", "Commit toutes tes modifications du repo et les pousse sur la branche du job. Jamais de force-push : si la branche a bougé, ça échoue.",
		func(ctx context.Context, env *agent.Env, a pushArgs) (string, error) {
			if strings.TrimSpace(a.Message) == "" {
				return "", errors.New("message de commit vide")
			}
			sha, err := env.Push(ctx, a.Message)
			if err != nil {
				return "", err
			}
			env.Pushes = append(env.Pushes, sha)
			return "Commit " + sha + " poussé.", nil
		}, agent.When(canPush), agent.Writes()))
}
