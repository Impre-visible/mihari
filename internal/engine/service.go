package engine

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/wailsapp/wails/v3/pkg/application"
)

// DownloadEvent carries a Download each time a download progresses or ends.
const DownloadEvent = "model:download"

type DownloadState string

const (
	DownloadRunning DownloadState = "running"
	DownloadDone    DownloadState = "done"
	DownloadFailed  DownloadState = "failed"
)

type Download struct {
	Engine Kind          `json:"engine"`
	Model  string        `json:"model"`
	State  DownloadState `json:"state"`
	// Bytes; Total is 0 while unknown.
	Completed int64  `json:"completed"`
	Total     int64  `json:"total"`
	Error     string `json:"error"`
}

// Service is exposed to the frontend through Wails bindings.
type Service struct {
	mu        sync.Mutex
	downloads map[string]*running
}

type running struct {
	status Download
	cancel context.CancelFunc
}

func key(kind Kind, model string) string {
	return string(kind) + "|" + model
}

// Presence says whether model is ready to run on kind.
func (s *Service) Presence(kind Kind, model string) Presence {
	return presence(kind, model)
}

// Downloads lists the downloads in progress, for a screen opened mid-download.
func (s *Service) Downloads() []Download {
	s.mu.Lock()
	defer s.mu.Unlock()
	list := []Download{}
	for _, d := range s.downloads {
		list = append(list, d.status)
	}
	return list
}

// Download starts fetching model in the background; progress arrives as DownloadEvent.
func (s *Service) Download(kind Kind, model string) {
	s.mu.Lock()
	if s.downloads == nil {
		s.downloads = map[string]*running{}
	}
	k := key(kind, model)
	if _, busy := s.downloads[k]; busy {
		s.mu.Unlock()
		return
	}
	ctx, cancel := context.WithCancel(context.Background())
	d := &running{status: Download{Engine: kind, Model: model, State: DownloadRunning}, cancel: cancel}
	s.downloads[k] = d
	s.mu.Unlock()
	emit(d.status)

	go func() {
		var last time.Time
		err := download(ctx, kind, model, func(done, total int64) {
			s.mu.Lock()
			d.status.Completed, d.status.Total = done, total
			status := d.status
			s.mu.Unlock()
			// Four updates a second are plenty for a progress bar.
			if time.Since(last) > 250*time.Millisecond {
				last = time.Now()
				emit(status)
			}
		})

		s.mu.Lock()
		delete(s.downloads, k)
		switch {
		case errors.Is(ctx.Err(), context.Canceled):
			d.status.State, d.status.Error = DownloadFailed, "téléchargement annulé"
		case err != nil:
			d.status.State, d.status.Error = DownloadFailed, err.Error()
		default:
			d.status.State = DownloadDone
		}
		status := d.status
		s.mu.Unlock()
		cancel()
		emit(status)
	}()
}

// CancelDownload stops a download; a GGUF or MLX file resumes from where it stopped next time.
func (s *Service) CancelDownload(kind Kind, model string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if d, ok := s.downloads[key(kind, model)]; ok {
		d.cancel()
	}
}

func emit(d Download) {
	if app := application.Get(); app != nil {
		app.Event.Emit(DownloadEvent, d)
	}
}
