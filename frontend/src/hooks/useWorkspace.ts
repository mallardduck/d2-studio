import { useCallback, useEffect, useRef, useState } from "react";
import { Events } from "@wailsio/runtime";
import { Close, OpenFolder, OpenFolderDialog } from "../../bindings/d2studio/workspace/service";
import type { Settings } from "../../bindings/d2studio/settings/models";
import type { Tree } from "../../bindings/d2studio/workspace/models";

const message = (e: unknown) => (e instanceof Error ? e.message : String(e));

/**
 * The folder open in the sidebar. It is restored from settings on startup,
 * kept in sync with disk through `workspace:changed` events, and exposes a
 * `revision` that bumps on every change so the compiler can re-run when an
 * imported file is edited.
 */
export function useWorkspace(settings: Settings | null, update: (patch: Partial<Settings>) => void) {
  const [tree, setTree] = useState<Tree | null>(null);
  const [error, setError] = useState("");
  const [revision, setRevision] = useState(0);
  const restored = useRef(false);

  // Reopen the remembered folder once settings have loaded.
  useEffect(() => {
    if (!settings || restored.current) return;
    restored.current = true;
    const saved = settings.workspace;
    if (!saved) return;
    OpenFolder(saved).then(
      (t) => setTree(t),
      (e) => {
        setError(`Could not reopen ${saved}: ${message(e)}`);
        update({ workspace: "" });
      },
    );
  }, [settings, update]);

  useEffect(() => {
    return Events.On("workspace:changed", (ev) => {
      const u = ev.data;
      if (!u) return;
      if (u.error) {
        setError(u.error);
      } else {
        setError("");
        setTree(u.tree);
      }
      setRevision((r) => r + 1);
    });
  }, []);

  const open = useCallback(
    async (startDir = "") => {
      try {
        const res = await OpenFolderDialog(startDir);
        if (!res || res.cancelled || !res.tree) return;
        setTree(res.tree);
        setError("");
        setRevision((r) => r + 1);
        update({ workspace: res.tree.root, sidebarOpen: true });
      } catch (e) {
        setError(`Could not open folder: ${message(e)}`);
      }
    },
    [update],
  );

  const close = useCallback(async () => {
    await Close();
    setTree(null);
    setError("");
    setRevision((r) => r + 1);
    update({ workspace: "" });
  }, [update]);

  return { tree, error, revision, root: tree?.root ?? "", open, close };
}
