import { useCallback, useEffect, useRef, useState } from "react";
import { Events, Window } from "@wailsio/runtime";
import { ConfirmQuit } from "../../bindings/d2studio/appservice";
import { OpenDialog, OpenPath, RemoveRecent, Save, SaveAs, Unwatch } from "../../bindings/d2studio/files/service";
import type { Document, SaveOptions, Version } from "../../bindings/d2studio/files/models";
import type { Banner } from "../components/FileBanner";
import type { DialogSpec } from "../components/ConfirmDialog";
import { UNTITLED, baseName, decideExternal, dirOf, encodingLabel, windowTitle } from "../files/model";

interface DocState {
  /** Absolute path, or null for a document that has never been saved. */
  path: string | null;
  name: string;
  /** On-disk version at the last open/save; the base for conflict checks. */
  version: Version | null;
  lineEnding: string;
  bom: boolean;
  mixedLineEndings: boolean;
  /** Editor text at the last open/save, to tell whether there are unsaved edits. */
  savedContent: string;
}

interface Deps {
  /** Current editor text. Called at action time, so it is never stale. */
  content: string;
  /** Replace the editor contents with a freshly opened document. */
  setContent: (text: string) => void;
  /** Show a modal and resolve with the chosen button id. */
  confirm: (spec: DialogSpec) => Promise<string>;
  /** Text of the untouched starting document, which counts as saved. */
  initialContent: string;
  /** When to save automatically. Only ever applies to a file that already has a path. */
  autosave?: { mode: string; delaySec: number };
}

const NOTICE_MS = 5000;

const untitled = (savedContent: string): DocState => ({
  path: null,
  name: UNTITLED,
  version: null,
  lineEnding: "lf",
  bom: false,
  mixedLineEndings: false,
  savedContent,
});

const fromDocument = (d: Document): DocState => ({
  path: d.path,
  name: baseName(d.path),
  version: d.version,
  lineEnding: d.lineEnding,
  bom: d.bom,
  mixedLineEndings: d.mixedLineEndings,
  savedContent: d.content,
});

const message = (e: unknown) => (e instanceof Error ? e.message : String(e));

/**
 * Owns the open document: which file it is, whether it has unsaved edits, and
 * the open / save / save-as / reload flows. The rule it enforces throughout is
 * that unsaved editor content is never discarded or overwritten without an
 * explicit choice by the user.
 */
