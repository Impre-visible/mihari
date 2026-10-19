package tools

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/Impre-visible/mihari/internal/agent"
)

type gitLogArgs struct {
	Path     string `json:"path,omitempty" desc:"Seulement les commits qui touchent ce fichier ou dossier"`
	Ref      string `json:"ref,omitempty" desc:"Branche ou commit de départ (HEAD par défaut)"`
	MaxCount int    `json:"max_count,omitempty" desc:"Nombre de commits (20 par défaut, 200 au plus)"`
	Patch    bool   `json:"patch,omitempty" desc:"Montre aussi le diff de chaque commit"`
}

func init() {
	agent.Register(agent.Define("git_log", "Historique des commits du repo, du plus récent au plus ancien. Le clone peut être partiel.",
		func(ctx context.Context, env *agent.Env, a gitLogArgs) (string, error) {
			if strings.HasPrefix(a.Ref, "-") {
				return "", errors.New("ref invalide")
			}
			n := a.MaxCount
			if n <= 0 {
				n = 20
			}
			cmd := fmt.Sprintf("%s%s log -n %d --date=short --format='%%h %%ad %%an : %%s'", needs("git"), git, min(n, 200))
			if a.Patch {
				cmd += " --patch"
			}
			if a.Ref != "" {
				cmd += " " + quote(a.Ref)
			}
			if a.Path != "" {
				cmd += " -- " + quote(inWorkspace(a.Path))
			}
			res, err := env.Runner.Exec(ctx, cmd, nil, 60*time.Second)
			if err != nil {
				return "", err
			}
			return format(res), nil
		}))
}
