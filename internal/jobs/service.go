// Package jobs turns work into agent runs. A ping profile runs one job per
// mention or review request, one at a time per merge request or issue. An
// issues profile works through the issues carrying its label, one at a time
// per project. Either way, at most the profile's MaxJobs run at once.
package jobs

import (
	"context"
	"fmt"
	"log"
	"slices"
	"sync"
	"time"

	"github.com/Impre-visible/mihari/internal/agent"
	"github.com/Impre-visible/mihari/internal/gitlab"
	"github.com/Impre-visible/mihari/internal/profiles"
	"github.com/Impre-visible/mihari/internal/scheduler"
	"github.com/wailsapp/wails/v3/pkg/application"
)

const (
	// UpdatedEvent carries every job's Summary when any job changes state.
	UpdatedEvent = "jobs:updated"
	// TranscriptEvent carries one new transcript line of a job.
	TranscriptEvent = "job:event"
)

// Finished jobs kept in memory for the Status page; more are on disk.
const historySize = 50

// How often a running job looks for new comments, and an issues profile for new issues.
const pollEvery = 30 * time.Second

type State string

const (
	StateQueued    State = "queued"
	StateRunning   State = "running"
	StateSucceeded State = "succeeded"
	StateFailed    State = "failed"
	StateCanceled  State = "canceled"
)

type Summary struct {
	ID         int               `json:"id"`
	Kind       agent.Kind        `json:"kind"`
	State      State             `json:"state"`
	TargetType gitlab.TargetType `json:"targetType"`
	Project    string            `json:"project"`
	ProjectID  int               `json:"projectId"`
	IID        int               `json:"iid"`
	Title      string            `json:"title"`
	URL        string            `json:"url"`
	// Who asked: the author of the mention, or who put the label.
	Author    string    `json:"author"`
	Request   string    `json:"request"`
	Simulated bool      `json:"simulated"`
	Profile   string    `json:"profile"`
	QueuedAt  time.Time `json:"queuedAt"`
	StartedAt time.Time `json:"startedAt"`
	EndedAt   time.Time `json:"endedAt"`
	Turns     int       `json:"turns"`
	Error     string    `json:"error"`
	Result    string    `json:"result"`
	// The merge request the agent opened, if any.
	OpenedMR string `json:"openedMr"`
}

func (s Summary) target() gitlab.Target {
	return gitlab.Target{Type: s.TargetType, IID: s.IID}
}

func (s Summary) finished() bool {
	return s.State != StateQueued && s.State != StateRunning
}

type TranscriptLine struct {
	JobID int         `json:"jobId"`
	Event agent.Event `json:"event"`
}

type job struct {
	Summary
	// The mention or review request a ping job answers.
	todo gitlab.Todo
	// Every todo this job answers: its own, plus mentions made in its thread meanwhile.
	todoIDs []int
	// For an issues job: the profile's label, and when it was put on the issue.
	label   string
	labelAt time.Time

	events []agent.Event
	cancel context.CancelFunc
	// Thread of the starting comment and the notes already seen.
	threadID string
	seen     map[int]bool
	// Built once the repository is cloned, since .mihari.md comes from it.
	systemPrompt string
	taskPrompt   string
	// The 👀 award, removed when the job ends.
	awardID int
}

func (j *job) key() string {
	if j.Kind == agent.KindIssues {
		return fmt.Sprintf("%d", j.ProjectID)
	}
	return fmt.Sprintf("%d%s", j.ProjectID, j.target().Ref())
}

// Engine is what jobs need from the scheduler.
type Engine interface {
	Active() scheduler.Active
	// Re-checks the schedule, as when the last job of a drained profile ends.
	Nudge()
}

