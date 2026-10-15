import type { Extension } from "@codemirror/state";
import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import { Engines, Language, Themes } from "../bindings/d2studio/compilerservice";
import type { Diagnostic, Engine, Options, Theme } from "../bindings/d2studio/compiler/models";
import { Editor, type EditorHandle } from "./components/Editor";
import { Preview } from "./components/Preview";
import { Diagnostics } from "./components/Diagnostics";
import { StatusBar } from "./components/StatusBar";
import { Toolbar } from "./components/Toolbar";
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
import { useMenu } from "./hooks/useMenu";
import type { MenuActionType } from "./menu/actions";
import type { State as MenuState } from "../bindings/d2studio/menu/models";
import { ClearRecent } from "../bindings/d2studio/files/service";
import type { HistoryStatus } from "./editor/history";
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
  const [history, setHistory] = useState<HistoryStatus>({ canUndo: false, canRedo: false });
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
  const docs = useDocument({
    content: source,
    setContent,
    confirm,
    initialContent: SAMPLE,
    autosave: settings ? { mode: settings.autosave, delaySec: settings.autosaveDelaySec } : undefined,
  });
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

  const mode = (settings?.viewMode ?? "split") as ViewMode;
  const setMode = useCallback((m: ViewMode) => update({ viewMode: m }), [update]);

  // The native menu bar owns the keyboard shortcuts (File, Edit, View). A menu
  // item does what the matching toolbar control does, and the app's state is
  // sent back so check marks and enabled items stay right.
  const resync = useRef<() => void>(() => {});
  const perform = (a: MenuActionType) => {
    switch (a.type) {
      case "new":
        return void docs.newDocument();
      case "open":
        return void docs.open();
      case "openRecent":
        return void docs.openPath(a.path);
      case "clearRecent":
        return void ClearRecent().then(() => resync.current());
      case "openFolder":
        return void workspace.open(dirOf(docs.path));
      case "closeFolder":
        return void workspace.close();
      case "save":
        return void docs.save();
      case "saveAs":
        return void docs.saveAs();
      case "quit":
        return void docs.quit();
      case "export":
        return void exporter.run(a.format);
      case "undo":
        return editor.current?.undo();
      case "redo":
        return editor.current?.redo();
    }
    if (!settings) return;
    switch (a.type) {
      case "autosave":
        return update({ autosave: a.mode, autosaveDelaySec: a.delaySec });
      case "toggleRemoteImages":
        return update({ exportRemoteImages: !settings.exportRemoteImages });
      case "viewMode":
        return update({ viewMode: a.mode });
      case "toggleSidebar":
        return update({ sidebarOpen: !settings.sidebarOpen });
      case "layout":
        return update({ layout: a.engine });
      case "theme":
        return update({ themeId: a.id });
    }
  };

  const menuState = useMemo<MenuState | null>(
    () =>
      settings
        ? {
            viewMode: settings.viewMode,
            sidebarOpen: settings.sidebarOpen,
            hasFolder: workspace.tree !== null,
            autosave: settings.autosave,
            autosaveDelaySec: settings.autosaveDelaySec,
            layout: settings.layout,
            themeId: settings.themeId,
            fetchRemoteImages: settings.exportRemoteImages,
            canUndo: history.canUndo,
            canRedo: history.canRedo,
            exporting: exporter.busy,
            path: docs.path ?? "",
          }
        : null,
    [settings, workspace.tree, history, exporter.busy, docs.path],
  );
  resync.current = useMenu(menuState, perform).resync;

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
            onOpen={docs.open}
            onSave={docs.save}
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
                onHistoryChange={setHistory}
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
