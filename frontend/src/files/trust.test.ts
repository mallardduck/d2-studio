import { describe, expect, it } from "vitest";
import { MAX_TRUSTED_FILES, isTrusted, withTrust } from "./trust";

describe("isTrusted", () => {
  it("is false unless the file was explicitly allowed", () => {
    expect(isTrusted([], "/a.d2")).toBe(false);
    expect(isTrusted(null, "/a.d2")).toBe(false);
    expect(isTrusted(undefined, "/a.d2")).toBe(false);
    expect(isTrusted(["/b.d2"], "/a.d2")).toBe(false);
  });

  it("is false for an unsaved document, which has no path to remember", () => {
    expect(isTrusted(["/a.d2"], null)).toBe(false);
    expect(isTrusted(["/a.d2"], "")).toBe(false);
  });

  it("is true for an allowed file, matching the whole path", () => {
    expect(isTrusted(["/a.d2", "/b.d2"], "/b.d2")).toBe(true);
    expect(isTrusted(["/work/a.d2"], "/a.d2")).toBe(false);
  });
});

describe("withTrust", () => {
  it("adds a file at the end and removes it again", () => {
    expect(withTrust(["/a.d2"], "/b.d2", true)).toEqual(["/a.d2", "/b.d2"]);
    expect(withTrust(["/a.d2", "/b.d2"], "/a.d2", false)).toEqual(["/b.d2"]);
  });

  it("does not duplicate, and moves a re-allowed file to the newest position", () => {
    expect(withTrust(["/a.d2", "/b.d2"], "/a.d2", true)).toEqual(["/b.d2", "/a.d2"]);
  });

  it("revoking a file that was never allowed changes nothing", () => {
    expect(withTrust(["/a.d2"], "/z.d2", false)).toEqual(["/a.d2"]);
    expect(withTrust(null, "/z.d2", false)).toEqual([]);
  });

  it("keeps only the newest files when the list is full", () => {
    const full = Array.from({ length: MAX_TRUSTED_FILES }, (_, i) => `/f${i}.d2`);
    const next = withTrust(full, "/new.d2", true);
    expect(next).toHaveLength(MAX_TRUSTED_FILES);
    expect(next[0]).toBe("/f1.d2");
    expect(next[next.length - 1]).toBe("/new.d2");
  });

  it("does not modify the list it was given", () => {
    const original = ["/a.d2"];
    withTrust(original, "/b.d2", true);
    expect(original).toEqual(["/a.d2"]);
  });
});
