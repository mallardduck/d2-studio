import { useCallback, useEffect, useRef, useState, type KeyboardEvent } from "react";
import { ClearRecent, Recent } from "../../bindings/d2studio/files/service";
import type { RecentFile } from "../../bindings/d2studio/files/models";

interface Props {
  onOpen: (file: RecentFile) => void;
}

/** "Recent" dropdown. The list is fetched each time it opens so it is never stale. */
export function RecentMenu({ onOpen }: Props) {
  const [open, setOpen] = useState(false);
  const [items, setItems] = useState<RecentFile[]>([]);
  const root = useRef<HTMLDivElement>(null);
  const trigger = useRef<HTMLButtonElement>(null);

  const close = useCallback((restoreFocus: boolean) => {
    setOpen(false);
    if (restoreFocus) trigger.current?.focus();
  }, []);

  useEffect(() => {
    if (!open) return;
    Recent().then((r) => setItems(r ?? []));
    const onDown = (e: MouseEvent) => {
      if (!root.current?.contains(e.target as Node)) close(false);
    };
    document.addEventListener("mousedown", onDown);
    return () => document.removeEventListener("mousedown", onDown);
  }, [open, close]);

  const onKeyDown = (e: KeyboardEvent<HTMLDivElement>) => {
    if (e.key === "Escape") {
      e.stopPropagation();
      close(true);
      return;
    }
    if (e.key !== "ArrowDown" && e.key !== "ArrowUp") return;
    e.preventDefault();
    const entries = Array.from(root.current?.querySelectorAll<HTMLElement>('[role="menuitem"]') ?? []);
    if (entries.length === 0) return;
    const i = entries.indexOf(document.activeElement as HTMLElement);
    const next = e.key === "ArrowDown" ? (i + 1) % entries.length : (i - 1 + entries.length) % entries.length;
    entries[next].focus();
  };

  return (
    <div ref={root} className="relative" onKeyDown={onKeyDown}>
      <button
        ref={trigger}
        type="button"
        aria-haspopup="menu"
        aria-expanded={open}
        onClick={() => setOpen((o) => !o)}
        className="rounded px-2 py-1 hover:bg-line/40"
      >
        Recent ▾
      </button>
      {open && (
        <div
          role="menu"
          aria-label="Recent files"
          className="absolute left-0 top-full z-20 mt-1 max-h-80 w-80 overflow-auto rounded-md border border-line bg-surface py-1 shadow-lg"
        >
          {items.length === 0 && <div className="px-3 py-2 text-fg-muted">No recent files</div>}
          {items.map((f) => (
            <button
              key={f.path}
              type="button"
              role="menuitem"
              title={f.path}
              onClick={() => {
                close(false);
                onOpen(f);
              }}
              className="flex w-full flex-col px-3 py-1 text-left hover:bg-line/40 focus-visible:bg-line/40"
            >
              <span className={f.exists ? "" : "text-fg-muted line-through"}>
                {f.name}
                {f.exists ? "" : " (missing)"}
              </span>
              <span className="truncate text-xs text-fg-muted">{f.dir}</span>
            </button>
          ))}
          {items.length > 0 && (
            <button
              type="button"
              role="menuitem"
              onClick={() => {
                ClearRecent().then(() => setItems([]));
              }}
              className="mt-1 w-full border-t border-line px-3 py-1.5 text-left text-fg-muted hover:bg-line/40"
            >
              Clear recent files
            </button>
          )}
        </div>
      )}
    </div>
  );
}
