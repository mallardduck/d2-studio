# Images in diagrams

Diagrams can show images, for example `icon: ./logo.png` or
`icon: https://icons.example.com/db.svg`. The preview and the exports handle
them the same way, and with the same rules, because a `.d2` file can come from
anywhere and should not be able to read your files or contact arbitrary
servers just by being opened.

## Local images

A relative path is relative to the `.d2` file. Local images are only read from
inside the **workspace folder** (when the file is in it) or **the file's own
folder**. Anything outside, including through a symlink or a `..` path, is
refused with a message saying so. Open the containing folder as the workspace
to allow it.

Reading goes through `os.Root`, which enforces this as each path component is
opened, so there is no moment in which a symlink could be swapped after a
check. Absolute symlink targets are refused even when they point inside the
folder; use relative symlinks.

An unsaved document has no folder, so local images cannot be found until it is
saved.

## Remote images

Remote (http/https) images are **off by default**. Opening a file never makes a
network request.

- **Preview:** turn them on per file with **View ▸ Show Remote Images in This
  File**, or the **Show** button that appears in the status bar when images are
  hidden. The choice is remembered for that file (not for others). For an
  unsaved document it lasts for the session and is kept when you first save.
- **Export (PNG and PDF):** **Export ▸ Fetch Remote Images**. SVG export keeps
  the links and never downloads anything.

When remote images are fetched, these limits apply:

| | |
|---|---|
| Schemes | `http` and `https` only |
| Networks | Loopback, private (10/8, 172.16/12, 192.168/16), link-local (including cloud metadata addresses), carrier-grade NAT and multicast addresses are refused. The check happens when connecting, so redirects and DNS tricks cannot get around it. |
| Size | 2 MB per image |
| Time | 10 seconds per image, at most 3 redirects |
| Content | Must be an image. The bytes are checked, not just the `Content-Type` header. |
| Caching | Kept in memory for 10 minutes (32 MB at most). A failure is remembered for 60 seconds so a dead URL is not retried on every keystroke. |
| Privacy | URLs are shown and logged without their query string or credentials. |

Proxies are not used for these fetches (the private-network check needs direct
connections). On a network that only allows access through a proxy, remote
images will not load in the preview.

## When an image cannot be loaded

The diagram still renders. The problem appears as a **warning** in the list
under the editor, naming the image and the reason, and the other images are
unaffected.
