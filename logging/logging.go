// Package logging is the app's structured logging, built on log/slog with one
// extra level, Trace, for very detailed output.
//
// Levels, from quietest: error, warn, info (the default), debug, trace.
// Debug records notable operations (a file opened, a compile finished).
// Trace records fine detail (each poll, each decision) and is noisy.
//
// Turn it up with the D2STUDIO_LOG environment variable (for example
// D2STUDIO_LOG=debug) or the "logLevel" field of settings.json. At debug or
// trace the log is also written to a file.
//
// The console and the file are separate sinks with their own format, "text"
// (key=value lines, for a person tailing the output) or "json" (one object
// per line, for tools to collect and analyse). The console defaults to text
// and the file to json; set D2STUDIO_LOG_FORMAT and D2STUDIO_LOG_FILE_FORMAT,
// or "logFormat" and "logFileFormat" in settings.json, to change them.
//
// Document contents are never logged. Log arguments may include file paths
// and sizes, never the source text.
package logging

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
)

// LevelTrace is below slog's Debug.
const LevelTrace = slog.Level(-8)

// The environment variables that control logging.
const (
	EnvLevel      = "D2STUDIO_LOG"
	EnvFormat     = "D2STUDIO_LOG_FORMAT"
	EnvFile       = "D2STUDIO_LOG_FILE"
	EnvFileFormat = "D2STUDIO_LOG_FILE_FORMAT"
)

// Output formats.
const (
	FormatText = "text"
	FormatJSON = "json"
)

// maxLogBytes is the size at which the previous log is rotated away at start.
const maxLogBytes = 5 << 20

var level slog.LevelVar

// ParseLevel understands "trace", "debug", "info", "warn" and "error" in any
// case. The second result is false for anything else.
func ParseLevel(s string) (slog.Level, bool) {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "trace":
		return LevelTrace, true
	case "debug":
		return slog.LevelDebug, true
	case "info", "":
		return slog.LevelInfo, true
	case "warn", "warning":
		return slog.LevelWarn, true
	case "error":
		return slog.LevelError, true
	}
	return slog.LevelInfo, false
}

// ParseFormat understands "text" and "json" in any case. The second result is
// false for anything else (the empty string is accepted as "use the default").
func ParseFormat(s string) (format string, ok bool) {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case FormatText:
		return FormatText, true
	case FormatJSON:
		return FormatJSON, true
	case "":
		return "", true
	}
	return "", false
}

// LevelName is the display name of a level, including Trace.
func LevelName(l slog.Level) string {
	if l == LevelTrace {
		return "TRACE"
	}
	return l.String()
}

// Options configure Init. The environment variables take precedence over the
// corresponding fields, which normally come from settings.json.
type Options struct {
	// Level is the level name; see ParseLevel. Empty means info.
	Level string
	// Format is the console format, "text" (the default) or "json".
	Format string
	// FileFormat is the log file format, "json" (the default) or "text".
	FileFormat string
	// Stderr is where the console sink writes. Defaults to os.Stderr.
	Stderr io.Writer
	// ConfigDir is where the default log file lives:
	// <ConfigDir>/logs/d2-studio.log. Empty means no default file.
	ConfigDir string
	// Getenv reads environment variables. Defaults to os.Getenv.
	Getenv func(string) string
}

