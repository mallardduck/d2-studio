import { EditorView, basicSetup } from "codemirror";
import { EditorState, type Extension } from "@codemirror/state";
import { useEffect, useRef } from "react";

interface Props {
  code: string;
  language?: Extension;
  label: string;
}

/** Highlighted, selectable, read-only D2 source. */
export function ReadOnlyCode({ code, language, label }: Props) {
  const host = useRef<HTMLDivElement>(null);
  useEffect(() => {
    const view = new EditorView({
      parent: host.current!,
      state: EditorState.create({
        doc: code,
        extensions: [
          basicSetup,
          language ?? [],
          EditorState.readOnly.of(true),
          EditorView.editable.of(true), // stays focusable and selectable, but cannot be changed
          EditorView.contentAttributes.of({ "aria-label": label, "aria-readonly": "true" }),
          EditorView.theme({
            "&": { height: "100%", color: "var(--fg)", backgroundColor: "var(--surface)" },
            ".cm-scroller": { fontFamily: "var(--font-mono)", fontSize: "13px", lineHeight: "1.55" },
            ".cm-gutters": { backgroundColor: "var(--surface-alt)", color: "var(--fg-muted)", borderRight: "1px solid var(--line)" },
            "&.cm-focused": { outline: "none" },
          }),
        ],
      }),
    });
    return () => view.destroy();
  }, [code, language, label]);
  return <div ref={host} className="h-full min-h-0" />;
}
