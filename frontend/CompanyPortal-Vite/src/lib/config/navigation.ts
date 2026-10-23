import type { LucideIcon } from "lucide-react";
import {
  Activity,
  BarChart3,
  CalendarDays,
  ClipboardCheck,
  ClipboardList,
  HeartPulse,
  Landmark,
  LayoutGrid,
  Monitor,
  Receipt,
  ScrollText,
  Search,
  Settings2,
  Table,
  Truck,
} from "lucide-react";

import type { DbRole } from "@/lib/auth/store";

// ─── Types ────────────────────────────────────────────────────────────────────

export type NavGroup =
  | "general"
  | "monitoring"
  | "forecasting"
  | "replenish"
  | "cit"
  | "invoice"
  | "cash-count"
  | "system";

export type NavPlacement = "sidebar" | "topbar";
export type NavBadgeSource = "approvals-inbox" | "cit-requests" | "eod-health";

export interface NavItem {
  id: string;
  label: string;
  icon: LucideIcon;
  href: string;
  roles: (DbRole | "*")[];
  group: NavGroup;
  disabled?: boolean;
  /** Default "sidebar". */
  placement?: NavPlacement;
  /** Sumber NavBadge (bukan nilai). */
  badge?: NavBadgeSource;
}

// ─── Group Labels (Bahasa Indonesia) ──────────────────────────────────────────

export const GROUP_LABELS: Record<NavGroup, string> = {
  general: "Umum",
  monitoring: "Monitoring",
  forecasting: "Peramalan",
  replenish: "Replenish",
  cit: "CIT",
  invoice: "Tagihan",
  "cash-count": "Perhitungan Kas",
  system: "Sistem",
};

/** Grup yang tampil di sidebar, berurutan. */
export const SIDEBAR_GROUP_ORDER: NavGroup[] = [
  "general",
  "replenish",
  "cit",
  "monitoring",
  "forecasting",
];

// ─── Navigation Configuration ─────────────────────────────────────────────────

