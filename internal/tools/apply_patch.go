package tools

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/Impre-visible/mihari/internal/agent"
)

type applyPatchArgs struct {
	Patch string `json:"patch" desc:"Diff unifié (comme git diff), chemins relatifs à la racine du repo avec les préfixes a/ et b/"`
}

func init() {
	agent.Register(agent.Define("apply_patch", "Applique un diff unifié au repo (git apply). Pratique pour plusieurs changements dans plusieurs fichiers en un appel ; tout ou rien.",
		func(ctx context.Context, env *agent.Env, a applyPatchArgs) (string, error) {
			if strings.TrimSpace(a.Patch) == "" {
				return "", errors.New("patch vide")
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
		}))
}
