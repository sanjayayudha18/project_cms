import { createBrowserRouter, Navigate } from 'react-router';
import { AppShell } from '@/app/AppShell';
import { ProtectedRoute, GuestRoute } from '@/features/auth/ProtectedRoute';
import { LoginPage } from '@/features/auth/LoginPage';
import { OrdersPage } from '@/features/orders/OrdersPage';
import { EvidencePage } from '@/features/evidence/EvidencePage';
import { InvoicesPage } from '@/features/invoices/InvoicesPage';
import { SchedulePage } from '@/features/schedule/SchedulePage';
import { DsrPage } from '@/features/dsr/DsrPage';
import { NotificationsPage } from '@/features/notifications/NotificationsPage';
import { NotFound } from '@/components/NotFound';

export const router = createBrowserRouter([
  // Root redirect to /orders
  {
    path: '/',
    element: <Navigate to="/orders" replace />,
  },

  // Guest-only route (login)
  {
    element: <GuestRoute />,
    children: [
      {
        path: '/login',
        element: <LoginPage />,
      },
    ],
  },

  // Protected routes wrapped in AppShell
  {
    element: <ProtectedRoute />,
    children: [
      {
        element: <AppShell />,
        children: [
          { path: '/orders', element: <OrdersPage /> },
          { path: '/orders/:id/evidence', element: <EvidencePage /> },
          { path: '/invoices', element: <InvoicesPage /> },
          { path: '/schedule', element: <SchedulePage /> },
          { path: '/dsr', element: <DsrPage /> },
          { path: '/notifications', element: <NotificationsPage /> },
        ],
      },
    ],
  },

  // Internal-only routes — show NotFound (don't reveal they exist internally)
  { path: '/admin', element: <NotFound /> },
  { path: '/forecasting', element: <NotFound /> },
  { path: '/reconciliation', element: <NotFound /> },

  // Catch-all for unknown routes
  { path: '*', element: <NotFound /> },
]);
