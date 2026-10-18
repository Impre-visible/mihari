// Package desktop keeps Mihari running in the background: an icon in the
// menu bar or notification area, a window that hides instead of closing,
// launch at login and system notifications.
package desktop

import (
	_ "embed"
	"fmt"
	"log"
	"runtime"
	"strconv"
	"sync"
	"time"

	"github.com/Impre-visible/mihari/internal/devmode"
	"github.com/Impre-visible/mihari/internal/jobs"
	"github.com/Impre-visible/mihari/internal/scheduler"
	"github.com/Impre-visible/mihari/internal/settings"
	"github.com/wailsapp/wails/v3/pkg/application"
	"github.com/wailsapp/wails/v3/pkg/events"
	"github.com/wailsapp/wails/v3/pkg/services/notifications"
)

var (
	// macOS tints a template image to match the menu bar.
	//go:embed icons/tray-template.png
	trayTemplate []byte
	// Dark glyph for light panels, light glyph for dark ones.
	//go:embed icons/tray-dark.png
	trayOnLight []byte
	//go:embed icons/tray-light.png
	trayOnDark []byte
)

// HiddenFlag is passed at login, so Mihari starts in the background.
const HiddenFlag = "--hidden"

// OpenJobEvent asks the frontend to show a job's transcript.
const OpenJobEvent = "desktop:open-job"

// Desktop is wired by main; the frontend reaches it through Service.
type Desktop struct {
	app      *application.App
	window   *application.WebviewWindow
	notifier *notifications.NotificationService
	busy     func() bool

	mu     sync.Mutex
	status *application.MenuItem
	menu   *application.Menu
	nextID int
}

// New prepares the notification service, which must be registered with the
// app before it runs.
func New() *Desktop {
	return &Desktop{notifier: notifications.New()}
}

func (d *Desktop) Notifier() *notifications.NotificationService {
	return d.notifier
}

// Attach sets up the tray icon and the window's close button once the app
// and its window exist. busy reports running jobs, which quitting would cut.
func (d *Desktop) Attach(app *application.App, window *application.WebviewWindow, busy func() bool) {
	d.app, d.window, d.busy = app, window, busy

	// Closing hides: the bot keeps watching. Quit is in the tray menu.
	window.RegisterHook(events.Common.WindowClosing, func(e *application.WindowEvent) {
		e.Cancel()
		window.Hide()
	})

	tray := app.SystemTray.New()
	if runtime.GOOS == "darwin" {
		tray.SetTemplateIcon(trayTemplate)
	} else {
		tray.SetIcon(trayOnLight)
		tray.SetDarkModeIcon(trayOnDark)
	}
	tray.SetTooltip("Mihari")

	menu := app.NewMenu()
	menu.Add("Ouvrir Mihari").OnClick(func(*application.Context) { d.Show() })
	menu.AddSeparator()
	d.status = menu.Add("Aucun job en cours").SetEnabled(false)
	menu.AddSeparator()
	menu.Add("Quitter Mihari").OnClick(func(*application.Context) { d.quit() })
	d.menu = menu
	tray.SetMenu(menu)
	tray.OnClick(d.Show)

	d.notifier.OnNotificationResponse(func(r notifications.NotificationResult) {
		d.Show()
		if id, ok := r.Response.UserInfo["jobId"]; ok {
			app.Event.Emit(OpenJobEvent, id)
		}
	})
	if runtime.GOOS == "darwin" {
		if _, err := d.notifier.RequestNotificationAuthorization(); err != nil {
			log.Printf("notifications: %v", err)
		}
	}
	d.applyAutostart()
}

// Show brings the window back.
func (d *Desktop) Show() {
	if d.window == nil {
		return
	}
	d.window.Show()
	d.window.Focus()
}

