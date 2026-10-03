import { Format } from "../../bindings/d2studio/export/models";
import { FORMATS } from "../export/model";
import { MenuButton } from "./MenuButton";

interface Props {
  busy: boolean;
  fetchRemote: boolean;
  onExport: (format: Format) => void;
  onToggleRemote: (value: boolean) => void;
}

export function ExportMenu({ busy, fetchRemote, onExport, onToggleRemote }: Props) {
  return (
    <MenuButton label={busy ? "Exporting…" : "Export ▾"} menuLabel="Export" disabled={busy}>
      {(close) => (
        <>
          {FORMATS.map((f) => (
            <button
              key={f.format}
              type="button"
              role="menuitem"
              onClick={() => {
                close();
                onExport(f.format);
              }}
              className="flex w-full flex-col px-3 py-1.5 text-left hover:bg-line/40 focus-visible:bg-line/40"
            >
              <span>Export as {f.label}…</span>
              <span className="text-xs text-fg-muted">{f.hint}</span>
            </button>
          ))}
          <button
            type="button"
            role="menuitemcheckbox"
            aria-checked={fetchRemote}
            onClick={() => onToggleRemote(!fetchRemote)}
            className="mt-1 flex w-full items-start gap-2 border-t border-line px-3 py-1.5 text-left hover:bg-line/40 focus-visible:bg-line/40"
          >
            <span aria-hidden="true" className="mt-0.5 w-4 shrink-0">
              {fetchRemote ? "✓" : ""}
            </span>
            <span className="flex flex-col">
              <span>Fetch remote images</span>
              <span className="text-xs text-fg-muted">
                For PNG and PDF. Downloads icons from the web; off by default so exports work offline.
              </span>
            </span>
          </button>
        </>
      )}
    </MenuButton>
  );
}
