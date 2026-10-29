import { render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { beforeEach, describe, expect, it, vi } from "vitest";

const branchesHook = vi.hoisted(() => vi.fn());
vi.mock("@/features/admin-vendors/hooks", () => ({
  useVendorsList: () => ({
    data: {
      vendors: [
        { id: 7, name: "Vendor Tujuh", code: "V7" },
        { id: 8, name: "Vendor Delapan", code: "V8" },
      ],
    },
  }),
  useVendorBranches: (...a: unknown[]) => {
    branchesHook(...a);
    return { data: { branches: [{ id: 3, branch_name: "Cabang Tiga", branch_code: "B3" }] } };
  },
}));
const flow = vi.hoisted(() => ({
  request: vi.fn(),
  requestError: null as unknown,
  isRequesting: false,
}));
vi.mock("../hooks", async (orig) => ({
  ...(await orig<typeof import("../hooks")>()),
  useExportFlow: () => ({
    request: flow.request,
    requestError: flow.requestError,
    isRequesting: flow.isRequesting,
    dialogProps: {
      open: false,
      phase: "idle",
      job: null,
      message: null,
      onClose: vi.fn(),
      onRetry: vi.fn(),
    },
  }),
  useMyExports: () => ({ data: [], isLoading: false }),
}));

vi.mock("./MyExportsTable", () => ({ MyExportsTable: () => <p>tabel</p> }));
vi.mock("./ExportProgressDialog", () => ({ ExportProgressDialog: () => null }));

import { ExportDataPage } from "./ExportDataPage";
import { PackagePriceExportForm } from "./PackagePriceExportForm";
import { VendorExportForm } from "./VendorExportForm";

const user = userEvent.setup();
const props = () => ({ onRequest: vi.fn(), isRequesting: false, error: null as unknown });
const pick = (label: string, value: string) =>
  user.selectOptions(screen.getByRole("combobox", { name: label }), value);
const optionTexts = (label: string) =>
  within(screen.getByRole("combobox", { name: label }))
    .getAllByRole("option")
    .map((o) => o.textContent);
const submit = () => user.click(screen.getByRole("button", { name: "Minta ekspor" }));

beforeEach(() => {
  vi.clearAllMocks();
  flow.requestError = null;
  flow.isRequesting = false;
});

describe("VendorExportForm", () => {
  it("level switches the entity list", async () => {
    render(<VendorExportForm {...props()} />);
    expect(optionTexts("Data yang diekspor")).toContain("Profil vendor");
    await user.click(screen.getByRole("radio", { name: "Cabang" }));
    const texts = optionTexts("Data yang diekspor");
    expect(texts).not.toContain("Profil vendor");
    expect(texts).toContain("Vault");
  });

  it("requires cabang for level Cabang", async () => {
    render(<VendorExportForm {...props()} />);
    await user.click(screen.getByRole("radio", { name: "Cabang" }));
    await pick("Vendor", "7");
    expect(screen.getByRole("button", { name: "Minta ekspor" })).toBeDisabled();
    await pick("Cabang", "3");
    expect(screen.getByRole("button", { name: "Minta ekspor" })).toBeEnabled();
  });

  it("resets cabang when the vendor changes", async () => {
    render(<VendorExportForm {...props()} />);
    await user.click(screen.getByRole("radio", { name: "Cabang" }));
    await pick("Vendor", "7");
    await pick("Cabang", "3");
    await pick("Vendor", "8");
    expect(screen.getByRole("combobox", { name: "Cabang" })).toHaveValue("");
    expect(branchesHook).toHaveBeenLastCalledWith(8, expect.anything());
  });

  it("requests vendor kind with default status active", async () => {
    const p = props();
    render(<VendorExportForm {...p} />);
    await pick("Vendor", "7");
    await submit();
    expect(p.onRequest).toHaveBeenCalledWith({
      kind: "vendor",
      vendorId: 7,
      entity: "vendor",
      status: "active",
    });
  });

  it("requests cabang kind", async () => {
    const p = props();
    render(<VendorExportForm {...p} />);
    await user.click(screen.getByRole("radio", { name: "Cabang" }));
    await pick("Vendor", "7");
    await pick("Cabang", "3");
    await pick("Status", "disabled");
    await submit();
    expect(p.onRequest).toHaveBeenCalledWith({
      kind: "cabang",
      vendorId: 7,
      branchId: 3,
      entity: "vaults",
      status: "disabled",
    });
  });

  it("disables the button while requesting", () => {
    render(<VendorExportForm {...props()} isRequesting />);
    expect(screen.getByRole("button", { name: "Minta ekspor" })).toBeDisabled();
  });
});

describe("PackagePriceExportForm", () => {
  it("requests harga_paket with defaults (price level all, status active)", async () => {
    const p = props();
    render(<PackagePriceExportForm {...p} />);
    await pick("Vendor", "7");
    await submit();
    expect(p.onRequest).toHaveBeenCalledWith({
      kind: "harga_paket",
      vendorId: 7,
      branchId: undefined,
      priceLevel: "all",
      status: "active",
    });
  });

  it("passes optional cabang and chosen level", async () => {
    const p = props();
    render(<PackagePriceExportForm {...p} />);
    await pick("Vendor", "7");
    await pick("Cabang", "3");
    await pick("Tingkat harga", "atm");
    await submit();
    expect(p.onRequest).toHaveBeenCalledWith(
      expect.objectContaining({ branchId: 3, priceLevel: "atm" }),
    );
  });
});

describe("request errors", () => {
  it("429 shows an inline message", () => {
    const e = { status: 429, code: "too_many_exports", message: "x" };
    render(<VendorExportForm {...props()} error={e} />);
    expect(screen.getByRole("alert")).toHaveTextContent(/Terlalu banyak ekspor/);
  });

  it("other errors are inline too", () => {
    render(<VendorExportForm {...props()} error={new Error("Gagal total")} />);
    expect(screen.getByRole("alert")).toHaveTextContent("Gagal total");
  });
});

describe("ExportDataPage", () => {
  it("renders the three sections", () => {
    render(<ExportDataPage />);
    for (const name of ["Data Vendor", "Harga Paket", "Ekspor Saya"]) {
      expect(screen.getByRole("region", { name })).toBeInTheDocument();
    }
  });

  it("shows the request error only beside the form that asked", async () => {
    flow.request.mockResolvedValue({ error: new Error("boom") });
    flow.requestError = new Error("boom");
    render(<ExportDataPage />);
    expect(screen.queryByRole("alert")).not.toBeInTheDocument();
    const harga = screen.getByRole("region", { name: "Harga Paket" });
    await user.selectOptions(within(harga).getByLabelText("Vendor"), "7");
    await user.click(within(harga).getByRole("button", { name: "Minta ekspor" }));
    expect(within(harga).getByRole("alert")).toBeInTheDocument();
    expect(
      within(screen.getByRole("region", { name: "Data Vendor" })).queryByRole("alert"),
    ).not.toBeInTheDocument();
  });
});
