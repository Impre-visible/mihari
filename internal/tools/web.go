package tools

import (
	"context"
	"fmt"
	"net/url"
	"strings"
	"time"

	"golang.org/x/net/html"

	"github.com/Impre-visible/mihari/internal/agent"
)

// Pages fetched through curl in the container, the job's network boundary.
const (
	webDir   = "/tmp/mihari/web"
	webLimit = 2 << 20
	agentUA  = "Mozilla/5.0 (X11; Linux x86_64) Mihari"
	// SearXNG's bot detection, among others, turns away a request without it.
	acceptLanguage = "Accept-Language: fr-FR,fr;q=0.9,en;q=0.8"
)

type page struct {
	Status      int
	ContentType string
	Body        string
}

// fetch downloads u in the container and reads it back whole.
func fetch(ctx context.Context, env *agent.Env, u string) (page, error) {
	p, file, err := download(ctx, env, u)
	if err != nil {
		return page{}, err
	}
	defer remove(ctx, env, file)
	if p.Body, err = readFile(ctx, env, file, webLimit); err != nil {
		return page{}, err
	}
	return p, nil
}

// fetchDocument is fetch for a page to read, where a PDF comes back as its
// text, extracted by pdftotext in the container.
func fetchDocument(ctx context.Context, env *agent.Env, u string) (page, error) {
	p, file, err := download(ctx, env, u)
	if err != nil {
		return page{}, err
	}
	defer remove(ctx, env, file, file+".txt")
	head, err := readFile(ctx, env, file, 5)
	if err != nil {
		return page{}, err
	}
	read := file
	if strings.Contains(p.ContentType, "pdf") || head == "%PDF-" {
		cmd := needsPackage("pdftotext", "poppler-utils") + fmt.Sprintf("pdftotext -layout -enc UTF-8 %s %s.txt", file, file)
		if _, err := output(ctx, env, cmd, nil, 60*time.Second); err != nil {
			return page{}, fmt.Errorf("PDF illisible : %w", err)
		}
		read, p.ContentType = file+".txt", p.ContentType+", converti en texte"
	}
	if p.Body, err = readFile(ctx, env, read, webLimit); err != nil {
		return page{}, err
	}
	return p, nil
}

// download saves u to a file of the container and returns its status and type.
func download(ctx context.Context, env *agent.Env, u string) (page, string, error) {
	parsed, err := url.Parse(u)
	if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.Host == "" {
		return page{}, "", fmt.Errorf("URL invalide : %q (http ou https)", u)
	}
	file := fmt.Sprintf("%s/%d", webDir, time.Now().UnixNano())
	cmd := needs("curl") + fmt.Sprintf("mkdir -p %s && curl -sSL --compressed --max-time 30 --max-filesize %d -A %s -H %s -o %s -w '%%{http_code} %%{content_type}' %s",
		webDir, webLimit, quote(agentUA), quote(acceptLanguage), file, quote(parsed.String()))
	out, err := output(ctx, env, cmd, nil, 45*time.Second)
	if err != nil {
		return page{}, "", err
	}
	var p page
	status, contentType, _ := strings.Cut(strings.TrimSpace(out), " ")
	fmt.Sscan(status, &p.Status)
	p.ContentType = contentType
	return p, file, nil
}

func remove(ctx context.Context, env *agent.Env, files ...string) {
	_, _ = env.Runner.Exec(ctx, "rm -f "+strings.Join(files, " "), nil, 10*time.Second)
}

func isHTML(p page, body string) bool {
	if p.ContentType != "" {
		return strings.Contains(p.ContentType, "html")
	}
	head := strings.ToLower(body[:min(len(body), 512)])
	return strings.Contains(head, "<html") || strings.Contains(head, "<!doctype html")
}

// Elements whose text is never worth reading.
var skipped = map[string]bool{"script": true, "style": true, "noscript": true, "svg": true, "template": true, "iframe": true, "head": true, "nav": true, "footer": true}

var blocks = map[string]bool{"p": true, "div": true, "section": true, "article": true, "main": true, "header": true, "br": true,
	"ul": true, "ol": true, "table": true, "tr": true, "blockquote": true, "dl": true, "dt": true, "dd": true, "figure": true, "hr": true}

