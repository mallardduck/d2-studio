import { describe, expect, it } from "vitest";
import {
  FIT_PADDING,
  MAX_FIT_SCALE,
  MAX_SCALE,
  MIN_SCALE,
  fit,
  parseSvgSize,
  wheelFactor,
  zoomAt,
  zoomTo,
} from "./viewport";

describe("fit", () => {
  it("scales content down to fit and centers it", () => {
    const vp = fit({ width: 1000, height: 500 }, { width: 2000, height: 400 }, 0);
    expect(vp.scale).toBeCloseTo(0.5);
    expect(vp.x).toBeCloseTo(0);
    expect(vp.y).toBeCloseTo((500 - 400 * 0.5) / 2);
  });

  it("scales small diagrams up to fill the pane, up to a cap", () => {
    const vp = fit({ width: 1000, height: 800 }, { width: 100, height: 100 });
    expect(vp.scale).toBe(MAX_FIT_SCALE);
    expect(vp.x).toBeCloseTo((1000 - 100 * MAX_FIT_SCALE) / 2);
    const mid = fit({ width: 400, height: 400 }, { width: 200, height: 100 });
    expect(mid.scale).toBeCloseTo((400 - FIT_PADDING * 2) / 200);
  });

  it("is safe for degenerate sizes", () => {
    for (const [c, s] of [
      [{ width: 0, height: 0 }, { width: 10, height: 10 }],
      [{ width: 100, height: 100 }, { width: 0, height: 10 }],
    ] as const) {
      expect(fit(c, s)).toEqual({ scale: 1, x: 0, y: 0 });
    }
    expect(Number.isFinite(fit({ width: 10, height: 10 }, { width: 5, height: 5 }).scale)).toBe(true);
  });

  it("clamps absurdly large diagrams to the minimum scale", () => {
    expect(fit({ width: 500, height: 500 }, { width: 1e7, height: 1e7 }).scale).toBe(MIN_SCALE);
  });
});

describe("zoomAt", () => {
  it("keeps the point under the cursor fixed", () => {
    const vp = { scale: 1, x: 40, y: 20 };
    const px = 300;
    const py = 200;
    // Content coordinate under the cursor before zooming.
    const cx = (px - vp.x) / vp.scale;
    const cy = (py - vp.y) / vp.scale;
    const z = zoomAt(vp, 2.5, px, py);
    expect(z.scale).toBeCloseTo(2.5);
    expect(cx * z.scale + z.x).toBeCloseTo(px);
    expect(cy * z.scale + z.y).toBeCloseTo(py);
  });

  it("clamps to the scale limits without drifting", () => {
    const top = zoomAt({ scale: MAX_SCALE, x: 10, y: 10 }, 5, 100, 100);
    expect(top).toEqual({ scale: MAX_SCALE, x: 10, y: 10 });
    expect(zoomAt({ scale: MIN_SCALE, x: 0, y: 0 }, 0.1, 5, 5).scale).toBe(MIN_SCALE);
  });

  it("zoom in then out returns to the start", () => {
    const start = { scale: 1.3, x: 12, y: -7 };
    const back = zoomAt(zoomAt(start, 1.7, 250, 180), 1 / 1.7, 250, 180);
    expect(back.scale).toBeCloseTo(start.scale);
    expect(back.x).toBeCloseTo(start.x);
    expect(back.y).toBeCloseTo(start.y);
  });
});

describe("zoomTo", () => {
  it("sets an absolute zoom around the container center", () => {
    const container = { width: 800, height: 600 };
    const z = zoomTo({ scale: 0.5, x: 100, y: 50 }, 1, container);
    expect(z.scale).toBeCloseTo(1);
    // The content point at the container center stays at the center.
    const before = { x: (400 - 100) / 0.5, y: (300 - 50) / 0.5 };
    expect(before.x * z.scale + z.x).toBeCloseTo(400);
    expect(before.y * z.scale + z.y).toBeCloseTo(300);
  });
});

describe("wheelFactor", () => {
  it("zooms in on scroll up and out on scroll down, symmetrically", () => {
    expect(wheelFactor(-100, 0, false)).toBeGreaterThan(1);
    expect(wheelFactor(100, 0, false)).toBeLessThan(1);
    expect(wheelFactor(-100, 0, false) * wheelFactor(100, 0, false)).toBeCloseTo(1);
  });

  it("limits the step from a huge delta and normalizes line mode", () => {
    expect(wheelFactor(-1e6, 0, false)).toBeCloseTo(wheelFactor(-200, 0, false));
    expect(wheelFactor(-3, 1, false)).toBeCloseTo(wheelFactor(-48, 0, false));
  });

  it("makes pinch (ctrlKey) more sensitive per pixel", () => {
    expect(wheelFactor(-5, 0, true)).toBeGreaterThan(wheelFactor(-5, 0, false));
  });
});

describe("parseSvgSize", () => {
  it("reads the viewBox", () => {
    expect(parseSvgSize('<svg xmlns="x" width="100%" viewBox="0 0 640.5 480">…</svg>')).toEqual({
      width: 640.5,
      height: 480,
    });
  });

  it("handles negative origins and comma separators", () => {
    expect(parseSvgSize('<svg viewBox="-12,-8,300,200">')).toEqual({ width: 300, height: 200 });
  });

  it("falls back to numeric width and height", () => {
    expect(parseSvgSize('<svg width="320px" height="240">')).toEqual({ width: 320, height: 240 });
  });

  it("rejects percentages, zero sizes, and non-svg input", () => {
    expect(parseSvgSize('<svg width="100%" height="100%">')).toBeNull();
    expect(parseSvgSize('<svg viewBox="0 0 0 0">')).toBeNull();
    expect(parseSvgSize("<div></div>")).toBeNull();
    expect(parseSvgSize("")).toBeNull();
  });

  it("only looks at the root element, not nested svgs", () => {
    const svg = '<svg viewBox="0 0 10 20"><svg viewBox="0 0 999 999"></svg></svg>';
    expect(parseSvgSize(svg)).toEqual({ width: 10, height: 20 });
  });
});
