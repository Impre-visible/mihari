package tools

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/Impre-visible/mihari/internal/agent"
)

type applyPatchArgs struct {
	Patch string `json:"patch" desc:"Le patch : au format *** Begin Patch / *** Update File: chemin / @@ / *** End Patch, ou un diff unifié (comme git diff, préfixes a/ et b/). Chemins relatifs à la racine du repo"`
}

func init() {
	agent.Register(agent.Define("apply_patch", `Applique un patch au repo, tout ou rien : pratique pour plusieurs changements dans plusieurs fichiers en un appel. Deux formats :
*** Begin Patch
*** Add File: chemin        (suivi de lignes +)
*** Delete File: chemin
*** Update File: chemin     (optionnellement *** Move to: nouveau/chemin)
@@ ligne de repère facultative
 contexte
-ligne retirée
+ligne ajoutée
*** End Patch
ou un diff unifié, appliqué avec git apply.`,
		func(ctx context.Context, env *agent.Env, a applyPatchArgs) (string, error) {
			if strings.TrimSpace(a.Patch) == "" {
				return "", errors.New("patch vide")
			}
			if isCodexPatch(a.Patch) {
				return applyCodexPatch(ctx, env, a.Patch)
			}
			patch := a.Patch
			if !strings.HasSuffix(patch, "\n") {
				patch += "\n"
			}
			out, err := output(ctx, env, needs("git")+git+" apply --verbose --recount --whitespace=nowarn - 2>&1", []byte(patch), 60*time.Second)
			if err != nil {
				return "", err
			}
			return "Patch appliqué.\n" + strings.TrimSpace(out), nil
		}, agent.Writes()))
}

// applyCodexPatch works out every file's new content before touching any,
// so that a hunk that does not apply leaves the repository as it was.
func applyCodexPatch(ctx context.Context, env *agent.Env, patch string) (string, error) {
	ops, err := parseCodexPatch(patch)
	if err != nil {
		return "", err
	}
	type change struct {
		path    string
		content string
		delete  bool
	}
	var changes []change
	var summary []string
	for _, op := range ops {
		switch op.kind {
		case "add":
			changes = append(changes, change{path: op.path, content: strings.Join(op.lines, "\n") + "\n"})
			summary = append(summary, "A "+op.path)
		case "delete":
			if _, err := output(ctx, env, "test -f "+quote(inWorkspace(op.path)), nil, 10*time.Second); err != nil {
				return "", fmt.Errorf("%s n'existe pas", op.path)
			}
			changes = append(changes, change{path: op.path, delete: true})
			summary = append(summary, "D "+op.path)
		case "update":
			content, err := readFile(ctx, env, inWorkspace(op.path), editLimit+1)
			if err != nil {
				return "", err
			}
			if len(content) > editLimit {
				return "", fmt.Errorf("%s est trop gros pour ce format : utilise un diff unifié", op.path)
			}
			updated, err := applyCodexChunks(content, op.path, op.chunks)
			if err != nil {
				return "", err
			}
			if op.moveTo != "" && op.moveTo != op.path {
				changes = append(changes, change{path: op.moveTo, content: updated}, change{path: op.path, delete: true})
				summary = append(summary, fmt.Sprintf("M %s → %s", op.path, op.moveTo))
			} else {
				changes = append(changes, change{path: op.path, content: updated})
				summary = append(summary, "M "+op.path)
			}
		}
	}
	for _, c := range changes {
		var err error
		if c.delete {
			_, err = output(ctx, env, "rm -f "+quote(inWorkspace(c.path)), nil, 10*time.Second)
		} else {
			err = write(ctx, env, c.path, c.content)
		}
		if err != nil {
			return "", fmt.Errorf("%s : %w (patch appliqué en partie)", c.path, err)
		}
	}
	return "Patch appliqué.\n" + strings.Join(summary, "\n"), nil
}
