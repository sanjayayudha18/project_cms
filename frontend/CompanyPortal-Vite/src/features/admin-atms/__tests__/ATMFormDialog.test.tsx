import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { ATMFormDialog } from "../components/ATMFormDialog";
import type { AdminATM } from "../types";

const EXISTING_ATM: AdminATM = {
  id: 1,
  terminal_id: "TATM001",
  location_id: 10,
  location_name: "Jakarta Pusat",
  machine_type: "ATM",
  brand: "NCR",
  model: "SelfServ",
  operation_hours: "24 Hours",
  deployment_type: "ONSITE",
  capacity_amount: "100000000.00",
  low_threshold_amount: null,
  critical_threshold_amount: null,
  blacklisted: false,
  escrow_account: null,
  priority_class: "VIP",
  is_active: true,
  created_at: null,
  updated_at: null,
  deleted_at: null,
};

const createMutateAsync = vi.fn();
const updateMutateAsync = vi.fn();
const useLocationOptionsMock = vi.fn();

vi.mock("../hooks", () => ({
  useCreateATM: () => ({ mutateAsync: createMutateAsync, isPending: false }),
  useUpdateATM: () => ({ mutateAsync: updateMutateAsync, isPending: false }),
  useLocationOptions: () => useLocationOptionsMock(),
}));

beforeEach(() => {
  createMutateAsync.mockReset();
  updateMutateAsync.mockReset();
  useLocationOptionsMock.mockReturnValue({
    data: {
      locations: [
        {
          id: 10,
          name: "Jakarta Pusat",
          city_or_regency: "Jakarta Pusat",
          province: "DKI Jakarta",
        },
        { id: 20, name: "Bandung", city_or_regency: "Bandung", province: "Jawa Barat" },
      ],
    },
  });
});

function fillRequiredFields() {
  return {
    terminalId: screen.getByLabelText("Terminal ID") as HTMLInputElement,
  };
}

async function fillAllRequiredFields(user: ReturnType<typeof userEvent.setup>, terminalId: string) {
  await user.type(screen.getByLabelText("Terminal ID"), terminalId);
  await user.selectOptions(screen.getByLabelText("Lokasi"), "10");
  await user.type(screen.getByLabelText("Tipe Mesin"), "ATM");
  await user.type(screen.getByLabelText("Brand"), "NCR");
  await user.type(screen.getByLabelText("Model"), "SelfServ");
  await user.type(screen.getByLabelText("Jam Operasional"), "24 Hours");
  await user.type(screen.getByLabelText("Deployment"), "ONSITE");
}

describe("ATMFormDialog", () => {
  it("populates the Location select from useLocationOptions", () => {
    render(<ATMFormDialog open onClose={vi.fn()} atm={null} />);

    expect(screen.getByRole("option", { name: "Jakarta Pusat" })).toBeInTheDocument();
    expect(screen.getByRole("option", { name: "Bandung" })).toBeInTheDocument();
  });

  it("disables Terminal ID in edit mode", () => {
    render(<ATMFormDialog open onClose={vi.fn()} atm={EXISTING_ATM} />);

    const { terminalId } = fillRequiredFields();
    expect(terminalId).toBeDisabled();
    expect(terminalId.value).toBe("TATM001");
  });

  it("does not disable Terminal ID in create mode", () => {
    render(<ATMFormDialog open onClose={vi.fn()} atm={null} />);

    const { terminalId } = fillRequiredFields();
    expect(terminalId).not.toBeDisabled();
  });

  it("maps a 422 validation_error to its field and keeps the dialog open", async () => {
    const user = userEvent.setup();
    createMutateAsync.mockRejectedValue({
      status: 422,
      message: "Validasi gagal",
      details: [{ field: "terminal_id", message: "wajib diisi" }],
    });
    const onClose = vi.fn();
    render(<ATMFormDialog open onClose={onClose} atm={null} />);

    await fillAllRequiredFields(user, "TATM999");
    await user.click(screen.getByRole("button", { name: "Simpan" }));

    expect(await screen.findByText("wajib diisi")).toBeInTheDocument();
    expect(onClose).not.toHaveBeenCalled();
  });

  it("maps a 409 conflict to the Terminal ID field and keeps the dialog open", async () => {
    const user = userEvent.setup();
    createMutateAsync.mockRejectedValue({
      status: 409,
      message: "atm terminal_id already exists",
    });
    const onClose = vi.fn();
    render(<ATMFormDialog open onClose={onClose} atm={null} />);

    await fillAllRequiredFields(user, "TATM001");
    await user.click(screen.getByRole("button", { name: "Simpan" }));

    expect(await screen.findByText("atm terminal_id already exists")).toBeInTheDocument();
    expect(onClose).not.toHaveBeenCalled();
  });

  it("maps a 400 invalid_reference to the Lokasi field and keeps the dialog open", async () => {
    const user = userEvent.setup();
    updateMutateAsync.mockRejectedValue({
      status: 400,
      message: "referenced location does not exist",
    });
    const onClose = vi.fn();
    render(<ATMFormDialog open onClose={onClose} atm={EXISTING_ATM} />);

    await user.click(screen.getByRole("button", { name: "Simpan" }));

    expect(await screen.findByText("referenced location does not exist")).toBeInTheDocument();
    expect(onClose).not.toHaveBeenCalled();
  });

  it("submits a valid decimal amount as a string, unchanged", async () => {
    const user = userEvent.setup();
    createMutateAsync.mockResolvedValue({});
    render(<ATMFormDialog open onClose={vi.fn()} atm={null} />);

    await fillAllRequiredFields(user, "TATM777");
    await user.type(screen.getByLabelText("Kapasitas"), "1234567890123456.78");
    await user.click(screen.getByRole("button", { name: "Simpan" }));

    expect(createMutateAsync).toHaveBeenCalledWith(
      expect.objectContaining({ capacity_amount: "1234567890123456.78" }),
    );
  });
});
