import { useQuery } from "@tanstack/react-query";
import { Banknote, Cpu, Landmark, Truck } from "lucide-react";

import { CASH_FLOW_QUERY_KEY, CASH_FLOW_STALE_TIME, VENDOR_COLORS } from "./constants";
import type { CashFlowSummary, UseCashFlowDataReturn } from "./types";

/**
 * Data mock sesuai prototipe ClickUp.
 * Akan diganti dengan data real API dari: GET /api/v1/cash-flow/summary
 */
const MOCK_CASH_FLOW_DATA: CashFlowSummary = {
  stats: [
    {
      label: "Total Kas Beredar",
      icon: Banknote,
      value: "Rp 48,2 M",
      trend: { direction: "up", percentage: 2.4 },
    },
    {
      label: "Saldo Vault Vendor",
      icon: Landmark,
      value: "Rp 21,7 M",
    },
    {
      label: "Kas di Mesin ATM",
      icon: Cpu,
      value: "Rp 26,5 M",
      trend: { direction: "down", percentage: 1.1 },
    },
    {
      label: "Drop CIT Hari Ini",
      icon: Truck,
      value: "Rp 3,9 M",
      subtitle: "6 order",
    },
  ],
  vendorChart: {
    data: [
      { date: "2026-07-15", Abacus: 8.2, "Bijak Jakarta": 6.1, Advantage: 5.4, SSI: 3.9 },
      { date: "2026-07-16", Abacus: 7.5, "Bijak Jakarta": 6.8, Advantage: 5.1, SSI: 4.1 },
      { date: "2026-07-17", Abacus: 9.0, "Bijak Jakarta": 5.9, Advantage: 6.2, SSI: 3.7 },
      { date: "2026-07-18", Abacus: 8.8, "Bijak Jakarta": 7.2, Advantage: 5.8, SSI: 4.3 },
      { date: "2026-07-19", Abacus: 7.9, "Bijak Jakarta": 6.5, Advantage: 5.6, SSI: 3.5 },
      { date: "2026-07-20", Abacus: 8.5, "Bijak Jakarta": 6.3, Advantage: 5.9, SSI: 4.0 },
      { date: "2026-07-21", Abacus: 9.2, "Bijak Jakarta": 7.0, Advantage: 6.0, SSI: 4.2 },
    ],
    vendors: VENDOR_COLORS,
  },
  atmLevels: [
    { id: "ATM-00417", label: "ATM-00417", percentage: 82 },
    { id: "ATM-00523", label: "ATM-00523", percentage: 65 },
    { id: "ATM-00189", label: "ATM-00189", percentage: 43 },
    { id: "ATM-00671", label: "ATM-00671", percentage: 28 },
    { id: "ATM-00302", label: "ATM-00302", percentage: 15 },
    { id: "ATM-00845", label: "ATM-00845", percentage: 8 },
  ],
};

/**
 * Fungsi fetch mock — mengembalikan data statis sesuai prototipe ClickUp.
 * Akan diganti dengan: GET /api/v1/cash-flow/summary
 */
async function fetchCashFlowSummary(): Promise<CashFlowSummary> {
  await new Promise((resolve) => setTimeout(resolve, 300));
  return MOCK_CASH_FLOW_DATA;
}

/**
 * Hook TanStack Query untuk data monitoring cash flow.
 *
 * Menggunakan query key ['cash-flow', 'summary'] dengan stale time 5 menit.
 * Saat ini mengembalikan data mock; akan terhubung ke Go backend API setelah tersedia.
 */
export function useCashFlowData(): UseCashFlowDataReturn {
  const query = useQuery<CashFlowSummary>({
    queryKey: CASH_FLOW_QUERY_KEY,
    queryFn: fetchCashFlowSummary,
    staleTime: CASH_FLOW_STALE_TIME,
  });

  return {
    data: query.data,
    isLoading: query.isLoading,
    isError: query.isError,
    error: query.error,
    refetch: query.refetch,
  };
}
