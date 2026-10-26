import { describe, expect, it } from "vitest";
import { DOC_TAB, closeTab, initialTabs, openTab, pruneTabs } from "./model";

describe("tabs", () => {
  it("opens a snippet and makes it active", () => {
    expect(openTab(initialTabs, "a")).toEqual({ open: ["a"], active: "a" });
  });
  it("re-opening switches to the existing tab instead of duplicating it", () => {
    let t = openTab(openTab(initialTabs, "a"), "b");
    t = openTab({ ...t, active: DOC_TAB }, "a");
    expect(t).toEqual({ open: ["a", "b"], active: "a" });
  });
  it("closing the active tab activates its left neighbour, else the document", () => {
    const t = openTab(openTab(openTab(initialTabs, "a"), "b"), "c");
    expect(closeTab(t, "c")).toEqual({ open: ["a", "b"], active: "b" });
    expect(closeTab({ ...t, active: "a" }, "a")).toEqual({ open: ["b", "c"], active: DOC_TAB });
  });
  it("closing a background tab keeps the active one", () => {
    const t = openTab(openTab(initialTabs, "a"), "b");
    expect(closeTab(t, "a")).toEqual({ open: ["b"], active: "b" });
  });
  it("ignores closing a tab that is not open", () => {
    expect(closeTab(initialTabs, "zzz")).toBe(initialTabs);
  });
  it("drops tabs for snippets that no longer exist", () => {
    const t = openTab(openTab(initialTabs, "a"), "b");
    expect(pruneTabs(t, new Set(["a", "b"]))).toBe(t);
    expect(pruneTabs(t, new Set(["a"]))).toEqual({ open: ["a"], active: DOC_TAB });
  });
});
