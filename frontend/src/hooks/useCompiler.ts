import { useEffect, useRef, useState } from "react";
import { Compile } from "../../bindings/d2studio/compilerservice";
import type { Diagnostic, Meta, Options } from "../../bindings/d2studio/compiler/models";

export type CompileState = "idle" | "compiling" | "success" | "error";

// Interaction target, not a guarantee: compilation itself may take longer.
const DEBOUNCE_MS = 120;

export interface CompilerView {
  state: CompileState;
  /** Last SVG that compiled successfully. Kept while the current source fails. */
  svg: string;
  diagnostics: Diagnostic[];
  /** The exact source that `diagnostics` refer to. */
  diagnosticsSource: string;
  /** Problems that did not stop the diagram rendering, e.g. an image that failed to load. */
  warnings: Diagnostic[];
  meta: Meta | null;
}

const EMPTY: CompilerView = {
  state: "idle",
  svg: "",
  diagnostics: [],
  diagnosticsSource: "",
  warnings: [],
  meta: null,
};

/**
 * Debounced live compilation. Outdated results never replace newer ones: each
 * run gets an id, the previous in-flight call is cancelled, and anything that
 * is not the latest run (or that the backend marks "stale") is discarded.
 */
export function useCompiler(
  source: string,
  options: Options,
  path = "",
  workspace = "",
  /** Bump to recompile without a source change (an imported file changed). */
  revision = 0,
): CompilerView {
  const [view, setView] = useState<CompilerView>(EMPTY);
  const latest = useRef(0);
  const inflight = useRef<{ cancel: () => void } | null>(null);
  const optionsKey = JSON.stringify(options);

  useEffect(() => {
    const timer = setTimeout(() => {
      const id = ++latest.current;
      inflight.current?.cancel();
      setView((v) => ({ ...v, state: "compiling" }));

      const call = Compile({ source, options, path, workspace });
      inflight.current = call;

      call
        .then((res) => {
          if (id !== latest.current || !res || res.status === "stale") return;
          if (res.status === "success") {
            setView({
              state: "success",
              svg: res.svg,
              diagnostics: [],
              diagnosticsSource: source,
              warnings: res.warnings ?? [],
              meta: res.meta,
            });
          } else {
            // Keep the previous successful SVG and meta on error.
            setView((v) => ({
              ...v,
              state: "error",
              diagnostics: res.diagnostics ?? [],
              diagnosticsSource: source,
            }));
          }
        })
        .catch((err: unknown) => {
          if (id !== latest.current) return;
          const message = err instanceof Error ? err.message : String(err);
          setView((v) => ({
            ...v,
            state: "error",
            diagnostics: [
              {
                severity: "error",
                message: `Compiler unavailable: ${message}`,
                hasLocation: false,
                line: 0,
                column: 0,
                endLine: 0,
                endColumn: 0,
                from: 0,
                to: 0,
                path: "",
              } as Diagnostic,
            ],
            diagnosticsSource: source,
          }));
        });
    }, DEBOUNCE_MS);

    return () => clearTimeout(timer);
    // `options` is tracked through optionsKey.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [source, optionsKey, path, workspace, revision]);

  return view;
}
