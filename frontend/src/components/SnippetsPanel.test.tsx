// @vitest-environment jsdom
import { cleanup, fireEvent, render, screen, waitFor } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import type { Snippet } from "../../bindings/d2studio/snippets/models";
import { SnippetsPanel, matches } from "./SnippetsPanel";

const snip = (o: Partial<Snippet>): Snippet => ({
  id: "x",
  title: "T",
  category: "Basics",
  description: "d",
  code: "a -> b\n",
  remote: false,
  ...o,
});
const data = [
  snip({ id: "labels", title: "Labels", description: "Text after a colon" }),
  snip({ id: "grid", title: "Grid layout", category: "Special diagrams", code: "grid-columns: 3\n" }),
];

const writeText = vi.fn(() => Promise.resolve());
beforeEach(() => {
  writeText.mockClear();
  Object.assign(navigator, { clipboard: { writeText } });
});
afterEach(cleanup);

const setup = (over: Partial<Parameters<typeof SnippetsPanel>[0]> = {}) => {
  const props = { snippets: data, error: "", onOpen: vi.fn(), onInsert: vi.fn(), ...over };
  render(<SnippetsPanel {...props} />);
  return props;
};

describe("matches", () => {
  it("requires every word, in title, category, description or code", () => {
    expect(matches(data[1], "grid columns")).toBe(true);
    expect(matches(data[1], "special")).toBe(true);
    expect(matches(data[0], "grid")).toBe(false);
    expect(matches(data[0], "")).toBe(true);
  });
});

describe("SnippetsPanel", () => {
  it("lists only names, grouped by category, with no preview or code", () => {
    setup();
    expect(screen.getByRole("heading", { name: "Special diagrams" })).toBeTruthy();
    expect(screen.getByText("Grid layout")).toBeTruthy();
    expect(screen.queryByRole("img")).toBeNull();
    expect(screen.queryByText("grid-columns: 3", { exact: false })).toBeNull();
  });

  it("opens, inserts and copies the right snippet", async () => {
    const p = setup();
    fireEvent.click(screen.getByRole("button", { name: "Open Grid layout in a tab" }));
    expect(p.onOpen).toHaveBeenCalledWith("grid");
    fireEvent.click(screen.getByRole("button", { name: "Insert Grid layout" }));
    expect(p.onInsert).toHaveBeenCalledWith("grid-columns: 3\n");
    fireEvent.click(screen.getByRole("button", { name: "Copy Labels" }));
    expect(writeText).toHaveBeenCalledWith("a -> b\n");
    await waitFor(() => expect(screen.getByRole("button", { name: "Copy Labels" }).textContent).toBe("Copied"));
  });

  it("filters as you type and says when nothing matches", () => {
    setup();
    fireEvent.change(screen.getByLabelText("Search snippets"), { target: { value: "grid" } });
    expect(screen.queryByText("Labels")).toBeNull();
    expect(screen.getByText("Grid layout")).toBeTruthy();
    fireEvent.change(screen.getByLabelText("Search snippets"), { target: { value: "zzz" } });
    expect(screen.getByText("No snippets match.")).toBeTruthy();
  });

  it("reports a load error", () => {
    setup({ snippets: [], error: "boom" });
    expect(screen.getByRole("alert").textContent).toContain("boom");
  });
});
