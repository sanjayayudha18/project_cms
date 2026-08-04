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

// Mock Recharts ResponsiveContainer to avoid ResizeObserver issues in test env
vi.mock('recharts', async () => {
  const actual = await vi.importActual<typeof import('recharts')>('recharts');
  return {
    ...actual,
    ResponsiveContainer: ({ children }: { children: React.ReactNode }) => (
      <div style={{ width: 800, height: 280 }}>{children}</div>
    ),
  };
});

// Polyfill ResizeObserver for JSDOM
class ResizeObserver {
  observe() {}
  unobserve() {}
  disconnect() {}
}
(globalThis as Record<string, unknown>).ResizeObserver = ResizeObserver;

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

describe('Cash Flow Monitoring — navigation integration', () => {
  it('Sidebar displays "Cash Flow Monitoring" link', () => {
    render(<TestApp initialRoute="/dashboard" />);

    const link = screen.getByRole('link', { name: /cash flow monitoring/i });
    expect(link).toBeInTheDocument();
  });

  it('Sidebar displays "Monitoring" group label', () => {
    render(<TestApp initialRoute="/dashboard" />);

    expect(screen.getByText('Monitoring')).toBeInTheDocument();
  });

  it('Navigating to /cash-flow renders CashFlowScreen with page title', async () => {
    render(<TestApp initialRoute="/cash-flow" />);

    // CashFlowScreen renders PageHeader with title "Cash Flow Monitoring"
    expect(
      await screen.findByRole('heading', { name: /cash flow monitoring/i }),
    ).toBeInTheDocument();
  });

  it('Clicking "Cash Flow Monitoring" link navigates to the page', async () => {
    const user = userEvent.setup();
    render(<TestApp initialRoute="/dashboard" />);

    const link = screen.getByRole('link', { name: /cash flow monitoring/i });
    await user.click(link);

    expect(
      await screen.findByRole('heading', { name: /cash flow monitoring/i }),
    ).toBeInTheDocument();
  });

  it('Active state: link has bg-[var(--red-50)] class when at /cash-flow', () => {
    render(<TestApp initialRoute="/cash-flow" />);

    const link = screen.getByRole('link', { name: /cash flow monitoring/i });
    expect(link.className).toContain('bg-[var(--red-50)]');
  });
});
