// @vitest-environment jsdom
import { act, renderHook, waitFor } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import type { DialogSpec } from "../components/ConfirmDialog";

const h = vi.hoisted(() => ({ handlers: {} as Record<string, (ev: { data?: unknown }) => unknown> }));

vi.mock("@wailsio/runtime", () => ({
  Events: {
    On: (name: string, cb: (ev: { data?: unknown }) => unknown) => {
      h.handlers[name] = cb;
      return () => delete h.handlers[name];
    },
  },
  Window: { SetTitle: () => Promise.resolve() },
}));
vi.mock("../../bindings/d2studio/appservice", () => ({ ConfirmQuit: vi.fn(() => Promise.resolve()) }));
vi.mock("../../bindings/d2studio/files/service", () => ({
  OpenDialog: vi.fn(),
  OpenPath: vi.fn(),
  RemoveRecent: vi.fn(() => Promise.resolve()),
  Save: vi.fn(),
  SaveAs: vi.fn(),
  Unwatch: vi.fn(() => Promise.resolve()),
}));

import { ConfirmQuit } from "../../bindings/d2studio/appservice";
import { OpenDialog, OpenPath, RemoveRecent, Save, SaveAs, Unwatch } from "../../bindings/d2studio/files/service";
import { useDocument } from "./useDocument";

const v = (hash: string) => ({ modTimeNs: 1, size: 1, hash });
const fileDoc = (path: string, content: string, hash = "h1") => ({
  path,
  content,
  version: v(hash),
  lineEnding: "lf",
  bom: false,
  mixedLineEndings: false,
});

/** A scripted confirm(): answers each dialog with the next queued choice. */
function scriptedConfirm(...choices: string[]) {
  const queue = [...choices];
  const shown: DialogSpec[] = [];
  const confirm = vi.fn(async (spec: DialogSpec) => {
    shown.push(spec);
    const next = queue.shift();
    if (next === undefined) throw new Error(`unexpected dialog: ${spec.title}`);
    return next;
  });
  return { confirm, shown };
}

/**
 * Mounts the hook clean, opens "/w/a.d2" (saved text "saved"), then puts the
 * editor on `content`, the way a real edit would.
 */
async function mountWithFile(content = "saved", choices: string[] = []) {
  const { confirm, shown } = scriptedConfirm(...choices);
  const setContent = vi.fn();
  vi.mocked(OpenDialog).mockResolvedValueOnce({ cancelled: false, document: fileDoc("/w/a.d2", "saved") });
  const hook = renderHook(
    ({ text }) => useDocument({ content: text, setContent, confirm, initialContent: "init" }),
    { initialProps: { text: "init" } },
  );
  await act(async () => {
    await hook.result.current.open();
  });
  // The real editor now shows the opened file's text, then the user edits it.
  hook.rerender({ text: "saved" });
  if (content !== "saved") hook.rerender({ text: content });
  setContent.mockClear();
  return { ...hook, setContent, confirm, shown, edit: (text: string) => hook.rerender({ text }) };
}

beforeEach(() => {
  vi.clearAllMocks();
  for (const k of Object.keys(h.handlers)) delete h.handlers[k];
});

describe("dirty tracking", () => {
  it("treats the starting document as saved", () => {
    const { result } = renderHook(() =>
      useDocument({ content: "init", setContent: vi.fn(), confirm: vi.fn(), initialContent: "init" }),
    );
    expect(result.current.dirty).toBe(false);
    expect(result.current.name).toBe("Untitled.d2");
  });

  it("is dirty exactly when the text differs from what was saved", async () => {
    const t = await mountWithFile();
    expect(t.result.current.dirty).toBe(false);
    t.edit("saved plus edits");
    expect(t.result.current.dirty).toBe(true);
    t.edit("saved");
    expect(t.result.current.dirty).toBe(false);
  });
});

describe("discarding unsaved work needs consent", () => {
  it("Cancel keeps the document and does nothing else", async () => {
    const t = await mountWithFile("edited", ["cancel"]);
    await act(async () => {
      await t.result.current.newDocument();
    });
    expect(t.shown[0].title).toContain("Save changes");
    expect(t.setContent).not.toHaveBeenCalled();
    expect(Unwatch).not.toHaveBeenCalled();
    expect(t.result.current.name).toBe("a.d2");
  });

  it("Don't save proceeds without writing anything", async () => {
    const t = await mountWithFile("edited", ["discard"]);
    await act(async () => {
      await t.result.current.newDocument();
    });
    expect(Save).not.toHaveBeenCalled();
    expect(t.setContent).toHaveBeenCalledWith("");
    expect(t.result.current.name).toBe("Untitled.d2");
  });

  it("Save writes first, then proceeds", async () => {
    vi.mocked(Save).mockResolvedValueOnce({ saved: true, conflict: "", version: v("h2") });
    const t = await mountWithFile("edited", ["save"]);
    await act(async () => {
      await t.result.current.newDocument();
    });
    expect(Save).toHaveBeenCalledWith("/w/a.d2", "edited", expect.objectContaining({ force: false }));
    expect(t.setContent).toHaveBeenCalledWith("");
  });

  it("a failed Save stops the operation instead of discarding", async () => {
    vi.mocked(Save).mockRejectedValueOnce(new Error("disk full"));
    const t = await mountWithFile("edited", ["save"]);
    await act(async () => {
      await t.result.current.newDocument();
    });
    expect(t.setContent).not.toHaveBeenCalled();
    expect(t.result.current.banner).toEqual({ kind: "error", message: "Could not save: disk full" });
  });

  it("does not prompt when there is nothing unsaved", async () => {
    const t = await mountWithFile("saved");
    await act(async () => {
      await t.result.current.newDocument();
    });
    expect(t.confirm).not.toHaveBeenCalled();
    expect(t.setContent).toHaveBeenCalledWith("");
  });
});

