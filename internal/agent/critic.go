package agent

import (
	"context"
	"encoding/json"
	"strings"
	"time"
)

const criticSystem = `Tu es le relecteur indépendant de Mihari. Un autre agent pense avoir terminé la demande ci-dessous. Vérifie son travail avec un regard neuf : c'est la dernière étape avant que la personne le voie.

- Tu peux lire, chercher, lancer des commandes (tests compris) et consulter GitLab, mais ni modifier de fichier ni poster.
- Pour une review de merge request : relis le diff toi-même avec get_diff et read_file. Cherche les problèmes importants que l'agent n'a pas signalés (bugs, régressions, failles, cas limites oubliés) et les remarques fausses parmi celles qu'il a postées. Ignore le style et les préférences.
- Pour du code écrit ou modifié : vérifie que la demande est entièrement couverte, lance les tests et vérifie qu'ils passent, et que rien d'étranger à la demande n'a été touché.
- Ne signale que ce dont tu es sûr, avec le fichier, la ligne et la raison. Ne redemande pas ce que l'agent a déjà posté.
- Termine par finish : success à true si le travail tient, sinon false avec dans summary la liste numérotée des problèmes concrets à reprendre.`

// critique has a read-only reviewer check the work before the job ends, and
// returns the problems it found, "" when the work holds or when the review
// could not be done: it never blocks the job.
func critique(ctx context.Context, cfg Config, env *Env, task, summary string, actions []string, emit func(Event)) string {
	done := "Aucune action qui change quelque chose (pas de commentaire posté, pas de code poussé)."
	if len(actions) > 0 {
		done = strings.Join(actions, "\n")
	}
	brief := "## Demande d'origine\n\n" + task + "\n\n## Ce que l'agent a fait\n\n" + done + "\n\n## Son bilan\n\n" + summary
	// Shown as a call of its own, which the reviewer's lines go inside.
	emit(Event{At: time.Now(), Kind: "tool", Title: "critic", Text: `{"summary":` + quote(summary) + `}`})
	verdict := func(text string) {
		emit(Event{At: time.Now(), Kind: "result", Title: "critic", Text: text})
	}
	system := criticSystem
	if rules := strings.TrimSpace(cfg.CriticRules); rules != "" {
		system += "\n\n" + rules
	}
	out, err := Run(ctx, subConfig(cfg), readOnly(env), system, brief, nil, func(e Event) {
		e.Agent = "critic"
		emit(e)
	})
	switch {
	case err != nil:
		verdict("Relecture interrompue (" + err.Error() + ") : le job se termine sans elle.")
		return ""
	case out.Success || strings.TrimSpace(out.Summary) == "":
		verdict("Rien à reprendre : " + strings.TrimSpace(out.Summary))
		return ""
	}
	verdict("À reprendre :\n\n" + strings.TrimSpace(out.Summary))
	return strings.TrimSpace(out.Summary)
}

func quote(s string) string {
	data, _ := json.Marshal(s)
	return string(data)
}
