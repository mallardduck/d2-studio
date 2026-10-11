import { useCallback, useEffect, useRef, useState, type KeyboardEvent, type ReactNode } from "react";

interface Props {
  /** Text of the button that opens the menu. */
  label: ReactNode;
  /** Accessible name of the menu itself. */
  menuLabel: string;
  disabled?: boolean;
  /** Called each time the menu opens, e.g. to refresh its contents. */
  onOpen?: () => void;
  /** Menu contents. Items should use role="menuitem", "menuitemcheckbox" or "menuitemradio". */
  children: (close: () => void) => ReactNode;
}

const ITEM = '[role="menuitem"], [role="menuitemcheckbox"], [role="menuitemradio"]';

/**
 * A button that opens a menu: closes on Escape (returning focus to the
 * button) and on outside clicks, and moves between items with Up/Down.
 */
export function MenuButton({ label, menuLabel, disabled, onOpen, children }: Props) {
  const [open, setOpen] = useState(false);
  const root = useRef<HTMLDivElement>(null);
  const trigger = useRef<HTMLButtonElement>(null);
  const onOpenRef = useRef(onOpen);
  onOpenRef.current = onOpen;

  const close = useCallback((restoreFocus = false) => {
    setOpen(false);
    if (restoreFocus) trigger.current?.focus();
  }, []);

  useEffect(() => {
    if (!open) return;
    onOpenRef.current?.();
    const onDown = (e: MouseEvent) => {
      if (!root.current?.contains(e.target as Node)) close();
    };
    document.addEventListener("mousedown", onDown);
    return () => document.removeEventListener("mousedown", onDown);
  }, [open, close]);

  const onKeyDown = (e: KeyboardEvent<HTMLDivElement>) => {
    if (e.key === "Escape" && open) {
      e.stopPropagation();
      close(true);
      return;
    }
    if (e.key !== "ArrowDown" && e.key !== "ArrowUp") return;
    const items = Array.from(root.current?.querySelectorAll<HTMLElement>(ITEM) ?? []);
    if (items.length === 0) return;
    e.preventDefault();
    const i = items.indexOf(document.activeElement as HTMLElement);
    const next = e.key === "ArrowDown" ? (i + 1) % items.length : (i - 1 + items.length) % items.length;
    items[next].focus();
  };

  return (
    <div ref={root} className="relative" onKeyDown={onKeyDown}>
      <button
        ref={trigger}
        type="button"
        aria-haspopup="menu"
        aria-expanded={open}
        disabled={disabled}
        onClick={() => setOpen((o) => !o)}
        className="rounded px-2 py-1 hover:bg-line/40 disabled:opacity-50"
      >
        {label}
      </button>
      {open && (
        <div
          role="menu"
          aria-label={menuLabel}
          className="absolute left-0 top-full z-20 mt-1 max-h-80 w-80 overflow-auto rounded-md border border-line bg-surface py-1 shadow-lg"
        >
          {children(() => close(false))}
        </div>
      )}
    </div>
  );
}
