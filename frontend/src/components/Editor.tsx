import { forwardRef, useCallback, useEffect, useImperativeHandle, useRef } from "react";
import { EditorView, basicSetup } from "codemirror";
import { redo, undo } from "@codemirror/commands";
import { Compartment, EditorState, type Extension } from "@codemirror/state";
import { setDiagnostics, lintGutter } from "@codemirror/lint";
import { TAB_HELP, tabKeys } from "../editor/tabKey";
import { historyStatus, type HistoryStatus } from "../editor/history";
import type { Diagnostic as D2Diagnostic } from "../../bindings/d2studio/compiler/models";

export interface EditorHandle {
  /** Move the cursor to a source range and focus the editor. */
  reveal(from: number, to: number): void;
  /** Replace the whole document with a fresh state (new undo history). */
  setDoc(text: string): void;
  /** The same undo and redo the keyboard uses, so menu and shortcut agree. */
  undo(): void;
  redo(): void;
}

interface Props {
  initialDoc: string;
  onChange: (doc: string) => void;
  diagnostics: D2Diagnostic[];
  /** Source the diagnostics were computed for. */
  diagnosticsSource: string;
  /** D2 language support; plain text until D2's vocabulary has loaded. */
  language?: Extension;
  /** Called when whether there is anything to undo or redo changes. */
  onHistoryChange?: (status: HistoryStatus) => void;
}

const theme = EditorView.theme({
  "&": { height: "100%", color: "var(--fg)", backgroundColor: "var(--surface)" },
  ".cm-scroller": { fontFamily: "var(--font-mono)", fontSize: "13px", lineHeight: "1.55" },
  ".cm-gutters": {
    backgroundColor: "var(--surface-alt)",
    color: "var(--fg-muted)",
    borderRight: "1px solid var(--line)",
  },
  ".cm-activeLine, .cm-activeLineGutter": { backgroundColor: "color-mix(in srgb, var(--accent) 10%, transparent)" },
  "&.cm-focused": { outline: "none" },
  ".cm-cursor": { borderLeftColor: "var(--fg)" },
});

export const Editor = forwardRef<EditorHandle, Props>(function Editor(
  { initialDoc, onChange, diagnostics, diagnosticsSource, language, onHistoryChange },
  ref,
) {
  const host = useRef<HTMLDivElement>(null);
  const view = useRef<EditorView | null>(null);
  const languageSlot = useRef(new Compartment());
  const onChangeRef = useRef(onChange);
  onChangeRef.current = onChange;

  const languageRef = useRef(language);
  languageRef.current = language;

  // Report undo/redo availability, but only when it actually changes.
  const onHistoryRef = useRef(onHistoryChange);
  onHistoryRef.current = onHistoryChange;
  const lastHistory = useRef<HistoryStatus>({ canUndo: false, canRedo: false });
  const reportHistory = useCallback((state: EditorState) => {
    const next = historyStatus(state);
    if (next.canUndo !== lastHistory.current.canUndo || next.canRedo !== lastHistory.current.canRedo) {
      lastHistory.current = next;
      onHistoryRef.current?.(next);
    }
  }, []);

  // Built on demand so a new document starts with the current language.
  const makeExtensions = useCallback(
    (): Extension[] => [
      basicSetup,
      tabKeys(),
      languageSlot.current.of(languageRef.current ?? []),
      lintGutter(),
      theme,
      EditorView.contentAttributes.of({ "aria-label": "D2 source", "aria-describedby": "editor-tab-help" }),
      EditorView.updateListener.of((u) => {
        if (u.docChanged) onChangeRef.current(u.state.doc.toString());
        reportHistory(u.state);
      }),
    ],
    [reportHistory],
  );

  useEffect(() => {
    const v = new EditorView({
      parent: host.current!,
      state: EditorState.create({ doc: initialDoc, extensions: makeExtensions() }),
    });
    view.current = v;
    return () => {
      v.destroy();
      view.current = null;
    };
    // The editor owns its document after creation.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  useEffect(() => {
    view.current?.dispatch({ effects: languageSlot.current.reconfigure(language ?? []) });
  }, [language]);

  // Only show diagnostics whose positions still match the editor text; after
  // that CodeMirror maps them through further edits.
  useEffect(() => {
    const v = view.current;
    if (!v) return;
    const doc = v.state.doc;
    const matches = doc.length === diagnosticsSource.length && doc.toString() === diagnosticsSource;
    const marks = matches
      ? diagnostics
          // Errors in an imported file carry that file's offsets, so they must
          // not be drawn in this one.
          .filter((d) => d.hasLocation && !d.path)
          .map((d) => {
            const from = Math.min(d.from, doc.length);
            const to = Math.min(Math.max(d.to, from + 1), doc.length);
            return { from, to: Math.max(to, from), severity: "error" as const, message: d.message };
          })
      : [];
    if (matches || diagnostics.length === 0) v.dispatch(setDiagnostics(v.state, marks));
  }, [diagnostics, diagnosticsSource]);

  useImperativeHandle(ref, () => ({
    setDoc(text) {
      // A new state, not a transaction, so undo cannot cross into the
      // previous document.
      const state = EditorState.create({ doc: text, extensions: makeExtensions() });
      view.current?.setState(state);
      reportHistory(state); // a new document starts with nothing to undo
    },
    undo() {
      if (view.current) {
        undo(view.current);
        view.current.focus();
      }
    },
    redo() {
      if (view.current) {
        redo(view.current);
        view.current.focus();
      }
    },
    reveal(from, to) {
      const v = view.current;
      if (!v) return;
      const len = v.state.doc.length;
      const a = Math.min(from, len);
      v.dispatch({
        selection: { anchor: a, head: Math.min(Math.max(to, a), len) },
        effects: EditorView.scrollIntoView(a, { y: "center" }),
      });
      v.focus();
    },
  }));

  return (
    <>
      <p id="editor-tab-help" className="sr-only">
        {TAB_HELP}
      </p>
      <div ref={host} className="h-full min-h-0" />
    </>
  );
});
