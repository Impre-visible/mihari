package tools

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/Impre-visible/mihari/internal/agent"
)

type openMRArgs struct {
	Title       string `json:"title" desc:"Titre de la MR, court"`
	Description string `json:"description" desc:"Ce que tu as changé et pourquoi, en Markdown"`
}

func init() {
	agent.Register(agent.Define("open_merge_request", "Commit tes modifications, pousse la branche et ouvre une MR en brouillon vers la branche par défaut. Une seule fois par job ; ensuite, push met la MR à jour.",
		func(ctx context.Context, env *agent.Env, a openMRArgs) (string, error) {
			if strings.TrimSpace(a.Title) == "" {
				return "", errors.New("titre vide")
			}
			mr, err := env.OpenMR(ctx, a.Title, a.Description)
			if err != nil {
				return "", err
			}
			env.OpenedMR, env.MR = &mr, mr
			return fmt.Sprintf("MR !%d ouverte en brouillon : %s", mr.IID, mr.WebURL), nil
		}, agent.When(canOpenMR), agent.Writes()))
}
