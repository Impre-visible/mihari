package jobs

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/Impre-visible/mihari/internal/agent"
	"github.com/Impre-visible/mihari/internal/engine"
	"github.com/Impre-visible/mihari/internal/gitlab"
	"github.com/Impre-visible/mihari/internal/profiles"
	"github.com/Impre-visible/mihari/internal/scheduler"
	"github.com/wailsapp/wails/v3/pkg/application"
)

// The bench reviews one merge request with several profiles, one after the
// other, each on its own engine, and keeps what each would have posted
// instead of posting it.

// BenchEvent carries a Bench each time one progresses.
const BenchEvent = "bench:updated"

// Benches kept on disk.
const benchKeep = 30

type BenchRun struct {
	ProfileID   string `json:"profileId"`
	ProfileName string `json:"profileName"`
	Engine      string `json:"engine"`
	Model       string `json:"model"`
	// pending, loading, running, done, failed.
	State string `json:"state"`
	Error string `json:"error"`
	// Model load time, apart from the review itself.
	LoadMs           int64        `json:"loadMs"`
	DurationMs       int64        `json:"durationMs"`
	Turns            int          `json:"turns"`
	PromptTokens     int          `json:"promptTokens"`
	CompletionTokens int          `json:"completionTokens"`
	Summary          string       `json:"summary"`
	Posts            []agent.Post `json:"posts"`
	// The run's transcript, for a closer look.
	Events []agent.Event `json:"events"`
}

type Bench struct {
	ID        int        `json:"id"`
	MR        string     `json:"mr"`
	Title     string     `json:"title"`
	URL       string     `json:"url"`
	StartedAt time.Time  `json:"startedAt"`
	EndedAt   time.Time  `json:"endedAt"`
	Running   bool       `json:"running"`
	Runs      []BenchRun `json:"runs"`
}

type benchState struct {
	mu      sync.Mutex
	current *Bench
	cancel  context.CancelFunc
}

func benchDir() (string, error) {
	base, err := os.UserCacheDir()
	if err != nil {
		return "", err
	}
	dir := filepath.Join(base, "mihari", "bench")
	return dir, os.MkdirAll(dir, 0o700)
}

func saveBench(b Bench) {
	dir, err := benchDir()
	if err != nil {
		return
	}
	if data, err := json.Marshal(b); err == nil {
		path := filepath.Join(dir, strconv.Itoa(b.ID)+".json")
		if os.WriteFile(path+".tmp", data, 0o600) == nil {
			_ = os.Rename(path+".tmp", path)
		}
	}
}

// Benches lists past benches, newest first, without their transcripts.
func (s *Service) Benches() []Bench {
	dir, err := benchDir()
	if err != nil {
		return []Bench{}
	}
	entries, _ := os.ReadDir(dir)
	var ids []int
	for _, e := range entries {
		if id, err := strconv.Atoi(strings.TrimSuffix(e.Name(), ".json")); err == nil && strings.HasSuffix(e.Name(), ".json") {
			ids = append(ids, id)
		}
	}
	sort.Sort(sort.Reverse(sort.IntSlice(ids)))
	for len(ids) > benchKeep {
		_ = os.Remove(filepath.Join(dir, strconv.Itoa(ids[len(ids)-1])+".json"))
		ids = ids[:len(ids)-1]
	}
	out := []Bench{}
	for _, id := range ids {
		if b, ok := s.loadBench(id); ok {
			for i := range b.Runs {
				b.Runs[i].Events = nil
			}
			out = append(out, b)
		}
	}
	return out
}

func (s *Service) loadBench(id int) (Bench, bool) {
	s.bench.mu.Lock()
	if s.bench.current != nil && s.bench.current.ID == id {
		b := *s.bench.current
		s.bench.mu.Unlock()
		return b, true
	}
	s.bench.mu.Unlock()
	dir, err := benchDir()
	if err != nil {
		return Bench{}, false
	}
	data, err := os.ReadFile(filepath.Join(dir, strconv.Itoa(id)+".json"))
	if err != nil {
		return Bench{}, false
	}
	var b Bench
	return b, json.Unmarshal(data, &b) == nil
}

// BenchDetail returns a bench with its transcripts.
func (s *Service) BenchDetail(id int) (Bench, error) {
	b, ok := s.loadBench(id)
	if !ok {
		return Bench{}, errors.New("banc introuvable")
	}
	return b, nil
}

// StopBench interrupts the running bench; finished runs are kept.
func (s *Service) StopBench() {
	s.bench.mu.Lock()
	defer s.bench.mu.Unlock()
	if s.bench.cancel != nil {
		s.bench.cancel()
	}
}

