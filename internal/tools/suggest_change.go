package tools

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/Impre-visible/mihari/internal/agent"
)

type suggestArgs struct {
	Path      string `json:"path" desc:"Fichier, tel qu'il apparaît dans le diff"`
	StartLine int    `json:"start_line" desc:"Première ligne remplacée, dans la nouvelle version du fichier"`
	EndLine   int    `json:"end_line,omitempty" desc:"Dernière ligne remplacée (start_line si vide)"`
	Code      string `json:"code" desc:"Le code qui remplace ces lignes, tel quel, sans bloc Markdown"`
	Comment   string `json:"comment" desc:"Pourquoi ce changement, en une ou deux phrases"`
}

func init() {
	agent.Register(agent.Define("suggest_change", "Propose un correctif que l'auteur applique en un clic sur GitLab : les lignes start_line à end_line sont remplacées par code. Les lignes doivent être dans le diff.",
		func(ctx context.Context, env *agent.Env, a suggestArgs) (string, error) {
			body, line, err := suggestion(a)
			if err != nil {
				return "", err
			}
			oldPath, newPath, err := diffPaths(ctx, env, a.Path)
			if err != nil {
				return "", err
			}
			post := agent.Post{Kind: "suggestion", Path: a.Path, Line: line, Body: body}
			if err := env.Again(post); err != nil {
				return "", err
			}
			if env.Keep(post) {
				env.Sent(post)
				return fmt.Sprintf("Suggestion postée sur %s:%d.", a.Path, line), nil
			}
			// A suggestion only applies anchored to a line of the new file in the diff.
			thread, err := env.GitLab.CommentLine(ctx, env.MR, oldPath, newPath, line, 0, body)
			if err != nil {
				return "", fmt.Errorf("la ligne %d de %s n'est pas modifiable par une suggestion (elle doit être dans le diff) : %w", line, a.Path, err)
			}
			post.Thread = thread
			env.Sent(post)
			return fmt.Sprintf("Suggestion postée sur %s:%d (fil %s, pour reply_to_thread).", a.Path, line, thread), nil
		}, agent.When(reviewing), agent.Writes()))
}

// suggestion builds GitLab's suggestion block for a, anchored on its last
// line: "suggestion:-N+0" also replaces the N lines above it.
func suggestion(a suggestArgs) (string, int, error) {
	end := a.EndLine
	if end == 0 {
		end = a.StartLine
	}
	switch {
	case a.StartLine < 1:
		return "", 0, errors.New("start_line doit être un numéro de ligne")
	case end < a.StartLine:
		return "", 0, errors.New("end_line est avant start_line")
	case end-a.StartLine > 100:
		return "", 0, errors.New("suggestion trop longue : 100 lignes au plus")
	}
	code := strings.TrimSuffix(a.Code, "\n")
	// A fence inside the code would close the block early.
	fence := "```"
	for strings.Contains(code, fence) {
		fence += "`"
	}
	body := fmt.Sprintf("%s\n\n%ssuggestion:-%d+0\n%s\n%s", strings.TrimSpace(a.Comment), fence, end-a.StartLine, code, fence)
	return body, end, nil
}
