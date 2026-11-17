import { CashCountParametersPage } from "@/features/cash-count-parameters/CashCountParametersPage";
import { useAuthStore } from "@/lib/auth/store";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { beforeEach, describe, expect, it, vi } from "vitest";

const { get, post, toast } = vi.hoisted(() => ({ get: vi.fn(), post: vi.fn(), toast: vi.fn() }));

vi.mock("@/lib/api/client", () => ({ api: { get, post, put: vi.fn() } }));
vi.mock("@/lib/hooks/useToast", () => ({ useToast: () => ({ toast }) }));

const ACTIVE = {
  medium_min_amount: "5000000000.00",
  medium_max_amount: "15000000000.00",
  onsite_high: 3,
  onsite_medium: 2,
  onsite_low: 1,
  effective_from: "2026-11-01T00:00:00Z",
  superseded_at: null,
};

function renderPage() {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  return render(
    <QueryClientProvider client={client}>
      <CashCountParametersPage />
    </QueryClientProvider>,
  );
}

type User = ReturnType<typeof userEvent.setup>;

async function openDialogAndFill(user: User, min: string, max: string) {
  await user.click(await screen.findByRole("button", { name: "Ajukan Set Baru" }));
  const minInput = screen.getByLabelText("Batas bawah Medium (IDR)");
  const maxInput = screen.getByLabelText("Batas atas Medium (IDR)");
  await user.clear(minInput);
  await user.type(minInput, min);
  await user.clear(maxInput);
  await user.type(maxInput, max);
}

describe("CashCountParametersPage - Kategori Vault (AC-5.1-03)", () => {
  beforeEach(() => {
    for (const f of [get, post, toast]) f.mockReset();
    get.mockResolvedValue({ data: { active: ACTIVE, history: [] } });
    useAuthStore.setState({ user: { role: "CC-CWM" } as never });
  });

  it("submit -> 202 shows the pending-approval message, never 'saved'", async () => {
    post.mockResolvedValue({
      data: {
        change_request_id: 42,
        status: "pending",
        entity_type: "cash_count_category_params",
        op: "create",
      },
    });
    const user = userEvent.setup();
    renderPage();
    await openDialogAndFill(user, "6000000000.00", "16000000000.00");
    await user.click(screen.getByRole("button", { name: "Ajukan" }));

    await waitFor(() => expect(toast).toHaveBeenCalled());
    expect(post).toHaveBeenCalledWith(
      "/cash-count/category-params",
      expect.objectContaining({
        medium_min_amount: "6000000000.00",
        medium_max_amount: "16000000000.00",
        currency_code: "IDR",
      }),
    );
    expect(toast).toHaveBeenCalledWith({
      type: "success",
      message: expect.stringContaining("menunggu persetujuan (#42)"),
    });
  });

  it("shows the 422 message from the API and keeps the dialog open", async () => {
    post.mockRejectedValue({
      status: 422,
      message: "Batas bawah harus lebih kecil dari batas atas",
    });
    const user = userEvent.setup();
    renderPage();
    await openDialogAndFill(user, "9000000000.00", "8000000000.00");
    await user.click(screen.getByRole("button", { name: "Ajukan" }));

    await waitFor(() =>
      expect(toast).toHaveBeenCalledWith({
        type: "error",
        message: "Batas bawah harus lebih kecil dari batas atas",
      }),
    );
    expect(screen.getByRole("button", { name: "Ajukan" })).toBeInTheDocument();
  });

  it("blocks submit for non-decimal amounts and onsite counts outside 0-6", async () => {
    const user = userEvent.setup();
    renderPage();
    await openDialogAndFill(user, "abc", "16000000000.00");
    expect(screen.getByRole("button", { name: "Ajukan" })).toBeDisabled();

    const minInput = screen.getByLabelText("Batas bawah Medium (IDR)");
    await user.clear(minInput);
    await user.type(minInput, "1000.00");
    expect(screen.getByRole("button", { name: "Ajukan" })).toBeEnabled();

    const high = screen.getByLabelText("Onsite High");
    await user.clear(high);
    await user.type(high, "7");
    expect(screen.getByRole("button", { name: "Ajukan" })).toBeDisabled();
    expect(post).not.toHaveBeenCalled();
  });

  it("hides the maker button for ADMIN (read-only)", async () => {
    useAuthStore.setState({ user: { role: "ADMIN" } as never });
    renderPage();
    expect(await screen.findByText("Set aktif")).toBeInTheDocument();
    expect(screen.queryByRole("button", { name: "Ajukan Set Baru" })).not.toBeInTheDocument();
  });
});
