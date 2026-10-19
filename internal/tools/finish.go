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
		func(ctx context.Context, env *agent.Env, a finishArgs) (string, error) {
			env.Finished = &agent.Finish{Success: a.Success, Summary: strings.TrimSpace(a.Summary)}
			if a.Success {
				finishPlan(ctx, env)
			}
			return "Job terminé.", nil
		}))
}

// finishPlan ticks the step still under way when the job ends well: models
// often answer, which was that step, and stop without saying so in the plan.
// Steps never started stay as they are.
func finishPlan(ctx context.Context, env *agent.Env) {
	changed := false
	for i, t := range env.Todos {
		if t.Status == "in_progress" {
			env.Todos[i].Status, changed = "completed", true
		}
	}
	// A plan still in its own comment is taken in by the final answer, as it is now.
	if !changed || env.PlanBody == "" || env.Keep(agent.Post{Kind: "plan", Body: env.PlanNote()}) {
		return
	}
	if env.PlanNoteID != 0 && env.GitLab != nil {
		_ = env.GitLab.EditNote(ctx, env.Target, env.PlanNoteID, env.PlanNote())
	}
}
