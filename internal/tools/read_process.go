package tools

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/Impre-visible/mihari/internal/agent"
)

type readProcessArgs struct {
	Name  string `json:"name,omitempty" desc:"Identifiant donné par start_process ; vide pour lister les process"`
	Lines int    `json:"lines,omitempty" desc:"Dernières lignes de sortie à lire (100 par défaut, 1000 au plus)"`
}

func init() {
	agent.Register(agent.Define("read_process", "État et dernières lignes de sortie d'un process lancé par start_process, ou la liste des process sans name.",
		func(ctx context.Context, env *agent.Env, a readProcessArgs) (string, error) {
			if a.Name == "" {
				cmd := fmt.Sprintf(`for f in %s/*.pid; do [ -f "$f" ] || continue; id=$(basename "$f" .pid); if kill -0 "$(cat "$f")" 2>/dev/null; then echo "$id : en cours"; else echo "$id : terminé"; fi; done`, procDir)
				out, err := output(ctx, env, cmd, nil, 30*time.Second)
				if err != nil {
					return "", err
				}
				if strings.TrimSpace(out) == "" {
					return "Aucun process lancé.", nil
				}
				return out, nil
			}
			pid, log, err := procFiles(a.Name)
			if err != nil {
				return "", err
			}
			lines := a.Lines
			if lines <= 0 {
				lines = 100
			}
			cmd := fmt.Sprintf(`[ -f %[1]s ] || { echo 'aucun process sous ce nom' >&2; exit 1; }
if kill -0 "$(cat %[1]s)" 2>/dev/null; then echo '[en cours]'; else echo '[terminé]'; fi
tail -n %[3]d %[2]s`, pid, log, min(lines, 1000))
			return output(ctx, env, cmd, nil, 30*time.Second)
		}))
}
