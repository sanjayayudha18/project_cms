import { render, screen } from "@testing-library/react";
import type { ReactNode } from "react";
import { describe, expect, it, vi } from "vitest";
import { ReportExportNotice } from "./ReportExportNotice";

vi.mock("@tanstack/react-router", () => ({
  Link: ({ to, children }: { to: string; children: ReactNode }) => <a href={to}>{children}</a>,
}));

const LINK = { name: "Lihat Ekspor Saya" };

describe("ReportExportNotice", () => {
  it("shows the my-exports link by default", () => {
    render(<ReportExportNotice error={null} />);
    expect(screen.getByRole("link", LINK)).toBeInTheDocument();
  });

  it("hides the link but keeps the error when showMyExportsLink is false", () => {
    render(<ReportExportNotice error={new Error("Permintaan gagal")} showMyExportsLink={false} />);
    expect(screen.queryByRole("link", LINK)).not.toBeInTheDocument();
    expect(screen.getByRole("alert")).toHaveTextContent("Permintaan gagal");
  });
});
