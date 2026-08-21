import { Header } from "@/components/layout/Header";
import { Sidebar } from "@/components/layout/Sidebar";
import { useAuthStore } from "@/lib/auth/store";
import { cn } from "@/lib/utils/cn";
import { useRouter, useRouterState } from "@tanstack/react-router";
import { useCallback, useEffect, useRef, useState } from "react";

interface AppShellProps {
  children: React.ReactNode;
}

const SIDEBAR_EXPANDED_WIDTH = "256px";
const SIDEBAR_COLLAPSED_WIDTH = "64px";
const BREAKPOINT_LG = 1024;

/**
 * AppShell — the persistent layout frame for CMS.
 *
 * Renders a fixed Sidebar (left) + fixed Header (top) + scrollable main content area.
 * Uses CSS Grid: sidebar column + header row + main area.
 * Manages sidebar expanded/collapsed state via local state.
 * Responsive: sidebar collapses to icon-only at < 1024px.
 */
export function AppShell({ children }: AppShellProps) {
  const [sidebarCollapsed, setSidebarCollapsed] = useState(false);
  const mainRef = useRef<HTMLElement>(null);
  const router = useRouter();
  const user = useAuthStore((s) => s.user);
  const logout = useAuthStore((s) => s.logout);

  // Move focus to main content on route change for keyboard users
  const location = useRouterState({ select: (s) => s.location });
  const isFirstRender = useRef(true);

  // biome-ignore lint/correctness/useExhaustiveDependencies: location.pathname is the intentional trigger to shift focus to <main> on route change, even though it isn't read in the body.
  useEffect(() => {
    // Skip focus shift on initial mount
    if (isFirstRender.current) {
      isFirstRender.current = false;
      return;
    }
    mainRef.current?.focus();
  }, [location.pathname]);

  // Collapse sidebar on small viewports
  useEffect(() => {
    const mql = window.matchMedia(`(max-width: ${BREAKPOINT_LG - 1}px)`);

    const handleChange = (e: MediaQueryListEvent | MediaQueryList) => {
      setSidebarCollapsed(e.matches);
    };

    // Set initial state
    handleChange(mql);

    mql.addEventListener("change", handleChange);
    return () => mql.removeEventListener("change", handleChange);
  }, []);

  const handleSidebarToggle = useCallback(() => {
    setSidebarCollapsed((prev) => !prev);
  }, []);

  const handleNavigate = useCallback(
    (href: string) => {
      router.navigate({ to: href });
    },
    [router],
  );

  const sidebarWidth = sidebarCollapsed ? SIDEBAR_COLLAPSED_WIDTH : SIDEBAR_EXPANDED_WIDTH;

  return (
    <div
      className="grid h-screen overflow-hidden"
      style={{
        gridTemplateColumns: `${sidebarWidth} 1fr`,
        gridTemplateRows: "auto 1fr",
      }}
    >
      {/* Sidebar — spans both rows */}
      <aside
        className={cn(
          "row-span-2 flex flex-col overflow-y-auto",
          "border-r transition-[width] duration-200",
        )}
        style={{
          width: sidebarWidth,
          borderColor: "var(--n-200)",
          backgroundColor: "var(--n-0)",
          transitionTimingFunction: "cubic-bezier(0.22, 1, 0.36, 1)",
        }}
        aria-label="Navigasi utama"
      >
        {/* Sidebar toggle + brand */}
        <div
          className="flex items-center px-[var(--space-4)] py-[var(--space-3)]"
          style={{ borderBottom: "1px solid var(--n-200)" }}
        >
          <button
            type="button"
            onClick={handleSidebarToggle}
            className={cn(
              "flex items-center justify-center rounded-[var(--radius-md)]",
              "h-8 w-8 transition-colors duration-200",
            )}
            style={{
              color: "var(--n-600)",
              transitionTimingFunction: "cubic-bezier(0.22, 1, 0.36, 1)",
            }}
            aria-label={sidebarCollapsed ? "Perluas navigasi" : "Kecilkan navigasi"}
            aria-expanded={!sidebarCollapsed}
          >
            <svg
              xmlns="http://www.w3.org/2000/svg"
              width="20"
              height="20"
              viewBox="0 0 24 24"
              fill="none"
              stroke="currentColor"
              strokeWidth="2"
              strokeLinecap="round"
              strokeLinejoin="round"
              aria-hidden="true"
            >
              <line x1="3" y1="6" x2="21" y2="6" />
              <line x1="3" y1="12" x2="21" y2="12" />
              <line x1="3" y1="18" x2="21" y2="18" />
            </svg>
          </button>
          {!sidebarCollapsed && (
            <span
              className="ml-[var(--space-3)] text-sm font-semibold"
              style={{ color: "var(--n-900)" }}
            >
              CMS
            </span>
          )}
        </div>

        {/* Sidebar navigation */}
        <Sidebar
          collapsed={sidebarCollapsed}
          onToggle={handleSidebarToggle}
          currentPath={location.pathname}
          onNavigate={handleNavigate}
        />
      </aside>

      {/* Header — occupies top-right cell */}
      <header
        className="flex items-center px-[var(--space-6)] py-[var(--space-3)]"
        style={{
          borderBottom: "1px solid var(--n-200)",
          backgroundColor: "var(--n-0)",
        }}
      >
        {user && (
          <Header
            user={user}
            onLogout={logout}
            onSidebarToggle={handleSidebarToggle}
            sidebarCollapsed={sidebarCollapsed}
          />
        )}
      </header>

      {/* Main content — scrollable area */}
      <main
        ref={mainRef}
        tabIndex={-1}
        id="main-content"
        className="overflow-y-auto px-[var(--space-6)] py-[var(--space-6)] outline-none"
        style={{ backgroundColor: "var(--n-50)" }}
      >
        {children}
      </main>
    </div>
  );
}
