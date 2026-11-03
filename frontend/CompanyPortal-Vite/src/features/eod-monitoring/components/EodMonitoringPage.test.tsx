/**
 * fe-coverage C5: EOD Monitoring page with its real sections and retry confirmation.
 * The query hooks are mocked per section; RetryDrawer is stubbed (own test).
 */

import { useToastStore } from "@/lib/hooks/useToast";
import { fireEvent, render, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { beforeEach, describe, expect, it, vi } from "vitest";

type Q = {
  isLoading?: boolean;
  isError?: boolean;
  error?: unknown;
  data?: unknown;
  dataUpdatedAt?: number;
};

const m = vi.hoisted(() => ({
  summary: {} as Q,
  status: {} as Q,
  late: {} as Q,
  audit: {} as Q,
  refetch: vi.fn(),
  retry: { mutate: vi.fn(), isPending: false },
  calls: { summary: [] as unknown[][], audit: [] as unknown[][] },
}));

vi.mock("../hooks/useEodQueries", async (importOriginal) => {
  const actual = await importOriginal<typeof import("../hooks/useEodQueries")>();
  return {
    ...actual,
    useEodSummary: (...a: unknown[]) => {
      m.calls.summary.push(a);
      return { ...m.summary, refetch: m.refetch };
    },
    useEodStatus: () => ({ ...m.status, refetch: m.refetch }),
    useEodLate: () => ({ ...m.late, refetch: m.refetch }),
    useEodAudit: (...a: unknown[]) => {
      m.calls.audit.push(a);
      return { ...m.audit, refetch: m.refetch };
    },
    useRetryFile: () => m.retry,
  };
});
vi.mock("./RetryDrawer", () => ({
  RetryDrawer: ({
    file,
    onRetryClick,
  }: {
    file: { filename: string } | null;
    onRetryClick: (f: unknown) => void;
  }) =>
    file ? (
      <aside aria-label="drawer">
        {file.filename}
        <button type="button" onClick={() => onRetryClick(file)}>
          Retry Manual
        </button>
      </aside>
    ) : null,
}));

import { EodMonitoringPage } from "./EodMonitoringPage";

const FILE = {
  file_id: "f-1",
  filename: "Order_All_20261015.xlsx",
  checksum: "abc",
  processing_status: "failed",
  retry_count: 3,
  max_retries_exhausted: true,
  detected_at: "2026-10-15T00:05:00Z",
  last_retry_at: null,
  failure_reason: "Sheet tidak ditemukan",
};

beforeEach(() => {
  m.refetch.mockReset();
  m.retry = { mutate: vi.fn(), isPending: false };
  m.calls = { summary: [], audit: [] };
  m.summary = {
    isLoading: false,
    isError: false,
    dataUpdatedAt: Date.parse("2026-10-15T01:00:00Z"),
    data: {
      counts: {
        pending: 1,
        processing: 2,
        completed: 7,
        failed: 1,
        max_retries_exhausted: 1,
        late: 1,
      },
    },
  };
  m.status = {
    isLoading: false,
    isError: false,
    data: {
      processing_date: "2026-10-15",
      by_file_type: {
        dmaa: [FILE],
        itm_cashpos: [
          {
            ...FILE,
            file_id: "f-2",
            filename: "cashpos.csv",
            processing_status: "completed",
            retry_count: 0,
            last_retry_at: "2026-10-15T01:30:00Z",
            failure_reason: null,
          },
        ],
        itm_replenish: [],
      },
    },
  };
  m.late = {
    isLoading: false,
    isError: false,
    data: [
      {
        id: "l1",
        file_type: "dmaa",
        sla_deadline: "06:00:00",
        detected_at: "2026-10-14T23:30:00Z",
        is_resolved: false,
      },
      {
        id: "l2",
        file_type: "itm_cashpos",
        sla_deadline: "07:00:00",
        detected_at: "2026-10-15T00:10:00Z",
        is_resolved: true,
      },
    ],
  };
  m.audit = {
    isLoading: false,
    isError: false,
    data: [
      {
        id: "a1",
        event_type: "retry",
        trigger: "manual",
        file_type: "dmaa",
        initiated_by: "admin",
        outcome: "failed",
        duration_ms: 1234,
        error_detail: "boom",
        created_at: "2026-10-15T01:02:03Z",
      },
      {
        id: "a2",
        event_type: "scan",
        trigger: "auto",
        file_type: "itm_replenish",
        initiated_by: "scheduler",
        outcome: null,
        duration_ms: null,
        error_detail: null,
        created_at: "2026-10-15T01:00:00Z",
      },
    ],
  };
  useToastStore.setState({ toasts: [] });
});

describe("EodMonitoringPage", () => {
  it("shows the summary counts, file statuses, late alerts and the audit log", () => {
    render(<EodMonitoringPage />);

    const summary = within(screen.getByRole("region", { name: "Ringkasan" }));
    expect(summary.getByText("Completed")).toBeInTheDocument();
    expect(summary.getByText("7")).toBeInTheDocument();

    const files = within(screen.getByRole("region", { name: "Status File" }));
    expect(files.getByText("Order_All_20261015.xlsx")).toBeInTheDocument();
    expect(files.getByText("Sheet tidak ditemukan")).toBeInTheDocument();
    expect(files.getByText("15 Oct 2026 08:30")).toBeInTheDocument(); // last retry, WIB

    const late = within(screen.getByRole("region", { name: "Peringatan Keterlambatan" }));
    expect(late.getByText("SLA: 06:00 WIB")).toBeInTheDocument();
    expect(late.getByText("Resolved")).toBeInTheDocument();
    expect(late.getByText("Late")).toBeInTheDocument();

    const audit = within(screen.getByRole("region", { name: "Log Audit" }));
    expect(audit.getByText("15 Oct 2026 08:02:03")).toBeInTheDocument();
    expect(audit.getByText("Berlangsung")).toBeInTheDocument(); // outcome null
    expect(audit.getByText(/1\.234 ms/)).toBeInTheDocument();

    expect(screen.getByText(/Terakhir diperbarui: 15 Oct 2026 08:00/)).toBeInTheDocument();
  });

  it("polls every 60s until polling is switched off, and follows the chosen date", async () => {
    render(<EodMonitoringPage />);
    expect(m.calls.summary.at(-1)?.[1]).toBe(60_000);

    const toggle = screen.getByRole("switch");
    expect(toggle).toHaveTextContent("Polling Aktif");
    await userEvent.setup().click(toggle);
    expect(toggle).toHaveAttribute("aria-checked", "false");
    expect(m.calls.summary.at(-1)?.[1]).toBe(false);

    fireEvent.change(screen.getByLabelText("Tanggal Proses"), {
      target: { value: "2026-10-01" },
    });
    expect(m.calls.summary.at(-1)?.[0]).toBe("2026-10-01");
  });

  it("filters the audit log by file type and trigger", async () => {
    const user = userEvent.setup();
    render(<EodMonitoringPage />);

    await user.selectOptions(screen.getByLabelText("File Type"), "dmaa");
    await user.selectOptions(screen.getByLabelText("Trigger"), "manual");

    expect(m.calls.audit.at(-1)?.slice(1, 3)).toEqual(["dmaa", "manual"]);
  });

  it("each section shows loading, error-with-retry and empty states", async () => {
    m.summary = { isLoading: true };
    m.status = { isLoading: false, isError: true, error: new Error("Status gagal") };
    m.late = { isLoading: false, isError: false, data: [] };
    m.audit = { isLoading: false, isError: false, data: [] };
    const { rerender } = render(<EodMonitoringPage />);

    expect(screen.getByRole("region", { name: "Ringkasan" })).toHaveAttribute("aria-busy", "true");
    expect(screen.getByText("Status gagal")).toBeInTheDocument();
    expect(screen.getByText("Tidak ada file terlambat")).toBeInTheDocument();
    expect(screen.getByText("Tidak ada log audit")).toBeInTheDocument();
    expect(screen.getByText("Terakhir diperbarui: -")).toBeInTheDocument();
    await userEvent.setup().click(screen.getByRole("button", { name: "Coba Lagi" }));
    expect(m.refetch).toHaveBeenCalled();

    m.summary = { isLoading: false, isError: true, error: new Error("Ringkasan gagal") };
    m.status = { isLoading: true };
    m.late = { isLoading: false, isError: true, error: new Error("Late gagal") };
    m.audit = { isLoading: false, isError: true, error: new Error("Audit gagal") };
    rerender(<EodMonitoringPage />);
    expect(screen.getByText("Ringkasan gagal")).toBeInTheDocument();
    expect(screen.getByText("Late gagal")).toBeInTheDocument();
    expect(screen.getByText("Audit gagal")).toBeInTheDocument();

    m.status = { isLoading: false, isError: false, data: { by_file_type: { dmaa: [] } } };
    rerender(<EodMonitoringPage />);
    expect(screen.getByText("Tidak ada file untuk tanggal proses ini")).toBeInTheDocument();
  });

  it("manual retry: drawer → confirmation → retry → success toast", async () => {
    m.retry.mutate.mockImplementation((_id, opts: { onSuccess: () => void }) => opts.onSuccess());
    const user = userEvent.setup();
    render(<EodMonitoringPage />);

    await user.click(screen.getByRole("button", { name: "DMAA" }));
    await user.click(
      within(screen.getByRole("complementary", { name: "drawer" })).getByRole("button", {
        name: "Retry Manual",
      }),
    );

    const dialog = screen.getByRole("alertdialog", { name: "Konfirmasi Retry Manual" });
    expect(dialog).toHaveTextContent("Order_All_20261015.xlsx");
    expect(dialog).toHaveTextContent("Jumlah Retry: 3");
    await user.click(within(dialog).getByRole("button", { name: "Konfirmasi Retry" }));

    expect(m.retry.mutate).toHaveBeenCalledWith("f-1", expect.anything());
    expect(useToastStore.getState().toasts[0]).toMatchObject({
      type: "success",
      message: "Retry berhasil dipicu untuk Order_All_20261015.xlsx",
    });
    expect(screen.queryByRole("alertdialog")).not.toBeInTheDocument();
  });

  it("a failed retry shows the error; Escape and the backdrop close the dialog", async () => {
    m.retry.mutate.mockImplementation((_id, opts: { onError: (e: Error) => void }) =>
      opts.onError(new Error("Sudah diproses")),
    );
    const user = userEvent.setup();
    render(<EodMonitoringPage />);
    const openDialog = async () => {
      await user.click(screen.getByRole("button", { name: "DMAA" }));
      await user.click(screen.getByRole("button", { name: "Retry Manual" }));
    };

    await openDialog();
    await user.click(screen.getByRole("button", { name: "Konfirmasi Retry" }));
    expect(useToastStore.getState().toasts[0]).toMatchObject({
      type: "error",
      message: "Sudah diproses",
    });

    await openDialog();
    fireEvent.keyDown(document, { key: "Escape" });
    await waitFor(() => expect(screen.queryByRole("alertdialog")).not.toBeInTheDocument());

    await openDialog();
    const [backdrop] = screen.getAllByRole("button", { name: "Batal" });
    await user.click(backdrop as HTMLElement);
    expect(screen.queryByRole("alertdialog")).not.toBeInTheDocument();
  });

  it("while a retry runs, the dialog cannot be dismissed", async () => {
    m.retry = { mutate: vi.fn(), isPending: true };
    const user = userEvent.setup();
    render(<EodMonitoringPage />);

    await user.click(screen.getByRole("button", { name: "DMAA" }));
    await user.click(screen.getByRole("button", { name: "Retry Manual" }));
    fireEvent.keyDown(document, { key: "Escape" });
    const [backdrop] = screen.getAllByRole("button", { name: "Batal" });
    await user.click(backdrop as HTMLElement);

    expect(screen.getByRole("alertdialog")).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Konfirmasi Retry" })).toBeDisabled();
  });
});
