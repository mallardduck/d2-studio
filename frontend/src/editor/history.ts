import { redoDepth, undoDepth } from "@codemirror/commands";
import type { EditorState } from "@codemirror/state";

export interface HistoryStatus {
  canUndo: boolean;
  canRedo: boolean;
}

/** Whether the editor has anything to undo or redo, for the Edit menu. */
export function historyStatus(state: EditorState): HistoryStatus {
  return { canUndo: undoDepth(state) > 0, canRedo: redoDepth(state) > 0 };
}
