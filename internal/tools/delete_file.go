package tools

import (
	"context"
	"fmt"
	"time"

	"github.com/Impre-visible/mihari/internal/agent"
)

type deleteFileArgs struct {
	Path      string `json:"path" desc:"Fichier ou dossier à supprimer, relatif à la racine du repo"`
	Recursive bool   `json:"recursive,omitempty" desc:"Nécessaire pour supprimer un dossier et son contenu"`
}

func init() {
	agent.Register(agent.Define("delete_file", "Supprime un fichier du repo, ou un dossier avec recursive.",
		func(ctx context.Context, env *agent.Env, a deleteFileArgs) (string, error) {
			target := inWorkspace(a.Path)
			if target == inWorkspace("") {
				return "", fmt.Errorf("refusé : c'est la racine du repo")
			}
			rm := "rm"
			if a.Recursive {
				rm = "rm -r"
			}
			cmd := fmt.Sprintf("test -e %[1]s || { echo 'introuvable' >&2; exit 1; }; test -d %[1]s && [ %[2]t = false ] && { echo 'c'\\''est un dossier : passe recursive' >&2; exit 1; }; %[3]s -f %[1]s",
				quote(target), a.Recursive, rm)
			if _, err := output(ctx, env, cmd, nil, 30*time.Second); err != nil {
				return "", err
			}
			return a.Path + " supprimé.", nil
		}, agent.Writes()))
}
