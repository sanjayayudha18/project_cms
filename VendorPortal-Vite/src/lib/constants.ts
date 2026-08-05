import type { LucideIcon } from 'lucide-react';
import {
  ClipboardList,
  FileUp,
  FileText,
  Calendar,
  Activity,
  Bell,
} from 'lucide-react';

// --- Route Paths ---

export const ROUTES = {
  LOGIN: '/login',
  ORDERS: '/orders',
  EVIDENCE: '/orders/:id/evidence',
  INVOICES: '/invoices',
  SCHEDULE: '/schedule',
  DSR: '/dsr',
  NOTIFICATIONS: '/notifications',
} as const;

// --- Navigation Items ---

export interface NavItem {
  readonly label: string;
  readonly path: string;
  readonly icon: LucideIcon;
  readonly hiddenFromNav?: boolean;
}

export const NAV_ITEMS: readonly NavItem[] = [
  { label: 'DSR Menu', path: ROUTES.DSR, icon: Activity },
  { label: 'CIT Orders', path: ROUTES.ORDERS, icon: ClipboardList },
  { label: 'Upload Evidence', path: ROUTES.EVIDENCE, icon: FileUp, hiddenFromNav: true },
  { label: 'Invoices', path: ROUTES.INVOICES, icon: FileText },
  { label: 'Replenishment Schedule', path: ROUTES.SCHEDULE, icon: Calendar },
  { label: 'Notifications', path: ROUTES.NOTIFICATIONS, icon: Bell },
] as const;

// --- Balance Thresholds (IDR) ---

export const CRITICAL_THRESHOLD = 50_000_000;
export const LOW_THRESHOLD = 150_000_000;
