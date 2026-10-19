package engine

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/Impre-visible/mihari/internal/devmode"
	"github.com/Impre-visible/mihari/internal/profiles"
	"github.com/Impre-visible/mihari/internal/settings"
	"github.com/Impre-visible/mihari/internal/version"
)

// OpenCode's APIs, without the /v1 suffix the API clients add: Go is the
// monthly subscription, Zen the pay-as-you-go credits.
const (
	OpenCodeGoURL  = "https://opencode.ai/inference/go"
	OpenCodeZenURL = "https://opencode.ai/zen"

	DefaultRemoteURL = OpenCodeGoURL
)

// OpenCode Go turns away clients that do not name themselves, or send no
// conversation id; jobs send theirs in this header.
const sessionHeader = "X-Opencode-Session"

var userAgent = "mihari/" + version.Version

const (
	remoteURLField = "dev_inference_url"
	remoteKeyName  = "dev_inference_key"
)

// Remote is the OpenAI-compatible API the OpenCode engine sends requests to.
type Remote struct {
	URL    string `json:"url"`
	HasKey bool   `json:"hasKey"`
}

func remoteURL() string {
	value, _ := (&settings.Service{}).Get(remoteURLField)
	if raw, ok := value.(string); ok && strings.TrimSpace(raw) != "" {
		return strings.TrimRight(strings.TrimSpace(raw), "/")
	}
	return DefaultRemoteURL
}

func remoteKey() string {
	key, _ := settings.Secret(remoteKeyName)
	return key
}

// RemoteSettings returns the remote API the OpenCode engine uses.
func (s *Service) RemoteSettings() (Remote, error) {
	if !devmode.Enabled {
		return Remote{}, devmode.ErrDisabled
	}
	return Remote{URL: remoteURL(), HasKey: remoteKey() != ""}, nil
}

// SetRemote saves the remote API's URL, "" meaning OpenCode Go. An empty key
// keeps the stored one.
func (s *Service) SetRemote(rawURL, key string) error {
	if !devmode.Enabled {
		return devmode.ErrDisabled
	}
	rawURL = strings.TrimRight(strings.TrimSpace(rawURL), "/")
	if rawURL != "" {
		u, err := url.Parse(rawURL)
		if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
			return fmt.Errorf("URL invalide : %q", rawURL)
		}
	}
	if err := (&settings.Service{}).Set(remoteURLField, rawURL); err != nil {
		return err
	}
	if key = strings.TrimSpace(key); key != "" {
		return settings.SetSecret(remoteKeyName, key)
	}
	return nil
}

// ForgetRemoteKey removes the stored API key.
func (s *Service) ForgetRemoteKey() error {
	if !devmode.Enabled {
		return devmode.ErrDisabled
	}
	return settings.SetSecret(remoteKeyName, "")
}

// RemoteTest is what a test request to the remote API got back.
type RemoteTest struct {
	Text       string `json:"text"`
	DurationMs int64  `json:"durationMs"`
}

