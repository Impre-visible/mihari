package tools

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/Impre-visible/mihari/internal/agent"
	"github.com/Impre-visible/mihari/internal/gitlab"
)

type getIssueArgs struct {
	IID int `json:"iid,omitempty" desc:"Numéro d'une autre issue du projet (le 42 de #42) ; celle du job si vide"`
}

func init() {
	agent.Register(agent.Define("get_issue", "Titre, description, auteur et labels d'une issue : celle du job, ou une autre citée (#42).",
		func(ctx context.Context, env *agent.Env, a getIssueArgs) (string, error) {
			var issue gitlab.Issue
			switch {
			case a.IID != 0 && (env.Issue == nil || a.IID != env.Issue.IID):
				var err error
				if issue, err = env.GitLab.Issue(ctx, a.IID); err != nil {
					return "", err
				}
			case env.Issue != nil:
				issue = *env.Issue
			default:
				return "", errors.New("ce job n'a pas d'issue : donne un iid")
			}
			return fmt.Sprintf("#%d %s\nAuteur : @%s\nLabels : %s\nÉtat : %s\n%s\n\n%s",
				issue.IID, issue.Title, issue.Author, strings.Join(issue.Labels, ", "), issue.State, issue.WebURL,
				strings.TrimSpace(issue.Description)), nil
		}, agent.When(hasTarget)))
}
