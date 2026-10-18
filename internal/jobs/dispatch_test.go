package jobs

import (
	"context"
	"testing"
	"time"

	"github.com/Impre-visible/mihari/internal/agent"
	"github.com/Impre-visible/mihari/internal/gitlab"
	"github.com/Impre-visible/mihari/internal/profiles"
	"github.com/Impre-visible/mihari/internal/scheduler"
)

type fakeEngine struct{ active scheduler.Active }

func (f *fakeEngine) Active() scheduler.Active { return f.active }
func (f *fakeEngine) Nudge()                   {}

func newTestService(t *testing.T, trigger profiles.Trigger, maxJobs int) (*Service, *fakeEngine, *[]int) {
	t.Setenv("XDG_CACHE_HOME", t.TempDir())
	t.Setenv("HOME", t.TempDir())
	engine := &fakeEngine{active: scheduler.Active{Ready: true, Profile: profiles.Profile{Name: "p", Trigger: trigger, MaxJobs: maxJobs}}}
	var started []int
	s := &Service{Scheduler: engine, known: map[int]bool{}, wake: make(chan struct{}, 1), ctx: context.Background()}
	s.start = func(_ context.Context, j *job, _ scheduler.Active) {
		s.mu.Lock()
		started = append(started, j.ID)
		s.mu.Unlock()
	}
	return s, engine, &started
}

func ping(id, project, iid int) gitlab.Todo {
	return gitlab.Todo{ID: id, TargetType: gitlab.MergeRequestTarget, ProjectID: project, IID: iid}
}

// waitStarted returns the started job ids in order, whatever order their goroutines ran in.
func waitStarted(s *Service, started *[]int, n int) []int {
	for range 100 {
		s.mu.Lock()
		done := len(*started) >= n
		s.mu.Unlock()
		if done {
			break
		}
		time.Sleep(5 * time.Millisecond)
	}
	var ids []int
	for _, j := range s.List() {
		if j.State == StateRunning {
			ids = append(ids, j.ID)
		}
	}
	return ids
}

func TestDispatchPings(t *testing.T) {
	s, _, started := newTestService(t, profiles.TriggerPing, 2)
	// Two pings on !1, one on !2, one on project 2's !1.
	s.Offer([]gitlab.Todo{ping(10, 1, 1), ping(11, 1, 1), ping(12, 1, 2), ping(13, 2, 1)})
	s.dispatch()
	if got := waitStarted(s, started, 2); len(got) != 2 || got[0] != 1 || got[1] != 3 {
		t.Fatalf("started %v, want jobs 1 and 3: one per merge request, two at most", got)
	}
	// The same todo offered again is not queued twice.
	s.Offer([]gitlab.Todo{ping(10, 1, 1)})
	if n := len(s.List()); n != 4 {
		t.Fatalf("%d jobs, want 4", n)
	}
}

func TestDispatchWaitsForEngine(t *testing.T) {
	s, engine, started := newTestService(t, profiles.TriggerPing, 4)
	engine.active.Draining = true
	s.Offer([]gitlab.Todo{ping(10, 1, 1)})
	s.dispatch()
	engine.active.Draining, engine.active.Ready = false, false
	s.dispatch()
	if len(*started) != 0 {
		t.Fatal("a job started while the engine was draining or not ready")
	}
}

func TestDispatchIssues(t *testing.T) {
	s, engine, started := newTestService(t, profiles.TriggerIssues, 2)
	now := time.Now()
	add := func(project, iid int, labelledAgo time.Duration) {
		s.enqueue(&job{Summary: Summary{Kind: agent.KindIssues, TargetType: gitlab.IssueTarget, ProjectID: project, IID: iid},
			labelAt: now.Add(-labelledAgo)})
	}
	add(1, 5, time.Hour)     // job 1
	add(1, 6, 3*time.Hour)   // job 2: labelled first
	add(2, 7, time.Minute)   // job 3
	add(3, 8, 2*time.Minute) // job 4
	// A ping waits while an issues profile is active.
	s.Offer([]gitlab.Todo{ping(10, 4, 1)})

	s.dispatch()
	if got := waitStarted(s, started, 2); len(got) != 2 || got[0] != 2 || got[1] != 4 {
		t.Fatalf("started %v, want 2 (project 1's oldest label) then 4 (next oldest, another project)", got)
	}

	// Back to a ping profile: queued issues are dropped, the ping can go.
	engine.active.Profile.Trigger = profiles.TriggerPing
	s.dispatch()
	for _, j := range s.List() {
		if j.Kind == agent.KindIssues && j.State == StateQueued {
			t.Errorf("issue job %d still queued after the issues period", j.ID)
		}
	}
}
