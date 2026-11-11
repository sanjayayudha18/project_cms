import { render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { MemoryRouter, useRoutes } from 'react-router-dom';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { describe, it, expect, beforeAll, vi } from 'vitest';

import { RoleProvider } from '@/context/RoleContext';
import { AppShell } from '@/app/AppShell';
import { routes } from '@/app/routes';

// Mock matchMedia — required by Sidebar auto-collapse logic
beforeAll(() => {
  Object.defineProperty(window, 'matchMedia', {
    writable: true,
    value: vi.fn().mockImplementation((query: string) => ({
      matches: false,
      media: query,
      onchange: null,
      addListener: vi.fn(),
      removeListener: vi.fn(),
      addEventListener: vi.fn(),
      removeEventListener: vi.fn(),
      dispatchEvent: vi.fn(),
    })),
  });
});

function createTestQueryClient() {
  return new QueryClient({
    defaultOptions: {
      queries: { staleTime: Infinity, retry: false },
    },
  });
}

function TestApp({ initialRoute }: { initialRoute: string }) {
  function Routes() {
    return useRoutes([{ element: <AppShell />, children: routes }]);
  }
  return (
    <QueryClientProvider client={createTestQueryClient()}>
      <RoleProvider>
        <MemoryRouter initialEntries={[initialRoute]}>
          <Routes />
        </MemoryRouter>
      </RoleProvider>
    </QueryClientProvider>
  );
}

describe('App integration — navigation and toasts', () => {
  it('Root redirects to /dashboard', () => {
    render(<TestApp initialRoute="/" />);
    // DashboardScreen renders the greeting heading
    expect(screen.getByText(/here's what's happening/i)).toBeInTheDocument();
  });

  it('Clicking sidebar nav navigates to correct route', async () => {
    const user = userEvent.setup();
    render(<TestApp initialRoute="/dashboard" />);

    const replenishmentLink = screen.getByRole('link', { name: /replenishment/i });
    await user.click(replenishmentLink);

    // ReplenishmentScreen has a page header with title "Replenishment"
    expect(screen.getByRole('heading', { name: /replenishment/i })).toBeInTheDocument();
  });

  it('Active sidebar item has correct styling', () => {
    render(<TestApp initialRoute="/dashboard" />);

    const dashboardLink = screen.getByRole('link', { name: /dashboard/i });
    expect(dashboardLink.className).toContain('bg-[var(--red-50)]');
  });

  it('Toast: "New schedule" on Dashboard triggers toast', async () => {
    const user = userEvent.setup();
    render(<TestApp initialRoute="/dashboard" />);

    const newScheduleBtn = screen.getByRole('button', { name: /new schedule/i });
    await user.click(newScheduleBtn);

    expect(await screen.findByText('Schedule created successfully')).toBeInTheDocument();
  });

  it('Toast: "Run reconciliation" on Reconciliation triggers toast', async () => {
    const user = userEvent.setup();
    render(<TestApp initialRoute="/reconciliation" />);

    const runBtn = screen.getByRole('button', { name: /run reconciliation/i });
    await user.click(runBtn);

    expect(await screen.findByText('Reconciliation initiated')).toBeInTheDocument();
  });

  it('Unknown route shows NotFound', () => {
    render(<TestApp initialRoute="/unknown" />);
    expect(screen.getByText('Page not found')).toBeInTheDocument();
  });
});
