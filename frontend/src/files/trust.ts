/** How many files can be remembered; the backend keeps the same number. */
export const MAX_TRUSTED_FILES = 100;

/** Whether the user has allowed this file to load remote images. */
export function isTrusted(trusted: readonly string[] | null | undefined, path: string | null): boolean {
  return !!path && !!trusted && trusted.includes(path);
}

/**
 * The list after allowing (or no longer allowing) a file. Order is oldest
 * first, so the newest are kept if the list is full.
 */
export function withTrust(trusted: readonly string[] | null | undefined, path: string, allow: boolean): string[] {
  const rest = (trusted ?? []).filter((p) => p !== path);
  if (!allow) return rest;
  return [...rest, path].slice(-MAX_TRUSTED_FILES);
}