// Service is exposed to the frontend through Wails bindings.
type Service struct {
	// Set before startup.
	Scheduler Engine
	// Drops a simulated todo once its job ends.
	RemoveSimulated func(id int)
	// Called when a job ends, and when an issues period ends with what it did.
	OnFinished  func(Summary)
	OnPeriodEnd func(PeriodSummary)
	// Called with every job's Summary whenever one changes.
	OnChange func([]Summary)
	// Called when the agent opens a merge request, with its URL in Summary.OpenedMR.
	OnMROpened func(Summary)

	mu     sync.Mutex
	jobs   []*job
	nextID int
	// Todos already turned into a job or attached to one, never queued again.
	known map[int]bool
	wake  chan struct{}
	ctx   context.Context
	// Runs a started job; tests replace it.
	start func(ctx context.Context, j *job, active scheduler.Active)
	// Tests replace how commands find the bot's name and answer.
	bot   func(projectID int) string
	reply func(t gitlab.Todo, body string)
	// When the current issues period began; zero outside one.
	periodStart time.Time
}

// PeriodSummary is what an issues period did, for the morning notification.
type PeriodSummary struct {
	Profile   string
	Succeeded int
	Failed    int
	// Issues still running when the period ended; they finish on their own.
	Running int
	// Merge requests the period opened, to review.
	MRs []string
}

func (s *Service) ServiceStartup(ctx context.Context, _ application.ServiceOptions) error {
	s.known = map[int]bool{}
	s.wake = make(chan struct{}, 1)
	s.ctx = ctx
	s.restore()
	go s.loop(ctx)
	go s.watchIssues(ctx)
	return nil
}

// restore reloads the history; jobs cut short by the last shutdown are marked
// so. Their todos and mihari::doing labels bring them back on their own.
func (s *Service) restore() {
	for _, r := range loadAll() {
		if !r.Summary.finished() {
			r.Summary.State, r.Summary.Error = StateCanceled, "interrompu par l'arrêt de Mihari"
			if r.Summary.EndedAt.IsZero() {
				r.Summary.EndedAt = time.Now()
			}
			save(r)
		}
		s.nextID = max(s.nextID, r.Summary.ID)
		s.jobs = append(s.jobs, &job{Summary: r.Summary})
	}
	if len(s.jobs) > historySize {
		s.jobs = s.jobs[len(s.jobs)-historySize:]
	}
}

// Busy reports whether a job is running, which holds the engine in place.
func (s *Service) Busy() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return slices.ContainsFunc(s.jobs, func(j *job) bool { return j.State == StateRunning })
}

// List returns queued and running jobs first, then the most recent finished ones.
func (s *Service) List() []Summary {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.summaries()
}

func (s *Service) summaries() []Summary {
	out := []Summary{}
	for _, j := range s.jobs {
		if !j.finished() {
			out = append(out, j.Summary)
		}
	}
	for i := len(s.jobs) - 1; i >= 0; i-- {
		if j := s.jobs[i]; j.finished() {
			out = append(out, j.Summary)
		}
	}
	return out
}

// Transcript returns what a job has done so far.
func (s *Service) Transcript(id int) []agent.Event {
	s.mu.Lock()
	for _, j := range s.jobs {
		if j.ID == id && (j.events != nil || !j.finished()) {
			defer s.mu.Unlock()
			return append([]agent.Event{}, j.events...)
		}
	}
	s.mu.Unlock()
	// A job from an earlier run: its transcript is on disk only.
	if r, ok := loadRecord(id); ok {
		return r.Events
	}
	return []agent.Event{}
}

// Cancel stops a running job, or drops a queued one.
func (s *Service) Cancel(id int) {
	s.mu.Lock()
	var closeIDs []int
	for _, j := range s.jobs {
		if j.ID != id {
			continue
		}
		switch j.State {
		case StateRunning:
			j.cancel()
		case StateQueued:
			j.State, j.EndedAt, j.Error = StateCanceled, time.Now(), "annulé avant de démarrer"
			// A ping is closed on GitLab, or the next poll would bring it back.
			// A labelled issue keeps its label and comes back next scan.
			if j.Kind == agent.KindPing {
				closeIDs = append(closeIDs, j.todoIDs...)
			}
			go save(record{Summary: j.Summary})
		}
	}
	s.mu.Unlock()
	if len(closeIDs) > 0 {
		go s.closeTodos(closeIDs)
	}
	s.publish()
}

