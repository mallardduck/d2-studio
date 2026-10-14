package menu

import (
	"encoding/json"
	"flag"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

var update = flag.Bool("update", false, "rewrite the ID fixture used by the frontend tests")

func i64(v int64) *int64 { return &v }

func inputs(goos string) Inputs {
	return Inputs{
		GOOS: goos,
		Recent: []Recent{
			{Path: "/Users/me/work/arch.d2", Name: "arch.d2", Dir: "~/work", Exists: true},
			{Path: "/Users/me/other/arch.d2", Name: "arch.d2", Dir: "~/other", Exists: true},
			{Path: "/Users/me/gone.d2", Name: "gone.d2", Dir: "~", Exists: false},
			{Path: "/Users/me/a:b c.d2", Name: "a:b c.d2", Dir: "~", Exists: true}, // colon and space
			{Path: `C:\Users\me\win.d2`, Name: "win.d2", Dir: `C:\Users\me`, Exists: true},
		},
		Engines: []Engine{{"dagre", "Dagre", true}, {"elk", "ELK", true}, {"tala", "TALA", false}},
		Themes: []Theme{
			{0, "Neutral default", false}, {1, "Neutral Grey", false},
			{200, "Dark Mauve", true}, {201, "Dark Flagship", true},
		},
	}
}

func defaultState() State {
	return State{ViewMode: "split", SidebarOpen: true, Autosave: "off", AutosaveDelaySec: 2}
}

func find(t *testing.T, nodes []Node, label string) Node {
	t.Helper()
	for _, n := range nodes {
		if n.Label == label {
			return n
		}
	}
	t.Fatalf("no %q in %v", label, labels(nodes))
	return Node{}
}

func labels(nodes []Node) []string {
	var out []string
	for _, n := range nodes {
		switch {
		case n.Kind == Separator:
			out = append(out, "—")
		case n.Kind == RoleItem:
			out = append(out, "role:"+n.Role)
		default:
			out = append(out, n.Label)
		}
	}
	return out
}

func walk(nodes []Node, fn func(Node)) {
	for _, n := range nodes {
		fn(n)
		walk(n.Children, fn)
	}
}

func TestTopLevelLayoutFollowsPlatformConventions(t *testing.T) {
	mac := labels(Build(defaultState(), inputs("darwin")))
	if strings.Join(mac, ",") != "role:app,File,Edit,View,role:window" {
		t.Errorf("macOS menu bar = %v", mac)
	}
	for _, goos := range []string{"windows", "linux"} {
		got := labels(Build(defaultState(), inputs(goos)))
		if strings.Join(got, ",") != "File,Edit,View" {
			t.Errorf("%s menu bar = %v", goos, got)
		}
	}
}

func TestQuitIsInTheFileMenuOnlyOffMac(t *testing.T) {
	has := func(goos string) bool {
		for _, id := range IDs(Build(defaultState(), inputs(goos))) {
			if id == IDQuit {
				return true
			}
		}
		return false
	}
	if has("darwin") {
		t.Error("macOS has Quit in the application menu already")
	}
	if !has("windows") || !has("linux") {
		t.Error("Windows and Linux need an Exit item in the File menu")
	}
}

func TestIDsAreUniqueAndWellFormed(t *testing.T) {
	grammar := regexp.MustCompile(`^(file\.(new|open|openFolder|closeFolder|save|saveAs|quit|recent\.clear|recent:.+)|` +
		`autosave:(off|blur|delay:\d+)|export:(svg|png|pdf)|export\.remote|edit\.(undo|redo)|` +
		`view\.(sidebar|mode:(source|split|preview))|layout:[a-z0-9-]*|theme:\d*)$`)
	for _, goos := range []string{"darwin", "windows", "linux"} {
		seen := map[string]bool{}
		for _, id := range IDs(Build(defaultState(), inputs(goos))) {
			if seen[id] {
				t.Errorf("%s: duplicate ID %q", goos, id)
			}
			seen[id] = true
			if !grammar.MatchString(id) {
				t.Errorf("%s: ID %q does not match the grammar the frontend parses", goos, id)
			}
		}
	}
}

// Wails only logs a malformed accelerator, leaving the item without a
// shortcut, so check them here.
func TestAcceleratorsAreValidAndUnique(t *testing.T) {
	valid := regexp.MustCompile(`^CmdOrCtrl(\+Shift)?(\+Alt)?\+[A-Z0-9]$`)
	for _, goos := range []string{"darwin", "windows", "linux"} {
		seen := map[string]string{}
		walk(Build(defaultState(), inputs(goos)), func(n Node) {
			if n.Accelerator == "" {
				return
			}
			if !valid.MatchString(n.Accelerator) {
				t.Errorf("%s: %q has an invalid accelerator %q", goos, n.Label, n.Accelerator)
			}
			if other, dup := seen[n.Accelerator]; dup {
				t.Errorf("%s: %s is used by both %q and %q", goos, n.Accelerator, other, n.Label)
			}
			seen[n.Accelerator] = n.Label
		})
	}
}

func TestEditMenuUsesTheEditorsOwnUndoAndRedo(t *testing.T) {
	edit := find(t, Build(defaultState(), inputs("darwin")), "Edit")
	undo, redo := find(t, edit.Children, "Undo"), find(t, edit.Children, "Redo")
	if undo.Kind != Item || redo.Kind != Item {
		t.Fatal("Undo and Redo must be our own items (not the webview's roles) so they match the keyboard")
	}
	if !undo.Disabled || !redo.Disabled {
		t.Error("with nothing to undo or redo, both should be disabled")
	}

	s := defaultState()
	s.CanUndo = true
	edit = find(t, Build(s, inputs("darwin")), "Edit")
	if find(t, edit.Children, "Undo").Disabled || !find(t, edit.Children, "Redo").Disabled {
		t.Error("only Undo should be enabled")
	}
	s.CanRedo = true
	edit = find(t, Build(s, inputs("darwin")), "Edit")
	if find(t, edit.Children, "Redo").Disabled {
		t.Error("Redo should be enabled")
	}

	var roles []string
	for _, n := range edit.Children {
		if n.Kind == RoleItem {
			roles = append(roles, n.Role)
		}
	}
	if strings.Join(roles, ",") != "cut,copy,paste,selectAll" {
		t.Errorf("clipboard roles = %v", roles)
	}
}

func TestRedoShortcutFollowsThePlatform(t *testing.T) {
	redo := func(goos string) string {
		return find(t, find(t, Build(defaultState(), inputs(goos)), "Edit").Children, "Redo").Accelerator
	}
	if redo("darwin") != "CmdOrCtrl+Shift+Z" {
		t.Errorf("macOS redo = %q", redo("darwin"))
	}
	if redo("windows") != "CmdOrCtrl+Y" || redo("linux") != "CmdOrCtrl+Y" {
		t.Errorf("windows/linux redo = %q / %q", redo("windows"), redo("linux"))
	}
}

func fileMenuOf(t *testing.T, s State, goos string) Node {
	t.Helper()
	return find(t, Build(s, inputs(goos)), "File")
}

func TestCloseFolderIsDisabledWithoutAFolder(t *testing.T) {
	if !find(t, fileMenuOf(t, defaultState(), "darwin").Children, "Close Folder").Disabled {
		t.Error("Close Folder should be disabled with no folder open")
	}
	s := defaultState()
	s.HasFolder = true
	if find(t, fileMenuOf(t, s, "darwin").Children, "Close Folder").Disabled {
		t.Error("Close Folder should be enabled with a folder open")
	}
}

func TestOpenRecent(t *testing.T) {
	recent := find(t, fileMenuOf(t, defaultState(), "darwin").Children, "Open Recent")
	var ids []string
	for _, n := range recent.Children {
		ids = append(ids, n.ID)
	}
	want := []string{
		"file.recent:/Users/me/work/arch.d2", "file.recent:/Users/me/other/arch.d2", "file.recent:/Users/me/gone.d2",
		"file.recent:/Users/me/a:b c.d2", `file.recent:C:\Users\me\win.d2`, "", IDClearRecent,
	}
	if strings.Join(ids, "|") != strings.Join(want, "|") {
		t.Fatalf("ids = %q\nwant %q", ids, want)
	}

	// Same-named files are told apart by folder; missing files are marked and disabled.
	if recent.Children[0].Label != "arch.d2 — ~/work" || recent.Children[1].Label != "arch.d2 — ~/other" {
		t.Errorf("duplicate names not disambiguated: %q, %q", recent.Children[0].Label, recent.Children[1].Label)
	}
	gone := recent.Children[2]
	if !gone.Disabled || !strings.Contains(gone.Label, "missing") {
		t.Errorf("missing file = %+v", gone)
	}
	if recent.Children[3].Label != "a:b c.d2" {
		t.Errorf("unique names should not get a folder suffix: %q", recent.Children[3].Label)
	}
}

func TestOpenRecentWhenEmpty(t *testing.T) {
	in := inputs("darwin")
	in.Recent = nil
	recent := find(t, find(t, Build(defaultState(), in), "File").Children, "Open Recent")
	if len(recent.Children) != 1 || !recent.Children[0].Disabled || recent.Children[0].ID != "" {
		t.Fatalf("empty recent menu = %+v", recent.Children)
	}
}

func checkedLabels(n Node) []string {
	var out []string
	for _, c := range n.Children {
		if c.Checked {
			out = append(out, c.Label)
		}
	}
	return out
}

func TestAutosaveRadios(t *testing.T) {
	cases := []struct {
		mode string
		sec  int
		want string // "" = nothing checked
	}{
		{"off", 2, "Off"},
		{"delay", 2, "After a 2-Second Pause"},
		{"delay", 10, "After a 10-Second Pause"},
		{"blur", 2, "When the Window Loses Focus"},
		{"delay", 30, ""}, // a hand-edited delay matches no preset; nothing is claimed
	}
	for _, c := range cases {
		s := defaultState()
		s.Autosave, s.AutosaveDelaySec = c.mode, c.sec
		got := checkedLabels(find(t, fileMenuOf(t, s, "darwin").Children, "Autosave"))
		switch {
		case c.want == "" && len(got) != 0, c.want != "" && (len(got) != 1 || got[0] != c.want):
			t.Errorf("%s/%d: checked = %v, want %q", c.mode, c.sec, got, c.want)
		}
	}
}

func TestExportMenu(t *testing.T) {
	export := find(t, fileMenuOf(t, defaultState(), "darwin").Children, "Export")
	if export.Children[0].ID != "export:svg" || export.Children[1].ID != "export:png" || export.Children[2].ID != "export:pdf" {
		t.Fatalf("export items = %v", IDs(export.Children))
	}
	remote := find(t, export.Children, "Fetch Remote Images")
	if remote.Kind != Check || remote.Checked {
		t.Errorf("remote images should be an unchecked checkbox by default: %+v", remote)
	}

	s := defaultState()
	s.FetchRemoteImages, s.Exporting = true, true
	export = find(t, fileMenuOf(t, s, "darwin").Children, "Export")
	if !find(t, export.Children, "Fetch Remote Images").Checked {
		t.Error("the checkbox should reflect the setting")
	}
	for _, n := range export.Children[:3] {
		if !n.Disabled {
			t.Errorf("%s should be disabled while an export runs", n.Label)
		}
	}
	if find(t, export.Children, "Fetch Remote Images").Disabled {
		t.Error("the option itself stays usable while exporting")
	}
}

func viewOf(t *testing.T, s State) Node { return find(t, Build(s, inputs("darwin")), "View") }

func TestViewModeAndSidebar(t *testing.T) {
	for _, mode := range []string{"source", "split", "preview"} {
		s := defaultState()
		s.ViewMode = mode
		var checked []string
		for _, n := range viewOf(t, s).Children {
			if strings.HasPrefix(n.ID, "view.mode:") && n.Checked {
				checked = append(checked, n.ID)
			}
		}
		if len(checked) != 1 || checked[0] != "view.mode:"+mode {
			t.Errorf("mode %s: checked = %v", mode, checked)
		}
	}
	s := defaultState()
	if !find(t, viewOf(t, s).Children, "Show Sidebar").Checked {
		t.Error("sidebar should show as on")
	}
	s.SidebarOpen = false
	if find(t, viewOf(t, s).Children, "Show Sidebar").Checked {
		t.Error("sidebar should show as off")
	}
}

func TestLayoutEngineRadios(t *testing.T) {
	s := defaultState()
	layout := find(t, viewOf(t, s).Children, "Layout Engine")
	if got := checkedLabels(layout); len(got) != 1 || got[0] != "Auto (from File)" {
		t.Errorf("default: %v", got)
	}
	s.Layout = "elk"
	layout = find(t, viewOf(t, s).Children, "Layout Engine")
	if got := checkedLabels(layout); len(got) != 1 || got[0] != "ELK" {
		t.Errorf("elk: %v", got)
	}
	if !find(t, layout.Children, "TALA").Disabled {
		t.Error("an unavailable engine should be disabled, not hidden")
	}
}

func TestThemeRadiosGroupLightThenDark(t *testing.T) {
	s := defaultState()
	theme := find(t, viewOf(t, s).Children, "Theme")
	if strings.Join(labels(theme.Children), ",") != "Auto (from File),—,Neutral default,Neutral Grey,—,Dark Mauve,Dark Flagship" {
		t.Fatalf("theme menu = %v", labels(theme.Children))
	}
	if got := checkedLabels(theme); len(got) != 1 || got[0] != "Auto (from File)" {
		t.Errorf("default: %v", got)
	}
	s.ThemeID = i64(200)
	theme = find(t, viewOf(t, s).Children, "Theme")
	if got := checkedLabels(theme); len(got) != 1 || got[0] != "Dark Mauve" {
		t.Errorf("theme 200: %v", got)
	}
	// Theme 0 is a real theme, distinct from "Auto".
	s.ThemeID = i64(0)
	theme = find(t, viewOf(t, s).Children, "Theme")
	if got := checkedLabels(theme); len(got) != 1 || got[0] != "Neutral default" {
		t.Errorf("theme 0: %v", got)
	}
}

func TestThemeIDRoundTrip(t *testing.T) {
	if IDTheme(nil) != "theme:" || IDTheme(i64(0)) != "theme:0" || IDTheme(i64(300)) != "theme:300" {
		t.Fatalf("theme IDs: %q %q %q", IDTheme(nil), IDTheme(i64(0)), IDTheme(i64(300)))
	}
}

// The frontend's tests parse every ID in this fixture, so a menu ID the
// frontend cannot handle fails there. Run `go test ./menu -update` after
// changing the menu.
func TestIDFixtureIsCurrent(t *testing.T) {
	seen := map[string]bool{}
	var ids []string
	for _, goos := range []string{"darwin", "windows"} {
		s := defaultState()
		for _, id := range IDs(Build(s, inputs(goos))) {
			if !seen[id] {
				seen[id] = true
				ids = append(ids, id)
			}
		}
	}
	got, err := json.MarshalIndent(ids, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	got = append(got, '\n')

	path := filepath.Join("..", "frontend", "src", "menu", "ids.fixture.json")
	if *update {
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, got, 0o644); err != nil {
			t.Fatal(err)
		}
		return
	}
	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("%v; run `go test ./menu -update`", err)
	}
	if string(want) != string(got) {
		t.Fatal("frontend/src/menu/ids.fixture.json is stale; run `go test ./menu -update`")
	}
}
