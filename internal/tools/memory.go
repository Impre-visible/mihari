package tools

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/Impre-visible/mihari/internal/agent"
)

type memoryOp struct {
	Action  string `json:"action" enum:"add,replace,remove" desc:"add, replace ou remove"`
	Content string `json:"content,omitempty" desc:"La note à ajouter, ou son nouveau texte pour replace, en une phrase"`
	OldText string `json:"old_text,omitempty" desc:"Un extrait de la note à remplacer ou supprimer, qui ne désigne qu'elle"`
}

type memoryArgs struct {
	Operations []memoryOp `json:"operations,omitempty" desc:"Les changements, appliqués ensemble ; vide pour seulement relire les notes"`
}

// Notes past this many drop their oldest ones.
const memoryLimit = 200

func init() {
	agent.Register(agent.Define("memory", "Les notes que tu gardes sur ce projet pour tes prochains jobs : convention, commande de test, piège, préférence de l'équipe. Ajoute, corrige ou supprime en un appel ; une note périmée se remplace plutôt que de s'empiler. Pas de note sur la tâche en cours.",
		func(_ context.Context, env *agent.Env, a memoryArgs) (string, error) {
			notes, err := applyMemory(env.Notes(), a.Operations, time.Now())
			if err != nil {
				return "", err
			}
			if len(a.Operations) > 0 {
				if err := env.SetNotes(notes); err != nil {
					return "", err
				}
			}
			if notes == "" {
				return "Aucune note sur ce projet.", nil
			}
			return "Notes du projet :\n" + notes, nil
		}, agent.When(func(env *agent.Env) bool { return env.Notes != nil && env.SetNotes != nil }), agent.Writes()))
}

// applyMemory runs ops on notes, one "- date note" line each, all or none.
func applyMemory(notes string, ops []memoryOp, now time.Time) (string, error) {
	var lines []string
	for _, l := range strings.Split(notes, "\n") {
		if strings.TrimSpace(l) != "" {
			lines = append(lines, l)
		}
	}
	find := func(old string) (int, error) {
		old = strings.TrimSpace(old)
		if old == "" {
			return 0, errors.New("old_text est vide")
		}
		found := -1
		for i, l := range lines {
			if strings.Contains(l, old) {
				if found >= 0 {
					return 0, fmt.Errorf("%q désigne plusieurs notes : précise", old)
				}
				found = i
			}
		}
		if found < 0 {
			return 0, fmt.Errorf("aucune note ne contient %q", old)
		}
		return found, nil
	}
	line := func(content string) (string, error) {
		content = strings.Join(strings.Fields(content), " ")
		if content == "" {
			return "", errors.New("content est vide")
		}
		return fmt.Sprintf("- %s %s", now.Format("2006-01-02"), content), nil
	}
	apply := func(op memoryOp) error {
		switch op.Action {
		case "add":
			l, err := line(op.Content)
			if err != nil {
				return err
			}
			// The same lesson twice is noise.
			if !strings.Contains(strings.Join(lines, "\n"), strings.SplitN(l, " ", 3)[2]) {
				lines = append(lines, l)
			}
		case "replace":
			i, err := find(op.OldText)
			if err != nil {
				return err
			}
			if lines[i], err = line(op.Content); err != nil {
				return err
			}
		case "remove":
			i, err := find(op.OldText)
			if err != nil {
				return err
			}
			lines = append(lines[:i], lines[i+1:]...)
		default:
			return fmt.Errorf("action inconnue : add, replace ou remove")
		}
		return nil
	}
	for n, op := range ops {
		if err := apply(op); err != nil {
			return "", fmt.Errorf("opération %d (%s) : %w", n+1, op.Action, err)
		}
	}
	if len(lines) > memoryLimit {
		lines = lines[len(lines)-memoryLimit:]
	}
	return strings.Join(lines, "\n"), nil
}
