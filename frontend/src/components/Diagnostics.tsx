import type { Diagnostic } from "../../bindings/d2studio/compiler/models";
import { baseName } from "../files/model";

interface Props {
  diagnostics: Diagnostic[];
  onSelect: (d: Diagnostic) => void;
}

/** Non-blocking error list. Selecting an entry moves the editor cursor to it. */
export function Diagnostics({ diagnostics, onSelect }: Props) {
  if (diagnostics.length === 0) return null;
  return (
    <ul
      aria-label="Compiler diagnostics"
      className="max-h-40 shrink-0 overflow-auto border-t border-line bg-surface-alt"
    >
      {diagnostics.map((d, i) => (
        <li key={i}>
          <button
            type="button"
            disabled={!d.hasLocation && !d.path}
            onClick={() => onSelect(d)}
            className="flex w-full items-baseline gap-2 px-3 py-1 text-left enabled:hover:bg-line/40"
          >
            <span className="font-medium text-err">Error</span>
            {d.hasLocation && (
              <span className="font-mono text-fg-muted" title={d.path || undefined}>
                {d.path ? `${baseName(d.path)}:` : ""}
                {d.line}:{d.column}
              </span>
            )}
            <span className="min-w-0 break-words">{d.message}</span>
          </button>
        </li>
      ))}
    </ul>
  );
}
