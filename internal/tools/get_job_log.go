package tools

import (
	"context"
	"fmt"

	"github.com/Impre-visible/mihari/internal/agent"
)

type getJobLogArgs struct {
	JobID     int `json:"job_id" desc:"Identifiant du job CI, tel que get_pipeline l'affiche"`
	TailBytes int `json:"tail_bytes,omitempty" desc:"Octets lus depuis la fin du log (20000 par défaut, 200000 au plus)"`
}

func init() {
	agent.Register(agent.Define("get_job_log", "Fin du log d'un job CI précis, plus longue que dans get_pipeline.",
		func(ctx context.Context, env *agent.Env, a getJobLogArgs) (string, error) {
			n := a.TailBytes
			if n <= 0 {
				n = 20000
			}
			log, err := env.GitLab.JobLog(ctx, a.JobID, min(n, 200000))
			if err != nil {
				return "", err
			}
			return fmt.Sprintf("=== log du job %d\n%s", a.JobID, log), nil
		}, agent.When(hasTarget)))
}
