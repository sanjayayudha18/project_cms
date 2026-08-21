import { ErrorBoundary } from "@/components/feedback";
import { useAuthStore } from "@/lib/auth/store";
import { AlertTriangle } from "lucide-react";
import { AttentionPanel } from "./AttentionPanel";
import { MetricStrip } from "./MetricStrip";
import { ReplenishmentSummary } from "./ReplenishmentSummary";

/**
 * DashboardScreen — halaman landing operasional.
 * Menampilkan greeting header, KPI metric strip, tabel ringkasan pengisian ulang,
 * dan panel perhatian dalam layout grid responsif.
 *
 * Wrapped with ErrorBoundary to gracefully handle JSON load failures
 * with error indicator icon, message, and retry action.
 *
 * @validates Requirements 2.1, 2.5, 2.6, 2.7
 */
export function DashboardScreen() {
  return (
    <ErrorBoundary fallback={DashboardErrorFallback}>
      <DashboardContent />
    </ErrorBoundary>
  );
}

function DashboardContent() {
  const user = useAuthStore((s) => s.user);
  const firstName = user?.fullName?.split(" ")[0] ?? "Pengguna";

  const now = new Date();
  const greeting = getGreeting(now.getHours());
  const dateStr = formatDateID(now);

  return (
    <div className="py-6 max-[759px]:py-4">
      {/* Greeting header */}
      <header className="flex flex-col gap-1 mb-8 max-[759px]:mb-6">
        <p className="text-xs font-extrabold uppercase tracking-[0.09em] text-[var(--red-600)]">
          {dateStr}
        </p>
        <h1
          className="text-[clamp(1.85rem,3vw,2.55rem)] font-bold tracking-[-0.045em] text-[var(--n-900)]"
          style={{ textWrap: "balance" }}
        >
          {greeting}, {firstName}
        </h1>
        <p className="text-[var(--n-500)] max-w-[62ch] text-sm mt-1">
          Berikut ringkasan operasi kas Anda hari ini.
        </p>
      </header>

      <MetricStrip />

      <div className="grid grid-cols-1 min-[1080px]:grid-cols-[minmax(0,1.55fr)_minmax(300px,0.75fr)] gap-7 mt-8">
        <ReplenishmentSummary />
        <AttentionPanel />
      </div>
    </div>
  );
}

/**
 * Error fallback for dashboard — shows error indicator icon, descriptive message,
 * and retry action when JSON data fails to load or is malformed.
 *
 * @validates Requirements 2.6
 */
function DashboardErrorFallback({
  resetErrorBoundary,
}: { error: Error; resetErrorBoundary: () => void }) {
  return (
    <div role="alert" className="flex flex-col items-center justify-center gap-4 py-16 text-center">
      <div
        className="flex items-center justify-center w-14 h-14 rounded-full"
        style={{ backgroundColor: "var(--danger-bg)" }}
      >
        <AlertTriangle size={28} style={{ color: "var(--danger-fg)" }} aria-hidden="true" />
      </div>
      <h2 className="text-lg font-semibold" style={{ color: "var(--n-900)" }}>
        Gagal Memuat Dashboard
      </h2>
      <p className="max-w-md text-sm" style={{ color: "var(--n-600)" }}>
        Data dashboard tidak dapat dimuat. Periksa koneksi Anda atau coba muat ulang.
      </p>
      <button
        type="button"
        onClick={resetErrorBoundary}
        className="mt-2 inline-flex items-center gap-2 rounded-[var(--radius-md)] px-4 py-2 text-sm font-medium text-white transition-colors duration-150"
        style={{ backgroundColor: "var(--red-500)" }}
        onMouseEnter={(e) => {
          e.currentTarget.style.backgroundColor = "var(--red-600)";
        }}
        onMouseLeave={(e) => {
          e.currentTarget.style.backgroundColor = "var(--red-500)";
        }}
      >
        Coba Lagi
      </button>
    </div>
  );
}

/**
 * Get time-of-day greeting in Bahasa Indonesia based on hour (0–23).
 */
function getGreeting(hour: number): string {
  if (hour >= 0 && hour <= 11) return "Selamat pagi";
  if (hour >= 12 && hour <= 14) return "Selamat siang";
  if (hour >= 15 && hour <= 18) return "Selamat sore";
  return "Selamat malam";
}

/**
 * Format date in id-ID locale: "Selasa, 21 Juli 2026"
 */
function formatDateID(date: Date): string {
  return date.toLocaleDateString("id-ID", {
    weekday: "long",
    day: "numeric",
    month: "long",
    year: "numeric",
  });
}
