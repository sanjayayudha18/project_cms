import { Navigate } from 'react-router-dom';

import { NotFound } from '@/components/NotFound';
import { DashboardScreen } from '@/features/dashboard';
import { ReplenishmentScreen } from '@/features/replenishment/ReplenishmentScreen';
import { ReconciliationScreen } from '@/features/reconciliation/ReconciliationScreen';
import { DsrDashboard } from '@/features/dsr/DsrDashboard';
import { ForecastView } from '@/features/forecast/ForecastView';
import { CashFlowScreen } from '@/features/cash-flow';
import { InvoiceFlow } from '@/features/invoice/InvoiceFlow';

function PlaceholderScreen({ title }: { title: string }) {
  return (
    <div className="flex items-center justify-center h-full">
      <p className="text-n-500 text-sm">{title} — coming soon</p>
    </div>
  );
}

export const routes = [
  { path: '/', element: <Navigate to="/dashboard" replace /> },
  { path: '/dashboard', element: <DashboardScreen /> },
  { path: '/replenishment', element: <ReplenishmentScreen /> },
  { path: '/cash-count', element: <PlaceholderScreen title="Cash Count" /> },
  { path: '/reconciliation', element: <ReconciliationScreen /> },
  { path: '/invoices', element: <InvoiceFlow /> },
  { path: '/reports', element: <DsrDashboard /> },
  { path: '/forecast', element: <ForecastView /> },
  { path: '/settings', element: <PlaceholderScreen title="Settings" /> },
  { path: '/cash-flow', element: <CashFlowScreen /> },
  { path: '*', element: <NotFound /> },
];
