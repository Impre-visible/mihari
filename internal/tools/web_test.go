package tools

import (
	"strings"
	"testing"

	"github.com/Impre-visible/mihari/internal/agent"
	"github.com/Impre-visible/mihari/internal/sandbox"
)

func TestHTMLText(t *testing.T) {
	doc := `<!doctype html><html><head><title>Doc</title><style>p{}</style></head><body>
<nav>Menu</nav><h1>Install</h1><p>Run <code>go get</code> then
<a href="/next">continue</a>.</p><ul><li>one</li><li>two</li></ul>
<pre>line 1
  line 2</pre><script>alert(1)</script></body></html>`
	got, err := htmlText(doc, "https://example.com/docs/")
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"Titre : Doc", "# Install", "Run `go get` then [continue](https://example.com/next).", "- one\n- two", "```\nline 1\n  line 2\n```"} {
		if !strings.Contains(got, want) {
			t.Errorf("missing %q in:\n%s", want, got)
		}
	}
	for _, unwanted := range []string{"Menu", "alert", "p{}"} {
		if strings.Contains(got, unwanted) {
			t.Errorf("kept %q in:\n%s", unwanted, got)
		}
	}
}

func TestParseDuckDuckGo(t *testing.T) {
	page := `<div class="result"><a class="result__a" href="//duckduckgo.com/l/?uddg=https%3A%2F%2Fduckduckgo.com%2Fy.js%3Fad">Ad</a><a class="result__snippet">ad text</a></div>
<div class="result"><a class="result__a" href="//duckduckgo.com/l/?uddg=https%3A%2F%2Fgo.dev%2Fdoc&rut=x">The <b>Go</b> docs</a>
<a class="result__snippet" href="#">Documentation for Go.</a></div>`
	results, err := parseDuckDuckGo(page)
	if err != nil || len(results) != 1 || results[0] != (result{"The Go docs", "https://go.dev/doc", "Documentation for Go."}) {
		t.Errorf("got %+v, %v", results, err)
	}
}

func TestParseSearXNG(t *testing.T) {
	results, err := parseSearXNG(`{"results":[{"title":"Go","url":"https://go.dev","content":"The  Go\nlanguage"}]}`)
	if err != nil || len(results) != 1 || results[0].Snippet != "The Go language" {
		t.Errorf("got %+v, %v", results, err)
	}
}

func TestWebFetch(t *testing.T) {
	runner := &fakeRunner{respond: func(cmd string) sandbox.Result {
		if strings.Contains(cmd, "curl") {
			return sandbox.Result{Stdout: "200 text/html; charset=utf-8"}
		}
		if strings.Contains(cmd, "tail -c") {
			return sandbox.Result{Stdout: "<p>Bonjour</p>"}
		}
		return sandbox.Result{}
	}}
	out, err := run(t, &agent.Env{Runner: runner}, "web_fetch", map[string]any{"url": "https://example.com"})
	if err != nil || out != "https://example.com (HTTP 200, text/html; charset=utf-8)\n\nBonjour" {
		t.Fatalf("got %q, %v", out, err)
	}
	if !strings.Contains(runner.commands[0], "command -v curl") || !strings.Contains(runner.commands[0], "Accept-Language") {
		t.Errorf("no hint when curl is missing: %q", runner.commands[0])
	}
	if _, err := run(t, &agent.Env{Runner: runner}, "web_fetch", map[string]any{"url": "file:///etc/passwd"}); err == nil {
		t.Error("only http and https")
	}
}
