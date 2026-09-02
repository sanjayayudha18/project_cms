import { Card } from "@/components/ui/Card";
import type { CITOrder } from "@/lib/types";
import { CheckCircle, Package, Truck, XCircle } from "lucide-react";

interface OrderSummaryBarProps {
  readonly orders: readonly CITOrder[];
}

const statusConfig = [
  { status: "Scheduled" as const, label: "Scheduled", icon: Package, colorClass: "text-info-fg" },
  {
    status: "In Transit" as const,
    label: "In Transit",
    icon: Truck,
    colorClass: "text-warning-fg",
  },
  {
    status: "Completed" as const,
    label: "Completed",
    icon: CheckCircle,
    colorClass: "text-success-fg",
  },
  { status: "Failed" as const, label: "Failed", icon: XCircle, colorClass: "text-danger-fg" },
] as const;

/**
 * Summary bar showing total counts per status from the UNFILTERED dataset.
 * Always reflects the full data regardless of active filters.
 */
export function OrderSummaryBar({ orders }: OrderSummaryBarProps) {
  const counts = {
    Scheduled: orders.filter((o) => o.status === "Scheduled").length,
    "In Transit": orders.filter((o) => o.status === "In Transit").length,
    Completed: orders.filter((o) => o.status === "Completed").length,
    Failed: orders.filter((o) => o.status === "Failed").length,
  };

  return (
    <div className="flex flex-wrap gap-3">
      {statusConfig.map(({ status, label, icon: Icon, colorClass }) => (
        <Card key={status} className="flex-1 min-w-[140px]">
          <div className="flex items-center gap-3">
            <Icon className={`size-5 ${colorClass}`} aria-hidden="true" />
            <div>
              <p className="text-xs font-medium uppercase tracking-wide text-neutral-500">
                {label}
              </p>
              <p className="text-lg font-semibold tabular-nums text-surface-text">
                {counts[status]}
              </p>
            </div>
          </div>
        </Card>
      ))}
    </div>
  );
}
