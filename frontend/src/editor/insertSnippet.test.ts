import { describe, expect, it } from "vitest";
import { snippetInsertion } from "./insertSnippet";

describe("snippetInsertion", () => {
  it("inserts as-is into an empty document", () => {
    expect(snippetInsertion("", 0, "a -> b\n")).toBe("a -> b\n");
  });
  it("inserts as-is at the start of a line", () => {
    expect(snippetInsertion("x\ny\n", 2, "a -> b\n")).toBe("a -> b\n");
  });
  it("starts a new line when the cursor is mid-line or at a line's end", () => {
    expect(snippetInsertion("x -> y", 6, "a -> b\n")).toBe("\na -> b\n");
    expect(snippetInsertion("x -> y", 2, "a -> b\n")).toBe("\na -> b\n");
  });
  it("makes sure the snippet ends with a newline", () => {
    expect(snippetInsertion("", 0, "a")).toBe("a\n");
  });
});
