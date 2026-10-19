package tools

import (
	"context"
	"fmt"
	"time"

	"github.com/Impre-visible/mihari/internal/agent"
)

type listFilesArgs struct {
	Path  string `json:"path,omitempty" desc:"Dossier à lister, relatif à la racine du repo"`
	Depth int    `json:"depth,omitempty" desc:"Profondeur maximale (2 par défaut)"`
}

func init() {
	agent.Register(agent.Define("list_files", "Liste les fichiers et dossiers du repo, sans .git.",
		func(ctx context.Context, env *agent.Env, a listFilesArgs) (string, error) {
			depth := a.Depth
			if depth <= 0 {
				depth = 2
			}
			cmd := fmt.Sprintf("cd %s && find . -maxdepth %d -not -path './.git' -not -path './.git/*' | sort | head -500",
				quote(inWorkspace(a.Path)), depth)
			res, err := env.Runner.Exec(ctx, cmd, nil, 30*time.Second)
			if err != nil {
				return "", err
			}
			return format(res), nil
		}))
}
