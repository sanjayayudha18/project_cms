import type { LucideIcon } from "lucide-react";
import {
  Activity,
  BarChart3,
  CalendarDays,
  ClipboardCheck,
  ClipboardList,
  FileClock,
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
import { useMemo } from "react";

import { SCREEN_KEYS, type ScreenKey } from "@/lib/auth/permissionKeys";
import { usePermissionStore } from "@/lib/auth/permissionStore";

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

export type NavPlacement = "sidebar" | "sidebar-footer";
export type NavBadgeSource = "approvals-inbox" | "cit-requests" | "eod-health";

export interface NavItem {
  id: string;
  label: string;
  icon: LucideIcon;
  href: string;
  /** Kunci katalog izin (menu_features.key); array = any-of. */
  permission: ScreenKey | readonly ScreenKey[];
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

/** Item Pengaturan terlihat bila user punya salah satu layar `settings.*` (hub). */
const SETTINGS_HUB_KEYS: readonly ScreenKey[] = SCREEN_KEYS.filter((k) =>
  k.startsWith("settings."),
);

export const NAV_CONFIG: NavItem[] = [
  // General
  {
    id: "dashboard",
    label: "Dashboard",
    icon: LayoutGrid,
    href: "/",
    permission: "dashboard",
    group: "general",
  },
  {
    id: "cash-flow",
    label: "Cash Flow Monitoring",
    icon: Activity,
    href: "/cash-flow",
    permission: "cash-flow",
    group: "general",
  },
  {
    id: "approvals",
    label: "Persetujuan",
    icon: ClipboardCheck,
    href: "/approvals",
    permission: "approvals",
    group: "general",
    badge: "approvals-inbox",
  },
  {
    id: "settings",
    label: "Pengaturan",
    icon: Settings2,
    href: "/settings",
    permission: SETTINGS_HUB_KEYS,
    group: "system",
    placement: "sidebar-footer",
  },

  // Replenish
  {
    id: "forecast-browser",
    label: "Rekomendasi CIT",
    icon: Search,
    href: "/replenishment/forecast-browser",
    permission: "replenish.forecast-browser",
    group: "replenish",
  },
  {
    id: "vendor-requests",
    label: "Request CIT",
    icon: ClipboardList,
    href: "/replenishment/vendor-requests",
    permission: "replenish.vendor-requests",
    group: "replenish",
    badge: "cit-requests",
  },
  {
    id: "replenishment",
    label: "Jadwal CIT",
    icon: Truck,
    href: "/replenishment",
    permission: "replenish.schedule",
    group: "replenish",
  },
  // CIT (cit-acm-plan): tim ACM menetapkan branch vault per ATM.
  {
    id: "vault-plans",
    label: "Penetapan Vault",
    icon: Landmark,
    href: "/cit/vault-plans",
    permission: "cit.vault-plan",
    group: "cit",
  },
  // Monitoring
  {
    id: "atm-portal",
    label: "ATM Portal",
    icon: Monitor,
    href: "/atm-portal",
    permission: "monitoring.atm-portal",
    group: "monitoring",
  },
  {
    id: "dsr-late-report",
    label: "Laporan DSR Telat",
    icon: FileClock,
    href: "/dsr-late-report",
    permission: "monitoring.dsr-late-report",
    group: "monitoring",
  },
  {
    id: "eod-monitoring",
    label: "EOD Monitoring",
    icon: HeartPulse,
    href: "/eod-monitoring",
    permission: "monitoring.eod-monitoring",
    group: "monitoring",
    badge: "eod-health",
  },
  {
    id: "audit-logs",
    label: "Log Audit",
    icon: ScrollText,
    href: "/audit-logs",
    permission: "monitoring.audit-logs",
    group: "monitoring",
  },

  {
    id: "dmaa-forecast",
    label: "DMAA Forecast",
    icon: BarChart3,
    href: "/forecasting/dmaa-forecast",
    permission: "forecasting.dmaa-forecast",
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
    permission: "forecasting.holidays",
    group: "forecasting",
  },

  // Invoice
  {
    id: "invoice-upload",
    label: "Unggah Invoice",
    icon: Receipt,
    href: "/invoice/upload",
    permission: "invoice",
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
    permission: "cash-count",
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
    permission: "cash-count",
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
    permission: "cash-count",
    group: "cash-count",
    disabled: true,
  },
];

// ─── Utilities ────────────────────────────────────────────────────────────────

/** Item terlihat bila `has` meluluskan salah satu key izinnya. Tanpa bypass role (D5). */
export function filterNavByPermissions(
  items: NavItem[],
  has: (key: ScreenKey) => boolean,
): NavItem[] {
  return items.filter((item) =>
    typeof item.permission === "string" ? has(item.permission) : item.permission.some(has),
  );
}

export interface VisibleNav {
  all: NavItem[];
  sidebar: Map<NavGroup, NavItem[]>;
  settings?: NavItem;
}

/** Item yang terlihat untuk izin user (tanpa `disabled`), dikelompokkan untuk sidebar; grup kosong dibuang. */
export function getVisibleNav(has: (key: ScreenKey) => boolean): VisibleNav {
  const all = filterNavByPermissions(NAV_CONFIG, has).filter((i) => !i.disabled);
  const sidebar = new Map<NavGroup, NavItem[]>();
  for (const group of SIDEBAR_GROUP_ORDER) {
    const items = all.filter((i) => i.group === group && i.placement !== "sidebar-footer");
    if (items.length > 0) sidebar.set(group, items);
  }
  return { all, sidebar, settings: all.find((i) => i.placement === "sidebar-footer") };
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

/** Hook: nav terlihat dari store izin; saat loading/error store hanya meloloskan Always_On. */
export function useVisibleNav(): {
  visible: VisibleNav;
  permissionError: boolean;
  retry: () => void;
} {
  const keys = usePermissionStore((s) => s.keys);
  const status = usePermissionStore((s) => s.status);
  const load = usePermissionStore((s) => s.load);
  // biome-ignore lint/correctness/useExhaustiveDependencies: keys/status adalah pemicu hitung ulang `has`
  const visible = useMemo(
    () => getVisibleNav((k) => usePermissionStore.getState().has(k)),
    [keys, status],
  );
  return { visible, permissionError: status === "error", retry: () => void load() };
}