func (d *Desktop) quit() {
	if d.busy == nil || !d.busy() {
		d.app.Quit()
		return
	}
	dialog := d.app.Dialog.Question().
		SetTitle("Des jobs tournent encore").
		SetMessage("Quitter maintenant les arrête. Ils seront marqués interrompus et repris au prochain lancement.")
	dialog.AddButton("Quitter quand même").OnClick(func() { d.app.Quit() })
	dialog.AddButton("Annuler").SetAsDefault().SetAsCancel()
	dialog.Show()
}

// SetRunning updates the tray menu's status line.
func (d *Desktop) SetRunning(n int) {
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.status == nil {
		return
	}
	switch n {
	case 0:
		d.status.SetLabel("Aucun job en cours")
	case 1:
		d.status.SetLabel("1 job en cours")
	default:
		d.status.SetLabel(strconv.Itoa(n) + " jobs en cours")
	}
	d.menu.Update()
}

func (d *Desktop) notify(title, body string, jobID int) {
	if d.app == nil {
		return
	}
	d.mu.Lock()
	d.nextID++
	id := fmt.Sprintf("mihari-%d-%d", time.Now().Unix(), d.nextID)
	d.mu.Unlock()
	opts := notifications.NotificationOptions{ID: id, Title: title, Body: body}
	if jobID != 0 {
		opts.Data = map[string]any{"jobId": jobID}
	}
	if err := d.notifier.SendNotification(opts); err != nil {
		log.Printf("notification: %v", err)
	}
}

// JobFinished notifies a failed job; successes stay quiet.
func (d *Desktop) JobFinished(j jobs.Summary) {
	if j.State != jobs.StateFailed {
		return
	}
	ref := j.Project + "!" + strconv.Itoa(j.IID)
	if j.TargetType == "Issue" {
		ref = j.Project + "#" + strconv.Itoa(j.IID)
	}
	d.notify("Job en échec : "+ref, j.Error, j.ID)
}

// EngineFailed notifies an engine that would not start or stopped on its own:
// no job runs until someone acts.
func (d *Desktop) EngineFailed(s scheduler.Snapshot) {
	d.notify("Le moteur de « "+s.ProfileName+" » est en erreur", s.Error, 0)
}

// PeriodEnded sums up the night's issues.
func (d *Desktop) PeriodEnded(p jobs.PeriodSummary) {
	body := fmt.Sprintf("%d traitée(s), %d en échec", p.Succeeded, p.Failed)
	if p.Running > 0 {
		body += fmt.Sprintf(", %d encore en cours", p.Running)
	}
	d.notify("Fin de « "+p.Profile+" »", body, 0)
}

// Service is exposed to the frontend through Wails bindings.
type Service struct {
	desktop *Desktop
}

// Service returns the frontend's view of d.
func (d *Desktop) Service() *Service {
	return &Service{desktop: d}
}

// Autostart reports whether Mihari launches at login. On by default.
func (s *Service) Autostart() bool {
	return s.desktop.Autostart()
}

// SetAutostart turns launch at login on or off.
func (s *Service) SetAutostart(enabled bool) error {
	return s.desktop.SetAutostart(enabled)
}

// DevBuild reports a build where launch at login is never registered.
func (s *Service) DevBuild() bool {
	return devmode.Enabled
}

// Autostart reports whether Mihari launches at login. On by default.
func (d *Desktop) Autostart() bool {
	value, _ := (&settings.Service{}).Get("autostart")
	enabled, isBool := value.(bool)
	return !isBool || enabled
}

// SetAutostart turns launch at login on or off.
func (d *Desktop) SetAutostart(enabled bool) error {
	if err := (&settings.Service{}).Set("autostart", enabled); err != nil {
		return err
	}
	return d.applyAutostart()
}

// applyAutostart registers the binary to launch at login, hidden. A dev
// build is never registered: it would start a binary wails3 dev rebuilds.
func (d *Desktop) applyAutostart() error {
	if d.app == nil || devmode.Enabled {
		return nil
	}
	if !d.Autostart() {
		return d.app.Autostart.Disable()
	}
	return d.app.Autostart.EnableWithOptions(application.AutostartOptions{Arguments: []string{HiddenFlag}})
}