export function useDocument({ content, setContent, confirm, initialContent, autosave }: Deps) {
  const [doc, setDoc] = useState<DocState>(() => untitled(initialContent));
  const [banner, setBanner] = useState<Banner | null>(null);
  const [notice, setNotice] = useState("");

  const dirty = content !== doc.savedContent;

  // Event handlers and async flows read the latest values through refs.
  const live = useRef({ doc, content, dirty, banner });
  live.current = { doc, content, dirty, banner };

  // Manual and automatic saves must not interleave: two writes finishing out
  // of order could leave the file holding older text than the editor thinks.
  const writeQueue = useRef<Promise<unknown>>(Promise.resolve());
  const exclusive = useCallback(<T,>(fn: () => Promise<T>): Promise<T> => {
    const run = writeQueue.current.then(fn);
    writeQueue.current = run.catch(() => undefined);
    return run;
  }, []);
  // The text an automatic save last failed or conflicted on. It is not retried
  // until the text changes or the user resolves the problem.
  const autosaveBlockedFor = useRef<string | null>(null);
  const autosaveMode = useRef(autosave?.mode ?? "off");
  autosaveMode.current = autosave?.mode ?? "off";
  const noticeTimer = useRef<ReturnType<typeof setTimeout> | undefined>(undefined);

  const flash = useCallback((text: string) => {
    setNotice(text);
    clearTimeout(noticeTimer.current);
    noticeTimer.current = setTimeout(() => setNotice(""), NOTICE_MS);
  }, []);

  const fail = useCallback((what: string, e: unknown) => setBanner({ kind: "error", message: `${what}: ${message(e)}` }), []);

  const load = useCallback(
    (d: Document) => {
      setContent(d.content);
      setDoc(fromDocument(d));
      setBanner(null);
      autosaveBlockedFor.current = null;
    },
    [setContent],
  );

  const options = (d: DocState, force: boolean): SaveOptions => ({
    base: force ? null : d.version,
    force,
    lineEnding: d.lineEnding,
    bom: d.bom,
  });

  const saveAs = useCallback(async (): Promise<boolean> => {
    const { doc: d, content: text } = live.current;
    try {
      const res = await SaveAs(dirOf(d.path), d.name, text, options(d, true));
      if (!res || res.cancelled) return false;
      setDoc({ ...d, path: res.path, name: res.name, version: res.version, savedContent: text });
      setBanner(null);
      flash(`Saved ${res.name}`);
      return true;
    } catch (e) {
      fail("Could not save", e);
      return false;
    }
  }, [fail, flash]);

  /** Re-read the file from disk, replacing the editor content. */
  const reload = useCallback(async (): Promise<boolean> => {
    const path = live.current.doc.path;
    if (!path) return false;
    try {
      const res = await OpenPath(path);
      if (!res?.document) return false;
      load(res.document);
      return true;
    } catch (e) {
      fail("Could not reload", e);
      return false;
    }
  }, [fail, load]);

  /**
   * The file changed or vanished since we opened it; let the user choose.
   * `overwrite` is passed in (rather than calling save directly) so that save
   * and this function do not depend on each other.
   */
  const resolveConflict = useCallback(
    async (kind: string, d: DocState, overwrite: () => Promise<boolean>): Promise<boolean> => {
      if (kind === "deleted") {
        const choice = await confirm({
          title: `“${d.name}” no longer exists`,
          message:
            "It was deleted or moved outside D2 Studio. You can recreate it with your current content, or save a copy elsewhere.",
          cancelId: "cancel",
          buttons: [
            { id: "cancel", label: "Cancel" },
            { id: "saveas", label: "Save As…" },
            { id: "recreate", label: "Recreate file", kind: "primary" },
          ],
        });
        if (choice === "recreate") return overwrite();
        if (choice === "saveas") return saveAs();
        return false;
      }
      const choice = await confirm({
        title: `“${d.name}” changed on disk`,
        message: "Another program modified this file after you opened it. Saving now would overwrite those changes.",
        cancelId: "cancel",
        buttons: [
          { id: "cancel", label: "Cancel", kind: "primary" },
          { id: "saveas", label: "Save As…" },
          { id: "reload", label: "Reload from disk" },
          { id: "overwrite", label: "Overwrite", kind: "danger" },
        ],
      });
      if (choice === "overwrite") return overwrite();
      if (choice === "saveas") return saveAs();
      if (choice === "reload") return reload();
      return false;
    },
    [confirm, reload, saveAs],
  );

  const save = useCallback(
    async (force = false): Promise<boolean> => {
      const { doc: d, content: text } = live.current;
      if (!d.path) return saveAs();
      try {
        const res = await exclusive(() => Save(d.path!, text, options(d, force)));
        if (!res) return false;
        if (res.saved) {
          autosaveBlockedFor.current = null;
          setDoc((cur) => ({ ...cur, version: res.version, savedContent: text }));
          setBanner(null);
          flash(`Saved ${d.name}`);
          return true;
        }
        return await resolveConflict(res.conflict, d, () => save(true));
      } catch (e) {
        fail("Could not save", e);
        return false;
      }
    },
    [exclusive, fail, flash, resolveConflict, saveAs],
  );

  /**
   * Saves the open file without asking, if that is safe. Returns the text it
   * saved, or null when it did nothing. It never forces: if the file changed
   * or vanished on disk it shows the usual banner and stops, so it can never
   * overwrite someone else's changes.
   */
  const autosaveNow = useCallback(async (): Promise<string | null> => {
    const { doc: d, content: text, dirty: isDirty, banner: b } = live.current;
    if (!d.path || !isDirty) return null; // an unsaved document has nowhere to go
    if (b && b.kind !== "error") return null; // an unresolved external change
    if (autosaveBlockedFor.current === text) return null;
    try {
      const res = await exclusive(() => Save(d.path!, text, options(d, false)));
      if (!res) return null;
      if (res.saved) {
        setDoc((cur) => ({ ...cur, version: res.version, savedContent: text }));
        setBanner((cur) => (cur?.kind === "error" ? null : cur));
        flash(`Autosaved ${d.name}`);
        return text;
      }
      autosaveBlockedFor.current = text;
      setBanner(res.conflict === "deleted" ? { kind: "deleted" } : { kind: "modified", version: res.version });
      return null;
    } catch (e) {
      autosaveBlockedFor.current = text;
      fail("Autosave failed", e);
      return null;
    }
  }, [exclusive, fail, flash]);

  /** True when it is safe to replace the current document. */
  const guard = useCallback(async (): Promise<boolean> => {
    const { doc: d, dirty: isDirty } = live.current;
    if (!isDirty) return true;
    // With autosave on, save quietly first. If that works and nothing was
    // typed meanwhile there is nothing to ask; otherwise fall back to asking.
    if (autosaveMode.current !== "off") {
      const saved = await autosaveNow();
      if (saved !== null && live.current.content === saved) return true;
    }
    const choice = await confirm({
      title: `Save changes to “${d.name}”?`,
      message: "Your changes will be lost if you don't save them.",
      cancelId: "cancel",
      buttons: [
        { id: "cancel", label: "Cancel" },
        { id: "discard", label: "Don't save", kind: "danger" },
        { id: "save", label: "Save", kind: "primary" },
      ],
    });
    if (choice === "save") return save();
    return choice === "discard";
  }, [autosaveNow, confirm, save]);

  const newDocument = useCallback(async () => {
    if (!(await guard())) return;
    await Unwatch();
    setContent("");
    setDoc(untitled(""));
    setBanner(null);
  }, [guard, setContent]);

  const open = useCallback(async () => {
    if (!(await guard())) return;
    try {
      const res = await OpenDialog(dirOf(live.current.doc.path));
      if (res?.document) load(res.document);
    } catch (e) {
      fail("Could not open file", e);
    }
  }, [fail, guard, load]);

  const openPath = useCallback(
    async (path: string) => {
      if (!(await guard())) return;
      try {
        const res = await OpenPath(path);
        if (res?.document) load(res.document);
      } catch (e) {
        fail("Could not open file", e);
        // A recent file that no longer exists should not linger in the list.
        RemoveRecent(path).catch(() => {});
      }
    },
    [fail, guard, load],
  );

  const keepMine = useCallback((version: Version) => {
    // Accept the on-disk version as the base so the next Save is an explicit
    // choice to overwrite it, not a conflict.
    setDoc((cur) => ({ ...cur, version }));
    setBanner(null);
    autosaveBlockedFor.current = null;
  }, []);

  // External changes to the open file.
  useEffect(() => {
    return Events.On("file:changed", (ev) => {
      const e = ev.data;
      const { doc: d, dirty: isDirty } = live.current;
      if (!e || e.path !== d.path) return;
      const action = decideExternal(e, isDirty);
      switch (action.type) {
        case "reload":
          reload().then((ok) => ok && flash(`Reloaded “${d.name}”: it changed on disk`));
          break;
        case "prompt-modified":
          setBanner({ kind: "modified", version: e.version });
          break;
        case "prompt-deleted":
          setBanner({ kind: "deleted" });
          break;
        case "prompt-unavailable":
          setBanner({ kind: "unavailable", message: action.message });
          break;
        case "clear":
          setBanner((b) => (b && b.kind !== "error" ? null : b));
          break;
      }
    });
  }, [flash, reload]);

  // Closing the window or quitting: the shell cancels it and asks us, and the
  // menu's Exit item asks the same way.
  const closing = useRef(false);
  const requestQuit = useCallback(async () => {
    if (closing.current) return;
    closing.current = true;
    try {
      if (await guard()) await ConfirmQuit();
    } finally {
      closing.current = false;
    }
  }, [guard]);
  useEffect(() => {
    return Events.On("app:close-requested", () => void requestQuit());
  }, [requestQuit]);

  // Autosave after a pause in typing. The timer restarts on every edit, so it
  // never fires mid-sentence.
  const delaySec = autosave?.delaySec ?? 2;
  const autosaveIsDelay = autosave?.mode === "delay";
  useEffect(() => {
    if (!autosaveIsDelay || !dirty || !doc.path) return;
    const timer = setTimeout(() => void autosaveNow(), delaySec * 1000);
    return () => clearTimeout(timer);
  }, [autosaveIsDelay, delaySec, dirty, doc.path, content, banner, autosaveNow]);

  // Autosave when the window loses focus or is hidden.
  const autosaveIsBlur = autosave?.mode === "blur";
  useEffect(() => {
    if (!autosaveIsBlur) return;
    const run = () => void autosaveNow();
    const onVisibility = () => document.visibilityState === "hidden" && run();
    window.addEventListener("blur", run);
    document.addEventListener("visibilitychange", onVisibility);
    return () => {
      window.removeEventListener("blur", run);
      document.removeEventListener("visibilitychange", onVisibility);
    };
  }, [autosaveIsBlur, autosaveNow]);

  // Window title shows the file and an unsaved-changes dot.
  useEffect(() => {
    Window.SetTitle(windowTitle(doc.name, dirty)).catch(() => {
      /* not running inside Wails (e.g. a plain browser) */
    });
  }, [doc.name, dirty]);

  return {
    name: doc.name,
    path: doc.path,
    dirty,
    banner,
    notice,
    encoding: encodingLabel(doc),
    dismissBanner: () => setBanner(null),
    /** Quit, after asking about unsaved changes if needed. */
    quit: requestQuit,
    /** Show a transient message in the status bar. */
    notify: flash,
    /** Show a non-blocking error banner. */
    showError: (text: string) => setBanner({ kind: "error", message: text }),
    keepMine,
    newDocument,
    open,
    openPath,
    save: () => save(false),
    saveAs,
    /** Reload from disk without asking: the caller has already chosen to discard edits. */
    reload,
  };
}
