import { ModuleCard, type ModuleCardProps } from "@/components/ui/ModuleCard";
import { createRoute } from "@tanstack/react-router";
import { Calculator, Calendar, CheckCircle, FileText, TrendingUp, Upload } from "lucide-react";
import { protectedRoute } from "../_protected";

export const forecastingRoute = createRoute({
  path: "/forecasting",
  getParentRoute: () => protectedRoute,
  component: ForecastingPage,
});

const FORECASTING_CARDS: ModuleCardProps[] = [
  {
    id: "dsr-upload",
    title: "Unggah DSR",
    description:
      "Unggah laporan harian saldo vault, rencana pengisian, dan hasil rekonsiliasi dari vendor.",
    href: "/forecasting/dsr-upload",
    icon: Upload,
    disabled: false,
  },
  {
    id: "fill-instruction",
    title: "Instruksi Pengisian",
    description:
      "Daftar harian lokasi ATM/CRM yang harus diisi, termasuk deduplikasi dan pengecualian mesin bermasalah.",
    href: "/forecasting/fill-instruction",
    icon: FileText,
    disabled: true,
  },
  {
    id: "fill-validation",
    title: "Validasi Pengisian",
    description:
      "Kategorisasi pengisian aktual: sesuai jadwal, maju, mundur, atau tidak dilakukan.",
    href: "/forecasting/fill-validation",
    icon: CheckCircle,
    disabled: true,
  },
  {
    id: "cash-supply",
    title: "Cash Supply",
    description:
      "Perhitungan kebutuhan kas berdasarkan order, rencana isi, saldo vault, dan perkiraan refund.",
    href: "/forecasting/cash-supply",
    icon: Calculator,
    disabled: true,
  },
  {
    id: "h2-projection",
    title: "Proyeksi H+2",
    description:
      "Proyeksi kebutuhan kas dua hari ke depan berdasarkan refund, saldo, dan tren transaksi.",
    href: "/forecasting/h2-projection",
    icon: TrendingUp,
    disabled: true,
  },
  {
    id: "holiday-calendar",
    title: "Kalender Libur",
    description:
      "Pengelolaan kalender hari libur nasional yang memengaruhi logika penjadwalan pengisian.",
    href: "/forecasting/holiday-calendar",
    icon: Calendar,
    disabled: true,
  },
];

function ForecastingPage() {
  return (
    <div className="flex flex-col gap-[var(--space-6)]">
      <div className="flex flex-col gap-[var(--space-2)]">
        <h1 className="text-xl font-semibold" style={{ color: "var(--n-900)" }}>
          Peramalan
        </h1>
        <p className="text-sm" style={{ color: "var(--n-600)" }}>
          Kelola proses peramalan kas ATM/CRM mulai dari penerimaan DSR hingga proyeksi kebutuhan
          harian.
        </p>
      </div>

      <div className="grid grid-cols-1 gap-[var(--space-4)] sm:grid-cols-2 lg:grid-cols-3">
        {FORECASTING_CARDS.map((card) => (
          <ModuleCard key={card.id} {...card} />
        ))}
      </div>
    </div>
  );
}
