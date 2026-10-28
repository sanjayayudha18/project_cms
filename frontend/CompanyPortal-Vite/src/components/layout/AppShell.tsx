import { SIDEBAR_WIDTH_EXPANDED, SIDEBAR_WIDTH_RAIL, Sidebar } from "@/components/layout/Sidebar";
import { Topbar } from "@/components/layout/Topbar";
import { useNavBadges } from "@/components/layout/useNavBadges";
import { useAuthStore } from "@/lib/auth/store";
import { type NavItem, findActiveNavItem, getVisibleNav } from "@/lib/config/navigation";
import { useShellPrefs } from "@/lib/shell/prefs";
import { useShellShortcuts } from "@/lib/shell/shortcuts";
import { useRouter, useRouterState } from "@tanstack/react-router";
import {
  Component,
  type ReactNode,
  Suspense,
  lazy,
  useCallback,
  useEffect,
  useMemo,
  useRef,
  useState,
} from "react";

// Palette dimuat lazy supaya tidak masuk initial-load budget (Req 15.4).
const CommandPalette = lazy(() => import("./CommandPalette"));

interface AppShellProps {
  children: React.ReactNode;
  /** Rendered in the topbar (the notification bell). */
  notifications?: React.ReactNode;
}

const NARROW_QUERY = "(max-width: 1023px)";

/** Gagal memuat chunk palette → tutup palette, shell tetap hidup. */
class PaletteBoundary extends Component<
  { onError: () => void; children: ReactNode },
  { failed: boolean }
> {
  state = { failed: false };
  static getDerivedStateFromError() {
    return { failed: true };
  }
  componentDidCatch() {
    this.props.onError();
  }
  render() {
    return this.state.failed ? null : this.props.children;
  }
}

function useIsNarrow(): boolean {
  const [narrow, setNarrow] = useState(() => window.matchMedia(NARROW_QUERY).matches);
  useEffect(() => {
    const mql = window.matchMedia(NARROW_QUERY);
    const onChange = (e: MediaQueryListEvent) => setNarrow(e.matches);
    setNarrow(mql.matches);
    mql.addEventListener("change", onChange);
    return () => mql.removeEventListener("change", onChange);
  }, []);
  return narrow;
}

/**
 * AppShell — the persistent layout frame for CMS.
 *
 * Grid `var(--sb-w) 1fr`: Sidebar (row-span-2) + Topbar (row 1) + scrollable <main> (row 2).
 * Rail = prefs.rail || viewport < 1024px (never written back to prefs).
 */
export function AppShell({ children, notifications }: AppShellProps) {
  const mainRef = useRef<HTMLElement>(null);
  const searchRef = useRef<HTMLButtonElement>(null);
  const [paletteOpen, setPaletteOpen] = useState(false);
  const role = useAuthStore((s) => s.user?.role ?? null);
  const prefs = useShellPrefs();
  const isNarrow = useIsNarrow();
  const router = useRouter();
  const visible = useMemo(() => getVisibleNav(role), [role]);
  const badges = useNavBadges(visible, role);

  // Move focus to main content on route change for keyboard users
  const pathname = useRouterState({ select: (s) => s.location.pathname });
  const isFirstRender = useRef(true);

  // biome-ignore lint/correctness/useExhaustiveDependencies: pathname is the intentional trigger to shift focus to <main> on route change, even though it isn't read in the body.
  useEffect(() => {
    // Skip focus shift on initial mount
    if (isFirstRender.current) {
      isFirstRender.current = false;
      return;
    }
    mainRef.current?.focus();
  }, [pathname]);

  const openPalette = useCallback(() => setPaletteOpen(true), []);
  const closePalette = useCallback(() => {
    setPaletteOpen(false);
    searchRef.current?.focus();
  }, []);
  const selectItem = useCallback(
    (item: NavItem) => {
      router.navigate({ to: item.href });
      closePalette();
    },
    [router, closePalette],
  );

  useShellShortcuts({
    onPalette: openPalette,
    onToggleRail: prefs.toggleRail,
    enabled: !paletteOpen,
  });

  const active = findActiveNavItem(visible.all, pathname);
  const rail = prefs.rail || isNarrow;

  return (
    <div
      className="grid h-screen overflow-hidden"
      style={
        {
          "--sb-w": rail ? SIDEBAR_WIDTH_RAIL : SIDEBAR_WIDTH_EXPANDED,
          gridTemplateColumns: "var(--sb-w) 1fr",
          gridTemplateRows: "auto minmax(0, 1fr)",
        } as React.CSSProperties
      }
    >
      <a
        href="#main-content"
        className="shell-focus sr-only focus:not-sr-only focus:fixed focus:left-3 focus:top-3 focus:z-50 focus:rounded-lg focus:bg-[var(--shell-accent)] focus:px-4 focus:py-2 focus:text-[var(--shell-on-accent)]"
      >
        Lewati ke konten utama
      </a>

      <div className="row-span-2 min-h-0" style={{ gridRow: "1 / span 2" }}>
        <Sidebar
          visible={visible}
          active={active}
          badges={badges}
          rail={rail}
          theme={prefs.sidebarTheme}
          collapsedGroups={prefs.collapsedGroups}
          onToggleGroup={prefs.toggleGroup}
          onToggleRail={prefs.toggleRail}
        />
      </div>

      <Topbar
        active={active}
        sidebarTheme={prefs.sidebarTheme}
        onToggleTheme={prefs.toggleTheme}
        onOpenPalette={openPalette}
        searchRef={searchRef}
        notifications={notifications}
      />

      {/* `relative` makes <main> the containing block for absolutely-positioned
          descendants (e.g. `sr-only`), so they are clipped by its scroll. */}
      <main
        ref={mainRef}
        tabIndex={-1}
        id="main-content"
        className="relative overflow-y-auto px-[var(--space-6)] py-[var(--space-6)] outline-none"
        style={{ backgroundColor: "var(--shell-canvas)" }}
      >
        {children}
      </main>

      {paletteOpen && (
        <PaletteBoundary onError={closePalette}>
          <Suspense fallback={null}>
            <CommandPalette
              items={visible.all}
              activeId={active?.id}
              onSelect={selectItem}
              onClose={closePalette}
            />
          </Suspense>
        </PaletteBoundary>
      )}
    </div>
  );
}
