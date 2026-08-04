import { Component, type ErrorInfo, type ReactNode, useState, useEffect, useCallback } from 'react';
import { NavLink, Outlet } from 'react-router';
import { LogOut, AlertTriangle, Menu, X } from 'lucide-react';
import { useAuth } from '@/features/auth/useAuth';
import { useNotifications } from '@/features/notifications/useNotifications';
import { NotificationBadge } from '@/components/layout/NotificationBadge';
import { truncate } from '@/lib/formatters';
import { NAV_ITEMS, ROUTES } from '@/lib/constants';

// --- Error Boundary ---

interface ErrorBoundaryProps {
  readonly children: ReactNode;
}

interface ErrorBoundaryState {
  readonly hasError: boolean;
}

class ErrorBoundary extends Component<ErrorBoundaryProps, ErrorBoundaryState> {
  constructor(props: ErrorBoundaryProps) {
    super(props);
    this.state = { hasError: false };
  }

  static getDerivedStateFromError(): ErrorBoundaryState {
    return { hasError: true };
  }

  componentDidCatch(error: Error, errorInfo: ErrorInfo): void {
    console.error('ErrorBoundary caught:', error, errorInfo);
  }

  handleReset = () => {
    this.setState({ hasError: false });
  };

  render() {
    if (this.state.hasError) {
      return (
        <div className="flex flex-col items-center justify-center gap-4 p-8 min-h-[300px]">
          <AlertTriangle className="size-12 text-danger-fg" aria-hidden="true" />
          <p className="text-lg font-semibold text-surface-text">
            Terjadi kesalahan
          </p>
          <button
            type="button"
            onClick={this.handleReset}
            className="px-4 py-2 rounded-md bg-topbar text-topbar-text font-medium
                       hover:opacity-90 transition-opacity duration-150
                       focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-topbar
                       min-h-[44px] min-w-[44px]"
          >
            Muat ulang
          </button>
        </div>
      );
    }

    return this.props.children;
  }
}

// --- AppShell Layout ---

export function AppShell() {
  const { state, logout } = useAuth();
  const user = state.user;
  const [mobileOpen, setMobileOpen] = useState(false);
  const { unreadCount } = useNotifications();

  const visibleNavItems = NAV_ITEMS.filter((item) => !item.hiddenFromNav);

  const closeMobileSidebar = useCallback(() => {
    setMobileOpen(false);
  }, []);

  // Close overlay on Escape key
  useEffect(() => {
    if (!mobileOpen) return;

    function handleKeyDown(e: KeyboardEvent) {
      if (e.key === 'Escape') {
        setMobileOpen(false);
      }
    }

    document.addEventListener('keydown', handleKeyDown);
    return () => document.removeEventListener('keydown', handleKeyDown);
  }, [mobileOpen]);

  return (
    <div className="min-h-svh flex flex-col">
      {/* Top Bar */}
      <header className="fixed top-0 left-0 right-0 z-50 h-14 bg-topbar text-topbar-text flex items-center justify-between px-4">
        <div className="flex items-center gap-2">
          {/* Mobile menu toggle — visible only below lg (1024px) */}
          <button
            type="button"
            onClick={() => setMobileOpen((prev) => !prev)}
            aria-label={mobileOpen ? 'Close navigation menu' : 'Open navigation menu'}
            className="flex items-center justify-center rounded-md p-2 lg:hidden
                       hover:bg-white/10 transition-colors duration-150
                       focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-white
                       min-h-[44px] min-w-[44px]"
          >
            {mobileOpen ? (
              <X className="size-5" aria-hidden="true" />
            ) : (
              <Menu className="size-5" aria-hidden="true" />
            )}
          </button>

          <span className="font-semibold text-sm truncate max-w-[300px]">
            {user ? truncate(user.vendorName, 30) : ''}
          </span>
        </div>

        <div className="flex items-center gap-3">
          <span className="text-sm hidden sm:inline">
            {user ? truncate(user.displayName, 20) : ''}
          </span>
          <button
            type="button"
            onClick={logout}
            aria-label="Logout"
            className="flex items-center justify-center rounded-md p-2
                       hover:bg-white/10 transition-colors duration-150
                       focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-white
                       min-h-[44px] min-w-[44px]"
          >
            <LogOut className="size-5" aria-hidden="true" />
          </button>
        </div>
      </header>

      <div className="flex flex-1 pt-14">
        {/* Backdrop — visible only when mobile overlay is open */}
        {mobileOpen && (
          <div
            className="fixed inset-0 z-30 bg-black/50 lg:hidden"
            aria-hidden="true"
            onClick={closeMobileSidebar}
          />
        )}

        {/* Sidebar */}
        <nav
          aria-label="Main navigation"
          className={`fixed top-14 left-0 bottom-0 bg-sidebar overflow-y-auto z-40
                      transition-[width] duration-200 ease-out
                      ${mobileOpen ? 'w-64' : 'w-16 lg:w-64'}`}
        >
          <ul className="flex flex-col gap-1 p-2">
            {visibleNavItems.map((item) => (
              <li key={item.path}>
                <NavLink
                  to={item.path}
                  onClick={closeMobileSidebar}
                  className={({ isActive }) =>
                    `flex items-center gap-3 px-3 py-2.5 rounded-md text-sm font-medium
                     transition-colors duration-150 min-h-[44px]
                     focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-white
                     ${
                       isActive
                         ? 'bg-sidebar-active text-sidebar-active-text'
                         : 'text-sidebar-text hover:bg-white/5 hover:text-sidebar-active-text'
                     }`
                  }
                >
                  <span className="relative shrink-0">
                    <item.icon className="size-5" aria-hidden="true" />
                    {item.path === ROUTES.NOTIFICATIONS && (
                      <NotificationBadge count={unreadCount} />
                    )}
                  </span>
                  <span className={mobileOpen ? 'inline' : 'hidden lg:inline'}>
                    {item.label}
                  </span>
                </NavLink>
              </li>
            ))}
          </ul>
        </nav>

        {/* Main Content */}
        <main className="flex-1 ml-16 lg:ml-64 bg-surface p-6 overflow-y-auto">
          <ErrorBoundary>
            <Outlet />
          </ErrorBoundary>
        </main>
      </div>
    </div>
  );
}
