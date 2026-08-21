import type { AuthUser } from "../../auth/store";

// ─── Mock Auth Data ───────────────────────────────────────────────────────────
// ROLE Role	Sidebar Items
// Admin	Everything (all modules + Settings)
// ATM_Support	Beranda, all Forecasting items
// Cash_Management	Beranda, all Forecasting items
// Vendor	Beranda, Unggah DSR, Instruksi Pengisian, Unggah Invoice
// WMO	Beranda, all Invoice items
// Finance	Beranda, all Invoice items
// Cash_Count_PIC	Beranda, all Cash Count items
// Cash_Count_Lead	Beranda, all Cash Count items
// Branch	Beranda, Proyeksi H+2 only
// Approver	Beranda, all Forecasting + Invoice + Cash Count

const MOCK_USER: AuthUser = {
  id: 1,
  username: "yudha.rangga",
  fullName: "Yudha Rangga",
  email: "Raden.Yudha@cimb.co.id",
  role: "ADMIN",
  isKaryawan: true,
  vendorId: null,
};

const MOCK_ACCESS_TOKEN =
  "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJzdWIiOiJ1c3ItMDAxIiwicm9sZXMiOlsiVmVuZG9yIl0sImV4cCI6OTk5OTk5OTk5OX0.stub-signature";

// ─── Handlers ─────────────────────────────────────────────────────────────────

export function handleLogin(_body: unknown): Response {
  return new Response(
    JSON.stringify({
      user: MOCK_USER,
      accessToken: MOCK_ACCESS_TOKEN,
    }),
    {
      status: 200,
      headers: { "Content-Type": "application/json" },
    },
  );
}

export function handleRefresh(): Response {
  return new Response(
    JSON.stringify({
      user: MOCK_USER,
      accessToken: MOCK_ACCESS_TOKEN,
    }),
    {
      status: 200,
      headers: { "Content-Type": "application/json" },
    },
  );
}
