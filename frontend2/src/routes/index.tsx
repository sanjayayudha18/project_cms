import { SkeletonCard } from "@/components/feedback/Skeleton";
import { ActivityFeed } from "@/features/dashboard/components/ActivityFeed";
import { MetricCard } from "@/features/dashboard/components/MetricCard";
import { useDashboardActivity, useDashboardMetrics } from "@/features/dashboard/hooks/useDashboard";
import { useAuthStore } from "@/lib/auth/store";
import { createRoute } from "@tanstack/react-router";
import {
  AlertTriangle,
  CheckCircle2,
  FileText,
  GitCompare,
  Monitor,
  RefreshCw,
} from "lucide-react";
import { protectedRoute } from "./_protected";

export const indexRoute = createRoute({
  path: "/",
  getParentRoute: () => protectedRoute,
  component: DashboardPage,
});

function getGreeting(): string {
  const hour = new Date().getHours();
  if (hour < 11) return "Selamat pagi";
  if (hour < 15) return "Selamat siang";
  if (hour < 19) return "Selamat sore";
  return "Selamat malam";
}

function formatToday(): string {
  return new Date().toLocaleDateString("id-ID", {
    weekday: "long",
    day: "numeric",
    month: "long",
    year: "numeric",
  });
}

function DashboardPage() {
  const metrics = useDashboardMetrics();
  const activity = useDashboardActivity();
  const user = useAuthStore((s) => s.user);

  const firstName = user?.fullName?.split(" ")[0] ?? "Pengguna";

  return (
    <div className="flex flex-col gap-[var(--space-6)]">
      {/* Greeting header */}
      <div className="flex flex-col gap-[var(--space-1)]">
        <h1
          className="text-2xl font-semibold tracking-tight"
          style={{ color: "var(--n-900)" }}
        >
          {getGreeting()}, {firstName}
        </h1>
        <p className="text-sm" style={{ color: "var(--n-500)" }}>
          {formatToday()}
        </p>
      </div>

      {/* Metrics section */}
      {metrics.isError ? (
        <MetricsError onRetry={() => metrics.refetch()} />
      ) : metrics.isLoading ? (
        <MetricsSkeleton />
      ) : metrics.data ? (
        <div className="grid grid-cols-1 gap-[var(--space-4)] sm:grid-cols-2 lg:grid-cols-4">
          <MetricCard
            label="Mesin ATM/CRM Aktif"
            value={metrics.data.activeMachines}
            icon={Monitor}
            accent="brand"
            hint="Total unit beroperasi"
          />
          <MetricCard
            label="Instruksi Pengisian"
            value={metrics.data.pendingFillInstructions}
            icon={FileText}
            accent="info"
            hint="Menunggu hari ini"
          />
          <MetricCard
            label="Rekonsiliasi Terbuka"
            value={metrics.data.openReconciliationItems}
            icon={GitCompare}
            accent="warning"
            hint="Perlu ditindaklanjuti"
          />
          <MetricCard
            label="Persetujuan Menunggu"
            value={metrics.data.pendingApprovals}
            icon={CheckCircle2}
            accent="success"
            hint="Dalam antrean D-3"
          />
        </div>
      ) : null}

      {/* Activity feed section — wrapped in a card */}
      <section
        className="flex flex-col overflow-hidden"
        style={{
          backgroundColor: "var(--n-0)",
          borderRadius: "var(--radius-lg)",
          border: "1px solid var(--n-200)",
        }}
      >
        <div
          className="flex items-center justify-between px-[var(--space-4)] py-[var(--space-3)]"
          style={{ borderBottom: "1px solid var(--n-100)" }}
        >
          <h2 className="text-base font-semibold" style={{ color: "var(--n-900)" }}>
            Aktivitas Terbaru
          </h2>
          <span className="text-xs" style={{ color: "var(--n-400)" }}>
            10 aktivitas terakhir
          </span>
        </div>

        <div className="px-[var(--space-4)] py-[var(--space-2)]">
          {activity.isLoading ? (
            <ActivitySkeleton />
          ) : activity.isError ? (
            <ActivityError onRetry={() => activity.refetch()} />
          ) : activity.data ? (
            <ActivityFeed events={activity.data} />
          ) : null}
        </div>
      </section>
    </div>
  );
}

