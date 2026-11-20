import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";
import { PksTypeFormDialog } from "../components/PksTypeFormDialog";
import type { PksType } from "../types";

const mockEditingType: PksType = {
  id: 1,
  code: "FLM",
  name: "First Line Maintenance",
  is_active: true,
  in_use: true,
  created_at: "2026-01-01T00:00:00Z",
  updated_at: "2026-01-01T00:00:00Z",
};

describe("PksTypeFormDialog", () => {
  it("renders create form when editing is null", () => {
    render(
      <PksTypeFormDialog
        open={true}
        editing={null}
        isPending={false}
        onClose={vi.fn()}
        onSubmit={vi.fn()}
      />,
    );

    expect(screen.getByText("Tambah Type PKS")).toBeInTheDocument();
    const codeInput = screen.getByLabelText(/Kode/);
    expect(codeInput).toBeInTheDocument();
    expect(codeInput).not.toBeDisabled();

    const nameInput = screen.getByLabelText(/Nama/);
    expect(nameInput).toBeInTheDocument();
    expect(nameInput).not.toBeDisabled();
  });

  it("renders edit form when editing is provided, with code disabled", () => {
    render(
      <PksTypeFormDialog
        open={true}
        editing={mockEditingType}
        isPending={false}
        onClose={vi.fn()}
        onSubmit={vi.fn()}
      />,
    );

    expect(screen.getByText("Ubah Type PKS")).toBeInTheDocument();
    const codeInput = screen.getByLabelText(/Kode/);
    expect(codeInput).toBeDisabled();
    expect(codeInput).toHaveValue("FLM");
    expect(screen.getByText("Kode tidak dapat diubah setelah dibuat.")).toBeInTheDocument();

    const nameInput = screen.getByLabelText(/Nama/);
    expect(nameInput).not.toBeDisabled();
    expect(nameInput).toHaveValue("First Line Maintenance");
  });

  it("validates required fields and code format on client", async () => {
    const onSubmit = vi.fn();
    render(
      <PksTypeFormDialog
        open={true}
        editing={null}
        isPending={false}
        onClose={vi.fn()}
        onSubmit={onSubmit}
      />,
    );

    const submitBtn = screen.getByRole("button", { name: "Simpan" });
    fireEvent.click(submitBtn);

    await waitFor(() => {
      expect(screen.getByText("Kode wajib diisi")).toBeInTheDocument();
      expect(screen.getByText("Nama wajib diisi")).toBeInTheDocument();
    });
    expect(onSubmit).not.toHaveBeenCalled();

    // Invalid code regex (lowercase or special char)
    const codeInput = screen.getByLabelText(/Kode/);
    fireEvent.change(codeInput, { target: { value: "A" } });
    fireEvent.click(submitBtn);

    await waitFor(() => {
      expect(
        screen.getByText(
          "Kode harus terdiri dari 2 hingga 10 karakter alfanumerik huruf besar (A-Z, 0-9)",
        ),
      ).toBeInTheDocument();
    });
  });

  it("submits trimmed and uppercase values on create", async () => {
    const onSubmit = vi.fn().mockResolvedValue(undefined);
    const onClose = vi.fn();

    render(
      <PksTypeFormDialog
        open={true}
        editing={null}
        isPending={false}
        onClose={onClose}
        onSubmit={onSubmit}
      />,
    );

    const codeInput = screen.getByLabelText(/Kode/);
    const nameInput = screen.getByLabelText(/Nama/);

    fireEvent.change(codeInput, { target: { value: "atm" } });
    fireEvent.change(nameInput, { target: { value: "  Automated Teller Machine  " } });

    fireEvent.click(screen.getByRole("button", { name: "Simpan" }));

    await waitFor(() => {
      expect(onSubmit).toHaveBeenCalledWith({
        code: "ATM",
        name: "Automated Teller Machine",
      });
      expect(onClose).toHaveBeenCalled();
    });
  });

  it("maps 409 conflict error to code field", async () => {
    const conflictError = {
      status: 409,
      message: "kode type PKS sudah dipakai",
    };
    const onSubmit = vi.fn().mockRejectedValue(conflictError);

    render(
      <PksTypeFormDialog
        open={true}
        editing={null}
        isPending={false}
        onClose={vi.fn()}
        onSubmit={onSubmit}
      />,
    );

    const codeInput = screen.getByLabelText(/Kode/);
    const nameInput = screen.getByLabelText(/Nama/);

    fireEvent.change(codeInput, { target: { value: "FLM" } });
    fireEvent.change(nameInput, { target: { value: "First Line Maintenance" } });

    fireEvent.click(screen.getByRole("button", { name: "Simpan" }));

    await waitFor(() => {
      expect(screen.getByText("kode type PKS sudah dipakai")).toBeInTheDocument();
    });
  });

  it("maps 422 validation error details to respective fields", async () => {
    const validationError = {
      status: 422,
      message: "validation failed",
      details: [{ field: "name", message: "Nama tidak boleh melebihi 100 karakter" }],
    };
    const onSubmit = vi.fn().mockRejectedValue(validationError);

    render(
      <PksTypeFormDialog
        open={true}
        editing={null}
        isPending={false}
        onClose={vi.fn()}
        onSubmit={onSubmit}
      />,
    );

    const codeInput = screen.getByLabelText(/Kode/);
    const nameInput = screen.getByLabelText(/Nama/);

    fireEvent.change(codeInput, { target: { value: "VALID" } });
    fireEvent.change(nameInput, { target: { value: "A very long name..." } });

    fireEvent.click(screen.getByRole("button", { name: "Simpan" }));

    await waitFor(() => {
      expect(screen.getByText("Nama tidak boleh melebihi 100 karakter")).toBeInTheDocument();
    });
  });

  it("calls onClose when Batal is clicked", () => {
    const onClose = vi.fn();
    render(
      <PksTypeFormDialog
        open={true}
        editing={null}
        isPending={false}
        onClose={onClose}
        onSubmit={vi.fn()}
      />,
    );

    fireEvent.click(screen.getByRole("button", { name: "Batal" }));
    expect(onClose).toHaveBeenCalled();
  });
});
