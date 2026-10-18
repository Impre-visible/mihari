// Package jobs turns ready todos into agent runs: one job per mention or
// review request, at most the active profile's MaxJobs at once, one at a time
// per merge request.
package jobs

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/Impre-visible/mihari/internal/agent"
	"github.com/Impre-visible/mihari/internal/gitlab"
	"github.com/Impre-visible/mihari/internal/profiles"
	"github.com/Impre-visible/mihari/internal/projects"
	"github.com/Impre-visible/mihari/internal/sandbox"
	"github.com/Impre-visible/mihari/internal/scheduler"
	"github.com/wailsapp/wails/v3/pkg/application"
)

const (
	// UpdatedEvent carries every job's Summary when any job changes state.
	UpdatedEvent = "jobs:updated"
	// TranscriptEvent carries one new transcript line of a job.
	TranscriptEvent = "job:event"
)

// Finished jobs kept for the Status page.
const historySize = 50

// How often a running job looks for new replies in its thread.
const threadPoll = 30 * time.Second

type State string

const (
	StateQueued    State = "queued"
	StateRunning   State = "running"
	StateSucceeded State = "succeeded"
	StateFailed    State = "failed"
	StateCanceled  State = "canceled"
)

type Summary struct {
	ID        int       `json:"id"`
	State     State     `json:"state"`
	Project   string    `json:"project"`
	MRIID     int       `json:"mrIid"`
	Title     string    `json:"title"`
	URL       string    `json:"url"`
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
}

type TranscriptLine struct {
	JobID int         `json:"jobId"`
	Event agent.Event `json:"event"`
}

type job struct {
	Summary
	todo gitlab.Todo
	// Every todo this job answers: its own, plus mentions made in its thread meanwhile.
	todoIDs []int
	events  []agent.Event
	cancel  context.CancelFunc
	// Thread of the starting comment and the notes already in it.
	threadID string
	seen     map[int]bool
	// Built once the repository is cloned, since .mihari.md comes from it.
	systemPrompt string
	taskPrompt   string
	// The 👀 award, removed when the job ends.
	awardID int
}

// Service is exposed to the frontend through Wails bindings.
type Service struct {
	// Set before startup.
	Scheduler *scheduler.Service
	// Drops a simulated todo once its job ends.
	RemoveSimulated func(id int)

	mu     sync.Mutex
	jobs   []*job
	nextID int
	// Todos already turned into a job or attached to one, never queued again.
	known map[int]bool
	wake  chan struct{}
	ctx   context.Context
}

func (s *Service) ServiceStartup(ctx context.Context, _ application.ServiceOptions) error {
	s.known = map[int]bool{}
	s.wake = make(chan struct{}, 1)
	s.ctx = ctx
	go s.loop(ctx)
	return nil
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
		if j.State == StateQueued || j.State == StateRunning {
			out = append(out, j.Summary)
		}
	}
	for i := len(s.jobs) - 1; i >= 0; i-- {
		if j := s.jobs[i]; j.State != StateQueued && j.State != StateRunning {
			out = append(out, j.Summary)
		}
	}
	return out
}

// Transcript returns what a job has done so far.
func (s *Service) Transcript(id int) []agent.Event {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, j := range s.jobs {
		if j.ID == id {
			return append([]agent.Event{}, j.events...)
		}
	}
	return []agent.Event{}
}

