import type { Extension } from "@codemirror/state";
import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import { Engines, Language, Themes } from "../bindings/d2studio/compilerservice";
import type { Diagnostic, Engine, Options, Theme } from "../bindings/d2studio/compiler/models";
import { Editor, type EditorHandle } from "./components/Editor";
import { Preview } from "./components/Preview";
import { Diagnostics } from "./components/Diagnostics";
import { StatusBar } from "./components/StatusBar";
import { Toolbar, MODES } from "./components/Toolbar";
import { OptionsBar } from "./components/OptionsBar";
import { SplitDivider } from "./components/SplitDivider";
import { ConfirmDialog } from "./components/ConfirmDialog";
import { FileBanner } from "./components/FileBanner";
import { FileControls } from "./components/FileControls";
import { Sidebar } from "./components/Sidebar";
import { ExportMenu } from "./components/ExportMenu";
import { useCompiler } from "./hooks/useCompiler";
import { useConfirm } from "./hooks/useConfirm";
import { useDocument } from "./hooks/useDocument";
import { useWorkspace } from "./hooks/useWorkspace";
import { useExport } from "./hooks/useExport";
import { dirOf } from "./files/model";
import { clampRatio, useSettings, type ViewMode } from "./hooks/useSettings";
import { d2Language } from "./editor/d2lang";
import { SAMPLE } from "./sample";

export default function App() {
  const [source, setSource] = useState(SAMPLE);
  const [themes, setThemes] = useState<Theme[]>([]);
  const editor = useRef<EditorHandle>(null);
  const main = useRef<HTMLElement>(null);
  const [engines, setEngines] = useState<Engine[]>([]);
  const [language, setLanguage] = useState<Extension>();
  const [zoom, setZoom] = useState<number | null>(null);
  const { settings, update, saveError } = useSettings();

  // Empty layout / null theme mean "follow the D2 source, else D2's default".
  const options = useMemo<Options>(
    () => ({ layout: settings?.layout ?? "", themeId: settings?.themeId ?? null, sketch: null }),
    [settings?.layout, settings?.themeId],
  );
  const { spec: dialog, confirm, choose } = useConfirm();
  const setContent = useCallback((text: string) => {
    editor.current?.setDoc(text);
    setSource(text);
  }, []);
  const docs = useDocument({ content: source, setContent, confirm, initialContent: SAMPLE });
  const workspace = useWorkspace(settings, update);
  const view = useCompiler(source, options, docs.path ?? "", workspace.root, workspace.revision);
  const exporter = useExport({
    compile: { source, options, path: docs.path ?? "", workspace: workspace.root },
    documentName: docs.name,
    dir: dirOf(docs.path),
    allowRemoteImages: settings?.exportRemoteImages ?? false,
    notify: docs.notify,
    showError: docs.showError,
  });

  // While dragging, the ratio is local; it is persisted once on release.
  const [dragRatio, setDragRatio] = useState<number | null>(null);

  useEffect(() => {
    Themes().then((t) => setThemes(t ?? []));
    Engines().then((e) => setEngines(e ?? []));
    Language().then((l) => setLanguage(d2Language(l)));
  }, []);

  // Cmd/Ctrl+N, O, S, Shift+S, Shift+O (folder) and B (sidebar).
  const { newDocument, open, save, saveAs } = docs;
  useEffect(() => {
    const onKey = (e: KeyboardEvent) => {
      if (!(e.metaKey || e.ctrlKey) || e.altKey) return;
      const k = e.key.toLowerCase();
      if (k === "s") e.shiftKey ? saveAs() : save();
      else if (k === "o" && e.shiftKey) workspace.open(dirOf(docs.path));
      else if (k === "b" && !e.shiftKey) update({ sidebarOpen: !(settings?.sidebarOpen ?? true) });
      else if (k === "o" && !e.shiftKey) open();
      else if (k === "n" && !e.shiftKey) newDocument();
      else return;
      e.preventDefault();
    };
    window.addEventListener("keydown", onKey);
    return () => window.removeEventListener("keydown", onKey);
  }, [newDocument, open, save, saveAs, workspace.open, docs.path, update, settings?.sidebarOpen]);

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

  const { openPath } = docs;
  const reveal = useCallback(
    (d: Diagnostic) => {
      // Offsets in an imported file mean nothing here: open that file instead.
      if (d.path) openPath(d.path);
      else editor.current?.reveal(d.from, d.to);
    },
    [openPath],
  );

  // Wait for saved settings so the saved layout is applied on first paint.
  if (!settings) return <div className="h-full bg-surface" />;

  const ratio = clampRatio(dragRatio ?? settings.splitRatio);
  const showSource = mode !== "preview";
  const showPreview = mode !== "source";

  return (
    <div className="flex h-full flex-col">
      <Toolbar
        fileName={docs.name}
        dirty={docs.dirty}
        files={
          <FileControls
            onNew={docs.newDocument}
            onOpen={docs.open}
            onSave={docs.save}
            onSaveAs={docs.saveAs}
            onOpenRecent={(f) => docs.openPath(f.path)}
            onOpenFolder={() => workspace.open(dirOf(docs.path))}
            extra={
              <ExportMenu
                busy={exporter.busy}
                fetchRemote={settings.exportRemoteImages}
                onExport={exporter.run}
                onToggleRemote={(exportRemoteImages) => update({ exportRemoteImages })}
              />
            }
          />
        }
        sidebarOpen={settings.sidebarOpen}
        onToggleSidebar={() => update({ sidebarOpen: !settings.sidebarOpen })}
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

      <FileBanner
        banner={docs.banner}
        name={docs.name}
        onReload={docs.reload}
        onKeepMine={docs.keepMine}
        onSave={docs.save}
        onSaveAs={docs.saveAs}
        onDismiss={docs.dismissBanner}
      />

      <div className="flex min-h-0 flex-1">
        {settings.sidebarOpen && (
          <Sidebar
            tree={workspace.tree}
            error={workspace.error}
            activePath={docs.path}
            onOpenFolder={() => workspace.open(dirOf(docs.path))}
            onCloseFolder={workspace.close}
            onOpenFile={docs.openPath}
          />
        )}
        <main ref={main} className="flex min-h-0 min-w-0 flex-1">
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
                language={language}
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
            <Preview svg={view.svg} state={view.state} onZoomChange={setZoom} />
          </section>
        </main>
      </div>

      <StatusBar
        state={view.state}
        meta={view.meta}
        themes={themes}
        zoom={mode === "source" ? null : zoom}
        errorCount={view.diagnostics.length}
        encoding={docs.encoding}
        notice={docs.notice || (saveError ? `Settings not saved: ${saveError}` : "")}
      />

      <ConfirmDialog spec={dialog} onChoose={choose} />
    </div>
  );
}
