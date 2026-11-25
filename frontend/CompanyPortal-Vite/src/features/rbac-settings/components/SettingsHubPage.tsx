/**
 * Settings hub landing page (/settings): links to the RBAC and admin sub-pages.
 * A vertical category rail (access, master data, export & import) picks the
 * category shown; inside it, modules are grouped under small headings and
 * rendered as uniform tiles. The search box filters by title/description and
 * the rail counts follow the search so users can see where matches live.
 * Cards the user lacks the permission key for are hidden here, and a category with no
 * visible cards drops out of the rail. Design: Settings Hub Revamp artifact.
 */

import { EmptyState } from "@/components/ui/EmptyState";
import { PageHeader } from "@/components/ui/PageHeader";
import type { ScreenKey } from "@/lib/auth/permissionKeys";
import { usePermissionStore } from "@/lib/auth/permissionStore";
import { Link } from "@tanstack/react-router";
import {
  ArrowDownUp,
  Building2,
  CalendarDays,
  CalendarOff,
  ChevronRight,
  Database,
  Download,
  FileSpreadsheet,
  FileText,
  GitBranch,
  HelpCircle,
  KeyRound,
  Landmark,
  Map as MapIcon,
  MapPinned,
  Scale,
  Search,
  Shield,
  Tags,
  UserCog,
  UserPlus,
  Users,
} from "lucide-react";
import type { LucideIcon } from "lucide-react";
import { useId, useState } from "react";

type SettingsCategory = "access" | "master" | "io";

interface HubCard {
  id: string;
  title: string;
  description: string;
  href: string;
  icon: LucideIcon;
  /** Catalog key that makes this card visible. */
  permission: ScreenKey;
  /** Search params for the target route (e.g. open a dialog on arrival). */
  search?: Record<string, unknown>;
}

interface HubGroup {
  id: string;
  label: string;
  cards: HubCard[];
}

interface HubCategory {
  id: SettingsCategory;
  label: string;
  icon: LucideIcon;
  groups: HubGroup[];
}

const CATEGORIES: HubCategory[] = [
  {
    id: "access",
    label: "Akses & persetujuan",
    icon: Shield,
    groups: [
      {
        id: "people",
        label: "Pengguna & peran",
        cards: [
          {
            id: "rbac-users",
            permission: "settings.rbac-users",
            title: "Hierarki Pengguna",
            description: "Susun hubungan atasan, peran, dan level persetujuan.",
            href: "/settings/rbac/users",
            icon: Users,
          },
          {
            id: "admin-users",
            permission: "settings.users",
            title: "Manajemen Pengguna",
            description: "Kelola akun, peran, sumber login, dan status aktif.",
            href: "/settings/admin/users",
            icon: UserCog,
          },
          {
            id: "add-user",
            permission: "settings.users",
            title: "Tambah Pengguna",
            description: "Buat akun pengguna baru beserta peran dan sumber login.",
            href: "/settings/admin/users/new",
            icon: UserPlus,
          },
          {
            id: "role-management",
            permission: "settings.roles",
            title: "Manajemen Peran",
            description: "Buat peran baru dan atur menu/fitur yang dapat diakses tiap peran.",
            href: "/settings/roles",
            icon: KeyRound,
          },
        ],
      },
      {
        id: "approval-flow",
        label: "Alur persetujuan",
        cards: [
          {
            id: "rbac-policies",
            permission: "settings.rbac-policies",
            title: "Kebijakan Persetujuan",
            description: "Tentukan batas nominal dan level per jenis dokumen.",
            href: "/settings/rbac/policies",
            icon: Scale,
          },
          {
            id: "rbac-delegations",
            permission: "settings.rbac-delegations",
            title: "Delegasi Persetujuan",
            description: "Alihkan wewenang saat approver tidak tersedia.",
            href: "/settings/rbac/delegations",
            icon: GitBranch,
          },
          {
            id: "rbac-leaves",
            permission: "settings.rbac-leaves",
            title: "Cuti Pengguna",
            description: "Atur periode cuti dan alur persetujuan cadangan.",
            href: "/settings/rbac/leaves",
            icon: CalendarOff,
          },
        ],
      },
    ],
  },
  {
    id: "master",
    label: "Data master",
    icon: Database,
    groups: [
      {
        id: "vendor",
        label: "Vendor & kontrak",
        cards: [
          {
            id: "admin-vendors",
            permission: "settings.vendors",
            title: "Manajemen Vendor",
            description: "Data mitra penyedia layanan CIT, cabang, vault, dan PIC.",
            href: "/settings/admin/vendors",
            icon: Building2,
          },
          {
            id: "admin-pks",
            permission: "settings.pks",
            title: "Daftar PKS",
            description: "Pantau masa berlaku PKS semua vendor dan atur reminder perpanjangan.",
            href: "/settings/admin/pks",
            icon: FileText,
          },
          {
            id: "admin-pks-types",
            permission: "settings.pks-types",
            title: "Type PKS",
            description: "Master type perjanjian kerja sama vendor (CIT, FLM, …).",
            href: "/settings/admin/pks-types",
            icon: Tags,
          },
        ],
      },
      {
        id: "atm",
        label: "Jaringan ATM & wilayah",
        cards: [
          {
            id: "admin-atms",
            permission: "settings.atms",
            title: "Manajemen ATM",
            description: "Data master ATM, kelolaan vendor, dan status aktif.",
            href: "/settings/admin/atms",
            icon: Landmark,
          },
          {
            id: "admin-regions",
            permission: "settings.regions",
            title: "Manajemen Region",
            description: "Daftar region baku untuk ATM dan cabang vendor.",
            href: "/settings/admin/regions",
            icon: MapIcon,
          },
          {
            id: "admin-acm-areas",
            permission: "settings.acm-areas",
            title: "Area ACM",
            description: "Petakan region ke Area ACM dan atur anggota tim per area.",
            href: "/settings/admin/acm-areas",
            icon: MapPinned,
          },
        ],
      },
      {
        id: "calendar",
        label: "Kalender operasional",
        cards: [
          {
            id: "admin-holidays",
            permission: "forecasting.holidays",
            title: "Hari Libur",
            description: "Kalender libur untuk hitung hari kerja realisasi pengisian ATM.",
            href: "/settings/admin/holidays",
            icon: CalendarDays,
          },
        ],
      },
    ],
  },
  {
    id: "io",
    label: "Export & Import",
    icon: ArrowDownUp,
    groups: [
      {
        id: "bulk",
        label: "Data massal",
        cards: [
          {
            id: "admin-master-data-io",
            permission: "settings.master-data-io",
            title: "Impor CSV Data Master",
            description:
              "Impor perubahan massal data master vendor dan ATM, lalu pantau persetujuannya.",
            href: "/settings/admin/master-data-io",
            icon: FileSpreadsheet,
          },
          {
            id: "admin-exports",
            permission: "exports",
            title: "Export Data",
            description:
              "Ekspor data master dan laporan: lihat pratinjau tabel, lalu unduh sebagai CSV.",
            href: "/exports",
            icon: Download,
          },
        ],
      },
    ],
  },
];

