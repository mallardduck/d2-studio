import type { CompileState } from "../hooks/useCompiler";
import type { Meta } from "../../bindings/d2studio/compiler/models";

interface Props {
  state: CompileState;
  meta: Meta | null;
  themeName: string;
  errorCount: number;
  /** Non-blocking message, e.g. a settings save failure. */
  notice?: string;
}

const LABEL: Record<CompileState, { text: string; color: string }> = {
  idle: { text: "Idle", color: "text-fg-muted" },
  compiling: { text: "Compiling…", color: "text-warn" },
  success: { text: "Success", color: "text-ok" },
  error: { text: "Error", color: "text-err" },
};

const SOURCE_NOTE: Record<string, string> = {
  ui: "selected in app",
  source: "set in d2-config",
  default: "default",
};

export function StatusBar({ state, meta, themeName, errorCount, notice }: Props) {
  const s = LABEL[state];
  return (
    <footer className="flex h-7 shrink-0 items-center gap-4 border-t border-line bg-surface-alt px-3 text-fg-muted">
      <span aria-live="polite" className={`font-medium ${s.color}`}>
        {s.text}
        {state === "error" && errorCount > 0 ? ` (${errorCount})` : ""}
      </span>
      {meta && (
        <>
          <span title={SOURCE_NOTE[meta.layoutSource] ?? ""}>Layout: {meta.layout}</span>
          <span>Theme: {themeName}</span>
          <span>{meta.durationMs} ms</span>
        </>
      )}
      {notice && <span className="text-warn">{notice}</span>}
      <span className="ml-auto">UTF-8</span>
    </footer>
  );
}
