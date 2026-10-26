// @vitest-environment jsdom
import { cleanup, fireEvent, render, screen } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";
import { TabBar } from "./TabBar";

afterEach(cleanup);
const tabs = [
  { id: "doc", label: "notes.d2", suffix: " •" },
  { id: "a", label: "Labels" },
  { id: "b", label: "Grid" },
];

describe("TabBar", () => {
  it("marks the active tab and shows the unsaved marker", () => {
    render(<TabBar tabs={tabs} active="a" onSelect={vi.fn()} onClose={vi.fn()} />);
    expect(screen.getByRole("tab", { name: "Labels" }).getAttribute("aria-selected")).toBe("true");
    expect(screen.getByRole("tab", { name: "notes.d2 •" }).getAttribute("aria-selected")).toBe("false");
  });

  it("the document tab cannot be closed; snippet tabs can", () => {
    const onClose = vi.fn();
    render(<TabBar tabs={tabs} active="doc" onSelect={vi.fn()} onClose={onClose} />);
    expect(screen.queryByRole("button", { name: "Close notes.d2" })).toBeNull();
    fireEvent.click(screen.getByRole("button", { name: "Close Grid" }));
    expect(onClose).toHaveBeenCalledWith("b");
  });

  it("arrow keys, Home and End move between tabs, wrapping around", () => {
    const onSelect = vi.fn();
    render(<TabBar tabs={tabs} active="b" onSelect={onSelect} onClose={vi.fn()} />);
    const list = screen.getByRole("tablist");
    fireEvent.keyDown(list, { key: "ArrowRight" });
    fireEvent.keyDown(list, { key: "ArrowLeft" });
    fireEvent.keyDown(list, { key: "Home" });
    fireEvent.keyDown(list, { key: "End" });
    expect(onSelect.mock.calls.map((c) => c[0])).toEqual(["doc", "a", "doc", "b"]);
  });

  it("only the active tab is in the tab order", () => {
    render(<TabBar tabs={tabs} active="a" onSelect={vi.fn()} onClose={vi.fn()} />);
    expect(screen.getByRole("tab", { name: "Labels" }).tabIndex).toBe(0);
    expect(screen.getByRole("tab", { name: "Grid" }).tabIndex).toBe(-1);
  });
});
