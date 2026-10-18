// Package updates offers new releases of Mihari from GitHub and installs
// them when the user says yes.
package updates

import (
	"context"
	"errors"
	"log"
	"os"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/Impre-visible/mihari/internal/devmode"
	"github.com/Impre-visible/mihari/internal/version"
	"github.com/wailsapp/wails/v3/pkg/application"
	"github.com/wailsapp/wails/v3/pkg/updater"
	"github.com/wailsapp/wails/v3/pkg/updater/providers/github"
)

const (
	repository = "Impre-visible/mihari"
	// Published by the release workflow next to the installers.
	checksums  = "SHA256SUMS"
	checkEvery = 6 * time.Hour
)

// UpdatedEvent carries an Info whenever what is known about updates changes.
const UpdatedEvent = "updates:changed"

// How a release can be installed on this machine.
type Method string

const (
	// Wails swaps the app bundle or binary in place and restarts it.
	MethodInPlace Method = "inplace"
	// The AppImage file Mihari runs from is replaced.
	MethodAppImage Method = "appimage"
	// Installed by a package manager: the release page opens instead.
	MethodManual Method = "manual"
)

type Info struct {
	Current   string `json:"current"`
	Available bool   `json:"available"`
	Version   string `json:"version"`
	Notes     string `json:"notes"`
	// The release page, for notes and manual downloads.
	URL        string `json:"url"`
	Method     Method `json:"method"`
	Installing bool   `json:"installing"`
	// Bytes downloaded so far, for the progress bar.
	Downloaded int64     `json:"downloaded"`
	Total      int64     `json:"total"`
	Error      string    `json:"error"`
	CheckedAt  time.Time `json:"checkedAt"`
	// A development build never offers updates.
	DevBuild bool `json:"devBuild"`
}

// Service is exposed to the frontend through Wails bindings.
type Service struct {
	mu       sync.Mutex
	info     Info
	provider *github.Provider
	release  *updater.Release
	wake     chan struct{}
}

func (s *Service) ServiceStartup(ctx context.Context, _ application.ServiceOptions) error {
	s.info = Info{Current: version.Version, Method: method(), DevBuild: devmode.Enabled}
	s.wake = make(chan struct{}, 1)
	if devmode.Enabled {
		return nil
	}
	p, err := github.New(github.Config{Repository: repository, ChecksumAsset: checksums, AssetMatcher: matchAsset})
	if err != nil {
		return err
	}
	s.provider = p
	if s.info.Method == MethodInPlace {
		if app := application.Get(); app != nil {
			err := app.Updater.Init(updater.Config{
				CurrentVersion: version.Version,
				Providers:      []updater.Provider{p},
				// Our own banner asks; Wails never opens its window.
				Window: updater.WindowNone,
			})
			if err != nil {
				log.Printf("updater: %v", err)
				s.info.Method = MethodManual
			}
		}
	}
	go s.loop(ctx)
	return nil
}

// method tells how this copy of Mihari can be replaced.
func method() Method {
	switch runtime.GOOS {
	case "darwin", "windows":
		return MethodInPlace
	case "linux":
		if os.Getenv("APPIMAGE") != "" {
			return MethodAppImage
		}
	}
	return MethodManual
}

// matchAsset picks the release file for this machine: the zipped app on
// macOS and Windows, the AppImage on Linux.
func matchAsset(req updater.CheckRequest, assets []github.ReleaseAsset) int {
	want := map[string]string{
		"darwin":  "-darwin-arm64.zip",
		"windows": "-windows-amd64.zip",
		"linux":   "-linux-x86_64.appimage",
	}[req.Platform]
	for i, a := range assets {
		if want != "" && strings.HasSuffix(strings.ToLower(a.Name), want) {
			return i
		}
	}
	return -1
}

func (s *Service) loop(ctx context.Context) {
	ticker := time.NewTicker(checkEvery)
	defer ticker.Stop()
	for {
		s.check(ctx)
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		case <-s.wake:
		}
	}
}

func (s *Service) check(ctx context.Context) {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	var (
		rel *updater.Release
		err error
	)
	if app := application.Get(); app != nil && s.methodIs(MethodInPlace) {
		rel, err = app.Updater.Check(ctx)
	} else {
		rel, err = s.provider.Check(ctx, updater.CheckRequest{CurrentVersion: version.Version, Platform: runtime.GOOS, Arch: runtime.GOARCH})
	}

	s.mu.Lock()
	s.info.CheckedAt = time.Now()
	s.info.Error = ""
	switch {
	case err != nil:
		s.info.Error = "vérification impossible : " + err.Error()
	case rel == nil:
		s.release = nil
		s.info.Available, s.info.Version, s.info.Notes, s.info.URL = false, "", "", ""
	default:
		s.release = rel
		s.info.Available, s.info.Version, s.info.Notes = true, rel.Version, rel.Notes
		s.info.URL, _ = rel.Metadata["github.release.htmlURL"].(string)
	}
	s.mu.Unlock()
	s.publish()
}

func (s *Service) methodIs(m Method) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.info.Method == m
}

func (s *Service) publish() {
	s.mu.Lock()
	info := s.info
	s.mu.Unlock()
	if app := application.Get(); app != nil {
		app.Event.Emit(UpdatedEvent, info)
	}
}

// Status returns what is known about updates.
func (s *Service) Status() Info {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.info
}

// CheckNow looks for a release without waiting for the next periodic check.
func (s *Service) CheckNow() {
	if s.wake == nil {
		return
	}
	select {
	case s.wake <- struct{}{}:
	default:
	}
}

// Install downloads the available release and restarts on it. Where Mihari
// cannot replace itself, it opens the release page instead.
func (s *Service) Install() error {
	s.mu.Lock()
	rel, m, page := s.release, s.info.Method, s.info.URL
	if rel == nil {
		s.mu.Unlock()
		return errors.New("aucune mise à jour disponible")
	}
	if s.info.Installing {
		s.mu.Unlock()
		return nil
	}
	if m != MethodManual {
		s.info.Installing, s.info.Error = true, ""
	}
	s.mu.Unlock()
	s.publish()

	app := application.Get()
	if m == MethodManual {
		if app == nil {
			return errors.New("application indisponible")
		}
		return app.Browser.OpenURL(page)
	}

	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
		defer cancel()
		var err error
		if m == MethodInPlace {
			if err = app.Updater.DownloadAndInstall(ctx); err == nil {
				err = app.Updater.Restart(ctx)
			}
		} else {
			err = s.replaceAppImage(ctx, rel)
		}
		if err != nil {
			s.mu.Lock()
			s.info.Installing, s.info.Error = false, "mise à jour impossible : "+err.Error()
			s.mu.Unlock()
			s.publish()
		}
	}()
	return nil
}
