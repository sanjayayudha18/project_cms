# Plan: Forecast Browser — ringkasan vendor × region

Status: **accepted 2026-09-30** (user: "ok D1 primary setuju, lanjut implementasi"). Stage: 3 Build — **selesai 2026-09-30** (B1–B4, F1–F3, D sesuai plan; deviasi: `FilterSelect` dipakai ulang dari `components/ui`, tombol "Lihat" pakai teks sr-only karena `Button` tidak meneruskan `aria-label`).
Input: `spec.md` (accepted 2026-09-30). Trigger ke stage berikut: engineer menerima plan → implementasi → `tests.md`.

## Keputusan yang perlu disetujui (code ≠ spec)
**D1 — Pool untuk ringkasan.** Spec FR1 menulis "dibaca dari replica (`s.read`)", tetapi di kode `VendorRequestService.read`
= `db.New(pool)` dengan **pool primary** (`backend/internal/service/vendor_request.go:361`, `cmd/api/main.go:433`); replica
pool (`main.go:59-75`) opsional dan hanya dipakai export master data.
- **Usulan: pakai `s.read` (primary)**, sama dengan `BrowseForecast` hari ini. Alasan: ringkasan dipakai persis setelah
  user membuat Vendor Request (read-after-write — Sec 6 mengizinkan primary), dan replica lag akan membuat angka "Belum"
  tidak turun. Beban kecil (satu query agregat per buka halaman). Spec FR1/NFR dikoreksi setelah disetujui.
- Alternatif (tidak diambil): tambah replica querier opsional ke service → dua sumber data di satu halaman, bisa saling
  tidak konsisten (ringkasan dari replica, detail dari primary).

## Urutan kerja
Backend dulu (kontrak), lalu frontend. Setiap langkah diakhiri `go test ./...` / `pnpm test` hijau.

### B1 — SQL (`backend/queries/vendor_request.sql`)
1. `ListForecastForDate`: tambah param `unassigned bool` → `AND (NOT sqlc.arg('unassigned')::bool OR v.id IS NULL)`;
   tambah kolom
   `EXISTS (SELECT 1 FROM vendor_request_items vri JOIN vendor_requests vr ON vr.id = vri.vendor_request_id
   WHERE vri.terminal_id = f.terminal_id AND vri.periode_pred = f.periode_pred AND vri.denom = f.denom
   AND vr.status NOT IN ('cancelled','rejected') AND vr.is_canceled = false) AS is_requested`.
2. `CountForecastForDate`: filter `unassigned` yang sama + tie-breaker `avp.id DESC` di LATERAL (samakan dengan List).
3. Query baru `SummarizeForecastForDate :many` — satu query, 3 tahap:
   ```sql
   WITH r AS (   -- join persis seperti ListForecastForDate, tanpa filter selain tanggal
     SELECT f.terminal_id, f.amount_replenish, v.name AS flm_vendor, vb.region AS flm_vendor_region,
            EXISTS (...sama dengan di atas...) AS is_requested
     FROM dmaa_atm_forecast f LEFT JOIN atms a ... LEFT JOIN LATERAL (...) active_pkg ON true
     LEFT JOIN vendor_branches vb ... LEFT JOIN vendors v ...
     WHERE f.periode_pred = @forecast_date),
   atm AS (      -- 1 baris per ATM; ATM "sudah" hanya bila semua denom sudah
     SELECT terminal_id, flm_vendor, flm_vendor_region, bool_and(is_requested) AS all_requested,
            SUM(amount_replenish) AS amount,
            COALESCE(SUM(amount_replenish) FILTER (WHERE NOT is_requested), 0) AS unrequested_amount
     FROM r GROUP BY 1, 2, 3)
   SELECT COALESCE(flm_vendor,'') AS flm_vendor, COALESCE(flm_vendor_region,'') AS flm_vendor_region,
          COUNT(*) AS atm_count, COUNT(*) FILTER (WHERE all_requested) AS requested_atm_count,
          SUM(amount)::bigint AS amount_replenish, SUM(unrequested_amount)::bigint AS unrequested_amount_replenish
   FROM atm GROUP BY 1, 2
   ORDER BY (COUNT(*) - COUNT(*) FILTER (WHERE all_requested)) DESC, 1, 2;
   ```
   `totals` dihitung di Go dari penjumlahan grup (tanpa query kedua).
