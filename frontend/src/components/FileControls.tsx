import type { RecentFile } from "../../bindings/d2studio/files/models";
import { RecentMenu } from "./RecentMenu";

interface Props {
  onNew: () => void;
  onOpen: () => void;
  onSave: () => void;
  onSaveAs: () => void;
  onOpenRecent: (file: RecentFile) => void;
}

const isMac = typeof navigator !== "undefined" && /Mac/i.test(navigator.platform);
const MOD = isMac ? "⌘" : "Ctrl+";
const SHIFT = isMac ? "⇧" : "Shift+";

const btn = "rounded px-2 py-1 hover:bg-line/40";

export function FileControls({ onNew, onOpen, onSave, onSaveAs, onOpenRecent }: Props) {
  return (
    <div role="group" aria-label="File" className="flex items-center gap-0.5">
      <button type="button" className={btn} title={`New (${MOD}N)`} onClick={onNew}>
        New
      </button>
      <button type="button" className={btn} title={`Open… (${MOD}O)`} onClick={onOpen}>
        Open…
      </button>
      <RecentMenu onOpen={onOpenRecent} />
      <button type="button" className={btn} title={`Save (${MOD}S)`} onClick={onSave}>
        Save
      </button>
      <button type="button" className={btn} title={`Save As… (${MOD}${SHIFT}S)`} onClick={onSaveAs}>
        Save As…
      </button>
    </div>
  );
}
