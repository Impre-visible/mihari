package jobs

import (
	"context"
	"fmt"
	"log"
	"regexp"
	"strings"
	"time"

	"github.com/Impre-visible/mihari/internal/agent"
	"github.com/Impre-visible/mihari/internal/gitlab"
)

// Commands are written right after the mention, with a slash: "@bot /stop".
// They act at once, without the model, whatever profile is active.
const helpText = `Ce que je comprends, juste après ma mention :

- **/stop** : arrête tous mes jobs sur cette MR ou cette issue, en cours et en attente.
- **/retry** : relance mon dernier job ici, avec la même demande.
- **/help** : cette aide.

Sans commande, écris ta demande normalement (« fais une review », « ajoute des tests pour le parser »…).`

var commandAfterMention = regexp.MustCompile(`(?i)@([\w.\-]+)\s+/([a-z]+)\b`)

// parseCommand returns the command given to bot in body, if any.
func parseCommand(body, bot string) (string, bool) {
	for _, m := range commandAfterMention.FindAllStringSubmatch(body, -1) {
		if strings.EqualFold(m[1], bot) {
			return strings.ToLower(m[2]), true
		}
	}
	return "", false
}

// command handles a todo carrying a command and reports whether it did.
func (s *Service) command(t gitlab.Todo) bool {
	bot := s.botName(t.ProjectID)
	cmd, ok := parseCommand(t.Body, bot)
	if !ok {
		return false
	}
	var answer string
	switch cmd {
	case "stop":
		answer = s.stopTarget(t)
	case "retry":
		answer = s.retry(t)
	case "help":
		answer = helpText
	default:
		answer = fmt.Sprintf("Je ne connais pas la commande **/%s**.\n\n%s", cmd, helpText)
	}
	s.answer(t, answer)
	s.closeTodos([]int{t.ID})
	return true
}

// stopTarget cancels every job on the todo's merge request or issue.
func (s *Service) stopTarget(t gitlab.Todo) string {
	s.mu.Lock()
	var ids []int
	for _, j := range s.jobs {
		if !j.finished() && j.ProjectID == t.ProjectID && j.target() == t.Target() {
			ids = append(ids, j.ID)
		}
	}
	s.mu.Unlock()
	for _, id := range ids {
		s.Cancel(id)
	}
	switch len(ids) {
	case 0:
		return "Aucun job ne tourne ici."
	case 1:
		return "⏹ Job arrêté."
	default:
		return fmt.Sprintf("⏹ %d jobs arrêtés.", len(ids))
	}
}

// retry queues the last job's request again, asked by whoever wrote /retry.
func (s *Service) retry(t gitlab.Todo) string {
	s.mu.Lock()
	var last *job
	for _, j := range s.jobs {
		if j.ProjectID == t.ProjectID && j.target() == t.Target() && (last == nil || j.ID > last.ID) {
			last = j
		}
	}
	s.mu.Unlock()
	if last == nil {
		return "Je n'ai encore rien fait ici, il n'y a rien à relancer."
	}
	if !last.finished() {
		return "Un job tourne déjà ici. Écris **/stop** d'abord pour le relancer."
	}

	request := last.Request
	if last.Kind == agent.KindIssues || strings.TrimSpace(request) == "" {
		if t.TargetType == gitlab.IssueTarget {
			request = "Traite cette issue et ouvre une MR avec open_merge_request."
		}
	}
	// The retry answers in the thread of the /retry comment.
	retried := t
	retried.Body = request
	if strings.TrimSpace(request) == "" {
		retried.Action = "review_requested"
	} else {
		retried.Action = "mentioned"
	}
	s.enqueue(&job{
		Summary: Summary{
			Kind: agent.KindPing, TargetType: t.TargetType, Project: t.Project, ProjectID: t.ProjectID,
			IID: t.IID, Title: t.Title, URL: t.URL, Author: t.Author, Request: request, Simulated: t.Simulated,
		},
		todo: retried,
		// The /retry todo is closed now; the new job answers no todo of its own.
		todoIDs: nil,
	})
	s.publish()
	s.poke()
	return "🔁 Je relance mon dernier job ici."
}

// botName is the account the bot answers to.
func (s *Service) botName(projectID int) string {
	if s.bot != nil {
		return s.bot(projectID)
	}
	client, err := gitlab.NewClient(projectID)
	if err != nil {
		return ""
	}
	return client.Bot
}

// answer replies in the thread of the command, or in a new comment.
func (s *Service) answer(t gitlab.Todo, body string) {
	if s.reply != nil {
		s.reply(t, body)
		return
	}
	if t.Simulated {
		log.Printf("simulated command on %s: %s", t.Target().Ref(), body)
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	client, err := gitlab.NewClient(t.ProjectID)
	if err != nil {
		return
	}
	if t.NoteID != 0 {
		if discussions, err := client.Discussions(ctx, t.Target()); err == nil {
			if d, ok := gitlab.DiscussionOf(discussions, t.NoteID); ok {
				if _, err := client.Reply(ctx, t.Target(), d.ID, body); err == nil {
					return
				}
			}
		}
	}
	if _, err := client.Comment(ctx, t.Target(), body); err != nil {
		log.Printf("answer on %s: %v", t.Target().Ref(), err)
	}
}
