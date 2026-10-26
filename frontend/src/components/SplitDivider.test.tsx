// @vitest-environment jsdom
import { cleanup, fireEvent, render, screen } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";
import { SplitDivider } from "./SplitDivider";

afterEach(cleanup);

const box = (rect: Partial<DOMRect>) => ({ current: { getBoundingClientRect: () => ({ left: 0, top: 0, width: 0, height: 0, ...rect }) } }) as never;

function drag(el: HTMLElement, to: { clientX?: number; clientY?: number }) {
  el.setPointerCapture = vi.fn();
  el.releasePointerCapture = vi.fn();
  fireEvent.pointerDown(el, { pointerId: 1 });
  fireEvent.pointerMove(el, { pointerId: 1, ...to });
  fireEvent.pointerUp(el, { pointerId: 1 });
}

describe("SplitDivider vertical (default)", () => {
  it("follows the pointer horizontally and commits on release", () => {
    const onChange = vi.fn();
    render(<SplitDivider container={box({ left: 100, width: 400 })} ratio={0.5} onChange={onChange} />);
    drag(screen.getByRole("separator"), { clientX: 200 }); // (200-100)/400
    expect(onChange.mock.calls).toEqual([[0.25, false], [0.25, true]]);
  });
  it("uses the left and right arrows", () => {
    const onChange = vi.fn();
    render(<SplitDivider container={box({ width: 400 })} ratio={0.5} onChange={onChange} />);
    const sep = screen.getByRole("separator");
    expect(sep.getAttribute("aria-orientation")).toBe("vertical");
    fireEvent.keyDown(sep, { key: "ArrowLeft" });
    fireEvent.keyDown(sep, { key: "ArrowUp" }); // not this bar's axis
    expect(onChange).toHaveBeenCalledTimes(1);
    expect(onChange.mock.calls[0][0]).toBeCloseTo(0.48);
  });
});

describe("SplitDivider horizontal", () => {
  const horizontal = (onChange = vi.fn(), ratio = 0.5) => {
    render(<SplitDivider container={box({ top: 50, height: 500 })} ratio={ratio} orientation="horizontal" label="Resize files and snippets" onChange={onChange} />);
    return { onChange, sep: screen.getByRole("separator", { name: "Resize files and snippets" }) };
  };
  it("follows the pointer vertically", () => {
    const { onChange, sep } = horizontal();
    drag(sep, { clientY: 300 }); // (300-50)/500
    expect(onChange.mock.calls).toEqual([[0.5, false], [0.5, true]]);
    onChange.mockClear();
    drag(sep, { clientY: 150 }); // 0.2
    expect(onChange.mock.calls[1]).toEqual([0.2, true]);
  });
  it("clamps so neither section can be dragged away", () => {
    const { onChange, sep } = horizontal();
    drag(sep, { clientY: 0 });
    drag(sep, { clientY: 9999 });
    expect(onChange.mock.calls.map((c) => c[0])).toEqual([0.15, 0.15, 0.85, 0.85]);
  });
  it("uses the up and down arrows, plus Home, End and double-click to reset", () => {
    const { onChange, sep } = horizontal(undefined, 0.6);
    expect(sep.getAttribute("aria-orientation")).toBe("horizontal");
    fireEvent.keyDown(sep, { key: "ArrowDown" });
    fireEvent.keyDown(sep, { key: "ArrowRight" }); // not this bar's axis
    fireEvent.keyDown(sep, { key: "Home" });
    fireEvent.keyDown(sep, { key: "End" });
    fireEvent.doubleClick(sep);
    expect(onChange.mock.calls.map((c) => c[0])).toEqual([expect.closeTo(0.62), 0.15, 0.85, 0.5]);
  });
});
