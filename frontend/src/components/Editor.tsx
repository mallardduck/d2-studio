import { forwardRef, useEffect, useImperativeHandle, useRef } from "react";
import { EditorView, basicSetup } from "codemirror";
import { Compartment, EditorState, type Extension } from "@codemirror/state";
import { setDiagnostics, lintGutter } from "@codemirror/lint";
import { TAB_HELP, tabKeys } from "../editor/tabKey";
import type { Diagnostic as D2Diagnostic } from "../../bindings/d2studio/compiler/models";

export interface EditorHandle {
  /** Move the cursor to a source range and focus the editor. */
  reveal(from: number, to: number): void;
}

interface Props {
  initialDoc: string;
  onChange: (doc: string) => void;
  diagnostics: D2Diagnostic[];
  /** Source the diagnostics were computed for. */
  diagnosticsSource: string;
  /** D2 language support; plain text until D2's vocabulary has loaded. */
  language?: Extension;
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
  { initialDoc, onChange, diagnostics, diagnosticsSource, language },
  ref,
) {
  const host = useRef<HTMLDivElement>(null);
  const view = useRef<EditorView | null>(null);
  const languageSlot = useRef(new Compartment());
  const onChangeRef = useRef(onChange);
  onChangeRef.current = onChange;

  useEffect(() => {
    const v = new EditorView({
      parent: host.current!,
      state: EditorState.create({
        doc: initialDoc,
        extensions: [
          basicSetup,
          tabKeys(),
          languageSlot.current.of(language ?? []),
          lintGutter(),
          theme,
          EditorView.contentAttributes.of({ "aria-label": "D2 source", "aria-describedby": "editor-tab-help" }),
          EditorView.updateListener.of((u) => {
            if (u.docChanged) onChangeRef.current(u.state.doc.toString());
          }),
        ],
      }),
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
          .filter((d) => d.hasLocation)
          .map((d) => {
            const from = Math.min(d.from, doc.length);
            const to = Math.min(Math.max(d.to, from + 1), doc.length);
            return { from, to: Math.max(to, from), severity: "error" as const, message: d.message };
          })
      : [];
    if (matches || diagnostics.length === 0) v.dispatch(setDiagnostics(v.state, marks));
  }, [diagnostics, diagnosticsSource]);

  useImperativeHandle(ref, () => ({
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
