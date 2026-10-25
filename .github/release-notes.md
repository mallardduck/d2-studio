## D2 Studio release candidate

A local editor and live preview for [D2](https://d2lang.com) diagrams. This is a **pre-release for testing**. Please try it and report what you find.

Everything runs on your machine: D2 is built in, so there is nothing else to install, no account, and no network connection is needed to edit and render.

### Download

| Platform | File | Notes |
|---|---|---|
| macOS 12+ (Apple silicon and Intel) | `…-macos-universal.zip` | Unzip and move **D2 Studio.app** to Applications. |
| Windows 10/11 (64-bit) | `…-windows-amd64-setup.exe` | Installer; also associates `.d2` files. |
| | `…-windows-amd64-portable.zip` | No install; run `d2-studio.exe`. |
| Linux (64-bit) | `…-linux-amd64.deb` | Ubuntu 24.04+ / Debian 13+ (needs GTK 4 and WebKitGTK 6.0). |
| | `…-linux-amd64.AppImage` | If present; may need FUSE. |
| | `…-linux-amd64.tar.gz` | Just the binary and licenses. |

Check downloads against `SHA256SUMS.txt`.

### These builds are not signed

- **macOS:** the first time, right-click the app and choose **Open** (or allow it under *System Settings ▸ Privacy & Security ▸ Open Anyway*).
- **Windows:** SmartScreen may warn; choose **More info ▸ Run anyway**.
- There is no auto-update. Download newer builds from the releases page.

### What to try

- Edit D2 and watch the preview update; switch Source / Split / Preview (⌘/Ctrl+1–3).
- Open files (⌘/Ctrl+O), open a folder as a workspace (⇧⌘/Ctrl+Shift+O), and import files from each other (`...@other`).
- Double-click a `.d2` file, or use **Open With ▸ D2 Studio**.
- Export SVG, PNG and PDF (File ▸ Export).
- Menus on each platform, undo and redo, autosave, and quitting with unsaved changes.

### Known limitations

- Only the main board of a diagram is rendered; layers, scenarios and steps are not shown yet.
- The PDF export is an image on a page (text is not selectable). Use SVG for vector output.
- Remote images (such as web icons) are off until you allow them for a file (*View ▸ Show Remote Images in This File*).
- Not yet: indent guides, a sketch-mode toggle, a manual light/dark switch, clicking a shape to jump to its source.
- The Windows and Linux builds are new and have had less testing than macOS. Menus, file associations and window behaviour are the likeliest rough edges.

### Reporting a problem

Please include your OS and version, the D2 Studio version (it is in the log's first line), and what you did. For more detail, start the app with logging turned up:

```
D2STUDIO_LOG=debug   # or trace
```

The log is written to `logs/d2-studio.log` in the app's settings folder (`~/Library/Application Support/D2 Studio`, `%AppData%\D2 Studio`, or `~/.config/D2 Studio`). Logs describe what the app did, never the contents of your diagrams.

### License

D2 Studio is under the [Mozilla Public License 2.0](https://www.mozilla.org/MPL/2.0/). Third-party notices are included with every download.
