// @vitest-environment jsdom
import { act, renderHook } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";
import { useRemoteImages } from "./useRemoteImages";

type Props = { path: string | null; files: string[] };

function setup(initial: Props) {
  const update = vi.fn();
  const hook = renderHook(({ path, files }: Props) => useRemoteImages({ remoteImageFiles: files }, path, update), {
    initialProps: initial,
  });
  return { ...hook, update };
}

describe("useRemoteImages", () => {
  it("is off for a file that was never allowed, so opening it makes no request", () => {
    expect(setup({ path: "/a.d2", files: [] }).result.current.on).toBe(false);
    expect(setup({ path: "/a.d2", files: ["/b.d2"] }).result.current.on).toBe(false);
  });

  it("is on only for files that were allowed", () => {
    const t = setup({ path: "/a.d2", files: ["/a.d2"] });
    expect(t.result.current.on).toBe(true);
    t.rerender({ path: "/b.d2", files: ["/a.d2"] });
    expect(t.result.current.on).toBe(false);
  });

  it("allowing a saved file remembers it", () => {
    const t = setup({ path: "/a.d2", files: ["/x.d2"] });
    act(() => t.result.current.toggle());
    expect(t.update).toHaveBeenCalledWith({ remoteImageFiles: ["/x.d2", "/a.d2"] });
  });

  it("turning it off again forgets the file", () => {
    const t = setup({ path: "/a.d2", files: ["/x.d2", "/a.d2"] });
    act(() => t.result.current.toggle());
    expect(t.update).toHaveBeenCalledWith({ remoteImageFiles: ["/x.d2"] });
  });

  it("an unsaved document uses a session choice and saves nothing", () => {
    const t = setup({ path: null, files: [] });
    expect(t.result.current.on).toBe(false);
    act(() => t.result.current.toggle());
    expect(t.result.current.on).toBe(true);
    expect(t.update).not.toHaveBeenCalled();
    act(() => t.result.current.toggle());
    expect(t.result.current.on).toBe(false);
  });

  it("keeps the choice when an unsaved document is first saved", () => {
    const t = setup({ path: null, files: ["/x.d2"] });
    act(() => t.result.current.toggle());
    t.rerender({ path: "/new.d2", files: ["/x.d2"] });
    expect(t.update).toHaveBeenCalledWith({ remoteImageFiles: ["/x.d2", "/new.d2"] });
  });

  it("does not trust a newly saved document the user never allowed", () => {
    const t = setup({ path: null, files: [] });
    t.rerender({ path: "/new.d2", files: [] });
    expect(t.update).not.toHaveBeenCalled();
    expect(t.result.current.on).toBe(false);
  });

  it("does nothing before settings have loaded", () => {
    const update = vi.fn();
    const { result } = renderHook(() => useRemoteImages(null, "/a.d2", update));
    act(() => result.current.toggle());
    expect(update).not.toHaveBeenCalled();
    expect(result.current.on).toBe(false);
  });
});
