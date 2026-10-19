package tools

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/Impre-visible/mihari/internal/agent"
)

type stopProcessArgs struct {
	Name string `json:"name" desc:"Identifiant donné par start_process"`
}

func init() {
	agent.Register(agent.Define("stop_process", "Arrête un process lancé par start_process, avec ses enfants, et rend la fin de sa sortie.",
		func(ctx context.Context, env *agent.Env, a stopProcessArgs) (string, error) {
			pid, log, err := procFiles(a.Name)
			if err != nil {
				return "", err
			}
			// The negative pid is its process group, there when setsid was.
			cmd := fmt.Sprintf(`[ -f %[1]s ] || { echo 'aucun process sous ce nom' >&2; exit 1; }
p=$(cat %[1]s)
kill -TERM -$p 2>/dev/null || kill -TERM $p 2>/dev/null
for i in 1 2 3 4 5; do kill -0 $p 2>/dev/null || break; sleep 1; done
kill -KILL -$p 2>/dev/null; kill -KILL $p 2>/dev/null
tail -n 30 %[2]s`, pid, log)
			out, err := output(ctx, env, cmd, nil, 30*time.Second)
			if err != nil {
				return "", err
			}
			return fmt.Sprintf("Process %q arrêté.\n%s", a.Name, strings.TrimRight(out, "\n")), nil
		}))
}
