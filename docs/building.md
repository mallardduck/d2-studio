# Building, testing and releasing

D2 Studio is a [Wails v3](https://v3.wails.io) app: a Go backend that compiles and renders D2 in-process, and a React + TypeScript frontend (CodeMirror 6, Tailwind, Vite) shown in the system webview.

## Prerequisites

Everywhere:

| Tool | Version | Notes |
|---|---|---|
| Go | **1.27** or newer (1.27.3+ recommended) | D2 v0.9.0 requires Go 1.27 |
| Node.js | 24 (what CI and development use) | with npm |
| Wails CLI | **v3.0.0** | `go install github.com/wailsapp/wails/v3/cmd/wails3@v3.0.0`, then `wails3 doctor` |

Per platform:

- **macOS:** Xcode command line tools (`xcode-select --install`). macOS 12 or newer to run.
- **Windows:** nothing for a plain build (no C compiler needed). The WebView2 runtime is needed to run (included with Windows 11 and current Windows 10). To make the installer, install [NSIS](https://nsis.sourceforge.io) so `makensis` is on your `PATH`.
- **Linux:** a C toolchain plus the GTK 4 and WebKitGTK 6.0 development packages (Wails v3's default stack; Ubuntu 24.04+ / Debian 13+):

  ```sh
  sudo apt install build-essential pkg-config libgtk-4-dev libwebkitgtk-6.0-dev
  ```

  Fedora: `gtk4-devel webkitgtk6.0-devel`. The built app needs `libgtk-4-1` and `libwebkitgtk-6.0-4` at run time.

## Develop

```sh
wails3 dev                      # runs the app with hot reload
D2STUDIO_LOG=debug wails3 dev   # with detailed logs (see logging.md)
```

The frontend's TypeScript bindings for the Go services are generated, not committed:

```sh
wails3 generate bindings -clean=true -ts -i
```

`wails3 dev` and the build tasks do this for you; run it by hand if your editor shows missing `bindings` imports.

## Test

```sh
go vet ./...
go test -race ./...

cd frontend
npm ci
npx tsc --noEmit
npx vitest run
```

Also:

```sh
go test ./menu -update                 # after changing the menu; refreshes frontend/src/menu/ids.fixture.json
go run ./tools/gennotices -check       # every third-party license is recognised
```

Several tests guard things that are easy to break silently (the menu's IDs matching the frontend, the build assets surviving regeneration, the CI workflows being pinned and read-only). If one fails, its message says what to do.

## Build and package

Build for the machine you are on:

| | Build | Package | Output in `bin/` |
|---|---|---|---|
| macOS | `wails3 task build` | `wails3 task package` | `d2-studio.app` (ad-hoc signed). `wails3 task darwin:package:universal` makes an Apple silicon + Intel build. |
| Windows | `wails3 task build` | `wails3 task package` | `d2-studio.exe`, and `d2-studio-<arch>-installer.exe` (needs NSIS) |
| Linux | `wails3 task build` | `wails3 task linux:create:deb` / `linux:create:appimage` | `d2-studio`, `.deb`, `.AppImage` |

`wails3 task` runs the tasks defined in `Taskfile.yml` and `build/`. Release builds are stripped and use the `production` tag.

### Cross-compiling

What has been verified from an Apple silicon Mac:

- **Windows** needs no C compiler, so it is a plain Go cross-compile, with no Docker:

  ```sh
  wails3 task build GOOS=windows ARCH=amd64    # or ARCH=arm64
  ```

- **Linux** needs cgo (GTK), so it builds inside the official Wails image. Pull it once (`docker pull ghcr.io/wailsapp/wails-cross:latest`; a few GB), then:

  ```sh
  docker run --rm -v "$PWD:/app" -v "$HOME/go/pkg/mod:/go/pkg/mod" \
    -e APP_NAME=d2-studio -e GOTOOLCHAIN=go1.27.3 \
    ghcr.io/wailsapp/wails-cross:latest linux arm64      # writes bin/d2-studio-linux-arm64
  ```

  Two things to know. The image's own Go is older than D2 needs, hence `GOTOOLCHAIN` (Go downloads and checks the newer toolchain). And the container builds for **its own CPU architecture only**: on an Apple silicon host that is `arm64`; asking for `amd64` fails with `gcc: unrecognized command-line option '-m64'`. For Linux x86-64, build on a Linux x86-64 machine or let CI do it.

- **macOS** cannot be built from other systems; it needs Apple's toolchain.

CI builds all three natively, which is the reliable route for releases.

## Continuous integration and releases

`.github/workflows/ci.yml` runs on every push to `main` and every pull request:

1. **Checks** (Linux): frontend type-check, tests and build; `go vet`; `go test -race`; the license check.
2. **Build** (Linux, Windows, macOS): packages the app, smoke-tests that it starts, and uploads the files as workflow artifacts.

A **release candidate** is made by pushing a tag:

```sh
git tag v0.1.0-rc.1
git push origin v0.1.0-rc.1
```

`.github/workflows/release.yml` then runs the same build and creates a GitHub release with the files, `SHA256SUMS.txt` and the notes from `.github/release-notes.md`. The tag's number must equal the version in `build/config.yml`.

**By default every release is a draft**: nothing is published until a person reviews it and clicks Publish. A tag with a suffix (`v0.1.0-rc.1`) makes it a pre-release.

**Optional: let release candidates publish themselves.** Once a few candidates have gone out cleanly, you can set the repository variable `AUTO_PUBLISH_PRERELEASES` to `true` (Settings ▸ Secrets and variables ▸ Actions ▸ Variables, or `gh variable set AUTO_PUBLISH_PRERELEASES --body true`). Then a pre-release tag is published automatically, as a pre-release and never as the "latest" release, but only if **all** of these hold (`scripts/release-gate.sh`):

- the tag has a suffix (**stable tags like `v0.1.0` are always drafts**);
- the variable is exactly `true`;
- every expected file was built (macOS zip, Windows installer and portable zip, Linux `.tar.gz` and `.deb`, and the checksums; the AppImage is optional);
- the smoke test passed on macOS, Windows and Linux.

Otherwise it stays a draft, and the run's summary says which condition failed. To turn auto-publishing off again, set the variable to anything else or delete it.

To check what a release would contain without CI, after packaging on your machine:

```sh
scripts/package-ci.sh macos v0.1.0-rc.1      # or windows / linux; writes dist/
scripts/smoke.sh bin/d2-studio.app/Contents/MacOS/d2-studio
```

Release builds are **not signed or notarized**. Signing needs certificates; the tasks `wails3 task darwin:sign`, `windows:sign` and `linux:sign:*` exist for when they are available.

## Changing the version or the product details

Edit `build/config.yml`, then regenerate the platform files:

```sh
wails3 task common:update:build-assets
```

Three cautions:

- The version also lives in `version.go` (`appVersion`). A test fails if they differ. It must be a plain `major.minor.patch`; release candidates are marked by the git tag.
- Regeneration **overwrites** some hand-made changes: the Linux package's license and MIME mapping (`build/linux/nfpm/nfpm.yaml`). A test (`build_assets_test.go`) fails and names what to restore. Review the diff (`git diff build/`) before committing.
- It also recreates `build/ios` and `build/android`; the task deletes them again, since mobile is not supported.

## Upgrading D2

See [upgrading-d2.md](upgrading-d2.md). Other docs: [logging.md](logging.md), [images.md](images.md).
