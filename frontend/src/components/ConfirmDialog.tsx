import { useEffect, useRef } from "react";

export interface DialogButton {
  id: string;
  label: string;
  /** "primary" is focused first; "danger" marks a destructive choice. */
  kind?: "primary" | "danger" | "normal";
}

export interface DialogSpec {
  title: string;
  message: string;
  buttons: DialogButton[];
  /** Returned when the user presses Escape. */
  cancelId: string;
}

interface Props {
  spec: DialogSpec | null;
  onChoose: (id: string) => void;
}

/**
 * Modal confirmation built on the native <dialog>, which traps focus, labels
 * itself for screen readers and closes on Escape. Reserved for choices that
 * can lose work; everything else uses non-blocking banners.
 */
export function ConfirmDialog({ spec, onChoose }: Props) {
  const ref = useRef<HTMLDialogElement>(null);

  useEffect(() => {
    const d = ref.current;
    if (!d) return;
    if (spec && !d.open) d.showModal();
    if (!spec && d.open) d.close();
  }, [spec]);

  if (!spec) return <dialog ref={ref} />;

  return (
    <dialog
      ref={ref}
      aria-labelledby="confirm-title"
      aria-describedby="confirm-message"
      onCancel={(e) => {
        e.preventDefault(); // Escape: choose the cancel option instead of silently closing
        onChoose(spec.cancelId);
      }}
      className="m-auto w-[min(28rem,90vw)] rounded-lg border border-line bg-surface p-5 text-fg shadow-xl backdrop:bg-black/40"
    >
      <h2 id="confirm-title" className="text-base font-semibold">
        {spec.title}
      </h2>
      <p id="confirm-message" className="mt-2 text-fg-muted">
        {spec.message}
      </p>
      <div className="mt-5 flex flex-wrap justify-end gap-2">
        {spec.buttons.map((b) => (
          <button
            key={b.id}
            type="button"
            autoFocus={b.kind === "primary"}
            onClick={() => onChoose(b.id)}
            className={`rounded-md border px-3 py-1.5 ${
              b.kind === "primary"
                ? "border-accent bg-accent font-medium text-surface"
                : b.kind === "danger"
                  ? "border-err text-err hover:bg-err/10"
                  : "border-line hover:bg-line/40"
            }`}
          >
            {b.label}
          </button>
        ))}
      </div>
    </dialog>
  );
}
