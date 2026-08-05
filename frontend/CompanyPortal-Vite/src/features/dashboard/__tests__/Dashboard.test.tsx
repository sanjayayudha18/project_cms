import { render, screen } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

// ─── Mock @tanstack/react-router (Link used by ReplenishmentSummary) ─────────

vi.mock("@tanstack/react-router", () => ({
  Link: ({ children, ...props }: { children: React.ReactNode; to?: string }) => (
    <a href={props.to ?? "#"} data-testid="mock-link">{children}</a>
  ),
}));

// ─── Mock useAuthStore ────────────────────────────────────────────────────────

const mockUseAuthStore = vi.fn();

vi.mock("@/lib/auth/store", () => ({
  useAuthStore: (selector: (state: unknown) => unknown) => mockUseAuthStore(selector),
}));

// ─── Import after mocks ──────────────────────────────────────────────────────

import { DashboardScreen } from "../DashboardScreen";
import { MetricStrip } from "../MetricStrip";

// ─── Test Data ────────────────────────────────────────────────────────────────

const mockUser = {
  id: "user-1",
  fullName: "Raden Budiman",
  email: "raden@cimb.com",
  roles: ["ATM_Support"],
  primaryRole: "ATM_Support",
};

describe("MetricStrip", () => {
  it("renders exactly 4 KPI cards with correct data from dashboard-kpi.json", () => {
    render(<MetricStrip />);

    // 4 KPI labels should be present
    expect(screen.getByText("Managed Cash")).toBeInTheDocument();
    expect(screen.getByText("ATM Availability")).toBeInTheDocument();
    expect(screen.getByText("Rute Hari Ini")).toBeInTheDocument();
    expect(screen.getByText("Exceptions")).toBeInTheDocument();

    // Values from the JSON
    expect(screen.getByText("IDR 18.4T")).toBeInTheDocument();
    expect(screen.getByText("98.7%")).toBeInTheDocument();
    expect(screen.getByText("184")).toBeInTheDocument();
    expect(screen.getByText("7")).toBeInTheDocument();
  });

  it("renders meta descriptions for each KPI card", () => {
    render(<MetricStrip />);

    expect(screen.getByText("↑ 2.4% dari kemarin")).toBeInTheDocument();
    expect(screen.getByText("4.812 dari 4.875 online")).toBeInTheDocument();
    expect(screen.getByText("142 selesai, 42 aktif")).toBeInTheDocument();
    expect(screen.getByText("3 prioritas tinggi sebelum 14:00")).toBeInTheDocument();
  });

  it("applies tabular-nums class on value elements", () => {
    const { container } = render(<MetricStrip />);

    const valueElements = container.querySelectorAll(".tabular-nums");
    expect(valueElements.length).toBe(4);
  });
});

describe("DashboardScreen", () => {
  beforeEach(() => {
    mockUseAuthStore.mockImplementation((selector: (state: { user: typeof mockUser }) => unknown) =>
      selector({ user: mockUser }),
    );
  });

  it("displays greeting with authenticated user's first name", () => {
    render(<DashboardScreen />);

    // Should show first name "Raden"
    expect(screen.getByRole("heading", { level: 1 })).toHaveTextContent("Raden");
  });

  it("displays today's date formatted in id-ID locale", () => {
    render(<DashboardScreen />);

    const now = new Date();
    const expectedDate = now.toLocaleDateString("id-ID", {
      weekday: "long",
      day: "numeric",
      month: "long",
      year: "numeric",
    });

    expect(screen.getByText(expectedDate)).toBeInTheDocument();
  });

  it("renders MetricStrip section with 4 KPI cards", () => {
    render(<DashboardScreen />);

    expect(screen.getByText("Managed Cash")).toBeInTheDocument();
    expect(screen.getByText("ATM Availability")).toBeInTheDocument();
    expect(screen.getByText("Rute Hari Ini")).toBeInTheDocument();
    expect(screen.getByText("Exceptions")).toBeInTheDocument();
  });

  it("falls back to 'Pengguna' when user is null (empty state)", () => {
    mockUseAuthStore.mockImplementation((selector: (state: { user: null }) => unknown) =>
      selector({ user: null }),
    );

    render(<DashboardScreen />);

    expect(screen.getByRole("heading", { level: 1 })).toHaveTextContent("Pengguna");
  });
});

describe("DashboardScreen Error State", () => {
  beforeEach(() => {
    mockUseAuthStore.mockImplementation((selector: (state: { user: typeof mockUser }) => unknown) =>
      selector({ user: mockUser }),
    );
  });

  it("renders error fallback with alert role and retry button when a child throws", async () => {
    // Suppress console.error from React ErrorBoundary
    const consoleSpy = vi.spyOn(console, "error").mockImplementation(() => {});

    // A component that throws to trigger the ErrorBoundary
    function ThrowingChild(): never {
      throw new Error("JSON load failed");
    }

    const { ErrorBoundary } = await import("@/components/feedback");

    render(
      <ErrorBoundary>
        <ThrowingChild />
      </ErrorBoundary>,
    );

    // The error fallback should render with role="alert"
    expect(screen.getByRole("alert")).toBeInTheDocument();
    // Should have a retry/reload button
    expect(screen.getByRole("button")).toBeInTheDocument();

    consoleSpy.mockRestore();
  });
});
