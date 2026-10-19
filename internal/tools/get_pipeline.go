package tools

import (
	"context"
	"fmt"
	"strings"

	"github.com/Impre-visible/mihari/internal/agent"
	"github.com/Impre-visible/mihari/internal/gitlab"
)

func init() {
	agent.Register(agent.Define("get_pipeline", "État du dernier pipeline CI de la MR, et la fin du log des jobs en échec. Pour le log complet d'un job, get_job_log.",
		func(ctx context.Context, env *agent.Env, _ agent.NoArgs) (string, error) {
			p, ok, err := env.GitLab.LatestPipeline(ctx, env.MR.IID)
			if err != nil {
				return "", err
			}
			if !ok {
				return "Cette MR n'a pas de pipeline.", nil
			}
			var b strings.Builder
			fmt.Fprintf(&b, "Pipeline %d : %s (commit %s)\n", p.ID, p.Status, short(p.SHA))
			var failed []gitlab.PipelineJob
			for _, j := range p.Jobs {
				fmt.Fprintf(&b, "- [job %d] %s / %s : %s\n", j.ID, j.Stage, j.Name, j.Status)
				if j.Status == "failed" {
					failed = append(failed, j)
				}
			}
			// The end of a failing log is where the error is; three logs are plenty.
			for _, j := range failed[:min(3, len(failed))] {
				log, err := env.GitLab.JobLog(ctx, j.ID, 4000)
				if err != nil {
					fmt.Fprintf(&b, "\n=== %s : log illisible (%v)\n", j.Name, err)
					continue
				}
				fmt.Fprintf(&b, "\n=== fin du log de %s\n%s\n", j.Name, log)
			}
			return b.String(), nil
		}, agent.When(hasMR)))
}
