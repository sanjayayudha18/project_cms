import { fireEvent, render, screen, within } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

import { AdminHolidaysPage } from "../AdminHolidaysPage";
import type { Holiday } from "../api";

const { create, rename, setActive, upload, toast, listResult } = vi.hoisted(() => ({
  create: vi.fn(),
  rename: vi.fn(),
  setActive: vi.fn(),
  upload: vi.fn(),
  toast: vi.fn(),
  listResult: { current: null as unknown },
}));

vi.mock("../api", async (importOriginal) => {
  const actual = await importOriginal<typeof import("../api")>();
  return {
    ...actual,
    useHolidays: () => listResult.current,
    useCreateHoliday: () => ({ mutate: create, isPending: false }),
    useRenameHoliday: () => ({ mutate: rename, isPending: false }),
    useSetHolidayActive: () => ({ mutate: setActive, isPending: false }),
    useImportHolidays: () => ({ mutate: upload, isPending: false }),
  };
});
vi.mock("@/lib/hooks/useToast", () => ({ useToast: () => ({ toast }) }));

const NATAL: Holiday = {
  id: 7,
  holiday_date: "2026-12-25T00:00:00Z",
  name: "Hari Raya Natal",
  is_active: true,
};

function rows(items: Holiday[]) {
  return { isLoading: false, isError: false, data: { items, total: items.length } };
}

describe("AdminHolidaysPage (replenishment-realisasi FR1)", () => {
  beforeEach(() => {
    for (const f of [create, rename, setActive, upload, toast]) f.mockReset();
    listResult.current = rows([NATAL, { ...NATAL, id: 8, name: "Natal lama", is_active: false }]);
  });

  it("lists holidays with a text status", () => {
    render(<AdminHolidaysPage />);
    expect(screen.getByText("Hari Raya Natal")).toBeInTheDocument();
    const table = within(screen.getByRole("table"));
    expect(table.getByText("Aktif")).toBeInTheDocument();
    expect(table.getByText("Nonaktif")).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Aktifkan" })).toBeInTheDocument();
  });

  it("explains the Monday-Friday fallback when the year is empty", () => {
    listResult.current = rows([]);
    render(<AdminHolidaysPage />);
    expect(screen.getByText(/hari kerja = Senin–Jumat/)).toBeInTheDocument();
  });

  it("adds a holiday with a trimmed name, only when date + name are filled", () => {
    render(<AdminHolidaysPage />);
    const add = screen.getByRole("button", { name: "Tambah" });
    expect(add).toBeDisabled();
    fireEvent.change(screen.getByLabelText("Tanggal"), { target: { value: "2026-12-26" } });
    fireEvent.change(screen.getByLabelText("Nama"), { target: { value: "  Cuti bersama  " } });
    fireEvent.click(add);
    expect(create.mock.calls[0]?.[0]).toEqual({ holiday_date: "2026-12-26", name: "Cuti bersama" });
  });

  it("disables an active holiday and renames inline", () => {
    render(<AdminHolidaysPage />);
    fireEvent.click(screen.getByRole("button", { name: "Nonaktifkan" }));
    expect(setActive.mock.calls[0]?.[0]).toEqual({ id: 7, active: false });

    fireEvent.click(screen.getAllByRole("button", { name: "Ubah nama" })[0] as HTMLElement);
    fireEvent.change(screen.getByLabelText("Nama baru Hari Raya Natal"), {
      target: { value: "Natal 2026" },
    });
    fireEvent.click(screen.getByRole("button", { name: "Simpan" }));
    expect(rename.mock.calls[0]?.[0]).toEqual({ id: 7, name: "Natal 2026" });
  });

  it("imports a CSV and reports created + skipped dates", () => {
    upload.mockImplementation((_f: File, opts: { onSuccess: (r: unknown) => void }) =>
      opts.onSuccess({ created: 2, skipped: ["2026-12-25"] }),
    );
    render(<AdminHolidaysPage />);
    const file = new File(["holiday_date,name\n"], "libur.csv", { type: "text/csv" });
    fireEvent.change(screen.getByLabelText(/Impor CSV/), { target: { files: [file] } });
    fireEvent.click(screen.getByRole("button", { name: /Unggah/ }));
    expect(toast).toHaveBeenCalledWith({
      type: "success",
      message: "2 hari libur diimpor, 1 dilewati (sudah ada): 2026-12-25",
    });
  });
});
