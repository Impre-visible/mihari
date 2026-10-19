package tools

import (
	"context"
	"fmt"
	"path"
	"time"

	"github.com/Impre-visible/mihari/internal/agent"
)

type writeFileArgs struct {
	Path    string `json:"path" desc:"Chemin du fichier, relatif à la racine du repo"`
	Content string `json:"content" desc:"Contenu complet du fichier"`
}

func init() {
	agent.Register(agent.Define("write_file", "Crée ou remplace un fichier entier (les dossiers sont créés au besoin). Pour changer quelques lignes d'un fichier existant, edit_file est plus sûr.",
		func(ctx context.Context, env *agent.Env, a writeFileArgs) (string, error) {
			if err := write(ctx, env, a.Path, a.Content); err != nil {
				return "", err
			}
			return fmt.Sprintf("%s écrit (%d octets).", a.Path, len(a.Content)), nil
		}, agent.Writes()))
}

// write replaces the file at p with content.
func write(ctx context.Context, env *agent.Env, p, content string) error {
	target := inWorkspace(p)
	cmd := fmt.Sprintf("mkdir -p %s && cat > %s", quote(path.Dir(target)), quote(target))
	_, err := output(ctx, env, cmd, []byte(content), 30*time.Second)
	return err
}
