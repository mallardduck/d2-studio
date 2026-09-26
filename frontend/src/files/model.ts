import { EventKind, type Document, type Event as FileEvent } from "../../bindings/d2studio/files/models";

export const APP_NAME = "D2 Studio";
export const UNTITLED = "Untitled.d2";

/** Window title: a dot marks unsaved changes. */
export function windowTitle(name: string, dirty: boolean): string {
  return `${dirty ? "● " : ""}${name} — ${APP_NAME}`;
}

/** Status-bar text for how a file is stored, e.g. "UTF-8 · CRLF". */
export function encodingLabel(doc: Pick<Document, "bom" | "lineEnding" | "mixedLineEndings"> | null): string {
  if (!doc) return "UTF-8";
  const parts = [doc.bom ? "UTF-8 with BOM" : "UTF-8"];
  if (doc.lineEnding === "crlf") parts.push("CRLF");
  if (doc.mixedLineEndings) parts.push("mixed line endings");
  return parts.join(" · ");
}

/** What to do when the open file changes on disk. */
export type ExternalAction =
  /** Safe to replace the editor contents; the user has nothing to lose. */
  | { type: "reload" }
  /** Unsaved edits exist: ask, never overwrite them silently. */
  | { type: "prompt-modified" }
  | { type: "prompt-deleted" }
  | { type: "prompt-unavailable"; message: string }
  /** The file is fine again: dismiss any warning about it. */
  | { type: "clear" };

/**
 * Decides how to react to an external file event. The one rule that matters:
 * unsaved editor content is never replaced without the user's say-so.
 */
export function decideExternal(event: Pick<FileEvent, "kind" | "message">, dirty: boolean): ExternalAction {
  switch (event.kind) {
    case EventKind.EventModified:
      return dirty ? { type: "prompt-modified" } : { type: "reload" };
    case EventKind.EventDeleted:
      return { type: "prompt-deleted" };
    case EventKind.EventUnavailable:
      return { type: "prompt-unavailable", message: event.message };
    case EventKind.EventRestored:
      return { type: "clear" };
    default:
      // Unknown kinds from a newer backend are ignored rather than guessed at.
      return { type: "clear" };
  }
}

/** Directory of a path, for starting dialogs next to the current file. */
export function dirOf(path: string | null): string {
  if (!path) return "";
  const i = Math.max(path.lastIndexOf("/"), path.lastIndexOf("\\"));
  return i > 0 ? path.slice(0, i) : "";
}

export function baseName(path: string): string {
  return path.slice(Math.max(path.lastIndexOf("/"), path.lastIndexOf("\\")) + 1);
}
