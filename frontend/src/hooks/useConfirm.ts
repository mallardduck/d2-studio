import { useCallback, useRef, useState } from "react";
import type { DialogSpec } from "../components/ConfirmDialog";

/**
 * Promise-based modal confirmation. `confirm` has a stable identity so it is
 * safe in hook dependency lists. If a second dialog is requested while one is
 * open, the first resolves with its cancel option rather than hanging forever.
 */
export function useConfirm() {
  const [spec, setSpec] = useState<DialogSpec | null>(null);
  const pending = useRef<{ spec: DialogSpec; resolve: (id: string) => void } | null>(null);

  const confirm = useCallback(
    (next: DialogSpec) =>
      new Promise<string>((resolve) => {
        pending.current?.resolve(pending.current.spec.cancelId);
        pending.current = { spec: next, resolve };
        setSpec(next);
      }),
    [],
  );

  const choose = useCallback((id: string) => {
    const p = pending.current;
    pending.current = null;
    setSpec(null);
    p?.resolve(id);
  }, []);

  return { spec, confirm, choose };
}
