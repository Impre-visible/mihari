package tools

import (
	"context"
	"fmt"

	"github.com/Impre-visible/mihari/internal/agent"
)

type commentLineArgs struct {
	Path string `json:"path" desc:"Fichier, tel qu'il apparaît dans le diff"`
	Line int    `json:"line" desc:"Numéro de ligne dans la nouvelle version du fichier (ou dans l'ancienne pour une ligne supprimée)"`
	Body string `json:"body" desc:"Le commentaire, en Markdown"`
}

func init() {
	agent.Register(agent.Define("comment_line", "Commente une ligne précise du diff, comme dans une review. Une remarque par appel.",
		func(ctx context.Context, env *agent.Env, a commentLineArgs) (string, error) {
			oldPath, newPath, err := diffPaths(ctx, env, a.Path)
			if err != nil {
				return "", err
			}
			post := agent.Post{Kind: "comment", Path: a.Path, Line: a.Line, Body: a.Body}
			if err := env.Again(post); err != nil {
				return "", err
			}
			if env.Keep(post) {
				env.Sent(post)
				return fmt.Sprintf("Commentaire posté sur %s:%d.", a.Path, a.Line), nil
			}
			thread, err := env.GitLab.CommentLine(ctx, env.MR, oldPath, newPath, a.Line, 0, a.Body)
			if err != nil {
				// Maybe a removed line, numbered in the old file.
				thread, err = env.GitLab.CommentLine(ctx, env.MR, oldPath, newPath, 0, a.Line, a.Body)
			}
			if err != nil {
				// Outside the diff: still worth saying, as a regular comment.
				body := fmt.Sprintf("**%s:%d**\n\n%s", a.Path, a.Line, a.Body)
				if _, err := env.GitLab.Comment(ctx, env.Target, body); err != nil {
					return "", err
				}
				env.Sent(post)
				return fmt.Sprintf("La ligne %d de %s n'est pas dans le diff : posté en commentaire normal.", a.Line, a.Path), nil
			}
			post.Thread = thread
			env.Sent(post)
			return fmt.Sprintf("Commentaire posté sur %s:%d (fil %s, pour reply_to_thread).", a.Path, a.Line, thread), nil
		}, agent.When(reviewing), agent.Writes()))
}
