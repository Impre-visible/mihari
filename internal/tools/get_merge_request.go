package tools

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/Impre-visible/mihari/internal/agent"
)

type getMergeRequestArgs struct {
	IID int `json:"iid,omitempty" desc:"Numéro d'une autre MR du projet (le 12 de !12) ; celle du job si vide"`
}

func init() {
	agent.Register(agent.Define("get_merge_request", "Titre, description, auteur et branches d'une MR : celle du job, ou une autre citée (!12).",
		func(ctx context.Context, env *agent.Env, a getMergeRequestArgs) (string, error) {
			mr := env.MR
			if a.IID != 0 && a.IID != mr.IID {
				var err error
				if mr, err = env.GitLab.MergeRequest(ctx, a.IID); err != nil {
					return "", err
				}
			}
			if mr.IID == 0 {
				return "", errors.New("ce job n'a pas de MR : donne un iid")
			}
			return fmt.Sprintf("!%d %s\nAuteur : @%s\nBranche : %s → %s\nÉtat : %s\n%s\n\n%s",
				mr.IID, mr.Title, mr.Author, mr.SourceBranch, mr.TargetBranch, mr.State, mr.WebURL,
				strings.TrimSpace(mr.Description)), nil
		}, agent.When(hasTarget)))
}