describe("saving", () => {
  it("clears the dirty state and uses the version as the base", async () => {
    vi.mocked(Save).mockResolvedValueOnce({ saved: true, conflict: "", version: v("h2") });
    const t = await mountWithFile("edited");
    await act(async () => {
      await t.result.current.save();
    });
    expect(Save).toHaveBeenCalledWith("/w/a.d2", "edited", {
      base: v("h1"),
      force: false,
      lineEnding: "lf",
      bom: false,
    });
    expect(t.result.current.dirty).toBe(false);
  });

  it("text typed while saving stays dirty", async () => {
    let finish!: (r: { saved: boolean; conflict: string; version: ReturnType<typeof v> }) => void;
    vi.mocked(Save).mockReturnValueOnce(new Promise((r) => (finish = r)) as never);
    const t = await mountWithFile("edited");
    let saving!: Promise<boolean>;
    act(() => {
      saving = t.result.current.save();
    });
    t.edit("edited and more"); // typed while the write is in flight
    await act(async () => {
      finish({ saved: true, conflict: "", version: v("h2") });
      await saving;
    });
    expect(t.result.current.dirty).toBe(true);
  });

  it("an untitled document goes through Save As", async () => {
    vi.mocked(SaveAs).mockResolvedValueOnce({ cancelled: false, path: "/w/new.d2", name: "new.d2", version: v("n1") });
    const setContent = vi.fn();
    const { result } = renderHook(() =>
      useDocument({ content: "x", setContent, confirm: vi.fn(), initialContent: "" }),
    );
    await act(async () => {
      await result.current.save();
    });
    expect(Save).not.toHaveBeenCalled();
    expect(result.current.name).toBe("new.d2");
    expect(result.current.dirty).toBe(false);
  });

  it("a cancelled Save As leaves everything as it was", async () => {
    vi.mocked(SaveAs).mockResolvedValueOnce({ cancelled: true, path: "", name: "", version: v("") });
    const { result } = renderHook(() =>
      useDocument({ content: "x", setContent: vi.fn(), confirm: vi.fn(), initialContent: "" }),
    );
    await act(async () => {
      expect(await result.current.save()).toBe(false);
    });
    expect(result.current.dirty).toBe(true);
    expect(result.current.name).toBe("Untitled.d2");
  });
});

describe("save conflicts never overwrite silently", () => {
  const conflict = { saved: false, conflict: "modified", version: v("theirs") };

  it("Cancel leaves the file alone", async () => {
    vi.mocked(Save).mockResolvedValueOnce(conflict);
    const t = await mountWithFile("edited", ["cancel"]);
    let ok = true;
    await act(async () => {
      ok = await t.result.current.save();
    });
    expect(ok).toBe(false);
    expect(Save).toHaveBeenCalledTimes(1);
    expect(t.result.current.dirty).toBe(true);
  });

  it("the default (focused) choice is the safe one", async () => {
    vi.mocked(Save).mockResolvedValueOnce(conflict);
    const t = await mountWithFile("edited", ["cancel"]);
    await act(async () => {
      await t.result.current.save();
    });
    const primary = t.shown[0].buttons.find((b) => b.kind === "primary");
    expect(primary?.id).toBe("cancel");
    expect(t.shown[0].buttons.find((b) => b.id === "overwrite")?.kind).toBe("danger");
  });

  it("Overwrite saves again with force", async () => {
    vi.mocked(Save)
      .mockResolvedValueOnce(conflict)
      .mockResolvedValueOnce({ saved: true, conflict: "", version: v("h3") });
    const t = await mountWithFile("edited", ["overwrite"]);
    await act(async () => {
      await t.result.current.save();
    });
    expect(vi.mocked(Save).mock.calls[1][2]).toMatchObject({ force: true });
    expect(t.result.current.dirty).toBe(false);
  });

  it("Reload from disk replaces the editor content", async () => {
    vi.mocked(Save).mockResolvedValueOnce(conflict);
    vi.mocked(OpenPath).mockResolvedValueOnce({ cancelled: false, document: fileDoc("/w/a.d2", "theirs", "theirs") });
    const t = await mountWithFile("edited", ["reload"]);
    await act(async () => {
      await t.result.current.save();
    });
    expect(t.setContent).toHaveBeenCalledWith("theirs");
  });

  it("a deleted file offers to recreate or save elsewhere", async () => {
    vi.mocked(Save).mockResolvedValueOnce({ saved: false, conflict: "deleted", version: v("") });
    const t = await mountWithFile("edited", ["cancel"]);
    await act(async () => {
      await t.result.current.save();
    });
    expect(t.shown[0].buttons.map((b) => b.id)).toEqual(["cancel", "saveas", "recreate"]);
    expect(Save).toHaveBeenCalledTimes(1);
  });
});

