package main

import (
	"embed"
	"log"

	"d2studio/compiler"
	"d2studio/export"
	"d2studio/files"
	"d2studio/menu"
	"d2studio/settings"
	"d2studio/workspace"

	"github.com/wailsapp/wails/v3/pkg/application"
	"github.com/wailsapp/wails/v3/pkg/events"
)

// The built frontend is embedded into the binary.
//
//go:embed all:frontend/dist
var assets embed.FS

func init() {
	// Registering the event types gives the frontend typed event payloads.
	application.RegisterEvent[files.Event](files.EventFileChanged)
	application.RegisterEvent[workspace.Update](workspace.EventChanged)
	application.RegisterEvent[menu.Action](EventMenuAction)
	application.RegisterEvent[application.Void](EventCloseRequested)
}

func main() {
	compilerCore, err := compiler.New(nil)
	if err != nil {
		log.Fatal(err)
	}
	compilerService := NewCompilerService(compilerCore)

	// shouldQuit is set once the app exists; options need it first.
	var shouldQuit func() bool
	// The service must exist before the app (it is part of the options); the
	// functions that need the app are filled in once it has been created.
	appService := &AppService{}

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

	appService.emit = func() {
		log.Printf("quit requested: asking the frontend whether it is safe")
		app.Event.Emit(EventCloseRequested)
	}
	appService.quit = app.Quit
	shouldQuit = appService.shouldQuit

	var recent *settings.RecentStore
	if p, err := settings.RecentPath(); err != nil {
		log.Printf("recent files will not be remembered: %v", err)
	} else {
		recent = settings.NewRecentStore(p)
	}
	fileService := files.NewService(&wailsPlatform{app: app}, recent, files.DefaultPollInterval)
	app.RegisterService(application.NewService(fileService))
	workspaceService := workspace.NewService(&wailsPlatform{app: app}, workspace.DefaultLimits, workspace.DefaultPollInterval)
	app.RegisterService(application.NewService(workspaceService))
	recentFiles = fileService.Recent
	menuService := NewMenuService(app)
	app.RegisterService(application.NewService(menuService))
	// Start with our own menu bar rather than Wails' default; the frontend
	// sends its real state as soon as it loads.
	menuService.ApplyInitial(menu.State{ViewMode: "split", SidebarOpen: true, Autosave: "off", AutosaveDelaySec: 2})
	app.RegisterService(application.NewService(export.NewService(compilerCore, &wailsPlatform{app: app})))

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
	workspaceService.Close()
	if err != nil {
		log.Fatal(err)
	}
}
