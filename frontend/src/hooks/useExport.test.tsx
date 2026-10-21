// @vitest-environment jsdom
import { act, renderHook } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { Format } from "../../bindings/d2studio/export/models";

vi.mock("../../bindings/d2studio/export/service", () => ({ Export: vi.fn() }));
import { Export } from "../../bindings/d2studio/export/service";
import { useExport } from "./useExport";

const written = (o: object = {}) => ({
  cancelled: false,
  diagnostics: [],
  path: "/out/diagram.png",
  bytes: 4096,
  remoteImages: 0,
  remoteImagesBlocked: false,
  ...o,
});

function setup(overrides: object = {}) {
  const notify = vi.fn();
  const showError = vi.fn();
  const hook = renderHook(() =>
    useExport({
      compile: {
        source: "a -> b",
        options: { layout: "elk", themeId: null, sketch: null, remoteImages: false },
        path: "/w/a.d2",
        workspace: "/w",
      },
      documentName: "a.d2",
      dir: "/w",
      allowRemoteImages: true,
      notify,
      showError,
      ...overrides,
    }),
  );
  return { ...hook, notify, showError };
}

beforeEach(() => vi.clearAllMocks());

describe("useExport", () => {
  it("sends the editor's content, settings and file context", async () => {
    vi.mocked(Export).mockResolvedValueOnce(written());
    const t = setup();
    await act(async () => t.result.current.run(Format.PNG));
    expect(Export).toHaveBeenCalledWith({
      format: "png",
      compile: {
        source: "a -> b",
        options: { layout: "elk", themeId: null, sketch: null, remoteImages: false },
        path: "/w/a.d2",
        workspace: "/w",
      },
      suggestedName: "a.d2",
      dir: "/w",
      allowRemoteImages: true,
    });
  });

  it("reports a successful export with the file name", async () => {
    vi.mocked(Export).mockResolvedValueOnce(written());
    const t = setup();
    await act(async () => t.result.current.run(Format.PNG));
    expect(t.notify).toHaveBeenCalledWith("Exported diagram.png (4.0 KB)");
    expect(t.showError).not.toHaveBeenCalled();
  });

  it("says nothing when the user cancels the dialog", async () => {
    vi.mocked(Export).mockResolvedValueOnce(written({ cancelled: true, path: "" }));
    const t = setup();
    await act(async () => t.result.current.run(Format.SVG));
    expect(t.notify).not.toHaveBeenCalled();
    expect(t.showError).not.toHaveBeenCalled();
  });

  it("shows an error, not a success, when the diagram has errors", async () => {
    vi.mocked(Export).mockResolvedValueOnce(written({ path: "", diagnostics: [{}] }));
    const t = setup();
    await act(async () => t.result.current.run(Format.PDF));
    expect(t.showError).toHaveBeenCalledWith(expect.stringContaining("Cannot export"));
    expect(t.notify).not.toHaveBeenCalled();
  });

  it("shows an error, not a success, when the backend throws", async () => {
    vi.mocked(Export).mockRejectedValueOnce(new Error("could not write out.png: disk full"));
    const t = setup();
    await act(async () => t.result.current.run(Format.PNG));
    expect(t.showError).toHaveBeenCalledWith("Export failed: could not write out.png: disk full");
    expect(t.notify).not.toHaveBeenCalled();
  });

  it("tells the user when remote images were replaced", async () => {
    vi.mocked(Export).mockResolvedValueOnce(written({ remoteImages: 2, remoteImagesBlocked: true }));
    const t = setup({ allowRemoteImages: false });
    await act(async () => t.result.current.run(Format.PNG));
    expect(t.notify).toHaveBeenCalledWith(expect.stringContaining("2 remote images were replaced"));
  });

  it("runs one export at a time and is usable again afterwards", async () => {
    let finish!: (r: ReturnType<typeof written>) => void;
    vi.mocked(Export).mockReturnValueOnce(new Promise((r) => (finish = r)) as never);
    const t = setup();

    let first!: Promise<void>;
    act(() => {
      first = t.result.current.run(Format.PNG);
    });
    expect(t.result.current.busy).toBe(true);

    await act(async () => t.result.current.run(Format.PDF)); // ignored while busy
    expect(Export).toHaveBeenCalledTimes(1);

    await act(async () => {
      finish(written());
      await first;
    });
    expect(t.result.current.busy).toBe(false);

    vi.mocked(Export).mockResolvedValueOnce(written());
    await act(async () => t.result.current.run(Format.SVG));
    expect(Export).toHaveBeenCalledTimes(2);
  });

  it("is not stuck busy after a failure", async () => {
    vi.mocked(Export).mockRejectedValueOnce(new Error("boom"));
    const t = setup();
    await act(async () => t.result.current.run(Format.PNG));
    expect(t.result.current.busy).toBe(false);
  });
});
