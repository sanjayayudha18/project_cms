import { useAuthStore } from "@/lib/auth/store";
import { fireEvent, render, screen } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { AppShell } from "./AppShell";

// Mock TanStack Router for testing without RouterProvider.
// AppShell calls useRouterState({ select }) with a selector and useRouter().navigate.
vi.mock("@tanstack/react-router", () => ({
  useRouterState: (opts?: { select?: (s: { location: { pathname: string } }) => unknown }) =>
    opts?.select
      ? opts.select({ location: { pathname: "/" } })
      : { location: { pathname: "/" } },
  useRouter: () => ({ navigate: vi.fn() }),
}));

// Mock matchMedia for responsive behavior testing
function createMatchMedia(matches: boolean) {
  const listeners: Array<(e: MediaQueryListEvent) => void> = [];
  return vi.fn().mockImplementation((query: string) => ({
    matches,
    media: query,
    onchange: null,
    addEventListener: (_event: string, handler: (e: MediaQueryListEvent) => void) => {
      listeners.push(handler);
    },
    removeEventListener: (_event: string, handler: (e: MediaQueryListEvent) => void) => {
      const index = listeners.indexOf(handler);
      if (index > -1) listeners.splice(index, 1);
    },
    dispatchEvent: vi.fn(),
    // Expose for testing
    _listeners: listeners,
    _trigger: (newMatches: boolean) => {
      for (const listener of listeners) {
        listener({ matches: newMatches } as MediaQueryListEvent);
      }
    },
  }));
}

describe("AppShell", () => {
  beforeEach(() => {
    // Header is guarded by `{user && ...}` in AppShell, so provide a user.
    useAuthStore.setState({
      user: {
        id: "1",
        fullName: "Test User",
        email: "t@t.com",
        roles: ["Admin"],
        primaryRole: "Admin",
      },
      isAuthenticated: true,
      isLoading: false,
    });
  });

  it("renders children in main content area", () => {
    window.matchMedia = createMatchMedia(false);
    render(
      <AppShell>
        <p>Konten utama</p>
      </AppShell>,
    );

    expect(screen.getByText("Konten utama")).toBeInTheDocument();
    expect(screen.getByRole("main")).toContainElement(screen.getByText("Konten utama"));
  });

  it("renders grid layout with sidebar, header, and main areas", () => {
    window.matchMedia = createMatchMedia(false);
    const { container } = render(
      <AppShell>
        <p>Content</p>
      </AppShell>,
    );

    const grid = container.firstElementChild as HTMLElement;
    expect(grid).toHaveClass("grid", "h-screen", "overflow-hidden");
    expect(grid.style.gridTemplateColumns).toBe("256px 1fr");
    expect(grid.style.gridTemplateRows).toBe("auto 1fr");
  });

  it("renders sidebar with navigation landmark", () => {
    window.matchMedia = createMatchMedia(false);
    render(
      <AppShell>
        <p>Content</p>
      </AppShell>,
    );

    const sidebar = screen.getByLabelText("Navigasi utama");
    expect(sidebar.tagName).toBe("ASIDE");
  });

  it("renders header element", () => {
    window.matchMedia = createMatchMedia(false);
    render(
      <AppShell>
        <p>Content</p>
      </AppShell>,
    );

    expect(screen.getByRole("banner")).toBeInTheDocument();
  });

  it("renders toggle button with accessible label when expanded", () => {
    window.matchMedia = createMatchMedia(false);
    render(
      <AppShell>
        <p>Content</p>
      </AppShell>,
    );

    const toggle = screen.getByLabelText("Kecilkan navigasi");
    expect(toggle).toBeInTheDocument();
    expect(toggle).toHaveAttribute("aria-expanded", "true");
  });

  it("shows CMS brand text when sidebar is expanded", () => {
    window.matchMedia = createMatchMedia(false);
    render(
      <AppShell>
        <p>Content</p>
      </AppShell>,
    );

    expect(screen.getByText("CMS")).toBeInTheDocument();
  });

  it("collapses sidebar when toggle button is clicked", () => {
    window.matchMedia = createMatchMedia(false);
    const { container } = render(
      <AppShell>
        <p>Content</p>
      </AppShell>,
    );

    const toggle = screen.getByLabelText("Kecilkan navigasi");
    fireEvent.click(toggle);

    const grid = container.firstElementChild as HTMLElement;
    expect(grid.style.gridTemplateColumns).toBe("64px 1fr");

    // Label changes to expand
    expect(screen.getByLabelText("Perluas navigasi")).toBeInTheDocument();
  });

  it("hides CMS brand text when sidebar is collapsed", () => {
    window.matchMedia = createMatchMedia(false);
    render(
      <AppShell>
        <p>Content</p>
      </AppShell>,
    );

    const toggle = screen.getByLabelText("Kecilkan navigasi");
    fireEvent.click(toggle);

    expect(screen.queryByText("CMS")).not.toBeInTheDocument();
  });

  it("auto-collapses sidebar on small viewports (< 1024px)", () => {
    window.matchMedia = createMatchMedia(true);
    const { container } = render(
      <AppShell>
        <p>Content</p>
      </AppShell>,
    );

    const grid = container.firstElementChild as HTMLElement;
    expect(grid.style.gridTemplateColumns).toBe("64px 1fr");
  });

  it("expands sidebar on large viewports (≥ 1024px)", () => {
    window.matchMedia = createMatchMedia(false);
    const { container } = render(
      <AppShell>
        <p>Content</p>
      </AppShell>,
    );

    const grid = container.firstElementChild as HTMLElement;
    expect(grid.style.gridTemplateColumns).toBe("256px 1fr");
  });

  it("main content area is scrollable", () => {
    window.matchMedia = createMatchMedia(false);
    render(
      <AppShell>
        <p>Content</p>
      </AppShell>,
    );

    const main = screen.getByRole("main");
    expect(main).toHaveClass("overflow-y-auto");
  });
});
