import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { beforeEach, describe, expect, it, vi } from "vitest";

// fe-coverage C2: the approver's inbox. Data hook mocked; the review dialog is stubbed to
// show which request it was opened for (the dialog itself has its own tests).
const inbox = vi.hoisted(() => ({
  state: {} as Record<string, unknown>,
  refetch: vi.fn(),
}));

vi.mock("../hooks", () => ({
  useInbox: () => ({ ...inbox.state, refetch: inbox.refetch }),
}));
vi.mock("../components/MasterDataReviewDialog", () => ({
  MasterDataReviewDialog: ({ requestId }: { requestId: number | null }) =>
    requestId === null ? null : <dialog open>Tinjau permintaan #{requestId}</dialog>,
}));

import { ApprovalInboxPage } from "../components/ApprovalInboxPage";

beforeEach(() => {
  inbox.refetch.mockReset();
  inbox.state = { isLoading: false, isError: false, data: [] };
});

describe("ApprovalInboxPage", () => {
  it("shows a loading skeleton, not the empty message, while loading", () => {
    inbox.state = { isLoading: true, isError: false, data: undefined };
    render(<ApprovalInboxPage />);

    expect(screen.queryByText(/Tidak ada permintaan/)).not.toBeInTheDocument();
    expect(screen.queryByRole("table")).not.toBeInTheDocument();
  });

  it("shows an error with a retry button that refetches", async () => {
    inbox.state = { isLoading: false, isError: true, data: undefined };
    render(<ApprovalInboxPage />);

    expect(screen.getByText("Gagal memuat kotak masuk")).toBeInTheDocument();
    await userEvent.setup().click(screen.getByRole("button", { name: "Coba Lagi" }));
    expect(inbox.refetch).toHaveBeenCalledTimes(1);
  });

  it("tells a non-approver there is nothing waiting", () => {
    render(<ApprovalInboxPage />);

    expect(
      screen.getByText("Tidak ada permintaan yang menunggu persetujuan Anda"),
    ).toBeInTheDocument();
  });

  it("lists pending steps; only master-data rows can be reviewed here", async () => {
    inbox.state = {
      isLoading: false,
      isError: false,
      data: [
        { step_id: 1, request_id: 41, step_level: 1, document_type: "master_data", maker_id: 7 },
        { step_id: 2, request_id: 42, step_level: 2, document_type: "invoice", maker_id: 8 },
      ],
    };
    render(<ApprovalInboxPage />);

    expect(screen.getByText("#41")).toBeInTheDocument();
    expect(screen.getByText("Perubahan data master")).toBeInTheDocument();
    expect(screen.getByText("Pengguna #7")).toBeInTheDocument();
    expect(screen.getByText("invoice")).toBeInTheDocument();
    expect(screen.getByText("Ditinjau di layar jenis dokumennya")).toBeInTheDocument();
    expect(screen.getAllByRole("button", { name: "Tinjau" })).toHaveLength(1);
    expect(screen.queryByRole("dialog")).not.toBeInTheDocument();

    await userEvent.setup().click(screen.getByRole("button", { name: "Tinjau" }));

    expect(screen.getByRole("dialog")).toHaveTextContent("Tinjau permintaan #41");
  });
});
