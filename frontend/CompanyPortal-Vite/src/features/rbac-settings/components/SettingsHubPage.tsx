/**
 * Settings hub landing page (/settings): links to the RBAC and admin sub-pages.
 * Tabs filter by category (access vs. master data), a search box filters by
 * title/description, "Hierarki Pengguna" is featured with an org-chart
 * illustration since it is the foundational entity every approval flow
 * depends on, and the rest render as list-row panels grouped by section
 * (design.md ban on repeated identical card grids).
 */

import { EmptyState } from "@/components/ui/EmptyState";
import { PageHeader } from "@/components/ui/PageHeader";
import { useAuthStore } from "@/lib/auth/store";
import { cn } from "@/lib/utils/cn";
import { Link } from "@tanstack/react-router";
import {
  ArrowRight,
  ArrowUpRight,
  Building2,
  CalendarOff,
  FileSpreadsheet,
  GitBranch,
  HelpCircle,
  KeyRound,
  Landmark,
  Lock,
  Scale,
  Search,
  Shield,
  ShieldCheck,
  UserCog,
  Users,
} from "lucide-react";
import type { LucideIcon } from "lucide-react";
import { useState } from "react";

type SettingsCategory = "access" | "master";

interface HubCard {
  id: string;
  title: string;
  description: string;
  href: string;
  icon: LucideIcon;
  category: SettingsCategory;
}

const FEATURED_CARD: HubCard = {
  id: "rbac-users",
  title: "Hierarki Pengguna",
  description:
    "Susun hubungan atasan, peran, dan level persetujuan. Fondasi alur otorisasi yang terarah.",
  href: "/settings/rbac/users",
  icon: Users,
  category: "access",
};

const RBAC_CARDS: HubCard[] = [
  {
    id: "rbac-delegations",
    title: "Delegasi Persetujuan",
    description: "Alihkan wewenang saat approver tidak tersedia.",
    href: "/settings/rbac/delegations",
    icon: GitBranch,
    category: "access",
  },
  {
    id: "rbac-leaves",
    title: "Cuti Pengguna",
    description: "Atur periode cuti dan alur persetujuan cadangan.",
    href: "/settings/rbac/leaves",
    icon: CalendarOff,
    category: "access",
  },
  {
    id: "rbac-policies",
    title: "Kebijakan Persetujuan",
    description: "Tentukan batas nominal dan level per jenis dokumen.",
    href: "/settings/rbac/policies",
    icon: Scale,
    category: "access",
  },
  {
    id: "role-management",
    title: "Manajemen Peran",
    description: "Buat peran baru dan atur menu/fitur yang dapat diakses tiap peran.",
    href: "/settings/roles",
    icon: KeyRound,
    category: "access",
  },
];

const MASTER_DATA_CARDS: HubCard[] = [
  {
    id: "admin-users",
    title: "Manajemen Pengguna",
    description: "Kelola akun, peran, dan status aktif pengguna.",
    href: "/settings/admin/users",
    icon: UserCog,
    category: "master",
  },
  {
    id: "admin-vendors",
    title: "Manajemen Vendor",
    description: "Kelola data mitra penyedia layanan CIT.",
    href: "/settings/admin/vendors",
    icon: Building2,
    category: "master",
  },
  {
    id: "admin-atms",
    title: "Manajemen ATM",
    description: "Pelihara data master dan status aktif ATM.",
    href: "/settings/admin/atms",
    icon: Landmark,
    category: "master",
  },
  {
    id: "admin-master-data-io",
    title: "Ekspor & Impor CSV",
    description: "Unduh, ubah massal, dan impor data master vendor dan ATM.",
    href: "/settings/admin/master-data-io",
    icon: FileSpreadsheet,
    category: "master",
  },
];

const TABS: { id: SettingsCategory; label: string }[] = [
  { id: "access", label: "Akses & persetujuan" },
  { id: "master", label: "Data master" },
];

function matchesQuery(card: HubCard, query: string): boolean {
  if (!query) return true;
  const haystack = `${card.title} ${card.description}`.toLowerCase();
  return haystack.includes(query);
}

