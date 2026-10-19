package tools

import (
	"context"
	"fmt"
	"regexp"
	"slices"
	"strings"
	"time"

	"github.com/Impre-visible/mihari/internal/agent"
)

type globArgs struct {
	Pattern string `json:"pattern" desc:"Motif de chemin : *.go, src/**/*.ts, **/{Dockerfile,*.yml}. Sans /, il s'applique au nom de fichier à toute profondeur"`
	Path    string `json:"path,omitempty" desc:"Dossier où chercher, relatif à la racine du repo"`
}

const globLimit = 300

func init() {
	agent.Register(agent.Define("glob", "Trouve des fichiers par motif de chemin (hors .git et node_modules), triés.",
		func(ctx context.Context, env *agent.Env, a globArgs) (string, error) {
			match, err := globRegexp(a.Pattern)
			if err != nil {
				return "", err
			}
			cmd := fmt.Sprintf("cd %s && find . -type f -not -path './.git/*' -not -path '*/node_modules/*' | sed 's|^\\./||'", quote(inWorkspace(a.Path)))
			// Many paths can overflow one command's output: ask find for a
			// first cut by name when the pattern allows it.
			if base := a.Pattern[strings.LastIndex(a.Pattern, "/")+1:]; base != "" && !strings.ContainsAny(base, "{") && base != "**" {
				cmd = strings.Replace(cmd, "-type f", "-type f -name "+quote(base), 1)
			}
			out, err := output(ctx, env, cmd, nil, 60*time.Second)
			if err != nil {
				return "", err
			}
			var found []string
			for _, p := range strings.Split(out, "\n") {
				if p != "" && match.MatchString(p) {
					found = append(found, p)
				}
			}
			if len(found) == 0 {
				return "Aucun fichier ne correspond.", nil
			}
			slices.Sort(found)
			more := ""
			if len(found) > globLimit {
				more = fmt.Sprintf("\n… et %d autres : précise le motif.", len(found)-globLimit)
				found = found[:globLimit]
			}
			return strings.Join(found, "\n") + more, nil
		}))
}

// globRegexp turns a glob into a regexp on slash-separated paths: ** crosses
// directories, * and ? stay within one, {a,b} picks one of a or b.
func globRegexp(pattern string) (*regexp.Regexp, error) {
	pattern = strings.TrimPrefix(pattern, "./")
	if pattern == "" {
		return nil, fmt.Errorf("motif vide")
	}
	var b strings.Builder
	if !strings.Contains(pattern, "/") {
		b.WriteString("(.*/)?")
	}
	depth := 0
	for i := 0; i < len(pattern); i++ {
		c := pattern[i]
		switch {
		case strings.HasPrefix(pattern[i:], "**/"):
			b.WriteString("(.*/)?")
			i += 2
		case strings.HasPrefix(pattern[i:], "**"):
			b.WriteString(".*")
			i++
		case c == '*':
			b.WriteString("[^/]*")
		case c == '?':
			b.WriteString("[^/]")
		case c == '{':
			b.WriteString("(")
			depth++
		case c == '}' && depth > 0:
			b.WriteString(")")
			depth--
		case c == ',' && depth > 0:
			b.WriteString("|")
		default:
			b.WriteString(regexp.QuoteMeta(string(c)))
		}
	}
	if depth != 0 {
		return nil, fmt.Errorf("accolade non fermée dans %q", pattern)
	}
	return regexp.Compile("^" + b.String() + "$")
}
