import { useCallback, useRef, useState } from "react";
import { Export } from "../../bindings/d2studio/export/service";
import type { Format } from "../../bindings/d2studio/export/models";
import type { Options } from "../../bindings/d2studio/compiler/models";
import { describeResult } from "../export/model";
import { baseName } from "../files/model";

interface Deps {
  /** What to compile: the editor's current content and settings. */
  compile: { source: string; options: Options; path: string; workspace: string };
  documentName: string;
  /** Folder the save dialog starts in. */
  dir: string;
  allowRemoteImages: boolean;
  notify: (text: string) => void;
  showError: (text: string) => void;
}

/** Runs one export at a time and reports exactly what happened. */
export function useExport(deps: Deps) {
  const [busy, setBusy] = useState(false);
  const running = useRef(false);
  const latest = useRef(deps);
  latest.current = deps;

  const run = useCallback(async (format: Format) => {
    if (running.current) return;
    running.current = true;
    setBusy(true);
    const d = latest.current;
    try {
      const result = await Export({
        format,
        compile: { ...d.compile },
        suggestedName: d.documentName,
        dir: d.dir,
        allowRemoteImages: d.allowRemoteImages,
      });
      const outcome = describeResult(result, baseName);
      if (outcome.kind === "done") d.notify(outcome.message);
      else if (outcome.kind === "errors") d.showError(outcome.message);
    } catch (e) {
      d.showError(`Export failed: ${e instanceof Error ? e.message : String(e)}`);
    } finally {
      running.current = false;
      setBusy(false);
    }
  }, []);

  return { busy, run };
}
