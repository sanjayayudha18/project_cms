import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vitest";
import type { ExportDialogProps } from "../hooks";
import type { ExportJob } from "../types";
import { ExportProgressDialog } from "./ExportProgressDialog";

const job = (over: Partial<ExportJob> = {}): ExportJob =>
  ({ id: 1, file_name: "vendor.csv", status: "queued", row_count: 0, ...over }) as ExportJob;

function setup(over: Partial<ExportDialogProps> = {}) {
  const props: ExportDialogProps = {
    open: true,
    phase: "waiting",
    job: job(),
    message: null,
    onClose: vi.fn(),
    onRetry: vi.fn(),
    ...over,
  };
  const utils = render(<ExportProgressDialog {...props} />);
  return { props, ...utils };
}

const closeButtons = () => screen.getAllByRole("button", { name: "Tutup" });
const NOTE = /Jika ditutup, ekspor tetap berjalan/;

describe("ExportProgressDialog", () => {
  it("shows Antri when queued", () => {
    setup();
    expect(screen.getByText("Antri")).toBeInTheDocument();
    expect(screen.getByText(NOTE)).toBeInTheDocument();
  });

  it("shows Membuat file with row count while running", () => {
    setup({ job: job({ status: "running", row_count: 1234 }) });
    expect(screen.getByText("Membuat file")).toBeInTheDocument();
    expect(screen.getByText(/1\.234 baris/)).toHaveClass("tabular-nums");
  });

  it("hides row count when zero", () => {
    setup({ job: job({ status: "running" }) });
    expect(screen.queryByText(/baris/)).not.toBeInTheDocument();
  });

  it("shows Mengunduh with the keep-running note", () => {
    setup({ phase: "downloading", job: job({ status: "downloading" }) });
    expect(screen.getByText("Mengunduh")).toBeInTheDocument();
    expect(screen.getByText(NOTE)).toBeInTheDocument();
  });

  it("shows Selesai with file name and rows, without the note", () => {
    setup({ phase: "done", job: job({ status: "downloaded", row_count: 42 }) });
    expect(screen.getByText("Selesai")).toBeInTheDocument();
    expect(screen.getByText("vendor.csv")).toBeInTheDocument();
    expect(screen.getByText(/42 baris/)).toBeInTheDocument();
    expect(screen.queryByText(NOTE)).not.toBeInTheDocument();
  });

  it("failed: alert with message, Coba lagi calls onRetry, no note", async () => {
    const { props } = setup({ phase: "failed", message: "Query gagal" });
    expect(screen.getByRole("alert")).toHaveTextContent("Ekspor gagal");
    expect(screen.getByText("Query gagal")).toBeInTheDocument();
    expect(screen.queryByText(NOTE)).not.toBeInTheDocument();
    await userEvent.click(screen.getByRole("button", { name: "Coba lagi" }));
    expect(props.onRetry).toHaveBeenCalledTimes(1);
  });

  it("download_failed: alert, message and Ekspor Saya hint, no retry", () => {
    setup({ phase: "download_failed", message: "Jaringan putus." });
    expect(screen.getByRole("alert")).toBeInTheDocument();
    expect(screen.getByText("Jaringan putus.")).toBeInTheDocument();
    expect(screen.getByText(/Ekspor Saya/)).toBeInTheDocument();
    expect(screen.queryByRole("button", { name: "Coba lagi" })).not.toBeInTheDocument();
  });

  it("Tutup is present in every phase and calls onClose", async () => {
    for (const phase of ["waiting", "downloading", "done", "failed", "download_failed"] as const) {
      const { props, unmount } = setup({ phase });
      expect(closeButtons().length).toBeGreaterThan(0);
      await userEvent.click(closeButtons().at(-1) as HTMLElement);
      expect(props.onClose).toHaveBeenCalled();
      unmount();
    }
  });

  it("Escape closes", async () => {
    const { props } = setup();
    await userEvent.keyboard("{Escape}");
    expect(props.onClose).toHaveBeenCalled();
  });

  it("live region text follows phase changes", () => {
    const { props, rerender } = setup();
    const region = () => document.querySelector("p[aria-live='polite']");
    expect(region()).toHaveTextContent("Antri");
    rerender(<ExportProgressDialog {...props} job={job({ status: "running" })} />);
    expect(region()).toHaveTextContent("Membuat file");
    rerender(<ExportProgressDialog {...props} phase="downloading" />);
    expect(region()).toHaveTextContent("Mengunduh");
  });

  it("spinner respects reduced motion", () => {
    setup({ job: job({ status: "running" }) });
    const spinner = document.querySelector("svg.animate-spin");
    expect(spinner).toHaveClass("motion-reduce:animate-none");
  });

  it("renders nothing when closed", () => {
    setup({ open: false, phase: "idle" });
    expect(screen.queryByRole("dialog")).not.toBeInTheDocument();
  });
});
