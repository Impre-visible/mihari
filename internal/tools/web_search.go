package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strings"

	"golang.org/x/net/html"

	"github.com/Impre-visible/mihari/internal/agent"
	"github.com/Impre-visible/mihari/internal/settings"
)

type webSearchArgs struct {
	Query      string `json:"query" desc:"Ce que tu cherches, comme dans un moteur de recherche"`
	MaxResults int    `json:"max_results,omitempty" desc:"Nombre de résultats (8 par défaut, 20 au plus)"`
}

type result struct {
	Title, URL, Snippet string
}

// SearXNGSetting is the settings key of the SearXNG instance web_search uses,
// DuckDuckGo when it is empty.
const SearXNGSetting = "searxng_url"

func init() {
	agent.Register(agent.Define("web_search", "Cherche sur le web et rend les premiers résultats (titre, adresse, extrait). Lis ensuite une page avec web_fetch.",
		func(ctx context.Context, env *agent.Env, a webSearchArgs) (string, error) {
			n := a.MaxResults
			if n <= 0 {
				n = 8
			}
			n = min(n, 20)
			var results []result
			var err error
			if instance := searxng(); instance != "" {
				results, err = searchSearXNG(ctx, env, instance, a.Query)
			} else {
				results, err = searchDuckDuckGo(ctx, env, a.Query)
			}
			if err != nil {
				return "", err
			}
			if len(results) == 0 {
				return "Aucun résultat.", nil
			}
			var b strings.Builder
			for i, r := range results[:min(n, len(results))] {
				fmt.Fprintf(&b, "%d. %s\n   %s\n", i+1, r.Title, r.URL)
				if r.Snippet != "" {
					fmt.Fprintf(&b, "   %s\n", r.Snippet)
				}
			}
			return b.String(), nil
		}))
}

func searxng() string {
	value, err := (&settings.Service{}).Get(SearXNGSetting)
	if err != nil {
		return ""
	}
	s, _ := value.(string)
	return strings.TrimRight(strings.TrimSpace(s), "/")
}

func searchSearXNG(ctx context.Context, env *agent.Env, instance, query string) ([]result, error) {
	p, err := fetch(ctx, env, instance+"/search?format=json&q="+url.QueryEscape(query))
	if err != nil {
		return nil, err
	}
	if p.Status != 200 {
		return nil, fmt.Errorf("SearXNG a répondu %d (le format json doit être activé dans son settings.yml)", p.Status)
	}
	return parseSearXNG(p.Body)
}

func parseSearXNG(body string) ([]result, error) {
	var out struct {
		Results []struct {
			Title   string `json:"title"`
			URL     string `json:"url"`
			Content string `json:"content"`
		} `json:"results"`
	}
	if err := json.Unmarshal([]byte(body), &out); err != nil {
		return nil, fmt.Errorf("réponse SearXNG illisible : %w", err)
	}
	results := make([]result, len(out.Results))
	for i, r := range out.Results {
		results[i] = result{Title: r.Title, URL: r.URL, Snippet: strings.Join(strings.Fields(r.Content), " ")}
	}
	return results, nil
}

func searchDuckDuckGo(ctx context.Context, env *agent.Env, query string) ([]result, error) {
	p, err := fetch(ctx, env, "https://html.duckduckgo.com/html/?q="+url.QueryEscape(query))
	if err != nil {
		return nil, err
	}
	if p.Status != 200 {
		return nil, fmt.Errorf("DuckDuckGo a répondu %d : réessaie plus tard, ou règle une instance SearXNG dans Mihari", p.Status)
	}
	return parseDuckDuckGo(p.Body)
}

// parseDuckDuckGo reads the results of DuckDuckGo's HTML page, whose links
// go through a redirect carrying the real address in uddg.
func parseDuckDuckGo(body string) ([]result, error) {
	root, err := html.Parse(strings.NewReader(body))
	if err != nil {
		return nil, err
	}
	var results []result
	// The result a snippet that follows belongs to; -1 after an ad.
	last := -1
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if hasClass(n, "result__a") {
			href := attr(n, "href")
			if u, err := url.Parse(href); err == nil && u.Query().Get("uddg") != "" {
				href = u.Query().Get("uddg")
			}
			last = -1
			// Ads point back to DuckDuckGo.
			if !strings.Contains(href, "duckduckgo.com/y.js") {
				results = append(results, result{Title: textOf(n), URL: href})
				last = len(results) - 1
			}
			return
		}
		if hasClass(n, "result__snippet") && last >= 0 {
			results[last].Snippet = textOf(n)
			last = -1
			return
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(root)
	return results, nil
}
