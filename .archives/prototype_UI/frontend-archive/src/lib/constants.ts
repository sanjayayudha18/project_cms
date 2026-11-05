import type { LucideIcon } from 'lucide-react';
import {
  Activity,
  BarChart3,
  Calculator,
  FileText,
  LayoutDashboard,
  Scale,
  Settings,
  TrendingUp,
  Truck,
} from 'lucide-react';

// --- Role types ---

export type Role = 'Admin' | 'Operator' | 'Manager' | 'Vendor';

export const ROLES: readonly Role[] = ['Admin', 'Operator', 'Manager', 'Vendor'] as const;

export const INTERNAL_ROLES: readonly Role[] = ['Admin', 'Operator', 'Manager'] as const;

// --- Status types ---

export type DsrStatus = 'Critical' | 'Low' | 'Normal';

export type CitStatus = 'Scheduled' | 'In Transit' | 'Completed' | 'Failed';

export type Priority = 'High' | 'Medium' | 'Low';

export type ValidationStatus = 'Uploaded' | 'Validated' | 'Approved' | 'Mismatch Detected';

export type MatchStatus = 'Matched' | 'Mismatch' | 'Pending Review';

// --- Navigation ---

export interface NavItem {
  path: string;
  label: string;
  icon: LucideIcon;
  badge?: number;
}

export interface NavGroup {
  label: string;
  items: NavItem[];
}

export const NAV_GROUPS: NavGroup[] = [
  {
    label: 'Monitoring',
    items: [
      { path: '/dashboard', label: 'Dashboard', icon: LayoutDashboard },
      { path: '/cash-flow', label: 'Cash Flow Monitoring', icon: Activity },
    ],
  },
  {
    label: 'Operations',
    items: [
      { path: '/replenishment', label: 'Replenishment', icon: Truck },
      { path: '/cash-count', label: 'Cash Count', icon: Calculator },
      { path: '/reconciliation', label: 'Reconciliation', icon: Scale },
    ],
  },
  {
    label: 'Control',
    items: [
      { path: '/invoices', label: 'Vendor Invoices', icon: FileText },
      { path: '/reports', label: 'DSR Reports', icon: BarChart3 },
      { path: '/forecast', label: 'Forecasting', icon: TrendingUp },
    ],
  },
];

export const SETTINGS_NAV: NavItem = {
  path: '/settings',
  label: 'Settings',
  icon: Settings,
};

// --- Navigation helpers ---

/**
 * Returns all navigation items flattened from all groups.
 */
export function getAllNavItems(): NavItem[] {
  return NAV_GROUPS.flatMap((group) => group.items);
}

/**
 * Filters navigation items based on the user's role.
 * Internal roles (Admin, Operator, Manager) see all items.
 * Vendor role sees only items where internalOnly is false.
 *
 * @deprecated Use NAV_GROUPS directly — the new navigation structure
 * does not use role-based filtering for visibility.
 */
export function filterNavByRole(items: Array<NavItem & { internalOnly?: boolean }>, role: Role): Array<NavItem & { internalOnly?: boolean }> {
  const isInternal = (INTERNAL_ROLES as readonly string[]).includes(role);
  if (isInternal) {
    return items;
  }
  return items.filter((item) => !(item as { internalOnly?: boolean }).internalOnly);
}
