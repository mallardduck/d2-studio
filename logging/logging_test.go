package logging

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

func env(m map[string]string) func(string) string { return func(k string) string { return m[k] } }

func restore(t *testing.T) {
	t.Helper()
	prev := slog.Default()
	t.Cleanup(func() { slog.SetDefault(prev); level.Set(slog.LevelInfo) })
}

func TestParseLevel(t *testing.T) {
	cases := map[string]slog.Level{
		"trace": LevelTrace, "TRACE": LevelTrace, "debug": slog.LevelDebug, " Debug ": slog.LevelDebug,
		"info": slog.LevelInfo, "": slog.LevelInfo, "warn": slog.LevelWarn, "warning": slog.LevelWarn, "error": slog.LevelError,
	}
	for in, want := range cases {
		if got, ok := ParseLevel(in); !ok || got != want {
			t.Errorf("ParseLevel(%q) = %v, %v; want %v, true", in, got, ok, want)
		}
	}
	if got, ok := ParseLevel("verbose"); ok || got != slog.LevelInfo {
		t.Errorf("an unknown level should fall back to info and say so: %v, %v", got, ok)
	}
}

func TestTraceIsBelowDebugAndNamedTrace(t *testing.T) {
	if LevelTrace >= slog.LevelDebug {
		t.Fatal("trace must be more verbose than debug")
	}
	if LevelName(LevelTrace) != "TRACE" || LevelName(slog.LevelDebug) != "DEBUG" {
		t.Fatalf("names: %q %q", LevelName(LevelTrace), LevelName(slog.LevelDebug))
	}
}

func output(t *testing.T, lvl string) (*bytes.Buffer, Log) {
	t.Helper()
	restore(t)
	var buf bytes.Buffer
	Init(Options{Level: lvl, Stderr: &buf, Getenv: env(nil)})
	buf.Reset() // drop the "logging started" line
	return &buf, For("test")
}

func TestLevelsFilterOutput(t *testing.T) {
	cases := []struct {
		level string
		want  []string // messages that should appear
	}{
		{"error", []string{"e"}},
		{"warn", []string{"e", "w"}},
		{"info", []string{"e", "w", "i"}},
		{"debug", []string{"e", "w", "i", "d"}},
		{"trace", []string{"e", "w", "i", "d", "t"}},
	}
	for _, c := range cases {
		t.Run(c.level, func(t *testing.T) {
			buf, log := output(t, c.level)
			log.Error("e")
			log.Warn("w")
			log.Info("i")
			log.Debug("d")
			log.Trace("t")
			for _, m := range []string{"e", "w", "i", "d", "t"} {
				present := regexp.MustCompile(`msg=` + m + `( |$)`).MatchString(buf.String())
				wanted := false
				for _, w := range c.want {
					wanted = wanted || w == m
				}
				if present != wanted {
					t.Errorf("message %q present=%v want=%v\n%s", m, present, wanted, buf.String())
				}
			}
		})
	}
}

func TestOutputNamesTheComponentAndTraceLevel(t *testing.T) {
	buf, log := output(t, "trace")
	log.Trace("polling", "files", 3)
	got := buf.String()
	for _, want := range []string{"level=TRACE", "component=test", "msg=polling", "files=3"} {
		if !strings.Contains(got, want) {
			t.Errorf("missing %q in %q", want, got)
		}
	}
}

func TestEnabledLetsCallersSkipExpensiveWork(t *testing.T) {
	_, log := output(t, "info")
	if log.Enabled(LevelTrace) || log.Enabled(slog.LevelDebug) || !log.Enabled(slog.LevelWarn) {
		t.Fatal("Enabled disagrees with the configured level")
	}
	_, log = output(t, "trace")
	if !log.Enabled(LevelTrace) {
		t.Fatal("trace should be enabled at trace")
	}
}

func TestEnvironmentOverridesTheConfiguredLevel(t *testing.T) {
	restore(t)
	var buf bytes.Buffer
	Init(Options{Level: "error", Stderr: &buf, Getenv: env(map[string]string{EnvLevel: "debug"})})
	buf.Reset()
	For("x").Debug("visible")
	if !strings.Contains(buf.String(), "visible") {
		t.Fatalf("D2STUDIO_LOG should win over the setting: %q", buf.String())
	}
}

func TestUnknownLevelWarnsAndFallsBackToInfo(t *testing.T) {
	restore(t)
	var buf bytes.Buffer
	Init(Options{Level: "chatty", Stderr: &buf, Getenv: env(nil)})
	out := buf.String()
	if !strings.Contains(out, "unknown log level") || !strings.Contains(out, "chatty") || !strings.Contains(out, "log_level=INFO") {
		t.Fatalf("output: %s", out)
	}
	buf.Reset()
	For("x").Debug("hidden")
	if buf.Len() != 0 {
		t.Fatal("falling back to info should hide debug")
	}
}

