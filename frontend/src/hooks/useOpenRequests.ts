import { useEffect, useRef } from "react";
import { Events } from "@wailsio/runtime";
import { Take } from "../../bindings/d2studio/openservice";

/**
 * Opens the files the operating system asks for (double-click, "Open With", a
 * path on the command line, or a second launch of the app).
 *
 * Requests are queued by the backend, so one that arrives before the window
 * has loaded is collected here at startup, and later ones when the backend
 * signals. A document holds one file, so the most recent request is opened;
 * `open` asks about unsaved changes first, like any other way of opening.
 */
export function useOpenRequests(open: (path: string) => void | Promise<void>) {
  const opener = useRef(open);
  opener.current = open;

  useEffect(() => {
    let cancelled = false;
    const collect = () => {
      Take()
        .then((paths) => {
          const latest = paths?.[paths.length - 1];
          if (!cancelled && latest) void opener.current(latest);
        })
        .catch(() => {
          // Not running inside the desktop shell: there is nothing to collect.
        });
    };
    collect();
    const off = Events.On("app:open-requested", collect);
    return () => {
      cancelled = true;
      off();
    };
  }, []);
}
