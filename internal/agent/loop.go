package agent

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"
)

type Config struct {
	URL   string
	Model string
	// Model calls and time before the job fails; 0 lifts the limit.
	MaxTurns int
	Timeout  time.Duration
	// Context window of one job, in tokens.
	ContextTokens int
}

// Event is one line of a job's transcript.
type Event struct {
	At time.Time `json:"at"`
	// system, task, assistant, reasoning, tool, result, message, info or error.
	Kind  string `json:"kind"`
	Title string `json:"title"`
	Text  string `json:"text"`
	// Set on the lines of a subagent, which the chat shows inside its call.
	Agent string `json:"agent,omitempty"`
}

// Outcome is how the loop ended.
type Outcome struct {
	Success bool
	Summary string
	Turns   int
	// Summed over every model call, as the engine counted them.
	PromptTokens     int
	CompletionTokens int
}

const trimmed = "[sortie retirée pour libérer du contexte]"

// How many answers in a row without a tool call before giving up on the model.
const maxIdleAnswers = 3

// Waits before trying the engine again after a passing failure; a variable for tests.
var retryAfter = []time.Duration{2 * time.Second, 5 * time.Second, 15 * time.Second, 30 * time.Second, time.Minute}

// Run lets the model work on task until it calls finish. Messages arriving on
// inbox, such as replies in the thread, are handed over after the next tool call.
func Run(ctx context.Context, cfg Config, env *Env, system, task string, inbox <-chan Event, emit func(Event)) (Outcome, error) {
	if cfg.Timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, cfg.Timeout)
		defer cancel()
	}
	if env.Inbox == nil {
		env.Inbox = inbox
	}
	// A subagent gets no subagent of its own.
	if !env.ReadOnly && env.Subagent == nil {
		env.Subagent = func(ctx context.Context, task string) (string, error) {
			return runSubagent(ctx, cfg, env, task, emit)
		}
	}
	say := func(kind, title, text string) {
		emit(Event{At: time.Now(), Kind: kind, Title: title, Text: text})
	}

	messages := []message{{Role: "system", Content: system}, {Role: "user", Content: task}}
	session := rand.Text()
	say("system", "Prompt", system)
	say("task", "Demande", task)

	// Characters per token, measured on the engine's own counts as the job goes.
	charsPerToken := 3.5
	idle := 0
	var promptTokens, completionTokens int
	// Every way out reports the tokens spent so far.
	out := func(o Outcome) Outcome {
		o.PromptTokens, o.CompletionTokens = promptTokens, completionTokens
		return o
	}
	for turn := 1; ; turn++ {
		if cfg.MaxTurns > 0 && turn > cfg.MaxTurns {
			return out(Outcome{Turns: turn - 1}), fmt.Errorf("limite de %d tours atteinte", cfg.MaxTurns)
		}
		messages = fitContext(messages, cfg.ContextTokens, charsPerToken)
		// Recomputed each turn: opening a merge request makes its tools available.
		tools := toolsFor(env)
		byName := map[string]Tool{}
		for _, t := range tools {
			byName[t.Name()] = t
		}

		res, err := chat(ctx, cfg.URL, cfg.Model, session, messages, tools)
		for attempt := 0; err != nil && errors.As(err, new(transientError)) && attempt < len(retryAfter); attempt++ {
			say("info", "Moteur", fmt.Sprintf("%v ; nouvel essai dans %s.", err, retryAfter[attempt]))
			select {
			case <-ctx.Done():
			case <-time.After(retryAfter[attempt]):
				res, err = chat(ctx, cfg.URL, cfg.Model, session, messages, tools)
				continue
			}
			break
		}
		if err != nil {
			if errors.Is(ctx.Err(), context.DeadlineExceeded) {
				return out(Outcome{Turns: turn}), fmt.Errorf("délai de %s dépassé", cfg.Timeout)
			}
			if ctx.Err() != nil {
				return out(Outcome{Turns: turn}), ctx.Err()
			}
			return out(Outcome{Turns: turn}), err
		}
		promptTokens += res.PromptTokens
		completionTokens += res.CompletionTokens
		if res.PromptTokens > 0 {
			charsPerToken = float64(size(messages)) / float64(res.PromptTokens)
		}
		if res.Reasoning != "" {
			say("reasoning", "Réflexion", res.Reasoning)
		}
		if strings.TrimSpace(res.Message.Content) != "" {
			say("assistant", "Modèle", res.Message.Content)
		}
		messages = append(messages, res.Message)

		if len(res.Message.ToolCalls) == 0 {
			idle++
			if idle >= maxIdleAnswers {
				return out(Outcome{Turns: turn}), errors.New("le modèle répond sans appeler d'outil")
			}
			messages = append(messages, message{Role: "user",
				Content: "Continue en appelant les outils. Quand tu as terminé, appelle finish."})
			continue
		}
		idle = 0

		for _, call := range res.Message.ToolCalls {
			name := call.Function.Name
			say("tool", name, call.Function.Arguments)
			var out string
			if tool, ok := byName[name]; ok {
				result, err := tool.Run(ctx, env, json.RawMessage(call.Function.Arguments))
				if err != nil {
					out = "Erreur : " + err.Error()
				} else {
					out = result
				}
			} else {
				out = fmt.Sprintf("Erreur : outil inconnu %q.", name)
			}
			say("result", name, out)
			messages = append(messages, message{Role: "tool", ToolCallID: call.ID, Content: out})
		}

		// Images ride in a message of their own: a tool result is only text.
		if len(env.Images) > 0 {
			messages = append(messages, message{Role: "user", Content: "Image(s) chargée(s) par read_image :", Images: env.Images})
			env.Images = nil
		}

		if env.Finished != nil {
			return out(Outcome{Success: env.Finished.Success, Summary: env.Finished.Summary, Turns: turn}), nil
		}

		// News from the thread rides along with the latest tool result.
		if news := drain(env.Inbox); len(news) > 0 {
			var b strings.Builder
			for _, n := range news {
				say(n.Kind, n.Title, n.Text)
				fmt.Fprintf(&b, "\n\n---\n%s :\n%s", n.Title, n.Text)
			}
			last := &messages[len(messages)-1]
			last.Content += b.String()
		}
	}
}

