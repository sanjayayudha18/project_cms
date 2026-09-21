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

    expect(screen.getByText("Impor CSV · 250 perubahan")).toBeInTheDocument();
    expect(screen.getByText("240")).toBeInTheDocument();
    expect(screen.getByText("V-NEW")).toBeInTheDocument();
    expect(screen.getByText("T009")).toBeInTheDocument();
    expect(screen.getByText("Menampilkan 2 dari 250 perubahan.")).toBeInTheDocument();
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

  it("does not offer a decision until the detail has loaded", () => {
    mockDetail(undefined, { isLoading: true });
    render(<MasterDataReviewDialog requestId={7} onClose={vi.fn()} />);

    expect(screen.getByText("Memuat perubahan…")).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Setujui" })).toBeDisabled();
    expect(screen.getByRole("button", { name: "Tolak" })).toBeDisabled();
  });
});
