package tools

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/Impre-visible/mihari/internal/agent"
)

type editFileArgs struct {
	Path       string `json:"path" desc:"Chemin du fichier, relatif à la racine du repo"`
	OldString  string `json:"old_string" desc:"Texte exact à remplacer, espaces et indentation compris ; assez long pour n'apparaître qu'une fois"`
	NewString  string `json:"new_string" desc:"Texte qui le remplace"`
	ReplaceAll bool   `json:"replace_all,omitempty" desc:"Remplace toutes les occurrences au lieu d'exiger qu'il n'y en ait qu'une"`
}

// Files edit_file reads whole; past this, apply_patch or run_command do better.
const editLimit = 2 << 20

func init() {
	agent.Register(agent.Define("edit_file", "Remplace un passage exact d'un fichier existant par un autre, sans réécrire tout le fichier. Lis le fichier avant, et copie old_string tel quel.",
		func(ctx context.Context, env *agent.Env, a editFileArgs) (string, error) {
			if a.OldString == "" {
				return "", errors.New("old_string est vide : pour créer un fichier, utilise write_file")
			}
			if a.OldString == a.NewString {
				return "", errors.New("old_string et new_string sont identiques")
			}
			content, err := readFile(ctx, env, inWorkspace(a.Path), editLimit+1)
			if err != nil {
				return "", err
			}
			if len(content) > editLimit {
				return "", errors.New("fichier trop gros pour edit_file : utilise apply_patch")
			}
			updated, line, err := replace(content, a.OldString, a.NewString, a.ReplaceAll)
			if err != nil {
				return "", err
			}
			if err := write(ctx, env, a.Path, updated); err != nil {
				return "", err
			}
			return fmt.Sprintf("%s modifié à la ligne %d.", a.Path, line), nil
		}, agent.Writes()))
}

// replace swaps old for new in content and returns the line of the first change.
func replace(content, old, new string, all bool) (string, int, error) {
	n := strings.Count(content, old)
	switch {
	case n == 0:
		return "", 0, errors.New("old_string introuvable : relis le fichier et copie le passage exactement, indentation comprise")
	case n > 1 && !all:
		return "", 0, fmt.Errorf("old_string apparaît %d fois : ajoute du contexte autour pour qu'il soit unique, ou passe replace_all", n)
	}
	line := strings.Count(content[:strings.Index(content, old)], "\n") + 1
	if all {
		return strings.ReplaceAll(content, old, new), line, nil
	}
	return strings.Replace(content, old, new, 1), line, nil
}