4. `cd backend && sqlc generate` (v1.31.1) → fix `UserLeafe` → `UserLeave` di `internal/db/`.

### B2 — Service (`internal/service/vendor_request.go`, `vendor_request_actions.go`)
- `BrowseForecastParams.Unassigned bool`; `ForecastRow.IsRequested bool`.
- `BrowseForecast`: hapus dua validasi "wajib dipilih" (CIT-2 Req 1.4); tambah validasi `Unassigned` + vendor/region
  non-kosong → `ValidationError{Field: "unassigned", ...}`. Batas 255 karakter tetap. Update komentar fungsi.
- Tipe baru `ForecastSummaryGroup`, `ForecastSummaryTotals`, `ForecastSummaryResult`; method baru
  `ForecastSummary(ctx, forecastDate string)` (validasi tanggal sama dengan BrowseForecast; `unassigned_atm_count` =
  `atm_count` grup ber-vendor kosong).
- `VendorRequestRepository` + `VendorRequestServicer` interface: tambah method baru.

### B3 — Handler (`internal/handler/vendor_request_handler.go`, `vendor_request_response.go`)
- Route `GET /forecast/summary` (viewer roles), handler `ForecastSummary`.
- `BrowseForecast`: parse `unassigned` (`""`/`false` → false, `true` → true, lainnya → 400 `bad_request`).
- Response: `forecastRowResponse.IsRequested bool \`json:"is_requested"\``; tipe `forecastSummaryResponse` (flat JSON per spec).

### B4 — Test backend
- `vendor_request_forecast_test.go`: fake repo + method baru; unit test validasi (vendor/region kosong → OK;
  unassigned + vendor → error; tanggal salah → error); totals = Σ grup; tanggal kosong → groups `[]`.
- `vendor_request_handler_test.go`: fake servicer + method baru; `/forecast/summary` 200/400; `unassigned=abc` → 400;
  JSON `is_requested` ada; RBAC: role non-viewer → 403.
- **Integration baru** `vendor_request_summary_integration_test.go` (skip bila `DATABASE_URL` kosong): seed 2 vendor/region
  + 1 ATM tanpa paket + ATM 2 denom; buat request draft/approved/cancelled/rejected → cek (a) Σ atm_count = total,
  (b) atm_count grup = distinct terminal dari ListForecastForDate dengan filter sama, (c) cancelled/rejected = belum,
  (d) ATM 2 denom dengan 1 denom di-request = belum, (e) `unassigned=true` hanya ATM tanpa paket, (f) Count = len(List).
- Test lama yang mengharapkan 400 untuk vendor/region kosong diubah (bukan dihapus) menjadi harapan 200.

### F1 — Data layer (`frontend/CompanyPortal-Vite/src/features/vendor-request/`)
- `types.ts`: `ForecastRow.is_requested`, `BrowseForecastParams.unassigned?`, `flmVendor?/flmVendorRegion?` opsional,
  tipe `ForecastSummary*`.
- `api.ts`: `buildForecastQuery` hanya set vendor/region bila terisi, set `unassigned=true` bila aktif;
  `fetchForecastSummary(date)`.
- `hooks.ts`: `useForecastSummary(date)` dengan key `["vendor-requests","forecast-summary",date]` (ikut invalidasi prefix).

### F2 — Komponen
- **Baru** `ForecastSummary.tsx` (+ test): KPI strip 4 kartu + tabel ringkasan; props `data`, `isLoading`, `isError`,
  `onRetry`, `activeKey`, `onSelectGroup(group)`. Status = ikon lucide + teks; baris = `<button>` di sel "Lihat"
  (keyboard/44 px), `aria-current` untuk grup aktif. Uang via `formatIDR`, `tabular-nums`.
