package tools

import (
	"context"
	"strings"

	"github.com/Impre-visible/mihari/internal/agent"
)

type finishArgs struct {
	Success bool   `json:"success" desc:"true si la demande est traitée, false si tu n'as pas pu"`
	Summary string `json:"summary" desc:"Ce que tu as fait, ou pourquoi tu n'as pas pu, en 1 à 3 phrases"`
}

func init() {
	agent.Register(agent.Define("finish", "Termine le job. Appelle-le une seule fois, quand tout est fait (ou impossible).",
		func(_ context.Context, env *agent.Env, a finishArgs) (string, error) {
			env.Finished = &agent.Finish{Success: a.Success, Summary: strings.TrimSpace(a.Summary)}
			return "Job terminé.", nil
		}))
}
