// Package tasks polls GitLab for the work assigned to the bot.
package tasks

import (
	"context"
	"errors"
	"log"
	"slices"
	"sync"
	"time"

	"github.com/Impre-visible/mihari/internal/devmode"
	"github.com/Impre-visible/mihari/internal/gitlab"
	"github.com/Impre-visible/mihari/internal/projects"
	"github.com/wailsapp/wails/v3/pkg/application"
)

// Interval between two polls of the GitLab to-do list.
const Interval = 30 * time.Second

// UpdatedEvent carries a Snapshot after each poll.
const UpdatedEvent = "tasks:updated"

// Snapshot is the outcome of the latest poll.
type Snapshot struct {
	// Ready to run: an enabled project, and an author allowed to call the bot.
	Todos []gitlab.Todo `json:"todos"`
	// On switched-off projects: left pending on GitLab and read again each poll,
	// so they run once the project is switched on.
	Waiting []Waiting `json:"waiting"`
	// Recently refused because the author may not call the bot; GitLab todos
	// among them were marked done.
	Refused []Refusal `json:"refused"`
	// Zero until the first poll has finished.
	PolledAt time.Time `json:"polledAt"`
	// Set when the latest poll failed; Todos then still holds the last good list.
	Error string `json:"error"`
	// GitLab has no URL or token yet, so polling is waiting for the settings.
	NotConfigured bool `json:"notConfigured"`
}

type Waiting struct {
	Todo   gitlab.Todo `json:"todo"`
	Reason string      `json:"reason"`
}

type Refusal struct {
	Todo gitlab.Todo `json:"todo"`
	At   time.Time   `json:"at"`
}

// How many refusals the Status page keeps.
const refusalHistory = 20

// Service is exposed to the frontend through Wails bindings.
type Service struct {
	mu       sync.Mutex
	snapshot Snapshot
	// The latest todos read from GitLab, kept apart from the simulated ones.
	polled    []gitlab.Todo
	simulated []gitlab.Todo
	// Simulated todos get negative ids so they never collide with GitLab's.
	lastSimID int
	wake      chan struct{}
}

func (s *Service) ServiceStartup(ctx context.Context, _ application.ServiceOptions) error {
	s.wake = make(chan struct{}, 1)
	go s.run(ctx)
	return nil
}

// Snapshot returns the outcome of the latest poll.
func (s *Service) Snapshot() Snapshot {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.snapshot
}

// PollNow asks for a poll without waiting for the next tick, for instance
// right after the GitLab settings were saved.
func (s *Service) PollNow() {
	select {
	case s.wake <- struct{}{}:
	default:
	}
}

func (s *Service) run(ctx context.Context) {
	ticker := time.NewTicker(Interval)
	defer ticker.Stop()

	for {
		s.poll(ctx)
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		case <-s.wake:
			ticker.Reset(Interval)
		}
	}
}

func (s *Service) poll(ctx context.Context) {
	ctx, cancel := context.WithTimeout(ctx, 25*time.Second)
	defer cancel()

	todos, err := gitlab.PendingTodos(ctx)

	s.mu.Lock()
	s.snapshot.PolledAt = time.Now()
	s.snapshot.NotConfigured = errors.Is(err, gitlab.ErrNotConfigured)
	s.snapshot.Error = ""
	switch {
	case s.snapshot.NotConfigured:
		s.polled = nil
	case err != nil:
		s.snapshot.Error = err.Error()
		log.Printf("poll GitLab todos: %v", err)
	default:
		s.polled = todos
	}
	all := append(append([]gitlab.Todo{}, s.polled...), s.simulated...)
	s.mu.Unlock()

	if err == nil {
		s.sort(ctx, all)
	}
	s.publish()
}

// sort splits todos into ready, waiting and refused. Membership is read
// fresh on each poll, so a role change applies at once.
func (s *Service) sort(ctx context.Context, todos []gitlab.Todo) {
	config, err := projects.Load()
	if err != nil {
		log.Printf("load projects: %v", err)
		return
	}
	members := map[int][]gitlab.Member{}
	memberErrors := map[int]error{}

	ready := []gitlab.Todo{}
	waiting := []Waiting{}
	var refused []Refusal
	for _, todo := range todos {
		project := config.Project(todo.ProjectID)
		if !project.Enabled {
			waiting = append(waiting, Waiting{Todo: todo, Reason: "projet éteint"})
			continue
		}
		if _, ok := members[todo.ProjectID]; !ok && memberErrors[todo.ProjectID] == nil {
			list, err := gitlab.ProjectMembers(ctx, todo.ProjectID)
			members[todo.ProjectID], memberErrors[todo.ProjectID] = list, err
		}
		if err := memberErrors[todo.ProjectID]; err != nil {
			waiting = append(waiting, Waiting{Todo: todo, Reason: "membres illisibles : " + err.Error()})
			continue
		}
		if project.Authorizes(todo.Author, members[todo.ProjectID]) {
			ready = append(ready, todo)
			continue
		}
		// Refused in silence: nothing is posted, the todo just leaves the bot's list.
		if !todo.Simulated {
			if err := gitlab.MarkTodoDone(ctx, todo.ID); err != nil {
				log.Printf("mark todo %d done: %v", todo.ID, err)
				continue
			}
		}
		refused = append(refused, Refusal{Todo: todo, At: time.Now()})
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	s.snapshot.Todos, s.snapshot.Waiting = ready, waiting
	for _, r := range refused {
		s.dropSimulated(r.Todo.ID)
	}
	s.snapshot.Refused = append(refused, s.snapshot.Refused...)
	if len(s.snapshot.Refused) > refusalHistory {
		s.snapshot.Refused = s.snapshot.Refused[:refusalHistory]
	}
}

// publish sends the snapshot to the frontend.
func (s *Service) publish() {
	s.mu.Lock()
	snapshot := s.snapshot
	s.mu.Unlock()

	if app := application.Get(); app != nil {
		app.Event.Emit(UpdatedEvent, snapshot)
	}
}

// Simulate queues the todo author mentioning the bot with body on the merge
// request at mrURL would create, for testing without pinging on GitLab.
func (s *Service) Simulate(mrURL, author, body string) (gitlab.Todo, error) {
	if !devmode.Enabled {
		return gitlab.Todo{}, devmode.ErrDisabled
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	todo, err := gitlab.MergeRequestTodo(ctx, mrURL, author, body)
	if err != nil {
		return gitlab.Todo{}, err
	}
	s.mu.Lock()
	s.lastSimID--
	todo.ID = s.lastSimID
	s.simulated = append(s.simulated, todo)
	s.mu.Unlock()
	// Sorting it needs the project's members, which the poll reads.
	s.PollNow()
	return todo, nil
}

// RemoveSimulated drops a simulated todo; GitLab todos are left alone.
func (s *Service) RemoveSimulated(id int) {
	s.mu.Lock()
	s.dropSimulated(id)
	s.snapshot.Todos = slices.DeleteFunc(s.snapshot.Todos, func(t gitlab.Todo) bool { return t.ID == id })
	s.snapshot.Waiting = slices.DeleteFunc(s.snapshot.Waiting, func(w Waiting) bool { return w.Todo.ID == id })
	s.mu.Unlock()
	s.publish()
}

// dropSimulated forgets a simulated todo; s.mu must be held.
func (s *Service) dropSimulated(id int) {
	s.simulated = slices.DeleteFunc(s.simulated, func(t gitlab.Todo) bool { return t.ID == id })
}
