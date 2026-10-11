import { AUTOSAVE_CHOICES, currentChoice, type AutosaveChoice } from "../files/autosave";
import { MenuButton } from "./MenuButton";

interface Props {
  mode: string;
  delaySec: number;
  onChange: (choice: AutosaveChoice) => void;
}

export function AutosaveMenu({ mode, delaySec, onChange }: Props) {
  const current = currentChoice(mode, delaySec);
  return (
    <MenuButton label={`Autosave: ${current.mode === "off" ? "Off" : "On"} ▾`} menuLabel="Autosave">
      {(close) => (
        <>
          {AUTOSAVE_CHOICES.map((c) => (
            <button
              key={c.id}
              type="button"
              role="menuitemradio"
              aria-checked={c.id === current.id}
              onClick={() => {
                close();
                onChange(c);
              }}
              className="flex w-full items-center gap-2 px-3 py-1.5 text-left hover:bg-line/40 focus-visible:bg-line/40"
            >
              <span aria-hidden="true" className="w-4 shrink-0">
                {c.id === current.id ? "✓" : ""}
              </span>
              {c.label}
            </button>
          ))}
          <p className="mt-1 border-t border-line px-3 py-2 text-xs text-fg-muted">
            Saves files that already have a name. It never overwrites changes made outside D2 Studio; if the file
            changed on disk it stops and asks.
          </p>
        </>
      )}
    </MenuButton>
  );
}
