import type { LucideIcon } from "lucide-react";

/** Arah indikator tren */
export type TrendDirection = "up" | "down";

/** Tier warna semantik untuk level kas ATM */
export type CashLevelTier = "success" | "warning" | "danger";

/** Data satu kartu KPI statistik */
export interface StatsCardData {
  readonly label: string;
  readonly icon: LucideIcon;
  readonly value: string;
  readonly subtitle?: string;
  readonly trend?: {
    readonly direction: TrendDirection;
    readonly percentage: number;
  };
}

/** Satu titik data chart — arus kas harian per vendor */
export interface VendorDayFlow {
  readonly date: string; // ISO date string (YYYY-MM-DD)
  readonly [vendorName: string]: number | string; // jumlah vendor + kunci tanggal
}

/** Metadata vendor untuk legenda chart dan pemetaan warna */
export interface VendorConfig {
  readonly name: string;
  readonly color: string; // Nilai warna OKLCH dari design tokens
}

/** Satu baris level kas ATM */
export interface AtmLevel {
  readonly id: string; // e.g. "ATM-00417"
  readonly label: string; // Identifier tampilan
  readonly percentage: number; // 0–100
}

/** Struktur data lengkap dari useCashFlowData */
export interface CashFlowSummary {
  readonly stats: readonly StatsCardData[];
  readonly vendorChart: {
    readonly data: readonly VendorDayFlow[];
    readonly vendors: readonly VendorConfig[];
  };
  readonly atmLevels: readonly AtmLevel[];
}

/** Tipe return hook yang membungkus state TanStack Query */
export interface UseCashFlowDataReturn {
  readonly data: CashFlowSummary | undefined;
  readonly isLoading: boolean;
  readonly isError: boolean;
  readonly error: Error | null;
  readonly refetch: () => void;
}
