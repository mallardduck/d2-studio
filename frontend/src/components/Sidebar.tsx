import type { Tree } from "../../bindings/d2studio/workspace/models";
import { FileTree } from "./FileTree";

interface Props {
  tree: Tree | null;
  error: string;
  activePath: string | null;
  onOpenFolder: () => void;
  onCloseFolder: () => void;
  onOpenFile: (path: string) => void;
}

const btn = "rounded border border-line bg-surface px-2 py-1 hover:bg-line/40";

export function Sidebar({ tree, error, activePath, onOpenFolder, onCloseFolder, onOpenFile }: Props) {
  return (
    <aside aria-label="Workspace" className="flex w-60 shrink-0 flex-col border-r border-line bg-surface-alt">
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
    </aside>
  );
}
