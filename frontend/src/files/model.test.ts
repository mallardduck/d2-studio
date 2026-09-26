import { describe, expect, it } from "vitest";
import { baseName, decideExternal, dirOf, encodingLabel, windowTitle } from "./model";

const ev = (kind: string, message = "") => ({ kind, message }) as Parameters<typeof decideExternal>[0];

describe("decideExternal", () => {
  it("reloads silently only when there are no unsaved edits", () => {
    expect(decideExternal(ev("modified"), false)).toEqual({ type: "reload" });
  });

  it("never replaces unsaved edits: it prompts instead", () => {
    expect(decideExternal(ev("modified"), true)).toEqual({ type: "prompt-modified" });
  });

  it("always warns about deletion, dirty or not", () => {
    expect(decideExternal(ev("deleted"), false).type).toBe("prompt-deleted");
    expect(decideExternal(ev("deleted"), true).type).toBe("prompt-deleted");
  });

  it("surfaces the reason a file became unavailable", () => {
    expect(decideExternal(ev("unavailable", "permission denied"), true)).toEqual({
      type: "prompt-unavailable",
      message: "permission denied",
    });
  });

  it("clears warnings when the file is restored", () => {
    expect(decideExternal(ev("restored"), true)).toEqual({ type: "clear" });
  });

  it("ignores unknown event kinds without touching the editor", () => {
    for (const dirty of [true, false]) {
      expect(decideExternal(ev("from-the-future"), dirty)).toEqual({ type: "clear" });
    }
  });
});

describe("windowTitle", () => {
  it("marks unsaved changes", () => {
    expect(windowTitle("a.d2", true)).toBe("● a.d2 — D2 Studio");
    expect(windowTitle("a.d2", false)).toBe("a.d2 — D2 Studio");
  });
});

describe("encodingLabel", () => {
  const doc = (o: object) => ({ bom: false, lineEnding: "lf", mixedLineEndings: false, ...o });
  it("describes how the file is stored", () => {
    expect(encodingLabel(null)).toBe("UTF-8");
    expect(encodingLabel(doc({}))).toBe("UTF-8");
    expect(encodingLabel(doc({ lineEnding: "crlf" }))).toBe("UTF-8 · CRLF");
    expect(encodingLabel(doc({ bom: true, lineEnding: "crlf", mixedLineEndings: true }))).toBe(
      "UTF-8 with BOM · CRLF · mixed line endings",
    );
  });
});

describe("path helpers", () => {
  it("handles POSIX and Windows separators", () => {
    expect(dirOf("/home/me/a.d2")).toBe("/home/me");
    expect(dirOf("C:\\Users\\me\\a.d2")).toBe("C:\\Users\\me");
    expect(dirOf(null)).toBe("");
    expect(dirOf("a.d2")).toBe("");
    expect(baseName("/home/me/a.d2")).toBe("a.d2");
    expect(baseName("C:\\Users\\me\\a.d2")).toBe("a.d2");
    expect(baseName("a.d2")).toBe("a.d2");
  });
});
