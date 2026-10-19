package tools

import (
	"context"
	"crypto/rand"
	"fmt"
	"strings"
	"time"

	"github.com/Impre-visible/mihari/internal/agent"
)

type startProcessArgs struct {
	Command     string `json:"command" desc:"Commande shell qui tourne longtemps : serveur de dev, watcher, base de données…"`
	Name        string `json:"name,omitempty" desc:"Identifiant pour read_process et stop_process (minuscules, chiffres, tirets) ; tiré au hasard si vide"`
	WaitSeconds int    `json:"wait_seconds,omitempty" desc:"Secondes à attendre avant de rendre le début de la sortie (3 par défaut, 60 au plus)"`
}

func init() {
	agent.Register(agent.Define("start_process", "Lance une commande en arrière-plan dans le container et rend la main tout de suite, avec le début de sa sortie. Lis la suite avec read_process, arrête-la avec stop_process.",
		func(ctx context.Context, env *agent.Env, a startProcessArgs) (string, error) {
			id := a.Name
			if id == "" {
				id = strings.ToLower(rand.Text()[:6])
			}
			pid, log, err := procFiles(id)
			if err != nil {
				return "", err
			}
			wait := a.WaitSeconds
			if wait <= 0 {
				wait = 3
			}
			wait = min(wait, 60)
			// In its own session when setsid exists, so stop_process takes its children too.
			cmd := fmt.Sprintf(`mkdir -p %[1]s
if [ -f %[2]s ] && kill -0 "$(cat %[2]s)" 2>/dev/null; then echo 'un process tourne déjà sous ce nom' >&2; exit 1; fi
if command -v setsid >/dev/null 2>&1; then setsid sh -c %[4]s >%[3]s 2>&1 </dev/null & else sh -c %[4]s >%[3]s 2>&1 </dev/null & fi
echo $! >%[2]s
sleep %[5]d
if kill -0 "$(cat %[2]s)" 2>/dev/null; then echo "[en cours, pid $(cat %[2]s)]"; else echo '[déjà terminé]'; fi
tail -n 50 %[3]s`, procDir, pid, log, quote(a.Command), wait)
			out, err := output(ctx, env, cmd, nil, time.Duration(wait+30)*time.Second)
			if err != nil {
				return "", err
			}
			return fmt.Sprintf("Process %q lancé.\n%s", id, strings.TrimRight(out, "\n")), nil
		}))
}
