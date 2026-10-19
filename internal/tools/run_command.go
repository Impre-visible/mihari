package tools

import (
	"context"
	"time"

	"github.com/Impre-visible/mihari/internal/agent"
)

type runCommandArgs struct {
	Command        string `json:"command" desc:"Commande shell, lancée avec sh -c à la racine du repo"`
	TimeoutSeconds int    `json:"timeout_seconds,omitempty" desc:"Délai maximum (120 par défaut, 600 au plus)"`
}

func init() {
	agent.Register(agent.Define("run_command", "Lance une commande shell dans le container du job (tests, build, installation de paquets…) et attend sa fin. Le réseau est disponible. Pour un serveur ou un watcher qui ne s'arrête pas, utilise start_process.",
		func(ctx context.Context, env *agent.Env, a runCommandArgs) (string, error) {
			timeout := a.TimeoutSeconds
			if timeout <= 0 {
				timeout = 120
			}
			res, err := env.Runner.Exec(ctx, a.Command, nil, time.Duration(min(timeout, 600))*time.Second)
			if err != nil {
				return "", err
			}
			return format(res), nil
		}))
}
