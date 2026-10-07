import { DsrUploadDialog } from "@/features/dsr/DsrUploadDialog";
import type { DsrConfirmResponse, DsrDryRunResponse } from "@/features/dsr/dsrUploadApi";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

const dryRunResponse: DsrDryRunResponse = {
  mode: "dry_run",
  filename: "BIJAK__7__laporan.xlsx",
  original_filename: "laporan.xlsx",
  checksum: "abc123",
  vendor_code: "BIJAK",
  vendor_name: "Bijak",
  uploaded_by_user_id: 7,
  staged_filename: "BIJAK__7__laporan.xlsx",
  daily: {
    rows: [
      {
        row_no: 1,
        section: "d0",
        flow: "saldo_awal",
        line_label: "SALDO AWAL",
        memo_no: null,
        denom_100k: "1000000",
        denom_50k: "500000",
        denom_20k: "0",
        denom_10k: "0",
        denom_5k: "0",
        denom_2k: "0",
        denom_1k: "0",
        line_total_idr: "1500000",
        remarks: null,
      },
    ],
    error_count: 0,
  },
  rencana_isi: {
    plan_date: "2026-07-16",
    rows: [
      {
        row_no: 1,
        atm_terminal_id: "1234",
        atm_location: "TEST LOCATION",
        denom_config: "100",
        fill_100k_idr: "400000",
        fill_50k_idr: "0",
        splank_balance_0800_idr: "350000",
        remarks: null,
      },
    ],
  },
};

const confirmResponse: DsrConfirmResponse = {
  checksum: "abc123",
  daily: { file_id: 1, status: "completed", row_count: 1, success_count: 1, error_count: 0, errors: [] },
  rencana_isi: { file_id: 2, status: "completed", row_count: 1, success_count: 1, error_count: 0, errors: [] },
};

function jsonResponse(body: unknown, status = 200): Response {
  return new Response(JSON.stringify(body), {
    status,
    headers: { "Content-Type": "application/json" },
  });
}

describe("DsrUploadDialog", () => {
  beforeEach(() => {
    vi.stubGlobal("fetch", vi.fn());
  });

  afterEach(() => {
    vi.unstubAllGlobals();
  });

  it("shows a preview after upload without confirming, then persists only after Confirm", async () => {
    const fetchMock = fetch as unknown as ReturnType<typeof vi.fn>;
    fetchMock
      .mockResolvedValueOnce(jsonResponse(dryRunResponse))
      .mockResolvedValueOnce(jsonResponse(confirmResponse));

    const user = userEvent.setup();
    const onConfirmed = vi.fn();
    render(<DsrUploadDialog onClose={vi.fn()} onConfirmed={onConfirmed} />);

    const file = new File(["fake xlsx bytes"], "laporan.xlsx", {
      type: "application/vnd.ms-excel",
    });
    const input = document.querySelector('input[type="file"]') as HTMLInputElement;
    await user.upload(input, file);

    // Preview renders the parsed rows -- nothing persisted yet (only the
    // dry-run endpoint has been called so far).
    expect(await screen.findByText("SALDO AWAL")).toBeInTheDocument();
    expect(screen.getByText("1234")).toBeInTheDocument();
    expect(fetchMock).toHaveBeenCalledTimes(1);
    expect(fetchMock.mock.calls[0][0]).toContain("/api/v1/dsr/uploads");

    // Only clicking Confirm triggers the actual persist call.
    await user.click(screen.getByRole("button", { name: /konfirmasi & simpan/i }));

    expect(await screen.findByText(/berhasil disimpan/i)).toBeInTheDocument();
    expect(fetchMock).toHaveBeenCalledTimes(2);
    expect(fetchMock.mock.calls[1][0]).toContain("/api/v1/dsr/uploads/confirm");
    expect(onConfirmed).toHaveBeenCalledWith(confirmResponse);
  });

  it("lets the vendor discard the preview and re-pick a file instead of submitting", async () => {
    const fetchMock = fetch as unknown as ReturnType<typeof vi.fn>;
    fetchMock.mockResolvedValueOnce(jsonResponse(dryRunResponse));

    const user = userEvent.setup();
    render(<DsrUploadDialog onClose={vi.fn()} />);

    const file = new File(["fake xlsx bytes"], "laporan.xlsx", {
      type: "application/vnd.ms-excel",
    });
    const input = document.querySelector('input[type="file"]') as HTMLInputElement;
    await user.upload(input, file);

    expect(await screen.findByText("SALDO AWAL")).toBeInTheDocument();

    await user.click(screen.getByRole("button", { name: /batal.*ganti file/i }));

    // Back to the file picker; confirm was never called.
    expect(screen.getByRole("button", { name: /pilih file/i })).toBeInTheDocument();
    expect(fetchMock).toHaveBeenCalledTimes(1);
  });
});
