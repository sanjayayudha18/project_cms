/** Largest export the worker accepts (Req 5.6). */
export const MAX_EXPORT_ROWS = 1_000_000;

export interface ExportButtonInput {
  /** Draft filter differs from the Applied_Filter. */
  dirty: boolean;
  /** The table behind the button is (re)loading. */
  loading: boolean;
  total: number;
}

export interface ExportButtonState {
  enabled: boolean;
  /** Why the button is disabled; `null` when enabled. */
  message: string | null;
}

/** Enabled iff `!dirty && !loading && 0 < total <= MAX_EXPORT_ROWS` (Req 5.4-5.6). */
export function exportButtonState({ dirty, loading, total }: ExportButtonInput): ExportButtonState {
  if (dirty) {
    return { enabled: false, message: "Klik Search untuk memperbarui tabel sebelum mengekspor." };
  }
  if (loading) return { enabled: false, message: "Menunggu tabel selesai dimuat" };
  if (total <= 0) return { enabled: false, message: "Tidak ada baris untuk diekspor" };
  if (total > MAX_EXPORT_ROWS) {
    return {
      enabled: false,
      message: "Lebih dari 1.000.000 baris. Persempit filter agar ekspor bisa dibuat.",
    };
  }
  return { enabled: true, message: null };
}
