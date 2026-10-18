package agent

import (
	"context"
	"errors"
	"fmt"
	"strings"
)

func onMR(env *Env) bool { return env.MR.IID != 0 }

// Pushing goes to the merge request's own branch, which a fork keeps out of reach.
func canPush(env *Env) bool { return onMR(env) && env.Push != nil && !env.MR.FromFork() }

type getDiffArgs struct {
	Path string `json:"path,omitempty" desc:"Un seul fichier ; tous les fichiers si vide"`
}

type commentLineArgs struct {
	Path string `json:"path" desc:"Fichier, tel qu'il apparaît dans le diff"`
	Line int    `json:"line" desc:"Numéro de ligne dans la nouvelle version du fichier (ou dans l'ancienne pour une ligne supprimée)"`
	Body string `json:"body" desc:"Le commentaire, en Markdown"`
}

type replyArgs struct {
	Body string `json:"body" desc:"La réponse, en Markdown"`
}

type pushArgs struct {
	Message string `json:"message" desc:"Message de commit, première ligne courte"`
}

func init() {
	Register(
		Define("get_merge_request", "Titre, description, auteur et branches de la MR.",
			func(ctx context.Context, env *Env, _ NoArgs) (string, error) {
				mr := env.MR
				return fmt.Sprintf("!%d %s\nAuteur : @%s\nBranche : %s → %s\nÉtat : %s\n\n%s",
					mr.IID, mr.Title, mr.Author, mr.SourceBranch, mr.TargetBranch, mr.State,
					strings.TrimSpace(mr.Description)), nil
			}, When(onMR)),

		Define("get_diff", "Diff de la MR, fichier par fichier.",
			func(ctx context.Context, env *Env, a getDiffArgs) (string, error) {
				diffs, err := env.GitLab.Diffs(ctx, env.MR.IID)
				if err != nil {
					return "", err
				}
				var b strings.Builder
				for _, d := range diffs {
					if a.Path != "" && d.NewPath != a.Path && d.OldPath != a.Path {
						continue
					}
					header := d.NewPath
					switch {
					case d.New:
						header += " (nouveau)"
					case d.Deleted:
						header += " (supprimé)"
					case d.Renamed:
						header = d.OldPath + " → " + d.NewPath
					}
					fmt.Fprintf(&b, "=== %s\n%s\n", header, d.Diff)
				}
				if b.Len() == 0 {
					return "Aucun fichier ne correspond.", nil
				}
				return b.String(), nil
			}, When(onMR)),

		Define("read_thread", "Relit le fil de commentaires qui a lancé le job.",
			func(ctx context.Context, env *Env, _ NoArgs) (string, error) {
				if env.ThreadID == "" {
					return "Ce job vient d'une demande de review, sans fil de commentaires.", nil
				}
				d, err := env.GitLab.Discussion(ctx, env.MR.IID, env.ThreadID)
				if err != nil {
					return "", err
				}
				return formatNotes(d.Notes), nil
			}, When(onMR)),

		Define("read_comments", "Relit tous les commentaires de la MR, y compris les anciens échanges avec toi.",
			func(ctx context.Context, env *Env, _ NoArgs) (string, error) {
				discussions, err := env.GitLab.Discussions(ctx, env.MR.IID)
				if err != nil {
					return "", err
				}
				var b strings.Builder
				for _, d := range discussions {
					b.WriteString(formatNotes(d.Notes) + "\n")
				}
				if b.Len() == 0 {
					return "Aucun commentaire.", nil
				}
				return b.String(), nil
			}, When(onMR)),

		Define("comment_line", "Commente une ligne précise du diff, comme dans une review. Une remarque par appel.",
			func(ctx context.Context, env *Env, a commentLineArgs) (string, error) {
				diffs, err := env.GitLab.Diffs(ctx, env.MR.IID)
				if err != nil {
					return "", err
				}
				oldPath, newPath := a.Path, a.Path
				for _, d := range diffs {
					if d.NewPath == a.Path || d.OldPath == a.Path {
						oldPath, newPath = d.OldPath, d.NewPath
					}
				}
				err = env.GitLab.CommentLine(ctx, env.MR, oldPath, newPath, a.Line, 0, a.Body)
				if err != nil {
					// Maybe a removed line, numbered in the old file.
					err = env.GitLab.CommentLine(ctx, env.MR, oldPath, newPath, 0, a.Line, a.Body)
				}
				if err != nil {
					// Outside the diff: still worth saying, as a regular comment.
					body := fmt.Sprintf("**%s:%d**\n\n%s", a.Path, a.Line, a.Body)
					if _, err := env.GitLab.Comment(ctx, env.MR.IID, body); err != nil {
						return "", err
					}
					return fmt.Sprintf("La ligne %d de %s n'est pas dans le diff : posté en commentaire normal.", a.Line, a.Path), nil
				}
				return "Commentaire posté sur " + fmt.Sprintf("%s:%d.", a.Path, a.Line), nil
			}, When(onMR)),

		Define("reply", "Répond à la personne qui t'a sollicité : ton résumé, ta réponse ou ta question. Appelle-le avant finish.",
			func(ctx context.Context, env *Env, a replyArgs) (string, error) {
				if strings.TrimSpace(a.Body) == "" {
					return "", errors.New("réponse vide")
				}
				var err error
				if env.ThreadID != "" {
					_, err = env.GitLab.Reply(ctx, env.MR.IID, env.ThreadID, a.Body)
				} else {
					_, err = env.GitLab.Comment(ctx, env.MR.IID, a.Body)
				}
				if err != nil {
					return "", err
				}
				env.Replied = true
				return "Réponse postée.", nil
			}, When(onMR)),

		Define("push", "Commit toutes tes modifications du repo et les pousse sur la branche de la MR. Jamais de force-push : si la branche a bougé, ça échoue.",
			func(ctx context.Context, env *Env, a pushArgs) (string, error) {
				if strings.TrimSpace(a.Message) == "" {
					return "", errors.New("message de commit vide")
				}
				sha, err := env.Push(ctx, a.Message)
				if err != nil {
					return "", err
				}
				env.Pushes = append(env.Pushes, sha)
				return fmt.Sprintf("Commit %s poussé sur %s.", sha, env.MR.SourceBranch), nil
			}, When(canPush)),
	)
}

func formatNotes(notes []gitlabNote) string {
	var b strings.Builder
	for _, n := range notes {
		where := ""
		if n.Path != "" {
			where = fmt.Sprintf(" sur %s:%d", n.Path, n.Line)
		}
		fmt.Fprintf(&b, "@%s%s, %s :\n%s\n\n", n.Author, where, n.CreatedAt.Format("02/01 15:04"), strings.TrimSpace(n.Body))
	}
	return b.String()
}
