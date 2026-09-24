package files

import (
	"bytes"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

func write(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func tmpFile(t *testing.T, name, content string) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), name)
	write(t, p, content)
	return p
}

// bump moves a file's mtime so a change is visible even on coarse clocks.
func bump(t *testing.T, path string) {
	t.Helper()
	future := time.Now().Add(time.Hour)
	if err := os.Chtimes(path, future, future); err != nil {
		t.Fatal(err)
	}
}

func TestOpenBasics(t *testing.T) {
	p := tmpFile(t, "a.d2", "a -> b\n")
	doc, err := Open(p)
	if err != nil {
		t.Fatal(err)
	}
	if doc.Content != "a -> b\n" || doc.LineEnding != LineEndingLF || doc.BOM || doc.MixedLineEndings {
		t.Fatalf("doc = %+v", doc)
	}
	if doc.Version.Hash == "" || doc.Version.Size != 7 {
		t.Fatalf("version = %+v", doc.Version)
	}
}

func TestOpenNormalizesCRLFAndRecordsIt(t *testing.T) {
	p := tmpFile(t, "a.d2", "a\r\nb\r\nc\r\n")
	doc, err := Open(p)
	if err != nil {
		t.Fatal(err)
	}
	if doc.Content != "a\nb\nc\n" || doc.LineEnding != LineEndingCRLF || doc.MixedLineEndings {
		t.Fatalf("doc = %+v", doc)
	}
}

func TestOpenDetectsMixedLineEndings(t *testing.T) {
	doc, err := Open(tmpFile(t, "a.d2", "a\r\nb\r\nc\nd\r\n"))
	if err != nil {
		t.Fatal(err)
	}
	if doc.LineEnding != LineEndingCRLF || !doc.MixedLineEndings || strings.Contains(doc.Content, "\r") {
		t.Fatalf("doc = %+v", doc)
	}
}

func TestOpenStripsBOM(t *testing.T) {
	p := filepath.Join(t.TempDir(), "a.d2")
	if err := os.WriteFile(p, append([]byte{0xEF, 0xBB, 0xBF}, "x: y"...), 0o644); err != nil {
		t.Fatal(err)
	}
	doc, err := Open(p)
	if err != nil {
		t.Fatal(err)
	}
	if !doc.BOM || doc.Content != "x: y" {
		t.Fatalf("doc = %+v", doc)
	}
}

func TestOpenRefusesBadInput(t *testing.T) {
	dir := t.TempDir()
	invalid := filepath.Join(dir, "latin1.d2")
	if err := os.WriteFile(invalid, []byte{'a', ':', ' ', 0xE9}, 0o644); err != nil { // é in Latin-1
		t.Fatal(err)
	}
	for name, path := range map[string]string{
		"missing":       filepath.Join(dir, "nope.d2"),
		"directory":     dir,
		"invalid utf-8": invalid,
	} {
		if _, err := Open(path); err == nil {
			t.Errorf("%s: expected an error", name)
		}
	}
	if _, err := Open(invalid); err == nil || !strings.Contains(err.Error(), "UTF-8") {
		t.Errorf("error should mention UTF-8: %v", err)
	}
	if _, err := Open(filepath.Join(dir, "nope.d2")); err == nil || !strings.Contains(err.Error(), "does not exist") {
		t.Errorf("missing-file error should be friendly: %v", err)
	}
}

func TestOpenRefusesHugeFiles(t *testing.T) {
	p := filepath.Join(t.TempDir(), "big.d2")
	f, err := os.Create(p)
	if err != nil {
		t.Fatal(err)
	}
	if err := f.Truncate(MaxFileSize + 1); err != nil { // sparse; no real disk use
		t.Fatal(err)
	}
	f.Close()
	if _, err := Open(p); err == nil || !strings.Contains(err.Error(), "limit") {
		t.Fatalf("err = %v", err)
	}
}

func TestSaveNewFileAndRoundTrip(t *testing.T) {
	p := filepath.Join(t.TempDir(), "new.d2")
	res, err := Save(p, "a -> b\n", SaveOptions{})
	if err != nil || !res.Saved {
		t.Fatalf("res=%+v err=%v", res, err)
	}
	doc, err := Open(p)
	if err != nil || doc.Content != "a -> b\n" {
		t.Fatalf("doc=%+v err=%v", doc, err)
	}
	if doc.Version != res.Version {
		t.Fatalf("Save reported %+v but Open sees %+v", res.Version, doc.Version)
	}
}

// Saving must not silently rewrite a file's line endings or drop its BOM.
func TestSavePreservesCRLFAndBOM(t *testing.T) {
	p := filepath.Join(t.TempDir(), "a.d2")
	if err := os.WriteFile(p, append([]byte{0xEF, 0xBB, 0xBF}, "a\r\nb\r\n"...), 0o644); err != nil {
		t.Fatal(err)
	}
	doc, err := Open(p)
	if err != nil {
		t.Fatal(err)
	}
	res, err := Save(p, doc.Content+"c\n", SaveOptions{Base: &doc.Version, LineEnding: doc.LineEnding, BOM: doc.BOM})
	if err != nil || !res.Saved {
		t.Fatalf("res=%+v err=%v", res, err)
	}
	got, _ := os.ReadFile(p)
	want := append([]byte{0xEF, 0xBB, 0xBF}, "a\r\nb\r\nc\r\n"...)
	if !bytes.Equal(got, want) {
		t.Fatalf("file bytes = %q, want %q", got, want)
	}
}