function matchesQuery(card: HubCard, query: string): boolean {
  if (!query) return true;
  return `${card.title} ${card.description}`.toLowerCase().includes(query);
}

/**
 * Categories with their groups filtered to the cards the user's permission set
 * covers and the query matches. Hiding a card is UX only; every target route
 * enforces its own requirePermission guard and the API its own checks.
 */
function filterCategories(has: (key: string) => boolean, query: string) {
  return CATEGORIES.map((category) => {
    const allowed = category.groups.map((group) => ({
      ...group,
      cards: group.cards.filter((card) => has(card.permission)),
    }));
    const groups = allowed
      .map((group) => ({ ...group, cards: group.cards.filter((c) => matchesQuery(c, query)) }))
      .filter((group) => group.cards.length > 0);
    return {
      ...category,
      hasAccess: allowed.some((group) => group.cards.length > 0),
      groups,
      count: groups.reduce((sum, group) => sum + group.cards.length, 0),
    };
  }).filter((category) => category.hasAccess);
}

function HubTile({ card }: { card: HubCard }) {
  return (
    <Link
      to={card.href}
      search={card.search}
      className="group flex min-h-[44px] items-start gap-3.5 rounded-[var(--radius-lg)] border p-[var(--space-4)] shadow-[var(--shadow-sm)] transition-[border-color,box-shadow,transform] duration-150 hover:-translate-y-px hover:border-[var(--n-300)] hover:shadow-[var(--shadow-md)] focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-[var(--red-400)] motion-reduce:transition-none motion-reduce:hover:translate-y-0"
      style={{ backgroundColor: "var(--n-0)", borderColor: "var(--n-200)" }}
    >
      <span className="flex h-10 w-10 shrink-0 items-center justify-center rounded-[var(--radius-md)] bg-[var(--n-100)] text-[var(--n-700)] transition-colors duration-150 group-hover:bg-[var(--red-50)] group-hover:text-[var(--red-500)]">
        <card.icon size={18} aria-hidden="true" />
      </span>
      <span className="flex min-w-0 flex-1 flex-col gap-1">
        <span className="text-sm font-semibold text-[var(--n-900)] transition-colors duration-150 group-hover:text-[var(--red-600)]">
          {card.title}
        </span>
        <span className="text-[13px] leading-[1.45] text-[var(--n-600)]">{card.description}</span>
      </span>
      <ChevronRight
        size={16}
        className="mt-0.5 shrink-0 text-[var(--n-400)] transition-[color,transform] duration-150 group-hover:translate-x-0.5 group-hover:text-[var(--red-500)]"
        aria-hidden="true"
      />
    </Link>
  );
}

