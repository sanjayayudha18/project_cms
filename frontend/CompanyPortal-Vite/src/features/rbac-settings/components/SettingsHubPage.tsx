/**
 * Settings hub landing page (/settings): links to the four RBAC sub-pages.
 * A varied bento layout, not a repeated identical card grid (design.md ban):
 * "Hierarki Pengguna" is the featured, horizontal card since it is the
 * foundational entity every approval flow depends on; the other three sit as
 * equal compact cards below it.
 */

import { PageHeader } from "@/components/ui/PageHeader";
import { cn } from "@/lib/utils/cn";
import { Link } from "@tanstack/react-router";
import { Building2, CalendarOff, GitBranch, Scale, UserCog, Users } from "lucide-react";
import type { LucideIcon } from "lucide-react";

interface HubCard {
  id: string;
  title: string;
  description: string;
  href: string;
  icon: LucideIcon;
}

const FEATURED_CARD: HubCard = {
  id: "rbac-users",
  title: "Hierarki Pengguna",
  description:
    "Atur atasan (supervisor) dan level persetujuan setiap pengguna, beserta peran yang dimiliki.",
  href: "/settings/rbac/users",
  icon: Users,
};

const SECONDARY_CARDS: HubCard[] = [
  {
    id: "rbac-delegations",
    title: "Delegasi Persetujuan",
    description: "Alihkan wewenang persetujuan sementara saat approver tidak tersedia.",
    href: "/settings/rbac/delegations",
    icon: GitBranch,
  },
  {
    id: "rbac-leaves",
    title: "Cuti Pengguna",
    description: "Catat periode cuti pengguna untuk mengaktifkan alur persetujuan cadangan.",
    href: "/settings/rbac/leaves",
    icon: CalendarOff,
  },
  {
    id: "rbac-policies",
    title: "Kebijakan Persetujuan",
    description: "Lihat batas nominal dan level persetujuan yang berlaku per jenis dokumen.",
    href: "/settings/rbac/policies",
    icon: Scale,
  },
  {
    id: "admin-users",
    title: "Manajemen Pengguna",
    description: "Tambah, ubah, dan nonaktifkan akun pengguna.",
    href: "/settings/admin/users",
    icon: UserCog,
  },
  {
    id: "admin-vendors",
    title: "Manajemen Vendor",
    description: "Tambah, ubah, dan nonaktifkan data vendor CIT.",
    href: "/settings/admin/vendors",
    icon: Building2,
  },
];

export function SettingsHubPage() {
  return (
    <div className="flex flex-col gap-6">
      <PageHeader
        eyebrow="Admin"
        title="Pengaturan"
        description="Kelola konfigurasi RBAC: hierarki pengguna, delegasi, cuti, dan kebijakan persetujuan."
      />

      <Link
        to={FEATURED_CARD.href}
        className={cn(
          "group flex items-center gap-[var(--space-5)] rounded-[var(--radius-lg)] border p-[var(--space-6)]",
          "bg-[var(--n-0)] border-[var(--n-200)] shadow-[var(--shadow-sm)]",
          "transition-all duration-200 hover:border-[var(--red-200)] hover:shadow-[var(--shadow-md)]",
        )}
      >
        <div
          className="flex h-14 w-14 shrink-0 items-center justify-center rounded-[var(--radius-md)] transition-colors duration-200 group-hover:bg-[var(--red-50)]"
          style={{ backgroundColor: "var(--n-100)" }}
        >
          <FEATURED_CARD.icon
            size={26}
            className="transition-colors duration-200 group-hover:text-[var(--red-500)]"
            style={{ color: "var(--n-700)" }}
            aria-hidden="true"
          />
        </div>
        <div className="flex flex-col gap-1">
          <h2
            className="text-base font-semibold transition-colors duration-200 group-hover:text-[var(--red-600)]"
            style={{ color: "var(--n-900)" }}
          >
            {FEATURED_CARD.title}
          </h2>
          <p className="text-sm" style={{ color: "var(--n-600)" }}>
            {FEATURED_CARD.description}
          </p>
        </div>
      </Link>

      <div className="grid grid-cols-1 gap-[var(--space-4)] sm:grid-cols-3">
        {SECONDARY_CARDS.map((card) => (
          <Link
            key={card.id}
            to={card.href}
            className={cn(
              "group flex flex-col gap-[var(--space-3)] rounded-[var(--radius-lg)] border p-[var(--space-5)]",
              "bg-[var(--n-0)] border-[var(--n-200)] shadow-[var(--shadow-sm)]",
              "transition-all duration-200 hover:border-[var(--red-200)] hover:shadow-[var(--shadow-md)]",
            )}
          >
            <div
              className="flex h-10 w-10 items-center justify-center rounded-[var(--radius-md)] transition-colors duration-200 group-hover:bg-[var(--red-50)]"
              style={{ backgroundColor: "var(--n-100)" }}
            >
              <card.icon
                size={20}
                className="transition-colors duration-200 group-hover:text-[var(--red-500)]"
                style={{ color: "var(--n-700)" }}
                aria-hidden="true"
              />
            </div>
            <div className="flex flex-col gap-[var(--space-1)]">
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
          </Link>
        ))}
      </div>
    </div>
  );
}
