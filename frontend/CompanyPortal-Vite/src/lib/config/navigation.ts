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

import type { DbRole } from "@/lib/auth/store";

// ─── Types ────────────────────────────────────────────────────────────────────

export type NavGroup = "general" | "forecasting" | "invoice" | "cash-count";

export interface NavItem {
  id: string;
  label: string;
  icon: LucideIcon;
  href: string;
  roles: (DbRole | "*")[];
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
    roles: ["ATM-USER", "ATM-SPV"],
    group: "general",
  },
  {
    id: "cit",
    label: "CIT Tracker",
    icon: Truck,
    href: "/cit",
    roles: ["ATM-USER", "ATM-SPV"],
    group: "general",
  },
  {
    id: "replenishment",
    label: "Pengisian Ulang",
    icon: Truck,
    href: "/replenishment",
    roles: ["ATM-USER", "ATM-SPV"],
    group: "general",
  },
  {
    id: "settings",
    label: "Pengaturan",
    icon: Settings,
    href: "/settings",
    roles: ["ADMIN"],
    group: "general",
  },

  // Forecasting
  {
    id: "dsr-upload",
    label: "Unggah DSR",
    icon: Upload,
    href: "/forecasting/dsr-upload",
    roles: ["VENDOR-USER", "ATM-USER", "ATM-SPV"],
    group: "forecasting",
  },
  {
    id: "dsr-dashboard",
    label: "DSR Dashboard",
    icon: BarChart3,
    href: "/forecasting/dsr-dashboard",
    roles: ["ATM-USER", "ATM-SPV", "VENDOR-USER"],
    group: "forecasting",
  },
  {
    id: "forecast",
    label: "Forecasting",
    icon: TrendingUp,
    href: "/forecasting/forecast",
    roles: ["ATM-USER", "ATM-SPV"],
    group: "forecasting",
  },
  {
    id: "fill-instruction",
    label: "Instruksi Pengisian",
    icon: FileText,
    href: "/forecasting/fill-instruction",
    roles: ["ATM-USER", "ATM-SPV", "VENDOR-USER"],
    group: "forecasting",
    disabled: true,
  },
  {
    id: "fill-validation",
    label: "Validasi Pengisian",
    icon: CheckCircle,
    href: "/forecasting/fill-validation",
    roles: ["ATM-USER", "ATM-SPV"],
    group: "forecasting",
    disabled: true,
  },
  {
    id: "cash-supply",
    label: "Cash Supply",
    icon: Calculator,
    href: "/forecasting/cash-supply",
    roles: ["ATM-USER", "ATM-SPV"],
    group: "forecasting",
    disabled: true,
  },
  {
    id: "h2-projection",
    label: "Proyeksi H+2",
    icon: TrendingUp,
    href: "/forecasting/h2-projection",
    roles: ["ATM-USER", "ATM-SPV", "BRANCH-USER", "BRANCH-SPV", "BRANCH-ATM-USER", "BRANCH-ATM-SPV"],
    group: "forecasting",
    disabled: true,
  },
  {
    id: "holiday-calendar",
    label: "Kalender Libur",
    icon: Calendar,
    href: "/forecasting/holiday-calendar",
    roles: ["ATM-USER", "ATM-SPV"],
    group: "forecasting",
    disabled: true,
  },

  // Invoice
  {
    id: "invoice-list",
    label: "Daftar Invoice",
    icon: FileText,
    href: "/invoice/list",
    roles: ["BRANCH-USER", "BRANCH-SPV", "VENDOR-USER"],
    group: "invoice",
  },
  {
    id: "invoice-upload",
    label: "Unggah Invoice",
    icon: Receipt,
    href: "/invoice/upload",
    roles: ["VENDOR-USER", "BRANCH-USER"],
    group: "invoice",
    disabled: true,
  },
  {
    id: "reconciliation",
    label: "Rekonsiliasi",
    icon: GitCompare,
    href: "/invoice/reconciliation",
    roles: ["BRANCH-USER", "BRANCH-SPV"],
    group: "invoice",
    disabled: false,
  },
  {
    id: "charge-calc",
    label: "Perhitungan Beban",
    icon: Calculator,
    href: "/invoice/charge-calculation",
    roles: ["BRANCH-USER", "BRANCH-SPV"],
    group: "invoice",
    disabled: true,
  },
  {
    id: "doc-gen",
    label: "Pembuatan Dokumen",
    icon: FileOutput,
    href: "/invoice/documents",
    roles: ["BRANCH-USER", "BRANCH-SPV"],
    group: "invoice",
    disabled: true,
  },

  // Cash Count
  {
    id: "scheduling",
    label: "Penjadwalan",
    icon: CalendarDays,
    href: "/cash-count/scheduling",
    roles: ["BRANCH-ATM-SPV", "BRANCH-ATM-USER"],
    group: "cash-count",
    disabled: true,
  },
  {
    id: "tier-analysis",
    label: "Analisis Tier Saldo",
    icon: BarChart3,
    href: "/cash-count/tier-analysis",
    roles: ["BRANCH-ATM-SPV"],
    group: "cash-count",
    disabled: true,
  },
  {
    id: "execution",
    label: "Pelaksanaan (BA)",
    icon: ClipboardCheck,
    href: "/cash-count/execution",
    roles: ["BRANCH-ATM-USER"],
    group: "cash-count",
    disabled: true,
  },
  {
    id: "checklists",
    label: "Checklist",
    icon: ListChecks,
    href: "/cash-count/checklists",
    roles: ["BRANCH-ATM-USER"],
    group: "cash-count",
    disabled: true,
  },
  {
    id: "cc-reconciliation",
    label: "Rekonsiliasi",
    icon: Scale,
    href: "/cash-count/reconciliation",
    roles: ["BRANCH-ATM-SPV", "BRANCH-ATM-USER"],
    group: "cash-count",
    disabled: true,
  },
  {
    id: "recapitulation",
    label: "Rekapitulasi",
    icon: Table,
    href: "/cash-count/recapitulation",
    roles: ["BRANCH-ATM-SPV"],
    group: "cash-count",
    disabled: true,
  },
];

// ─── Utilities ────────────────────────────────────────────────────────────────

/**
 * Filters navigation items based on user role.
 * Items with `roles: ['*']` are visible to all authenticated users.
 * Users with 'ADMIN' or 'ADMIN_PARAM' role see all navigation items.
 * Items are included if the user's role is in the item's allowed list.
 */
export function filterNavByRoles(items: NavItem[], userRole: DbRole): NavItem[] {
  // ADMIN and ADMIN_PARAM see everything
  if (userRole === "ADMIN" || userRole === "ADMIN_PARAM") return items;

  return items.filter((item) => {
    if (item.roles.includes("*")) return true;
    return item.roles.includes(userRole);
  });
}
