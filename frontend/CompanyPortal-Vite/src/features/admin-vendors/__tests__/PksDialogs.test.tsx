/**
 * vendor-pks 11.5 (Req 9.5-9.8): PksFormDialog + PksReasonDialog -- validation, 202 / 422 / 404 / 409
 * handling, screen-reader labels and keyboard use. Mutations mocked; schema + copy are real.
 */

import { useToastStore } from "@/lib/hooks/useToast";
import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { beforeEach, describe, expect, it, vi } from "vitest";

const m = vi.hoisted(() => ({ submit: vi.fn(), terminate: vi.fn(), unterminate: vi.fn() }));
vi.mock("../hooks", () => ({
  useSubmitVendorPks: () => ({ mutateAsync: m.submit, isPending: false }),
  useTerminateVendorPks: () => ({ mutateAsync: m.terminate, isPending: false }),
  useUnterminateVendorPks: () => ({ mutateAsync: m.unterminate, isPending: false }),
}));

import { PksFormDialog } from "../components/PksFormDialog";
import { PksReasonDialog } from "../components/PksReasonDialog";

const PENDING_TOAST = "Perubahan diajukan dan menunggu persetujuan (#901)";

beforeEach(() => {
  for (const fn of Object.values(m)) fn.mockReset();
  useToastStore.setState({ toasts: [] });
});

async function fillForm(
  user: ReturnType<typeof userEvent.setup>,
  v: { name?: string; eff?: string; exp?: string },
) {
  if (v.name !== undefined) await user.type(screen.getByLabelText("Nama PKS"), v.name);
  if (v.eff) await user.type(screen.getByLabelText("Tanggal efektif"), v.eff);
  if (v.exp) await user.type(screen.getByLabelText("Tanggal expired"), v.exp);
}