func TestLogFileIsWrittenAtDebugAndTraceOnly(t *testing.T) {
	for _, lvl := range []string{"debug", "trace"} {
		restore(t)
		dir := t.TempDir()
		closeFn, file := Init(Options{Level: lvl, Stderr: &bytes.Buffer{}, ConfigDir: dir, Getenv: env(nil)})
		For("x").Debug("to the file")
		closeFn()
		want := filepath.Join(dir, "logs", "d2-studio.log")
		if file != want {
			t.Fatalf("%s: file = %q, want %q", lvl, file, want)
		}
		b, err := os.ReadFile(want)
		if err != nil || !strings.Contains(string(b), "to the file") {
			t.Fatalf("%s: file = %q err=%v", lvl, b, err)
		}
	}

	restore(t)
	dir := t.TempDir()
	closeFn, file := Init(Options{Level: "info", Stderr: &bytes.Buffer{}, ConfigDir: dir, Getenv: env(nil)})
	closeFn()
	if file != "" {
		t.Fatalf("no log file should be written at info, got %q", file)
	}
	if _, err := os.Stat(filepath.Join(dir, "logs")); err == nil {
		t.Fatal("the logs folder should not even be created at info")
	}
}

func TestExplicitLogFileWorksAtAnyLevel(t *testing.T) {
	restore(t)
	path := filepath.Join(t.TempDir(), "custom.log")
	closeFn, file := Init(Options{Level: "info", Stderr: &bytes.Buffer{}, Getenv: env(map[string]string{EnvFile: path})})
	For("x").Warn("kept")
	closeFn()
	b, _ := os.ReadFile(path)
	if file != path || !strings.Contains(string(b), "kept") {
		t.Fatalf("file=%q content=%q", file, b)
	}
}

func TestUnwritableLogFileFallsBackToTheConsole(t *testing.T) {
	restore(t)
	blocker := filepath.Join(t.TempDir(), "file")
	if err := os.WriteFile(blocker, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	var buf bytes.Buffer
	// A path whose parent is a regular file cannot be created.
	closeFn, file := Init(Options{Level: "debug", Stderr: &buf, Getenv: env(map[string]string{EnvFile: filepath.Join(blocker, "sub", "x.log")})})
	defer closeFn()
	if file != "" {
		t.Fatalf("file = %q", file)
	}
	if !strings.Contains(buf.String(), "cannot write the log file") {
		t.Fatalf("the failure should be reported: %s", buf.String())
	}
	buf.Reset()
	For("x").Info("still logging")
	if !strings.Contains(buf.String(), "still logging") {
		t.Fatal("logging must continue on the console")
	}
}

func TestLargeLogIsRotatedAtStart(t *testing.T) {
	path := filepath.Join(t.TempDir(), "logs", "d2-studio.log")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, bytes.Repeat([]byte("x"), maxLogBytes+1), 0o644); err != nil {
		t.Fatal(err)
	}
	f, err := openLogFile(path)
	if err != nil {
		t.Fatal(err)
	}
	f.Close()
	if info, err := os.Stat(path + ".1"); err != nil || info.Size() != maxLogBytes+1 {
		t.Fatalf("the old log should be kept as .1: %v %v", info, err)
	}
	if info, _ := os.Stat(path); info.Size() != 0 {
		t.Fatalf("the new log should start empty, has %d bytes", info.Size())
	}
}

func TestLogsDefaultLoggerAtCallTime(t *testing.T) {
	// A Log made before Init must still honour a later Init.
	restore(t)
	early := For("early")
	var buf bytes.Buffer
	Init(Options{Level: "debug", Stderr: &buf, Getenv: env(nil)})
	buf.Reset()
	early.Debug("late binding")
	if !strings.Contains(buf.String(), "late binding") {
		t.Fatal("a package-level Log should use whatever logger is current")
	}
}

// ---- formats ----

func TestConsoleDefaultsToTextAndFileToJSON(t *testing.T) {
	restore(t)
	var console bytes.Buffer
	dir := t.TempDir()
	closeFn, file := Init(Options{Level: "debug", Stderr: &console, ConfigDir: dir, Getenv: env(nil)})
	For("files").Debug("opened", "path", "/w/a.d2", "bytes", 42)
	closeFn()

	if !strings.Contains(console.String(), "msg=opened") || strings.Contains(console.String(), `"msg"`) {
		t.Errorf("console should be human-readable text: %s", console.String())
	}
	data, err := os.ReadFile(file)
	if err != nil {
		t.Fatal(err)
	}
	var found bool
	for _, line := range strings.Split(strings.TrimSpace(string(data)), "\n") {
		var rec map[string]any
		if err := json.Unmarshal([]byte(line), &rec); err != nil {
			t.Fatalf("every file line should be a JSON object: %q: %v", line, err)
		}
		if rec["msg"] == "opened" {
			found = true
			if rec["component"] != "files" || rec["path"] != "/w/a.d2" || rec["bytes"] != float64(42) || rec["level"] != "DEBUG" || rec["time"] == nil {
				t.Errorf("record = %v", rec)
			}
		}
	}
	if !found {
		t.Fatalf("the record is missing from the file: %s", data)
	}
}

