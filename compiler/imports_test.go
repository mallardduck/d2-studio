package compiler

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// tree writes files (relative path -> content) under a temp dir.
func tree(t *testing.T, files map[string]string) string {
	t.Helper()
	dir := t.TempDir()
	// Resolve symlinks so macOS /var -> /private/var does not confuse paths.
	dir, err := filepath.EvalSymlinks(dir)
	if err != nil {
		t.Fatal(err)
	}
	for name, content := range files {
		p := filepath.Join(dir, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

func compileAt(t *testing.T, path, workspace, source string) *Result {
	t.Helper()
	s := newService(t)
	res, err := s.Compile(context.Background(), Request{Source: source, Path: path, Workspace: workspace})
	if err != nil {
		t.Fatal(err)
	}
	return res
}

func readSrc(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func TestImportFromSameDirectory(t *testing.T) {
	dir := tree(t, map[string]string{
		"main.d2":  "...@parts\nfrom_main",
		"parts.d2": "imported_node",
	})
	main := filepath.Join(dir, "main.d2")
	res := compileAt(t, main, "", readSrc(t, main))
	if res.Status != StatusSuccess {
		t.Fatalf("diagnostics = %+v", res.Diagnostics)
	}
	for _, label := range []string{"from_main", "imported_node"} {
		if !strings.Contains(res.SVG, label) {
			t.Errorf("SVG is missing %q", label)
		}
	}
}

func TestImportFromSubdirectory(t *testing.T) {
	dir := tree(t, map[string]string{
		"main.d2":     "x: @sub/part\n",
		"sub/part.d2": "deep_node",
	})
	main := filepath.Join(dir, "main.d2")
	res := compileAt(t, main, "", readSrc(t, main))
	if res.Status != StatusSuccess || !strings.Contains(res.SVG, "deep_node") {
		t.Fatalf("status=%s diagnostics=%+v", res.Status, res.Diagnostics)
	}
}

func TestImportOutsideFolderIsBlockedWithAClearMessage(t *testing.T) {
	dir := tree(t, map[string]string{
		"proj/main.d2":     "...@../shared/secret\nok",
		"shared/secret.d2": "leaked_node",
	})
	main := filepath.Join(dir, "proj", "main.d2")
	res := compileAt(t, main, "", readSrc(t, main))
	if res.Status != StatusError {
		t.Fatalf("status = %s, want error", res.Status)
	}
	msg := res.Diagnostics[0].Message
	if !strings.Contains(msg, "outside") || !strings.Contains(msg, "workspace") {
		t.Fatalf("message should explain the restriction and the fix: %q", msg)
	}
	if strings.Contains(res.SVG, "leaked_node") {
		t.Fatal("content from outside the folder leaked into the output")
	}
}

func TestWorkspaceFolderWidensWhatImportsMayRead(t *testing.T) {
	dir := tree(t, map[string]string{
		"proj/main.d2":     "...@../shared/common\nok",
		"shared/common.d2": "shared_node",
	})
	main := filepath.Join(dir, "proj", "main.d2")
	res := compileAt(t, main, dir, readSrc(t, main)) // workspace = parent folder
	if res.Status != StatusSuccess || !strings.Contains(res.SVG, "shared_node") {
		t.Fatalf("status=%s diagnostics=%+v", res.Status, res.Diagnostics)
	}
}

func TestWorkspaceThatDoesNotContainTheFileIsIgnored(t *testing.T) {
	dir := tree(t, map[string]string{
		"a/main.d2":  "...@../b/x\nok",
		"b/x.d2":     "x_node",
		"other/keep": "",
	})
	main := filepath.Join(dir, "a", "main.d2")
	// The workspace is an unrelated folder, so it must not grant access to b/.
	res := compileAt(t, main, filepath.Join(dir, "other"), readSrc(t, main))
	if res.Status != StatusError {
		t.Fatalf("status = %s, want error", res.Status)
	}
}

func TestSymlinkCannotEscapeTheFolder(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlinks need privileges on Windows")
	}
	dir := tree(t, map[string]string{
		"proj/main.d2":        "...@link/secret\nok",
		"elsewhere/secret.d2": "escaped_node",
	})
	if err := os.Symlink(filepath.Join(dir, "elsewhere"), filepath.Join(dir, "proj", "link")); err != nil {
		t.Fatal(err)
	}
	main := filepath.Join(dir, "proj", "main.d2")
	res := compileAt(t, main, "", readSrc(t, main))
	if res.Status != StatusError || strings.Contains(res.SVG, "escaped_node") {
		t.Fatalf("a symlink let an import escape: status=%s svg-has-content=%v", res.Status, strings.Contains(res.SVG, "escaped_node"))
	}
}

func TestSymlinkInsideTheFolderIsFine(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlinks need privileges on Windows")
	}
	dir := tree(t, map[string]string{
		"main.d2":      "...@alias\nok",
		"real/part.d2": "via_link",
	})
	if err := os.Symlink(filepath.Join(dir, "real", "part.d2"), filepath.Join(dir, "alias.d2")); err != nil {
		t.Fatal(err)
	}
	main := filepath.Join(dir, "main.d2")
	res := compileAt(t, main, "", readSrc(t, main))
	if res.Status != StatusSuccess || !strings.Contains(res.SVG, "via_link") {
		t.Fatalf("status=%s diagnostics=%+v", res.Status, res.Diagnostics)
	}
}

func TestMissingImportIsReportedNotSilentlyIgnored(t *testing.T) {
	dir := tree(t, map[string]string{"main.d2": "...@nope\nok"})
	main := filepath.Join(dir, "main.d2")
	res := compileAt(t, main, "", readSrc(t, main))
	if res.Status != StatusError || !strings.Contains(res.Diagnostics[0].Message, "nope") {
		t.Fatalf("status=%s diagnostics=%+v", res.Status, res.Diagnostics)
	}
}

func TestCyclicImportIsReported(t *testing.T) {
	dir := tree(t, map[string]string{
		"a.d2": "...@b\na_node",
		"b.d2": "...@a\nb_node",
	})
	a := filepath.Join(dir, "a.d2")
	res := compileAt(t, a, "", readSrc(t, a))
	if res.Status != StatusError || !strings.Contains(res.Diagnostics[0].Message, "cyclic") {
		t.Fatalf("status=%s diagnostics=%+v", res.Status, res.Diagnostics)
	}
}

// An error inside an imported file must be attributed to that file, because
// its line numbers mean nothing in the file being edited.
func TestErrorInImportedFileNamesThatFile(t *testing.T) {
	dir := tree(t, map[string]string{
		"main.d2":   "...@broken\nok",
		"broken.d2": "a: {\n",
	})
	main := filepath.Join(dir, "main.d2")
	res := compileAt(t, main, "", readSrc(t, main))
	if res.Status != StatusError {
		t.Fatalf("status = %s", res.Status)
	}
	found := false
	for _, d := range res.Diagnostics {
		if d.HasLocation && strings.HasSuffix(d.Path, "broken.d2") {
			found = true
		}
		if d.HasLocation && d.Path == "" && strings.Contains(d.Message, "terminated") {
			t.Errorf("error in broken.d2 was attributed to the active file: %+v", d)
		}
	}
	if !found {
		t.Fatalf("no diagnostic names broken.d2: %+v", res.Diagnostics)
	}
}

func TestUnsavedDocumentHasNoImportLocation(t *testing.T) {
	res := compileAt(t, "", "", "...@something\nok")
	if res.Status != StatusError {
		t.Fatalf("status = %s; an unsaved document cannot resolve imports", res.Status)
	}
}

func TestImportRoot(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "sub", "a.d2")
	cases := []struct {
		name, file, workspace, want string
	}{
		{"unsaved", "", dir, ""},
		{"no workspace", file, "", filepath.Join(dir, "sub")},
		{"file inside workspace", file, dir, dir},
		{"file outside workspace", file, filepath.Join(dir, "elsewhere"), filepath.Join(dir, "sub")},
		{"workspace is the file's own dir", file, filepath.Join(dir, "sub"), filepath.Join(dir, "sub")},
		{"sibling with shared prefix", filepath.Join(dir, "ab", "a.d2"), filepath.Join(dir, "a"), filepath.Join(dir, "ab")},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := ImportRoot(c.file, c.workspace); got != c.want {
				t.Errorf("ImportRoot = %q, want %q", got, c.want)
			}
		})
	}
}

// "/x/proj-evil" starts with "/x/proj" but is not inside it.
func TestImportIntoSiblingWithSharedPrefixIsBlocked(t *testing.T) {
	dir := tree(t, map[string]string{
		"proj/main.d2":        "...@../proj-evil/secret\nok",
		"proj-evil/secret.d2": "evil_node",
	})
	main := filepath.Join(dir, "proj", "main.d2")
	res := compileAt(t, main, "", readSrc(t, main))
	if res.Status != StatusError || strings.Contains(res.SVG, "evil_node") {
		t.Fatalf("status=%s: a sibling folder sharing a name prefix was readable", res.Status)
	}
}
