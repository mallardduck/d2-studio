/** The document tab is always present; snippet tabs are opened and closed. */
export const DOC_TAB = "doc";

export interface Tabs {
  /** Snippet IDs with an open tab, in tab order. */
  open: string[];
  /** DOC_TAB or the ID of an open snippet. */
  active: string;
}

export const initialTabs: Tabs = { open: [], active: DOC_TAB };

/** Open a snippet's tab (or switch to it if it is already open). */
export function openTab(t: Tabs, id: string): Tabs {
  return { open: t.open.includes(id) ? t.open : [...t.open, id], active: id };
}

/** Close a snippet tab. If it was active, the tab to its left (else the document) takes over. */
export function closeTab(t: Tabs, id: string): Tabs {
  const i = t.open.indexOf(id);
  if (i < 0) return t;
  const open = t.open.filter((x) => x !== id);
  if (t.active !== id) return { open, active: t.active };
  return { open, active: i > 0 ? open[i - 1] : DOC_TAB };
}

/** Keep only snippets that still exist (e.g. after the list reloads). */
export function pruneTabs(t: Tabs, known: Set<string>): Tabs {
  const open = t.open.filter((id) => known.has(id));
  if (open.length === t.open.length) return t;
  return { open, active: t.active === DOC_TAB || open.includes(t.active) ? t.active : DOC_TAB };
}
