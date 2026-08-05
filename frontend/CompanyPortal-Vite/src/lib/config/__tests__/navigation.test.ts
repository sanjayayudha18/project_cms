import {
  Activity,
  BarChart3,
  FileText,
  GitCompare,
  LayoutDashboard,
  TrendingUp,
  Truck,
} from "lucide-react";
import { describe, expect, it } from "vitest";
import { NAV_CONFIG } from "../navigation";

// ─── 8 Ported Module Entries ──────────────────────────────────────────────────

/**
 * Unit tests for NAV_CONFIG entries added by the frontend consolidation.
 * Validates: Requirements 10.1, 10.4, 10.5
 */

const REQUIRED_ENTRY_IDS = [
  "dashboard",
  "cash-flow",
  "cit",
  "dsr-dashboard",
  "forecast",
  "invoice-list",
  "reconciliation",
  "replenishment",
] as const;

describe("NAV_CONFIG — 8 ported module entries", () => {
  it("contains all 8 required entries", () => {
    const configIds = NAV_CONFIG.map((item) => item.id);
    for (const id of REQUIRED_ENTRY_IDS) {
      expect(configIds).toContain(id);
    }
  });

  // ── Dashboard ─────────────────────────────────────────────────────────────

  describe("dashboard entry", () => {
    const entry = NAV_CONFIG.find((item) => item.id === "dashboard");

    it("exists", () => {
      expect(entry).toBeDefined();
    });

    it("has correct href", () => {
      expect(entry?.href).toBe("/");
    });

    it("has correct label", () => {
      expect(entry?.label).toBe("Dashboard");
    });

    it("has correct icon", () => {
      expect(entry?.icon).toBe(LayoutDashboard);
    });

    it("is visible to all authenticated users (wildcard role)", () => {
      expect(entry?.roles).toContain("*");
    });
  });

  // ── Cash Flow ─────────────────────────────────────────────────────────────

  describe("cash-flow entry", () => {
    const entry = NAV_CONFIG.find((item) => item.id === "cash-flow");

    it("exists", () => {
      expect(entry).toBeDefined();
    });

    it("has correct href", () => {
      expect(entry?.href).toBe("/cash-flow");
    });

    it("has correct label", () => {
      expect(entry?.label).toBe("Cash Flow Monitoring");
    });

    it("has correct icon", () => {
      expect(entry?.icon).toBe(Activity);
    });

    it("has correct roles", () => {
      expect(entry?.roles).toEqual(["ATM_Support", "Cash_Management"]);
    });
  });

  // ── CIT Tracker ───────────────────────────────────────────────────────────

  describe("cit entry", () => {
    const entry = NAV_CONFIG.find((item) => item.id === "cit");

    it("exists", () => {
      expect(entry).toBeDefined();
    });

    it("has correct href", () => {
      expect(entry?.href).toBe("/cit");
    });

    it("has correct label", () => {
      expect(entry?.label).toBe("CIT Tracker");
    });

    it("has correct icon", () => {
      expect(entry?.icon).toBe(Truck);
    });

    it("has correct roles", () => {
      expect(entry?.roles).toEqual(["ATM_Support", "Cash_Management"]);
    });
  });

  // ── DSR Dashboard ─────────────────────────────────────────────────────────

  describe("dsr-dashboard entry", () => {
    const entry = NAV_CONFIG.find((item) => item.id === "dsr-dashboard");

    it("exists", () => {
      expect(entry).toBeDefined();
    });

    it("has correct href", () => {
      expect(entry?.href).toBe("/forecasting/dsr-dashboard");
    });

    it("has correct label", () => {
      expect(entry?.label).toBe("DSR Dashboard");
    });

    it("has correct icon", () => {
      expect(entry?.icon).toBe(BarChart3);
    });

    it("has correct roles", () => {
      expect(entry?.roles).toEqual(["ATM_Support", "Cash_Management", "Vendor"]);
    });
  });

  // ── Forecast ──────────────────────────────────────────────────────────────

  describe("forecast entry", () => {
    const entry = NAV_CONFIG.find((item) => item.id === "forecast");

    it("exists", () => {
      expect(entry).toBeDefined();
    });

    it("has correct href", () => {
      expect(entry?.href).toBe("/forecasting/forecast");
    });

    it("has correct label", () => {
      expect(entry?.label).toBe("Forecasting");
    });

    it("has correct icon", () => {
      expect(entry?.icon).toBe(TrendingUp);
    });

    it("has correct roles", () => {
      expect(entry?.roles).toEqual(["ATM_Support", "Cash_Management"]);
    });
  });

  // ── Invoice List ──────────────────────────────────────────────────────────

  describe("invoice-list entry", () => {
    const entry = NAV_CONFIG.find((item) => item.id === "invoice-list");

    it("exists", () => {
      expect(entry).toBeDefined();
    });

    it("has correct href", () => {
      expect(entry?.href).toBe("/invoice/list");
    });

    it("has correct label", () => {
      expect(entry?.label).toBe("Daftar Invoice");
    });

    it("has correct icon", () => {
      expect(entry?.icon).toBe(FileText);
    });

    it("has correct roles", () => {
      expect(entry?.roles).toEqual(["WMO", "Finance", "Vendor"]);
    });
  });

  // ── Reconciliation ────────────────────────────────────────────────────────

  describe("reconciliation entry", () => {
    const entry = NAV_CONFIG.find((item) => item.id === "reconciliation");

    it("exists", () => {
      expect(entry).toBeDefined();
    });

    it("has correct href", () => {
      expect(entry?.href).toBe("/invoice/reconciliation");
    });

    it("has correct label", () => {
      expect(entry?.label).toBe("Rekonsiliasi");
    });

    it("has correct icon", () => {
      expect(entry?.icon).toBe(GitCompare);
    });

    it("has correct roles", () => {
      expect(entry?.roles).toEqual(["WMO", "Finance"]);
    });

    it("is not disabled (disabled: false)", () => {
      expect(entry?.disabled).toBe(false);
    });
  });

  // ── Replenishment ─────────────────────────────────────────────────────────

  describe("replenishment entry", () => {
    const entry = NAV_CONFIG.find((item) => item.id === "replenishment");

    it("exists", () => {
      expect(entry).toBeDefined();
    });

    it("has correct href", () => {
      expect(entry?.href).toBe("/replenishment");
    });

    it("has correct label", () => {
      expect(entry?.label).toBe("Pengisian Ulang");
    });

    it("has correct icon", () => {
      expect(entry?.icon).toBe(Truck);
    });

    it("has correct roles", () => {
      expect(entry?.roles).toEqual(["ATM_Support", "Cash_Management"]);
    });
  });
});

