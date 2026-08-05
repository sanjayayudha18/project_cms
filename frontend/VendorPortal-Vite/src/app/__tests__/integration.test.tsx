import { render, screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { createMemoryRouter, RouterProvider } from 'react-router';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { AuthProvider, AuthContext, type AuthContextValue } from '@/features/auth/AuthContext';
import { AppShell } from '@/app/AppShell';
import { ProtectedRoute, GuestRoute } from '@/features/auth/ProtectedRoute';
import { LoginPage } from '@/features/auth/LoginPage';
import { OrdersPage } from '@/features/orders/OrdersPage';
import { InvoicesPage } from '@/features/invoices/InvoicesPage';
import { SchedulePage } from '@/features/schedule/SchedulePage';
import { DsrPage } from '@/features/dsr/DsrPage';
import { NotificationsPage } from '@/features/notifications/NotificationsPage';
import { NotFound } from '@/components/NotFound';
import { Navigate } from 'react-router';
import { type ReactNode, useMemo } from 'react';
import ordersData from '@/data/orders.json';
import type { AuthState, CITOrder, VendorUser } from '@/lib/types';

// --- Test Auth Provider (pre-authenticated) ---

const mockUser: VendorUser = {
  id: 'user-1',
  username: 'gardanet.admin',
  displayName: 'Budi Santoso',
  vendorId: 'vendor-gardanet',
  vendorName: 'PT Gardanet',
  role: 'Vendor',
};

function TestAuthProvider({ children }: { children: ReactNode }) {
  const state: AuthState = useMemo(
    () => ({ token: 'fake-jwt', user: mockUser, isAuthenticated: true }),
    [],
  );

  const value: AuthContextValue = useMemo(
    () => ({
      state,
      login: async () => {},
      logout: () => {},
    }),
    [state],
  );

  return <AuthContext.Provider value={value}>{children}</AuthContext.Provider>;
}

// --- Helper: render app with full provider stack ---

function renderApp(initialPath: string = '/') {
  const queryClient = new QueryClient({
    defaultOptions: {
      queries: { retry: false, gcTime: 0 },
    },
  });

  const routes = [
    { path: '/', element: <Navigate to="/orders" replace /> },
    {
      element: <GuestRoute />,
      children: [{ path: '/login', element: <LoginPage /> }],
    },
    {
      element: <ProtectedRoute />,
      children: [
        {
          element: <AppShell />,
          children: [
            { path: '/orders', element: <OrdersPage /> },
            { path: '/invoices', element: <InvoicesPage /> },
            { path: '/schedule', element: <SchedulePage /> },
            { path: '/dsr', element: <DsrPage /> },
            { path: '/notifications', element: <NotificationsPage /> },
          ],
        },
      ],
    },
    { path: '*', element: <NotFound /> },
  ];

  const router = createMemoryRouter(routes, {
    initialEntries: [initialPath],
  });

  const result = render(
    <QueryClientProvider client={queryClient}>
      <AuthProvider>
        <RouterProvider router={router} />
      </AuthProvider>
    </QueryClientProvider>,
  );

  return { ...result, router, queryClient };
}

// --- Helper: perform login ---

async function performLogin(
  user: ReturnType<typeof userEvent.setup>,
  username: string,
  password: string,
) {
  const usernameInput = screen.getByLabelText('Username');
  const passwordInput = screen.getByLabelText('Password');
  const submitButton = screen.getByRole('button', { name: /masuk/i });

  await user.type(usernameInput, username);
  await user.type(passwordInput, password);
  await user.click(submitButton);
}

// ===== Test 1: Login → redirect → data display =====

describe('Login → redirect → data display flow', () => {
  it('redirects unauthenticated user to login, then shows orders after login', async () => {
    const user = userEvent.setup();
    renderApp('/orders');

    // Should redirect to login since unauthenticated
    await waitFor(() => {
      expect(screen.getByLabelText('Username')).toBeInTheDocument();
    });

    // Fill in credentials and submit
    await performLogin(user, 'gardanet.admin', 'password123');

    // Should redirect to orders page and display CIT Orders heading
    await waitFor(() => {
      expect(screen.getByRole('heading', { name: /cit orders/i })).toBeInTheDocument();
    });

    // Verify orders data is displayed (Gardanet ATM IDs visible)
    await waitFor(() => {
      expect(screen.getByText('ATM-JKT-001')).toBeInTheDocument();
    });
  });

  it('preserves intended URL after login redirect', async () => {
    const user = userEvent.setup();

    // Use a custom router without GuestRoute wrapper so login page's navigate(from)
    // is not raced by the GuestRoute's authenticated redirect.
    const queryClient = new QueryClient({
      defaultOptions: { queries: { retry: false, gcTime: 0 } },
    });

    const routes = [
      { path: '/login', element: <LoginPage /> },
      {
        element: <ProtectedRoute />,
        children: [
          {
            element: <AppShell />,
            children: [
              { path: '/orders', element: <OrdersPage /> },
              { path: '/invoices', element: <InvoicesPage /> },
            ],
          },
        ],
      },
    ];

    const router = createMemoryRouter(routes, {
      initialEntries: ['/invoices'],
    });

    render(
      <QueryClientProvider client={queryClient}>
        <AuthProvider>
          <RouterProvider router={router} />
        </AuthProvider>
      </QueryClientProvider>,
    );

    // Should redirect to login (ProtectedRoute preserves /invoices in state)
    await waitFor(() => {
      expect(screen.getByLabelText('Username')).toBeInTheDocument();
    });

    // Login
    await performLogin(user, 'gardanet.admin', 'password123');

    // Should redirect to originally requested /invoices page
    await waitFor(() => {
      expect(screen.getByRole('heading', { name: /invoices/i })).toBeInTheDocument();
    });
  });
});

// ===== Test 2: Vendor scoping =====

describe('Vendor scoping', () => {
  it('logged in as Gardanet shows only Gardanet data, no SSI or G4S data', async () => {
    const user = userEvent.setup();
    renderApp('/orders');

    // Login as gardanet.admin
    await waitFor(() => {
      expect(screen.getByLabelText('Username')).toBeInTheDocument();
    });
    await performLogin(user, 'gardanet.admin', 'password123');

    // Wait for orders page to render
    await waitFor(() => {
      expect(screen.getByRole('heading', { name: /cit orders/i })).toBeInTheDocument();
    });

    // Wait for data to load — check a known Gardanet ATM ID
    await waitFor(() => {
      expect(screen.getByText('ATM-JKT-001')).toBeInTheDocument();
    });

    // Verify Gardanet ATM IDs are present (first 3 Gardanet orders)
    const gardanetOrders = (ordersData as CITOrder[]).filter(
      (o) => o.vendorId === 'vendor-gardanet',
    );
    for (const order of gardanetOrders.slice(0, 3)) {
      expect(screen.getByText(order.atmId)).toBeInTheDocument();
    }

    // Verify SSI ATM IDs are NOT visible (ATM-SBY-001 belongs to SSI)
    expect(screen.queryByText('ATM-SBY-001')).not.toBeInTheDocument();
    // Verify G4S ATM IDs are NOT visible (ATM-SMG-001 belongs to G4S)
    expect(screen.queryByText('ATM-SMG-001')).not.toBeInTheDocument();
  });
});

// ===== Test 3: Navigation between all screens =====

describe('Navigation between screens', () => {
  it('can navigate to each screen via sidebar links', async () => {
    const user = userEvent.setup();
    renderApp('/orders');

    // Login first
    await waitFor(() => {
      expect(screen.getByLabelText('Username')).toBeInTheDocument();
    });
    await performLogin(user, 'gardanet.admin', 'password123');

    // Wait for initial page (orders)
    await waitFor(() => {
      expect(screen.getByRole('heading', { name: /cit orders/i })).toBeInTheDocument();
    });

    // Navigate to Invoices
    const invoicesLink = screen.getByRole('link', { name: /invoices/i });
    await user.click(invoicesLink);
    await waitFor(() => {
      expect(screen.getByRole('heading', { name: /invoices/i })).toBeInTheDocument();
    });

    // Navigate to Replenishment Schedule
    const scheduleLink = screen.getByRole('link', { name: /replenishment schedule/i });
    await user.click(scheduleLink);
    await waitFor(() => {
      expect(screen.getByRole('heading', { name: /replenishment schedule/i })).toBeInTheDocument();
    });

    // Navigate to DSR Monitor
    const dsrLink = screen.getByRole('link', { name: /dsr monitor/i });
    await user.click(dsrLink);
    await waitFor(() => {
      expect(screen.getByRole('heading', { name: /dsr monitor/i })).toBeInTheDocument();
    });

    // Navigate to Notifications
    const notificationsLink = screen.getByRole('link', { name: /notifications/i });
    await user.click(notificationsLink);
    await waitFor(() => {
      expect(screen.getByRole('heading', { name: /notifications/i })).toBeInTheDocument();
    });

    // Navigate back to CIT Orders
    const ordersLink = screen.getByRole('link', { name: /cit orders/i });
    await user.click(ordersLink);
    await waitFor(() => {
      expect(screen.getByRole('heading', { name: /cit orders/i })).toBeInTheDocument();
    });
  });
});

// ===== Test 4: Error boundary fallback rendering =====

describe('Error boundary fallback', () => {
  function ThrowingComponent(): ReactNode {
    throw new Error('Test error');
  }

  it('catches errors and shows "Terjadi kesalahan" fallback', async () => {
    const consoleSpy = vi.spyOn(console, 'error').mockImplementation(() => {});

    const queryClient = new QueryClient({
      defaultOptions: { queries: { retry: false } },
    });

    const routes = [
      {
        element: <AppShell />,
        children: [{ path: '/', element: <ThrowingComponent /> }],
      },
    ];

    const router = createMemoryRouter(routes, {
      initialEntries: ['/'],
    });

    render(
      <QueryClientProvider client={queryClient}>
        <TestAuthProvider>
          <RouterProvider router={router} />
        </TestAuthProvider>
      </QueryClientProvider>,
    );

    // Should show error boundary fallback
    await waitFor(() => {
      expect(screen.getByText('Terjadi kesalahan')).toBeInTheDocument();
    });

    // Should show the reload button
    expect(screen.getByRole('button', { name: /muat ulang/i })).toBeInTheDocument();

    consoleSpy.mockRestore();
  });

  it('"Muat ulang" button resets the error boundary', async () => {
    const consoleSpy = vi.spyOn(console, 'error').mockImplementation(() => {});
    const user = userEvent.setup();

    let shouldThrow = true;

    function ConditionalThrower() {
      if (shouldThrow) throw new Error('Test error');
      return <p>Content recovered</p>;
    }

    const queryClient = new QueryClient({
      defaultOptions: { queries: { retry: false } },
    });

    const routes = [
      {
        element: <AppShell />,
        children: [{ path: '/', element: <ConditionalThrower /> }],
      },
    ];

    const router = createMemoryRouter(routes, {
      initialEntries: ['/'],
    });

    render(
      <QueryClientProvider client={queryClient}>
        <TestAuthProvider>
          <RouterProvider router={router} />
        </TestAuthProvider>
      </QueryClientProvider>,
    );

    // Error boundary should catch
    await waitFor(() => {
      expect(screen.getByText('Terjadi kesalahan')).toBeInTheDocument();
    });

    // Stop throwing before clicking reset
    shouldThrow = false;

    // Click "Muat ulang" to reset the boundary
    const reloadButton = screen.getByRole('button', { name: /muat ulang/i });
    await user.click(reloadButton);

    // Content should recover
    await waitFor(() => {
      expect(screen.getByText('Content recovered')).toBeInTheDocument();
    });
    expect(screen.queryByText('Terjadi kesalahan')).not.toBeInTheDocument();

    consoleSpy.mockRestore();
  });
});
