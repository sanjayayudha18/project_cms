import type { LucideIcon } from "lucide-react";
import {
  Activity,
  BarChart3,
  Calculator,
  Calendar,
  CalendarDays,
  CheckCircle,
  ClipboardCheck,
  FileOutput,
  FileText,
  GitCompare,
  LayoutDashboard,
  ListChecks,
  Receipt,
  Scale,
  Settings,
  Table,
  TrendingUp,
  Truck,
  Upload,
} from "lucide-react";

import type { Role } from "@/lib/auth/store";

// ─── Types ────────────────────────────────────────────────────────────────────

export type NavGroup = "general" | "forecasting" | "invoice" | "cash-count";

export interface NavItem {
  id: string;
  label: string;
  icon: LucideIcon;
  href: string;
  roles: (Role | "*")[];
  group: NavGroup;
  disabled?: boolean;
}

// ─── Group Labels (Bahasa Indonesia) ──────────────────────────────────────────

export const GROUP_LABELS: Record<NavGroup, string> = {
  general: "Umum",
  forecasting: "Peramalan",
  invoice: "Tagihan",
  "cash-count": "Perhitungan Kas",
};

// ─── Navigation Configuration ─────────────────────────────────────────────────

export const NAV_CONFIG: NavItem[] = [
  // General
  {
    id: "dashboard",
    label: "Dashboard",
    icon: LayoutDashboard,
    href: "/",
    roles: ["*"],
    group: "general",
  },
  {
    id: "cash-flow",
    label: "Cash Flow Monitoring",
    icon: Activity,
    href: "/cash-flow",
    roles: ["ATM_Support", "Cash_Management"],
    group: "general",
  },
  {
    id: "cit",
    label: "CIT Tracker",
    icon: Truck,
    href: "/cit",
    roles: ["ATM_Support", "Cash_Management"],
    group: "general",
  },
  {
    id: "replenishment",
    label: "Pengisian Ulang",
    icon: Truck,
    href: "/replenishment",
    roles: ["ATM_Support", "Cash_Management"],
    group: "general",
  },
  {
    id: "settings",
    label: "Pengaturan",
    icon: Settings,
    href: "/settings",
    roles: ["Admin"],
    group: "general",
  },

  // Forecasting
  {
    id: "dsr-upload",
    label: "Unggah DSR",
    icon: Upload,
    href: "/forecasting/dsr-upload",
    roles: ["Vendor", "ATM_Support"],
    group: "forecasting",
  },
  {
    id: "dsr-dashboard",
    label: "DSR Dashboard",
    icon: BarChart3,
    href: "/forecasting/dsr-dashboard",
    roles: ["ATM_Support", "Cash_Management", "Vendor"],
    group: "forecasting",
  },
  {
    id: "forecast",
    label: "Forecasting",
    icon: TrendingUp,
    href: "/forecasting/forecast",
    roles: ["ATM_Support", "Cash_Management"],
    group: "forecasting",
  },
  {
    id: "fill-instruction",
    label: "Instruksi Pengisian",
    icon: FileText,
    href: "/forecasting/fill-instruction",
    roles: ["ATM_Support", "Cash_Management", "Vendor"],
    group: "forecasting",
    disabled: true,
  },
  {
    id: "fill-validation",
    label: "Validasi Pengisian",
    icon: CheckCircle,
    href: "/forecasting/fill-validation",
    roles: ["ATM_Support", "Cash_Management"],
    group: "forecasting",
    disabled: true,
  },
  {
    id: "cash-supply",
    label: "Cash Supply",
    icon: Calculator,
    href: "/forecasting/cash-supply",
    roles: ["ATM_Support", "Cash_Management"],
    group: "forecasting",
    disabled: true,
  },
  {
    id: "h2-projection",
    label: "Proyeksi H+2",
    icon: TrendingUp,
    href: "/forecasting/h2-projection",
    roles: ["ATM_Support", "Cash_Management", "Branch"],
    group: "forecasting",
    disabled: true,
  },
  {
    id: "holiday-calendar",
    label: "Kalender Libur",
    icon: Calendar,
    href: "/forecasting/holiday-calendar",
    roles: ["ATM_Support", "Cash_Management"],
    group: "forecasting",
    disabled: true,
  },

  // Invoice
  {
    id: "invoice-list",
    label: "Daftar Invoice",
    icon: FileText,
    href: "/invoice/list",
    roles: ["WMO", "Finance", "Vendor"],
    group: "invoice",
  },
  {
    id: "invoice-upload",
    label: "Unggah Invoice",
    icon: Receipt,
    href: "/invoice/upload",
    roles: ["Vendor", "WMO"],
    group: "invoice",
    disabled: true,
  },
  {
    id: "reconciliation",
    label: "Rekonsiliasi",
    icon: GitCompare,
    href: "/invoice/reconciliation",
    roles: ["WMO", "Finance"],
    group: "invoice",
    disabled: false,
  },
  {
    id: "charge-calc",
    label: "Perhitungan Beban",
    icon: Calculator,
    href: "/invoice/charge-calculation",
    roles: ["WMO", "Finance"],
    group: "invoice",
    disabled: true,
  },
  {
    id: "doc-gen",
    label: "Pembuatan Dokumen",
    icon: FileOutput,
    href: "/invoice/documents",
    roles: ["WMO", "Finance"],
    group: "invoice",
    disabled: true,
  },

  // Cash Count
  {
    id: "scheduling",
    label: "Penjadwalan",
    icon: CalendarDays,
    href: "/cash-count/scheduling",
    roles: ["Cash_Count_Lead", "Cash_Count_PIC"],
    group: "cash-count",
    disabled: true,
  },
  {
    id: "tier-analysis",
    label: "Analisis Tier Saldo",
    icon: BarChart3,
    href: "/cash-count/tier-analysis",
    roles: ["Cash_Count_Lead"],
    group: "cash-count",
    disabled: true,
  },
  {
    id: "execution",
    label: "Pelaksanaan (BA)",
    icon: ClipboardCheck,
    href: "/cash-count/execution",
    roles: ["Cash_Count_PIC"],
    group: "cash-count",
    disabled: true,
  },
  {
    id: "checklists",
    label: "Checklist",
    icon: ListChecks,
    href: "/cash-count/checklists",
    roles: ["Cash_Count_PIC"],
    group: "cash-count",
    disabled: true,
  },
  {
    id: "cc-reconciliation",
    label: "Rekonsiliasi",
    icon: Scale,
    href: "/cash-count/reconciliation",
    roles: ["Cash_Count_Lead", "Cash_Count_PIC"],
    group: "cash-count",
    disabled: true,
  },
  {
    id: "recapitulation",
    label: "Rekapitulasi",
    icon: Table,
    href: "/cash-count/recapitulation",
    roles: ["Cash_Count_Lead"],
    group: "cash-count",
    disabled: true,
  },
];

// ─── Utilities ────────────────────────────────────────────────────────────────

/**
 * Filters navigation items based on user roles.
 * Items with `roles: ['*']` are visible to all authenticated users.
 * Users with the 'Admin' role see all navigation items.
 * Items are included if the user has at least one matching role.
 */
export function filterNavByRoles(items: NavItem[], userRoles: Role[]): NavItem[] {
  if (userRoles.length === 0) return [];

  // Admin sees everything
  if (userRoles.includes("Admin")) return items;

  return items.filter((item) => {
    if (item.roles.includes("*")) return true;
    return item.roles.some((role) => userRoles.includes(role as Role));
  });
}
