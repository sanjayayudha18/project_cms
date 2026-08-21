/**
 * Total/Critical/Low/Normal summary cards (Req 3.3, 3.4), in a grid that's
 * 4 columns >=1024px, 2 columns 768-1023px, 1 column below 768px (Req
 * 152.1). Each card carries an aria-label with metric name + value (Req
 * 165.3, e.g. "Total ATM: 150"). Loading renders 4 SkeletonCards (Req
 * 124.1) in the same grid, so layout doesn't shift once data arrives.
 */

import { SkeletonCard } from "@/components/feedback/Skeleton";
import type { BadgeVariant } from "@/components/ui/Badge";
import type { AtmSummary } from "../types";

const VALUE_COLOR_CLASS: Record<BadgeVariant, string> = {
  success: "text-[var(--success-fg)]",
  warning: "text-[var(--warning-fg)]",
  danger: "text-[var(--danger-fg)]",
  info: "text-[var(--info-fg)]",
  neutral: "text-[var(--n-900)]",
};

const SUMMARY_CARDS: ReadonlyArray<{
  key: keyof AtmSummary;
  label: string;
  variant: BadgeVariant;
}> = [
  { key: "total", label: "Total ATM", variant: "neutral" },
  { key: "critical", label: "Critical", variant: "danger" },
  { key: "low", label: "Low", variant: "warning" },
  { key: "normal", label: "Normal", variant: "success" },
];

interface SummaryCardsGridProps {
  summary?: AtmSummary;
  isLoading?: boolean;
}

export function SummaryCardsGrid({ summary, isLoading = false }: SummaryCardsGridProps) {
  return (
    <div className="grid grid-cols-1 gap-4 min-[768px]:grid-cols-2 min-[1024px]:grid-cols-4">
      {isLoading
        ? SUMMARY_CARDS.map((card) => <SkeletonCard key={card.key} />)
        : SUMMARY_CARDS.map((card) => {
            const value = summary?.[card.key] ?? 0;
            return (
              <div
                key={card.key}
                // biome-ignore lint/a11y/useSemanticElements: <fieldset> groups form controls, not read-only stat content — role="group" is the correct WAI-ARIA pattern here
                role="group"
                aria-label={`${card.label}: ${value}`}
                className="rounded-[var(--radius-lg)] bg-[var(--n-0)] p-4 shadow-sm"
              >
                <p className="text-sm font-medium text-[var(--n-600)]">{card.label}</p>
                <p
                  className={`mt-1 text-2xl font-semibold tabular-nums ${VALUE_COLOR_CLASS[card.variant]}`}
                >
                  {value}
                </p>
              </div>
            );
          })}
    </div>
  );
}
