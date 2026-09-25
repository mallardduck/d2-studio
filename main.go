package main

import (
	"embed"
	"log"

	"d2studio/files"
	"d2studio/settings"

	"github.com/wailsapp/wails/v3/pkg/application"
	"github.com/wailsapp/wails/v3/pkg/events"
)

// The built frontend is embedded into the binary.
//
//go:embed all:frontend/dist
var assets embed.FS

func main() {
	compilerService, err := NewCompilerService()
	if err != nil {
		log.Fatal(err)
	}

	// shouldQuit is set once the app exists; options need it first.
	var shouldQuit func() bool
	appService := NewAppService(nil)

	app := application.New(application.Options{
		Name:        "D2 Studio",
		Description: "A local editor and live preview for D2 diagrams",
		Services: []application.Service{
			application.NewService(compilerService),
			application.NewService(NewSettingsService()),
			application.NewService(appService),
		},
		ShouldQuit: func() bool { return shouldQuit() },
		Assets: application.AssetOptions{
			Handler: application.AssetFileServerFS(assets),
		},
		Mac: application.MacOptions{
			ApplicationShouldTerminateAfterLastWindowClosed: true,
		},
	})

	appService.quit = app.Quit
	shouldQuit = func() bool { return appService.shouldQuit(app) }

	var recent *settings.RecentStore
	if p, err := settings.RecentPath(); err != nil {
		log.Printf("recent files will not be remembered: %v", err)
	} else {
		recent = settings.NewRecentStore(p)
	}
	fileService := files.NewService(&wailsPlatform{app: app}, recent, files.DefaultPollInterval)
	app.RegisterService(application.NewService(fileService))

	window := app.Window.NewWithOptions(application.WebviewWindowOptions{
		Title:     "D2 Studio",
		Width:     1280,
		Height:    800,
		MinWidth:  720,
		MinHeight: 480,
		URL:       "/",
	})

	// Closing the window goes through the same gate as quitting.
	window.RegisterHook(events.Common.WindowClosing, func(e *application.WindowEvent) {
		if !shouldQuit() {
			e.Cancel()
		}
	})

	err = app.Run()
	fileService.Shutdown()
	if err != nil {
		log.Fatal(err)
	}
}