function CategoryPanel({
  category,
}: {
  category: { label: string; count: number; groups: HubGroup[] };
}) {
  const headingId = useId();
  return (
    <section aria-labelledby={headingId} className="flex flex-col gap-4">
      <div className="flex items-baseline justify-between gap-3">
        <h2 id={headingId} className="text-lg font-bold text-[var(--n-900)]">
          {category.label}
        </h2>
        <span className="text-[13px] tabular-nums text-[var(--n-600)]">{category.count} modul</span>
      </div>
      {category.groups.map((group) => (
        <div key={group.id} className="flex flex-col gap-2.5">
          <h3 className="text-xs font-bold uppercase tracking-[0.06em] text-[var(--n-600)]">
            {group.label}
          </h3>
          <div className="grid grid-cols-[repeat(auto-fill,minmax(260px,1fr))] gap-3">
            {group.cards.map((card) => (
              <HubTile key={card.id} card={card} />
            ))}
          </div>
        </div>
      ))}
    </section>
  );
}

export function SettingsHubPage() {
  const keys = usePermissionStore((s) => s.keys);
  const status = usePermissionStore((s) => s.status);
  const [selected, setSelected] = useState<SettingsCategory>("access");
  const [query, setQuery] = useState("");

  // No hub card is Always_On, so nothing shows until the permission set is ready.
  const has = (key: string) => status === "ready" && keys.has(key);
  const trimmedQuery = query.trim();
  const categories = filterCategories(has, trimmedQuery.toLowerCase());
  // Fall back to the first category this role can see (ATM-SPV: Data master).
  const active = categories.find((c) => c.id === selected) ?? categories[0];
  const resultText = trimmedQuery
    ? `${active?.count ?? 0} hasil untuk “${trimmedQuery}”`
    : `${active?.count ?? 0} modul tersedia`;

  return (
    <div className="flex max-w-[1200px] flex-col gap-6">
      <PageHeader
        eyebrow="Administrasi"
        title="Pengaturan"
        description="Kelola akses, alur persetujuan, dan data master CROWN dari satu tempat."
        actions={
          <button
            type="button"
            title="Bantuan"
            className="flex h-11 w-11 shrink-0 items-center justify-center rounded-full border transition-colors duration-200 hover:bg-[var(--n-50)]"
            style={{
              borderColor: "var(--n-200)",
              color: "var(--n-600)",
              backgroundColor: "var(--n-0)",
            }}
          >
            <HelpCircle size={20} aria-hidden="true" />
            <span className="sr-only">Bantuan</span>
          </button>
        }
      />

      <div className="flex flex-wrap items-center gap-3">
        <label
          className="flex min-h-12 flex-[1_1_320px] items-center gap-2.5 rounded-[var(--radius-lg)] border bg-[var(--n-0)] px-3.5 shadow-[var(--shadow-sm)] focus-within:border-[var(--red-400)] focus-within:ring-2 focus-within:ring-[var(--red-100)]"
          style={{ borderColor: "var(--n-200)" }}
        >
          <Search size={18} className="shrink-0 text-[var(--n-500)]" aria-hidden="true" />
          <span className="sr-only">Cari pengaturan</span>
          <input
            type="search"
            value={query}
            onChange={(e) => setQuery(e.target.value)}
            placeholder="Cari modul, mis. PKS, ATM, delegasi…"
            className="min-w-0 flex-1 bg-transparent text-sm text-[var(--n-900)] focus-visible:outline-none"
          />
        </label>
        <p aria-live="polite" className="text-[13px] tabular-nums text-[var(--n-600)]">
          {resultText}
        </p>
      </div>

      <div className="flex flex-wrap items-start gap-8">
        <div
          role="tablist"
          aria-label="Kategori pengaturan"
          aria-orientation="vertical"
          className="flex flex-[1_1_220px] flex-col gap-1"
        >
          {categories.map((category) => {
            const isActive = category.id === active?.id;
            return (
              <button
                key={category.id}
                type="button"
                role="tab"
                aria-selected={isActive}
                onClick={() => setSelected(category.id)}
                className="flex min-h-[44px] w-full items-center gap-2.5 rounded-[var(--radius-md)] border px-3 py-2 text-left text-sm font-semibold transition-colors duration-150 hover:bg-[var(--n-100)] focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-[var(--red-400)]"
                style={
                  isActive
                    ? {
                        backgroundColor: "var(--n-0)",
                        borderColor: "var(--n-200)",
                        color: "var(--n-900)",
                        boxShadow: "var(--shadow-sm)",
                      }
                    : { borderColor: "transparent", color: "var(--n-600)" }
                }
              >
                <category.icon
                  size={18}
                  className="shrink-0"
                  style={{ color: isActive ? "var(--red-500)" : "var(--n-500)" }}
                  aria-hidden="true"
                />
                <span className="flex-1">{category.label}</span>
                <span
                  aria-hidden="true"
                  className="min-w-6 rounded-full px-2 py-px text-center text-xs font-semibold tabular-nums"
                  style={{
                    backgroundColor: isActive ? "var(--red-50)" : "var(--n-100)",
                    color: isActive ? "var(--red-700)" : "var(--n-600)",
                  }}
                >
                  {category.count}
                </span>
              </button>
            );
          })}
        </div>

        <div role="tabpanel" className="flex min-w-0 flex-[999_1_560px] flex-col gap-8">
          {active && active.count > 0 ? (
            <CategoryPanel category={active} />
          ) : (
            <EmptyState icon={Search} message="Tidak ada pengaturan yang cocok." />
          )}
        </div>
      </div>
    </div>
  );
}
