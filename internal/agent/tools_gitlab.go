package agent

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/Impre-visible/mihari/internal/gitlab"
)

func hasMR(env *Env) bool     { return env.MR.IID != 0 }
func hasTarget(env *Env) bool { return env.Target.IID != 0 }
func hasIssue(env *Env) bool  { return env.Issue != nil }
func reviewing(env *Env) bool { return env.Target.Type == gitlab.MergeRequestTarget && hasMR(env) }
func canPush(env *Env) bool   { return env.Push != nil }
func canOpenMR(env *Env) bool { return env.OpenMR != nil && env.OpenedMR == nil }

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

type openMRArgs struct {
	Title       string `json:"title" desc:"Titre de la MR, court"`
	Description string `json:"description" desc:"Ce que tu as changé et pourquoi, en Markdown"`
}

func init() {
	Register(
		Define("get_merge_request", "Titre, description, auteur et branches de la MR.",
			func(ctx context.Context, env *Env, _ NoArgs) (string, error) {
				mr := env.MR
				return fmt.Sprintf("!%d %s\nAuteur : @%s\nBranche : %s → %s\nÉtat : %s\n\n%s",
					mr.IID, mr.Title, mr.Author, mr.SourceBranch, mr.TargetBranch, mr.State,
					strings.TrimSpace(mr.Description)), nil
			}, When(hasMR)),

		Define("get_issue", "Titre, description, auteur et labels de l'issue.",
			func(ctx context.Context, env *Env, _ NoArgs) (string, error) {
				i := env.Issue
				return fmt.Sprintf("#%d %s\nAuteur : @%s\nLabels : %s\nÉtat : %s\n\n%s",
					i.IID, i.Title, i.Author, strings.Join(i.Labels, ", "), i.State, strings.TrimSpace(i.Description)), nil
			}, When(hasIssue)),

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
			}, When(hasMR)),

		Define("read_thread", "Relit le fil de commentaires qui a lancé le job.",
			func(ctx context.Context, env *Env, _ NoArgs) (string, error) {
				if env.ThreadID == "" {
					return "Ce job n'a pas de fil de commentaires : il vient d'une demande de review ou d'un label.", nil
				}
				d, err := env.GitLab.Discussion(ctx, env.Target, env.ThreadID)
				if err != nil {
					return "", err
				}
				return formatNotes(d.Notes), nil
			}, When(hasTarget)),

		Define("read_comments", "Relit tous les commentaires, y compris les anciens échanges avec toi.",
			func(ctx context.Context, env *Env, _ NoArgs) (string, error) {
				discussions, err := env.GitLab.Discussions(ctx, env.Target)
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
			}, When(hasTarget)),

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
					if _, err := env.GitLab.Comment(ctx, env.Target, body); err != nil {
						return "", err
					}
					return fmt.Sprintf("La ligne %d de %s n'est pas dans le diff : posté en commentaire normal.", a.Line, a.Path), nil
				}
				return fmt.Sprintf("Commentaire posté sur %s:%d.", a.Path, a.Line), nil
			}, When(reviewing)),

		Define("reply", "Répond à la personne qui t'a sollicité : ton résumé, ta réponse ou ta question. Appelle-le avant finish.",
			func(ctx context.Context, env *Env, a replyArgs) (string, error) {
				if strings.TrimSpace(a.Body) == "" {
					return "", errors.New("réponse vide")
				}
				var err error
				if env.ThreadID != "" {
					_, err = env.GitLab.Reply(ctx, env.Target, env.ThreadID, a.Body)
				} else {
					_, err = env.GitLab.Comment(ctx, env.Target, a.Body)
				}
				if err != nil {
					return "", err
				}
				env.Replied = true
				return "Réponse postée.", nil
			}, When(hasTarget)),

		Define("push", "Commit toutes tes modifications du repo et les pousse sur la branche du job. Jamais de force-push : si la branche a bougé, ça échoue.",
			func(ctx context.Context, env *Env, a pushArgs) (string, error) {
				if strings.TrimSpace(a.Message) == "" {
					return "", errors.New("message de commit vide")
				}
				sha, err := env.Push(ctx, a.Message)
				if err != nil {
					return "", err
				}
				env.Pushes = append(env.Pushes, sha)
				return "Commit " + sha + " poussé.", nil
			}, When(canPush)),

		Define("open_merge_request", "Commit tes modifications, pousse la branche et ouvre une MR en brouillon vers la branche par défaut. Une seule fois par job ; ensuite, push met la MR à jour.",
			func(ctx context.Context, env *Env, a openMRArgs) (string, error) {
				if strings.TrimSpace(a.Title) == "" {
					return "", errors.New("titre vide")
				}
				mr, err := env.OpenMR(ctx, a.Title, a.Description)
				if err != nil {
					return "", err
				}
				env.OpenedMR, env.MR = &mr, mr
				return fmt.Sprintf("MR !%d ouverte en brouillon : %s", mr.IID, mr.WebURL), nil
			}, When(canOpenMR)),
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
