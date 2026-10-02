import type { Node } from "../../bindings/d2studio/workspace/models";

/** One visible line of the tree. */
export interface Row {
  node: Node;
  depth: number;
  /** Only meaningful for folders. */
  expanded: boolean;
  /** Path of the containing folder, or null at the top level. */
  parent: string | null;
}

/** Go serializes empty slices as null; treat that as "no children". */
export const childrenOf = (n: Node): Node[] => (n.children ?? []).filter((c): c is Node => c !== null);

/** The rows currently visible, given which folders the user collapsed. */
export function visibleRows(
  nodes: (Node | null)[] | null,
  collapsed: ReadonlySet<string>,
  depth = 0,
  parent: string | null = null,
): Row[] {
  const rows: Row[] = [];
  for (const node of nodes ?? []) {
    if (!node) continue;
    const expanded = node.dir && !collapsed.has(node.path);
    rows.push({ node, depth, expanded, parent });
    if (node.dir && expanded) rows.push(...visibleRows(node.children, collapsed, depth + 1, node.path));
  }
  return rows;
}

/** Folders that contain `path`, outermost first. */
export function ancestorsOf(nodes: (Node | null)[] | null, path: string, trail: string[] = []): string[] | null {
  for (const node of nodes ?? []) {
    if (!node) continue;
    if (node.path === path) return trail;
    if (node.dir) {
      const found = ancestorsOf(node.children, path, [...trail, node.path]);
      if (found) return found;
    }
  }
  return null;
}

export type Navigation =
  | { type: "focus"; path: string }
  | { type: "expand"; path: string }
  | { type: "collapse"; path: string }
  | { type: "open"; path: string }
  | { type: "none" };

/**
 * Keyboard behaviour of a tree, following the WAI-ARIA tree pattern:
 * Up/Down move, Right expands a folder or enters it, Left collapses it or
 * moves to the parent, Home/End jump, Enter/Space opens a file or toggles a
 * folder.
 */
export function navigate(rows: readonly Row[], current: string | null, key: string): Navigation {
  if (rows.length === 0) return { type: "none" };
  const i = rows.findIndex((r) => r.node.path === current);
  const row = i >= 0 ? rows[i] : null;
  const focus = (r: Row | undefined): Navigation => (r ? { type: "focus", path: r.node.path } : { type: "none" });

  switch (key) {
    case "ArrowDown":
      return focus(rows[Math.min(rows.length - 1, i + 1)]);
    case "ArrowUp":
      return focus(rows[Math.max(0, i - 1)]);
    case "Home":
      return focus(rows[0]);
    case "End":
      return focus(rows[rows.length - 1]);
    case "ArrowRight":
      if (!row || !row.node.dir) return { type: "none" };
      if (!row.expanded) return { type: "expand", path: row.node.path };
      return focus(rows[i + 1]?.parent === row.node.path ? rows[i + 1] : undefined);
    case "ArrowLeft":
      if (!row) return { type: "none" };
      if (row.node.dir && row.expanded) return { type: "collapse", path: row.node.path };
      return focus(rows.find((r) => r.node.path === row.parent));
    case "Enter":
    case " ":
      if (!row) return { type: "none" };
      if (!row.node.dir) return { type: "open", path: row.node.path };
      return row.expanded ? { type: "collapse", path: row.node.path } : { type: "expand", path: row.node.path };
    default:
      return { type: "none" };
  }
}
