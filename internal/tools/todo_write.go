package tools

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/Impre-visible/mihari/internal/agent"
	"github.com/Impre-visible/mihari/internal/gitlab"
)

type todoWriteArgs struct {
	Todos []agent.Todo `json:"todos" desc:"Le plan complet, dans l'ordre : chaque appel remplace le précédent"`
}

func init() {
	agent.Register(agent.Define("todo_write", "Tiens ton plan à jour : les étapes de la tâche et leur état (pending, in_progress, completed). Fais-le dès que la tâche a plusieurs étapes, puis à chaque étape finie ; une seule in_progress à la fois. La personne le suit sur GitLab.",
		func(ctx context.Context, env *agent.Env, a todoWriteArgs) (string, error) {
			if len(a.Todos) == 0 {
				return "", errors.New("plan vide")
			}
			done := 0
			for i, t := range a.Todos {
				if strings.TrimSpace(t.Content) == "" {
					return "", fmt.Errorf("étape %d sans contenu", i+1)
				}
				switch t.Status {
				case "completed":
					done++
				case "pending", "in_progress":
				default:
					return "", fmt.Errorf("état inconnu %q pour l'étape %d : pending, in_progress ou completed", t.Status, i+1)
				}
			}
			env.Todos = a.Todos
			// Once an answer shows the plan, updates go to it, not to a new comment.
			env.PlanOpen = env.PlanBody == ""
			result := fmt.Sprintf("Plan à jour : %d/%d étapes faites.", done, len(a.Todos))
			if !hasTarget(env) || env.Keep(agent.Post{Kind: "plan", Body: env.PlanNote()}) {
				return result, nil
			}
			// Editing the plan's comment notifies nobody: only the first post does.
			var err error
			if env.PlanNoteID != 0 {
				err = env.GitLab.EditNote(ctx, env.Target, env.PlanNoteID, env.PlanNote())
			} else {
				var note gitlab.Note
				if note, err = env.Respond(ctx, env.Plan()); err == nil {
					env.PlanNoteID = note.ID
				}
			}
			if err != nil {
				// The plan still guides the model; GitLab only shows it.
				result += " (pas affiché sur GitLab : " + err.Error() + ")"
			}
			return result, nil
		}, agent.Writes()))
}
