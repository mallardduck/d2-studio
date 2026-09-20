import { describe, expect, it } from "vitest";
import { EditorState } from "@codemirror/state";
import { StringStream, foldable } from "@codemirror/language";
import { braceFolding, createTokenizer } from "./d2lang";

const info = {
  keywords: ["shape", "style", "label", "fill", "vars", "d2-config"],
  values: ["cylinder", "person", "up"],
};

type Tok = [text: string, type: string];

/** Runs the tokenizer over multi-line text, returning non-whitespace tokens. */
function tokenize(text: string): Tok[][] {
  const tz = createTokenizer(info);
  const state = tz.startState();
  return text.split("\n").map((line) => {
    const out: Tok[] = [];
    const stream = new StringStream(line, 2, 2);
    while (!stream.eol()) {
      stream.start = stream.pos;
      const type = tz.token(stream, state);
      if (stream.pos === stream.start) throw new Error(`tokenizer made no progress at "${line}":${stream.pos}`);
      if (type) out.push([stream.current(), type]);
    }
    return out;
  });
}

const types = (line: string) => tokenize(line)[0].map(([, t]) => t);

describe("D2 tokenizer", () => {
  it("highlights keys, operators, and values", () => {
    expect(tokenize("a -> b: calls")[0]).toEqual([
      ["a", "variableName"],
      ["->", "operator"],
      ["b", "variableName"],
      [":", "punctuation"],
      ["calls", "string"],
    ]);
  });

  it("distinguishes reserved keywords from ordinary keys", () => {
    expect(tokenize("shape: cylinder")[0]).toEqual([
      ["shape", "keyword"],
      [":", "punctuation"],
      ["cylinder", "atom"],
    ]);
    expect(types("network: x")[0]).toBe("variableName");
  });

  it("recognizes all edge operators, with or without spaces", () => {
    for (const op of ["->", "<-", "<->", "--"]) {
      expect(types(`a ${op} b`)).toContain("operator");
    }
    expect(tokenize("a->b")[0].map(([s]) => s)).toEqual(["a", "->", "b"]);
  });

  it("keeps hyphenated words together and treats 3d as a word", () => {
    expect(tokenize("d2-config: {")[0][0]).toEqual(["d2-config", "keyword"]);
    expect(tokenize("fill-pattern: dots")[0][0]).toEqual(["fill-pattern", "variableName"]);
    expect(types("3d: true")).toEqual(["variableName", "punctuation", "bool"]);
  });

  it("separates comments from hex colors by position", () => {
    expect(tokenize("# a comment")[0]).toEqual([["# a comment", "comment"]]);
    expect(tokenize("fill: #ff8800")[0].pop()).toEqual(["#ff8800", "number"]);
    expect(tokenize("a: b # trailing")[0].pop()).toEqual(["# trailing", "comment"]);
  });

  it("handles quoted strings, keys in quotes, and escapes", () => {
    expect(tokenize('"hello world": x')[0][0]).toEqual(['"hello world"', "propertyName"]);
    expect(tokenize('label: "say \\"hi\\""')[0].pop()).toEqual(['"say \\"hi\\""', "string"]);
  });

  it("highlights numbers, booleans, null, imports and substitutions", () => {
    expect(types("width: 120")).toContain("number");
    expect(types("x: 50%")).toContain("number");
    expect(types("a: true")).toContain("bool");
    expect(types("a: null")).toContain("null");
    expect(tokenize("...@base.d2")[0][0]).toEqual(["...@", "meta"]);
    expect(tokenize("label: ${name}")[0].pop()).toEqual(["${name}", "variableName.special"]);
  });

  it("carries multi-line block strings across lines", () => {
    const lines = tokenize("x: |md\n# not a heading comment\n- a -> b\n|\ny: z");
    expect(lines[1]).toEqual([["# not a heading comment", "string"]]);
    expect(lines[2].every(([, t]) => t === "string")).toBe(true);
    // After the block closes, normal tokenization resumes.
    expect(lines[4][0]).toEqual(["y", "variableName"]);
  });

  it("supports extra pipes so a block can contain a pipe", () => {
    const lines = tokenize("x: ||md\na | b\n||\ny: z");
    expect(lines[1].every(([, t]) => t === "string")).toBe(true);
    expect(lines[3][0]).toEqual(["y", "variableName"]);
  });

  it("resets value position on each new line", () => {
    expect(tokenize("a: b\nc: d")[1][0]).toEqual(["c", "variableName"]);
  });

  it("never stalls on unusual input", () => {
    expect(() => tokenize("`~^%$ é😀 ??? <> \\")).not.toThrow();
  });
});

describe("brace folding", () => {
  const state = (doc: string) => EditorState.create({ doc, extensions: [braceFolding] });
  const fold = (doc: string, line: number) => {
    const s = state(doc);
    const l = s.doc.line(line);
    return foldable(s, l.from, l.to);
  };

  it("folds a map from after '{' to the matching '}'", () => {
    const doc = "a: {\n  b: 1\n  c: 2\n}\nz";
    const r = fold(doc, 1)!;
    expect(doc.slice(r.from, r.to)).toBe("\n  b: 1\n  c: 2\n");
  });

  it("matches nested braces", () => {
    const doc = "a: {\n  b: {\n    c\n  }\n  d\n}";
    expect(doc.slice(fold(doc, 1)!.from, fold(doc, 1)!.to)).toBe("\n  b: {\n    c\n  }\n  d\n");
    expect(doc.slice(fold(doc, 2)!.from, fold(doc, 2)!.to)).toBe("\n    c\n  ");
  });

  it("ignores braces inside strings and comments", () => {
    const doc = 'a: {\n  b: "}"  # }\n  c\n}';
    expect(doc.slice(fold(doc, 1)!.from, fold(doc, 1)!.to)).toBe('\n  b: "}"  # }\n  c\n');
  });

  it("allows a trailing comment after the opening brace", () => {
    const doc = "a: { # comment\n  b\n}";
    expect(fold(doc, 1)).not.toBeNull();
  });

  it("does not fold unterminated, empty, or non-brace lines", () => {
    expect(fold("a: {\n  b", 1)).toBeNull();
    expect(fold("a: {}", 1)).toBeNull();
    expect(fold("a: {\n}", 1)).toBeNull();
    expect(fold("a -> b\nc", 1)).toBeNull();
  });
});