- `ForecastTable.tsx`: kolom **Status** (badge "Sudah di-request"); `enableRowSelection: row => !row.original.is_requested
  && row.original.flm_vendor !== ""`; checkbox `disabled` mengikuti `row.getCanSelect()`; header select-all hanya baris
  yang bisa dipilih.
- `ForecastBrowser.tsx`:
  - Hapus blok "Pilih FLM Vendor dan FLM Vendor Region…" + gating `filtersReady`; tabel detail selalu tampil.
  - Select vendor/region: opsi pertama `Semua` (value `""`), tidak disabled; hapus tanda `*`.
  - State `unassigned` + chip "Hanya ATM tanpa vendor aktif ✕"; `onSelectGroup` mengisi vendor/region atau
    `unassigned`, reset page & pilihan, `scrollIntoView` ke tabel detail.
  - `selectedVendors = new Set(selected.map(r => r.flm_vendor))`; tombol Buat aktif bila `size === 1`; teks bantu bila
    `> 1`; `vendorId` dari `vendorOptions.find(v => v.name === [...selectedVendors][0])`.
  - "Pilih Semua Rekomendasi" disabled bila `flmVendor === ""`; hasil disaring `!row.is_requested`.
  - File sudah ~380 baris; ringkasan dipisah ke `ForecastSummary.tsx` agar tetap < 400 baris.

### F3 — Test frontend
- `ForecastBrowser.test.tsx`: test empty-state lama diganti → halaman memuat ringkasan + detail tanpa filter
  (request tidak memuat `flm_vendor`); klik grup → request memuat `flm_vendor`/`flm_vendor_region`; klik grup tanpa vendor
  → `unassigned=true`; pilihan 2 vendor → tombol Buat disabled + teks bantu; select-all melewati `is_requested`.
- `ForecastTable.test.tsx`: baris `is_requested` → badge + checkbox disabled; baris tanpa vendor → disabled.
- `ForecastSummary.test.tsx`: label status (bukan warna saja), format Rp, `aria-current`, callback `onSelectGroup`.

### D — Dokumen
- `spec.md` FR1/NFR: koreksi pool sesuai D1 (setelah disetujui).
- `CLAUDE.md` tidak berubah (tidak ada tabel/modul baru). `tests.md` ditulis di Stage 4.
- `graphify update .` setelah kode selesai.

## Risiko
| Risiko | Mitigasi |
|---|---|
| Angka ringkasan ≠ baris detail karena LATERAL berbeda | B1.2 samakan tie-breaker; integration test (b) & (f) |
| `dmaa_atm_forecast` punya baris ganda (terminal, tanggal, denom) dari >1 file | Perilaku sama dengan query lama (tidak di-dedupe); dicatat, tidak diubah di sini |
| Performa agregat (EXISTS per baris) | Index `vendor_request_items_terminal_idx` sudah ada; ukur `EXPLAIN ANALYZE` di dev, catat di `tests.md` |
| Request dobel tetap mungkin lewat API | Out of scope (spec); hanya penanda UI |
| Detail default tanpa filter → lebih banyak baris | Tetap berhalaman (maks 100/halaman), beban per request tidak berubah |

## Proof (dipetakan ke spec)
| Spec | Bukti |
|---|---|
| FR1 ringkasan + invarian | B4 integration (a)(b), handler test 200/400 |
| Definisi "sudah" (cancelled/rejected, semua denom) | B4 integration (c)(d) |
| FR2 filter opsional + unassigned | B4 unit + integration (e)(f), F3 |
| FR3 `is_requested` | B4 handler JSON, F3 ForecastTable |
| FR4 UI + single-vendor create | F3 ForecastBrowser |
| AC manual (klik-klik di browser) | **Outstanding — dilakukan user** (Golden Rule #10) |

Perintah: `cd backend && go test ./...`, `pnpm --dir frontend/CompanyPortal-Vite run test|lint|build`.

## Alternatif tidak diambil
- Endpoint terpisah untuk totals → dihitung di Go dari grup, satu query cukup.
- Kolom `request_number` di baris detail → YAGNI; badge cukup untuk mencegah request dobel dari UI.
