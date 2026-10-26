import { useRef, type KeyboardEvent } from "react";
import { DOC_TAB } from "../tabs/model";

export interface TabItem {
  id: string;
  label: string;
  /** Shown after the label, e.g. an unsaved marker. */
  suffix?: string;
}

interface Props {
  tabs: TabItem[];
  active: string;
  onSelect: (id: string) => void;
  onClose: (id: string) => void;
}

/** Document tab plus snippet tabs. Arrow keys, Home and End move between tabs. */
export function TabBar({ tabs, active, onSelect, onClose }: Props) {
  const list = useRef<HTMLDivElement>(null);

  const onKeyDown = (e: KeyboardEvent<HTMLDivElement>) => {
    const i = tabs.findIndex((t) => t.id === active);
    let next = -1;
    if (e.key === "ArrowRight") next = (i + 1) % tabs.length;
    else if (e.key === "ArrowLeft") next = (i - 1 + tabs.length) % tabs.length;
    else if (e.key === "Home") next = 0;
    else if (e.key === "End") next = tabs.length - 1;
    if (next < 0) return;
    e.preventDefault();
    onSelect(tabs[next].id);
    list.current?.querySelectorAll<HTMLElement>('[role="tab"]')[next]?.focus();
  };

  return (
    <div
      ref={list}
      role="tablist"
      aria-label="Open documents and snippets"
      onKeyDown={onKeyDown}
      className="flex shrink-0 gap-px overflow-x-auto border-b border-line bg-surface-alt"
    >
      {tabs.map((t) => {
        const selected = t.id === active;
        return (
          <div
            key={t.id}
            className={`flex shrink-0 items-center border-b-2 ${selected ? "border-accent bg-surface" : "border-transparent"}`}
          >
            <button
              type="button"
              role="tab"
              id={`tab-${t.id}`}
              aria-selected={selected}
              tabIndex={selected ? 0 : -1}
              onClick={() => onSelect(t.id)}
              className="max-w-48 truncate px-3 py-1.5 hover:bg-line/40"
            >
              {t.label}
              {t.suffix}
            </button>
            {t.id !== DOC_TAB && (
              <button
                type="button"
                aria-label={`Close ${t.label}`}
                tabIndex={selected ? 0 : -1}
                onClick={() => onClose(t.id)}
                className="mr-1 rounded px-1.5 hover:bg-line/40"
              >
                ×
              </button>
            )}
          </div>
        );
      })}
    </div>
  );
}
