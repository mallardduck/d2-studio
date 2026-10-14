// Package menu describes the application's native menu bar as plain data.
//
// It has no UI dependency: Build turns the app's state into a tree of Nodes,
// and a thin adapter (in package main) turns that tree into the platform's
// real menu. Keeping the description pure makes the menu's content, shortcuts
// and checked/disabled states testable without a window system.
//
// Choosing a menu item does not run any logic here. It sends the item's ID to
// the frontend, which performs the same action as the equivalent toolbar
// button. The frontend sends its state back (State) so check marks stay right.
package menu

import (
	"fmt"
	"strconv"
)

// Kind is the type of a Node.
type Kind string

const (
	Item      Kind = "item"
	Check     Kind = "check"
	Radio     Kind = "radio"
	Separator Kind = "separator"
	Submenu   Kind = "submenu"
	// RoleItem is a standard platform item (Cut, Copy, ...) named by Node.Role.
	RoleItem Kind = "role"
)

// Role names for RoleItem nodes. The adapter maps them to platform roles.
const (
	RoleApp       = "app"
	RoleWindow    = "window"
	RoleCut       = "cut"
	RoleCopy      = "copy"
	RolePaste     = "paste"
	RoleSelectAll = "selectAll"
)

// Node is one menu, item or separator.
type Node struct {
	Kind  Kind   `json:"kind"`
	ID    string `json:"id,omitempty"`
	Label string `json:"label,omitempty"`
	// Accelerator uses Wails' syntax, e.g. "CmdOrCtrl+Shift+S".
	Accelerator string `json:"accelerator,omitempty"`
	Checked     bool   `json:"checked,omitempty"`
	Disabled    bool   `json:"disabled,omitempty"`
	Role        string `json:"role,omitempty"`
	Children    []Node `json:"children,omitempty"`
}

// Action is sent to the frontend when a menu item is chosen.
type Action struct {
	ID string `json:"id"`
}

// State is what the menu needs to know to show the right check marks and
// enabled items. The frontend owns it and sends it after every change.
type State struct {
	ViewMode    string `json:"viewMode"`
	SidebarOpen bool   `json:"sidebarOpen"`
	HasFolder   bool   `json:"hasFolder"`
	// Autosave is "off", "delay" or "blur".
	Autosave          string `json:"autosave"`
	AutosaveDelaySec  int    `json:"autosaveDelaySec"`
	Layout            string `json:"layout"`
	ThemeID           *int64 `json:"themeId"`
	FetchRemoteImages bool   `json:"fetchRemoteImages"`
	CanUndo           bool   `json:"canUndo"`
	CanRedo           bool   `json:"canRedo"`
	// Exporting disables the export items while one is running.
	Exporting bool `json:"exporting"`
	// Path is the open file. It is not shown; changing it tells the menu to
	// refresh the recent-files list.
	Path string `json:"path"`
}

// Recent is a recent-files entry.
type Recent struct {
	Path   string
	Name   string
	Dir    string
	Exists bool
}

// Engine is a layout engine choice.
type Engine struct {
	ID        string
	Name      string
	Available bool
}

// Theme is a theme choice.
type Theme struct {
	ID   int64
	Name string
	Dark bool
}

// Inputs are the lists a menu is built from.
type Inputs struct {
	Recent  []Recent
	Engines []Engine
	Themes  []Theme
	// GOOS selects platform conventions (menu layout, Redo shortcut).
	GOOS string
}

// Menu IDs. The frontend parses these (frontend/src/menu/actions.ts); a test
// keeps the two in step.
const (
	IDNew          = "file.new"
	IDOpen         = "file.open"
	IDOpenFolder   = "file.openFolder"
	IDCloseFolder  = "file.closeFolder"
	IDSave         = "file.save"
	IDSaveAs       = "file.saveAs"
	IDQuit         = "file.quit"
	IDClearRecent  = "file.recent.clear"
	IDRemoteImages = "export.remote"
	IDUndo         = "edit.undo"
	IDRedo         = "edit.redo"
	IDSidebar      = "view.sidebar"
)

// Parameterized IDs.
func IDRecent(path string) string { return "file.recent:" + path }
func IDAutosave(mode string, sec int) string {
	if mode == "delay" {
		return "autosave:delay:" + strconv.Itoa(sec)
	}
	return "autosave:" + mode
}
func IDExport(format string) string { return "export:" + format }
func IDViewMode(mode string) string { return "view.mode:" + mode }
func IDLayout(engine string) string { return "layout:" + engine }
func IDTheme(id *int64) string {
	if id == nil {
		return "theme:"
	}
	return "theme:" + strconv.FormatInt(*id, 10)
}

func isMac(goos string) bool { return goos == "darwin" }

func sep() Node { return Node{Kind: Separator} }

func item(id, label, accel string) Node {
	return Node{Kind: Item, ID: id, Label: label, Accelerator: accel}
}

func sub(label string, children ...Node) Node {
	return Node{Kind: Submenu, Label: label, Children: children}
}

func radio(id, label string, checked bool) Node {
	return Node{Kind: Radio, ID: id, Label: label, Checked: checked}
}

func role(r string) Node { return Node{Kind: RoleItem, Role: r} }

// Build returns the top-level menus for the given state.
func Build(s State, in Inputs) []Node {
	var bar []Node
	if isMac(in.GOOS) {
		bar = append(bar, role(RoleApp))
	}
	bar = append(bar, fileMenu(s, in), editMenu(s, in), viewMenu(s, in))
	if isMac(in.GOOS) {
		bar = append(bar, role(RoleWindow))
	}
	return bar
}

