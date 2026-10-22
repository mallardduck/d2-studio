package main

import (
	"runtime"
	"runtime/debug"
	"strings"
)

// appVersion is the application version. It must match `version` in
// build/config.yml, which feeds the macOS bundle, the Windows file properties
// and the Linux packages; a test enforces that.
const appVersion = "0.1.0"

// buildInfo describes what is running, for the startup log line and bug reports.
type buildInfo struct {
	App      string
	Revision string // short VCS revision, "" when unknown
	Modified bool   // built from a tree with uncommitted changes
	D2       string
	Wails    string
	Go       string
	OS, Arch string
}

// readBuildInfo gathers build details from the binary itself.
func readBuildInfo() buildInfo {
	b := buildInfo{App: appVersion, Go: runtime.Version(), OS: runtime.GOOS, Arch: runtime.GOARCH}
	info, ok := debug.ReadBuildInfo()
	if !ok {
		return b
	}
	for _, s := range info.Settings {
		switch s.Key {
		case "vcs.revision":
			b.Revision = strings.TrimSpace(s.Value)
			if len(b.Revision) > 9 {
				b.Revision = b.Revision[:9]
			}
		case "vcs.modified":
			b.Modified = s.Value == "true"
		}
	}
	for _, d := range info.Deps {
		switch d.Path {
		case "github.com/d2lang/d2":
			b.D2 = d.Version
		case "github.com/wailsapp/wails/v3":
			b.Wails = d.Version
		}
	}
	return b
}
