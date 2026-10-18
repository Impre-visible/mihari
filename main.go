package main

import (
	"embed"

	"log"
	"time"

	"github.com/Impre-visible/mihari/internal/engine"
	"github.com/Impre-visible/mihari/internal/gitlab"
	"github.com/Impre-visible/mihari/internal/projects"
	"github.com/Impre-visible/mihari/internal/scheduler"
	"github.com/Impre-visible/mihari/internal/settings"
	"github.com/Impre-visible/mihari/internal/tasks"
	"github.com/wailsapp/wails/v3/pkg/application"
)

// Wails uses Go's `embed` package to embed the frontend files into the binary.
// Any files in the frontend/dist folder will be embedded into the binary and
// made available to the frontend.
// See https://pkg.go.dev/embed for more information.

//go:embed all:frontend/dist
var assets embed.FS

func init() {
	// Register a custom event whose associated data type is string.
	// This is not required, but the binding generator will pick up registered events
	// and provide a strongly typed JS/TS API for them.
	application.RegisterEvent[string]("time")
	application.RegisterEvent[tasks.Snapshot](tasks.UpdatedEvent)
	application.RegisterEvent[scheduler.Snapshot](scheduler.UpdatedEvent)
	application.RegisterEvent[engine.Download](engine.DownloadEvent)
}

// main function serves as the application's entry point. It initializes the application, creates a window,
// and starts a goroutine that emits a time-based event every second. It subsequently runs the application and
// logs any error that might occur.
func main() {

	// Create a new Wails application by providing the necessary options.
	// Variables 'Name' and 'Description' are for application metadata.
	// 'Assets' configures the asset server with the 'FS' variable pointing to the frontend files.
	// 'Bind' is a list of Go struct instances. The frontend has access to the methods of these instances.
	// 'Mac' options tailor the application when running an macOS.
	app := application.New(application.Options{
		Name:        "mihari",
		Description: "To be determined",
		Services: []application.Service{
			application.NewService(&settings.Service{}),
			application.NewService(&gitlab.Service{}),
			application.NewService(&projects.Service{}),
			application.NewService(&tasks.Service{}),
			application.NewService(&scheduler.Service{}),
			application.NewService(&engine.Service{}),
		},
		Assets: application.AssetOptions{
			Handler: application.AssetFileServerFS(assets),
		},
		Mac: application.MacOptions{
			ApplicationShouldTerminateAfterLastWindowClosed: true,
		},
	})

	// Create a new window with the necessary options.
	// 'Title' is the title of the window.
	// 'Mac' options tailor the window when running on macOS.
	// 'BackgroundColour' is the background colour of the window.
	// 'URL' is the URL that will be loaded into the webview.
	app.Window.NewWithOptions(application.WebviewWindowOptions{
		Title: "Mihari",
		// Window sized to the golden ratio (1000 / 618 ≈ 1.618).
		Width:     1000,
		Height:    618,
		MinWidth:  720,
		MinHeight: 480,
		Mac: application.MacWindow{
			// Vibrancy behind the sidebar, which the frontend leaves transparent.
			Backdrop: application.MacBackdropTranslucent,
			// Traffic lights inset to the middle of the frontend's 52px titlebar band.
			// Dragging comes from --wails-draggable in that band rather than from
			// InvisibleTitleBarHeight, which would also swallow clicks on toolbar buttons.
			TitleBar: application.MacTitleBarHiddenInsetUnified,
		},
		BackgroundColour: application.NewRGB(6, 7, 15),
		URL:              "/",
	})

	// Create a goroutine that emits an event containing the current time every second.
	// The frontend can listen to this event and update the UI accordingly.
	go func() {
		for {
			now := time.Now().Format(time.RFC1123)
			app.Event.Emit("time", now)
			time.Sleep(time.Second)
		}
	}()

	// Run the application. This blocks until the application has been exited.
	err := app.Run()

	// If an error occurred while running the application, log it and exit.
	if err != nil {
		log.Fatal(err)
	}
}
