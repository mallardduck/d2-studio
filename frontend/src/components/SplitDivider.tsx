import { useRef, type KeyboardEvent, type PointerEvent, type RefObject } from "react";
import { MAX_SPLIT, MIN_SPLIT, clampRatio } from "../hooks/useSettings";

interface Props {
  /** Element whose width (or height, for a horizontal bar) the ratio is measured against. */
  container: RefObject<HTMLElement | null>;
  ratio: number;
  /**
   * "vertical" is a bar between side-by-side panes (dragged left and right);
   * "horizontal" is a bar between stacked panes (dragged up and down).
   */
  orientation?: "vertical" | "horizontal";
  label?: string;
  /** `commit` is false while dragging and true when the change is final. */
  onChange: (ratio: number, commit: boolean) => void;
}

const KEY_STEP = 0.02;

/** Draggable, keyboard-operable divider between source and preview. */
export function SplitDivider({
  container,
  ratio,
  onChange,
  orientation = "vertical",
  label = "Resize source and preview panes",
}: Props) {
  const horizontal = orientation === "horizontal";
  const dragging = useRef(false);
  const last = useRef(ratio);

  const ratioAt = (e: PointerEvent<HTMLDivElement>) => {
    const rect = container.current?.getBoundingClientRect();
    const size = rect && (horizontal ? rect.height : rect.width);
    if (!rect || !size) return last.current;
    return clampRatio(horizontal ? (e.clientY - rect.top) / size : (e.clientX - rect.left) / size);
  };

  const onPointerDown = (e: PointerEvent<HTMLDivElement>) => {
    dragging.current = true;
    e.currentTarget.setPointerCapture(e.pointerId);
  };
  const onPointerMove = (e: PointerEvent<HTMLDivElement>) => {
    if (!dragging.current) return;
    last.current = ratioAt(e);
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
      e.key === (horizontal ? "ArrowUp" : "ArrowLeft") ? ratio - KEY_STEP
      : e.key === (horizontal ? "ArrowDown" : "ArrowRight") ? ratio + KEY_STEP
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
      aria-orientation={orientation}
      aria-label={label}
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
      className={`shrink-0 touch-none bg-line transition-colors hover:bg-accent focus-visible:bg-accent ${
        horizontal ? "h-1.5 w-full cursor-row-resize" : "w-1.5 cursor-col-resize"
      }`}
    />
  );
}
