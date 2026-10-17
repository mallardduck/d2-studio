package main

import (
	"encoding/json"
	"runtime"
	"sync"

	"github.com/wailsapp/wails/v3/pkg/application"

	"d2studio/compiler"
	"d2studio/files"
	"d2studio/logging"
	"d2studio/menu"
)

// EventMenuAction is emitted when the user chooses a menu item. Its data is a
// menu.Action; the frontend performs the same action as for the equivalent
// toolbar button.
const EventMenuAction = "menu:action"

var menuLog = logging.For("menu")

// platformRoles maps the model's role names to Wails' standard items.
var platformRoles = map[string]application.Role{
	menu.RoleApp:       application.AppMenu,
	menu.RoleWindow:    application.WindowMenu,
	menu.RoleCut:       application.Cut,
	menu.RoleCopy:      application.Copy,
	menu.RolePaste:     application.Paste,
	menu.RoleSelectAll: application.SelectAll,
}

// buildNativeMenu turns the menu description into a platform menu. Choosing
// an item calls emit with its ID.
func buildNativeMenu(nodes []menu.Node, emit func(id string)) *application.Menu {
	m := application.NewMenu()
	addNodes(m, nodes, emit)
	return m
}

func addNodes(m *application.Menu, nodes []menu.Node, emit func(id string)) {
	for _, n := range nodes {
		switch n.Kind {
		case menu.Separator:
			m.AddSeparator()
		case menu.Submenu:
			addNodes(m.AddSubmenu(n.Label), n.Children, emit)
		case menu.RoleItem:
			if r, ok := platformRoles[n.Role]; ok {
				m.AddRole(r)
			}
		case menu.Check:
			configure(m.AddCheckbox(n.Label, n.Checked), n, emit)
		case menu.Radio:
			configure(m.AddRadio(n.Label, n.Checked), n, emit)
		default:
			configure(m.Add(n.Label), n, emit)
		}
	}
}

func configure(item *application.MenuItem, n menu.Node, emit func(id string)) {
	if n.Accelerator != "" {
		item.SetAccelerator(n.Accelerator)
	}
	if n.Disabled {
		item.SetEnabled(false)
	}
	if id := n.ID; id != "" {
		item.OnClick(func(*application.Context) { emit(id) })
	}
}

// MenuService keeps the native menu bar in step with the app. The frontend
// owns the state and calls Sync after it changes.
type MenuService struct {
	app *application.App

	mu   sync.Mutex
	last string // what was last applied, to skip identical updates
}

func NewMenuService(app *application.App) *MenuService { return &MenuService{app: app} }

// Sync rebuilds the menu bar for the frontend's current state. It does
// nothing when nothing the menu shows has changed.
func (s *MenuService) Sync(state menu.State) { s.apply(state, true) }

// ApplyInitial sets the menu before the application runs, so the first thing
// the user sees is ours and not Wails' default. It is for the Go side only,
// so it is left out of the frontend bindings.
//
//wails:ignore
func (s *MenuService) ApplyInitial(state menu.State) { s.apply(state, false) }

func (s *MenuService) apply(state menu.State, running bool) {
	in := menuInputs(s.app)
	key, err := json.Marshal(struct {
		State  menu.State
		Inputs menu.Inputs
	}{state, in})
	if err != nil {
		return
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	if string(key) == s.last {
		return
	}
	s.last = string(key)

	native := buildNativeMenu(menu.Build(state, in), func(id string) {
		menuLog.Trace("menu item chosen", "id", id)
		s.app.Event.Emit(EventMenuAction, menu.Action{ID: id})
	})
	menuLog.Debug("menu rebuilt", "running", running, "view_mode", state.ViewMode, "autosave", state.Autosave,
		"recent_files", len(in.Recent), "can_undo", state.CanUndo, "can_redo", state.CanRedo)
	if !running {
		s.app.Menu.Set(native)
		return
	}
	// Menus must be changed on the main thread.
	application.InvokeSync(func() { s.app.Menu.Set(native) })
}

// recents is set by main once the file service exists.
var recentFiles func() []files.RecentFile

func menuInputs(*application.App) menu.Inputs {
	in := menu.Inputs{GOOS: runtime.GOOS}
	if recentFiles != nil {
		for _, r := range recentFiles() {
			in.Recent = append(in.Recent, menu.Recent{Path: r.Path, Name: r.Name, Dir: r.Dir, Exists: r.Exists})
		}
	}
	for _, e := range compiler.Engines() {
		in.Engines = append(in.Engines, menu.Engine{ID: e.ID, Name: e.Name, Available: e.Available})
	}
	for _, t := range compiler.Themes() {
		in.Themes = append(in.Themes, menu.Theme{ID: t.ID, Name: t.Name, Dark: t.Dark})
	}
	return in
}