// ─── No VendorPortal entries ──────────────────────────────────────────────────

describe("NAV_CONFIG — no VendorPortal entries", () => {
  const VENDOR_PORTAL_PATHS = [
    "/vendor-portal",
    "/VendorPortal",
    "/vendor/portal",
    "/vendor/orders",
    "/vendor/schedules",
    "/vendor/notifications",
    "/vendor/evidence",
  ];

  it("does not contain any entry pointing to VendorPortal paths", () => {
    for (const item of NAV_CONFIG) {
      for (const vendorPath of VENDOR_PORTAL_PATHS) {
        expect(item.href.toLowerCase()).not.toContain(vendorPath.toLowerCase());
      }
    }
  });

  it("no entry id contains 'vendor-portal' or 'vendorportal'", () => {
    for (const item of NAV_CONFIG) {
      expect(item.id.toLowerCase()).not.toContain("vendor-portal");
      expect(item.id.toLowerCase()).not.toContain("vendorportal");
    }
  });
});

// ─── Disabled entries verification ────────────────────────────────────────────

describe("NAV_CONFIG — disabled entries for ported modules", () => {
  it("reconciliation entry has disabled set to false", () => {
    const recon = NAV_CONFIG.find((item) => item.id === "reconciliation");
    expect(recon).toBeDefined();
    expect(recon?.disabled).toBe(false);
  });

  it("no ported module entry (8 required) has disabled set to true", () => {
    for (const id of REQUIRED_ENTRY_IDS) {
      const entry = NAV_CONFIG.find((item) => item.id === id);
      expect(entry).toBeDefined();
      // disabled should be undefined (not set) or explicitly false
      expect(entry?.disabled).not.toBe(true);
    }
  });
});
