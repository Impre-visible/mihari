package tools

import (
	"context"
	"fmt"

	"github.com/Impre-visible/mihari/internal/agent"
)

type webFetchArgs struct {
	URL      string `json:"url" desc:"Adresse http(s) de la page"`
	MaxChars int    `json:"max_chars,omitempty" desc:"Longueur maximale du texte rendu (20000 par défaut, 100000 au plus)"`
}

func init() {
	agent.Register(agent.Define("web_fetch", "Télécharge une page web depuis le container (curl) et la rend en texte : documentation, changelog, issue d'une dépendance…",
		func(ctx context.Context, env *agent.Env, a webFetchArgs) (string, error) {
			p, err := fetch(ctx, env, a.URL)
			if err != nil {
				return "", err
			}
			text := p.Body
			if isHTML(p, p.Body) {
				if text, err = htmlText(p.Body, a.URL); err != nil {
					return "", err
				}
			}
			limit := a.MaxChars
			if limit <= 0 {
				limit = 20000
			}
			head := fmt.Sprintf("%s (HTTP %d, %s)\n\n", a.URL, p.Status, p.ContentType)
			if p.Status >= 400 {
				return "", fmt.Errorf("%s\n%s", head, cut(text, 2000))
			}
			return head + cut(text, min(limit, 100000)), nil
		}))
}