// Offer hands over the todos the latest poll found ready.
func (s *Service) Offer(todos []gitlab.Todo) {
	var fresh []gitlab.Todo
	s.mu.Lock()
	for _, t := range todos {
		if !s.known[t.ID] {
			fresh = append(fresh, t)
		}
	}
	s.mu.Unlock()

	for _, t := range fresh {
		s.mu.Lock()
		s.known[t.ID] = true
		s.mu.Unlock()
		if s.command(t) {
			continue
		}
		// A mention in the thread of a running job is part of that job: its
		// watcher passes the comment on, and the todo is closed with the job.
		if j := s.runningThreadFor(t); j != nil {
			s.mu.Lock()
			j.todoIDs = append(j.todoIDs, t.ID)
			s.known[t.ID] = true
			s.mu.Unlock()
			continue
		}
		s.enqueue(&job{
			Summary: Summary{
				Kind: agent.KindPing, TargetType: t.TargetType, Project: t.Project, ProjectID: t.ProjectID,
				IID: t.IID, Title: t.Title, URL: t.URL, Author: t.Author, Request: t.Body, Simulated: t.Simulated,
			},
			todo:    t,
			todoIDs: []int{t.ID},
		})
		s.mu.Lock()
		s.known[t.ID] = true
		s.mu.Unlock()
	}
	if len(fresh) > 0 {
		s.publish()
	}
	s.poke()
}

func (s *Service) enqueue(j *job) {
	s.mu.Lock()
	s.nextID++
	j.ID, j.State, j.QueuedAt = s.nextID, StateQueued, time.Now()
	s.jobs = append(s.jobs, j)
	s.mu.Unlock()
}

// runningThreadFor returns the running job whose thread holds the todo's comment.
func (s *Service) runningThreadFor(t gitlab.Todo) *job {
	if t.NoteID == 0 {
		return nil
	}
	s.mu.Lock()
	var candidate *job
	for _, j := range s.jobs {
		if j.State == StateRunning && j.Kind == agent.KindPing && j.ProjectID == t.ProjectID &&
			j.target() == t.Target() && j.threadID != "" {
			candidate = j
		}
	}
	if candidate == nil {
		s.mu.Unlock()
		return nil
	}
	known, threadID := candidate.seen[t.NoteID], candidate.threadID
	s.mu.Unlock()
	if known {
		return candidate
	}
	// The watcher may not have seen the comment yet.
	client, err := gitlab.NewClient(t.ProjectID)
	if err != nil {
		return nil
	}
	d, err := client.Discussion(s.ctx, t.Target(), threadID)
	if err != nil {
		return nil
	}
	if slices.ContainsFunc(d.Notes, func(n gitlab.Note) bool { return n.ID == t.NoteID }) {
		return candidate
	}
	return nil
}

func (s *Service) poke() {
	select {
	case s.wake <- struct{}{}:
	default:
	}
}

func (s *Service) loop(ctx context.Context) {
	// Also catches the engine becoming ready, which sends no event here.
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()
	for {
		s.dispatch()
		select {
		case <-ctx.Done():
			s.mu.Lock()
			for _, j := range s.jobs {
				if j.State == StateRunning {
					j.cancel()
				}
			}
			s.mu.Unlock()
			return
		case <-ticker.C:
		case <-s.wake:
		}
	}
}

