import { useToastStore } from "@/lib/hooks/useToast";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { beforeEach, describe, expect, it, vi } from "vitest";
import type { MasterDataDetail } from "../api";
import { MasterDataReviewDialog } from "../components/MasterDataReviewDialog";

const useDetailMock = vi.fn();
const approveMutate = vi.fn();
const rejectMutate = vi.fn();

vi.mock("../hooks", () => ({
  useMasterDataDetail: (...args: unknown[]) => useDetailMock(...args),
  useApprove: () => ({ mutate: approveMutate, isPending: false }),
  useReject: () => ({ mutate: rejectMutate, isPending: false }),
}));

const SINGLE: MasterDataDetail = {
  request_id: 7,
  status: "pending",
  batch_id: null,
  total: 1,
  counts: { update: 1 },
  truncated: false,
  changes: [
    {
      id: 40,
      entity_type: "vendor",
      entity_id: 3,
      op: "update",
      payload: { name: "Vendor Baru", contact_email: "a@v.id" },
      before: { name: "Vendor Lama", contact_email: "a@v.id" },
      status: "pending",
      maker_id: 5,
    },
  ],
};

const BATCH: MasterDataDetail = {
  request_id: 8,
  status: "pending",
  batch_id: 2,
  total: 250,
  counts: { create: 240, disable: 10 },
  truncated: true,
  changes: [
    {
      id: 1,
      entity_type: "vendor",
      entity_id: null,
      op: "create",
      payload: { code: "V-NEW", name: "Baru" },
      before: null,
      status: "pending",
      maker_id: 5,
    },
    {
      id: 2,
      entity_type: "atm",
      entity_id: 9,
      op: "disable",
      payload: null,
      before: { terminal_id: "T009" },
      status: "pending",
      maker_id: 5,
    },
  ],
};

function mockDetail(data: MasterDataDetail | undefined, extra: Record<string, unknown> = {}) {
  useDetailMock.mockReturnValue({ data, isLoading: false, isError: false, ...extra });
}

beforeEach(() => {
  useDetailMock.mockReset();
  approveMutate.mockReset();
  rejectMutate.mockReset();
  useToastStore.setState({ toasts: [] });
});

