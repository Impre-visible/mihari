package tools

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/Impre-visible/mihari/internal/agent"
)

type readImageArgs struct {
	Path string `json:"path,omitempty" desc:"Image du repo ou du container (capture, rendu de test…)"`
	URL  string `json:"url,omitempty" desc:"Ou l'adresse http(s) d'une image, par exemple une capture jointe à une issue"`
}

// Past this size, an image is more likely a mistake than something to look at.
const imageLimit = 5 << 20

func init() {
	agent.Register(agent.Define("read_image", "Charge une image pour que tu la voies : capture d'erreur jointe à une issue, rendu d'une page, graphique produit par un test. Tu la reçois juste après ce résultat.",
		func(ctx context.Context, env *agent.Env, a readImageArgs) (string, error) {
			var data string
			var err error
			switch {
			case a.Path != "" && a.URL != "":
				return "", errors.New("donne path ou url, pas les deux")
			case a.Path != "":
				data, err = readFile(ctx, env, inWorkspace(a.Path), imageLimit+1)
			case a.URL != "":
				var p page
				if p, err = fetch(ctx, env, a.URL); err == nil && p.Status >= 400 {
					err = fmt.Errorf("HTTP %d", p.Status)
				}
				data = p.Body
			default:
				return "", errors.New("donne path ou url")
			}
			if err != nil {
				return "", err
			}
			if len(data) > imageLimit {
				return "", fmt.Errorf("image trop lourde (plus de %d Mo)", imageLimit>>20)
			}
			mime := http.DetectContentType([]byte(data))
			if !strings.HasPrefix(mime, "image/") {
				return "", fmt.Errorf("ce n'est pas une image (%s)", mime)
			}
			env.Images = append(env.Images, "data:"+mime+";base64,"+base64.StdEncoding.EncodeToString([]byte(data)))
			return fmt.Sprintf("Image chargée (%s, %d Ko) : tu la vois dans le message suivant.", mime, len(data)>>10), nil
		}, agent.When(func(env *agent.Env) bool { return env.Vision })))
}
