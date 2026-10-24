// @vitest-environment jsdom
import { renderHook, act } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

const h = vi.hoisted(() => ({ handlers: {} as Record<string, () => unknown> }));
vi.mock("@wailsio/runtime", () => ({
  Events: {
    On: (name: string, cb: () => unknown) => {
      h.handlers[name] = cb;
      return () => delete h.handlers[name];
    },
  },
}));
vi.mock("../../bindings/d2studio/openservice", () => ({ Take: vi.fn() }));

import { Take } from "../../bindings/d2studio/openservice";
import { useOpenRequests } from "./useOpenRequests";

const flush = () => act(async () => void (await Promise.resolve()));

beforeEach(() => {
  vi.clearAllMocks();
  for (const k of Object.keys(h.handlers)) delete h.handlers[k];
});

describe("useOpenRequests", () => {
  it("opens a file that was requested before the window loaded", async () => {
    vi.mocked(Take).mockReturnValueOnce(Promise.resolve(["/w/a.d2"]) as never);
    const open = vi.fn();
    renderHook(() => useOpenRequests(open));
    await flush();
    expect(open).toHaveBeenCalledWith("/w/a.d2");
  });

  it("does nothing when nothing was requested", async () => {
    vi.mocked(Take).mockReturnValue(Promise.resolve([]) as never);
    const open = vi.fn();
    renderHook(() => useOpenRequests(open));
    await flush();
    expect(open).not.toHaveBeenCalled();
  });

  it("opens the most recent request when several are waiting", async () => {
    vi.mocked(Take).mockReturnValueOnce(Promise.resolve(["/a.d2", "/b.d2", "/c.d2"]) as never);
    const open = vi.fn();
    renderHook(() => useOpenRequests(open));
    await flush();
    expect(open).toHaveBeenCalledTimes(1);
    expect(open).toHaveBeenCalledWith("/c.d2");
  });

  it("collects again when the backend signals a new request", async () => {
    vi.mocked(Take).mockReturnValueOnce(Promise.resolve([]) as never);
    const open = vi.fn();
    renderHook(() => useOpenRequests(open));
    await flush();

    vi.mocked(Take).mockReturnValueOnce(Promise.resolve(["/second.d2"]) as never);
    await act(async () => void h.handlers["app:open-requested"]());
    await flush();
    expect(open).toHaveBeenCalledWith("/second.d2");
  });

  it("always uses the latest opener, not the first one", async () => {
    vi.mocked(Take).mockReturnValueOnce(Promise.resolve([]) as never);
    const first = vi.fn();
    const second = vi.fn();
    const { rerender } = renderHook(({ fn }) => useOpenRequests(fn), { initialProps: { fn: first } });
    await flush();
    rerender({ fn: second });
    vi.mocked(Take).mockReturnValueOnce(Promise.resolve(["/x.d2"]) as never);
    await act(async () => void h.handlers["app:open-requested"]());
    await flush();
    expect(first).not.toHaveBeenCalled();
    expect(second).toHaveBeenCalledWith("/x.d2");
  });

  it("is harmless outside the desktop shell", async () => {
    vi.mocked(Take).mockReturnValue(Promise.reject(new Error("no runtime")) as never);
    const open = vi.fn();
    renderHook(() => useOpenRequests(open));
    await flush();
    expect(open).not.toHaveBeenCalled();
  });

  it("stops listening and ignores late results after unmounting", async () => {
    let resolve!: (p: string[]) => void;
    vi.mocked(Take).mockReturnValueOnce(new Promise<string[]>((r) => (resolve = r)) as never);
    const open = vi.fn();
    const { unmount } = renderHook(() => useOpenRequests(open));
    unmount();
    expect(h.handlers["app:open-requested"]).toBeUndefined();
    resolve(["/late.d2"]);
    await flush();
    expect(open).not.toHaveBeenCalled();
  });
});
