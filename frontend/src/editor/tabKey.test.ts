import { describe, expect, it } from "vitest";
import { EditorSelection, EditorState, type Transaction } from "@codemirror/state";
import { indentUnit } from "@codemirror/language";
import { createTabHandlers, insertIndent } from "./tabKey";

/** Minimal headless stand-in for an EditorView. */
function harness(doc: string, selection?: { anchor: number; head?: number }) {
  const view = {
    state: EditorState.create({
      doc,
      selection: selection ? EditorSelection.single(selection.anchor, selection.head) : undefined,
      extensions: [indentUnit.of("  ")],
    }),
    dispatch(tr: Transaction) {
      view.state = tr.state;
    },
  };
  return view as typeof view & Parameters<typeof insertIndent>[0];
}

describe("Tab inserts an indent", () => {
  it("inserts two spaces at the caret, not a tab character or a line-start indent", () => {
    const v = harness("a: b", { anchor: 2 });
    expect(createTabHandlers().tab(v)).toBe(true);
    expect(v.state.doc.toString()).toBe("a:   b");
    expect(v.state.doc.toString()).not.toContain("\t");
    expect(v.state.selection.main.head).toBe(4);
  });

  it("indents every selected line when the selection is non-empty", () => {
    const v = harness("a\nb\nc", { anchor: 0, head: 5 });
    createTabHandlers().tab(v);
    expect(v.state.doc.toString()).toBe("  a\n  b\n  c");
  });

  it("Shift+Tab outdents", () => {
    const v = harness("    a", { anchor: 4 });
    expect(createTabHandlers().shiftTab(v)).toBe(true);
    expect(v.state.doc.toString()).toBe("  a");
  });
});

describe("keyboard-trap escape hatch", () => {
  it("lets Tab through once after Escape, then indents again", () => {
    const h = createTabHandlers();
    const v = harness("a", { anchor: 1 });
    expect(h.escape(v)).toBe(false); // Escape itself is never consumed
    expect(h.armed).toBe(true);
    expect(h.tab(v)).toBe(false); // browser moves focus
    expect(v.state.doc.toString()).toBe("a");
    expect(h.tab(v)).toBe(true); // back to indenting
    expect(v.state.doc.toString()).toBe("a  ");
  });

  it("lets Shift+Tab through after Escape as well", () => {
    const h = createTabHandlers();
    const v = harness("  a", { anchor: 3 });
    h.escape(v);
    expect(h.shiftTab(v)).toBe(false);
    expect(v.state.doc.toString()).toBe("  a");
  });

  it("is cancelled by any other key, so a stray Escape does not leak", () => {
    const h = createTabHandlers();
    const v = harness("a", { anchor: 1 });
    h.escape(v);
    h.otherKey("x");
    expect(h.armed).toBe(false);
    expect(h.tab(v)).toBe(true);
  });

  it("is not cancelled by the Tab/Shift keys that make up the sequence", () => {
    const h = createTabHandlers();
    h.escape(harness("a"));
    h.otherKey("Shift");
    h.otherKey("Tab");
    expect(h.armed).toBe(true);
  });

  it("is cleared on blur", () => {
    const h = createTabHandlers();
    h.escape(harness("a"));
    h.reset();
    expect(h.armed).toBe(false);
  });
});