func fileMenu(s State, in Inputs) Node {
	m := []Node{
		item(IDNew, "New", "CmdOrCtrl+N"),
		item(IDOpen, "Open…", "CmdOrCtrl+O"),
		recentMenu(in.Recent),
		item(IDOpenFolder, "Open Folder…", "CmdOrCtrl+Shift+O"),
		disabledIf(item(IDCloseFolder, "Close Folder", ""), !s.HasFolder),
		sep(),
		item(IDSave, "Save", "CmdOrCtrl+S"),
		item(IDSaveAs, "Save As…", "CmdOrCtrl+Shift+S"),
		autosaveMenu(s),
		sep(),
		exportMenu(s),
	}
	if !isMac(in.GOOS) {
		// On macOS, Quit lives in the application menu.
		m = append(m, sep(), item(IDQuit, "Exit", "CmdOrCtrl+Q"))
	}
	return sub("File", m...)
}

func disabledIf(n Node, off bool) Node {
	n.Disabled = off
	return n
}

func recentMenu(recent []Recent) Node {
	if len(recent) == 0 {
		return sub("Open Recent", Node{Kind: Item, Label: "No Recent Files", Disabled: true})
	}
	// Distinguish same-named files by their folder.
	names := map[string]int{}
	for _, r := range recent {
		names[r.Name]++
	}
	var kids []Node
	for _, r := range recent {
		label := r.Name
		if names[r.Name] > 1 && r.Dir != "" {
			label = fmt.Sprintf("%s — %s", r.Name, r.Dir)
		}
		if !r.Exists {
			label += " (missing)"
		}
		kids = append(kids, disabledIf(item(IDRecent(r.Path), label, ""), !r.Exists))
	}
	kids = append(kids, sep(), item(IDClearRecent, "Clear Menu", ""))
	return sub("Open Recent", kids...)
}

func autosaveMenu(s State) Node {
	choices := []struct {
		mode  string
		sec   int
		label string
	}{
		{"off", 0, "Off"},
		{"delay", 2, "After a 2-Second Pause"},
		{"delay", 10, "After a 10-Second Pause"},
		{"blur", 0, "When the Window Loses Focus"},
	}
	var kids []Node
	for _, c := range choices {
		on := s.Autosave == c.mode && (c.mode != "delay" || s.AutosaveDelaySec == c.sec)
		kids = append(kids, radio(IDAutosave(c.mode, c.sec), c.label, on))
	}
	return sub("Autosave", kids...)
}

func exportMenu(s State) Node {
	return sub("Export",
		disabledIf(item(IDExport("svg"), "Export as SVG…", ""), s.Exporting),
		disabledIf(item(IDExport("png"), "Export as PNG…", ""), s.Exporting),
		disabledIf(item(IDExport("pdf"), "Export as PDF…", ""), s.Exporting),
		sep(),
		Node{Kind: Check, ID: IDRemoteImages, Label: "Fetch Remote Images", Checked: s.FetchRemoteImages},
	)
}

func editMenu(s State, in Inputs) Node {
	redoAccel := "CmdOrCtrl+Shift+Z"
	if !isMac(in.GOOS) {
		redoAccel = "CmdOrCtrl+Y"
	}
	return sub("Edit",
		disabledIf(item(IDUndo, "Undo", "CmdOrCtrl+Z"), !s.CanUndo),
		disabledIf(item(IDRedo, "Redo", redoAccel), !s.CanRedo),
		sep(),
		role(RoleCut), role(RoleCopy), role(RolePaste), role(RoleSelectAll),
	)
}

func viewMenu(s State, in Inputs) Node {
	modes := []struct{ id, label, accel string }{
		{"source", "Source", "CmdOrCtrl+1"},
		{"split", "Split", "CmdOrCtrl+2"},
		{"preview", "Preview", "CmdOrCtrl+3"},
	}
	var m []Node
	for _, md := range modes {
		n := radio(IDViewMode(md.id), md.label, s.ViewMode == md.id)
		n.Accelerator = md.accel
		m = append(m, n)
	}
	m = append(m, sep(),
		Node{Kind: Check, ID: IDSidebar, Label: "Show Sidebar", Accelerator: "CmdOrCtrl+B", Checked: s.SidebarOpen},
		sep(),
		layoutMenu(s, in.Engines),
		themeMenu(s, in.Themes),
	)
	return sub("View", m...)
}

func layoutMenu(s State, engines []Engine) Node {
	kids := []Node{radio(IDLayout(""), "Auto (from File)", s.Layout == "")}
	for _, e := range engines {
		n := radio(IDLayout(e.ID), e.Name, s.Layout == e.ID)
		n.Disabled = !e.Available
		kids = append(kids, n)
	}
	return sub("Layout Engine", kids...)
}

func themeMenu(s State, themes []Theme) Node {
	kids := []Node{radio(IDTheme(nil), "Auto (from File)", s.ThemeID == nil), sep()}
	add := func(dark bool) {
		for _, t := range themes {
			if t.Dark != dark {
				continue
			}
			id := t.ID
			on := s.ThemeID != nil && *s.ThemeID == t.ID
			kids = append(kids, radio(IDTheme(&id), t.Name, on))
		}
	}
	add(false)
	kids = append(kids, sep())
	add(true)
	return sub("Theme", kids...)
}

// IDs returns every actionable ID in the tree, in order.
func IDs(nodes []Node) []string {
	var out []string
	for _, n := range nodes {
		if n.ID != "" {
			out = append(out, n.ID)
		}
		out = append(out, IDs(n.Children)...)
	}
	return out
}
