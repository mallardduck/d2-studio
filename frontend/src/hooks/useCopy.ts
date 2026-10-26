import { useCallback, useEffect, useRef, useState } from "react";

/** Copy text to the clipboard and remember which key was copied for a moment. */
export function useCopy(resetMs = 1500): { copied: string; copy: (key: string, text: string) => void } {
  const [copied, setCopied] = useState("");
  const timer = useRef<ReturnType<typeof setTimeout>>(undefined);
  useEffect(() => () => clearTimeout(timer.current), []);
  const copy = useCallback(
    (key: string, text: string) => {
      navigator.clipboard.writeText(text).then(
        () => {
          setCopied(key);
          clearTimeout(timer.current);
          timer.current = setTimeout(() => setCopied(""), resetMs);
        },
        () => {}, // clipboard unavailable: nothing to confirm
      );
    },
    [resetMs],
  );
  return { copied, copy };
}