// dispatch starts the queued jobs of the active profile's kind while the
// engine and MaxJobs allow it.
func (s *Service) dispatch() {
	active := s.Scheduler.Active()
	kind := agent.KindPing
	if active.Profile.Trigger == profiles.TriggerIssues {
		kind = agent.KindIssues
	}

	s.mu.Lock()
	summary, ended := s.trackPeriod(kind, active)
	// Outside an issues period, queued issues are dropped: their label stays
	// on GitLab and the next period's scan finds them again, as they are then.
	if kind != agent.KindIssues || !active.Ready {
		s.jobs = slices.DeleteFunc(s.jobs, func(j *job) bool {
			return j.Kind == agent.KindIssues && j.State == StateQueued
		})
	}
	if !active.Ready || active.Draining {
		s.mu.Unlock()
		if ended && s.OnPeriodEnd != nil {
			s.OnPeriodEnd(summary)
		}
		return
	}

	running := 0
	busy := map[string]bool{}
	var queued []*job
	for _, j := range s.jobs {
		switch {
		case j.State == StateRunning:
			running++
			busy[j.key()] = true
		case j.State == StateQueued && j.Kind == kind:
			queued = append(queued, j)
		}
	}
	// Issues go by when their label was put; pings by arrival, as queued.
	slices.SortStableFunc(queued, func(a, b *job) int { return a.labelAt.Compare(b.labelAt) })

	var start []*job
	for _, j := range queued {
		if running >= active.Profile.MaxJobs {
			break
		}
		if busy[j.key()] {
			continue
		}
		ctx, cancel := context.WithCancel(s.ctx)
		j.State, j.StartedAt, j.Profile, j.cancel = StateRunning, time.Now(), active.Profile.Name, cancel
		j.seen = map[int]bool{}
		j.events = []agent.Event{}
		busy[j.key()] = true
		running++
		start = append(start, j)
		run := s.start
		if run == nil {
			run = s.run
		}
		go run(ctx, j, active)
	}
	s.mu.Unlock()
	if ended && s.OnPeriodEnd != nil {
		s.OnPeriodEnd(summary)
	}
	for _, j := range start {
		s.persist(j)
	}
	if len(start) > 0 {
		s.publish()
	}
}

// trackPeriod notes when an issues period starts and, once another kind of
// profile takes over, sums up the issues it worked on. s.mu must be held.
func (s *Service) trackPeriod(kind agent.Kind, active scheduler.Active) (PeriodSummary, bool) {
	if kind == agent.KindIssues && active.Ready {
		if s.periodStart.IsZero() {
			s.periodStart = time.Now()
		}
		return PeriodSummary{}, false
	}
	if s.periodStart.IsZero() || kind == agent.KindIssues {
		return PeriodSummary{}, false
	}
	sum := PeriodSummary{}
	for _, j := range s.jobs {
		if j.Kind != agent.KindIssues || j.StartedAt.Before(s.periodStart) {
			continue
		}
		sum.Profile = j.Profile
		if j.OpenedMR != "" {
			sum.MRs = append(sum.MRs, j.OpenedMR)
		}
		switch j.State {
		case StateSucceeded:
			sum.Succeeded++
		case StateFailed:
			sum.Failed++
		case StateRunning:
			sum.Running++
		}
	}
	s.periodStart = time.Time{}
	return sum, sum.Succeeded+sum.Failed+sum.Running > 0
}

func (s *Service) emit(j *job, kind, title, text string) {
	s.record(j, agent.Event{At: time.Now(), Kind: kind, Title: title, Text: text})
}

func (s *Service) record(j *job, e agent.Event) {
	s.mu.Lock()
	j.events = append(j.events, e)
	s.mu.Unlock()
	if app := application.Get(); app != nil {
		app.Event.Emit(TranscriptEvent, TranscriptLine{JobID: j.ID, Event: e})
	}
}

func (s *Service) persist(j *job) {
	s.mu.Lock()
	r := record{Summary: j.Summary, Events: append([]agent.Event{}, j.events...)}
	s.mu.Unlock()
	save(r)
}

func (s *Service) publish() {
	s.mu.Lock()
	// Keep every unfinished job and the latest finished ones. A new slice:
	// filtering in place from the end would overwrite jobs not read yet.
	finished := 0
	var kept []*job
	for i := len(s.jobs) - 1; i >= 0; i-- {
		j := s.jobs[i]
		if j.finished() {
			finished++
			if finished > historySize {
				continue
			}
		}
		kept = append(kept, j)
	}
	slices.Reverse(kept)
	s.jobs = kept
	list := s.summaries()
	s.mu.Unlock()
	if app := application.Get(); app != nil {
		app.Event.Emit(UpdatedEvent, list)
	}
	if s.OnChange != nil {
		s.OnChange(list)
	}
}

// closeTodos marks GitLab todos done and drops simulated ones.
func (s *Service) closeTodos(ids []int) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	for _, id := range ids {
		if id < 0 {
			if s.RemoveSimulated != nil {
				s.RemoveSimulated(id)
			}
			continue
		}
		if err := gitlab.MarkTodoDone(ctx, id); err != nil {
			log.Printf("mark todo %d done: %v", id, err)
		}
	}
}