// TestRemote sends a short message to model on the remote API. An empty URL
// or key falls back to the saved one, so the form can be tested before saving.
func (s *Service) TestRemote(rawURL, key, model string) (RemoteTest, error) {
	if !devmode.Enabled {
		return RemoteTest{}, devmode.ErrDisabled
	}
	base := strings.TrimRight(strings.TrimSpace(rawURL), "/")
	if base == "" {
		base = remoteURL()
	}
	if key = strings.TrimSpace(key); key == "" {
		key = remoteKey()
	}
	model = strings.TrimSpace(model)
	switch {
	case key == "":
		return RemoteTest{}, errors.New("aucune clé API renseignée")
	case model == "":
		return RemoteTest{}, errors.New("aucun modèle renseigné")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	if err := checkRemoteModel(ctx, base, key, model); err != nil {
		return RemoteTest{}, err
	}
	begin := time.Now()
	text, err := remoteChat(ctx, base, key, model, "Réponds seulement OK.")
	if err != nil {
		return RemoteTest{}, err
	}
	return RemoteTest{Text: text, DurationMs: time.Since(begin).Milliseconds()}, nil
}

// remoteChat sends one user message, since listing the models proves neither
// the key nor the model's endpoint: Zen lists every model to anyone.
func remoteChat(ctx context.Context, base, key, model, message string) (string, error) {
	body, err := json.Marshal(map[string]any{
		"model":    model,
		"messages": []map[string]string{{"role": "user", "content": message}},
	})
	if err != nil {
		return "", err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, base+"/v1/chat/completions", bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+key)
	req.Header.Set("User-Agent", userAgent)
	req.Header.Set(sessionHeader, rand.Text())
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("API distante injoignable : %w", err)
	}
	defer res.Body.Close()
	var out struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
		Error struct {
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.NewDecoder(res.Body).Decode(&out); err != nil {
		return "", fmt.Errorf("réponse illisible de l'API distante (%s) : %w", res.Status, err)
	}
	if res.StatusCode != http.StatusOK || len(out.Choices) == 0 {
		return "", fmt.Errorf("l'API distante a répondu %s : %s", res.Status, out.Error.Message)
	}
	return out.Choices[0].Message.Content, nil
}

// startRemote serves the remote API on 127.0.0.1 behind a proxy that adds the
// API key, so jobs talk to it like to a local engine and never see the key.
func startRemote(ctx context.Context, p profiles.Profile) (*Server, error) {
	if !devmode.Enabled {
		return nil, devmode.ErrDisabled
	}
	key := remoteKey()
	if key == "" {
		return nil, errors.New("aucune clé API : renseigne-la dans la page Dev")
	}
	return serveRemote(ctx, remoteURL(), key, strings.TrimSpace(p.Model))
}

func serveRemote(ctx context.Context, base, key, model string) (*Server, error) {
	upstream, err := url.Parse(base)
	if err != nil {
		return nil, err
	}
	if err := checkRemoteModel(ctx, base, key, model); err != nil {
		return nil, err
	}

	fallbackSession := rand.Text()
	proxy := &httputil.ReverseProxy{
		Rewrite: func(r *httputil.ProxyRequest) {
			r.SetURL(upstream)
			r.Out.Header.Set("Authorization", "Bearer "+key)
			r.Out.Header.Set("User-Agent", userAgent)
			if r.Out.Header.Get(sessionHeader) == "" {
				// The Dev page's test chat has no job to take an id from.
				r.Out.Header.Set(sessionHeader, fallbackSession)
			}
		},
	}
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return nil, err
	}
	srv := &http.Server{Handler: proxy}
	done := make(chan struct{})
	logs := &tail{}
	go func() {
		if err := srv.Serve(l); err != nil && !errors.Is(err, http.ErrServerClosed) {
			fmt.Fprintln(logs, err)
		}
	}()

	var once sync.Once
	return &Server{
		URL:   "http://" + l.Addr().String(),
		Model: model,
		done:  done,
		logs:  logs,
		cleanup: func() {
			once.Do(func() {
				_ = srv.Close()
				close(done)
			})
		},
	}, nil
}

// checkRemoteModel makes sure the API answers and serves model, so a typo
// fails at start rather than on the first job.
func checkRemoteModel(ctx context.Context, base, key, model string) error {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, base+"/v1/models", nil)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+key)
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("API distante injoignable : %w", err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return fmt.Errorf("l'API distante a répondu %s", res.Status)
	}
	var list struct {
		Data []struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	if err := json.NewDecoder(res.Body).Decode(&list); err != nil {
		return fmt.Errorf("liste des modèles illisible : %w", err)
	}
	ids := make([]string, len(list.Data))
	for i, m := range list.Data {
		if m.ID == model {
			return nil
		}
		ids[i] = m.ID
	}
	return fmt.Errorf("modèle %q inconnu de l'API distante, disponibles : %s", model, strings.Join(ids, ", "))
}
