import { Format, type Result } from "../../bindings/d2studio/export/models";

export interface FormatInfo {
  format: Format;
  label: string;
  /** One line shown in the menu so the trade-off is clear. */
  hint: string;
}

export const FORMATS: FormatInfo[] = [
  { format: Format.SVG, label: "SVG", hint: "Vector image; scales without losing quality" },
  { format: Format.PNG, label: "PNG", hint: "Image (2× resolution)" },
  { format: Format.PDF, label: "PDF", hint: "Image on a page; text is not selectable" },
];

/** What to tell the user about a finished export request. */
export type Outcome = { kind: "cancelled" } | { kind: "errors"; message: string } | { kind: "done"; message: string };

export function formatBytes(n: number): string {
  if (n < 1024) return `${n} B`;
  if (n < 1024 * 1024) return `${(n / 1024).toFixed(n < 10 * 1024 ? 1 : 0)} KB`;
  return `${(n / (1024 * 1024)).toFixed(1)} MB`;
}

const plural = (n: number, one: string, many: string) => `${n} ${n === 1 ? one : many}`;

/**
 * Turns an export result into a message. Success is only claimed when the
 * backend reports a written file, and images that were replaced by
 * placeholders are always called out.
 */
export function describeResult(result: Result | null, fileName: (path: string) => string): Outcome {
  if (!result) return { kind: "errors", message: "Export returned no result." };
  if (result.cancelled) return { kind: "cancelled" };

  const diagnostics = result.diagnostics ?? [];
  if (diagnostics.length > 0) {
    return {
      kind: "errors",
      message: `Cannot export: the diagram has ${plural(diagnostics.length, "error", "errors")}. Fix them and try again.`,
    };
  }
  if (!result.path) {
    // Never report success without a file.
    return { kind: "errors", message: "Export finished without writing a file." };
  }

  let message = `Exported ${fileName(result.path)} (${formatBytes(result.bytes)})`;
  if (result.remoteImagesBlocked) {
    message += `. ${plural(result.remoteImages, "remote image was", "remote images were")} replaced by a placeholder; turn on “Fetch remote images” in the Export menu to include them`;
  }
  return { kind: "done", message };
}
