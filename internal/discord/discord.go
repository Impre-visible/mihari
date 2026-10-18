// Package discord posts Mihari's important events to a Discord channel
// through a webhook.
package discord

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/Impre-visible/mihari/internal/jobs"
	"github.com/Impre-visible/mihari/internal/scheduler"
	"github.com/Impre-visible/mihari/internal/settings"
)

// The webhook URL lets anyone post in the channel, so it lives in the
// keyring with the GitLab token, not in the config file.
const secretName = "discord_webhook"

// Embed colours.
const (
	red    = 0xFF453A
	orange = 0xFF9F0A
	blue   = 0x0A84FF
	green  = 0x32D74B
)

type embed struct {
	Title       string    `json:"title"`
	Description string    `json:"description,omitempty"`
	URL         string    `json:"url,omitempty"`
	Color       int       `json:"color"`
	Timestamp   time.Time `json:"timestamp"`
}

type message struct {
	Username string  `json:"username"`
	Embeds   []embed `json:"embeds"`
	// Nobody is pinged, whatever the text contains.
	AllowedMentions struct {
		Parse []string `json:"parse"`
	} `json:"allowed_mentions"`
}

// Notifier turns Mihari's events into messages; main wires it to the jobs
// and the scheduler.
type Notifier struct {
	// Tests point it elsewhere.
	client *http.Client
}

// Service is exposed to the frontend through Wails bindings.
type Service struct {
	notifier *Notifier
}

// Service returns the frontend's view of n.
func (n *Notifier) Service() *Service {
	return &Service{notifier: n}
}

// Configured reports whether a webhook is saved, without revealing it.
func (s *Service) Configured() (bool, error) {
	u, err := settings.Secret(secretName)
	return u != "", err
}

// SetWebhook saves the webhook URL; "" turns Discord notifications off.
func (s *Service) SetWebhook(raw string) error {
	raw = strings.TrimSpace(raw)
	if raw != "" {
		if err := validate(raw); err != nil {
			return err
		}
	}
	return settings.SetSecret(secretName, raw)
}

func validate(raw string) error {
	u, err := url.Parse(raw)
	if err != nil || u.Scheme != "https" || !strings.HasPrefix(u.Path, "/api/webhooks/") ||
		(u.Host != "discord.com" && u.Host != "discordapp.com" && !strings.HasSuffix(u.Host, ".discord.com")) {
		return errors.New("ce n'est pas une URL de webhook Discord (https://discord.com/api/webhooks/…)")
	}
	return nil
}

// Test posts a message, with raw or, when it is empty, the saved webhook.
func (s *Service) Test(raw string) error {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		var err error
		if raw, err = settings.Secret(secretName); err != nil {
			return err
		}
		if raw == "" {
			return errors.New("aucun webhook renseigné")
		}
	} else if err := validate(raw); err != nil {
		return err
	}
	return s.notifier.post(raw, embed{Title: "Mihari est connecté", Description: "Les échecs, les erreurs du moteur, les MR ouvertes et le résumé de la nuit arriveront ici.", Color: blue})
}

// send posts to the saved webhook, quietly when there is none.
func (s *Notifier) send(e embed) {
	raw, err := settings.Secret(secretName)
	if err != nil || raw == "" {
		return
	}
	// Events come from job goroutines; Discord being slow must not hold them.
	go func() {
		if err := s.post(raw, e); err != nil {
			log.Printf("discord: %v", err)
		}
	}()
}

func (s *Notifier) post(webhook string, e embed) error {
	if e.Timestamp.IsZero() {
		e.Timestamp = time.Now()
	}
	// Discord's limits, kept with room to spare.
	e.Title = clip(e.Title, 250)
	e.Description = clip(e.Description, 3500)
	msg := message{Username: "Mihari", Embeds: []embed{e}}
	msg.AllowedMentions.Parse = []string{}
	body, err := json.Marshal(msg)
	if err != nil {
		return err
	}
	client := s.client
	if client == nil {
		client = &http.Client{Timeout: 15 * time.Second}
	}
	for attempt := 0; ; attempt++ {
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, webhook, bytes.NewReader(body))
		if err != nil {
			cancel()
			return err
		}
		req.Header.Set("Content-Type", "application/json")
		res, err := client.Do(req)
		cancel()
		if err != nil {
			return fmt.Errorf("Discord injoignable : %w", err)
		}
		res.Body.Close()
		switch {
		case res.StatusCode == http.StatusTooManyRequests && attempt == 0:
			// Discord says how long to wait; once is enough for a notification.
			wait, _ := strconv.ParseFloat(res.Header.Get("Retry-After"), 64)
			time.Sleep(time.Duration(min(max(wait, 1), 30) * float64(time.Second)))
			continue
		case res.StatusCode == http.StatusNotFound || res.StatusCode == http.StatusUnauthorized:
			return errors.New("webhook refusé par Discord : il a peut-être été supprimé")
		case res.StatusCode >= 300:
			return fmt.Errorf("Discord a répondu %s", res.Status)
		}
		return nil
	}
}

func clip(s string, n int) string {
	if r := []rune(s); len(r) > n {
		return string(r[:n-1]) + "…"
	}
	return s
}

func ref(j jobs.Summary) string {
	if j.TargetType == "Issue" {
		return j.Project + "#" + strconv.Itoa(j.IID)
	}
	return j.Project + "!" + strconv.Itoa(j.IID)
}

// JobFinished reports failed jobs.
func (s *Notifier) JobFinished(j jobs.Summary) {
	if j.State != jobs.StateFailed {
		return
	}
	s.send(embed{
		Title:       "Job en échec : " + ref(j),
		Description: fmt.Sprintf("**%s**\nDemandé par @%s\n\n%s", j.Title, j.Author, j.Error),
		URL:         j.URL,
		Color:       red,
	})
}

// EngineFailed reports an engine that will not run: nothing works until someone acts.
func (s *Notifier) EngineFailed(snap scheduler.Snapshot) {
	s.send(embed{
		Title:       "Moteur en erreur : " + snap.ProfileName,
		Description: fmt.Sprintf("%s (%s)\n\n%s", snap.Model, snap.Engine, snap.Error),
		Color:       orange,
	})
}

// MROpened reports a draft merge request the bot opened, to be reviewed.
func (s *Notifier) MROpened(j jobs.Summary) {
	s.send(embed{
		Title:       "MR ouverte pour " + ref(j),
		Description: fmt.Sprintf("**%s**\nDemandée par @%s, en brouillon, à relire.", j.Title, j.Author),
		URL:         j.OpenedMR,
		Color:       green,
	})
}

// PeriodEnded sums up an issues period.
func (s *Notifier) PeriodEnded(p jobs.PeriodSummary) {
	var b strings.Builder
	fmt.Fprintf(&b, "%d traitée(s), %d en échec", p.Succeeded, p.Failed)
	if p.Running > 0 {
		fmt.Fprintf(&b, ", %d encore en cours", p.Running)
	}
	if len(p.MRs) > 0 {
		b.WriteString("\n\nMR à relire :")
		for _, u := range p.MRs {
			b.WriteString("\n• " + u)
		}
	}
	s.send(embed{Title: "Fin de « " + p.Profile + " »", Description: b.String(), Color: blue})
}