// Cancel stops a running job, or drops a queued one.
func (s *Service) Cancel(id int) {
	s.mu.Lock()
	for _, j := range s.jobs {
		if j.ID != id {
			continue
		}
		switch j.State {
		case StateRunning:
			j.cancel()
		case StateQueued:
			j.State, j.EndedAt, j.Error = StateCanceled, time.Now(), "annulé avant de démarrer"
			// Closed on GitLab too, or the next poll would bring it back.
			go s.closeTodos(append([]int{}, j.todoIDs...))
		}
	}
	s.mu.Unlock()
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
		// A mention in the thread of a running job is part of that job: its
		// watcher passes the comment on, and the todo is closed with the job.
		if j := s.runningThreadFor(t); j != nil {
			s.mu.Lock()
			j.todoIDs = append(j.todoIDs, t.ID)
			s.known[t.ID] = true
			s.mu.Unlock()
			continue
		}
		s.mu.Lock()
		s.nextID++
		s.jobs = append(s.jobs, &job{
			Summary: Summary{
				ID: s.nextID, State: StateQueued, Project: t.Project, MRIID: t.MRIID, Title: t.Title,
				URL: t.URL, Author: t.Author, Request: t.Body, Simulated: t.Simulated, QueuedAt: time.Now(),
			},
			todo:    t,
			todoIDs: []int{t.ID},
		})
		s.known[t.ID] = true
		s.mu.Unlock()
	}
	if len(fresh) > 0 {
		s.publish()
	}
	s.poke()
}

