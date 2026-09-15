# Build D2 Studio — A Cross-Platform D2 Diagram Editor

Build a cross-platform, local desktop application called **D2 Studio**, a dedicated text editor and live-preview IDE for the [D2 diagramming language](https://d2lang.com/).

The experience should feel like a modern, lightweight Markdown editor such as Typora or Obsidian, tailored to software architecture diagrams. Prioritize fast feedback, a polished interface, reliable file handling, and minimal operational complexity.

The application must run locally without a remote service, browser-based account, separately installed D2 executable, or network connection required for normal editing and rendering.

## 1. Technology Stack

Use the following architecture:

- **Desktop framework:** Wails. Select and pin a stable version before implementation; use that version consistently for the Go application, runtime, dialogs, and generated bindings.
- **Backend:** Go, integrating directly with D2's Go compiler/library APIs.
- **Frontend:** TypeScript, React, and Vite.
- **Editor:** CodeMirror 6, with a custom D2 language extension.
- **Styling:** Tailwind CSS.
- **Preview:** SVG rendered in the frontend, with client-side pan and zoom.

Use the fewest technologies necessary to deliver a high-quality experience. Do not introduce Electron, WebAssembly, a separate D2 subprocess, a remote service, or an additional backend language.

### D2 compiler integration

Before implementing the bridge, inspect the pinned D2 module's actual public APIs, compilation pipeline, layout-engine configuration, theme configuration, and error types.

Do not assume example API names or signatures are correct without verification.

Create a small Go compilation service that:

1. Accepts D2 source and compilation options.
2. Invokes the existing D2 compilation pipeline directly.
3. Returns generated SVG, structured diagnostics, and compilation metadata.
4. Preserves useful error information, including source locations when available.
5. Supports cancellation or stale-result suppression for superseded compilations.
6. Can be tested independently of the UI.

Pin the D2 dependency and document the upgrade procedure. No separately installed D2 executable should be required.

## 2. Editor and View Modes

Provide a toolbar toggle and keyboard shortcuts for three modes:

- **Source:** Full-width CodeMirror editor with line numbers, syntax highlighting, code folding, and editor navigation.
- **Split:** Default mode, with a resizable source pane and live SVG preview.
- **Preview:** Full-width rendered diagram suitable for inspecting or presenting diagrams.

Persist the user's preferred view mode and split-pane ratio.

Implement a D2 language extension supporting comments, strings, keys, shape declarations, containers, styles, classes, edge operators (`->`, `--`), and relevant D2 syntax. Add context-aware completion where practical.

## 3. Live Compilation and Diagnostics

- Debounce source changes by approximately 100–150 ms before starting compilation.
- Keep the UI responsive while compilation runs.
- Prevent outdated compilation results from replacing newer results.
- Retain the last successfully rendered diagram when the current source fails to compile.
- Display non-blocking diagnostics with the compiler's actual error message and source location when available.
- Show compilation states: Idle, Compiling, Success, and Error.

Treat the debounce interval as an interaction target, not a guarantee that every diagram compiles within 150 ms.

## 4. Layout Engines and Themes

Provide selectors for the layout engines and themes supported by the pinned D2 version.

- Support Dagre and ELK when available.
- Expose TALA only when the packaged build supports it and its licensing and distribution requirements have been addressed.
- Populate theme options from verified D2 theme definitions rather than assuming display names or IDs.
- Preserve the user's selected options between compilations.
- Report unavailable engines or invalid options explicitly. Never silently substitute a different engine.

Where appropriate, allow D2 source configuration to take precedence or clearly explain how source-level settings interact with UI selections.

## 5. Interactive SVG Preview

Implement:

- Smooth zooming, including mouse-wheel zoom and fit-to-view.
- Click-and-drag panning.
- Zoom percentage and reset controls.
- Correct handling of SVG dimensions, viewBox, and scaling.
- Sensible behavior for large diagrams.

Support optional shape-to-source navigation only if a reliable source mapping can be implemented. Clicking a diagram element should then focus the corresponding editor location.

## 6. Export

Implement export through a clearly defined rendering pipeline:

- **SVG:** Export the generated SVG directly.
- **PNG:** Convert the generated SVG to a raster image using a maintained library that works in packaged builds.
- **PDF:** Use a verified SVG-to-PDF or equivalent conversion approach compatible with all supported platforms.

Use native save dialogs where appropriate. Do not report successful exports when conversion or file writing fails.

If an export format requires additional dependencies or platform-specific work, document that explicitly.

## 7. Files and Workspaces

Implement:

- Open, Save, and Save As for `.d2` files.
- Native file dialogs.
- Unsaved-change indicators and confirmation before discarding changes.
- Optional autosave with a clear, configurable behavior.
- Recent files.
- A lightweight, navigable workspace file tree.
- Multi-file project support consistent with D2 imports and relative paths.
- External file-change detection, with explicit reload and keep-local-changes options.

External file modifications must never silently overwrite unsaved editor content. Handle deleted, renamed, or temporarily inaccessible files gracefully.

## 8. UI and UX

Create a clean, minimalist interface with system-aware light and dark themes.

The layout should include:

- A compact toolbar for file operations and view modes.
- A resizable source/preview split.
- An optional collapsible workspace sidebar.
- A status bar displaying the active layout engine, theme, zoom level, compilation status, and file encoding where meaningful.

Use consistent spacing, typography, keyboard navigation, accessible contrast, and appropriate loading and error states. Avoid unnecessary modal dialogs and visual clutter.

## 9. Project Structure and Deliverables

First inspect the current repository and existing D2 APIs. If starting from an empty repository, create a conventional Wails project structure.

Provide:

1. A project directory tree.
2. The Go application entry point and Wails configuration.
3. The D2 compiler service and its configuration types.
4. The React split-view editor with CodeMirror 6 integration.
5. The live compilation and SVG preview pipeline.
6. Required TypeScript bindings and frontend dependencies.
7. File operations and native dialog integration.
8. Tests for compilation, diagnostics, stale-result handling, and file operations.
9. Build and development instructions for macOS, Windows, and Linux.

Do not present pseudocode, placeholder implementations, or invented API calls as completed functionality. Clearly label any features deferred from the initial implementation.

## 10. Implementation Order and Acceptance Criteria

Implement in stages, keeping the project buildable at each stage.

**Stage 1 — Working vertical slice**
- Launch the desktop application.
- Edit D2 source in CodeMirror.
- Compile through the Go library.
- Display generated SVG in split view.
- Preserve the previous successful preview on compilation errors.
- Demonstrate the basic workflow using a sample diagram.

**Stage 2 — Editing experience**
- Add view modes, resizing, syntax highlighting, themes, layout-engine selection, diagnostics, and preview navigation.

**Stage 3 — Files and export**
- Add open/save operations, recent files, workspace navigation, external-change detection, and SVG/PNG/PDF export.

**Stage 4 — Packaging and validation**
- Verify native builds and runtime behavior on macOS, Windows, and Linux.
- Document required platform toolchains and system dependencies.
- Run tests and provide exact commands for development, testing, and release builds.

Begin by verifying the D2 API and creating Stage 1. Explain important implementation decisions briefly, then provide complete, runnable code for each file you introduce or modify. Do not attempt to implement all stages as a single unverified code dump.