func drain(inbox <-chan Event) []Event {
	var out []Event
	for {
		select {
		case e, ok := <-inbox:
			if !ok {
				return out
			}
			out = append(out, e)
		default:
			return out
		}
	}
}

// What an image weighs in size, whatever its bytes: engines count it in
// tokens by its pixels, not its encoding.
const imageSize = 4000

func size(messages []message) int {
	n := 0
	for _, m := range messages {
		n += len(m.Content) + len(m.Images)*imageSize
		for _, c := range m.ToolCalls {
			n += len(c.Function.Arguments) + len(c.Function.Name)
		}
	}
	return n
}

// fitContext blanks the oldest tool results until the conversation fits in
// 85% of the window, keeping room for the answer. The prompt, the task and
// the latest exchanges stay whole.
func fitContext(messages []message, contextTokens int, charsPerToken float64) []message {
	if contextTokens <= 0 {
		return messages
	}
	budget := int(float64(contextTokens) * 0.85 * charsPerToken)
	keepFrom := max(2, len(messages)-4)
	for i := 2; i < keepFrom && size(messages) > budget; i++ {
		switch m := &messages[i]; {
		case m.Role == "tool" && m.Content != trimmed:
			m.Content = trimmed
		case len(m.Images) > 0:
			m.Images, m.Content = nil, m.Content+" [image retirée pour libérer du contexte]"
		}
	}
	return messages
}

// How many model calls a subagent gets before it must conclude.
const subagentTurns = 30

const subagentSystem = `Tu es un sous-agent de Mihari. Un autre agent t'a confié la tâche ci-dessous pour garder son propre contexte léger.

- Tu peux lire, chercher, lancer des commandes et consulter le web et GitLab, mais pas modifier de fichier ni poster sur GitLab.
- Va droit au but : quelques appels bien choisis valent mieux que tout parcourir.
- Termine par finish : success à true si tu as trouvé, et dans summary ta conclusion complète et autonome (chemins, lignes, extraits, commandes), car c'est tout ce que l'autre agent verra.`

// runSubagent runs task in a read-only loop of its own, with a fresh
// context, and returns its conclusion. Its lines go to the job's transcript
// marked as the subagent's.
func runSubagent(ctx context.Context, cfg Config, parent *Env, task string, emit func(Event)) (string, error) {
	child := &Env{
		Kind: parent.Kind, Runner: parent.Runner, GitLab: parent.GitLab, Target: parent.Target,
		MR: parent.MR, Issue: parent.Issue, ThreadID: parent.ThreadID, Requester: parent.Requester,
		ReviewedSHA: parent.ReviewedSHA, PastJobs: parent.PastJobs, Notes: parent.Notes, Vision: parent.Vision,
		ReadOnly: true,
	}
	sub := cfg
	sub.MaxTurns, sub.Timeout = subagentTurns, 0
	if cfg.MaxTurns > 0 {
		sub.MaxTurns = min(sub.MaxTurns, cfg.MaxTurns)
	}
	out, err := Run(ctx, sub, child, subagentSystem, task, nil, func(e Event) {
		e.Agent = "subagent"
		emit(e)
	})
	if err != nil {
		return "", fmt.Errorf("le sous-agent s'est arrêté : %w", err)
	}
	if !out.Success {
		return "Le sous-agent n'a pas abouti : " + out.Summary, nil
	}
	return out.Summary, nil
}
