import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import { Themes } from "../bindings/d2studio/compilerservice";
import type { Diagnostic, Options, Theme } from "../bindings/d2studio/compiler/models";
import { Editor, type EditorHandle } from "./components/Editor";
import { Preview } from "./components/Preview";
import { Diagnostics } from "./components/Diagnostics";
import { StatusBar } from "./components/StatusBar";
import { Toolbar, MODES } from "./components/Toolbar";
import { SplitDivider } from "./components/SplitDivider";
import { useCompiler } from "./hooks/useCompiler";
import { clampRatio, useSettings, type ViewMode } from "./hooks/useSettings";
import { SAMPLE } from "./sample";

// Empty options: layout engine and theme come from the D2 source or D2's defaults.
const NO_OPTIONS: Options = { layout: "", themeId: null, sketch: null };

export default function App() {
  const [source, setSource] = useState(SAMPLE);
  const [themes, setThemes] = useState<Theme[]>([]);
  const editor = useRef<EditorHandle>(null);
  const main = useRef<HTMLElement>(null);
  const view = useCompiler(source, NO_OPTIONS);
  const { settings, update, saveError } = useSettings();

  // While dragging, the ratio is local; it is persisted once on release.
  const [dragRatio, setDragRatio] = useState<number | null>(null);

  useEffect(() => {
    Themes().then((t) => setThemes(t ?? []));
  }, []);

  const mode = (settings?.viewMode ?? "split") as ViewMode;
  const setMode = useCallback((m: ViewMode) => update({ viewMode: m }), [update]);

  // Cmd/Ctrl+1/2/3 switch view mode.
  useEffect(() => {
    const onKey = (e: KeyboardEvent) => {
      if (!(e.metaKey || e.ctrlKey) || e.altKey || e.shiftKey) return;
      const m = MODES.find((x) => x.key === e.key);
      if (!m) return;
      e.preventDefault();
      setMode(m.id);
    };
    window.addEventListener("keydown", onKey);
    return () => window.removeEventListener("keydown", onKey);
  }, [setMode]);

  const themeName = useMemo(() => {
    if (!view.meta) return "";
    return themes.find((t) => t.id === view.meta!.themeId)?.name ?? String(view.meta.themeId);
  }, [themes, view.meta]);

  const reveal = useCallback((d: Diagnostic) => editor.current?.reveal(d.from, d.to), []);

  // Wait for saved settings so the saved layout is applied on first paint.
  if (!settings) return <div className="h-full bg-surface" />;

  const ratio = clampRatio(dragRatio ?? settings.splitRatio);
  const showSource = mode !== "preview";
  const showPreview = mode !== "source";

  return (
    <div className="flex h-full flex-col">
      <Toolbar fileName="Untitled.d2" mode={mode} onMode={setMode} />

      <main ref={main} className="flex min-h-0 flex-1">
        {/* Both panes stay mounted so editor state survives mode changes. */}
        <section
          aria-label="Source"
          hidden={!showSource}
          style={mode === "split" ? { width: `${ratio * 100}%` } : undefined}
          className={`min-w-0 flex-col ${showSource ? "flex" : ""} ${mode === "split" ? "shrink-0" : "flex-1"}`}
        >
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

        {mode === "split" && (
          <SplitDivider
            container={main}
            ratio={ratio}
            onChange={(r, commit) => {
              setDragRatio(commit ? null : r);
              if (commit) update({ splitRatio: r });
            }}
          />
        )}

        <section aria-label="Preview" hidden={!showPreview} className="min-w-0 flex-1">
          <Preview svg={view.svg} state={view.state} />
        </section>
      </main>

      <StatusBar
        state={view.state}
        meta={view.meta}
        themeName={themeName}
        errorCount={view.diagnostics.length}
        notice={saveError ? `Settings not saved: ${saveError}` : ""}
      />
    </div>
  );
}
