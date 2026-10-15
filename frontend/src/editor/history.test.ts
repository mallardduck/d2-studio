import { describe, expect, it } from "vitest";
import { EditorState } from "@codemirror/state";
import { history, undo, redo } from "@codemirror/commands";
import { historyStatus } from "./history";

/** Runs a CodeMirror command against a state without a view. */
function run(state: EditorState, command: (t: { state: EditorState; dispatch: (tr: never) => void }) => boolean) {
  let next = state;
  command({ state, dispatch: (tr) => (next = (tr as unknown as { state: EditorState }).state) });
  return next;
}

describe("historyStatus", () => {
  const start = () => EditorState.create({ doc: "a", extensions: [history()] });

  it("has nothing to undo or redo in a fresh document", () => {
    expect(historyStatus(start())).toEqual({ canUndo: false, canRedo: false });
  });

  it("can undo after an edit", () => {
    const edited = start().update({ changes: { from: 1, insert: "b" } }).state;
    expect(historyStatus(edited)).toEqual({ canUndo: true, canRedo: false });
  });

  it("can redo after undoing, and not undo again once at the start", () => {
    const edited = start().update({ changes: { from: 1, insert: "b" } }).state;
    const undone = run(edited, undo as never);
    expect(undone.doc.toString()).toBe("a");
    expect(historyStatus(undone)).toEqual({ canUndo: false, canRedo: true });
    const redone = run(undone, redo as never);
    expect(redone.doc.toString()).toBe("ab");
    expect(historyStatus(redone)).toEqual({ canUndo: true, canRedo: false });
  });

  it("a freshly created state (a newly opened file) starts clean", () => {
    // Opening a file builds a new EditorState, so history cannot cross files.
    expect(historyStatus(EditorState.create({ doc: "other file", extensions: [history()] }))).toEqual({
      canUndo: false,
      canRedo: false,
    });
  });
});
