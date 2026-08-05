import { formatIDR } from "@/lib/utils/format";
import type { LucideIcon } from "lucide-react";

// ─── MetricCard ───────────────────────────────────────────────────────────────

export interface MetricCardProps {
  label: string;
  value: number;
  icon: LucideIcon;
  formatAsIDR?: boolean;
  accent?: "brand" | "success" | "warning" | "info";
  hint?: string;
}

const ACCENT_STYLES = {
  brand: { bg: "var(--red-50)", fg: "var(--red-500)" },
  success: { bg: "var(--success-bg)", fg: "var(--success-fg)" },
  warning: { bg: "var(--warning-bg)", fg: "var(--warning-fg)" },
  info: { bg: "var(--info-bg)", fg: "var(--info-fg)" },
} as const;

export function MetricCard({
  label,
  value,
  icon: Icon,
  formatAsIDR = false,
  accent = "brand",
  hint,
}: MetricCardProps) {
  const formattedValue = formatAsIDR ? formatIDR(value) : value.toLocaleString("id-ID");
  const accentStyle = ACCENT_STYLES[accent];

  return (
    <div
      className="flex flex-col gap-[var(--space-4)] overflow-hidden p-[var(--space-4)]"
      style={{
        backgroundColor: "var(--n-0)",
        borderRadius: "var(--radius-lg)",
        border: "1px solid var(--n-200)",
      }}
    >
      {/* Top row: label + icon */}
      <div className="flex items-center justify-between gap-[var(--space-2)]">
        <span
          className="min-w-0 truncate text-[13px] font-medium"
          style={{ color: "var(--n-500)" }}
        >
          {label}
        </span>
        <div
          className="flex h-9 w-9 shrink-0 items-center justify-center rounded-[var(--radius-md)]"
          style={{ backgroundColor: accentStyle.bg, color: accentStyle.fg }}
        >
          <Icon size={18} aria-hidden="true" />
        </div>
      </div>

      {/* Value */}
      <div className="flex flex-col gap-[2px]">
        <span
          className="text-[28px] font-bold leading-none tabular-nums"
          style={{ color: "var(--n-900)" }}
        >
          {formattedValue}
        </span>
        {hint && (
          <span className="text-xs" style={{ color: "var(--n-400)" }}>
            {hint}
          </span>
        )}
      </div>
    </div>
  );
}