function FeaturedCard({ card }: { card: HubCard }) {
  return (
    <div
      className="flex flex-col gap-5 rounded-[var(--radius-lg)] border-l-4 p-[var(--space-6)]"
      style={{
        backgroundColor: "var(--n-0)",
        borderColor: "var(--n-200)",
        borderLeftColor: "var(--red-500)",
        boxShadow: "var(--shadow-sm)",
      }}
    >
      <div className="flex items-start gap-4">
        <div
          className="flex h-14 w-14 shrink-0 items-center justify-center rounded-[var(--radius-md)]"
          style={{ backgroundColor: "var(--red-50)" }}
        >
          <card.icon size={26} style={{ color: "var(--red-500)" }} aria-hidden="true" />
        </div>
        <div className="flex flex-col gap-1">
          <span
            className="text-[0.7rem] font-[800] uppercase tracking-[0.09em]"
            style={{ color: "var(--red-500)" }}
          >
            Struktur Organisasi
          </span>
          <h2 className="text-lg font-semibold" style={{ color: "var(--n-900)" }}>
            {card.title}
          </h2>
          <p className="text-sm" style={{ color: "var(--n-600)" }}>
            {card.description}
          </p>
        </div>
      </div>

      <Link
        to={card.href}
        className="flex items-center justify-center gap-2 rounded-[var(--radius-md)] px-[var(--space-5)] py-3 text-sm font-semibold text-white transition-colors duration-200 hover:brightness-95"
        style={{ backgroundColor: "var(--red-600)" }}
      >
        Kelola hierarki
        <ArrowRight size={16} aria-hidden="true" />
      </Link>

      <div className="flex flex-col items-center pt-1">
        <div
          className="flex items-center gap-2 rounded-[var(--radius-md)] border px-4 py-2"
          style={{ backgroundColor: "var(--n-50)", borderColor: "var(--n-200)" }}
        >
          <ShieldCheck size={16} style={{ color: "var(--red-500)" }} aria-hidden="true" />
          <span className="text-sm font-medium" style={{ color: "var(--n-800)" }}>
            Supervisor
          </span>
          <span className="text-xs" style={{ color: "var(--n-500)" }}>
            Level persetujuan
          </span>
        </div>

        <div className="h-4 w-px" style={{ backgroundColor: "var(--n-300)" }} />
        <div className="flex w-full max-w-[260px]">
          <div
            className="h-4 flex-1 rounded-tr-md border-t border-r"
            style={{ borderColor: "var(--n-300)" }}
          />
          <div
            className="h-4 flex-1 rounded-tl-md border-t border-l"
            style={{ borderColor: "var(--n-300)" }}
          />
        </div>
        <div className="flex w-full max-w-[260px] justify-between gap-4">
          {["Pengguna A", "Pengguna B"].map((name) => (
            <div
              key={name}
              className="flex flex-1 items-center gap-2 rounded-[var(--radius-md)] border px-3 py-2"
              style={{ borderColor: "var(--n-200)" }}
            >
              <Users size={14} style={{ color: "var(--n-500)" }} aria-hidden="true" />
              <span className="text-xs font-medium" style={{ color: "var(--n-700)" }}>
                {name}
              </span>
            </div>
          ))}
        </div>
      </div>
    </div>
  );
}

function SectionRow({ card }: { card: HubCard }) {
  return (
    <Link
      to={card.href}
      className="group flex items-center gap-4 p-[var(--space-4)] transition-colors duration-200 hover:bg-[var(--n-50)]"
    >
      <div
        className="flex h-10 w-10 shrink-0 items-center justify-center rounded-[var(--radius-md)] transition-colors duration-200 group-hover:bg-[var(--red-50)]"
        style={{ backgroundColor: "var(--n-100)" }}
      >
        <card.icon
          size={18}
          className="transition-colors duration-200 group-hover:text-[var(--red-500)]"
          style={{ color: "var(--n-700)" }}
          aria-hidden="true"
        />
      </div>
      <div className="flex flex-1 flex-col gap-0.5">
        <h3
          className="text-sm font-semibold transition-colors duration-200 group-hover:text-[var(--red-600)]"
          style={{ color: "var(--n-900)" }}
        >
          {card.title}
        </h3>
        <p className="text-xs" style={{ color: "var(--n-600)" }}>
          {card.description}
        </p>
      </div>
      <ArrowUpRight
        size={16}
        className="shrink-0 transition-colors duration-200 group-hover:text-[var(--red-500)]"
        style={{ color: "var(--n-400)" }}
        aria-hidden="true"
      />
    </Link>
  );
}

function SectionPanel({
  icon: Icon,
  title,
  description,
  cards,
}: {
  icon: LucideIcon;
  title: string;
  description: string;
  cards: HubCard[];
}) {
  if (cards.length === 0) return null;

  return (
    <section className="flex flex-col gap-[var(--space-3)]">
      <div className="flex items-start justify-between gap-4">
        <div className="flex items-start gap-2">
          <Icon size={18} className="mt-0.5" style={{ color: "var(--n-700)" }} aria-hidden="true" />
          <div className="flex flex-col gap-0.5">
            <h2 className="text-sm font-semibold" style={{ color: "var(--n-900)" }}>
              {title}
            </h2>
            <p className="text-xs" style={{ color: "var(--n-600)" }}>
              {description}
            </p>
          </div>
        </div>
        <span className="shrink-0 text-xs" style={{ color: "var(--n-500)" }}>
          {cards.length} modul
        </span>
      </div>

      <div
        className="flex flex-col divide-y rounded-[var(--radius-lg)] border"
        style={{
          backgroundColor: "var(--n-0)",
          borderColor: "var(--n-200)",
          boxShadow: "var(--shadow-sm)",
        }}
      >
        {cards.map((card) => (
          <SectionRow key={card.id} card={card} />
        ))}
      </div>
    </section>
  );
}