describe("MasterDataReviewDialog", () => {
  it("single change: shows the before/after diff with a text 'Berubah' label on changed fields only", () => {
    mockDetail(SINGLE);
    render(<MasterDataReviewDialog requestId={7} onClose={vi.fn()} />);

    expect(screen.getByText(/Vendor · Vendor Baru/)).toBeInTheDocument();
    expect(screen.getByText("Vendor Lama")).toBeInTheDocument();
    expect(screen.getAllByText("Berubah")).toHaveLength(1); // email is unchanged
  });

  it("batch: shows per-operation counts, the rows, and the truncation notice", () => {
    mockDetail(BATCH);
    render(<MasterDataReviewDialog requestId={8} onClose={vi.fn()} />);

    // No `batch` header in the fixture: the dialog still renders (task 13.1 tolerance).
    expect(screen.getByText("Impor CSV")).toBeInTheDocument();
    expect(screen.getByText("240")).toBeInTheDocument();
    expect(screen.getByText("V-NEW")).toBeInTheDocument();
    expect(screen.getByText("T009")).toBeInTheDocument();
    expect(screen.getByText("Menampilkan 1-2 dari 250 perubahan")).toBeInTheDocument();
    expect(
      screen.getByText(/jika satu baris gagal, tidak ada yang diterapkan/),
    ).toBeInTheDocument();
  });

  it("Setujui / Tolak call the matching mutation with the request id and close on success", async () => {
    mockDetail(SINGLE);
    const onClose = vi.fn();
    approveMutate.mockImplementation((_id, { onSuccess }) => onSuccess());
    const user = userEvent.setup();
    render(<MasterDataReviewDialog requestId={7} onClose={onClose} />);

    await user.click(screen.getByRole("button", { name: "Setujui" }));

    expect(approveMutate).toHaveBeenCalledWith(7, expect.anything());
    expect(onClose).toHaveBeenCalled();
    expect(useToastStore.getState().toasts[0]).toMatchObject({ type: "success" });

    rejectMutate.mockImplementation((_id, { onSuccess }) => onSuccess());
    await user.click(screen.getByRole("button", { name: "Tolak" }));
    expect(rejectMutate).toHaveBeenCalledWith(7, expect.anything());
  });

  it("an apply-time failure (409) is shown as an error toast and the dialog stays open", async () => {
    mockDetail(SINGLE);
    const onClose = vi.fn();
    approveMutate.mockImplementation((_id, { onError }) =>
      onError({ message: "vendor code already exists" }),
    );
    const user = userEvent.setup();
    render(<MasterDataReviewDialog requestId={7} onClose={onClose} />);

    await user.click(screen.getByRole("button", { name: "Setujui" }));

    expect(onClose).not.toHaveBeenCalled();
    expect(useToastStore.getState().toasts[0]).toMatchObject({
      type: "error",
      message: "vendor code already exists",
    });
  });

  describe("batch review", () => {
    const rowOf = (id: number, code: string): MasterDataDetail["changes"][number] => ({
      id,
      entity_type: "vendor",
      entity_id: null,
      op: "create",
      payload: { code, name: `Nama ${code}` },
      before: null,
      status: "pending",
      maker_id: 5,
    });
    const PAGES: Record<number, MasterDataDetail["changes"]> = {
      1: [rowOf(1, "V-P1A"), rowOf(2, "V-P1B")],
      2: [rowOf(51, "V-P2A"), rowOf(52, "V-P2B")],
      5: [rowOf(201, "V-P5A")],
    };
    const PAGED: MasterDataDetail = {
      ...BATCH,
      batch: {
        id: 2,
        entity: "vendor",
        file_name: "vendors.csv",
        maker_name: "Budi",
        row_count: 250,
      },
      truncated: false,
      changes: PAGES[1],
    };

    function mockPaged(batch: MasterDataDetail["batch"] = PAGED.batch) {
      useDetailMock.mockImplementation((_id: number | null, page: number) => ({
        data: { ...PAGED, batch, page, changes: PAGES[page] ?? [] },
        isLoading: false,
        isError: false,
      }));
    }

    it("shows the batch header: entity, file name, maker, counts and the all-or-nothing note", () => {
      mockPaged();
      render(<MasterDataReviewDialog requestId={8} onClose={vi.fn()} />);

      expect(screen.getByText("Impor CSV · Vendor · vendors.csv")).toBeInTheDocument();
      expect(screen.getByText(/Diajukan oleh Budi/)).toBeInTheDocument();
      expect(screen.getByText("240")).toBeInTheDocument();
      expect(screen.getByText("10")).toBeInTheDocument();
      expect(
        screen.getByText(/jika satu baris gagal, tidak ada yang diterapkan/),
      ).toBeInTheDocument();
    });

    it("tolerates a null file_name (pre-036 batch)", () => {
      mockPaged({ id: 2, entity: "vendor", file_name: null, maker_name: "Budi", row_count: 250 });
      render(<MasterDataReviewDialog requestId={8} onClose={vi.fn()} />);

      expect(screen.getByText("Impor CSV · Vendor")).toBeInTheDocument();
    });

    it("paging calls the detail query with the right page and swaps the rows", async () => {
      mockPaged();
      const user = userEvent.setup();
      render(<MasterDataReviewDialog requestId={8} onClose={vi.fn()} />);

      expect(useDetailMock).toHaveBeenLastCalledWith(8, 1);
      expect(screen.getByText("V-P1A")).toBeInTheDocument();
      expect(screen.getByText("Menampilkan 1-2 dari 250 perubahan")).toBeInTheDocument();

      await user.click(screen.getByRole("button", { name: "Berikutnya" }));

      expect(useDetailMock).toHaveBeenLastCalledWith(8, 2);
      expect(screen.getByText("V-P2A")).toBeInTheDocument();
      expect(screen.queryByText("V-P1A")).not.toBeInTheDocument();
      expect(screen.getByText("Menampilkan 51-52 dari 250 perubahan")).toBeInTheDocument();

      await user.click(screen.getByRole("button", { name: "Sebelumnya" }));
      expect(useDetailMock).toHaveBeenLastCalledWith(8, 1);
    });

    it("disables Sebelumnya on the first page and Berikutnya on the last", async () => {
      mockPaged();
      const user = userEvent.setup();
      render(<MasterDataReviewDialog requestId={8} onClose={vi.fn()} />);

      expect(screen.getByRole("button", { name: "Sebelumnya" })).toBeDisabled();
      expect(screen.getByRole("button", { name: "Berikutnya" })).toBeEnabled();

      await user.click(screen.getByRole("button", { name: "Halaman 5 dari 5" }));

      expect(useDetailMock).toHaveBeenLastCalledWith(8, 5);
      expect(screen.getByRole("button", { name: "Berikutnya" })).toBeDisabled();
      expect(screen.getByRole("button", { name: "Sebelumnya" })).toBeEnabled();
    });

    it("disclosure toggles aria-expanded and shows the diff for that row only", async () => {
      mockPaged();
      const user = userEvent.setup();
      render(<MasterDataReviewDialog requestId={8} onClose={vi.fn()} />);

      const [first, second] = screen.getAllByRole("button", { name: /Lihat perubahan/ });
      expect(first).toHaveAttribute("aria-expanded", "false");
      expect(screen.queryByRole("columnheader", { name: "Bidang" })).not.toBeInTheDocument();

      await user.click(first);

      expect(first).toHaveAttribute("aria-expanded", "true");
      expect(second).toHaveAttribute("aria-expanded", "false");
      expect(screen.getAllByRole("columnheader", { name: "Bidang" })).toHaveLength(1);
      expect(screen.getByText("Nama V-P1A")).toBeInTheDocument();
      expect(screen.queryByText("Nama V-P1B")).not.toBeInTheDocument();

      await user.click(first);
      expect(first).toHaveAttribute("aria-expanded", "false");
      expect(screen.queryByRole("columnheader", { name: "Bidang" })).not.toBeInTheDocument();
    });

    it("an open row does not open the rows of another page", async () => {
      mockPaged();
      const user = userEvent.setup();
      render(<MasterDataReviewDialog requestId={8} onClose={vi.fn()} />);

      await user.click(screen.getAllByRole("button", { name: /Lihat perubahan/ })[0]);
      await user.click(screen.getByRole("button", { name: "Berikutnya" }));

      for (const toggle of screen.getAllByRole("button", { name: /Lihat perubahan/ })) {
        expect(toggle).toHaveAttribute("aria-expanded", "false");
      }
      expect(screen.queryByRole("columnheader", { name: "Bidang" })).not.toBeInTheDocument();
    });

    it("Setujui and Tolak still work from page 2 with the same request id", async () => {
      mockPaged();
      const onClose = vi.fn();
      approveMutate.mockImplementation((_id, { onSuccess }) => onSuccess());
      rejectMutate.mockImplementation((_id, { onSuccess }) => onSuccess());
      const user = userEvent.setup();
      render(<MasterDataReviewDialog requestId={8} onClose={onClose} />);

      await user.click(screen.getByRole("button", { name: "Berikutnya" }));
      await user.click(screen.getByRole("button", { name: "Setujui" }));
      expect(approveMutate).toHaveBeenCalledWith(8, expect.anything());

      await user.click(screen.getByRole("button", { name: "Tolak" }));
      expect(rejectMutate).toHaveBeenCalledWith(8, expect.anything());
    });
  });

  it("single change: no batch header and no paging", () => {
    mockDetail(SINGLE);
    render(<MasterDataReviewDialog requestId={7} onClose={vi.fn()} />);

    expect(screen.queryByText(/Impor CSV/)).not.toBeInTheDocument();
    expect(screen.queryByRole("navigation", { name: "Halaman perubahan" })).not.toBeInTheDocument();
    expect(screen.queryByRole("button", { name: /Lihat perubahan/ })).not.toBeInTheDocument();
  });

  it("does not offer a decision until the detail has loaded", () => {
    mockDetail(undefined, { isLoading: true });
    render(<MasterDataReviewDialog requestId={7} onClose={vi.fn()} />);

    expect(screen.getByText("Memuat perubahan…")).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Setujui" })).toBeDisabled();
    expect(screen.getByRole("button", { name: "Tolak" })).toBeDisabled();
  });
});
