package tools

import (
	"context"
	"fmt"
	"time"

	"github.com/Impre-visible/mihari/internal/agent"
)

type gitBlameArgs struct {
	Path      string `json:"path" desc:"Fichier, relatif à la racine du repo"`
	StartLine int    `json:"start_line,omitempty" desc:"Première ligne (1 par défaut)"`
	EndLine   int    `json:"end_line,omitempty" desc:"Dernière ligne (start_line + 50 par défaut)"`
}

func init() {
	agent.Register(agent.Define("git_blame", "Qui a écrit chaque ligne d'un passage, et dans quel commit : pour comprendre pourquoi un code est là.",
		func(ctx context.Context, env *agent.Env, a gitBlameArgs) (string, error) {
			start := max(1, a.StartLine)
			end := a.EndLine
			if end < start {
				end = start + 50
			}
			cmd := fmt.Sprintf("%s%s blame --date=short -L %d,%d -- %s", needs("git"), git, start, end, quote(inWorkspace(a.Path)))
			res, err := env.Runner.Exec(ctx, cmd, nil, 60*time.Second)
			if err != nil {
				return "", err
			}
			return format(res), nil
		}))
}
