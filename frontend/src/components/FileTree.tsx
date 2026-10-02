import { useEffect, useMemo, useRef, useState, type KeyboardEvent } from "react";
import type { Node } from "../../bindings/d2studio/workspace/models";
import { ancestorsOf, navigate, visibleRows } from "../workspace/treeModel";

interface Props {
  nodes: (Node | null)[] | null;
  /** Absolute path of the file open in the editor, if it is in this tree. */
  activePath: string | null;
  onOpen: (path: string) => void;
}

/**
 * Accessible file tree (WAI-ARIA tree pattern): one tab stop, arrow keys to
 * move, Right/Left to expand and collapse, Enter to open.
 */
export function FileTree({ nodes, activePath, onOpen }: Props) {
  const [collapsed, setCollapsed] = useState<ReadonlySet<string>>(new Set());
  const [focused, setFocused] = useState<string | null>(null);
  const root = useRef<HTMLUListElement>(null);

  const rows = useMemo(() => visibleRows(nodes, collapsed), [nodes, collapsed]);

  // Opening a file from elsewhere (recent files, Open...) should reveal it.
  useEffect(() => {
    if (!activePath) return;
    const trail = ancestorsOf(nodes, activePath);
    if (!trail) return;
    setCollapsed((c) => (trail.some((p) => c.has(p)) ? new Set([...c].filter((p) => !trail.includes(p))) : c));
  }, [activePath, nodes]);

  // One row is the tab stop: the focused one, else the active file, else the first.
  const stop =
    rows.find((r) => r.node.path === focused)?.node.path ??
    rows.find((r) => r.node.path === activePath)?.node.path ??
    rows[0]?.node.path ??
    null;

  const toggle = (path: string, expand: boolean) =>
    setCollapsed((c) => {
      const next = new Set(c);
      if (expand) next.delete(path);
      else next.add(path);
      return next;
    });

  const focusRow = (path: string) => {
    setFocused(path);
    root.current?.querySelector<HTMLElement>(`[data-path="${CSS.escape(path)}"]`)?.focus();
  };

  const onKeyDown = (e: KeyboardEvent<HTMLUListElement>) => {
    if (e.ctrlKey || e.metaKey || e.altKey) return;
    const action = navigate(rows, stop, e.key);
    if (action.type === "none") return;
    e.preventDefault();
    if (action.type === "focus") focusRow(action.path);
    else if (action.type === "expand") toggle(action.path, true);
    else if (action.type === "collapse") toggle(action.path, false);
    else onOpen(action.path);
  };

  return (
    <ul ref={root} role="tree" aria-label="Diagram files" onKeyDown={onKeyDown} className="py-1">
      {rows.map(({ node, depth, expanded }) => {
        const isActive = node.path === activePath;
        return (
          <li
            key={node.path}
            role="treeitem"
            aria-level={depth + 1}
            aria-expanded={node.dir ? expanded : undefined}
            aria-selected={isActive}
            aria-current={isActive ? "page" : undefined}
            data-path={node.path}
            tabIndex={node.path === stop ? 0 : -1}
            title={node.path}
            onFocus={() => setFocused(node.path)}
            onClick={() => (node.dir ? toggle(node.path, !expanded) : onOpen(node.path))}
            style={{ paddingLeft: `${depth * 14 + 8}px` }}
            className={`flex cursor-pointer items-center gap-1.5 truncate py-0.5 pr-2 ${
              isActive ? "bg-accent/15 font-medium" : "hover:bg-line/40"
            }`}
          >
            <span aria-hidden="true" className="w-3 shrink-0 text-center text-fg-muted">
              {node.dir ? (expanded ? "▾" : "▸") : ""}
            </span>
            <span className="truncate">{node.name}</span>
          </li>
        );
      })}
    </ul>
  );
}
