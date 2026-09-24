import { indentLess, indentMore } from "@codemirror/commands";
import { acceptCompletion, completionStatus } from "@codemirror/autocomplete";
import { indentUnit } from "@codemirror/language";
import { EditorState, Prec, type Extension } from "@codemirror/state";
import { EditorView, keymap } from "@codemirror/view";

/** Spoken to screen-reader users via aria-describedby on the editor. */
export const TAB_HELP =
  "Tab indents. To move focus out of the editor, press Escape, then Tab or Shift+Tab.";

/** D2 style: two spaces per level. */
const INDENT = "  ";

type Target = Pick<EditorView, "state" | "dispatch">;

/** Insert one indent at the cursor, or indent the lines of a non-empty selection. */
export function insertIndent(view: Target): boolean {
  const { state } = view;
  if (state.selection.ranges.some((r) => !r.empty)) return indentMore(view);
  view.dispatch(state.update(state.replaceSelection(state.facet(indentUnit)), { userEvent: "input", scrollIntoView: true }));
  return true;
}

/**
 * Tab handling that does not trap keyboard users. Tab indents, but after
 * Escape the next Tab / Shift+Tab is left to the browser so focus can leave
 * the editor. That state is dropped by any other key and by blur.
 */
export function createTabHandlers() {
  let escaped = false;
  return {
    /** Escape arms the escape hatch, unless it was only closing a completion popup. */
    escape(view: Target): boolean {
      if (completionStatus(view.state) === null) escaped = true;
      return false; // never consume Escape: other handlers (search, completion) still need it
    },
    tab(view: Target): boolean {
      if (escaped) {
        escaped = false;
        return false;
      }
      return acceptCompletion(view as EditorView) || insertIndent(view);
    },
    shiftTab(view: Target): boolean {
      if (escaped) {
        escaped = false;
        return false;
      }
      return indentLess(view);
    },
    /** Any key other than the escape sequence cancels the pending escape. */
    otherKey(key: string) {
      if (key !== "Escape" && key !== "Tab" && key !== "Shift") escaped = false;
    },
    reset() {
      escaped = false;
    },
    get armed() {
      return escaped;
    },
  };
}

export function tabKeys(): Extension {
  const h = createTabHandlers();
  return [
    indentUnit.of(INDENT),
    EditorState.tabSize.of(INDENT.length),
    Prec.high(
      keymap.of([
        { key: "Escape", run: h.escape },
        { key: "Tab", run: h.tab, shift: h.shiftTab },
      ]),
    ),
    EditorView.domEventHandlers({
      keydown(e) {
        h.otherKey(e.key);
        return false;
      },
      blur() {
        h.reset();
        return false;
      },
    }),
  ];
}
