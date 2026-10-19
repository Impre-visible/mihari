package tools

import (
	"context"
	"fmt"
	"strings"

	"github.com/Impre-visible/mihari/internal/agent"
	"github.com/Impre-visible/mihari/internal/gitlab"
)

type searchIssuesArgs struct {
	Query string `json:"query" desc:"Mots cherchés dans le titre et la description"`
	Type  string `json:"type,omitempty" enum:"issue,merge_request,all" desc:"issue, merge_request ou all (par défaut)"`
	State string `json:"state,omitempty" enum:"opened,closed,merged,all" desc:"opened, closed, merged ou all (par défaut)"`
}

func init() {
	agent.Register(agent.Define("search_issues", "Cherche dans les issues et les MR du projet : un bug déjà signalé, une discussion passée, un travail en cours sur le même sujet.",
		func(ctx context.Context, env *agent.Env, a searchIssuesArgs) (string, error) {
			state := a.State
			if state == "all" {
				state = ""
			}
			var b strings.Builder
			for _, kind := range []gitlab.TargetType{gitlab.IssueTarget, gitlab.MergeRequestTarget} {
				if (a.Type == "issue" && kind != gitlab.IssueTarget) || (a.Type == "merge_request" && kind != gitlab.MergeRequestTarget) {
					continue
				}
				// Issues are never merged.
				if kind == gitlab.IssueTarget && state == "merged" {
					continue
				}
				found, err := env.GitLab.Search(ctx, kind, a.Query, state, 15)
				if err != nil {
					return "", err
				}
				for _, f := range found {
					fmt.Fprintf(&b, "%s %s (%s, @%s)\n", gitlab.Target{Type: kind, IID: f.IID}.Ref(), f.Title, f.State, f.Author)
				}
			}
			if b.Len() == 0 {
				return "Aucun résultat.", nil
			}
			return b.String(), nil
		}, agent.When(hasTarget)))
}
