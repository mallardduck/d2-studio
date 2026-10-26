package main

import (
	"embed"
	"os"
	"path/filepath"
	"runtime"

	"d2studio/compiler"
	"d2studio/export"
	"d2studio/files"
	"d2studio/logging"
	"d2studio/menu"
	"d2studio/openfile"
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
	application.RegisterEvent[application.Void](EventOpenRequested)
	application.RegisterEvent[application.Void](EventCloseRequested)
}

var mainLog = logging.For("main")

// fatal logs err and exits. (log.Fatal would bypass the structured log.)
func fatal(msg string, err error) {
	mainLog.Error(msg, "err", err)
	os.Exit(1)
}

// startLogging reads the logging options from settings.json (the environment
// can override them) before anything else runs, so startup is captured too.
func startLogging() (closeFn func()) {
	var opts logging.Options
	if path, err := settings.DefaultPath(); err == nil {
		opts.ConfigDir = filepath.Dir(path)
		// A missing or unreadable file just means the defaults.
		s, _ := settings.NewStore(path).Load()
		opts.Level, opts.Format, opts.FileFormat = s.LogLevel, s.LogFormat, s.LogFileFormat
	}
	closeFn, _ = logging.Init(opts)
	return closeFn
}

func main() {
	defer startLogging()()
	b := readBuildInfo()
	mainLog.Info("D2 Studio starting", "version", b.App, "revision", b.Revision, "modified", b.Modified,
		"d2", b.D2, "wails", b.Wails, "go", b.Go, "os", b.OS, "arch", b.Arch)

	compilerCore, err := compiler.New(nil)
	if err != nil {
		fatal("cannot start the compiler", err)
	}
	compilerService := NewCompilerService(compilerCore)

	// shouldQuit is set once the app exists; options need it first.
	var shouldQuit func() bool
	// The service must exist before the app (it is part of the options); the
	// functions that need the app are filled in once it has been created.
	appService := &AppService{}

	// Files the OS asks us to open (double-click, Open With, a path on the
	// command line, or a second launch). The notifier and the window exist
	// only after the app is created, so they are filled in afterwards.
	var window *application.WebviewWindow
	var app *application.App
	openRequests := openfile.New(func() {
		mainLog.Debug("the OS asked to open a file")
		app.Event.Emit(EventOpenRequested)
	})

	app = application.New(application.Options{
		Name:        "D2 Studio",
		Description: "A local editor and live preview for D2 diagrams",
		Services: []application.Service{
			application.NewService(compilerService),
			application.NewService(NewSnippetService(compilerCore)),
			application.NewService(NewSettingsService()),
			application.NewService(appService),
		},
		ShouldQuit: func() bool { return shouldQuit() },
		// With these, the OS can open .d2 files in us.
		FileAssociations: []string{".d2"},
		// Opening a second file from the OS goes to the window that is
		// already open instead of starting another copy.
		SingleInstance: &application.SingleInstanceOptions{
			UniqueID: "com.lucidinternets.d2studio",
			OnSecondInstanceLaunch: func(data application.SecondInstanceData) {
				mainLog.Debug("second launch forwarded to this instance", "args", len(data.Args))
				if len(data.Args) > 1 { // Args[0] is the program itself, not a file
					openRequests.AddArgs(data.Args[1:], data.WorkingDir)
				}
				if window != nil {
					window.UnMinimise()
					window.Show()
					window.Focus()
				}
			},
		},
		Assets: application.AssetOptions{
			Handler: application.AssetFileServerFS(assets),
		},
		Mac: application.MacOptions{
			ApplicationShouldTerminateAfterLastWindowClosed: true,
		},
	})

	appService.emit = func() {
		mainLog.Debug("quit requested: asking the frontend whether it is safe")
		app.Event.Emit(EventCloseRequested)
	}
	appService.quit = app.Quit
	shouldQuit = appService.shouldQuit

	var recent *settings.RecentStore
	if p, err := settings.RecentPath(); err != nil {
		mainLog.Warn("recent files will not be remembered", "err", err)
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
	app.RegisterService(application.NewService(&OpenService{requests: openRequests}))
	app.Event.OnApplicationEvent(events.Common.ApplicationOpenedWithFile, func(e *application.ApplicationEvent) {
		openRequests.Add(e.Context().Filename())
	})
	// A path on the command line of the first launch. Windows and Linux file
	// associations pass it this way; macOS delivers an event instead, and its
	// launcher can add arguments that are not files.
	if runtime.GOOS != "darwin" {
		openRequests.AddArgs(os.Args[1:], "")
	}
	// Start with our own menu bar rather than Wails' default; the frontend
	// sends its real state as soon as it loads.
	menuService.ApplyInitial(menu.State{ViewMode: "split", SidebarOpen: true, Autosave: "off", AutosaveDelaySec: 2})
	app.RegisterService(application.NewService(export.NewService(compilerCore, &wailsPlatform{app: app})))

	window = app.Window.NewWithOptions(application.WebviewWindowOptions{
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
		fatal("the application stopped with an error", err)
	}
}
