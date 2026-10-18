package jobs

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/Impre-visible/mihari/internal/gitlab"
)

// The framing every job gets after the profile's own prompt: how the tools
// fit together, which the profile's author should not have to explain.
const rules = `## Comment tu travailles

- Le repo est cloné dans /workspace, sur la branche de la MR. Tes commandes tournent dans un container jetable, avec le réseau. Installe ce qui te manque (apt-get, npm, pip…).
- Lis le code avant de conclure. Pour une review, utilise get_diff puis read_file autour des changements.
- Une remarque sur une ligne précise va dans comment_line, une remarque par appel.
- Si on te demande de modifier le code, modifie les fichiers puis appelle push avec un message de commit clair. Ne touche qu'à ce qui est demandé. Ne réécris jamais l'historique.
- Réponds avec reply à la personne qui t'a sollicité : ce que tu as fait, ou ta question si la demande n'est pas claire.
- Écris tes commentaires et ta réponse dans la langue de la demande.
- Quand tout est fait, appelle finish.`

// readMihariMD returns the repository's own instructions for the bot, read on
// the branch the job works on.
func readMihariMD(repo string) string {
	data, err := os.ReadFile(filepath.Join(repo, ".mihari.md"))
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(data))
}

func systemPrompt(profilePrompt, projectInstructions, mihariMD string) string {
	parts := []string{strings.TrimSpace(profilePrompt)}
	if s := strings.TrimSpace(projectInstructions); s != "" {
		parts = append(parts, "## Consignes du projet\n\n"+s)
	}
	if mihariMD != "" {
		parts = append(parts, "## Consignes du repo (.mihari.md)\n\n"+mihariMD)
	}
	return strings.Join(append(parts, rules), "\n\n")
}

func taskPrompt(todo gitlab.Todo, mr gitlab.MergeRequest) string {
	var b strings.Builder
	fmt.Fprintf(&b, "Merge request !%d « %s » de @%s, %s → %s.\n\n", mr.IID, mr.Title, mr.Author, mr.SourceBranch, mr.TargetBranch)
	if body := strings.TrimSpace(todo.Body); body != "" && todo.Action != "review_requested" {
		fmt.Fprintf(&b, "@%s t'a écrit :\n\n%s\n", todo.Author, body)
	} else {
		fmt.Fprintf(&b, "@%s te demande une review de cette merge request.\n", todo.Author)
	}
	if mr.FromFork() {
		b.WriteString("\nLa branche est dans un fork : tu peux lire et commenter, pas pousser.\n")
	}
	return b.String()
}
