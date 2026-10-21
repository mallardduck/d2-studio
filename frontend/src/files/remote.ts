export interface RemoteImageMeta {
  remoteImages: number;
  remoteImagesLoaded: number;
}

const plural = (n: number, one: string, many: string) => `${n} ${n === 1 ? one : many}`;

/**
 * A short note for the status bar about remote images, or null when there is
 * nothing to say. When remote images are off it says how many are hidden, and
 * that the user can show them; when on, it only speaks up about failures.
 */
export function remoteImagesNote(
  meta: RemoteImageMeta | null,
  allowed: boolean,
): { text: string; canShow: boolean } | null {
  if (!meta || meta.remoteImages === 0) return null;
  if (!allowed) {
    return { text: `${plural(meta.remoteImages, "remote image", "remote images")} hidden`, canShow: true };
  }
  const failed = meta.remoteImages - meta.remoteImagesLoaded;
  if (failed > 0) {
    return {
      text: `${failed} of ${plural(meta.remoteImages, "remote image", "remote images")} could not be loaded`,
      canShow: false,
    };
  }
  return null;
}