describe("PksFormDialog", () => {
  const renderForm = (onClose = vi.fn()) =>
    render(<PksFormDialog open onClose={onClose} vendorId={42} replacesActive={false} />);

  it("submits trimmed name + dates, toasts 'menunggu persetujuan' (202) and closes", async () => {
    m.submit.mockResolvedValue({ change_request_id: 901 });
    const onClose = vi.fn();
    const user = userEvent.setup();
    renderForm(onClose);

    await fillForm(user, { name: "  PKS Baru  ", eff: "2026-01-01", exp: "2027-01-01" });
    await user.click(screen.getByRole("button", { name: "Ajukan" }));

    await waitFor(() =>
      expect(m.submit).toHaveBeenCalledWith({
        pks_name: "PKS Baru",
        effective_date: "2026-01-01",
        expiry_date: "2027-01-01",
      }),
    );
    expect(useToastStore.getState().toasts[0]).toMatchObject({
      type: "success",
      message: PENDING_TOAST,
    });
    expect(onClose).toHaveBeenCalled();
  });

  it("requires name and both dates", async () => {
    const user = userEvent.setup();
    renderForm();

    await user.click(screen.getByRole("button", { name: "Ajukan" }));

    expect(await screen.findByText("Wajib diisi")).toBeInTheDocument();
    expect(screen.getAllByText("Tanggal wajib diisi")).toHaveLength(2);
    expect(m.submit).not.toHaveBeenCalled();
  });

  it("rejects expired before effective, accepts equal dates", async () => {
    m.submit.mockResolvedValue({ change_request_id: 901 });
    const user = userEvent.setup();
    renderForm();

    await fillForm(user, { name: "PKS", eff: "2026-06-10", exp: "2026-06-09" });
    await user.click(screen.getByRole("button", { name: "Ajukan" }));
    expect(
      await screen.findByText("Tanggal expired tidak boleh sebelum tanggal efektif"),
    ).toBeInTheDocument();
    expect(m.submit).not.toHaveBeenCalled();

    await user.clear(screen.getByLabelText("Tanggal expired"));
    await user.type(screen.getByLabelText("Tanggal expired"), "2026-06-10");
    await user.click(screen.getByRole("button", { name: "Ajukan" }));
    await waitFor(() => expect(m.submit).toHaveBeenCalledTimes(1));
  });

  it("rejects a name over 500 characters", async () => {
    const user = userEvent.setup();
    renderForm();

    await user.click(screen.getByLabelText("Nama PKS"));
    await user.paste("x".repeat(501));
    await fillForm(user, { eff: "2026-01-01", exp: "2027-01-01" });
    await user.click(screen.getByRole("button", { name: "Ajukan" }));

    expect(await screen.findByText("Maksimal 500 karakter")).toBeInTheDocument();
    expect(m.submit).not.toHaveBeenCalled();
  });

  it("marks the invalid field for screen readers (aria-invalid + described error)", async () => {
    const user = userEvent.setup();
    renderForm();

    await user.click(screen.getByRole("button", { name: "Ajukan" }));

    const name = await screen.findByLabelText("Nama PKS");
    expect(name).toHaveAttribute("aria-invalid", "true");
    expect(name).toHaveAccessibleDescription("Wajib diisi");
  });

  it("422 puts the server message on the field and keeps the dialog open", async () => {
    m.submit.mockRejectedValue({
      status: 422,
      message: "validation",
      details: [{ field: "expiry_date", message: "Tanggal expired tidak valid" }],
    });
    const onClose = vi.fn();
    const user = userEvent.setup();
    renderForm(onClose);

    await fillForm(user, { name: "PKS", eff: "2026-01-01", exp: "2027-01-01" });
    await user.click(screen.getByRole("button", { name: "Ajukan" }));

    expect(await screen.findByText("Tanggal expired tidak valid")).toBeInTheDocument();
    expect(screen.getByLabelText("Tanggal expired")).toHaveAttribute("aria-invalid", "true");
    expect(onClose).not.toHaveBeenCalled();
  });

  it.each([
    [404, "Vendor tidak ditemukan"],
    [409, "Masih ada perubahan PKS yang menunggu persetujuan"],
  ])("%i shows a banner alert and keeps the dialog open", async (status, message) => {
    m.submit.mockRejectedValue({ status, message });
    const onClose = vi.fn();
    const user = userEvent.setup();
    renderForm(onClose);

    await fillForm(user, { name: "PKS", eff: "2026-01-01", exp: "2027-01-01" });
    await user.click(screen.getByRole("button", { name: "Ajukan" }));

    expect(await screen.findByRole("alert")).toHaveTextContent(message);
    expect(onClose).not.toHaveBeenCalled();
  });

  it("is operable by keyboard: tab through the fields, Enter submits", async () => {
    m.submit.mockResolvedValue({ change_request_id: 901 });
    const user = userEvent.setup();
    renderForm();

    await user.click(screen.getByLabelText("Nama PKS"));
    await user.keyboard("PKS Kbd");
    await user.tab();
    expect(screen.getByLabelText("Tanggal efektif")).toHaveFocus();
    await user.keyboard("2026-01-01");
    await user.tab();
    expect(screen.getByLabelText("Tanggal expired")).toHaveFocus();
    await user.keyboard("2027-01-01{Enter}");

    await waitFor(() => expect(m.submit).toHaveBeenCalledTimes(1));
  });

  it("clears previous input and banner when reopened", async () => {
    m.submit.mockRejectedValue({ status: 409, message: "bentrok" });
    const user = userEvent.setup();
    const { rerender } = render(
      <PksFormDialog open onClose={vi.fn()} vendorId={42} replacesActive={false} />,
    );
    await fillForm(user, { name: "PKS", eff: "2026-01-01", exp: "2027-01-01" });
    await user.click(screen.getByRole("button", { name: "Ajukan" }));
    await screen.findByRole("alert");

    rerender(<PksFormDialog open={false} onClose={vi.fn()} vendorId={42} replacesActive={false} />);
    rerender(<PksFormDialog open onClose={vi.fn()} vendorId={42} replacesActive={false} />);

    expect(screen.queryByRole("alert")).not.toBeInTheDocument();
    expect(screen.getByLabelText("Nama PKS")).toHaveValue("");
  });
});