describe("external changes", () => {
  const emit = (data: object) => act(async () => void (await h.handlers["file:changed"]({ data })));

  it("reloads silently when there are no unsaved edits", async () => {
    vi.mocked(OpenPath).mockResolvedValueOnce({ cancelled: false, document: fileDoc("/w/a.d2", "from disk") });
    const t = await mountWithFile("saved");
    await emit({ path: "/w/a.d2", kind: "modified", version: v("x"), message: "" });
    await waitFor(() => expect(t.setContent).toHaveBeenCalledWith("from disk"));
    expect(t.result.current.notice).toContain("Reloaded");
  });

  it("NEVER replaces unsaved edits: it shows a banner instead", async () => {
    const t = await mountWithFile("my unsaved edits");
    await emit({ path: "/w/a.d2", kind: "modified", version: v("x"), message: "" });
    expect(t.setContent).not.toHaveBeenCalled();
    expect(OpenPath).not.toHaveBeenCalled();
    expect(t.result.current.banner).toMatchObject({ kind: "modified" });
    expect(t.result.current.dirty).toBe(true);
  });

  it("Keep my edits makes the next save an explicit overwrite, not a conflict", async () => {
    vi.mocked(Save).mockResolvedValueOnce({ saved: true, conflict: "", version: v("z") });
    const t = await mountWithFile("my edits");
    await emit({ path: "/w/a.d2", kind: "modified", version: v("theirs"), message: "" });
    act(() => t.result.current.keepMine(v("theirs")));
    expect(t.result.current.banner).toBeNull();
    await act(async () => {
      await t.result.current.save();
    });
    expect(vi.mocked(Save).mock.calls[0][2]).toMatchObject({ base: v("theirs"), force: false });
  });

  it("reports a deleted file without touching the editor", async () => {
    const t = await mountWithFile("my edits");
    await emit({ path: "/w/a.d2", kind: "deleted", version: v(""), message: "" });
    expect(t.result.current.banner).toEqual({ kind: "deleted" });
    expect(t.setContent).not.toHaveBeenCalled();
  });

  it("clears the warning when the file comes back", async () => {
    const t = await mountWithFile("my edits");
    await emit({ path: "/w/a.d2", kind: "deleted", version: v(""), message: "" });
    await emit({ path: "/w/a.d2", kind: "restored", version: v("h1"), message: "" });
    expect(t.result.current.banner).toBeNull();
  });

  it("ignores events for a file that is no longer open", async () => {
    const t = await mountWithFile("my edits");
    await emit({ path: "/w/other.d2", kind: "deleted", version: v(""), message: "" });
    expect(t.result.current.banner).toBeNull();
  });
});

describe("opening files", () => {
  it("a failed open shows an error and forgets the missing recent file", async () => {
    vi.mocked(OpenPath).mockRejectedValueOnce(new Error("/w/gone.d2 does not exist"));
    const t = await mountWithFile("saved");
    await act(async () => {
      await t.result.current.openPath("/w/gone.d2");
    });
    expect(t.result.current.banner).toMatchObject({ kind: "error" });
    expect(RemoveRecent).toHaveBeenCalledWith("/w/gone.d2");
    expect(t.result.current.name).toBe("a.d2"); // the open document is untouched
  });

  it("a cancelled Open dialog changes nothing", async () => {
    const t = await mountWithFile("saved");
    vi.mocked(OpenDialog).mockResolvedValueOnce({ cancelled: true, document: null });
    await act(async () => {
      await t.result.current.open();
    });
    expect(t.setContent).not.toHaveBeenCalled();
    expect(t.result.current.name).toBe("a.d2");
  });
});

describe("quitting", () => {
  const requestClose = () => act(async () => void (await h.handlers["app:close-requested"]({})));

  it("quits straight away when nothing is unsaved", async () => {
    await mountWithFile("saved");
    await requestClose();
    expect(ConfirmQuit).toHaveBeenCalledTimes(1);
  });

  it("asks first when there are unsaved edits, and Cancel stays open", async () => {
    const t = await mountWithFile("edited", ["cancel"]);
    await requestClose();
    expect(t.shown[0].title).toContain("Save changes");
    expect(ConfirmQuit).not.toHaveBeenCalled();
  });

  it("quits after the user chooses not to save", async () => {
    await mountWithFile("edited", ["discard"]);
    await requestClose();
    expect(ConfirmQuit).toHaveBeenCalledTimes(1);
  });

  it("does not quit when saving fails", async () => {
    vi.mocked(Save).mockRejectedValueOnce(new Error("read-only"));
    await mountWithFile("edited", ["save"]);
    await requestClose();
    expect(ConfirmQuit).not.toHaveBeenCalled();
  });
});
