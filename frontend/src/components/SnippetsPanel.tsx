import { useMemo, useState } from "react";
import type { Snippet } from "../../bindings/d2studio/snippets/models";
import { useCopy } from "../hooks/useCopy";

interface Props {
  snippets: Snippet[];
  error: string;
  onOpen: (id: string) => void;
  onInsert: (code: string) => void;
}

const act = "rounded px-1.5 py-0.5 text-fg-muted underline-offset-2 hover:bg-line/40 hover:text-fg hover:underline";

/** Matches a snippet against a search query (every word must appear). */
export function matches(s: Snippet, query: string): boolean {
  const words = query.toLowerCase().split(/\s+/).filter(Boolean);
  const hay = `${s.title} ${s.category} ${s.description} ${s.code}`.toLowerCase();
  return words.every((w) => hay.includes(w));
}

/** A compact list of snippet names; each can be opened in a tab, inserted or copied. */
export function SnippetsPanel({ snippets, error, onOpen, onInsert }: Props) {
  const [query, setQuery] = useState("");
  const { copied, copy } = useCopy();

  const groups = useMemo(() => {
    const m = new Map<string, Snippet[]>();
    for (const s of snippets) if (matches(s, query)) m.set(s.category, [...(m.get(s.category) ?? []), s]);
    return [...m];
  }, [snippets, query]);

  return (
    <div className="flex min-h-0 flex-1 flex-col">
      <div className="border-b border-line p-2">
        <input
          type="search"
          aria-label="Search snippets"
          placeholder="Search snippets…"
          value={query}
          onChange={(e) => setQuery(e.target.value)}
          className="w-full rounded border border-line bg-surface px-2 py-1"
        />
      </div>
      <div className="min-h-0 flex-1 overflow-auto" role="region" aria-label="Snippet list">
        {error && (
          <p role="alert" className="p-2 text-warn">
            Snippets could not be loaded: {error}
          </p>
        )}
        {!error && snippets.length > 0 && groups.length === 0 && <p className="p-3 text-fg-muted">No snippets match.</p>}
        {groups.map(([category, items]) => (
          <section key={category} aria-label={category}>
            <h3 className="sticky top-0 bg-surface-alt px-2 py-1 text-xs font-semibold uppercase tracking-wide text-fg-muted">
              {category}
            </h3>
            <ul className="m-0 list-none p-0">
              {items.map((s) => (
                <li key={s.id} className="border-b border-line/60 px-2 py-1" title={s.description}>
                  <div className="truncate">{s.title}</div>
                  <div className="flex gap-0.5 text-xs">
                    <button type="button" className={act} aria-label={`Open ${s.title} in a tab`} onClick={() => onOpen(s.id)}>
                      Open
                    </button>
                    <button type="button" className={act} aria-label={`Insert ${s.title}`} onClick={() => onInsert(s.code)}>
                      Insert
                    </button>
                    <button type="button" className={act} aria-label={`Copy ${s.title}`} onClick={() => copy(s.id, s.code)}>
                      {copied === s.id ? "Copied" : "Copy"}
                    </button>
                  </div>
                </li>
              ))}
            </ul>
          </section>
        ))}
        <p aria-live="polite" className="sr-only">
          {copied ? "Copied to clipboard" : ""}
        </p>
      </div>
    </div>
  );
}
