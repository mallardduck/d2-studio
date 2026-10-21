import { useCallback, useEffect, useRef, useState } from "react";
import type { Settings } from "../../bindings/d2studio/settings/models";
import { isTrusted, withTrust } from "../files/trust";

/**
 * Whether the preview may load remote images for the open file.
 *
 * It is a choice per file, remembered in settings, so opening a file never
 * makes a network request unless the user already allowed that file. An
 * unsaved document has no path to remember, so for it the choice lasts for the
 * session, and is kept when the document is first saved.
 */
export function useRemoteImages(
  settings: Pick<Settings, "remoteImageFiles"> | null,
  path: string | null,
  update: (patch: Partial<Settings>) => void,
) {
  const [sessionOn, setSessionOn] = useState(false);
  const on = path ? isTrusted(settings?.remoteImageFiles, path) : sessionOn;

  const toggle = useCallback(() => {
    if (!settings) return;
    if (path) update({ remoteImageFiles: withTrust(settings.remoteImageFiles, path, !on) });
    else setSessionOn(!on);
  }, [on, path, settings, update]);

  const previousPath = useRef(path);
  useEffect(() => {
    if (previousPath.current === null && path && sessionOn && settings) {
      update({ remoteImageFiles: withTrust(settings.remoteImageFiles, path, true) });
      setSessionOn(false);
    }
    previousPath.current = path;
  }, [path, sessionOn, settings, update]);

  return { on, toggle };
}
