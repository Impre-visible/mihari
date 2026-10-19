package tools

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/Impre-visible/mihari/internal/agent"
)

type executeCodeArgs struct {
	Code           string `json:"code" desc:"Le script complet ; affiche ce que tu veux récupérer sur la sortie standard"`
	Language       string `json:"language,omitempty" enum:"python,node,sh" desc:"python (par défaut), node ou sh"`
	TimeoutSeconds int    `json:"timeout_seconds,omitempty" desc:"Délai maximum (120 par défaut, 600 au plus)"`
}

var interpreters = map[string]struct{ program, ext string }{
	"python": {"python3", "py"},
	"node":   {"node", "js"},
	"sh":     {"sh", "sh"},
}

func init() {
	agent.Register(agent.Define("execute_code", "Exécute un script dans le container, à la racine du repo : pour enchaîner en un seul appel ce qui en demanderait beaucoup (parcourir des dizaines de fichiers, filtrer, compter, transformer des données). Le script ne peut pas appeler tes autres outils.",
		func(ctx context.Context, env *agent.Env, a executeCodeArgs) (string, error) {
			lang := a.Language
			if lang == "" {
				lang = "python"
			}
			in, ok := interpreters[lang]
			if !ok {
				return "", fmt.Errorf("langage inconnu %q : python, node ou sh", lang)
			}
			if a.Code == "" {
				return "", errors.New("script vide")
			}
			timeout := a.TimeoutSeconds
			if timeout <= 0 {
				timeout = 120
			}
			script := fmt.Sprintf("/tmp/mihari/code/%d.%s", time.Now().UnixNano(), in.ext)
			cmd := needs(in.program) + fmt.Sprintf("mkdir -p /tmp/mihari/code && cat > %[1]s && %[2]s %[1]s", script, in.program)
			res, err := env.Runner.Exec(ctx, cmd, []byte(a.Code), time.Duration(min(timeout, 600))*time.Second)
			if err != nil {
				return "", err
			}
			return format(res), nil
		}))
}
