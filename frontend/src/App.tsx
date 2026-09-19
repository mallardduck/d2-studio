import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import { Engines, Themes } from "../bindings/d2studio/compilerservice";
import type { Diagnostic, Engine, Options, Theme } from "../bindings/d2studio/compiler/models";
import { Editor, type EditorHandle } from "./components/Editor";
import { Preview } from "./components/Preview";
import { Diagnostics } from "./components/Diagnostics";
import { StatusBar } from "./components/StatusBar";
import { Toolbar, MODES } from "./components/Toolbar";
import { OptionsBar } from "./components/OptionsBar";
import { SplitDivider } from "./components/SplitDivider";
import { useCompiler } from "./hooks/useCompiler";
import { clampRatio, useSettings, type ViewMode } from "./hooks/useSettings";
import { SAMPLE } from "./sample";

export default function App() {
  const [source, setSource] = useState(SAMPLE);
  const [themes, setThemes] = useState<Theme[]>([]);
  const editor = useRef<EditorHandle>(null);
  const main = useRef<HTMLElement>(null);
  const [engines, setEngines] = useState<Engine[]>([]);
  const { settings, update, saveError } = useSettings();

  // Empty layout / null theme mean "follow the D2 source, else D2's default".
  const options = useMemo<Options>(
    () => ({ layout: settings?.layout ?? "", themeId: settings?.themeId ?? null, sketch: null }),
    [settings?.layout, settings?.themeId],
  );
  const view = useCompiler(source, options);

  // While dragging, the ratio is local; it is persisted once on release.
  const [dragRatio, setDragRatio] = useState<number | null>(null);

  useEffect(() => {
    Themes().then((t) => setThemes(t ?? []));
    Engines().then((e) => setEngines(e ?? []));
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

  const reveal = useCallback((d: Diagnostic) => editor.current?.reveal(d.from, d.to), []);

  // Wait for saved settings so the saved layout is applied on first paint.
  if (!settings) return <div className="h-full bg-surface" />;

  const ratio = clampRatio(dragRatio ?? settings.splitRatio);
  const showSource = mode !== "preview";
  const showPreview = mode !== "source";

  return (
    <div className="flex h-full flex-col">
      <Toolbar
        fileName="Untitled.d2"
        mode={mode}
        onMode={setMode}
        options={
          <OptionsBar
            engines={engines}
            themes={themes}
            layout={settings.layout}
            themeId={settings.themeId}
            meta={view.meta}
            onLayout={(layout) => update({ layout })}
            onTheme={(themeId) => update({ themeId })}
          />
        }
      />

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
        themes={themes}
        errorCount={view.diagnostics.length}
        notice={saveError ? `Settings not saved: ${saveError}` : ""}
      />
    </div>
  );
}
