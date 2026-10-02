import { describe, expect, it } from "vitest";
import type { Node } from "../../bindings/d2studio/workspace/models";
import { ancestorsOf, navigate, visibleRows } from "./treeModel";

const file = (path: string): Node => ({ name: path.split("/").pop()!, path, dir: false, children: [] });
const dir = (path: string, children: Node[]): Node => ({ name: path.split("/").pop()!, path, dir: true, children });

// /w
//   a/            (expanded)
//     one.d2
//     b/
//       deep.d2
//   z.d2
const nodes: Node[] = [dir("/w/a", [file("/w/a/one.d2"), dir("/w/a/b", [file("/w/a/b/deep.d2")])]), file("/w/z.d2")];
const none = new Set<string>();
const rowsOf = (collapsed: Set<string> = none) => visibleRows(nodes, collapsed);
const paths = (collapsed?: Set<string>) => rowsOf(collapsed).map((r) => r.node.path);

describe("visibleRows", () => {
  it("lists everything depth-first when nothing is collapsed", () => {
    expect(paths()).toEqual(["/w/a", "/w/a/one.d2", "/w/a/b", "/w/a/b/deep.d2", "/w/z.d2"]);
    expect(rowsOf().map((r) => r.depth)).toEqual([0, 1, 1, 2, 0]);
  });

  it("hides the contents of collapsed folders", () => {
    expect(paths(new Set(["/w/a"]))).toEqual(["/w/a", "/w/z.d2"]);
    expect(paths(new Set(["/w/a/b"]))).toEqual(["/w/a", "/w/a/one.d2", "/w/a/b", "/w/z.d2"]);
  });

  it("records each row's parent and expanded state", () => {
    const rows = rowsOf(new Set(["/w/a/b"]));
    expect(rows.find((r) => r.node.path === "/w/a/one.d2")?.parent).toBe("/w/a");
    expect(rows.find((r) => r.node.path === "/w/a/b")?.expanded).toBe(false);
    expect(rows[0].parent).toBeNull();
  });

  it("tolerates the nulls Go produces for empty slices", () => {
    const odd = [{ name: "x", path: "/x", dir: true, children: null }, null] as unknown as Node[];
    expect(visibleRows(odd, none).map((r) => r.node.path)).toEqual(["/x"]);
    expect(visibleRows(null, none)).toEqual([]);
  });
});

describe("ancestorsOf", () => {
  it("returns the containing folders, outermost first", () => {
    expect(ancestorsOf(nodes, "/w/a/b/deep.d2")).toEqual(["/w/a", "/w/a/b"]);
    expect(ancestorsOf(nodes, "/w/z.d2")).toEqual([]);
  });
  it("returns null for a file that is not in the tree", () => {
    expect(ancestorsOf(nodes, "/elsewhere.d2")).toBeNull();
  });
});

describe("keyboard navigation", () => {
  const nav = (current: string | null, key: string, collapsed?: Set<string>) =>
    navigate(rowsOf(collapsed), current, key);

  it("moves up and down, stopping at the ends", () => {
    expect(nav("/w/a", "ArrowDown")).toEqual({ type: "focus", path: "/w/a/one.d2" });
    expect(nav("/w/a/one.d2", "ArrowUp")).toEqual({ type: "focus", path: "/w/a" });
    expect(nav("/w/a", "ArrowUp")).toEqual({ type: "focus", path: "/w/a" });
    expect(nav("/w/z.d2", "ArrowDown")).toEqual({ type: "focus", path: "/w/z.d2" });
  });

  it("jumps with Home and End", () => {
    expect(nav("/w/a/b", "Home")).toEqual({ type: "focus", path: "/w/a" });
    expect(nav("/w/a", "End")).toEqual({ type: "focus", path: "/w/z.d2" });
  });

  it("Right expands a collapsed folder, then enters it", () => {
    expect(nav("/w/a", "ArrowRight", new Set(["/w/a"]))).toEqual({ type: "expand", path: "/w/a" });
    expect(nav("/w/a", "ArrowRight")).toEqual({ type: "focus", path: "/w/a/one.d2" });
  });

  it("Right does nothing on a file", () => {
    expect(nav("/w/z.d2", "ArrowRight")).toEqual({ type: "none" });
  });

  it("Left collapses an open folder, otherwise moves to the parent", () => {
    expect(nav("/w/a", "ArrowLeft")).toEqual({ type: "collapse", path: "/w/a" });
    expect(nav("/w/a/b/deep.d2", "ArrowLeft")).toEqual({ type: "focus", path: "/w/a/b" });
    expect(nav("/w/a/b", "ArrowLeft", new Set(["/w/a/b"]))).toEqual({ type: "focus", path: "/w/a" });
    expect(nav("/w/z.d2", "ArrowLeft")).toEqual({ type: "none" }); // top level: no parent
  });

  it("Enter and Space open files and toggle folders", () => {
    for (const key of ["Enter", " "]) {
      expect(nav("/w/z.d2", key)).toEqual({ type: "open", path: "/w/z.d2" });
      expect(nav("/w/a", key)).toEqual({ type: "collapse", path: "/w/a" });
      expect(nav("/w/a", key, new Set(["/w/a"]))).toEqual({ type: "expand", path: "/w/a" });
    }
  });

  it("is inert for unknown keys, an empty tree, or an unknown current row", () => {
    expect(nav("/w/a", "x")).toEqual({ type: "none" });
    expect(navigate([], null, "ArrowDown")).toEqual({ type: "none" });
    expect(nav("/gone.d2", "ArrowRight")).toEqual({ type: "none" });
    expect(nav(null, "ArrowDown")).toEqual({ type: "focus", path: "/w/a" });
  });
});
