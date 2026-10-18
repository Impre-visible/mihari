// Package tasks polls GitLab for the work assigned to the bot.
package tasks

import (
	"context"
	"errors"
	"log"
	"sync"
	"time"

	"github.com/Impre-visible/mihari/internal/devmode"
	"github.com/Impre-visible/mihari/internal/gitlab"
	"github.com/wailsapp/wails/v3/pkg/application"
)

// Interval between two polls of the GitLab to-do list.
const Interval = 30 * time.Second

// UpdatedEvent carries a Snapshot after each poll.
const UpdatedEvent = "tasks:updated"

// Snapshot is the outcome of the latest poll.
type Snapshot struct {
	Todos []gitlab.Todo `json:"todos"`
	// Zero until the first poll has finished.
	PolledAt time.Time `json:"polledAt"`
	// Set when the latest poll failed; Todos then still holds the last good list.
	Error string `json:"error"`
	// GitLab has no URL or token yet, so polling is waiting for the settings.
	NotConfigured bool `json:"notConfigured"`
}

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
	ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
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
	s.mu.Unlock()
	s.publish()
}

// publish merges the polled and simulated todos and sends the snapshot.
func (s *Service) publish() {
	s.mu.Lock()
	s.snapshot.Todos = append(append([]gitlab.Todo{}, s.polled...), s.simulated...)
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
	s.publish()
	return todo, nil
}

// RemoveSimulated drops a simulated todo; GitLab todos are left alone.
func (s *Service) RemoveSimulated(id int) {
	s.mu.Lock()
	kept := s.simulated[:0]
	for _, t := range s.simulated {
		if t.ID != id {
			kept = append(kept, t)
		}
	}
	s.simulated = kept
	s.mu.Unlock()
	s.publish()
}
