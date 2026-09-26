import type { Version } from "../../bindings/d2studio/files/models";

/** A non-blocking warning about the open file. */
export type Banner =
  | { kind: "modified"; version: Version }
  | { kind: "deleted" }
  | { kind: "unavailable"; message: string }
  | { kind: "error"; message: string };

interface Props {
  banner: Banner | null;
  name: string;
  onReload: () => void;
  onKeepMine: (version: Version) => void;
  onSave: () => void;
  onSaveAs: () => void;
  onDismiss: () => void;
}

const button = "rounded border border-line bg-surface px-2 py-0.5 hover:bg-line/40";

export function FileBanner({ banner, name, onReload, onKeepMine, onSave, onSaveAs, onDismiss }: Props) {
  if (!banner) return null;

  let text: string;
  let actions: React.ReactNode;
  switch (banner.kind) {
    case "modified":
      text = `“${name}” was changed outside D2 Studio, and you have unsaved edits.`;
      actions = (
        <>
          <button type="button" className={button} onClick={onReload}>
            Reload from disk (discard my edits)
          </button>
          <button type="button" className={button} onClick={() => onKeepMine(banner.version)}>
            Keep my edits
          </button>
        </>
      );
      break;
    case "deleted":
      text = `“${name}” was deleted or moved on disk. Your edits are still here.`;
      actions = (
        <>
          <button type="button" className={button} onClick={onSave}>
            Save to recreate it
          </button>
          <button type="button" className={button} onClick={onSaveAs}>
            Save As…
          </button>
          <button type="button" className={button} onClick={onDismiss}>
            Dismiss
          </button>
        </>
      );
      break;
    case "unavailable":
      text = `“${name}” cannot be read right now: ${banner.message}`;
      actions = (
        <button type="button" className={button} onClick={onDismiss}>
          Dismiss
        </button>
      );
      break;
    case "error":
      text = banner.message;
      actions = (
        <button type="button" className={button} onClick={onDismiss}>
          Dismiss
        </button>
      );
      break;
  }

  return (
    <div
      role="alert"
      className="flex shrink-0 flex-wrap items-center gap-x-3 gap-y-1 border-b border-line bg-surface-alt px-3 py-1.5"
    >
      <span className="font-medium text-warn">{banner.kind === "error" ? "Error" : "Heads up"}</span>
      <span className="min-w-0 break-words">{text}</span>
      <span className="ml-auto flex flex-wrap gap-2">{actions}</span>
    </div>
  );
}
