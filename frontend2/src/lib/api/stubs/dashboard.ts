// ─── Dashboard Mock Data ──────────────────────────────────────────────────────

interface DashboardMetrics {
  activeMachines: number;
  pendingFillInstructions: number;
  openReconciliationItems: number;
  pendingApprovals: number;
}

interface ActivityEvent {
  id: string;
  type: "upload" | "approval" | "reconciliation" | "generation";
  description: string;
  timestamp: string;
  actor: string;
}

const MOCK_METRICS: DashboardMetrics = {
  activeMachines: 2847,
  pendingFillInstructions: 156,
  openReconciliationItems: 23,
  pendingApprovals: 12,
};

function generateRecentTimestamp(minutesAgo: number): string {
  const date = new Date();
  date.setMinutes(date.getMinutes() - minutesAgo);
  return date.toISOString();
}

const MOCK_ACTIVITY: ActivityEvent[] = [
  {
    id: "act-001",
    type: "upload",
    description: "DSR harian berhasil diunggah untuk region Jabodetabek",
    timestamp: generateRecentTimestamp(5),
    actor: "Andi Pratama",
  },
  {
    id: "act-002",
    type: "approval",
    description: "Instruksi pengisian D-3 disetujui untuk 45 ATM",
    timestamp: generateRecentTimestamp(12),
    actor: "Budi Santoso",
  },
  {
    id: "act-003",
    type: "reconciliation",
    description: "Rekonsiliasi invoice CIT selesai — 3 selisih ditemukan",
    timestamp: generateRecentTimestamp(25),
    actor: "Citra Dewi",
  },
  {
    id: "act-004",
    type: "generation",
    description: "Proyeksi H+2 berhasil dibuat untuk 2.847 mesin",
    timestamp: generateRecentTimestamp(38),
    actor: "Sistem",
  },
  {
    id: "act-005",
    type: "upload",
    description: "Invoice CPC periode Juni diunggah oleh vendor FLM",
    timestamp: generateRecentTimestamp(55),
    actor: "Dian Nugroho",
  },
  {
    id: "act-006",
    type: "approval",
    description: "Berita acara cash count vault BSD disetujui",
    timestamp: generateRecentTimestamp(72),
    actor: "Eka Putri",
  },
  {
    id: "act-007",
    type: "reconciliation",
    description: "Rekonsiliasi escrow vault Tangerang selesai — status Close",
    timestamp: generateRecentTimestamp(90),
    actor: "Fajar Rahman",
  },
  {
    id: "act-008",
    type: "generation",
    description: "Surat konfirmasi pendebetan berhasil dibuat untuk 5 vendor",
    timestamp: generateRecentTimestamp(120),
    actor: "Sistem",
  },
  {
    id: "act-009",
    type: "upload",
    description: "DSR harian diunggah untuk region Jawa Timur",
    timestamp: generateRecentTimestamp(145),
    actor: "Gunawan Hadi",
  },
  {
    id: "act-010",
    type: "approval",
    description: "Jadwal cash count bulan Juli disetujui oleh Team Lead",
    timestamp: generateRecentTimestamp(180),
    actor: "Hendra Wijaya",
  },
];

// ─── Handlers ─────────────────────────────────────────────────────────────────

export function handleDashboardMetrics(): Response {
  return new Response(JSON.stringify(MOCK_METRICS), {
    status: 200,
    headers: { "Content-Type": "application/json" },
  });
}

export function handleDashboardActivity(): Response {
  return new Response(JSON.stringify(MOCK_ACTIVITY), {
    status: 200,
    headers: { "Content-Type": "application/json" },
  });
}
