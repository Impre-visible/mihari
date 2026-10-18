package agent

import (
	"context"
	"errors"
	"fmt"
	"path"
	"strconv"
	"strings"
	"time"

	"github.com/Impre-visible/mihari/internal/sandbox"
)

// quote makes s a single shell word.
func quote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}

// inWorkspace resolves p against /workspace; the container is the boundary,
// so absolute paths elsewhere are allowed.
func inWorkspace(p string) string {
	if p == "" {
		return sandbox.Workdir
	}
	if path.IsAbs(p) {
		return path.Clean(p)
	}
	return path.Join(sandbox.Workdir, p)
}

func format(res sandbox.Result) string {
	var b strings.Builder
	if res.Stdout != "" {
		b.WriteString(res.Stdout)
	}
	if res.Stderr != "" {
		if b.Len() > 0 && !strings.HasSuffix(b.String(), "\n") {
			b.WriteString("\n")
		}
		b.WriteString("[stderr]\n" + res.Stderr)
	}
	if b.Len() == 0 {
		b.WriteString("(aucune sortie)")
	}
	fmt.Fprintf(&b, "\n[code de sortie %d", res.ExitCode)
	if res.TimedOut {
		b.WriteString(", délai dépassé")
	}
	if res.Truncated {
		b.WriteString(", sortie tronquée")
	}
	b.WriteString("]")
	return b.String()
}

type readFileArgs struct {
	Path      string `json:"path" desc:"Chemin du fichier, relatif à la racine du repo"`
	StartLine int    `json:"start_line,omitempty" desc:"Première ligne à lire (1 par défaut)"`
	EndLine   int    `json:"end_line,omitempty" desc:"Dernière ligne à lire (fin du fichier par défaut)"`
}

type writeFileArgs struct {
	Path    string `json:"path" desc:"Chemin du fichier, relatif à la racine du repo"`
	Content string `json:"content" desc:"Contenu complet du fichier"`
}

type listFilesArgs struct {
	Path  string `json:"path,omitempty" desc:"Dossier à lister, relatif à la racine du repo"`
	Depth int    `json:"depth,omitempty" desc:"Profondeur maximale (2 par défaut)"`
}

type searchArgs struct {
	Pattern string `json:"pattern" desc:"Expression régulière (grep -E)"`
	Path    string `json:"path,omitempty" desc:"Dossier ou fichier où chercher"`
}

type runCommandArgs struct {
	Command        string `json:"command" desc:"Commande shell, lancée avec sh -c à la racine du repo"`
	TimeoutSeconds int    `json:"timeout_seconds,omitempty" desc:"Délai maximum (120 par défaut, 600 au plus)"`
}

func init() {
	Register(
		Define("read_file", "Lit un fichier du repo, avec les numéros de ligne.",
			func(ctx context.Context, env *Env, a readFileArgs) (string, error) {
				start := max(1, a.StartLine)
				// awk compares numbers, so a huge bound reads to the end.
				end := "1e18"
				if a.EndLine > 0 {
					end = strconv.Itoa(a.EndLine)
				}
				cmd := fmt.Sprintf("test -f %[1]s || { echo 'fichier introuvable' >&2; exit 1; }; awk 'NR>=%[2]d && NR<=%[3]s {printf \"%%6d  %%s\\n\", NR, $0}' %[1]s",
					quote(inWorkspace(a.Path)), start, end)
				res, err := env.Runner.Exec(ctx, cmd, nil, 30*time.Second)
				if err != nil {
					return "", err
				}
				if res.ExitCode != 0 {
					return "", errors.New(strings.TrimSpace(res.Stderr))
				}
				return format(res), nil
			}),

		Define("write_file", "Crée ou remplace un fichier du repo (les dossiers sont créés au besoin).",
			func(ctx context.Context, env *Env, a writeFileArgs) (string, error) {
				target := inWorkspace(a.Path)
				cmd := fmt.Sprintf("mkdir -p %s && cat > %s", quote(path.Dir(target)), quote(target))
				res, err := env.Runner.Exec(ctx, cmd, []byte(a.Content), 30*time.Second)
				if err != nil {
					return "", err
				}
				if res.ExitCode != 0 {
					return "", errors.New(strings.TrimSpace(res.Stderr))
				}
				return fmt.Sprintf("%s écrit (%d octets).", a.Path, len(a.Content)), nil
			}),

		Define("list_files", "Liste les fichiers et dossiers du repo, sans .git.",
			func(ctx context.Context, env *Env, a listFilesArgs) (string, error) {
				depth := a.Depth
				if depth <= 0 {
					depth = 2
				}
				cmd := fmt.Sprintf("cd %s && find . -maxdepth %d -not -path './.git' -not -path './.git/*' | sort | head -500",
					quote(inWorkspace(a.Path)), depth)
				res, err := env.Runner.Exec(ctx, cmd, nil, 30*time.Second)
				if err != nil {
					return "", err
				}
				return format(res), nil
			}),

		Define("search", "Cherche une expression dans les fichiers du repo (grep), avec fichier et ligne.",
			func(ctx context.Context, env *Env, a searchArgs) (string, error) {
				cmd := fmt.Sprintf("grep -rnE --exclude-dir=.git --exclude-dir=node_modules -- %s %s | head -200",
					quote(a.Pattern), quote(inWorkspace(a.Path)))
				res, err := env.Runner.Exec(ctx, cmd, nil, 60*time.Second)
				if err != nil {
					return "", err
				}
				if res.Stdout == "" && res.Stderr == "" {
					return "Aucun résultat.", nil
				}
				return format(res), nil
			}),

		Define("run_command", "Lance une commande shell dans le container du job (tests, build, installation de paquets…). Le réseau est disponible.",
			func(ctx context.Context, env *Env, a runCommandArgs) (string, error) {
				timeout := a.TimeoutSeconds
				if timeout <= 0 {
					timeout = 120
				}
				res, err := env.Runner.Exec(ctx, a.Command, nil, time.Duration(min(timeout, 600))*time.Second)
				if err != nil {
					return "", err
				}
				return format(res), nil
			}),
	)
}
