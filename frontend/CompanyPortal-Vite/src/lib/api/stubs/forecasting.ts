// ─── DSR Upload Mock Data ─────────────────────────────────────────────────────

interface DSRUploadRecord {
  id: string;
  date: string;
  filename: string;
  rowCount: number;
  status: "accepted" | "rejected" | "processing";
}

function generatePastDate(daysAgo: number): string {
  const date = new Date();
  date.setDate(date.getDate() - daysAgo);
  return date.toISOString().split("T")[0] ?? "";
}

const MOCK_DSR_UPLOADS: DSRUploadRecord[] = Array.from({ length: 30 }, (_, i) => {
  const rand = ((i * 7 + 3) % 100) / 100; // deterministic pseudo-random
  let status: DSRUploadRecord["status"] = "accepted";
  if (rand > 0.85) {
    status = "processing";
  } else if (rand > 0.7) {
    status = "rejected";
  }

  return {
    id: `dsr-${String(i + 1).padStart(3, "0")}`,
    date: generatePastDate(i),
    filename: `DSR_${generatePastDate(i).replace(/-/g, "")}_${i % 2 === 0 ? "Jabodetabek" : "Outregion"}.xlsx`,
    rowCount: 150 + ((i * 13 + 7) % 200),
    status,
  };
});

// ─── Handlers ─────────────────────────────────────────────────────────────────

export function handleDSRUploads(): Response {
  return new Response(JSON.stringify(MOCK_DSR_UPLOADS), {
    status: 200,
    headers: { "Content-Type": "application/json" },
  });
}

export function handleDSRSubmit(_body: unknown): Response {
  return new Response(
    JSON.stringify({
      id: `dsr-${Date.now()}`,
      timestamp: new Date().toISOString(),
      rowCount: 245,
      status: "accepted" as const,
    }),
    {
      status: 200,
      headers: { "Content-Type": "application/json" },
    },
  );
}