/**
 * Role Management's card is hidden for anyone but APPACCESS/ADMIN (Req 4.5,
 * 7.1), matching the backend's RequireRoles("APPACCESS", "ADMIN") guard on
 * /api/v1/admin/roles. Scoped to just this one card rather than adding a
 * roles field to HubCard: every other card here relies solely on its own
 * route's beforeLoad guard today (a broader gap, not introduced by this
 * change) — see roles.tsx's doc comment on the shared requireRoles()
 * ADMIN_PARAM bypass this still inherits once the user clicks through.
 */
function canSeeRoleManagementCard(role: string | undefined): boolean {
  return role === "APPACCESS" || role === "ADMIN";
}

export function SettingsHubPage() {
  const [activeTab, setActiveTab] = useState<SettingsCategory>("access");
  const [query, setQuery] = useState("");
  const userRole = useAuthStore((s) => s.user?.role);

  const normalizedQuery = query.trim().toLowerCase();
  const showAccess = activeTab === "access";
  const showMaster = activeTab === "master";

  const showFeatured = showAccess && matchesQuery(FEATURED_CARD, normalizedQuery);
  const rbacCards = showAccess
    ? RBAC_CARDS.filter(
        (c) =>
          matchesQuery(c, normalizedQuery) &&
          (c.id !== "role-management" || canSeeRoleManagementCard(userRole)),
      )
    : [];
  const masterCards = showMaster
    ? MASTER_DATA_CARDS.filter((c) => matchesQuery(c, normalizedQuery))
    : [];

  const hasResults = showFeatured || rbacCards.length > 0 || masterCards.length > 0;

  return (
    <div className="flex max-w-[960px] flex-col gap-6">
      <PageHeader
        eyebrow="Administrasi"
        title="Pengaturan"
        description="Kelola akses, alur persetujuan, dan data master CMS."
        actions={
          <button
            type="button"
            title="Bantuan"
            className="flex h-11 w-11 shrink-0 items-center justify-center rounded-full border transition-colors duration-200 hover:bg-[var(--n-50)]"
            style={{ borderColor: "var(--n-200)", color: "var(--n-600)" }}
          >
            <HelpCircle size={20} aria-hidden="true" />
            <span className="sr-only">Bantuan</span>
          </button>
        }
      />

      <div
        className="flex w-fit gap-1 rounded-full p-1"
        style={{ backgroundColor: "var(--n-100)" }}
        role="tablist"
      >
        {TABS.map((tab) => (
          <button
            key={tab.id}
            type="button"
            role="tab"
            aria-selected={activeTab === tab.id}
            onClick={() => setActiveTab(tab.id)}
            className={cn(
              "rounded-full px-4 py-2 text-sm font-medium transition-colors duration-200",
              activeTab === tab.id ? "shadow-[var(--shadow-sm)]" : "hover:text-[var(--n-800)]",
            )}
            style={{
              backgroundColor: activeTab === tab.id ? "var(--n-0)" : "transparent",
              color: activeTab === tab.id ? "var(--n-900)" : "var(--n-600)",
            }}
          >
            {tab.label}
          </button>
        ))}
      </div>

      <div className="relative">
        <Search
          size={16}
          className="pointer-events-none absolute top-1/2 left-4 -translate-y-1/2"
          style={{ color: "var(--n-400)" }}
          aria-hidden="true"
        />
        <input
          type="search"
          value={query}
          onChange={(e) => setQuery(e.target.value)}
          placeholder="Cari pengaturan..."
          className="min-h-[44px] w-full rounded-[var(--radius-md)] border py-2.5 pr-4 pl-11 text-sm outline-none focus-visible:ring-2 focus-visible:ring-[var(--red-100)] focus-visible:border-[var(--red-400)]"
          style={{
            borderColor: "var(--n-200)",
            backgroundColor: "var(--n-0)",
            color: "var(--n-800)",
          }}
        />
      </div>

      {!hasResults && <EmptyState icon={Search} message="Tidak ada pengaturan yang cocok." />}

      {showFeatured && <FeaturedCard card={FEATURED_CARD} />}

      <SectionPanel
        icon={Shield}
        title="Akses & persetujuan"
        description="Jaga otorisasi tetap berjalan."
        cards={rbacCards}
      />

      <SectionPanel
        icon={Building2}
        title="Data master & admin"
        description="Satu tempat untuk data operasional."
        cards={masterCards}
      />

      <div className="flex items-center justify-center gap-2 pt-2 pb-4">
        <Lock size={14} style={{ color: "var(--n-400)" }} aria-hidden="true" />
        <p className="text-xs" style={{ color: "var(--n-500)" }}>
          Pengaturan tersedia sesuai peran dan hak akses.
        </p>
      </div>
    </div>
  );
}
