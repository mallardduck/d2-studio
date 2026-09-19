// Package compiler wraps D2's in-process compilation pipeline (d2lib, bundled
// layout plugins, d2svg) behind a small API that is independent of the UI.
package compiler

// Options are the user's UI selections. Zero values mean "not chosen": the
// D2 source's own d2-config (or D2's default) applies. When a value is set it
// takes precedence over the source, because D2 only fills unset fields from
// the source config.
type Options struct {
	// Layout is a layout engine ID from Engines(), or "" for source/default.
	Layout string `json:"layout"`
	// ThemeID is a D2 theme ID from Themes(), or nil for source/default.
	ThemeID *int64 `json:"themeId"`
	// Sketch renders hand-drawn style. nil defers to source/default.
	Sketch *bool `json:"sketch"`
}

// Request is one compilation request.
type Request struct {
	Source  string  `json:"source"`
	Options Options `json:"options"`
	// Path is the file being edited, used for error locations and, in later
	// stages, relative imports. May be empty for unsaved buffers.
	Path string `json:"path"`
}

type Severity string

const (
	SeverityError Severity = "error"
)

// Diagnostic is a compiler message. Line/Column are 1-based; From/To are
// UTF-16 offsets into the source (matching JavaScript string indexes) and are
// only meaningful when HasLocation is true.
type Diagnostic struct {
	Severity    Severity `json:"severity"`
	Message     string   `json:"message"`
	HasLocation bool     `json:"hasLocation"`
	Line        int      `json:"line"`
	Column      int      `json:"column"`
	EndLine     int      `json:"endLine"`
	EndColumn   int      `json:"endColumn"`
	From        int      `json:"from"`
	To          int      `json:"to"`
	// Path is the file the diagnostic refers to when it differs from the
	// active file (for example, an error inside an import).
	Path string `json:"path"`
}

// Meta describes how a compilation ran.
type Meta struct {
	DurationMs int64 `json:"durationMs"`
	// Effective settings actually used.
	Layout  string `json:"layout"`
	ThemeID int64  `json:"themeId"`
	// LayoutSource and ThemeSource explain where the effective setting came
	// from: "ui" (selected in the app), "source" (d2-config in the .d2 file)
	// or "default" (D2's default).
	LayoutSource string `json:"layoutSource"`
	ThemeSource  string `json:"themeSource"`
	// SourceLayout and SourceThemeID are what the .d2 file's d2-config
	// declares, whether or not a UI selection overrides them. Empty / nil
	// when the source does not set them.
	SourceLayout  string `json:"sourceLayout"`
	SourceThemeID *int64 `json:"sourceThemeId"`
	Width         int    `json:"width"`
	Height        int    `json:"height"`
}

// Status of a compilation.
type Status string

const (
	StatusSuccess Status = "success"
	StatusError   Status = "error"
	// StatusStale means a newer request superseded this one. SVG and
	// Diagnostics are empty and the caller must discard the result.
	StatusStale Status = "stale"
)

// Result is the outcome of Compile.
type Result struct {
	Seq         uint64       `json:"seq"`
	Status      Status       `json:"status"`
	SVG         string       `json:"svg"`
	Diagnostics []Diagnostic `json:"diagnostics"`
	Meta        Meta         `json:"meta"`
}

// Engine describes a layout engine.
type Engine struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Available   bool   `json:"available"`
	// Reason explains why an engine is unavailable.
	Reason string `json:"reason"`
}

// Theme describes a D2 theme from D2's own catalog.
type Theme struct {
	ID   int64  `json:"id"`
	Name string `json:"name"`
	Dark bool   `json:"dark"`
}
