import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { useState } from "react";
import { describe, expect, it, vi } from "vitest";
import type { FileStatusRow, ProcessingStatus } from "../types";
import { RetryDrawer } from "./RetryDrawer";

vi.mock("../hooks/useEodQueries", () => ({
  useFileHistory: () => ({
    data: { file_id: "1", filename: "a.xlsx", file_type: "dmaa", attempts: [] },
    isLoading: false,
    isError: false,
    error: null,
    refetch: vi.fn(),
  }),
  getErrorMessage: () => "",
}));

function fileWith(status: ProcessingStatus): FileStatusRow {
  return {
    file_id: "1",
    file_type: "dmaa",
    filename: "a.xlsx",
    checksum: "a".repeat(64),
    processing_status: status,
    retry_count: 0,
    max_retries_exhausted: false,
    detected_at: "2099-01-01T00:00:00Z",
    last_retry_at: null,
    failure_reason: null,
  };
}

function renderDrawer(status: ProcessingStatus) {
  return render(<RetryDrawer file={fileWith(status)} onClose={vi.fn()} onRetryClick={vi.fn()} />);
}

describe("RetryDrawer", () => {
  it("shows superseded as 'Digantikan' with an icon and offers no retry", () => {
    renderDrawer("superseded");

    const badge = screen.getByText("Digantikan");
    expect(badge.parentElement?.querySelector("svg")).toBeTruthy();
    expect(screen.queryByRole("button", { name: "Retry Manual" })).toBeNull();
  });

  it.each(["failed", "max_retries_exhausted"] as const)("offers retry for %s", (status) => {
    renderDrawer(status);

    const button = screen.getByRole("button", { name: "Retry Manual" }) as HTMLButtonElement;
    expect(button.disabled).toBe(false);
  });

  it.each(["completed", "pending"] as const)("offers no retry for %s", (status) => {
    renderDrawer(status);

    expect(screen.queryByRole("button", { name: "Retry Manual" })).toBeNull();
  });

  it("is a labelled modal dialog", () => {
    renderDrawer("failed");

    const dialog = screen.getByRole("dialog", { name: "Riwayat Retry" });
    expect(dialog).toHaveAttribute("aria-modal", "true");
  });

  it("moves focus to the close button on open and closes on Escape", async () => {
    const onClose = vi.fn();
    render(<RetryDrawer file={fileWith("failed")} onClose={onClose} onRetryClick={vi.fn()} />);

    await waitFor(() => expect(screen.getByRole("button", { name: "Tutup" })).toHaveFocus());

    fireEvent.keyDown(document, { key: "Escape" });
    expect(onClose).toHaveBeenCalledTimes(1);
  });

  it("restores focus to the trigger when the drawer closes", async () => {
    function Harness() {
      const [file, setFile] = useState<FileStatusRow | null>(null);
      return (
        <>
          <button type="button" onClick={() => setFile(fileWith("failed"))}>
            Open
          </button>
          <RetryDrawer file={file} onClose={() => setFile(null)} onRetryClick={vi.fn()} />
        </>
      );
    }
    const user = userEvent.setup();
    render(<Harness />);

    const trigger = screen.getByRole("button", { name: "Open" });
    await user.click(trigger);
    await waitFor(() => expect(screen.getByRole("button", { name: "Tutup" })).toHaveFocus());

    await user.keyboard("{Escape}");
    expect(screen.queryByRole("dialog")).toBeNull();
    expect(trigger).toHaveFocus();
  });
});
