package agent

import (
	"context"
	"strings"
)

type finishArgs struct {
	Success bool   `json:"success" desc:"true si la demande est traitée, false si tu n'as pas pu"`
	Summary string `json:"summary" desc:"Ce que tu as fait, ou pourquoi tu n'as pas pu, en 1 à 3 phrases"`
}

func init() {
	Register(Define("finish", "Termine le job. Appelle-le une seule fois, quand tout est fait (ou impossible).",
		func(_ context.Context, env *Env, a finishArgs) (string, error) {
			env.Finished = &Finish{Success: a.Success, Summary: strings.TrimSpace(a.Summary)}
			return "Job terminé.", nil
		}))
}
