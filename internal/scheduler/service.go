// Package scheduler keeps the engine of the profile the schedule calls for running.
package scheduler

import (
	"context"
	"log"
	"reflect"
	"sync"
	"time"

	"github.com/Impre-visible/mihari/internal/engine"
	"github.com/Impre-visible/mihari/internal/profiles"
	"github.com/wailsapp/wails/v3/pkg/application"
)

// The schedule has minute precision, so checking twice a minute is enough.
const interval = 30 * time.Second

// UpdatedEvent carries a Snapshot whenever the engine state changes.
const UpdatedEvent = "engine:updated"

type State string

const (
	// No profile is scheduled right now.
	StateIdle     State = "idle"
	StateStarting State = "starting"
	StateReady    State = "ready"
	// The engine failed to start or exited; it stays down until the settings
	// change or Reload is called, rather than crash-looping every tick.
	StateError State = "error"
)

type Snapshot struct {
	State       State  `json:"state"`
	ProfileID   string `json:"profileId"`
	ProfileName string `json:"profileName"`
	Engine      string `json:"engine"`
	Model       string `json:"model"`
	MaxJobs     int    `json:"maxJobs"`
	URL         string `json:"url"`
	Error       string `json:"error"`
	// When State last changed.
	Since time.Time `json:"since"`
}

// Service is exposed to the frontend through Wails bindings.
type Service struct {
	mu       sync.Mutex
	snapshot Snapshot
	// The profile the running or failed engine was started from.
	current *profiles.Profile
	server  *engine.Server
	// Cancels a start in progress, so a settings change does not wait for a slow load.
	cancelStart context.CancelFunc
	wake        chan struct{}
	// The schedule calls for another profile, but jobs still run on this one.
	draining bool
	// Reports whether jobs are running; set before startup.
	Busy func() bool
	// Called when the engine fails to start or stops on its own.
	OnError func(Snapshot)
}

// Active is what jobs need from the running engine.
type Active struct {
	Ready   bool
	Profile profiles.Profile
	URL     string
	Model   string
	// No new job should start: the engine changes once the running ones end.
	Draining bool
}

func (s *Service) Active() Active {
	s.mu.Lock()
	defer s.mu.Unlock()
	a := Active{Ready: s.snapshot.State == StateReady, URL: s.snapshot.URL, Model: s.snapshot.Model, Draining: s.draining}
	if s.current != nil {
		a.Profile = *s.current
	}
	return a
}

// Nudge re-checks the schedule now, as when the last job of a drained profile ends.
func (s *Service) Nudge() {
	select {
	case s.wake <- struct{}{}:
	default:
	}
}

func (s *Service) ServiceStartup(ctx context.Context, _ application.ServiceOptions) error {
	engine.ReapOrphans()
	s.wake = make(chan struct{}, 1)
	s.snapshot = Snapshot{State: StateIdle, Since: time.Now()}
	go s.run(ctx)
	return nil
}

func (s *Service) ServiceShutdown() error {
	s.mu.Lock()
	server := s.server
	s.server = nil
	s.mu.Unlock()
	if server != nil {
		server.Stop()
	}
	return nil
}

// Snapshot returns the current engine state.
func (s *Service) Snapshot() Snapshot {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.snapshot
}

// Engines lists the inference engines and whether each is installed.
func (s *Service) Engines() []engine.Info {
	return engine.Catalog()
}

// Reload re-reads the profiles and the schedule now, and retries an engine
// that failed. The frontend calls it after saving either.
func (s *Service) Reload() {
	s.mu.Lock()
	if s.cancelStart != nil {
		s.cancelStart()
	}
	if s.snapshot.State == StateError {
		s.current = nil
	}
	s.mu.Unlock()
	select {
	case s.wake <- struct{}{}:
	default:
	}
}

func (s *Service) run(ctx context.Context) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		s.reconcile(ctx)

		s.mu.Lock()
		var exited <-chan struct{}
		if s.server != nil {
			exited = s.server.Done()
		}
		s.mu.Unlock()

		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		case <-s.wake:
		case <-exited:
			s.mu.Lock()
			server := s.server
			s.server = nil
			s.mu.Unlock()
			if server != nil {
				s.set(func(snap *Snapshot) {
					snap.State = StateError
					snap.Error = "le moteur s'est arrêté : " + server.Logs()
				})
			}
		}
	}
}

// reconcile starts, stops or swaps the engine so it matches the schedule.
func (s *Service) reconcile(ctx context.Context) {
	config, err := profiles.Load()
	if err != nil {
		log.Printf("load profiles: %v", err)
		return
	}
	want, scheduled := config.Active(time.Now())

	s.mu.Lock()
	current := s.current
	s.mu.Unlock()

	// An edited profile counts as a different one, so its engine restarts.
	if scheduled && current != nil && reflect.DeepEqual(*current, want) {
		s.setDraining(false)
		return
	}
	if !scheduled && current == nil {
		s.setDraining(false)
		return
	}
	// Running jobs finish on the engine they started on.
	if current != nil && s.Busy != nil && s.Busy() {
		s.setDraining(true)
		return
	}
	s.setDraining(false)

	s.stop()
	if !scheduled {
		s.set(func(snap *Snapshot) { *snap = Snapshot{State: StateIdle} })
		return
	}
	s.start(ctx, want)
}

func (s *Service) setDraining(on bool) {
	s.mu.Lock()
	s.draining = on
	s.mu.Unlock()
}

func (s *Service) stop() {
	s.mu.Lock()
	server := s.server
	s.server = nil
	s.current = nil
	s.mu.Unlock()
	if server != nil {
		server.Stop()
	}
}

func (s *Service) start(ctx context.Context, p profiles.Profile) {
	startCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	s.mu.Lock()
	s.current = &p
	s.cancelStart = cancel
	s.mu.Unlock()
	s.set(func(snap *Snapshot) {
		*snap = Snapshot{
			State: StateStarting, ProfileID: p.ID, ProfileName: p.Name,
			Engine: p.Engine, Model: p.Model, MaxJobs: p.MaxJobs,
		}
	})

	server, err := engine.Start(startCtx, p)

	s.mu.Lock()
	s.cancelStart = nil
	canceled := startCtx.Err() != nil
	if err == nil {
		s.server = server
	}
	s.mu.Unlock()

	switch {
	case canceled:
		// Reload or shutdown interrupted the load; the next reconcile decides what runs.
		if server != nil {
			server.Stop()
		}
		s.mu.Lock()
		s.server, s.current = nil, nil
		s.mu.Unlock()
	case err != nil:
		log.Printf("start engine for %s: %v", p.Name, err)
		s.set(func(snap *Snapshot) {
			snap.State = StateError
			snap.Error = err.Error()
		})
	default:
		s.set(func(snap *Snapshot) {
			snap.State = StateReady
			snap.URL = server.URL
			snap.Model = server.Model
		})
	}
}

func (s *Service) set(update func(*Snapshot)) {
	s.mu.Lock()
	previous := s.snapshot.State
	update(&s.snapshot)
	if s.snapshot.State != previous || s.snapshot.Since.IsZero() {
		s.snapshot.Since = time.Now()
	}
	snapshot := s.snapshot
	s.mu.Unlock()

	if app := application.Get(); app != nil {
		app.Event.Emit(UpdatedEvent, snapshot)
	}
	if snapshot.State == StateError && previous != StateError && s.OnError != nil {
		s.OnError(snapshot)
	}
}
