import { useCallback, useEffect, useRef, useState, type KeyboardEvent, type PointerEvent } from "react";
import type { CompileState } from "../hooks/useCompiler";
import { fit, parseSvgSize, wheelFactor, zoomAt, zoomTo, type Size, type Viewport } from "../preview/viewport";

interface Props {
  svg: string;
  state: CompileState;
  /** Reports the zoom percentage (null when there is nothing to show). */
  onZoomChange?: (percent: number | null) => void;
}

const BUTTON_STEP = 1.25;

/**
 * Pannable, zoomable diagram preview.
 *
 * The SVG is shown through an <img> backed by a blob URL: browsers do not run
 * scripts or event handlers in SVG loaded as an image, so a .d2 file from an
 * untrusted source cannot execute code in the app. D2 emits a viewBox but no
 * width/height, so the image is given explicit dimensions from the viewBox.
 *
 * While the user has not zoomed or panned, the view stays "fitted" and
 * re-fits as the diagram or window changes. Zooming or panning turns that off
 * until Fit is pressed.
 */
export function Preview({ svg, state, onZoomChange }: Props) {
  const box = useRef<HTMLDivElement>(null);
  const [url, setUrl] = useState("");
  const [size, setSize] = useState<Size | null>(null);
  const [container, setContainer] = useState<Size>({ width: 0, height: 0 });
  const [vp, setVp] = useState<Viewport>({ scale: 1, x: 0, y: 0 });
  const [fitted, setFitted] = useState(true);
  const [panning, setPanning] = useState(false);
  const vpRef = useRef(vp);
  vpRef.current = vp;
  const drag = useRef<{ x: number; y: number } | null>(null);

  useEffect(() => {
    if (!svg) {
      setUrl("");
      setSize(null);
      return;
    }
    const next = URL.createObjectURL(new Blob([svg], { type: "image/svg+xml" }));
    setUrl(next);
    setSize(parseSvgSize(svg));
    return () => URL.revokeObjectURL(next);
  }, [svg]);

  useEffect(() => {
    const el = box.current;
    if (!el) return;
    const ro = new ResizeObserver(([entry]) => {
      const { width, height } = entry.contentRect;
      setContainer({ width, height });
    });
    ro.observe(el);
    return () => ro.disconnect();
  }, []);

  useEffect(() => {
    if (fitted && size) setVp(fit(container, size));
  }, [fitted, size, container]);

  useEffect(() => {
    onZoomChange?.(url ? Math.round(vp.scale * 100) : null);
  }, [onZoomChange, url, vp.scale]);

  // Wheel zoom around the cursor. Registered natively because React's wheel
  // listener is passive and could not stop the page from scrolling.
  useEffect(() => {
    const el = box.current;
    if (!el) return;
    const onWheel = (e: WheelEvent) => {
      e.preventDefault();
      const rect = el.getBoundingClientRect();
      const factor = wheelFactor(e.deltaY, e.deltaMode, e.ctrlKey);
      setFitted(false);
      setVp(zoomAt(vpRef.current, factor, e.clientX - rect.left, e.clientY - rect.top));
    };
    el.addEventListener("wheel", onWheel, { passive: false });
    return () => el.removeEventListener("wheel", onWheel);
  }, []);

  const zoomBy = useCallback(
    (factor: number) => {
      setFitted(false);
      setVp((v) => zoomAt(v, factor, container.width / 2, container.height / 2));
    },
    [container],
  );
  const actualSize = useCallback(() => {
    setFitted(false);
    setVp((v) => zoomTo(v, 1, container));
  }, [container]);
  const fitView = useCallback(() => setFitted(true), []);

  const onPointerDown = (e: PointerEvent<HTMLDivElement>) => {
    if (e.button !== 0) return;
    drag.current = { x: e.clientX, y: e.clientY };
    setPanning(true);
    e.currentTarget.setPointerCapture(e.pointerId);
  };
  const onPointerMove = (e: PointerEvent<HTMLDivElement>) => {
    const last = drag.current;
    if (!last) return;
    const dx = e.clientX - last.x;
    const dy = e.clientY - last.y;
    drag.current = { x: e.clientX, y: e.clientY };
    setFitted(false);
    setVp((v) => ({ ...v, x: v.x + dx, y: v.y + dy }));
  };
  const endPan = (e: PointerEvent<HTMLDivElement>) => {
    if (!drag.current) return;
    drag.current = null;
    setPanning(false);
    e.currentTarget.releasePointerCapture(e.pointerId);
  };

  const onKeyDown = (e: KeyboardEvent<HTMLDivElement>) => {
    if (e.metaKey || e.ctrlKey || e.altKey) return;
    const handled = ["+", "=", "-", "_", "0", "1"];
    if (!handled.includes(e.key)) return;
    e.preventDefault();
    if (e.key === "+" || e.key === "=") zoomBy(BUTTON_STEP);
    else if (e.key === "-" || e.key === "_") zoomBy(1 / BUTTON_STEP);
    else if (e.key === "0") fitView();
    else actualSize();
  };

  return (
    <div className="relative h-full">
      <div
        ref={box}
        tabIndex={0}
        role="img"
        aria-label="Rendered D2 diagram (empty until the source compiles). Scroll to zoom, drag to pan, plus and minus to zoom, 0 to fit, 1 for actual size."
        onPointerDown={onPointerDown}
        onPointerMove={onPointerMove}
        onPointerUp={endPan}
        onPointerCancel={endPan}
        onDoubleClick={fitView}
        onKeyDown={onKeyDown}
        className={`relative h-full touch-none select-none overflow-hidden bg-surface-alt ${
          panning ? "cursor-grabbing" : "cursor-grab"
        }`}
      >
        {!url && (
          <div className="pointer-events-none flex h-full items-center justify-center text-fg-muted">
            {state === "error" ? "Fix the errors to see a preview." : "Preview will appear here."}
          </div>
        )}
        {url && (
          <img
            src={url}
            alt=""
            draggable={false}
            onLoad={(e) => {
              // Fallback when the SVG has no usable viewBox or width/height.
              if (!size) setSize({ width: e.currentTarget.naturalWidth, height: e.currentTarget.naturalHeight });
            }}
            style={{
              position: "absolute",
              left: 0,
              top: 0,
              width: size?.width,
              height: size?.height,
              maxWidth: "none",
              transformOrigin: "0 0",
              transform: `translate(${vp.x}px, ${vp.y}px) scale(${vp.scale})`,
              opacity: state === "error" ? 0.5 : 1,
            }}
          />
        )}
      </div>

      {url && state === "error" && (
        <div className="pointer-events-none absolute left-3 top-3 rounded border border-line bg-surface px-2 py-1 text-warn">
          Showing last successful render
        </div>
      )}

      {url && (
        <div
          role="toolbar"
          aria-label="Preview zoom"
          className="absolute bottom-3 right-3 flex items-center overflow-hidden rounded-md border border-line bg-surface shadow-sm"
        >
          <button
            type="button"
            aria-label="Zoom out"
            title="Zoom out (-)"
            onClick={() => zoomBy(1 / BUTTON_STEP)}
            className="px-2 py-1 hover:bg-line/40"
          >
            −
          </button>
          <button
            type="button"
            aria-label="Actual size (100%)"
            title="Actual size (1)"
            onClick={actualSize}
            className="min-w-14 border-x border-line px-2 py-1 tabular-nums hover:bg-line/40"
          >
            {Math.round(vp.scale * 100)}%
          </button>
          <button
            type="button"
            aria-label="Zoom in"
            title="Zoom in (+)"
            onClick={() => zoomBy(BUTTON_STEP)}
            className="px-2 py-1 hover:bg-line/40"
          >
            +
          </button>
          <button
            type="button"
            aria-label="Fit to view"
            aria-pressed={fitted}
            title="Fit to view (0, or double-click)"
            onClick={fitView}
            className={`border-l border-line px-2 py-1 hover:bg-line/40 ${fitted ? "font-medium text-accent" : ""}`}
          >
            Fit
          </button>
        </div>
      )}
    </div>
  );
}
