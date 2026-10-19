package tools

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/Impre-visible/mihari/internal/agent"
)

type searchArgs struct {
	Pattern    string `json:"pattern" desc:"Expression régulière (grep -E)"`
	Path       string `json:"path,omitempty" desc:"Dossier ou fichier où chercher"`
	Glob       string `json:"glob,omitempty" desc:"Seulement les fichiers dont le nom correspond, par exemple *.go"`
	IgnoreCase bool   `json:"ignore_case,omitempty" desc:"Ignore la casse"`
	Context    int    `json:"context,omitempty" desc:"Lignes affichées avant et après chaque résultat (0 par défaut, 10 au plus)"`
	FilesOnly  bool   `json:"files_only,omitempty" desc:"Liste seulement les fichiers qui contiennent le motif"`
}

func init() {
	agent.Register(agent.Define("search", "Cherche une expression dans les fichiers du repo (grep), avec fichier et ligne.",
		func(ctx context.Context, env *agent.Env, a searchArgs) (string, error) {
			flags := []string{"-rnE", "--exclude-dir=.git", "--exclude-dir=node_modules"}
			if a.IgnoreCase {
				flags = append(flags, "-i")
			}
			if a.Context > 0 {
				flags = append(flags, fmt.Sprintf("-C%d", min(a.Context, 10)))
			}
			if a.Glob != "" {
				flags = append(flags, "--include="+quote(a.Glob))
			}
			if a.FilesOnly {
				flags = append(flags, "-l")
			}
			cmd := fmt.Sprintf("grep %s -- %s %s | head -300", strings.Join(flags, " "), quote(a.Pattern), quote(inWorkspace(a.Path)))
			res, err := env.Runner.Exec(ctx, cmd, nil, 60*time.Second)
			if err != nil {
				return "", err
			}
			if res.Stdout == "" && res.Stderr == "" {
				return "Aucun résultat.", nil
			}
			return format(res), nil
		}))
}
