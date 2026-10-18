package updates

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/wailsapp/wails/v3/pkg/application"
	"github.com/wailsapp/wails/v3/pkg/updater"
)

// replaceAppImage downloads the new AppImage next to the running one, swaps
// it in and starts it; the swap is a rename, so a failed download leaves
// the current file untouched.
func (s *Service) replaceAppImage(ctx context.Context, rel *updater.Release) error {
	current := os.Getenv("APPIMAGE")
	tmp, err := os.CreateTemp(filepath.Dir(current), ".mihari-update-*")
	if err != nil {
		return fmt.Errorf("le dossier de l'AppImage n'est pas modifiable : %w", err)
	}
	defer os.Remove(tmp.Name())
	err = s.provider.Download(ctx, rel, tmp, func(written, total int64) {
		s.mu.Lock()
		s.info.Downloaded, s.info.Total = written, total
		s.mu.Unlock()
		s.publish()
	})
	if cerr := tmp.Close(); err == nil {
		err = cerr
	}
	if err != nil {
		return err
	}
	if err := os.Chmod(tmp.Name(), 0o755); err != nil {
		return err
	}
	if err := os.Rename(tmp.Name(), current); err != nil {
		return err
	}
	if err := relaunchAfterExit(current); err != nil {
		return err
	}
	application.Get().Quit()
	return nil
}
