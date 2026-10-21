import { Format } from "../../bindings/d2studio/export/models";

/** What a native menu item asks the app to do. */
export type MenuActionType =
  | { type: "new" }
  | { type: "open" }
  | { type: "openRecent"; path: string }
  | { type: "clearRecent" }
  | { type: "openFolder" }
  | { type: "closeFolder" }
  | { type: "save" }
  | { type: "saveAs" }
  | { type: "quit" }
  | { type: "autosave"; mode: "off" | "delay" | "blur"; delaySec: number }
  | { type: "export"; format: Format }
  | { type: "toggleRemoteImages" }
  | { type: "undo" }
  | { type: "redo" }
  | { type: "viewMode"; mode: "source" | "split" | "preview" }
  | { type: "toggleSidebar" }
  | { type: "toggleFileRemoteImages" }
  | { type: "layout"; engine: string }
  | { type: "theme"; id: number | null };

const SIMPLE: Record<string, MenuActionType> = {
  "file.new": { type: "new" },
  "file.open": { type: "open" },
  "file.openFolder": { type: "openFolder" },
  "file.closeFolder": { type: "closeFolder" },
  "file.save": { type: "save" },
  "file.saveAs": { type: "saveAs" },
  "file.quit": { type: "quit" },
  "file.recent.clear": { type: "clearRecent" },
  "export.remote": { type: "toggleRemoteImages" },
  "edit.undo": { type: "undo" },
  "edit.redo": { type: "redo" },
  "view.sidebar": { type: "toggleSidebar" },
  "view.remoteImages": { type: "toggleFileRemoteImages" },
};

const FORMATS: Record<string, Format> = { svg: Format.SVG, png: Format.PNG, pdf: Format.PDF };
const VIEW_MODES = ["source", "split", "preview"] as const;

/**
 * Parses a menu item ID (see the `menu` Go package) into an action. Returns
 * null for anything it does not recognise, so an unknown ID from a newer
 * backend does nothing instead of doing something wrong.
 */
export function parseAction(id: string): MenuActionType | null {
  if (id in SIMPLE) return SIMPLE[id];

  const recent = "file.recent:";
  if (id.startsWith(recent)) {
    const path = id.slice(recent.length);
    return path ? { type: "openRecent", path } : null;
  }

  if (id.startsWith("autosave:")) {
    const [, mode, sec] = id.split(":");
    if (mode === "off" || mode === "blur") return { type: "autosave", mode, delaySec: 2 };
    if (mode === "delay" && /^\d+$/.test(sec ?? "")) return { type: "autosave", mode, delaySec: Number(sec) };
    return null;
  }

  if (id.startsWith("export:")) {
    const format = FORMATS[id.slice("export:".length)];
    return format ? { type: "export", format } : null;
  }

  if (id.startsWith("view.mode:")) {
    const mode = id.slice("view.mode:".length);
    return (VIEW_MODES as readonly string[]).includes(mode)
      ? { type: "viewMode", mode: mode as (typeof VIEW_MODES)[number] }
      : null;
  }

  if (id.startsWith("layout:")) return { type: "layout", engine: id.slice("layout:".length) };

  if (id.startsWith("theme:")) {
    const rest = id.slice("theme:".length);
    if (rest === "") return { type: "theme", id: null }; // "Auto"
    return /^\d+$/.test(rest) ? { type: "theme", id: Number(rest) } : null;
  }
  return null;
}
