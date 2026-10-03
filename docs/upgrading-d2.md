# Upgrading D2

D2 is pinned in `go.mod` (`github.com/d2lang/d2`). It is a library dependency:
the app compiles, lays out, rasterizes and writes PDFs in-process, and needs no
separately installed `d2` executable.

Upgrade deliberately, not with a blanket `go get -u`.

## Steps

1. **Read the release notes** for the new version, especially anything about
   layout engines, themes, imports, image handling, PNG/PDF export or the
   Go API.
2. **Bump the version**
   ```sh
   go get github.com/d2lang/d2@vX.Y.Z
   go mod tidy
   ```
   The new D2 may require a newer Go toolchain; its `go.mod` says which.
3. **Re-check the code that mirrors D2 internals.** These are the places that
   copy or depend on behaviour that is not a stable public API:
   - `export/limits.go` copies the limits and settings from
     `d2cli/raster.go`. Diff it against the new version's file and update the
     constants and option structs to match.
   - `export/export.go` follows `d2cli`'s scene build, rasterize and PDF flow
     (`d2scenebuild.Build`, `d2raster.Render`, `lib/pdf`).
   - `compiler/engines.go` lists the layout plugins bundled into D2 and
     initializes the Dagre plugin's default options (D2's CLI does this from
     flag defaults; the plugin starts with nil options).
   - `compiler/compiler.go` mirrors the CLI's compile flow, including the
     `d2plugin.FeatureSupportCheck`.
   - `compiler/language.go` reads keywords, shapes, arrowheads and the
     completion engine (`d2lsp`) from D2.
   - `compiler/rootfs.go` supplies the filesystem D2 reads imports through.
4. **Run everything**
   ```sh
   go vet ./...
   go test -race ./...
   (cd frontend && npx tsc --noEmit && npx vitest run)
   ```
   The compiler and export tests exercise real diagrams on every engine, so
   most incompatibilities fail loudly. A new layout engine or theme appears in
   the app automatically (they come from D2's own lists).
5. **Regenerate frontend bindings** if any Go service signatures changed:
   `wails3 generate bindings -clean=true -ts -i`.
6. **Try it** in `wails3 dev`: open a diagram, switch engines and themes,
   import a file, and export SVG, PNG and PDF.

## Licensing

D2 is MPL-2.0. Since 0.9.0 it bundles TALA (also MPL-2.0), so TALA is offered
like any other engine and needs no license key. Keep D2's `LICENSE.txt` and
`THIRD_PARTY_NOTICES.txt` with any distributed build.
