import { ModuleCard, type ModuleCardProps } from "@/components/ui/ModuleCard";
import { createRoute } from "@tanstack/react-router";
import { BarChart3, CalendarDays, ClipboardCheck, ListChecks, Scale, Table } from "lucide-react";
import { protectedRoute } from "../_protected";

export const cashCountRoute = createRoute({
  path: "/cash-count",
  getParentRoute: () => protectedRoute,
  component: CashCountPage,
});

const CASH_COUNT_CARDS: ModuleCardProps[] = [
  {
    id: "scheduling",
    title: "Penjadwalan",
    description: "Kelola jadwal kunjungan bulanan ke vault (on-site dan virtual).",
    href: "/cash-count/scheduling",
    icon: CalendarDays,
    disabled: true,
  },
  {
    id: "tier-analysis",
    title: "Analisis Tier Saldo",
    description:
      "Kategorisasi vault berdasarkan rata-rata escrow: High, Medium, atau Low setiap triwulan.",
    href: "/cash-count/tier-analysis",
    icon: BarChart3,
    disabled: true,
  },
  {
    id: "execution",
    title: "Pelaksanaan (BA)",
    description:
      "Isi berita acara perhitungan kas untuk vault ATM, vault cash, dan selective machine.",
    href: "/cash-count/execution",
    icon: ClipboardCheck,
    disabled: true,
  },
  {
    id: "checklists",
    title: "Checklist",
    description: "Checklist risiko vendor PJPUR (13 item) dan selective machine (18 item).",
    href: "/cash-count/checklists",
    icon: ListChecks,
    disabled: true,
  },
  {
    id: "cc-reconciliation",
    title: "Rekonsiliasi",
    description: "Pencocokan hasil hitung fisik dengan saldo escrow dari eTP/SIBS.",
    href: "/cash-count/reconciliation",
    icon: Scale,
    disabled: true,
  },
  {
    id: "recapitulation",
    title: "Rekapitulasi",
    description: "Ringkasan otomatis dari BA, DSR, dan laporan harian MIS untuk rekap bulanan.",
    href: "/cash-count/recapitulation",
    icon: Table,
    disabled: true,
  },
];

function CashCountPage() {
  return (
    <div className="flex flex-col gap-[var(--space-6)]">
      <div className="flex flex-col gap-[var(--space-2)]">
        <h1 className="text-xl font-semibold" style={{ color: "var(--n-900)" }}>
          Perhitungan Kas
        </h1>
        <p className="text-sm" style={{ color: "var(--n-600)" }}>
          Kelola siklus perhitungan kas mulai dari penjadwalan, pelaksanaan, hingga rekonsiliasi dan
          rekapitulasi.
        </p>
      </div>

      <div className="grid grid-cols-1 gap-[var(--space-4)] sm:grid-cols-2 lg:grid-cols-3">
        {CASH_COUNT_CARDS.map((card) => (
          <ModuleCard key={card.id} {...card} />
        ))}
      </div>
    </div>
  );
}
