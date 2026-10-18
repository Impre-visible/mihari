package jobs

import (
	"strings"
	"testing"

	"github.com/Impre-visible/mihari/internal/agent"
	"github.com/Impre-visible/mihari/internal/gitlab"
	"github.com/Impre-visible/mihari/internal/profiles"
)

func TestParseCommand(t *testing.T) {
	cases := []struct {
		body, want string
		ok         bool
	}{
		{"@mihari-bot /stop", "stop", true},
		{"merci !\n\n@Mihari-Bot /RETRY s'il te plaît", "retry", true},
		{"@mihari-bot ajoute des tests pour /api/users", "", false},
		{"@someone /stop", "", false},
		{"@someone ping @mihari-bot /help", "help", true},
		{"@mihari-bot /deploy", "deploy", true},
	}
	for _, c := range cases {
		got, ok := parseCommand(c.body, "mihari-bot")
		if got != c.want || ok != c.ok {
			t.Errorf("parseCommand(%q) = %q, %v", c.body, got, ok)
		}
	}
}

func commandService(t *testing.T) (*Service, *[]string) {
	s, _, _ := newTestService(t, profiles.TriggerPing, 1)
	var answers []string
	s.bot = func(int) string { return "bot" }
	s.reply = func(_ gitlab.Todo, body string) { answers = append(answers, body) }
	return s, &answers
}

func mention(id, iid int, body string) gitlab.Todo {
	return gitlab.Todo{ID: id, TargetType: gitlab.MergeRequestTarget, ProjectID: 1, IID: iid, Author: "romeo", Body: body, Simulated: true}
}

func TestStopCommand(t *testing.T) {
	s, answers := commandService(t)
	// Queue two jobs on !1 and one on !2; the engine is not ready, so none starts.
	s.Scheduler.(*fakeEngine).active.Ready = false
	s.Offer([]gitlab.Todo{mention(-1, 1, "@bot review"), mention(-2, 1, "@bot tests"), mention(-3, 2, "@bot review")})
	s.Offer([]gitlab.Todo{mention(-4, 1, "@bot /stop")})

	if len(*answers) != 1 || !strings.Contains((*answers)[0], "2 jobs arrêtés") {
		t.Fatalf("answers %q", *answers)
	}
	for _, j := range s.List() {
		stopped := j.State == StateCanceled
		if (j.IID == 1) != stopped {
			t.Errorf("job %d on !%d: %s", j.ID, j.IID, j.State)
		}
	}
	// The command is not a job.
	if n := len(s.List()); n != 3 {
		t.Errorf("%d jobs, want 3", n)
	}
}

func TestRetryCommand(t *testing.T) {
	s, answers := commandService(t)
	s.Offer([]gitlab.Todo{mention(-1, 1, "/retry @bot /retry")})
	if !strings.Contains((*answers)[0], "rien à relancer") {
		t.Fatalf("retry with no history: %q", *answers)
	}

	s.enqueue(&job{Summary: Summary{Kind: agent.KindPing, TargetType: gitlab.MergeRequestTarget, ProjectID: 1, IID: 1, Request: "@bot ajoute des tests"}})
	s.jobs[len(s.jobs)-1].State = StateFailed
	s.Offer([]gitlab.Todo{mention(-2, 1, "@bot /retry")})
	list := s.List()
	if !strings.Contains((*answers)[1], "relance") || list[0].State != StateQueued || list[0].Request != "@bot ajoute des tests" || list[0].Author != "romeo" {
		t.Fatalf("answers %q, jobs %+v", *answers, list)
	}

	s.Offer([]gitlab.Todo{mention(-3, 1, "@bot /help")})
	if !strings.Contains((*answers)[2], "/stop") {
		t.Errorf("help: %q", (*answers)[2])
	}
}
