import type { ViewMode } from "../hooks/useSettings";

interface Props {
  fileName: string;
  mode: ViewMode;
  onMode: (m: ViewMode) => void;
}

const isMac = typeof navigator !== "undefined" && /Mac/i.test(navigator.platform);
const MOD = isMac ? "⌘" : "Ctrl+";

export const MODES: { id: ViewMode; label: string; key: string }[] = [
  { id: "source", label: "Source", key: "1" },
  { id: "split", label: "Split", key: "2" },
  { id: "preview", label: "Preview", key: "3" },
];

export function Toolbar({ fileName, mode, onMode }: Props) {
  return (
    <header className="flex h-10 shrink-0 items-center gap-3 border-b border-line bg-surface-alt px-3">
      <h1 className="text-sm font-semibold">D2 Studio</h1>
      <span className="text-fg-muted">{fileName}</span>

      <div role="group" aria-label="View mode" className="ml-auto flex overflow-hidden rounded-md border border-line">
        {MODES.map((m) => (
          <button
            key={m.id}
            type="button"
            aria-pressed={mode === m.id}
            title={`${m.label} (${MOD}${m.key})`}
            onClick={() => onMode(m.id)}
            className={`px-3 py-1 ${
              mode === m.id ? "bg-accent font-medium text-surface" : "bg-surface hover:bg-line/40"
            }`}
          >
            {m.label}
          </button>
        ))}
      </div>
    </header>
  );
}