// ─── Error States ─────────────────────────────────────────────────────────────

function MetricsError({ onRetry }: { onRetry: () => void }) {
  return (
    <div
      className="flex items-center gap-[var(--space-3)] rounded-[var(--radius-lg)] p-[var(--space-4)]"
      style={{ backgroundColor: "var(--danger-bg)" }}
      role="alert"
    >
      <AlertTriangle size={20} style={{ color: "var(--danger-fg)" }} aria-hidden="true" />
      <p className="flex-1 text-sm" style={{ color: "var(--danger-fg)" }}>
        Gagal memuat data metrik. Silakan coba lagi.
      </p>
      <button
        type="button"
        onClick={onRetry}
        className="inline-flex items-center gap-[var(--space-2)] rounded-[var(--radius-md)] px-[var(--space-3)] py-[var(--space-2)] text-sm font-medium transition-colors duration-150"
        style={{
          color: "var(--danger-fg)",
          backgroundColor: "var(--n-0)",
          border: "1px solid var(--danger-fg)",
        }}
      >
        <RefreshCw size={14} aria-hidden="true" />
        Coba Lagi
      </button>
    </div>
  );
}

function ActivityError({ onRetry }: { onRetry: () => void }) {
  return (
    <div
      className="my-[var(--space-3)] flex items-center gap-[var(--space-3)] rounded-[var(--radius-md)] p-[var(--space-4)]"
      style={{ backgroundColor: "var(--danger-bg)" }}
      role="alert"
    >
      <AlertTriangle size={20} style={{ color: "var(--danger-fg)" }} aria-hidden="true" />
      <p className="flex-1 text-sm" style={{ color: "var(--danger-fg)" }}>
        Gagal memuat aktivitas terbaru.
      </p>
      <button
        type="button"
        onClick={onRetry}
        className="inline-flex items-center gap-[var(--space-2)] rounded-[var(--radius-md)] px-[var(--space-3)] py-[var(--space-2)] text-sm font-medium transition-colors duration-150"
        style={{
          color: "var(--danger-fg)",
          backgroundColor: "var(--n-0)",
          border: "1px solid var(--danger-fg)",
        }}
      >
        <RefreshCw size={14} aria-hidden="true" />
        Coba Lagi
      </button>
    </div>
  );
}

// ─── Loading Skeletons ────────────────────────────────────────────────────────

function MetricsSkeleton() {
  return (
    <div className="grid grid-cols-1 gap-[var(--space-4)] sm:grid-cols-2 lg:grid-cols-4">
      <SkeletonCard />
      <SkeletonCard />
      <SkeletonCard />
      <SkeletonCard />
    </div>
  );
}

function ActivitySkeleton() {
  return (
    <div className="flex flex-col gap-[var(--space-3)] py-[var(--space-3)]">
      {Array.from({ length: 5 }, (_, i) => (
        <div key={i} className="flex items-start gap-[var(--space-3)]">
          <div
            className="h-8 w-8 shrink-0 animate-pulse rounded-full"
            style={{ backgroundColor: "var(--n-200)" }}
          />
          <div className="flex flex-1 flex-col gap-[var(--space-1)]">
            <div
              className="h-4 w-3/4 animate-pulse rounded"
              style={{ backgroundColor: "var(--n-200)" }}
            />
            <div
              className="h-3 w-1/3 animate-pulse rounded"
              style={{ backgroundColor: "var(--n-200)" }}
            />
          </div>
        </div>
      ))}
    </div>
  );
}
