import { useEffect, useState } from "react";
import { List } from "../../bindings/d2studio/snippetservice";
import type { Snippet } from "../../bindings/d2studio/snippets/models";

/** The built-in snippet list, loaded once. */
export function useSnippets(): { snippets: Snippet[]; error: string } {
  const [snippets, setSnippets] = useState<Snippet[]>([]);
  const [error, setError] = useState("");
  useEffect(() => {
    List().then(
      (l) => setSnippets(l ?? []),
      (e) => setError(String(e)),
    );
  }, []);
  return { snippets, error };
}
