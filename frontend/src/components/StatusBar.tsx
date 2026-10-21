import type { CompileState } from "../hooks/useCompiler";
import type { Meta, Theme } from "../../bindings/d2studio/compiler/models";
import { themeLabel } from "./OptionsBar";

interface Props {
  state: CompileState;
  meta: Meta | null;
  themes: Theme[];
  /** Preview zoom percentage; null when there is no visible preview. */
  zoom?: number | null;
  errorCount: number;
  /** A note about hidden or failed remote images, with an offer to show them. */
  remoteNote?: { text: string; canShow: boolean } | null;
  onShowRemote?: () => void;
  /** How the file is stored, e.g. "UTF-8 · CRLF". */
  encoding?: string;
  /** Non-blocking message, e.g. a settings save failure. */
  notice?: string;
}

const LABEL: Record<CompileState, { text: string; color: string }> = {
  idle: { text: "Idle", color: "text-fg-muted" },
  compiling: { text: "Compiling…", color: "text-warn" },
  success: { text: "Success", color: "text-ok" },
  error: { text: "Error", color: "text-err" },
};

/** Where a setting came from, shown only when it is not the plain default. */
function origin(source: string, overridden: string): string {
  if (overridden) return ` · overrides d2-config (${overridden})`;
  return source === "source" ? " · from d2-config" : "";
}

export function StatusBar({
  state,
  meta,
  themes,
  zoom,
  errorCount,
  remoteNote,
  onShowRemote,
  encoding = "UTF-8",
  notice,
}: Props) {
  const s = LABEL[state];
  const layoutOverrides =
    meta?.layoutSource === "ui" && meta.sourceLayout && meta.sourceLayout !== meta.layout ? meta.sourceLayout : "";
  const themeOverrides =
    meta?.themeSource === "ui" && meta.sourceThemeId != null && meta.sourceThemeId !== meta.themeId
      ? themeLabel(themes, meta.sourceThemeId)
      : "";
  return (
    <footer className="flex h-7 shrink-0 items-center gap-4 border-t border-line bg-surface-alt px-3 text-fg-muted">
      <span aria-live="polite" className={`font-medium ${s.color}`}>
        {s.text}
        {state === "error" && errorCount > 0 ? ` (${errorCount})` : ""}
      </span>
      {meta && (
        <>
          <span>
            Layout: {meta.layout}
            {origin(meta.layoutSource, layoutOverrides)}
          </span>
          <span>
            Theme: {themeLabel(themes, meta.themeId)}
            {origin(meta.themeSource, themeOverrides)}
          </span>
          {zoom != null && <span>Zoom: {zoom}%</span>}
          <span>{meta.durationMs} ms</span>
        </>
      )}
      {remoteNote && (
        <span className="flex items-center gap-1.5 text-warn">
          {remoteNote.text}
          {remoteNote.canShow && onShowRemote && (
            <button
              type="button"
              onClick={onShowRemote}
              title="Download and show remote images for this file (View ▸ Show Remote Images in This File)"
              className="rounded border border-line px-1.5 text-fg hover:bg-line/40"
            >
              Show
            </button>
          )}
        </span>
      )}
      <span role="status" className="text-fg">
        {notice}
      </span>
      <span className="ml-auto">{encoding}</span>
    </footer>
  );
}
