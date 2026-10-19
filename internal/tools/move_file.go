package tools

import (
	"context"
	"fmt"
	"path"
	"time"

	"github.com/Impre-visible/mihari/internal/agent"
)

type moveFileArgs struct {
	From string `json:"from" desc:"Fichier ou dossier à déplacer, relatif à la racine du repo"`
	To   string `json:"to" desc:"Nouveau chemin (les dossiers sont créés au besoin)"`
}

func init() {
	agent.Register(agent.Define("move_file", "Déplace ou renomme un fichier ou un dossier du repo. Refuse d'écraser un chemin existant.",
		func(ctx context.Context, env *agent.Env, a moveFileArgs) (string, error) {
			from, to := inWorkspace(a.From), inWorkspace(a.To)
			cmd := fmt.Sprintf("test -e %[1]s || { echo 'introuvable : %[3]s' >&2; exit 1; }; test -e %[2]s && { echo 'existe déjà : %[4]s' >&2; exit 1; }; mkdir -p %[5]s && mv %[1]s %[2]s",
				quote(from), quote(to), path.Base(from), path.Base(to), quote(path.Dir(to)))
			if _, err := output(ctx, env, cmd, nil, 30*time.Second); err != nil {
				return "", err
			}
			return fmt.Sprintf("%s déplacé vers %s.", a.From, a.To), nil
		}))
}
