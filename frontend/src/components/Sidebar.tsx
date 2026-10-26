import type { Tree } from "../../bindings/d2studio/workspace/models";
import { useRef, useState } from "react";
import type { Snippet } from "../../bindings/d2studio/snippets/models";
import { FileTree } from "./FileTree";
import { SnippetsPanel } from "./SnippetsPanel";
import { SplitDivider } from "./SplitDivider";
import { clampRatio } from "../hooks/useSettings";

interface Props {
  tree: Tree | null;
  error: string;
  activePath: string | null;
  onOpenFolder: () => void;
  onCloseFolder: () => void;
  onOpenFile: (path: string) => void;
  /** Fraction of the sidebar's height given to the file tree. */
  split: number;
  /** Called once, when a resize is finished. */
  onSplitChange: (ratio: number) => void;
  snippets: Snippet[];
  snippetsError: string;
  onOpenSnippet: (id: string) => void;
  onInsertSnippet: (code: string) => void;
}

const btn = "rounded border border-line bg-surface px-2 py-1 hover:bg-line/40";

export function Sidebar({ tree, error, activePath, onOpenFolder, onCloseFolder, onOpenFile, split, onSplitChange, snippets, snippetsError, onOpenSnippet, onInsertSnippet }: Props) {
  const [snippetsOpen, setSnippetsOpen] = useState(true);
  const aside = useRef<HTMLElement>(null);
  // While dragging the ratio is local; it is saved once on release.
  const [drag, setDrag] = useState<number | null>(null);
  const ratio = clampRatio(drag ?? split);
  return (
    <aside ref={aside} aria-label="Sidebar" className="flex w-64 shrink-0 flex-col border-r border-line bg-surface-alt">
      <section
        aria-label="Workspace"
        style={snippetsOpen ? { flexBasis: `${ratio * 100}%` } : undefined}
        className={`flex min-h-0 flex-col ${snippetsOpen ? "shrink-0 grow-0" : "flex-1"}`}
      >
      {tree ? (
        <>
          <div className="flex items-center gap-1 border-b border-line px-2 py-1.5">
            <span className="min-w-0 flex-1 truncate font-semibold" title={tree.root}>
              {tree.name}
            </span>
            <button
              type="button"
              className="rounded px-1.5 hover:bg-line/40"
              aria-label="Close folder"
              title="Close folder"
              onClick={onCloseFolder}
            >
              ×
            </button>
          </div>
          {error && (
            <div role="alert" className="border-b border-line px-2 py-1.5 text-warn">
              {error}
            </div>
          )}
          <div className="min-h-0 flex-1 overflow-auto">
            {tree.fileCount === 0 ? (
              <p className="px-3 py-2 text-fg-muted">No .d2 files in this folder.</p>
            ) : (
              <FileTree nodes={tree.nodes} activePath={activePath} onOpen={onOpenFile} />
            )}
          </div>
          {tree.truncated && (
            <p className="border-t border-line px-2 py-1.5 text-fg-muted">
              This folder is very large or deeply nested, so some of it was not scanned and some files may be missing.
              Open a smaller folder to see everything.
            </p>
          )}
        </>
      ) : (
        <div className="flex flex-col items-start gap-2 p-3">
          <p className="text-fg-muted">Open a folder to browse its D2 files. Imports can use any file inside it.</p>
          <button type="button" className={btn} onClick={onOpenFolder}>
            Open Folder…
          </button>
          {error && (
            <p role="alert" className="text-warn">
              {error}
            </p>
          )}
        </div>
      )}
      </section>
      {snippetsOpen && (
        <SplitDivider
          container={aside}
          orientation="horizontal"
          label="Resize files and snippets"
          ratio={ratio}
          onChange={(r, commit) => {
            setDrag(commit ? null : r);
            if (commit) onSplitChange(r);
          }}
        />
      )}
      <section
        aria-label="Snippets"
        className={`flex min-h-0 flex-col ${snippetsOpen ? "flex-1" : "border-t border-line"}`}
      >
        <h2 className="m-0 text-base font-normal">
          <button
            type="button"
            aria-expanded={snippetsOpen}
            onClick={() => setSnippetsOpen(!snippetsOpen)}
            className="flex w-full items-center gap-1 px-2 py-1.5 text-left font-semibold hover:bg-line/40"
          >
            <span aria-hidden="true">{snippetsOpen ? "▾" : "▸"}</span> Snippets
          </button>
        </h2>
        {snippetsOpen && <SnippetsPanel snippets={snippets} error={snippetsError} onOpen={onOpenSnippet} onInsert={onInsertSnippet} />}
      </section>
    </aside>
  );
}
