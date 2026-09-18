import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import { Themes } from "../bindings/d2studio/compilerservice";
import type { Diagnostic, Options, Theme } from "../bindings/d2studio/compiler/models";
import { Editor, type EditorHandle } from "./components/Editor";
import { Preview } from "./components/Preview";
import { Diagnostics } from "./components/Diagnostics";
import { StatusBar } from "./components/StatusBar";
import { useCompiler } from "./hooks/useCompiler";
import { SAMPLE } from "./sample";

// Empty options: layout engine and theme come from the D2 source or D2's defaults.
const NO_OPTIONS: Options = { layout: "", themeId: null, sketch: null };

export default function App() {
  const [source, setSource] = useState(SAMPLE);
  const [themes, setThemes] = useState<Theme[]>([]);
  const editor = useRef<EditorHandle>(null);
  const view = useCompiler(source, NO_OPTIONS);

  useEffect(() => {
    Themes().then((t) => setThemes(t ?? []));
  }, []);

  const themeName = useMemo(() => {
    if (!view.meta) return "";
    return themes.find((t) => t.id === view.meta!.themeId)?.name ?? String(view.meta.themeId);
  }, [themes, view.meta]);

  const reveal = useCallback((d: Diagnostic) => editor.current?.reveal(d.from, d.to), []);

  return (
    <div className="flex h-full flex-col">
      <header className="flex h-10 shrink-0 items-center gap-3 border-b border-line bg-surface-alt px-3">
        <h1 className="text-sm font-semibold">D2 Studio</h1>
        <span className="text-fg-muted">Untitled.d2</span>
      </header>

      <main className="flex min-h-0 flex-1">
        <section aria-label="Source" className="flex w-1/2 min-w-0 flex-col border-r border-line">
          <div className="min-h-0 flex-1">
            <Editor
              ref={editor}
              initialDoc={SAMPLE}
              onChange={setSource}
              diagnostics={view.diagnostics}
              diagnosticsSource={view.diagnosticsSource}
            />
          </div>
          <Diagnostics diagnostics={view.diagnostics} onSelect={reveal} />
        </section>

        <section aria-label="Preview" className="w-1/2 min-w-0">
          <Preview svg={view.svg} state={view.state} />
        </section>
      </main>

      <StatusBar
        state={view.state}
        meta={view.meta}
        themeName={themeName}
        errorCount={view.diagnostics.length}
      />
    </div>
  );
}
