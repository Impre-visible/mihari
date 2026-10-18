package jobs

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/Impre-visible/mihari/internal/agent"
)

// The framing every job gets after the profile's own prompt: how the tools
// fit together, which the profile's author should not have to explain.
const rules = `## Comment tu travailles

- Le repo est cloné dans /workspace, sur la branche du job. Tes commandes tournent dans un container jetable, avec le réseau. Installe ce qui te manque (apt-get, npm, pip…).
- Lis le code avant de conclure.
- Tout ce qui touche à GitLab passe par tes outils (MR, diff, commentaires, pipeline, push, MR à ouvrir). N'appelle jamais l'API GitLab toi-même (curl, wget, scripts) : tu n'as pas d'accès, et tes outils font l'appel correctement.
- Si tu modifies le code, ne touche qu'à ce qui est demandé, et ne réécris jamais l'historique.
- Écris tes commentaires et tes réponses dans la langue de la demande.
- Si tu apprends quelque chose d'utile pour les prochains jobs sur ce projet (commande de test, convention, piège), garde-le avec remember.
- Quand tout est fait, appelle finish.`

// What each kind of job is for, and the tools that matter for it.
const (
	reviewRules = `## Sur une merge request

- Pour une review, utilise get_diff puis read_file autour des changements. Une remarque sur une ligne précise va dans comment_line, une remarque par appel.
- Quand tu as le correctif exact, utilise suggest_change plutôt que comment_line : l'auteur l'applique en un clic. N'écris jamais toi-même de bloc suggestion dans un commentaire.
- Si on te demande de modifier le code, modifie les fichiers puis appelle push avec un message de commit clair.
- Réponds avec reply : ce que tu as fait, ou ta question si la demande n'est pas claire.`

	issuePingRules = `## Sur une issue

- Réponds avec reply.
- Si on te demande du code, modifie les fichiers puis ouvre une MR avec open_merge_request : elle part en brouillon vers la branche par défaut.`

	issueRules = `## Ton travail sur cette issue

- Ton travail se termine par une merge request : modifie les fichiers, puis appelle open_merge_request avec un titre et une description clairs. Sans MR, l'issue est considérée en échec.
- Après l'ouverture, tu peux encore corriger avec push.
- read_comments te donne la discussion de l'issue.`
)

// readRepoFile returns a file at the repository's root, "" when absent.
func readRepoFile(repo, name string) string {
	data, err := os.ReadFile(filepath.Join(repo, name))
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(data))
}

// Instructions an agent finds in the repository, in the order they are given.
var repoInstructions = []struct{ file, title string }{
	{"AGENTS.md", "Consignes du repo pour les agents (AGENTS.md)"},
	{".mihari.md", "Consignes du repo pour Mihari (.mihari.md)"},
}

func systemPrompt(profilePrompt, projectInstructions, repo, memory string) string {
	parts := []string{strings.TrimSpace(profilePrompt)}
	if s := strings.TrimSpace(projectInstructions); s != "" {
		parts = append(parts, "## Consignes du projet\n\n"+s)
	}
	for _, f := range repoInstructions {
		if s := readRepoFile(repo, f.file); s != "" {
			parts = append(parts, "## "+f.title+"\n\n"+s)
		}
	}
	if memory != "" {
		parts = append(parts, "## Ce que tu as noté sur ce projet lors de jobs précédents\n\n"+memory)
	}
	return strings.Join(append(parts, rules), "\n\n")
}

func taskPrompt(j *job, env *agent.Env) string {
	var b strings.Builder
	switch {
	case j.Kind == agent.KindIssues:
		i := env.Issue
		fmt.Fprintf(&b, "%s\n\nIssue #%d « %s », ouverte par @%s. @%s t'a demandé de la traiter.\n\n%s\n",
			issueRules, i.IID, i.Title, i.Author, j.Author, strings.TrimSpace(i.Description))
	case env.Issue != nil:
		i := env.Issue
		fmt.Fprintf(&b, "%s\n\nIssue #%d « %s », ouverte par @%s.\n\n@%s t'a écrit :\n\n%s\n",
			issuePingRules, i.IID, i.Title, i.Author, j.Author, strings.TrimSpace(j.todo.Body))
	default:
		mr := env.MR
		fmt.Fprintf(&b, "%s\n\nMerge request !%d « %s » de @%s, %s → %s.\n\n", reviewRules, mr.IID, mr.Title, mr.Author, mr.SourceBranch, mr.TargetBranch)
		if body := strings.TrimSpace(j.todo.Body); body != "" && j.todo.Action != "review_requested" {
			fmt.Fprintf(&b, "@%s t'a écrit :\n\n%s\n", j.Author, body)
		} else {
			fmt.Fprintf(&b, "@%s te demande une review de cette merge request.\n", j.Author)
		}
		if mr.FromFork() {
			b.WriteString("\nLa branche est dans un fork : tu peux lire et commenter, pas pousser.\n")
		}
	}
	return b.String()
}