// runningThreadFor returns the running job whose thread holds the todo's comment.
func (s *Service) runningThreadFor(t gitlab.Todo) *job {
	if t.NoteID == 0 {
		return nil
	}
	s.mu.Lock()
	var candidate *job
	for _, j := range s.jobs {
		if j.State == StateRunning && j.todo.ProjectID == t.ProjectID && j.MRIID == t.MRIID && j.threadID != "" {
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
	d, err := client.Discussion(s.ctx, t.MRIID, threadID)
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

// dispatch starts queued jobs while the engine and the profile allow it.
func (s *Service) dispatch() {
	active := s.Scheduler.Active()
	if !active.Ready || active.Draining || active.Profile.Trigger != profiles.TriggerPing {
		return
	}
	s.mu.Lock()
	running := 0
	busyMRs := map[string]bool{}
	for _, j := range s.jobs {
		if j.State == StateRunning {
			running++
			busyMRs[mrKey(j.todo)] = true
		}
	}
	var start []*job
	for _, j := range s.jobs {
		if running >= active.Profile.MaxJobs {
			break
		}
		if j.State != StateQueued || busyMRs[mrKey(j.todo)] {
			continue
		}
		ctx, cancel := context.WithCancel(s.ctx)
		j.State, j.StartedAt, j.Profile, j.cancel = StateRunning, time.Now(), active.Profile.Name, cancel
		j.seen = map[int]bool{}
		busyMRs[mrKey(j.todo)] = true
		running++
		start = append(start, j)
		go s.run(ctx, j, active)
	}
	s.mu.Unlock()
	if len(start) > 0 {
		s.publish()
	}
}

func mrKey(t gitlab.Todo) string {
	return fmt.Sprintf("%d!%d", t.ProjectID, t.MRIID)
}

func (s *Service) emit(j *job, kind, title, text string) {
	e := agent.Event{At: time.Now(), Kind: kind, Title: title, Text: text}
	s.mu.Lock()
	j.events = append(j.events, e)
	s.mu.Unlock()
	if app := application.Get(); app != nil {
		app.Event.Emit(TranscriptEvent, TranscriptLine{JobID: j.ID, Event: e})
	}
}

func (s *Service) publish() {
	s.mu.Lock()
	// Keep every unfinished job and the latest finished ones.
	finished := 0
	kept := s.jobs[:0]
	for i := len(s.jobs) - 1; i >= 0; i-- {
		j := s.jobs[i]
		if j.State != StateQueued && j.State != StateRunning {
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
}

// run carries a job from clone to the last GitLab update, whatever happens.
func (s *Service) run(ctx context.Context, j *job, active scheduler.Active) {
	env, cleanup, err := s.prepare(ctx, j, active)
	defer cleanup()

	var outcome agent.Outcome
	if err == nil {
		inbox := make(chan agent.Event, 16)
		watchCtx, stopWatch := context.WithCancel(ctx)
		go s.watchThread(watchCtx, j, env, inbox)

		p := active.Profile
		outcome, err = agent.Run(ctx, agent.Config{
			URL: active.URL, Model: active.Model, MaxTurns: p.MaxTurns,
			Timeout: time.Duration(p.MaxMinutes) * time.Minute, ContextTokens: p.ContextPerJob,
		}, env, j.systemPrompt, j.taskPrompt, inbox, func(e agent.Event) {
			s.mu.Lock()
			j.events = append(j.events, e)
			s.mu.Unlock()
			if app := application.Get(); app != nil {
				app.Event.Emit(TranscriptEvent, TranscriptLine{JobID: j.ID, Event: e})
			}
		})
		stopWatch()
		j.Turns = outcome.Turns
	}
	// The job's own time limit is internal to the loop; ctx ends only on Cancel or shutdown.
	s.finish(j, env, outcome, err, ctx.Err() != nil)
}

func (s *Service) prepare(ctx context.Context, j *job, active scheduler.Active) (*agent.Env, func(), error) {
	var cleanups []func()
	cleanup := func() {
		for i := len(cleanups) - 1; i >= 0; i-- {
			cleanups[i]()
		}
	}
	t := j.todo

	client, err := gitlab.NewClient(t.ProjectID)
	if err != nil {
		return nil, cleanup, err
	}
	mr, err := client.MergeRequest(ctx, t.MRIID)
	if err != nil {
		return nil, cleanup, err
	}
	env := &agent.Env{Kind: agent.KindPing, GitLab: client, MR: mr, Requester: t.Author}

	if id, err := client.React(ctx, mr.IID, "eyes"); err == nil {
		j.awardID = id
	} else {
		s.emit(j, "info", "GitLab", "Impossible d'ajouter 👀 : "+err.Error())
	}

	if t.NoteID != 0 {
		s.emit(j, "info", "GitLab", "Recherche du fil de la mention…")
		if discussions, err := client.Discussions(ctx, mr.IID); err == nil {
			if d, ok := gitlab.DiscussionOf(discussions, t.NoteID); ok {
				s.mu.Lock()
				j.threadID = d.ID
				for _, n := range d.Notes {
					j.seen[n.ID] = true
				}
				s.mu.Unlock()
				env.ThreadID = d.ID
			}
		}
	}

	config, err := projects.Load()
	if err != nil {
		return env, cleanup, err
	}
	dir, err := sandbox.NewWorkspace()
	if err != nil {
		return env, cleanup, err
	}
	cleanups = append(cleanups, func() { os.RemoveAll(dir) })
	repo := filepath.Join(dir, "repo")

	path, err := client.ProjectPath(ctx)
	if err != nil {
		return env, cleanup, err
	}
	s.emit(j, "info", "Préparation", "Clone de "+path+" ("+mr.SourceBranch+")…")
	if mr.FromFork() {
		err = gitlab.Clone(ctx, path, mr.TargetBranch, repo)
		if err == nil {
			err = gitlab.CheckoutMergeRequest(ctx, repo, mr.IID)
		}
	} else {
		err = gitlab.Clone(ctx, path, mr.SourceBranch, repo)
	}
	if err != nil {
		return env, cleanup, err
	}

	image := config.Image(t.ProjectID)
	s.emit(j, "info", "Préparation", "Démarrage du container "+image+"…")
	container, err := sandbox.Start(ctx, sandbox.Spec{Image: image, Workspace: repo})
	if err != nil {
		return env, cleanup, err
	}
	cleanups = append(cleanups, func() { _ = container.Close() })
	env.Runner = container

	env.Push = func(ctx context.Context, message string) (string, error) {
		container.Chown(ctx)
		bot, err := client.Identity(ctx, client.Bot)
		if err != nil {
			return "", err
		}
		if requester, err := client.Identity(ctx, t.Author); err == nil && t.Author != client.Bot {
			message = strings.TrimSpace(message) + fmt.Sprintf("\n\nCo-authored-by: %s <%s>", requester.Name, requester.Email)
		}
		return gitlab.Push(ctx, repo, mr.SourceBranch, message, bot)
	}

	j.systemPrompt = systemPrompt(active.Profile.Prompt, config.Project(t.ProjectID).Instructions, readMihariMD(repo))
	j.taskPrompt = taskPrompt(t, mr)
	return env, cleanup, nil
}

// watchThread passes new replies in the job's thread to the agent, from
// people allowed to call the bot, mention or not.
func (s *Service) watchThread(ctx context.Context, j *job, env *agent.Env, inbox chan<- agent.Event) {
	if env.ThreadID == "" {
		return
	}
	ticker := time.NewTicker(threadPoll)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
		d, err := env.GitLab.Discussion(ctx, env.MR.IID, env.ThreadID)
		if err != nil {
			continue
		}
		config, err := projects.Load()
		if err != nil {
			continue
		}
		var members []gitlab.Member
		for _, n := range d.Notes {
			s.mu.Lock()
			seen := j.seen[n.ID]
			j.seen[n.ID] = true
			s.mu.Unlock()
			if seen || n.Author == env.GitLab.Bot {
				continue
			}
			if members == nil {
				if members, err = gitlab.ProjectMembers(ctx, j.todo.ProjectID); err != nil {
					break
				}
			}
			if !config.Project(j.todo.ProjectID).Authorizes(n.Author, members) {
				continue
			}
			select {
			case inbox <- agent.Event{At: time.Now(), Kind: "message", Title: "Nouveau message de @" + n.Author + " dans le fil", Text: n.Body}:
			default:
			}
		}
	}
}

// finish reports the outcome on GitLab and closes the todos.
func (s *Service) finish(j *job, env *agent.Env, outcome agent.Outcome, runErr error, canceled bool) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()

	state, reason := StateSucceeded, ""
	switch {
	case canceled:
		state, reason = StateCanceled, "Job annulé depuis Mihari."
	case runErr != nil:
		state, reason = StateFailed, runErr.Error()
	case !outcome.Success:
		state, reason = StateFailed, outcome.Summary
	}

	if env != nil && env.GitLab != nil && env.MR.IID != 0 {
		// The model's summary is the answer when it forgot to post one.
		if state == StateSucceeded && !env.Replied && outcome.Summary != "" {
			s.post(ctx, env, outcome.Summary)
		}
		if state != StateSucceeded {
			msg := "❌ Je n'ai pas pu terminer : " + reason
			if canceled {
				msg = "❌ " + reason
			}
			s.post(ctx, env, msg)
		}
		if j.awardID != 0 {
			_ = env.GitLab.Unreact(ctx, env.MR.IID, j.awardID)
		}
		emoji := "white_check_mark"
		if state != StateSucceeded {
			emoji = "x"
		}
		_, _ = env.GitLab.React(ctx, env.MR.IID, emoji)
	}

	s.mu.Lock()
	ids := append([]int{}, j.todoIDs...)
	s.mu.Unlock()
	s.closeTodos(ids)

	s.mu.Lock()
	j.State, j.EndedAt, j.Error, j.Result = state, time.Now(), reason, outcome.Summary
	s.mu.Unlock()
	kind := "info"
	if state != StateSucceeded {
		kind = "error"
	}
	s.emit(j, kind, "Fin", strings.TrimSpace(fmt.Sprintf("%s %s", stateLabel(state), reason)))
	s.publish()
	// A drained profile can switch now; the next queued job can start.
	s.Scheduler.Nudge()
	s.poke()
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

func (s *Service) post(ctx context.Context, env *agent.Env, body string) {
	var err error
	if env.ThreadID != "" {
		_, err = env.GitLab.Reply(ctx, env.MR.IID, env.ThreadID, body)
	} else {
		_, err = env.GitLab.Comment(ctx, env.MR.IID, body)
	}
	if err != nil {
		log.Printf("post on !%d: %v", env.MR.IID, err)
	}
}

func stateLabel(s State) string {
	switch s {
	case StateSucceeded:
		return "Terminé."
	case StateCanceled:
		return "Annulé."
	default:
		return "Échec :"
	}
}
