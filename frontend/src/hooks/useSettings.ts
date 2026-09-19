import { useCallback, useEffect, useRef, useState } from "react";
import { Get, Save } from "../../bindings/d2studio/settingsservice";
import type { Settings } from "../../bindings/d2studio/settings/models";

export type ViewMode = "source" | "split" | "preview";
export const MIN_SPLIT = 0.15;
export const MAX_SPLIT = 0.85;
const SAVE_DELAY_MS = 300;

export const clampRatio = (r: number) => Math.min(MAX_SPLIT, Math.max(MIN_SPLIT, r));

/**
 * Loads persisted settings once and saves changes (debounced) through the Go
 * settings store. `settings` is null until the first load completes, so the UI
 * never flashes defaults before applying the saved layout.
 */
export function useSettings() {
  const [settings, setSettings] = useState<Settings | null>(null);
  const latest = useRef<Settings | null>(null);
  const timer = useRef<ReturnType<typeof setTimeout> | undefined>(undefined);
  const [saveError, setSaveError] = useState<string>("");

  const flush = useCallback(() => {
    clearTimeout(timer.current);
    timer.current = undefined;
    if (!latest.current) return;
    Save(latest.current).then(
      () => setSaveError(""),
      (err: unknown) => setSaveError(err instanceof Error ? err.message : String(err)),
    );
  }, []);

  useEffect(() => {
    Get().then((s) => {
      latest.current = s;
      setSettings(s);
    });
    // Don't lose a pending change if the window closes within the debounce.
    const onHide = () => timer.current && flush();
    window.addEventListener("pagehide", onHide);
    return () => window.removeEventListener("pagehide", onHide);
  }, [flush]);

  const update = useCallback(
    (patch: Partial<Settings>) => {
      if (!latest.current) return;
      latest.current = { ...latest.current, ...patch };
      setSettings(latest.current);
      clearTimeout(timer.current);
      timer.current = setTimeout(flush, SAVE_DELAY_MS);
    },
    [flush],
  );

  return { settings, update, saveError };
}