// StartBench reviews the merge request at mrURL with each profile, one after
// the other. It needs the GPU to itself: no job may be running.
func (s *Service) StartBench(mrURL string, profileIDs []string) (int, error) {
	if len(profileIDs) == 0 {
		return 0, errors.New("choisis au moins un profil")
	}
	if s.Busy() {
		return 0, errors.New("des jobs tournent : attends qu'ils finissent, le banc a besoin du GPU pour lui seul")
	}
	s.bench.mu.Lock()
	if s.bench.current != nil && s.bench.current.Running {
		s.bench.mu.Unlock()
		return 0, errors.New("un banc tourne déjà")
	}
	s.bench.mu.Unlock()

	ctx, cancel := context.WithCancel(s.ctx)
	todo, err := gitlab.SimulatedTodo(ctx, mrURL, "", "")
	if err != nil {
		cancel()
		return 0, err
	}
	if todo.TargetType != gitlab.MergeRequestTarget {
		cancel()
		return 0, errors.New("le banc relit une merge request, pas une issue")
	}
	config, err := profiles.Load()
	if err != nil {
		cancel()
		return 0, err
	}
	var chosen []profiles.Profile
	for _, id := range profileIDs {
		if p, ok := config.Profile(id); ok {
			chosen = append(chosen, p)
		}
	}
	if len(chosen) == 0 {
		cancel()
		return 0, errors.New("aucun des profils choisis n'existe")
	}

	b := &Bench{
		ID: int(time.Now().UnixMilli()), MR: todo.Project + "!" + strconv.Itoa(todo.IID),
		Title: todo.Title, URL: todo.URL, StartedAt: time.Now(), Running: true,
	}
	for _, p := range chosen {
		b.Runs = append(b.Runs, BenchRun{ProfileID: p.ID, ProfileName: p.Name, Engine: p.Engine, Model: p.Model, State: "pending"})
	}
	s.bench.mu.Lock()
	s.bench.current, s.bench.cancel = b, cancel
	s.bench.mu.Unlock()
	s.publishBench()

	go s.runBench(ctx, cancel, todo, chosen)
	return b.ID, nil
}

func (s *Service) runBench(ctx context.Context, cancel context.CancelFunc, todo gitlab.Todo, chosen []profiles.Profile) {
	defer cancel()
	// The scheduled engine leaves the GPU to the bench, and gets it back after.
	s.Scheduler.Hold()
	defer s.Scheduler.Release()

	for i, p := range chosen {
		if ctx.Err() != nil {
			s.updateRun(i, func(r *BenchRun) { r.State, r.Error = "failed", "banc arrêté" })
			continue
		}
		s.benchProfile(ctx, i, todo, p)
	}

	s.bench.mu.Lock()
	s.bench.current.Running, s.bench.current.EndedAt = false, time.Now()
	final := *s.bench.current
	s.bench.mu.Unlock()
	saveBench(final)
	s.publishBench()
	// Jobs held back by the bench can start.
	s.poke()
}

func (s *Service) benchProfile(ctx context.Context, i int, todo gitlab.Todo, p profiles.Profile) {
	s.updateRun(i, func(r *BenchRun) { r.State = "loading" })
	loadStart := time.Now()
	server, err := engine.Start(ctx, p)
	if err != nil {
		s.updateRun(i, func(r *BenchRun) { r.State, r.Error = "failed", err.Error() })
		return
	}
	defer server.Stop()
	s.updateRun(i, func(r *BenchRun) { r.State, r.LoadMs = "running", time.Since(loadStart).Milliseconds() })

	// A review request with no text: every profile gets the same task.
	review := todo
	review.Body, review.Action, review.NoteID = "", "review_requested", 0
	j := &job{
		Summary: Summary{Kind: agent.KindPing, TargetType: todo.TargetType, Project: todo.Project,
			ProjectID: todo.ProjectID, IID: todo.IID, Title: todo.Title, URL: todo.URL, Author: todo.Author},
		todo: review, dry: true, seen: map[int]bool{}, events: []agent.Event{},
	}
	active := scheduler.Active{Ready: true, Profile: p, URL: server.URL, Model: server.Model}
	env, cleanup, err := s.prepare(ctx, j, active)
	defer cleanup()
	if err != nil {
		s.updateRun(i, func(r *BenchRun) { r.State, r.Error, r.Events = "failed", err.Error(), j.events })
		return
	}

	begin := time.Now()
	outcome, err := agent.Run(ctx, agent.Config{
		URL: server.URL, Model: server.Model, MaxTurns: p.TurnLimit(),
		Timeout: p.TimeLimit(), ContextTokens: p.ContextPerJob,
		Critic: true, CriticRules: j.criticRules, KeepReasoning: keepsReasoning(p.Engine),
	}, env, j.systemPrompt, j.taskPrompt, nil, func(e agent.Event) {
		j.events = append(j.events, e)
	})
	s.updateRun(i, func(r *BenchRun) {
		r.DurationMs = time.Since(begin).Milliseconds()
		r.Turns, r.PromptTokens, r.CompletionTokens = outcome.Turns, outcome.PromptTokens, outcome.CompletionTokens
		r.Summary, r.Posts, r.Events = outcome.Summary, env.Posts, j.events
		r.State = "done"
		if err != nil {
			r.State, r.Error = "failed", err.Error()
		}
	})
}

func (s *Service) updateRun(i int, f func(*BenchRun)) {
	s.bench.mu.Lock()
	if s.bench.current != nil && i < len(s.bench.current.Runs) {
		f(&s.bench.current.Runs[i])
	}
	s.bench.mu.Unlock()
	s.publishBench()
}

func (s *Service) publishBench() {
	s.bench.mu.Lock()
	if s.bench.current == nil {
		s.bench.mu.Unlock()
		return
	}
	b := *s.bench.current
	b.Runs = slices.Clone(b.Runs)
	for i := range b.Runs {
		// Transcripts are fetched with BenchDetail; the event stays light.
		b.Runs[i].Events = nil
	}
	s.bench.mu.Unlock()
	if app := application.Get(); app != nil {
		app.Event.Emit(BenchEvent, b)
	}
}
