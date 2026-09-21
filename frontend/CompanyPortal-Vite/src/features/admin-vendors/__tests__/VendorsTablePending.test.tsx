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
