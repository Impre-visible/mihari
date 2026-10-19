package tools

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/Impre-visible/mihari/internal/agent"
)

type readFileArgs struct {
	Path      string `json:"path" desc:"Chemin du fichier, relatif à la racine du repo"`
	StartLine int    `json:"start_line,omitempty" desc:"Première ligne à lire (1 par défaut)"`
	EndLine   int    `json:"end_line,omitempty" desc:"Dernière ligne à lire (fin du fichier par défaut)"`
}

func init() {
	agent.Register(agent.Define("read_file", "Lit un fichier du repo, avec les numéros de ligne.",
		func(ctx context.Context, env *agent.Env, a readFileArgs) (string, error) {
			start := max(1, a.StartLine)
			// awk compares numbers, so a huge bound reads to the end.
			end := "1e18"
			if a.EndLine > 0 {
				end = strconv.Itoa(a.EndLine)
			}
			cmd := fmt.Sprintf("test -f %[1]s || { echo 'fichier introuvable' >&2; exit 1; }; awk 'NR>=%[2]d && NR<=%[3]s {printf \"%%6d  %%s\\n\", NR, $0}' %[1]s",
				quote(inWorkspace(a.Path)), start, end)
			res, err := env.Runner.Exec(ctx, cmd, nil, 30*time.Second)
			if err != nil {
				return "", err
			}
			if res.ExitCode != 0 {
				return "", errors.New(strings.TrimSpace(res.Stderr))
			}
			return format(res), nil
		}))
}
