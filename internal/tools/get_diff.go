package tools

import (
	"context"
	"fmt"
	"strings"

	"github.com/Impre-visible/mihari/internal/agent"
)

type getDiffArgs struct {
	Path string `json:"path,omitempty" desc:"Un seul fichier ; tous les fichiers si vide"`
}

func init() {
	agent.Register(agent.Define("get_diff", "Diff de la MR sur GitLab, fichier par fichier.",
		func(ctx context.Context, env *agent.Env, a getDiffArgs) (string, error) {
			diffs, err := env.GitLab.Diffs(ctx, env.MR.IID)
			if err != nil {
				return "", err
			}
			var b strings.Builder
			for _, d := range diffs {
				if a.Path != "" && d.NewPath != a.Path && d.OldPath != a.Path {
					continue
				}
				header := d.NewPath
				switch {
				case d.New:
					header += " (nouveau)"
				case d.Deleted:
					header += " (supprimé)"
				case d.Renamed:
					header = d.OldPath + " → " + d.NewPath
				}
				fmt.Fprintf(&b, "=== %s\n%s\n", header, d.Diff)
			}
			if b.Len() == 0 {
				return "Aucun fichier ne correspond.", nil
			}
			return b.String(), nil
		}, agent.When(hasMR)))
}
