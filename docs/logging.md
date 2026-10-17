# Logging

D2 Studio logs through Go's standard `log/slog`, with one extra level, **trace**.
Each line says which part of the app wrote it (`component=files`, `compiler`,
`workspace`, `export`, `menu`, `settings`, `main`).

## Levels

From quietest to noisiest:

| Level | What it records |
|---|---|
| `error` | Something failed in a way the user will notice. |
| `warn` | Something unexpected that the app recovered from. |
| `info` | The default. Startup, and little else. |
| `debug` | Notable operations: a file opened or saved, a compile finished, an export written, an external change detected, the menu rebuilt. |
| `trace` | Fine detail: every poll, every compile request, every image inlined. Very noisy. |

## Turning it up

Either of these (the environment variable wins):

- Environment: `D2STUDIO_LOG=debug` (or `trace`), e.g. `D2STUDIO_LOG=trace wails3 dev`.
- `settings.json` in the app's config folder: `"logLevel": "debug"`.
  (`~/Library/Application Support/D2 Studio` on macOS, `%AppData%\D2 Studio` on
  Windows, `~/.config/D2 Studio` on Linux.) This is how to enable it for a
  packaged build that has no terminal.

An unknown value is reported in the log and replaced by `info`.

## Formats

There are two sinks, each with its own format:

| Sink | Default | Why |
|---|---|---|
| Console (stderr) | `text` | `key=value` lines, easy for a person to read and `tail`. |
| Log file | `json` | One JSON object per line, for tools that collect and analyse logs. |

Change them with `D2STUDIO_LOG_FORMAT` / `D2STUDIO_LOG_FILE_FORMAT` (`text` or
`json`), or `"logFormat"` / `"logFileFormat"` in `settings.json`.

## The log file

At `debug` and `trace` the log is also written to
`<config folder>/logs/d2-studio.log`. At the start of a run, a file over 5 MB is
moved to `d2-studio.log.1` (one previous copy is kept). Set `D2STUDIO_LOG_FILE`
to write a file at any level, or to choose where it goes.

Handy ways to read it:

```sh
tail -f ~/Library/Application\ Support/D2\ Studio/logs/d2-studio.log | jq -c .
jq -c 'select(.component=="compiler" and .took_ms > 200)' d2-studio.log
```

## Privacy

Logs may be attached to bug reports, so they record **what the app did, never
what is in your documents**: sizes, timings, settings, and file paths, but not
diagram source, imported file contents, or labels. A test
(`TestTraceLoggingNeverContainsDocumentText`) plants a marker in a diagram and
checks it never reaches the log at the most verbose level. Keep that true when
adding log calls: log `len(source)`, not `source`.

## For developers

```go
var logger = logging.For("files")

logger.Debug("opened", "path", path, "bytes", n)
logger.Trace("poll: no change", "path", path)

if logger.Enabled(logging.LevelTrace) { /* only build costly arguments when needed */ }
```

Avoid an attribute named `level`, `time`, `msg` or `component`: they are used by
the record itself.
