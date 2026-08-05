import { ReconciliationScreen } from '@/features/reconciliation';
import { createRoute } from '@tanstack/react-router';
import { protectedRoute } from '../_protected';

export const reconciliationRoute = createRoute({
  path: '/invoice/reconciliation',
  getParentRoute: () => protectedRoute,
  component: ReconciliationScreen,
});