describe("PksReasonDialog", () => {
  const renderReason = (mode: "terminate" | "unterminate", onClose = vi.fn()) =>
    render(
      <PksReasonDialog
        open
        onClose={onClose}
        vendorId={42}
        pksId={7}
        pksName="PKS Aktif"
        mode={mode}
      />,
    );

  it.each([
    ["terminate", "Terminate PKS: PKS Aktif", "PKS menjadi Terminated setelah disetujui"],
    ["unterminate", "Batalkan terminate: PKS Aktif", "PKS kembali ke status sesuai tanggalnya"],
  ] as const)("%s: title + hint copy", (mode, title, hint) => {
    renderReason(mode);

    expect(screen.getByRole("dialog", { name: title })).toHaveTextContent(hint);
    expect(screen.getByLabelText("Alasan")).toBeInTheDocument();
  });

  it("requires a reason (blank and whitespace-only)", async () => {
    const user = userEvent.setup();
    renderReason("terminate");

    await user.type(screen.getByLabelText("Alasan"), "   ");
    await user.click(screen.getByRole("button", { name: "Ajukan" }));

    expect(await screen.findByText("Alasan wajib diisi")).toBeInTheDocument();
    expect(screen.getByLabelText("Alasan")).toHaveAccessibleDescription("Alasan wajib diisi");
    expect(m.terminate).not.toHaveBeenCalled();
  });

  it("caps the reason at 500 characters", async () => {
    const user = userEvent.setup();
    renderReason("terminate");

    expect(screen.getByLabelText("Alasan")).toHaveAttribute("maxlength", "500");
    await user.click(screen.getByLabelText("Alasan"));
    await user.paste("x".repeat(600));
    expect((screen.getByLabelText("Alasan") as HTMLTextAreaElement).value).toHaveLength(500);
  });

  it("terminate: sends the reason, toasts 202 and closes", async () => {
    m.terminate.mockResolvedValue({ change_request_id: 901 });
    const onClose = vi.fn();
    const user = userEvent.setup();
    renderReason("terminate", onClose);

    await user.type(screen.getByLabelText("Alasan"), "Kontrak diputus");
    await user.click(screen.getByRole("button", { name: "Ajukan" }));

    await waitFor(() =>
      expect(m.terminate).toHaveBeenCalledWith({
        pksId: 7,
        payload: { reason: "Kontrak diputus" },
      }),
    );
    expect(useToastStore.getState().toasts[0]).toMatchObject({
      type: "success",
      message: PENDING_TOAST,
    });
    expect(onClose).toHaveBeenCalled();
    expect(m.unterminate).not.toHaveBeenCalled();
  });

  it.each([
    [404, "PKS tidak ditemukan"],
    [409, "PKS sudah terminated"],
  ])("%i shows a banner alert and keeps the dialog open", async (status, message) => {
    m.terminate.mockRejectedValue({ status, message });
    const onClose = vi.fn();
    const user = userEvent.setup();
    renderReason("terminate", onClose);

    await user.type(screen.getByLabelText("Alasan"), "Alasan");
    await user.click(screen.getByRole("button", { name: "Ajukan" }));

    expect(await screen.findByRole("alert")).toHaveTextContent(message);
    expect(onClose).not.toHaveBeenCalled();
  });

  it("422 puts the server message on the reason field", async () => {
    m.unterminate.mockRejectedValue({
      status: 422,
      message: "validation",
      details: [{ field: "reason", message: "Alasan terlalu pendek" }],
    });
    const user = userEvent.setup();
    renderReason("unterminate");

    await user.type(screen.getByLabelText("Alasan"), "x");
    await user.click(screen.getByRole("button", { name: "Ajukan" }));

    expect(await screen.findByText("Alasan terlalu pendek")).toBeInTheDocument();
  });

  it("'Batal' closes without submitting", async () => {
    const onClose = vi.fn();
    const user = userEvent.setup();
    renderReason("terminate", onClose);

    await user.click(screen.getByRole("button", { name: "Batal" }));

    expect(onClose).toHaveBeenCalled();
    expect(m.terminate).not.toHaveBeenCalled();
  });
});
