/**
 * Ephemeral (not persisted) handoff from the Forecast Browser to the
 * Vendor Request creation page (Req 11.6: "client-side route state").
 * TanStack Router v1's navigate() has no generic history-state option
 * (unlike React Router), so this plays that role instead — same pattern as
 * the existing Zustand auth store (src/lib/auth/store.ts), just scoped to
 * one in-memory value the Create page reads once on mount and clears.
 */

import { create } from "zustand";
import type { ForecastRow } from "./types";

export interface PendingVendorRequestSelection {
  forecastDate: string;
  items: ForecastRow[];
  /** Resolved from the Forecast Browser's required FLM Vendor filter (CIT-2
   * Req 4, Q2): every create — DMAA-backed or manual — is constrained to one
   * vendor, and the create payload always needs its numeric id. */
  vendorId: number;
}

interface PendingSelectionState {
  pending: PendingVendorRequestSelection | null;
  setPending: (selection: PendingVendorRequestSelection) => void;
  clearPending: () => void;
}

export const usePendingVendorRequestSelection = create<PendingSelectionState>((set) => ({
  pending: null,
  setPending: (selection) => set({ pending: selection }),
  clearPending: () => set({ pending: null }),
}));