func TestFormatsAreIndependentAndConfigurable(t *testing.T) {
	cases := []struct {
		name                      string
		opts                      Options
		environ                   map[string]string
		consoleIsJSON, fileIsJSON bool
	}{
		{"defaults", Options{}, nil, false, true},
		{"both text", Options{Format: "text", FileFormat: "text"}, nil, false, false},
		{"both json", Options{Format: "json", FileFormat: "json"}, nil, true, true},
		{"json console, text file", Options{Format: "json", FileFormat: "text"}, nil, true, false},
		{"environment wins", Options{Format: "text", FileFormat: "json"}, map[string]string{EnvFormat: "json", EnvFileFormat: "text"}, true, false},
		{"case-insensitive", Options{Format: "JSON", FileFormat: "Text"}, nil, true, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			restore(t)
			var console bytes.Buffer
			c.opts.Level, c.opts.Stderr, c.opts.ConfigDir = "debug", &console, t.TempDir()
			c.opts.Getenv = env(c.environ)
			closeFn, file := Init(c.opts)
			console.Reset()
			For("x").Debug("probe")
			closeFn()
			data, _ := os.ReadFile(file)
			lastLine := func(b []byte) string {
				lines := strings.Split(strings.TrimSpace(string(b)), "\n")
				return lines[len(lines)-1]
			}
			isJSON := func(s string) bool { var m map[string]any; return json.Unmarshal([]byte(s), &m) == nil }
			if got := isJSON(lastLine(console.Bytes())); got != c.consoleIsJSON {
				t.Errorf("console json=%v, want %v: %s", got, c.consoleIsJSON, console.String())
			}
			if got := isJSON(lastLine(data)); got != c.fileIsJSON {
				t.Errorf("file json=%v, want %v: %s", got, c.fileIsJSON, data)
			}
		})
	}
}

func TestJSONOutputNamesTraceAndKeepsStructure(t *testing.T) {
	restore(t)
	var buf bytes.Buffer
	Init(Options{Level: "trace", Format: "json", Stderr: &buf, Getenv: env(nil)})
	buf.Reset()
	For("workspace").Trace("scanned", "files", 7, "took_ms", 3)
	var rec map[string]any
	if err := json.Unmarshal(buf.Bytes(), &rec); err != nil {
		t.Fatalf("%v: %s", err, buf.String())
	}
	if rec["level"] != "TRACE" || rec["component"] != "workspace" || rec["files"] != float64(7) || rec["took_ms"] != float64(3) {
		t.Fatalf("record = %v", rec)
	}
}

func TestUnknownFormatsWarnAndFallBackToTheDefaults(t *testing.T) {
	restore(t)
	var console bytes.Buffer
	dir := t.TempDir()
	closeFn, file := Init(Options{Level: "debug", Format: "yaml", FileFormat: "xml", Stderr: &console, ConfigDir: dir, Getenv: env(nil)})
	For("x").Info("after")
	closeFn()
	out := console.String()
	if !strings.Contains(out, "unknown log format") || !strings.Contains(out, "yaml") ||
		!strings.Contains(out, "unknown log file format") || !strings.Contains(out, "xml") {
		t.Fatalf("both bad values should be reported: %s", out)
	}
	if !strings.Contains(out, "msg=after") {
		t.Errorf("console should fall back to text: %s", out)
	}
	data, _ := os.ReadFile(file)
	last := data[bytes.LastIndexByte(bytes.TrimSpace(data), '\n')+1:]
	var rec map[string]any
	if err := json.Unmarshal(bytes.TrimSpace(last), &rec); err != nil {
		t.Errorf("the file should fall back to json: %q", last)
	}
}

func TestParseFormat(t *testing.T) {
	for in, want := range map[string]string{"text": "text", "JSON": "json", " json ": "json", "": ""} {
		if got, ok := ParseFormat(in); !ok || got != want {
			t.Errorf("ParseFormat(%q) = %q, %v", in, got, ok)
		}
	}
	if _, ok := ParseFormat("yaml"); ok {
		t.Error("yaml is not a format")
	}
}

// A record's own "level" key must not be repeated by an attribute: JSON with a
// duplicate key is ambiguous and tools may keep either value.
func TestStartupRecordHasNoDuplicateKeys(t *testing.T) {
	restore(t)
	var buf bytes.Buffer
	Init(Options{Level: "trace", Format: "json", Stderr: &buf, Getenv: env(nil)})
	line := strings.TrimSpace(buf.String())
	dec := json.NewDecoder(strings.NewReader(line))
	if _, err := dec.Token(); err != nil { // the opening brace
		t.Fatal(err)
	}
	seen := map[string]bool{}
	for dec.More() {
		k, err := dec.Token()
		if err != nil {
			t.Fatal(err)
		}
		key := k.(string)
		if seen[key] {
			t.Fatalf("duplicate key %q in %s", key, line)
		}
		seen[key] = true
		var v any
		if err := dec.Decode(&v); err != nil {
			t.Fatal(err)
		}
	}
	if !seen["log_level"] || !seen["level"] {
		t.Fatalf("expected both the record level and log_level: %s", line)
	}
}
