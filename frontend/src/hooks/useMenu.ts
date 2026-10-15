import { useCallback, useEffect, useRef } from "react";
import { Events } from "@wailsio/runtime";
import { Sync } from "../../bindings/d2studio/menuservice";
import type { State } from "../../bindings/d2studio/menu/models";
import { parseAction, type MenuActionType } from "../menu/actions";

const SYNC_DELAY_MS = 100;

/**
 * Connects the native menu bar to the app. Choosing a menu item arrives as an
 * action for `onAction`, which should do exactly what the matching toolbar
 * control does. The app's state is sent back so check marks and enabled items
 * stay right.
 */
export function useMenu(state: State | null, onAction: (action: MenuActionType) => void) {
  const handler = useRef(onAction);
  handler.current = onAction;

  useEffect(() => {
    return Events.On("menu:action", (ev) => {
      const action = parseAction(ev.data?.id ?? "");
      if (action) handler.current(action);
    });
  }, []);

  const latest = useRef(state);
  latest.current = state;
  const key = JSON.stringify(state);
  useEffect(() => {
    if (!latest.current) return;
    const timer = setTimeout(() => {
      // Outside the desktop shell (a plain browser) there is no menu to update.
      Sync(latest.current!).catch(() => {});
    }, SYNC_DELAY_MS);
    return () => clearTimeout(timer);
  }, [key]);

  /** Send the state again, for changes the menu reads itself (recent files). */
  const resync = useCallback(() => {
    if (latest.current) Sync(latest.current).catch(() => {});
  }, []);
  return { resync };
}
