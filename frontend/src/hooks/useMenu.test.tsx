// @vitest-environment jsdom
import { act, renderHook } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import type { State } from "../../bindings/d2studio/menu/models";

const h = vi.hoisted(() => ({ handlers: {} as Record<string, (ev: { data?: unknown }) => unknown> }));
vi.mock("@wailsio/runtime", () => ({
  Events: {
    On: (name: string, cb: (ev: { data?: unknown }) => unknown) => {
      h.handlers[name] = cb;
      return () => delete h.handlers[name];
    },
  },
}));
vi.mock("../../bindings/d2studio/menuservice", () => ({ Sync: vi.fn(() => Promise.resolve()) }));

import { Sync } from "../../bindings/d2studio/menuservice";
import { useMenu } from "./useMenu";

const state = (o: Partial<State> = {}): State => ({
  viewMode: "split",
  sidebarOpen: true,
  hasFolder: false,
  autosave: "off",
  autosaveDelaySec: 2,
  layout: "",
  themeId: null,
  fetchRemoteImages: false,
  remoteImages: false,
  canUndo: false,
  canRedo: false,
  exporting: false,
  path: "",
  ...o,
});

const click = (id: string) => act(async () => void (await h.handlers["menu:action"]({ data: { id } })));
const advance = (ms: number) => act(async () => void (await vi.advanceTimersByTimeAsync(ms)));

beforeEach(() => {
  vi.clearAllMocks();
  vi.useFakeTimers();
  for (const k of Object.keys(h.handlers)) delete h.handlers[k];
});
afterEach(() => vi.useRealTimers());

describe("menu actions", () => {
  it("dispatches a parsed action when an item is chosen", async () => {
    const onAction = vi.fn();
    renderHook(() => useMenu(state(), onAction));
    await click("file.save");
    await click("view.mode:preview");
    await click("theme:200");
    expect(onAction.mock.calls.map((c) => c[0])).toEqual([
      { type: "save" },
      { type: "viewMode", mode: "preview" },
      { type: "theme", id: 200 },
    ]);
  });

  it("ignores IDs it does not understand", async () => {
    const onAction = vi.fn();
    renderHook(() => useMenu(state(), onAction));
    await click("file.explode");
    await click("");
    await act(async () => void (await h.handlers["menu:action"]({})));
    expect(onAction).not.toHaveBeenCalled();
  });

  it("always calls the latest handler, not the first one", async () => {
    const first = vi.fn();
    const second = vi.fn();
    const { rerender } = renderHook(({ fn }) => useMenu(state(), fn), { initialProps: { fn: first } });
    rerender({ fn: second });
    await click("file.new");
    expect(first).not.toHaveBeenCalled();
    expect(second).toHaveBeenCalledTimes(1);
  });

  it("stops listening when unmounted", () => {
    const { unmount } = renderHook(() => useMenu(state(), vi.fn()));
    expect(h.handlers["menu:action"]).toBeDefined();
    unmount();
    expect(h.handlers["menu:action"]).toBeUndefined();
  });
});

describe("menu state sync", () => {
  it("sends the state after a short delay", async () => {
    renderHook(() => useMenu(state({ viewMode: "source" }), vi.fn()));
    expect(Sync).not.toHaveBeenCalled();
    await advance(150);
    expect(Sync).toHaveBeenCalledTimes(1);
    expect(vi.mocked(Sync).mock.calls[0][0]).toMatchObject({ viewMode: "source" });
  });

  it("does not resend when the state has not changed", async () => {
    const { rerender } = renderHook(({ s }) => useMenu(s, vi.fn()), { initialProps: { s: state() } });
    await advance(150);
    rerender({ s: state() }); // a new object with identical contents
    await advance(500);
    expect(Sync).toHaveBeenCalledTimes(1);
  });

  it("sends a change, and coalesces a burst into one call", async () => {
    const { rerender } = renderHook(({ s }) => useMenu(s, vi.fn()), { initialProps: { s: state() } });
    await advance(150);
    rerender({ s: state({ canUndo: true }) });
    await advance(30);
    rerender({ s: state({ canUndo: true, canRedo: true }) });
    await advance(30);
    rerender({ s: state({ canUndo: true, canRedo: true, sidebarOpen: false }) });
    await advance(150);
    expect(Sync).toHaveBeenCalledTimes(2); // the first, then one for the whole burst
    expect(vi.mocked(Sync).mock.calls[1][0]).toMatchObject({ canUndo: true, canRedo: true, sidebarOpen: false });
  });

  it("waits until settings have loaded (null state)", async () => {
    renderHook(() => useMenu(null, vi.fn()));
    await advance(1_000);
    expect(Sync).not.toHaveBeenCalled();
  });

  it("resync resends the latest state immediately", async () => {
    const { result } = renderHook(() => useMenu(state({ hasFolder: true }), vi.fn()));
    await advance(150);
    act(() => result.current.resync());
    expect(Sync).toHaveBeenCalledTimes(2);
    expect(vi.mocked(Sync).mock.calls[1][0]).toMatchObject({ hasFolder: true });
  });

  it("is harmless when there is no desktop shell to sync with", async () => {
    vi.mocked(Sync).mockRejectedValueOnce(new Error("runtime unavailable"));
    renderHook(() => useMenu(state(), vi.fn()));
    await advance(150); // a rejected promise must not surface as an unhandled rejection
    expect(Sync).toHaveBeenCalledTimes(1);
  });
});
