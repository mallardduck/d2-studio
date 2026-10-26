/**
 * What to put in the document to insert a snippet at `from` (replacing
 * `from`..`to`). A snippet is whole lines, so if the cursor is in the middle
 * of a line the snippet starts on a fresh line instead of being glued to the
 * text before it.
 */
export function snippetInsertion(doc: string, from: number, code: string): string {
  const text = code.endsWith("\n") ? code : code + "\n";
  const atLineStart = from === 0 || doc[from - 1] === "\n";
  return atLineStart ? text : "\n" + text;
}