export const NAV_CONFIG: NavItem[] = [
  // General
  {
    id: "dashboard",
    label: "Dashboard",
    icon: LayoutGrid,
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
    id: "approvals",
    label: "Persetujuan",
    icon: ClipboardCheck,
    href: "/approvals",
    roles: ["*"],
    group: "general",
    badge: "approvals-inbox",
  },
  {
    id: "settings",
    label: "Pengaturan",
    icon: Settings2,
    href: "/settings",
    roles: ["ADMIN", "ADMIN_PARAM", "APPACCESS"],
    group: "system",
    placement: "topbar",
  },

  // Replenish
  {
    id: "forecast-browser",
    label: "Rekomendasi CIT",
    icon: Search,
    href: "/replenishment/forecast-browser",
    roles: ["ATM-USER", "ATM-SPV", "BRANCH-ATM-USER", "BRANCH-ATM-SPV"],
    group: "replenish",
  },
  {
    id: "vendor-requests",
    label: "Request CIT",
    icon: ClipboardList,
    href: "/replenishment/vendor-requests",
    roles: ["ATM-USER", "ATM-SPV", "BRANCH-ATM-USER", "BRANCH-ATM-SPV"],
    group: "replenish",
    badge: "cit-requests",
  },
  {
    id: "replenishment",
    label: "Jadwal CIT",
    icon: Truck,
    href: "/replenishment",
    roles: ["ATM-USER", "ATM-SPV"],
    group: "replenish",
  },
  // CIT (cit-acm-plan): tim ACM menetapkan branch vault per ATM.
  {
    id: "vault-plans",
    label: "Penetapan Vault",
    icon: Landmark,
    href: "/cit/vault-plans",
    roles: ["ACM-USER", "ACM-SPV"],
    group: "cit",
  },
  // Monitoring
  {
    id: "atm-portal",
    label: "ATM Portal",
    icon: Monitor,
    href: "/atm-portal",
    roles: ["ATM-USER", "ATM-SPV"],
    group: "monitoring",
  },
  {
    id: "eod-monitoring",
    label: "EOD Monitoring",
    icon: HeartPulse,
    href: "/eod-monitoring",
    roles: ["ADMIN", "ADMIN_PARAM"],
    group: "monitoring",
    badge: "eod-health",
  },
  {
    id: "audit-logs",
    label: "Log Audit",
    icon: ScrollText,
    href: "/audit-logs",
    roles: ["ADMIN", "ADMIN_PARAM"],
    group: "monitoring",
  },

  {
    id: "dmaa-forecast",
    label: "DMAA Forecast",
    icon: BarChart3,
    href: "/forecasting/dmaa-forecast",
    roles: ["ATM-USER", "ATM-SPV", "BRANCH-ATM-USER", "BRANCH-ATM-SPV"],
    group: "forecasting",
  },
  /*  {
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
    roles: [
      "ATM-USER",
      "ATM-SPV",
      "BRANCH-USER",
      "BRANCH-SPV",
      "BRANCH-ATM-USER",
      "BRANCH-ATM-SPV",
    ],
    group: "forecasting",
    disabled: true,
  },
  */
  {
    // replenishment-realisasi FR1: the calendar the realisasi job counts workdays with; ADMIN only (Q14).
    id: "holiday-calendar",
    label: "Kalender Libur",
    icon: CalendarDays,
    href: "/settings/admin/holidays",
    roles: ["ADMIN"],
    group: "forecasting",
  },

  // Invoice
  {
    id: "invoice-upload",
    label: "Unggah Invoice",
    icon: Receipt,
    href: "/invoice/upload",
    roles: ["VENDOR-USER", "BRANCH-USER"],
    group: "invoice",
    disabled: true,
  },
  /*
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
  */
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
  /*
  {
    id: "tier-analysis",
    label: "Analisis Tier Saldo",
    icon: BarChart3,
    href: "/cash-count/tier-analysis",
    roles: ["BRANCH-ATM-SPV"],
    group: "cash-count",
    disabled: true,
  },
  */
  {
    id: "execution",
    label: "Pelaksanaan (BA)",
    icon: ClipboardCheck,
    href: "/cash-count/execution",
    roles: ["BRANCH-ATM-USER"],
    group: "cash-count",
    disabled: true,
  },
  /*
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
  */
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

export interface VisibleNav {
  all: NavItem[];
  sidebar: Map<NavGroup, NavItem[]>;
  settings?: NavItem;
}

/** Item yang terlihat untuk role (tanpa `disabled`), dikelompokkan untuk sidebar; grup kosong dibuang. */
export function getVisibleNav(role: DbRole | null): VisibleNav {
  const all = (role ? filterNavByRoles(NAV_CONFIG, role) : []).filter((i) => !i.disabled);
  const sidebar = new Map<NavGroup, NavItem[]>();
  for (const group of SIDEBAR_GROUP_ORDER) {
    const items = all.filter((i) => i.group === group && i.placement !== "topbar");
    if (items.length > 0) sidebar.set(group, items);
  }
  return { all, sidebar, settings: all.find((i) => i.placement === "topbar") };
}

const normalizePath = (p: string) => (p.length > 1 ? p.replace(/\/+$/, "") || "/" : p);

/** Item dengan href terpanjang yang cocok per segmen; "/" hanya exact. */
export function findActiveNavItem(items: NavItem[], pathname: string): NavItem | undefined {
  const path = normalizePath(pathname);
  let best: NavItem | undefined;
  for (const item of items) {
    const href = normalizePath(item.href);
    const match = href === "/" ? path === "/" : path === href || path.startsWith(`${href}/`);
    if (match && (!best || href.length > normalizePath(best.href).length)) best = item;
  }
  return best;
}

/** Filter label/nama grup, case-insensitive; urutan asli dipertahankan. */
export function filterPaletteItems(items: NavItem[], query: string): NavItem[] {
  const q = query.trim().toLocaleLowerCase("id-ID");
  if (!q) return items;
  return items.filter(
    (i) =>
      i.label.toLocaleLowerCase("id-ID").includes(q) ||
      GROUP_LABELS[i.group].toLocaleLowerCase("id-ID").includes(q),
  );
}
