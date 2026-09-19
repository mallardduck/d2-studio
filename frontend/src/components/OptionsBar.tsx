import type { Engine, Meta, Theme } from "../../bindings/d2studio/compiler/models";

interface Props {
  engines: Engine[];
  themes: Theme[];
  /** Selected engine ID, or "" to follow the D2 source / default. */
  layout: string;
  /** Selected theme ID, or null to follow the D2 source / default. */
  themeId: number | null;
  meta: Meta | null;
  onLayout: (id: string) => void;
  onTheme: (id: number | null) => void;
}

const selectClass =
  "rounded border border-line bg-surface px-1.5 py-0.5 text-fg hover:border-accent disabled:opacity-50";

/** Name of a theme by ID, falling back to the number. */
export const themeLabel = (themes: Theme[], id: number) =>
  themes.find((t) => t.id === id)?.name ?? String(id);

export function OptionsBar({ engines, themes, layout, themeId, meta, onLayout, onTheme }: Props) {
  const engineName = (id: string) => engines.find((e) => e.id === id)?.name ?? id;

  // "Auto" says what it currently resolves to, so the dropdown never
  // contradicts the diagram on screen.
  const autoLayout =
    meta?.layoutSource === "source" ? `Auto (d2-config: ${engineName(meta.sourceLayout)})` : "Auto (default)";
  const autoTheme =
    meta?.themeSource === "source" && meta.sourceThemeId != null
      ? `Auto (d2-config: ${themeLabel(themes, meta.sourceThemeId)})`
      : "Auto (default)";

  const light = themes.filter((t) => !t.dark);
  const dark = themes.filter((t) => t.dark);

  return (
    <div className="flex items-center gap-3">
      <label className="flex items-center gap-1.5 text-fg-muted">
        Layout
        <select
          className={selectClass}
          value={layout}
          onChange={(e) => onLayout(e.target.value)}
          aria-label="Layout engine"
        >
          <option value="">{autoLayout}</option>
          {engines.map((e) => (
            <option key={e.id} value={e.id} disabled={!e.available} title={e.available ? e.description : e.reason}>
              {e.name}
              {e.available ? "" : " (unavailable)"}
            </option>
          ))}
        </select>
      </label>

      <label className="flex items-center gap-1.5 text-fg-muted">
        Theme
        <select
          className={selectClass}
          value={themeId === null ? "" : String(themeId)}
          onChange={(e) => onTheme(e.target.value === "" ? null : Number(e.target.value))}
          aria-label="Theme"
        >
          <option value="">{autoTheme}</option>
          <optgroup label="Light">
            {light.map((t) => (
              <option key={t.id} value={t.id}>
                {t.name}
              </option>
            ))}
          </optgroup>
          <optgroup label="Dark">
            {dark.map((t) => (
              <option key={t.id} value={t.id}>
                {t.name}
              </option>
            ))}
          </optgroup>
        </select>
      </label>
    </div>
  );
}
