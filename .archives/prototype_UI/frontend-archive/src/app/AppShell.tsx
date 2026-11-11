import { useState, useEffect } from 'react';
import { Outlet, useLocation } from 'react-router-dom';

import { Sidebar } from '@/components/layout/Sidebar';
import { TopBar } from '@/components/layout/TopBar';
import { DevRoleSwitcher } from '@/components/dev/DevRoleSwitcher';
import { ToastProvider } from '@/context/ToastContext';

export function AppShell() {
  const [collapsed, setCollapsed] = useState(() => {
    // Auto-collapse on narrow viewports
    return window.matchMedia('(max-width: 1023px)').matches;
  });

  const [mobileMenuOpen, setMobileMenuOpen] = useState(false);
  const location = useLocation();

  // Auto-collapse sidebar when viewport shrinks
  useEffect(() => {
    const mql = window.matchMedia('(max-width: 1023px)');
    const handler = (e: MediaQueryListEvent) => {
      setCollapsed(e.matches);
    };
    mql.addEventListener('change', handler);
    return () => mql.removeEventListener('change', handler);
  }, []);

  // Close mobile sidebar on route change
  useEffect(() => {
    setMobileMenuOpen(false);
  }, [location.pathname]);

  return (
    <ToastProvider>
      <div className="flex h-screen overflow-hidden">
        <Sidebar
          collapsed={collapsed}
          onToggle={() => setCollapsed((c) => !c)}
          mobileOpen={mobileMenuOpen}
          onMobileClose={() => setMobileMenuOpen(false)}
        />

        <div className="flex flex-1 flex-col overflow-hidden">
          <TopBar onMenuClick={() => setMobileMenuOpen((o) => !o)} />

          <main className="flex-1 overflow-y-auto bg-n-50 px-4 min-[760px]:px-6">
            <div key={location.pathname} className="page-enter">
              <Outlet />
            </div>
          </main>
        </div>
      </div>

      <DevRoleSwitcher />
    </ToastProvider>
  );
}
