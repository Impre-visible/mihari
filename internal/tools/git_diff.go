package tools

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/Impre-visible/mihari/internal/agent"
)

type gitDiffArgs struct {
	Path string `json:"path,omitempty" desc:"Un seul fichier ou dossier ; tout le repo si vide"`
	Ref  string `json:"ref,omitempty" desc:"Commit ou branche de comparaison (HEAD par défaut : tes changements pas encore poussés)"`
}

func init() {
	agent.Register(agent.Define("git_diff", "Diff local du repo : tes changements depuis le dernier commit, nouveaux fichiers compris. Relis-le avant push.",
		func(ctx context.Context, env *agent.Env, a gitDiffArgs) (string, error) {
			if strings.HasPrefix(a.Ref, "-") {
				return "", errors.New("ref invalide")
			}
			ref := a.Ref
			if ref == "" {
				ref = "HEAD"
			}
			path := ""
			if a.Path != "" {
				path = " " + quote(inWorkspace(a.Path))
			}
			// New files are untracked, so git diff alone leaves them out.
			cmd := needs("git") + git + " diff " + quote(ref) + " --" + path +
				" && { " + git + " ls-files -z --others --exclude-standard --" + path +
				" | xargs -0 -r -n1 " + git + " diff --no-index -- /dev/null; true; }"
			out, err := output(ctx, env, cmd, nil, 60*time.Second)
			if err != nil {
				return "", err
			}
			if strings.TrimSpace(out) == "" {
				return "Aucun changement.", nil
			}
			return out, nil
		}))
}
