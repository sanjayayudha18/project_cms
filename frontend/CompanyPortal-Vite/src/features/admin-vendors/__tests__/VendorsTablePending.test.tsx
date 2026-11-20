import { render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";
import { VendorsTable } from "../components/VendorsTable";
import type { AdminVendor } from "../types";

vi.mock("@tanstack/react-router", () => ({
  Link: ({ children }: { children: React.ReactNode }) => <a href="/x">{children}</a>,
}));

const vendor = (id: number, code: string): AdminVendor =>
  ({ id, code, name: code, is_active: true }) as AdminVendor;

describe("VendorsTable pending approval", () => {
  it("badges a pending vendor with a text label and locks its actions only", () => {
    render(
      <VendorsTable
        vendors={[vendor(1, "V1"), vendor(2, "V2")]}
        pendingIds={new Set([1])}
        onEdit={() => {}}
        onDisable={() => {}}
        onEnable={() => {}}
      />,
    );
    expect(screen.getAllByText("Menunggu approval")).toHaveLength(1);
    const edit = screen.getAllByRole("button", { name: "Ubah" }) as HTMLButtonElement[];
    expect(edit.map((b) => b.disabled)).toEqual([true, false]);
  });
});

describe("VendorsTable PKS column (Req 7.3)", () => {
  const render1 = (v: Partial<AdminVendor>, canWrite = true) =>
    render(
      <VendorsTable
        vendors={[{ ...vendor(1, "V1"), ...v } as AdminVendor]}
        canWrite={canWrite}
        onEdit={() => {}}
        onDisable={() => {}}
        onEnable={() => {}}
      />,
    );

  it("shows 'N PKS', 'Belum ada' and '-' for ROH", () => {
    const { unmount } = render1({ kind: "FLM_VENDOR", pks_count: 3 });
    expect(screen.getByRole("columnheader", { name: "PKS" })).toBeTruthy();
    expect(screen.getByRole("link", { name: "3 PKS" })).toBeTruthy();
    unmount();
    const second = render1({ kind: "FLM_VENDOR", pks_count: 0 });
    expect(screen.getByRole("link", { name: "Belum ada" })).toBeTruthy();
    second.unmount();
    render1({ kind: "INTERNAL" });
    expect(screen.queryByRole("link", { name: /PKS|Belum ada/ })).toBeNull();
  });

  it("read-only (ATM-SPV) renders no Aksi column", () => {
    render1({ kind: "FLM_VENDOR", pks_count: 1 }, false);
    expect(screen.queryByRole("columnheader", { name: "Aksi" })).toBeNull();
    expect(screen.queryByRole("button", { name: "Ubah" })).toBeNull();
  });
});
