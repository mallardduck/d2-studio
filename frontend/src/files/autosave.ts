export interface AutosaveChoice {
  id: string;
  mode: "off" | "delay" | "blur";
  delaySec: number;
  label: string;
}

/** The choices offered in the menu. Off is first because it is the default. */
export const AUTOSAVE_CHOICES: AutosaveChoice[] = [
  { id: "off", mode: "off", delaySec: 2, label: "Off" },
  { id: "delay-2", mode: "delay", delaySec: 2, label: "After a 2-second pause" },
  { id: "delay-10", mode: "delay", delaySec: 10, label: "After a 10-second pause" },
  { id: "blur", mode: "blur", delaySec: 2, label: "When the window loses focus" },
];

/**
 * The menu choice matching the saved settings. A hand-edited delay that is not
 * one of the presets still gets an accurate entry rather than being shown as
 * something else.
 */
export function currentChoice(mode: string, delaySec: number): AutosaveChoice {
  if (mode === "blur") return AUTOSAVE_CHOICES.find((c) => c.id === "blur")!;
  if (mode === "delay") {
    return (
      AUTOSAVE_CHOICES.find((c) => c.mode === "delay" && c.delaySec === delaySec) ?? {
        id: `delay-${delaySec}`,
        mode: "delay",
        delaySec,
        label: `After a ${delaySec}-second pause`,
      }
    );
  }
  return AUTOSAVE_CHOICES[0]; // unknown values are treated as off, like the backend does
}
