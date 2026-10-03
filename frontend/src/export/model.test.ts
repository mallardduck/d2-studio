import { describe, expect, it } from "vitest";
import type { Result } from "../../bindings/d2studio/export/models";
import { describeResult, formatBytes } from "./model";

const base = (o: Partial<Result>): Result => ({
  cancelled: false,
  diagnostics: [],
  path: "/out/a.png",
  bytes: 2048,
  remoteImages: 0,
  remoteImagesBlocked: false,
  ...o,
});
const name = (p: string) => p.split("/").pop()!;

describe("describeResult", () => {
  it("reports a written file with its name and size", () => {
    expect(describeResult(base({}), name)).toEqual({ kind: "done", message: "Exported a.png (2.0 KB)" });
  });

  it("says nothing happened when the dialog was cancelled", () => {
    expect(describeResult(base({ cancelled: true, path: "" }), name)).toEqual({ kind: "cancelled" });
  });

  it("explains that a diagram with errors cannot be exported", () => {
    const r = describeResult(base({ path: "", diagnostics: [{} as never, {} as never] }), name);
    expect(r).toMatchObject({ kind: "errors" });
    expect((r as { message: string }).message).toContain("2 errors");
  });

  it("uses the singular for one error", () => {
    const r = describeResult(base({ path: "", diagnostics: [{} as never] }), name) as { message: string };
    expect(r.message).toContain("1 error.");
  });

  it("never claims success without a written file", () => {
    expect(describeResult(base({ path: "" }), name).kind).toBe("errors");
    expect(describeResult(null, name).kind).toBe("errors");
  });

  it("always calls out remote images that were replaced", () => {
    const one = describeResult(base({ remoteImages: 1, remoteImagesBlocked: true }), name) as { message: string };
    expect(one.message).toContain("1 remote image was replaced");
    expect(one.message).toContain("Fetch remote images");
    const many = describeResult(base({ remoteImages: 3, remoteImagesBlocked: true }), name) as { message: string };
    expect(many.message).toContain("3 remote images were replaced");
  });

  it("does not mention images that were fetched or absent", () => {
    expect(
      (describeResult(base({ remoteImages: 2, remoteImagesBlocked: false }), name) as { message: string }).message,
    ).not.toContain("remote");
  });
});

describe("formatBytes", () => {
  it("scales units", () => {
    expect(formatBytes(512)).toBe("512 B");
    expect(formatBytes(2048)).toBe("2.0 KB");
    expect(formatBytes(50 * 1024)).toBe("50 KB");
    expect(formatBytes(3 * 1024 * 1024)).toBe("3.0 MB");
  });
});
