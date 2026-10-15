import type { ReactNode } from "react";

interface Props {
  onOpen: () => void;
  onSave: () => void;
  /** Extra controls after Save, e.g. the Export menu. */
  extra?: ReactNode;
}

const isMac = typeof navigator !== "undefined" && /Mac/i.test(navigator.platform);
const MOD = isMac ? "⌘" : "Ctrl+";

const btn = "rounded px-2 py-1 hover:bg-line/40";

/**
 * The file actions used most often. Everything else (New, Save As, recent
 * files, folders, autosave) lives in the File menu.
 */
export function FileControls({ onOpen, onSave, extra }: Props) {
  return (
    <div role="group" aria-label="File" className="flex items-center gap-0.5">
      <button type="button" className={btn} title={`Open… (${MOD}O)`} onClick={onOpen}>
        Open…
      </button>
      <button type="button" className={btn} title={`Save (${MOD}S)`} onClick={onSave}>
        Save
      </button>
      {extra}
    </div>
  );
}