// htmlText turns a page into Markdown-like text: headings, lists, links and
// code survive, scripts, styles and menus go.
func htmlText(doc, base string) (string, error) {
	root, err := html.Parse(strings.NewReader(doc))
	if err != nil {
		return "", err
	}
	baseURL, _ := url.Parse(base)
	var b strings.Builder
	title := ""
	newline := func(n int) {
		s := b.String()
		trailing := len(s) - len(strings.TrimRight(s, "\n"))
		if b.Len() > 0 && trailing < n {
			b.WriteString(strings.Repeat("\n", n-trailing))
		}
	}
	var walk func(n *html.Node, pre bool)
	walk = func(n *html.Node, pre bool) {
		if n.Type == html.TextNode {
			if pre {
				b.WriteString(n.Data)
				return
			}
			text := strings.Join(strings.Fields(n.Data), " ")
			if text == "" {
				return
			}
			s := b.String()
			if strings.TrimSpace(n.Data) != n.Data && s != "" && !strings.HasSuffix(s, " ") && !strings.HasSuffix(s, "\n") {
				b.WriteString(" ")
			}
			b.WriteString(text)
			if strings.TrimRightFunc(n.Data, func(r rune) bool { return r == ' ' || r == '\n' || r == '\t' }) != n.Data {
				b.WriteString(" ")
			}
			return
		}
		if n.Type == html.ElementNode {
			tag := n.Data
			if tag == "title" && title == "" {
				title = textOf(n)
			}
			if tag == "head" {
				for c := n.FirstChild; c != nil; c = c.NextSibling {
					if c.Type == html.ElementNode && c.Data == "title" {
						title = textOf(c)
					}
				}
			}
			if skipped[tag] {
				return
			}
			switch tag {
			case "h1", "h2", "h3", "h4", "h5", "h6":
				newline(2)
				b.WriteString(strings.Repeat("#", int(tag[1]-'0')) + " ")
			case "li":
				newline(1)
				b.WriteString("- ")
			case "pre":
				newline(2)
				b.WriteString("```\n")
				for c := n.FirstChild; c != nil; c = c.NextSibling {
					walk(c, true)
				}
				newline(1)
				b.WriteString("```")
				newline(2)
				return
			case "code":
				if !pre {
					b.WriteString("`")
					defer b.WriteString("`")
				}
			case "a":
				href := attr(n, "href")
				if href != "" && !strings.HasPrefix(href, "#") && !strings.HasPrefix(href, "javascript:") {
					if u, err := url.Parse(href); err == nil && baseURL != nil {
						href = baseURL.ResolveReference(u).String()
					}
					b.WriteString("[")
					defer b.WriteString("](" + href + ")")
				}
			case "td", "th":
				b.WriteString(" | ")
			default:
				if blocks[tag] {
					newline(1)
					defer newline(1)
				}
			}
			if strings.HasPrefix(tag, "h") && len(tag) == 2 && tag[1] >= '1' && tag[1] <= '6' {
				defer newline(2)
			}
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c, pre)
		}
	}
	walk(root, false)
	text := b.String()
	// Collapse runs of blank lines left by nested blocks.
	for strings.Contains(text, "\n\n\n") {
		text = strings.ReplaceAll(text, "\n\n\n", "\n\n")
	}
	// Indentation only means something in code.
	lines := strings.Split(text, "\n")
	code := false
	for i, l := range lines {
		if strings.TrimSpace(l) == "```" {
			code = !code
		}
		if !code {
			lines[i] = strings.TrimSpace(l)
		}
	}
	text = strings.TrimSpace(strings.Join(lines, "\n"))
	if title != "" {
		text = "Titre : " + title + "\n\n" + text
	}
	return text, nil
}

func attr(n *html.Node, key string) string {
	for _, a := range n.Attr {
		if a.Key == key {
			return a.Val
		}
	}
	return ""
}

func hasClass(n *html.Node, class string) bool {
	return n.Type == html.ElementNode && strings.Contains(" "+attr(n, "class")+" ", " "+class+" ")
}

// textOf is the text inside n, spaces collapsed.
func textOf(n *html.Node) string {
	var b strings.Builder
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if n.Type == html.TextNode {
			b.WriteString(n.Data)
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(n)
	return strings.Join(strings.Fields(b.String()), " ")
}

// cut keeps the first limit characters of text, on a rune boundary.
func cut(text string, limit int) string {
	runes := []rune(text)
	if len(runes) <= limit {
		return text
	}
	return string(runes[:limit]) + fmt.Sprintf("\n\n… [%d caractères coupés : relance avec un max_chars plus grand si besoin]", len(runes)-limit)
}
