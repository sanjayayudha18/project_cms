/** Task 1 (settings-menu-review): BackLink. Req 1.1, 1.3, 1.5. */

import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import type { ReactNode } from "react";
import { describe, expect, it, vi } from "vitest";
import { BackLink } from "./BackLink";

// Link jadi <a href> biasa: test ini tidak butuh RouterProvider.
vi.mock("@tanstack/react-router", () => ({
  Link: ({
    to,
    children,
    search: _search,
    ...rest
  }: { to: string; children: ReactNode; search?: unknown }) => (
    <a href={to} {...rest}>
      {children}
    </a>
  ),
}));

describe("BackLink", () => {
  it("menautkan ke `to` dengan nama aksesibel = label (panah disembunyikan)", () => {
    render(<BackLink to="/settings" label="Kembali ke Pengaturan" />);
    const link = screen.getByRole("link", { name: "Kembali ke Pengaturan" });
    expect(link).toHaveAttribute("href", "/settings");
  });

  it("bisa difokus dengan keyboard", async () => {
    const user = userEvent.setup();
    render(<BackLink to="/settings" label="Kembali ke Pengaturan" />);
    await user.tab();
    expect(screen.getByRole("link", { name: "Kembali ke Pengaturan" })).toHaveFocus();
  });
});
