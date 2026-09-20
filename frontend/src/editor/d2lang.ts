import { StreamLanguage, HighlightStyle, foldService, syntaxHighlighting, type StringStream } from "@codemirror/language";
import type { CompletionContext, CompletionResult } from "@codemirror/autocomplete";
import type { Extension } from "@codemirror/state";
import { tags as t } from "@lezer/highlight";
import { Complete } from "../../bindings/d2studio/compilerservice";
import type { LanguageInfo } from "../../bindings/d2studio/compiler/models";

interface State {
  /** Terminator of an open |md ... | block string, or null. */
  block: string | null;
  /** True after a ':' on the current line, i.e. in value position. */
  afterColon: boolean;
}

const WORD = /^[\w\u0080-￿]+(?:-[\w\u0080-￿]+)*/;
const COLOR = /^#[0-9a-fA-F]{3,8}(?![\w])/;

/**
 * D2 tokenizer. Keyword and value vocabularies come from the pinned D2
 * library (via the Go service) so highlighting tracks the compiler.
 */
export function createTokenizer(info: LanguageInfo) {
  const keywords = new Set(info.keywords);
  const values = new Set(info.values);

  return {
    name: "d2",
    startState: (): State => ({ block: null, afterColon: false }),
    copyState: (s: State): State => ({ ...s }),

    token(stream: StringStream, state: State): string | null {
      if (state.block) {
        const end = stream.string.indexOf(state.block, stream.pos);
        if (end === -1) {
          stream.skipToEnd();
        } else {
          stream.pos = end + state.block.length;
          state.block = null;
        }
        return "string";
      }

      if (stream.sol()) state.afterColon = false;
      if (stream.eatSpace()) return null;

      // Colors in value position (fill: #fff) vs comments.
      if (state.afterColon && stream.match(COLOR)) return "number";
      if (stream.peek() === "#") {
        stream.skipToEnd();
        return "comment";
      }

      // Quoted strings; a quoted string followed by ':' is a key.
      const quote = stream.peek();
      if (quote === '"' || quote === "'") {
        stream.next();
        let escaped = false;
        let ch: string | void;
        while ((ch = stream.next()) != null) {
          if (ch === quote && !escaped) break;
          escaped = !escaped && ch === "\\";
        }
        return !state.afterColon && stream.match(/^\s*:/, false) ? "propertyName" : "string";
      }

      // Block strings: |md ... | (more pipes allow pipes inside).
      const pipes = stream.match(/^\|+/) as RegExpMatchArray | null;
      if (pipes) {
        state.block = pipes[0];
        stream.match(/^[\w-]*/);
        return "string";
      }

      if (stream.match(/^(<->|<-|->|--)/)) return "operator";
      if (stream.match(/^\.\.\.@?/) || stream.match(/^@/)) return "meta"; // spread / import
      if (stream.match(/^\$\{[^}]*\}/)) return "variableName.special"; // substitution
      if (stream.match(/^-?\d+(\.\d+)?%?(?![\w-])/)) return "number";

      if (stream.match(/^[{}[\]()]/)) return "bracket";
      if (stream.eat(":")) {
        state.afterColon = true;
        return "punctuation";
      }
      if (stream.eat(/[;.,*&!]/)) return "punctuation";

      const word = stream.match(WORD) as RegExpMatchArray | null;
      if (word) {
        const w = word[0];
        if (state.afterColon) {
          if (w === "true" || w === "false") return "bool";
          if (w === "null") return "null";
          return values.has(w) ? "atom" : "string";
        }
        return keywords.has(w) ? "keyword" : "variableName";
      }

      stream.next();
      return null;
    },

    languageData: {
      commentTokens: { line: "#" },
      closeBrackets: { brackets: ["(", "[", "{", '"', "'"] },
    },
  };
}