// Init installs the default logger. It returns a function that closes the log
// file, and the path of that file ("" when none is written). It never fails:
// a bad option is reported and replaced by its default, and a log file that
// cannot be opened is reported while logging continues on the console.
func Init(o Options) (closeFn func(), logFile string) {
	if o.Stderr == nil {
		o.Stderr = os.Stderr
	}
	if o.Getenv == nil {
		o.Getenv = os.Getenv
	}
	pick := func(env, configured string) (value, source string) {
		if v := o.Getenv(env); v != "" {
			return v, env
		}
		return configured, "settings"
	}

	levelName, levelFrom := pick(EnvLevel, o.Level)
	lvl, levelOK := ParseLevel(levelName)
	level.Set(lvl)

	consoleName, consoleFrom := pick(EnvFormat, o.Format)
	consoleFormat, consoleOK := ParseFormat(consoleName)
	if consoleFormat == "" {
		consoleFormat = FormatText
	}
	fileName, fileFrom := pick(EnvFileFormat, o.FileFormat)
	fileFormat, fileFmtOK := ParseFormat(fileName)
	if fileFormat == "" {
		fileFormat = FormatJSON
	}

	// The file is written at debug and trace, or when explicitly requested.
	path := o.Getenv(EnvFile)
	if path == "" && lvl <= slog.LevelDebug && o.ConfigDir != "" {
		path = filepath.Join(o.ConfigDir, "logs", "d2-studio.log")
	}
	var file *os.File
	var fileErr error
	if path != "" {
		if file, fileErr = openLogFile(path); fileErr == nil {
			logFile = path
		}
	}

	handlers := []slog.Handler{newHandler(o.Stderr, consoleFormat)}
	if file != nil {
		handlers = append(handlers, newHandler(file, fileFormat))
	}
	var handler slog.Handler = handlers[0]
	if len(handlers) > 1 {
		handler = slog.NewMultiHandler(handlers...)
	}
	slog.SetDefault(slog.New(handler))

	log := For("logging")
	if !levelOK {
		log.Warn("unknown log level; using info", "value", levelName, "from", levelFrom, "valid", "trace, debug, info, warn, error")
	}
	if !consoleOK {
		log.Warn("unknown log format; using text", "value", consoleName, "from", consoleFrom, "valid", "text, json")
	}
	if !fileFmtOK {
		log.Warn("unknown log file format; using json", "value", fileName, "from", fileFrom, "valid", "text, json")
	}
	if fileErr != nil {
		log.Warn("cannot write the log file; logging to the console only", "path", path, "err", fileErr)
	}
	log.Info("logging started", "log_level", LevelName(lvl), "console_format", consoleFormat,
		"file", logFile, "file_format", fileFormat)

	return func() {
		if file != nil {
			_ = file.Close()
		}
	}, logFile
}

func newHandler(w io.Writer, format string) slog.Handler {
	opts := &slog.HandlerOptions{
		Level: &level,
		// Show Trace by name instead of slog's "DEBUG-4".
		ReplaceAttr: func(_ []string, a slog.Attr) slog.Attr {
			if a.Key == slog.LevelKey {
				if l, ok := a.Value.Any().(slog.Level); ok {
					a.Value = slog.StringValue(LevelName(l))
				}
			}
			return a
		},
	}
	if format == FormatJSON {
		return slog.NewJSONHandler(w, opts)
	}
	return slog.NewTextHandler(w, opts)
}

// SetLevel changes the level while the app runs.
func SetLevel(l slog.Level) { level.Set(l) }

// openLogFile opens path for appending, first moving an oversized earlier log
// aside (keeping one previous generation).
func openLogFile(path string) (*os.File, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, err
	}
	if info, err := os.Stat(path); err == nil && info.Size() > maxLogBytes {
		if err := os.Rename(path, path+".1"); err != nil {
			return nil, fmt.Errorf("rotate log: %w", err)
		}
	}
	return os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
}

// Log is a logger for one component of the app. It always uses the current
// default logger, so it works regardless of when Init ran.
type Log struct{ component string }

// For returns the logger for a component such as "files" or "compiler".
func For(component string) Log { return Log{component: component} }

func (l Log) logger() *slog.Logger { return slog.Default().With("component", l.component) }

func (l Log) log(lvl slog.Level, msg string, args []any) {
	lg := l.logger()
	if lg.Enabled(context.Background(), lvl) {
		lg.Log(context.Background(), lvl, msg, args...)
	}
}

// Enabled reports whether messages at lvl would be written, so callers can
// skip building expensive arguments.
func (l Log) Enabled(lvl slog.Level) bool { return l.logger().Enabled(context.Background(), lvl) }

func (l Log) Trace(msg string, args ...any) { l.log(LevelTrace, msg, args) }
func (l Log) Debug(msg string, args ...any) { l.log(slog.LevelDebug, msg, args) }
func (l Log) Info(msg string, args ...any)  { l.log(slog.LevelInfo, msg, args) }
func (l Log) Warn(msg string, args ...any)  { l.log(slog.LevelWarn, msg, args) }
func (l Log) Error(msg string, args ...any) { l.log(slog.LevelError, msg, args) }
