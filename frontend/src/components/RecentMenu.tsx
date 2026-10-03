import { useState } from "react";
import { ClearRecent, Recent } from "../../bindings/d2studio/files/service";
import type { RecentFile } from "../../bindings/d2studio/files/models";
import { MenuButton } from "./MenuButton";

interface Props {
  onOpen: (file: RecentFile) => void;
}

/** "Recent" dropdown. The list is fetched each time it opens so it is never stale. */
export function RecentMenu({ onOpen }: Props) {
  const [items, setItems] = useState<RecentFile[]>([]);

  return (
    <MenuButton label="Recent ▾" menuLabel="Recent files" onOpen={() => Recent().then((r) => setItems(r ?? []))}>
      {(close) => (
        <>
          {items.length === 0 && <div className="px-3 py-2 text-fg-muted">No recent files</div>}
          {items.map((f) => (
            <button
              key={f.path}
              type="button"
              role="menuitem"
              title={f.path}
              onClick={() => {
                close();
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
        </>
      )}
    </MenuButton>
  );
}
