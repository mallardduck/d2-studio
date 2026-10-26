// @vitest-environment jsdom
import { cleanup, fireEvent, render, screen, waitFor } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";
import type { Snippet } from "../../bindings/d2studio/snippets/models";

const h = vi.hoisted(() => ({ preview: vi.fn() }));
vi.mock("../../bindings/d2studio/snippetservice", () => ({ Preview: h.preview }));
vi.mock("./Preview", () => ({ Preview: (p: { svg: string; state: string }) => <div data-testid="preview">{p.state}:{p.svg}</div> }));
vi.mock("./ReadOnlyCode", () => ({ ReadOnlyCode: (p: { code: string; label: string }) => <pre aria-label={p.label}>{p.code}</pre> }));

import { SnippetTab } from "./SnippetTab";

afterEach(cleanup);
const sn: Snippet = { id: "labels", title: "Labels", category: "Basics", description: "Text after a colon", code: "a -> b\n", remote: false };

describe("SnippetTab", () => {
  it("shows the description, the read-only code and the rendered preview", async () => {
    h.preview.mockResolvedValue("<svg/>");
    render(<SnippetTab snippet={sn} onInsert={vi.fn()} />);
    expect(screen.getByText("Text after a colon")).toBeTruthy();
    expect(screen.getByLabelText("Labels source (read only)").textContent).toBe("a -> b\n");
    await waitFor(() => expect(screen.getByTestId("preview").textContent).toBe("success:<svg/>"));
    expect(h.preview).toHaveBeenCalledWith("labels");
  });

  it("inserts the code from the tab", () => {
    h.preview.mockResolvedValue("<svg/>");
    const onInsert = vi.fn();
    render(<SnippetTab snippet={sn} onInsert={onInsert} />);
    fireEvent.click(screen.getByRole("button", { name: "Insert into document" }));
    expect(onInsert).toHaveBeenCalledWith("a -> b\n");
  });

  it("explains a failed preview and warns about web images", async () => {
    h.preview.mockRejectedValue(new Error("nope"));
    render(<SnippetTab snippet={{ ...sn, remote: true }} onInsert={vi.fn()} />);
    await waitFor(() => expect(screen.getByRole("alert").textContent).toContain("nope"));
    expect(screen.getByText(/image from the web/)).toBeTruthy();
  });
});
