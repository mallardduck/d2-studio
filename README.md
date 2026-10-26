# D2 Studio

A local desktop editor and live preview for [D2](https://d2lang.com) diagrams. Everything runs on your machine: D2 is compiled in, so there is no separate install, account or network connection needed to edit, preview and export.

> **Status: release candidate.** Usable and tested, but new; the macOS build has had the most use. See [Releases](../../releases) for downloads and known limitations.

## Features

- **Live preview** as you type: split, source-only and preview-only views, a resizable divider, and pan / zoom / fit.
- **D2 editor** (CodeMirror 6) with syntax highlighting, folding, context-aware completion from D2's own engine, and error underlines with the compiler's real messages. Your last good diagram stays on screen while the source has errors.
- **All of D2's layout engines** (Dagre, ELK, TALA) and every D2 theme, chosen from D2 itself; or let the file's own `d2-config` decide.
- **Files and folders:** open and save with conflict detection (it never overwrites changes made outside the app), recent files, a workspace sidebar, imports between files, autosave, and opening `.d2` files from the OS.
- **Export** SVG, PNG and PDF.
- **Images** in diagrams, local and (opt-in, per file) remote.
- Native menus and shortcuts, light and dark themes that follow your system.

## Download

Get the latest build from the [Releases](../../releases) page. Builds are not signed yet; the release notes explain how to open them on each OS.

## Build from source

You need Go 1.27+, Node 24, and the Wails CLI:

```sh
go install github.com/wailsapp/wails/v3/cmd/wails3@v3.0.0
git clone https://github.com/mallardduck/d2-studio && cd d2-studio
wails3 dev
```

Linux also needs the GTK 4 and WebKitGTK 6.0 development packages. See **[docs/building.md](docs/building.md)** for prerequisites on every platform, tests, packaging, cross-compiling and releases.

## Documentation

- [Building, testing and releasing](docs/building.md)
- [Images in diagrams](docs/images.md): what a diagram may load, and the safeguards
- [Logging](docs/logging.md): turning on debug and trace logs, and the log file
- [Upgrading D2](docs/upgrading-d2.md)

## Safety

Diagram files can come from anywhere, so opening one is designed to be safe: imports and local images are limited to the file's folder (or your workspace), remote images are off until you allow them for a file, previews are shown in a way that cannot run script, and unsaved work is never discarded or overwritten without asking.

## License

[Mozilla Public License 2.0](LICENSE). It bundles D2 (MPL-2.0) and other open-source components; see [THIRD_PARTY_NOTICES.md](THIRD_PARTY_NOTICES.md).