/** Colors come from CSS variables so light and dark follow the OS setting. */
const highlightStyle = HighlightStyle.define([
  { tag: t.keyword, color: "var(--syn-keyword)", fontWeight: "600" },
  { tag: t.variableName, color: "var(--syn-key)" },
  { tag: t.propertyName, color: "var(--syn-key)" },
  { tag: t.string, color: "var(--syn-string)" },
  { tag: t.number, color: "var(--syn-number)" },
  { tag: [t.bool, t.null, t.atom], color: "var(--syn-atom)" },
  { tag: t.comment, color: "var(--syn-comment)", fontStyle: "italic" },
  { tag: t.operator, color: "var(--syn-operator)", fontWeight: "600" },
  { tag: t.meta, color: "var(--syn-keyword)" },
  { tag: t.special(t.variableName), color: "var(--syn-special)" },
  { tag: [t.bracket, t.punctuation], color: "var(--fg-muted)" },
]);

const BRACE_AT_END = /\{\s*(#.*)?$/;
const MAX_FOLD_SCAN_LINES = 5000;

/**
 * Folds a map: from the '{' ending a line to its matching '}'. Brace counting
 * ignores quoted strings and comments on each line.
 */
export const braceFolding = foldService.of((state, lineStart, lineEnd) => {
  const line = state.doc.lineAt(lineStart);
  const m = BRACE_AT_END.exec(line.text);
  if (!m || line.to > lineEnd) return null;

  const open = line.from + line.text.lastIndexOf("{", m.index);
  let depth = 0;
  const last = Math.min(state.doc.lines, line.number + MAX_FOLD_SCAN_LINES);
  for (let n = line.number; n <= last; n++) {
    const text = state.doc.line(n).text;
    const from = n === line.number ? open - line.from : 0;
    for (let i = from, q = ""; i < text.length; i++) {
      const c = text[i];
      if (q) {
        if (c === "\\") i++;
        else if (c === q) q = "";
      } else if (c === '"' || c === "'") q = c;
      else if (c === "#" && !/^#[0-9a-fA-F]{3,8}(?![\w])/.test(text.slice(i))) break;
      else if (c === "{") depth++;
      else if (c === "}" && --depth === 0) {
        const close = state.doc.line(n).from + i;
        // Nothing worth folding when the map body is empty or only whitespace.
        return state.sliceDoc(open + 1, close).trim() ? { from: open + 1, to: close } : null;
      }
    }
  }
  return null;
});

/** Completion backed by D2's own completion engine (runs in the Go service). */
async function completeD2(ctx: CompletionContext): Promise<CompletionResult | null> {
  const word = ctx.matchBefore(/[\w-]*/);
  if (!word) return null;
  // Don't pop up while idle in the middle of nowhere; allow explicit Ctrl+Space.
  if (word.from === word.to && !ctx.explicit && ctx.state.sliceDoc(word.from - 1, word.from) !== "." && ctx.state.sliceDoc(word.from - 2, word.from) !== ": ") {
    return null;
  }

  const line = ctx.state.doc.lineAt(ctx.pos);
  // D2 positions are zero-based; CodeMirror offsets are UTF-16, as D2 expects.
  const items = await Complete(ctx.state.doc.toString(), line.number - 1, ctx.pos - line.from);
  if (ctx.aborted || !items || items.length === 0) return null;

  return {
    from: word.from,
    options: items.map((i) => ({
      label: i.label,
      detail: i.detail,
      apply: i.insert,
      type: i.kind === "shape" ? "class" : i.kind === "style" ? "property" : "keyword",
    })),
    validFor: /^[\w-]*$/,
  };
}

/** Everything the editor needs for D2, built from D2's vocabulary. */
export function d2Language(info: LanguageInfo): Extension {
  const language = StreamLanguage.define(createTokenizer(info));
  return [
    language,
    syntaxHighlighting(highlightStyle),
    braceFolding,
    language.data.of({ autocomplete: completeD2 }),
  ];
}
