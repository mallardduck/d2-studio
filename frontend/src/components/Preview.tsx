import { useEffect, useState } from "react";
import type { CompileState } from "../hooks/useCompiler";

interface Props {
  svg: string;
  state: CompileState;
}

/**
 * Renders SVG through an <img> backed by a blob URL. Browsers do not run
 * scripts or event handlers in SVG loaded as an image, so a .d2 file from an
 * untrusted source cannot execute code in the app.
 */
export function Preview({ svg, state }: Props) {
  const [url, setUrl] = useState<string>("");

  useEffect(() => {
    if (!svg) {
      setUrl("");
      return;
    }
    const next = URL.createObjectURL(new Blob([svg], { type: "image/svg+xml" }));
    setUrl(next);
    return () => URL.revokeObjectURL(next);
  }, [svg]);

  if (!url) {
    return (
      <div className="flex h-full items-center justify-center text-fg-muted">
        {state === "error" ? "Fix the errors to see a preview." : "Preview will appear here."}
      </div>
    );
  }

  return (
    <div className="relative flex h-full items-center justify-center overflow-auto bg-surface-alt p-4">
      <img
        src={url}
        alt="Rendered D2 diagram"
        className={`max-h-full max-w-full object-contain ${state === "error" ? "opacity-50" : ""}`}
        draggable={false}
      />
      {state === "error" && (
        <div className="absolute left-3 top-3 rounded border border-line bg-surface px-2 py-1 text-warn">
          Showing last successful render
        </div>
      )}
    </div>
  );
}
