import type { Extension } from "@codemirror/state";
import { useEffect, useState } from "react";
import { Preview as RenderSnippet } from "../../bindings/d2studio/snippetservice";
import type { Snippet } from "../../bindings/d2studio/snippets/models";
import { useCopy } from "../hooks/useCopy";
import type { CompileState } from "../hooks/useCompiler";
import { Preview } from "./Preview";
import { ReadOnlyCode } from "./ReadOnlyCode";

interface Props {
  snippet: Snippet;
  language?: Extension;
  onInsert: (code: string) => void;
}

const btn = "rounded border border-line bg-surface px-2 py-1 hover:bg-line/40";

/** A snippet shown in its own tab: description, rendered preview and read-only code. */
export function SnippetTab({ snippet, language, onInsert }: Props) {
  const [svg, setSvg] = useState("");
  const [state, setState] = useState<CompileState>("compiling");
  const [error, setError] = useState("");
  const { copied, copy } = useCopy();

  useEffect(() => {
    let live = true;
    setState("compiling");
    RenderSnippet(snippet.id).then(
      (s) => {
        if (!live) return;
        setSvg(s);
        setState("success");
      },
      (e) => {
        if (!live) return;
        setError(String(e));
        setState("error");
      },
    );
    return () => {
      live = false;
    };
  }, [snippet.id]);

  return (
    <div className="flex min-h-0 min-w-0 flex-1 flex-col">
      <div className="flex flex-wrap items-center gap-2 border-b border-line bg-surface-alt px-3 py-2">
        <div className="min-w-0 flex-1">
          <h2 className="m-0 truncate text-base font-semibold">{snippet.title}</h2>
          <p className="m-0 text-fg-muted">{snippet.description}</p>
        </div>
        <button type="button" className={btn} onClick={() => onInsert(snippet.code)}>
          Insert into document
        </button>
        <button type="button" className={btn} onClick={() => copy(snippet.id, snippet.code)}>
          {copied === snippet.id ? "Copied" : "Copy"}
        </button>
      </div>
      {snippet.remote && (
        <p className="border-b border-line px-3 py-1.5 text-fg-muted">
          This example uses an image from the web. It is not loaded here; turn on Remote images for your file to see it in
          the preview after inserting.
        </p>
      )}
      <div className="flex min-h-0 flex-1">
        <section aria-label="Snippet code" className="min-w-0 flex-1 border-r border-line">
          <ReadOnlyCode code={snippet.code} language={language} label={`${snippet.title} source (read only)`} />
        </section>
        <section aria-label="Snippet preview" className="min-w-0 flex-1">
          {error ? (
            <p role="alert" className="p-3 text-warn">
              Preview unavailable: {error}
            </p>
          ) : (
            <Preview svg={svg} state={state} />
          )}
        </section>
      </div>
    </div>
  );
}
