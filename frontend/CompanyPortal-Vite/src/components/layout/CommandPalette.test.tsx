/**
 * Task 8.2 (app-shell-revamp): Property 4 (`nextIndex` melingkar) + perilaku CommandPalette
 * (Req 12.1–12.3, 12.5–12.10). Palette di-unmount saat tutup, jadi "dibuka ulang" = render baru.
 */

import { NAV_CONFIG, type NavItem } from "@/lib/config/navigation";
import { fireEvent, render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import * as fc from "fast-check";
import { LayoutGrid, Settings2, Truck } from "lucide-react";
import { beforeAll, describe, expect, it, vi } from "vitest";
import CommandPalette, { nextIndex } from "./CommandPalette";

const ITEMS: NavItem[] = [
  {
    id: "dashboard",
    label: "Dashboard",
    icon: LayoutGrid,
    href: "/",
    roles: ["*"],
    group: "general",
  },
  {
    id: "vendor-requests",
    label: "Vendor Request",
    icon: Truck,
    href: "/replenishment/vendor-requests",
    roles: ["*"],
    group: "replenish",
  },
  {
    id: "settings",
    label: "Pengaturan",
    icon: Settings2,
    href: "/settings",
    roles: ["*"],
    group: "system",
    placement: "sidebar-footer",
  },
];

beforeAll(() => {
  // jsdom tidak punya scrollIntoView; komponen memanggilnya di effect.
  Element.prototype.scrollIntoView = vi.fn();
});

function setup(props: Partial<React.ComponentProps<typeof CommandPalette>> = {}) {
  const onSelect = vi.fn();
  const onClose = vi.fn();
  const utils = render(
    <CommandPalette items={ITEMS} onSelect={onSelect} onClose={onClose} {...props} />,
  );
  return { onSelect, onClose, ...utils };
}

describe("nextIndex (Property 4)", () => {
  it("selalu di [0, n) dan n kali ArrowDown/ArrowUp kembali ke indeks awal", () => {
    fc.assert(
      fc.property(
        fc.integer({ min: 1, max: 50 }),
        fc.nat(),
        fc.array(fc.constantFrom(1, -1), { maxLength: 100 }),
        (n, start, moves) => {
          const i0 = start % n;
          let i = i0;
          for (const d of moves) {
            i = nextIndex(i, d, n);
            expect(i).toBeGreaterThanOrEqual(0);
            expect(i).toBeLessThan(n);
          }
          let down = i0;
          let up = i0;
          for (let k = 0; k < n; k++) {
            down = nextIndex(down, 1, n);
            up = nextIndex(up, -1, n);
          }
          expect(down).toBe(i0);
          expect(up).toBe(i0);
        },
      ),
    );
  });

  it("membungkus di ujung dan aman untuk n = 0", () => {
    expect(nextIndex(2, 1, 3)).toBe(0);
    expect(nextIndex(0, -1, 3)).toBe(2);
    expect(nextIndex(0, 1, 0)).toBe(0);
    expect(nextIndex(0, -1, 0)).toBe(0);
  });
});

describe("CommandPalette", () => {
  it("render ARIA: dialog modal, combobox → listbox, option aria-selected (Req 12.1, 12.2)", () => {
    setup();
    expect(screen.getByRole("dialog", { name: "Cari menu" })).toHaveAttribute("aria-modal", "true");

    const input = screen.getByRole("combobox", { name: "Cari menu" });
    const list = screen.getByRole("listbox", { name: "Hasil menu" });
    expect(input).toHaveAttribute("aria-controls", list.id);

    const options = screen.getAllByRole("option");
    expect(options).toHaveLength(ITEMS.length);
    expect(options[0]).toHaveAttribute("aria-selected", "true");
    expect(options[1]).toHaveAttribute("aria-selected", "false");
    expect(input).toHaveAttribute("aria-activedescendant", options[0].id);
  });

  it("fokus ada di input saat terbuka (Req 12.1)", () => {
    setup();
    expect(screen.getByRole("combobox")).toHaveFocus();
  });

  it("aria-activedescendant mengikuti baris aktif saat ArrowDown/ArrowUp, melingkar (Req 12.6)", async () => {
    const user = userEvent.setup();
    setup();
    const input = screen.getByRole("combobox");
    const options = screen.getAllByRole("option");

    await user.keyboard("{ArrowDown}");
    expect(input).toHaveAttribute("aria-activedescendant", options[1].id);
    expect(options[1]).toHaveAttribute("aria-selected", "true");
    expect(options[0]).toHaveAttribute("aria-selected", "false");

    await user.keyboard("{ArrowUp}{ArrowUp}");
    expect(input).toHaveAttribute("aria-activedescendant", options[2].id);
  });

  it("filter berdasarkan label dan label grup (Req 12.4)", async () => {
    const user = userEvent.setup();
    setup();
    await user.type(screen.getByRole("combobox"), "vendor");
    expect(screen.getAllByRole("option").map((o) => o.textContent)).toEqual([
      expect.stringContaining("Vendor Request"),
    ]);

    await user.clear(screen.getByRole("combobox"));
    await user.type(screen.getByRole("combobox"), "sistem");
    expect(screen.getAllByRole("option")).toHaveLength(1);
  });

  it('empty state `Tidak ada menu "{q}"` tanpa listbox (Req 12.9)', async () => {
    const user = userEvent.setup();
    setup();
    await user.type(screen.getByRole("combobox"), "zzz");
    expect(screen.getByRole("status")).toHaveTextContent('Tidak ada menu "zzz"');
    expect(screen.queryByRole("listbox")).not.toBeInTheDocument();
    expect(screen.getByRole("combobox")).not.toHaveAttribute("aria-activedescendant");
  });

  it("Enter pada baris aktif memanggil onSelect(item) (Req 12.7)", async () => {
    const user = userEvent.setup();
    const { onSelect } = setup();
    await user.keyboard("{ArrowDown}{Enter}");
    expect(onSelect).toHaveBeenCalledTimes(1);
    expect(onSelect).toHaveBeenCalledWith(ITEMS[1]);
  });

  it("Enter tanpa hasil tidak memanggil onSelect", async () => {
    const user = userEvent.setup();
    const { onSelect } = setup();
    await user.type(screen.getByRole("combobox"), "zzz{Enter}");
    expect(onSelect).not.toHaveBeenCalled();
  });

  it("klik baris memanggil onSelect(item) (Req 12.7)", async () => {
    const user = userEvent.setup();
    const { onSelect } = setup();
    await user.click(screen.getByRole("option", { name: /Vendor Request/ }));
    expect(onSelect).toHaveBeenCalledWith(ITEMS[1]);
  });

  it("Escape memanggil onClose (Req 12.8)", async () => {
    const user = userEvent.setup();
    const { onClose } = setup();
    await user.keyboard("{Escape}");
    expect(onClose).toHaveBeenCalledTimes(1);
  });

  it("klik overlay menutup, klik panel tidak (Req 12.8)", () => {
    const { onClose } = setup();
    const dialog = screen.getByRole("dialog");
    fireEvent.pointerDown(dialog);
    expect(onClose).not.toHaveBeenCalled();

    fireEvent.pointerDown(dialog.parentElement as HTMLElement);
    expect(onClose).toHaveBeenCalledTimes(1);
  });

  it("Tab tidak memindahkan fokus keluar dari input (focus trap)", async () => {
    const user = userEvent.setup();
    setup();
    await user.tab();
    expect(screen.getByRole("combobox")).toHaveFocus();
  });

  it('Pengaturan tampil dengan grup "Sistem" (Req 12.3)', () => {
    setup();
    const option = screen.getByRole("option", { name: /Pengaturan/ });
    expect(option).toHaveTextContent("Sistem");
  });

  it("item dari NAV_CONFIG (termasuk Pengaturan topbar) tampil semua", () => {
    setup({ items: NAV_CONFIG.filter((i) => !i.disabled) });
    expect(screen.getByRole("option", { name: /Pengaturan/ })).toBeInTheDocument();
  });

  it("query dan indeks di-reset saat dibuka ulang (Req 12.10)", async () => {
    const user = userEvent.setup();
    const { unmount, onSelect, onClose } = setup();
    await user.type(screen.getByRole("combobox"), "vendor");
    unmount();

    render(<CommandPalette items={ITEMS} onSelect={onSelect} onClose={onClose} />);
    expect(screen.getByRole("combobox")).toHaveValue("");
    expect(screen.getAllByRole("option")).toHaveLength(ITEMS.length);
    expect(screen.getAllByRole("option")[0]).toHaveAttribute("aria-selected", "true");
  });

  it("mengetik mereset indeks aktif ke baris pertama", async () => {
    const user = userEvent.setup();
    setup();
    await user.keyboard("{ArrowDown}{ArrowDown}");
    await user.type(screen.getByRole("combobox"), "e");
    expect(screen.getAllByRole("option")[0]).toHaveAttribute("aria-selected", "true");
  });
});
