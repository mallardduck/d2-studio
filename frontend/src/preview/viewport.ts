/** Pure pan/zoom math for the diagram preview, kept free of the DOM for testing. */

export interface Viewport {
  /** Zoom factor; 1 is 100%. */
  scale: number;
  /** Translation of the content's top-left corner within the container, in px. */
  x: number;
  y: number;
}

export interface Size {
  width: number;
  height: number;
}

export const MIN_SCALE = 0.02;
export const MAX_SCALE = 8;
/** Fit may enlarge small diagrams, but not absurdly. */
export const MAX_FIT_SCALE = 3;
export const FIT_PADDING = 24;

export const clampScale = (s: number, max = MAX_SCALE) => Math.min(max, Math.max(MIN_SCALE, s));

/** Scale and center `content` inside `container` with padding. */
export function fit(container: Size, content: Size, padding = FIT_PADDING): Viewport {
  if (content.width <= 0 || content.height <= 0 || container.width <= 0 || container.height <= 0) {
    return { scale: 1, x: 0, y: 0 };
  }
  const availW = Math.max(1, container.width - padding * 2);
  const availH = Math.max(1, container.height - padding * 2);
  const scale = clampScale(Math.min(availW / content.width, availH / content.height), MAX_FIT_SCALE);
  return {
    scale,
    x: (container.width - content.width * scale) / 2,
    y: (container.height - content.height * scale) / 2,
  };
}

/** Scale by `factor`, keeping the content point under (px, py) fixed on screen. */
export function zoomAt(vp: Viewport, factor: number, px: number, py: number): Viewport {
  const scale = clampScale(vp.scale * factor);
  const k = scale / vp.scale;
  return { scale, x: px - (px - vp.x) * k, y: py - (py - vp.y) * k };
}

/** Set an absolute zoom (e.g. 100%) around the center of the container. */
export function zoomTo(vp: Viewport, scale: number, container: Size): Viewport {
  const s = clampScale(scale);
  return zoomAt(vp, s / vp.scale, container.width / 2, container.height / 2);
}

/** Factor for a wheel event; pinch gestures (ctrlKey) are reported with smaller deltas. */
export function wheelFactor(deltaY: number, deltaMode: number, ctrlKey: boolean): number {
  // deltaMode 1 = lines, 2 = pages; normalize to pixels.
  const px = deltaMode === 1 ? deltaY * 16 : deltaMode === 2 ? deltaY * 400 : deltaY;
  const clamped = Math.max(-200, Math.min(200, px));
  return Math.exp(-clamped * (ctrlKey ? 0.01 : 0.0015));
}

/**
 * Intrinsic size of an SVG document. Prefers the root's viewBox, which D2
 * always emits, and falls back to numeric width/height. Returns null when
 * neither is usable (percentages and missing sizes are rejected).
 */
export function parseSvgSize(svg: string): Size | null {
  const root = /<svg\b[^>]*>/i.exec(svg)?.[0];
  if (!root) return null;

  const viewBox = /\bviewBox\s*=\s*["']\s*([-\d.eE+]+)[\s,]+([-\d.eE+]+)[\s,]+([-\d.eE+]+)[\s,]+([-\d.eE+]+)\s*["']/.exec(root);
  if (viewBox) {
    const width = Number(viewBox[3]);
    const height = Number(viewBox[4]);
    if (width > 0 && height > 0) return { width, height };
  }

  const num = (attr: string) => {
    const m = new RegExp(`\\b${attr}\\s*=\\s*["']\\s*([\\d.]+)(?:px)?\\s*["']`, "i").exec(root);
    return m ? Number(m[1]) : NaN;
  };
  const width = num("width");
  const height = num("height");
  return width > 0 && height > 0 ? { width, height } : null;
}
