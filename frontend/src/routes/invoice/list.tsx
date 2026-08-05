import { InvoiceFlow } from '@/features/invoice';
import { createRoute } from '@tanstack/react-router';
import { protectedRoute } from '../_protected';

export const invoiceListRoute = createRoute({
  path: '/invoice/list',
  getParentRoute: () => protectedRoute,
  component: InvoiceFlow,
});
