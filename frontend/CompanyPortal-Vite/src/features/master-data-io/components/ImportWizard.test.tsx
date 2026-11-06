import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vitest";
import { ImportWizard } from "./ImportWizard";

vi.mock("@tanstack/react-router", () => ({
  Link: ({ children }: { children: React.ReactNode }) => <a href="/x">{children}</a>,
}));

const NOTE = /File ekspor vendor berisi kolom Direktur Utama dan PKS/;

function setup() {
  render(
    <QueryClientProvider client={new QueryClient()}>
      <ImportWizard />
    </QueryClientProvider>,
  );
}

describe("ImportWizard catatan ekspor vendor", () => {
  it("muncul untuk entitas vendor (default)", () => {
    setup();
    expect(screen.getByText(NOTE)).toBeInTheDocument();
  });

  it("hilang untuk entitas lain dan kembali untuk vendor", async () => {
    const user = userEvent.setup();
    setup();
    const select = screen.getByLabelText("Entitas");
    await user.selectOptions(select, "atms");
    expect(screen.queryByText(NOTE)).not.toBeInTheDocument();
    await user.selectOptions(select, "vendors");
    expect(screen.getByText(NOTE)).toBeInTheDocument();
  });
});
