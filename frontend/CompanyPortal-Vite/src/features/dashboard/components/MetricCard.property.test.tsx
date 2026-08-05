import { render, screen } from "@testing-library/react";
import * as fc from "fast-check";
import { CheckCircle2, FileText, GitCompare, Monitor } from "lucide-react";
import { describe, expect, it } from "vitest";
import type { DashboardMetrics } from "../types";
import { MetricCard } from "./MetricCard";

/**
 * Property 7: Dashboard Metrics Completeness
 * Validates: Requirements 4.2
 *
 * For any valid DashboardMetrics object (where all four fields are non-negative integers),
 * rendering the MetricCards component SHALL produce output containing all four metric values
 * (activeMachines, pendingFillInstructions, openReconciliationItems, pendingApprovals)
 * as formatted strings.
 */
describe("Property 7: Dashboard Metrics Completeness", () => {
  const arbDashboardMetrics: fc.Arbitrary<DashboardMetrics> = fc.record({
    activeMachines: fc.nat(),
    pendingFillInstructions: fc.nat(),
    openReconciliationItems: fc.nat(),
    pendingApprovals: fc.nat(),
  });

  it("all four metric values appear as formatted strings when MetricCards are rendered", () => {
    fc.assert(
      fc.property(arbDashboardMetrics, (metrics) => {
        const { unmount } = render(
          <div>
            <MetricCard label="Mesin ATM/CRM Aktif" value={metrics.activeMachines} icon={Monitor} />
            <MetricCard
              label="Instruksi Pengisian Hari Ini"
              value={metrics.pendingFillInstructions}
              icon={FileText}
            />
            <MetricCard
              label="Item Rekonsiliasi Terbuka"
              value={metrics.openReconciliationItems}
              icon={GitCompare}
            />
            <MetricCard
              label="Persetujuan Menunggu"
              value={metrics.pendingApprovals}
              icon={CheckCircle2}
            />
          </div>,
        );

        const expectedValues = [
          metrics.activeMachines.toLocaleString("id-ID"),
          metrics.pendingFillInstructions.toLocaleString("id-ID"),
          metrics.openReconciliationItems.toLocaleString("id-ID"),
          metrics.pendingApprovals.toLocaleString("id-ID"),
        ];

        // Each formatted value must appear at least once in the rendered output.
        // Using getAllByText to handle cases where two metrics share the same value.
        for (const value of expectedValues) {
          expect(screen.getAllByText(value).length).toBeGreaterThanOrEqual(1);
        }

        unmount();
      }),
      { numRuns: 50 },
    );
  });
});
