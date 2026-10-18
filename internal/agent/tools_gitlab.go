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

type suggestArgs struct {
	Path      string `json:"path" desc:"Fichier, tel qu'il apparaît dans le diff"`
	StartLine int    `json:"start_line" desc:"Première ligne remplacée, dans la nouvelle version du fichier"`
	EndLine   int    `json:"end_line,omitempty" desc:"Dernière ligne remplacée (start_line si vide)"`
	Code      string `json:"code" desc:"Le code qui remplace ces lignes, tel quel, sans bloc Markdown"`
	Comment   string `json:"comment" desc:"Pourquoi ce changement, en une ou deux phrases"`
}

type replyThreadArgs struct {
	ThreadID string `json:"thread_id" desc:"Identifiant du fil, tel que read_comments l'affiche"`
	Body     string `json:"body" desc:"La réponse, en Markdown"`
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
					state := ""
					if d.Open {
						state = ", ouvert"
					}
					fmt.Fprintf(&b, "[fil %s%s]\n%s\n", d.ID, state, formatNotes(d.Notes))
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
				if env.keep(Post{Kind: "comment", Path: a.Path, Line: a.Line, Body: a.Body}) {
					return fmt.Sprintf("Commentaire posté sur %s:%d.", a.Path, a.Line), nil
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

		Define("suggest_change", "Propose un correctif que l'auteur applique en un clic sur GitLab : les lignes start_line à end_line sont remplacées par code. Les lignes doivent être dans le diff.",
			func(ctx context.Context, env *Env, a suggestArgs) (string, error) {
				body, line, err := suggestion(a)
				if err != nil {
					return "", err
				}
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
				if env.keep(Post{Kind: "suggestion", Path: a.Path, Line: line, Body: body}) {
					return fmt.Sprintf("Suggestion postée sur %s:%d.", a.Path, line), nil
				}
				// A suggestion only applies anchored to a line of the new file in the diff.
				if err := env.GitLab.CommentLine(ctx, env.MR, oldPath, newPath, line, 0, body); err != nil {
					return "", fmt.Errorf("la ligne %d de %s n'est pas modifiable par une suggestion (elle doit être dans le diff) : %w", line, a.Path, err)
				}
				return fmt.Sprintf("Suggestion postée sur %s:%d.", a.Path, line), nil
			}, When(reviewing)),

		Define("get_new_changes", "Ce qui a changé dans la MR depuis ta dernière review : les nouveaux commits et leur diff. Pour une review incrémentale, c'est tout ce que tu relis.",
			func(ctx context.Context, env *Env, _ NoArgs) (string, error) {
				commits, diffs, err := env.GitLab.Compare(ctx, env.ReviewedSHA, env.MR.HeadSHA)
				if err != nil {
					return "", err
				}
				if len(commits) == 0 {
					return "Aucun nouveau commit depuis ta dernière review.", nil
				}
				var b strings.Builder
				b.WriteString("Nouveaux commits :\n")
				for _, c := range commits {
					fmt.Fprintf(&b, "- %s %s (%s)\n", c.SHA[:min(8, len(c.SHA))], c.Title, c.Author)
				}
				for _, d := range diffs {
					fmt.Fprintf(&b, "\n=== %s\n%s\n", d.NewPath, d.Diff)
				}
				return b.String(), nil
			}, When(func(env *Env) bool { return reviewing(env) && env.ReviewedSHA != "" })),

		Define("list_commits", "Liste les commits de la MR, du plus ancien au plus récent.",
			func(ctx context.Context, env *Env, _ NoArgs) (string, error) {
				commits, err := env.GitLab.Commits(ctx, env.MR.IID)
				if err != nil {
					return "", err
				}
				var b strings.Builder
				for _, c := range commits {
					fmt.Fprintf(&b, "%s %s (%s, %s)\n", c.SHA[:min(8, len(c.SHA))], c.Title, c.Author, c.Created.Format("02/01 15:04"))
				}
				if b.Len() == 0 {
					return "Aucun commit.", nil
				}
				return b.String(), nil
			}, When(hasMR)),

		Define("get_pipeline", "État du dernier pipeline CI de la MR, et la fin du log des jobs en échec.",
			func(ctx context.Context, env *Env, _ NoArgs) (string, error) {
				p, ok, err := env.GitLab.LatestPipeline(ctx, env.MR.IID)
				if err != nil {
					return "", err
				}
				if !ok {
					return "Cette MR n'a pas de pipeline.", nil
				}
				var b strings.Builder
				fmt.Fprintf(&b, "Pipeline %d : %s (commit %s)\n", p.ID, p.Status, p.SHA[:min(8, len(p.SHA))])
				var failed []gitlab.PipelineJob
				for _, j := range p.Jobs {
					fmt.Fprintf(&b, "- %s / %s : %s\n", j.Stage, j.Name, j.Status)
					if j.Status == "failed" {
						failed = append(failed, j)
					}
				}
				// The end of a failing log is where the error is; three logs are plenty.
				for _, j := range failed[:min(3, len(failed))] {
					log, err := env.GitLab.JobLog(ctx, j.ID, 4000)
					if err != nil {
						fmt.Fprintf(&b, "\n=== %s : log illisible (%v)\n", j.Name, err)
						continue
					}
					fmt.Fprintf(&b, "\n=== fin du log de %s\n%s\n", j.Name, log)
				}
				return b.String(), nil
			}, When(hasMR)),

		Define("reply_to_thread", "Répond dans un fil précis, par exemple pour dire qu'une de tes remarques est corrigée. Ne résout jamais le fil : c'est à l'humain.",
			func(ctx context.Context, env *Env, a replyThreadArgs) (string, error) {
				if strings.TrimSpace(a.Body) == "" || strings.TrimSpace(a.ThreadID) == "" {
					return "", errors.New("thread_id et body sont obligatoires")
				}
				if !env.keep(Post{Kind: "reply", Body: a.Body}) {
					if _, err := env.GitLab.Reply(ctx, env.Target, a.ThreadID, a.Body); err != nil {
						return "", err
					}
				}
				return "Réponse postée dans le fil.", nil
			}, When(hasTarget)),

		Define("reply", "Répond à la personne qui t'a sollicité : ton résumé, ta réponse ou ta question. Appelle-le avant finish.",
			func(ctx context.Context, env *Env, a replyArgs) (string, error) {
				if strings.TrimSpace(a.Body) == "" {
					return "", errors.New("réponse vide")
				}
				var err error
				switch {
				case env.keep(Post{Kind: "reply", Body: a.Body}):
				case env.ThreadID != "":
					_, err = env.GitLab.Reply(ctx, env.Target, env.ThreadID, a.Body)
				default:
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