func TestSaveLeavesNoTempFiles(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "a.d2")
	for i := 0; i < 3; i++ {
		if _, err := Save(p, "x", SaveOptions{}); err != nil {
			t.Fatal(err)
		}
	}
	entries, _ := os.ReadDir(dir)
	if len(entries) != 1 {
		t.Fatalf("directory contents: %v", entries)
	}
}

func TestSaveFailureLeavesOriginalIntact(t *testing.T) {
	// A destination in a missing directory fails; nothing should be created.
	dir := t.TempDir()
	p := filepath.Join(dir, "missing", "a.d2")
	if _, err := Save(p, "x", SaveOptions{}); err == nil {
		t.Fatal("expected an error")
	}
	entries, _ := os.ReadDir(dir)
	if len(entries) != 0 {
		t.Fatalf("left behind: %v", entries)
	}
}

func TestSavePreservesPermissions(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("POSIX permission bits")
	}
	p := tmpFile(t, "a.d2", "x")
	if err := os.Chmod(p, 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := Save(p, "y", SaveOptions{}); err != nil {
		t.Fatal(err)
	}
	info, _ := os.Stat(p)
	if info.Mode().Perm() != 0o600 {
		t.Fatalf("mode = %v, want 0600", info.Mode().Perm())
	}
}

func TestSaveWritesThroughSymlink(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlinks need privileges on Windows")
	}
	dir := t.TempDir()
	real := filepath.Join(dir, "real.d2")
	link := filepath.Join(dir, "link.d2")
	write(t, real, "old")
	if err := os.Symlink(real, link); err != nil {
		t.Fatal(err)
	}
	if _, err := Save(link, "new", SaveOptions{}); err != nil {
		t.Fatal(err)
	}
	if info, _ := os.Lstat(link); info.Mode()&os.ModeSymlink == 0 {
		t.Fatal("the symlink was replaced by a regular file")
	}
	if b, _ := os.ReadFile(real); string(b) != "new" {
		t.Fatalf("target = %q", b)
	}
}

func TestSaveDetectsExternalModification(t *testing.T) {
	p := tmpFile(t, "a.d2", "original")
	doc, _ := Open(p)

	write(t, p, "someone else was here")
	bump(t, p)

	res, err := Save(p, "my edits", SaveOptions{Base: &doc.Version})
	if err != nil {
		t.Fatal(err)
	}
	if res.Saved || res.Conflict != ConflictModified {
		t.Fatalf("res = %+v, want a modified conflict", res)
	}
	if b, _ := os.ReadFile(p); string(b) != "someone else was here" {
		t.Fatalf("file was overwritten despite the conflict: %q", b)
	}
	if res.Version.Hash == doc.Version.Hash {
		t.Fatal("conflict should report the current on-disk version")
	}
}

func TestSaveDetectsDeletion(t *testing.T) {
	p := tmpFile(t, "a.d2", "original")
	doc, _ := Open(p)
	os.Remove(p)

	res, err := Save(p, "mine", SaveOptions{Base: &doc.Version})
	if err != nil || res.Saved || res.Conflict != ConflictDeleted {
		t.Fatalf("res=%+v err=%v", res, err)
	}
	if _, err := os.Stat(p); err == nil {
		t.Fatal("a conflicting save must not recreate the file")
	}
}

func TestForceOverridesConflict(t *testing.T) {
	p := tmpFile(t, "a.d2", "original")
	doc, _ := Open(p)
	write(t, p, "other")
	bump(t, p)

	res, err := Save(p, "mine", SaveOptions{Base: &doc.Version, Force: true})
	if err != nil || !res.Saved {
		t.Fatalf("res=%+v err=%v", res, err)
	}
	if b, _ := os.ReadFile(p); string(b) != "mine" {
		t.Fatalf("file = %q", b)
	}
}

// A tool that touches the file without changing it must not cause a conflict.
func TestTouchWithoutChangeIsNotAConflict(t *testing.T) {
	p := tmpFile(t, "a.d2", "same")
	doc, _ := Open(p)
	bump(t, p)

	res, err := Save(p, "edited", SaveOptions{Base: &doc.Version})
	if err != nil || !res.Saved {
		t.Fatalf("res=%+v err=%v", res, err)
	}
}

func TestSaveAsOverwritesWithoutBase(t *testing.T) {
	p := tmpFile(t, "a.d2", "old")
	res, err := Save(p, "new", SaveOptions{}) // the dialog already confirmed overwrite
	if err != nil || !res.Saved {
		t.Fatalf("res=%+v err=%v", res, err)
	}
}

func TestSaveRejectsEmptyPath(t *testing.T) {
	if _, err := Save("", "x", SaveOptions{}); err == nil {
		t.Fatal("expected an error")
	}
}
