package tools

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/Impre-visible/mihari/internal/agent"
)

type askUserArgs struct {
	Question string   `json:"question" desc:"La question, courte et précise, en Markdown"`
	Choices  []string `json:"choices,omitempty" desc:"Réponses possibles, si la question en a quelques-unes"`
}

// How long ask_user waits; a variable for tests.
var askTimeout = 30 * time.Minute

func init() {
	agent.Register(agent.Define("ask_user", "Pose une question à la personne qui t'a sollicité, sur GitLab, et attend sa réponse (30 minutes au plus). Seulement quand un choix lui revient vraiment ; sinon décide seul.",
		func(ctx context.Context, env *agent.Env, a askUserArgs) (string, error) {
			if strings.TrimSpace(a.Question) == "" {
				return "", errors.New("question vide")
			}
			var b strings.Builder
			if env.Requester != "" {
				b.WriteString("@" + env.Requester + " ")
			}
			b.WriteString(strings.TrimSpace(a.Question))
			for i, c := range a.Choices {
				fmt.Fprintf(&b, "\n%d. %s", i+1, c)
			}
			if env.Keep(agent.Post{Kind: "question", Body: b.String()}) {
				return "Essai : personne ne répondra. Décide seul et continue.", nil
			}
			if _, err := env.Respond(ctx, b.String()); err != nil {
				return "", err
			}
			timer := time.NewTimer(askTimeout)
			defer timer.Stop()
			select {
			case <-ctx.Done():
				return "", ctx.Err()
			case <-timer.C:
				return fmt.Sprintf("Pas de réponse après %s. Décide seul et dis-le dans ta réponse finale.", askTimeout), nil
			case m, ok := <-env.Inbox:
				if !ok {
					return "Plus personne ne suit le fil. Décide seul.", nil
				}
				return fmt.Sprintf("%s :\n%s", m.Title, m.Text), nil
			}
		}, agent.When(pinged), agent.Writes()))
}
