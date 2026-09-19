import { useRef, type KeyboardEvent, type PointerEvent, type RefObject } from "react";
import { MAX_SPLIT, MIN_SPLIT, clampRatio } from "../hooks/useSettings";

interface Props {
  /** Element whose width the ratio is measured against. */
  container: RefObject<HTMLElement | null>;
  ratio: number;
  /** `commit` is false while dragging and true when the change is final. */
  onChange: (ratio: number, commit: boolean) => void;
}

const KEY_STEP = 0.02;

/** Draggable, keyboard-operable divider between source and preview. */
export function SplitDivider({ container, ratio, onChange }: Props) {
  const dragging = useRef(false);
  const last = useRef(ratio);

  const ratioAt = (clientX: number) => {
    const rect = container.current?.getBoundingClientRect();
    if (!rect || rect.width === 0) return last.current;
    return clampRatio((clientX - rect.left) / rect.width);
  };

  const onPointerDown = (e: PointerEvent<HTMLDivElement>) => {
    dragging.current = true;
    e.currentTarget.setPointerCapture(e.pointerId);
  };
  const onPointerMove = (e: PointerEvent<HTMLDivElement>) => {
    if (!dragging.current) return;
    last.current = ratioAt(e.clientX);
    onChange(last.current, false);
  };
  const onPointerUp = (e: PointerEvent<HTMLDivElement>) => {
    if (!dragging.current) return;
    dragging.current = false;
    e.currentTarget.releasePointerCapture(e.pointerId);
    onChange(last.current, true);
  };

  const onKeyDown = (e: KeyboardEvent<HTMLDivElement>) => {
    const next =
      e.key === "ArrowLeft" ? ratio - KEY_STEP
      : e.key === "ArrowRight" ? ratio + KEY_STEP
      : e.key === "Home" ? MIN_SPLIT
      : e.key === "End" ? MAX_SPLIT
      : null;
    if (next === null) return;
    e.preventDefault();
    last.current = clampRatio(next);
    onChange(last.current, true);
  };

  return (
    <div
      role="separator"
      aria-orientation="vertical"
      aria-label="Resize source and preview panes"
      aria-valuemin={Math.round(MIN_SPLIT * 100)}
      aria-valuemax={Math.round(MAX_SPLIT * 100)}
      aria-valuenow={Math.round(ratio * 100)}
      tabIndex={0}
      onPointerDown={onPointerDown}
      onPointerMove={onPointerMove}
      onPointerUp={onPointerUp}
      onPointerCancel={onPointerUp}
      onKeyDown={onKeyDown}
      onDoubleClick={() => onChange(0.5, true)}
      className="w-1.5 shrink-0 cursor-col-resize touch-none bg-line transition-colors hover:bg-accent focus-visible:bg-accent"
    />
  );
}
