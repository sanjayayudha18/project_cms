# Spec: Forecast Browser — ringkasan vendor × region + penanda "sudah di-request"

Status: **accepted 2026-09-30** (user: "ok spec diterima"). Stage: 2 Design — selesai.
Input: `intent.md` (accepted 2026-09-30). Trigger ke stage berikut: product owner menerima file ini.
Mengubah spec lama: `.kiro/specs/done/cit-vendor-request-enhancements` **Req 1.4** (FLM Vendor + Region wajib → opsional).

## Definisi
- **Rekomendasi** = satu baris `dmaa_atm_forecast` (terminal_id, periode_pred, denom) pada `forecast_date`.
- **Vendor/region ATM** = hasil resolusi yang **sama persis** dengan `ListForecastForDate`: paket aktif
  (`atm_vendor_packages` aktif per tanggal, `ORDER BY effective_start_date DESC, avp.id DESC LIMIT 1`) →
  `vendor_packages_branch` → `vendor_branches.region` → `vendors.name`.
- **Tanpa vendor aktif** = rekomendasi yang resolusi vendornya NULL.
- **Rekomendasi sudah di-request** = ada `vendor_request_items` dengan (terminal_id, periode_pred, denom) sama, milik
  `vendor_requests` dengan `status NOT IN ('cancelled','rejected') AND is_canceled = false`.
- **ATM sudah di-request** = **semua** rekomendasi ATM itu pada tanggal tsb sudah di-request. Satu saja belum → ATM
  dihitung "belum" (supaya tidak ada yang terlewat).

## Functional requirements

### FR1 — Endpoint ringkasan (baru)
`GET /api/v1/vendor-requests/forecast/summary?forecast_date=YYYY-MM-DD`
- Role: `vendorRequestViewerRoles` (sama dengan `GET /forecast`). Tanpa pembatasan per cabang (intent OQ3).
- Validasi: `forecast_date` wajib + format + `validateDateBound` (sama dengan BrowseForecast) → 400 bila salah.
- Dibaca dari **primary** (`s.read`, sama dengan BrowseForecast — plan D1, disetujui 2026-09-30: read-after-write setelah create). Response flat JSON (Sec 5, tetangga handler ATM):
```json
{
  "forecast_date": "2026-10-01",
  "totals": {
    "atm_count": 412, "requested_atm_count": 287, "unrequested_atm_count": 125,
    "unassigned_atm_count": 6,
    "amount_replenish": 58300000000, "unrequested_amount_replenish": 17900000000
  },
  "groups": [
    { "flm_vendor": "PT ABC", "flm_vendor_region": "Jabodetabek",
      "atm_count": 84, "requested_atm_count": 40, "unrequested_atm_count": 44,
      "amount_replenish": 12400000000, "unrequested_amount_replenish": 6100000000 },
    { "flm_vendor": "", "flm_vendor_region": "", "atm_count": 6, "...": "..." }
  ]
}
```
- `groups` satu baris per (vendor, region); grup tanpa vendor aktif = `flm_vendor: ""`, `flm_vendor_region: ""`.
  Cabang vendor **tanpa region** (`vendor_branches.region` NULL) = grup `(V, "")` (review R1, 2026-09-30).
  Region dikelompokkan case-insensitive (`LOWER`), ditampilkan `MIN(region)` — sama dengan filter `LOWER(...)`.
  Urutan server: `unrequested_atm_count DESC, flm_vendor ASC, flm_vendor_region ASC`.
- Amount = `SUM(amount_replenish)` (bigint, IDR penuh) → `int64` di Go, tidak pernah float.
- `unrequested_amount_replenish` = jumlah amount rekomendasi yang **belum** di-request.
- Tanggal tanpa data → `totals` nol semua, `groups: []` (200, bukan 404).
- Invarian (dites): Σ `groups[].atm_count` = `totals.atm_count`; untuk grup (V,R), `atm_count` = jumlah terminal distinct
  dari `GET /forecast?flm_vendor=V&flm_vendor_region=R`.

### FR2 — `GET /forecast`: filter vendor/region opsional + filter tanpa vendor
- `flm_vendor`, `flm_vendor_region` **opsional** (kosong = Semua). Batas 255 karakter tetap.
- Parameter baru `unassigned=true` → hanya rekomendasi tanpa vendor aktif. Bila dikombinasikan dengan `flm_vendor` atau
  `flm_vendor_region` non-kosong → 400 `unassigned tidak bisa digabung dengan filter vendor/region`.
  Nilai selain `true`/`false`/kosong → 400.
- Parameter baru `no_region=true` (review R1) → hanya rekomendasi ber-vendor yang cabangnya tanpa region
  (`v.id IS NOT NULL AND vb.region IS NULL`). Tidak bisa digabung dengan `flm_vendor_region` atau `unassigned` → 400.
  Nilai selain `true`/`false`/kosong → 400.
- `CountForecastForDate` ikut filter yang sama, dan LATERAL-nya disamakan dengan `ListForecastForDate`
  (tambah tie-breaker `avp.id DESC`) supaya hitungan & baris tidak pernah beda vendor.

### FR3 — `GET /forecast`: kolom `is_requested`
- Tiap baris mendapat `is_requested: boolean` (definisi di atas, level rekomendasi). Additive — field lain tidak berubah.

### FR4 — UI halaman `/replenishment/forecast-browser`
Urutan dari atas:
1. **Tanggal forecast** (default H+1 kerja, seperti sekarang).
2. **KPI strip** (4 kartu): Perlu isi (`atm_count` + total IDR) · Sudah di-request · Belum di-request (+ IDR belum) ·
   Tanpa vendor aktif (ikon peringatan bila > 0).
3. **Tabel ringkasan** Vendor × Region, kolom: Vendor · Region · ATM · Total (IDR) · Sudah · Belum · Status.
   - Status berlabel + ikon (bukan warna saja, Sec 13): `Belum lengkap` (belum > 0), `Selesai` (belum = 0),
     `Perlu master data` (grup tanpa vendor).
   - Angka & uang `tabular-nums`, rata kanan, "Rp".
   - Klik baris / tombol "Lihat" → set filter detail (vendor+region, atau `unassigned` untuk grup tanpa vendor), reset
     halaman ke 1, bersihkan pilihan, scroll ke tabel detail. Baris aktif ditandai (`aria-current`).
4. **Tabel detail** (ForecastTable yang ada) — **selalu tampil**, default Vendor = Semua, Region = Semua.
   - Filter: ATM ID, FLM Vendor (Semua + daftar), Region (Semua + daftar), Brand; bila filter `unassigned` aktif tampil
     chip "Hanya ATM tanpa vendor aktif ✕".
   - Kolom baru **Status**: badge "Sudah di-request" (ikon + teks) untuk `is_requested = true`; baris itu checkbox-nya
     disabled. Baris tanpa vendor juga tidak bisa dicentang (tidak bisa dibuat Vendor Request).
5. **Bar aksi** (seperti sekarang): item terpilih, total, Pilih Semua Rekomendasi, Bersihkan, Buat Vendor Request.
   - **Buat Vendor Request** aktif bila pilihan ≥ 1 **dan** semua baris terpilih punya `flm_vendor` yang sama.
     Pilihan lintas vendor → tombol disabled + teks bantu "Satu Vendor Request hanya untuk satu vendor".
     `vendorId` diambil dari `vendorOptions` berdasarkan `flm_vendor` baris terpilih (bukan lagi dari filter).
   - **Pilih Semua Rekomendasi** aktif hanya bila filter FLM Vendor terisi; hanya memilih baris `is_requested = false`.
     Cap 1000 item tetap.
6. Setelah Vendor Request dibuat, invalidasi prefix `["vendor-requests"]` yang sudah ada ikut me-refetch ringkasan.

## Non-functional
- Ringkasan ≤ 3 s p95 untuk ± 5.000 rekomendasi/tanggal (Sec 3a). Index yang dipakai sudah ada:
  `vendor_request_items_terminal_idx (terminal_id, periode_pred)`, `dmaa_atm_forecast_terminal_id_idx`.
  **Tidak ada migrasi.**
- Primary pool (plan D1): ringkasan + `is_requested` langsung konsisten setelah create — tidak ada replica lag.
- RBAC di middleware (route) — tidak ada perubahan role.
- Aksesibilitas: status tidak hanya warna; baris ringkasan bisa difokus & diaktifkan dengan keyboard; target ≥ 44 px.
- Tidak ada state change → tidak ada audit_log / maker-checker baru (read-only).

## Data model
Tidak berubah. Query sqlc baru `SummarizeForecastForDate` (join sama dengan `ListForecastForDate` + `EXISTS` ke
`vendor_request_items`/`vendor_requests`, lalu GROUP BY vendor, region). `ListForecastForDate`/`CountForecastForDate`
mendapat parameter `unassigned` dan kolom `is_requested`. `sqlc generate` (v1.31.1) + fix `UserLeafe`.

## Sec 4 #7 flags
- Money: hanya agregasi baca (SUM bigint). Tidak ada perhitungan/penulisan uang baru.
- Auth/recon/migration/delete: tidak disentuh.
- Perubahan kontrak: `flm_vendor`/`flm_vendor_region` jadi opsional (melonggarkan validasi CIT-2 Req 1.4) — disetujui
  lewat intent OQ1. Validasi vendor per terminal saat create (`GetActiveVendorForTerminal`) tetap menjadi penjaga.

## Acceptance criteria
1. Buka halaman tanpa memilih apa pun → KPI, ringkasan, dan tabel detail (semua vendor) tampil.
2. Grup tanpa vendor muncul bila ada ATM tanpa paket aktif; klik → detail hanya ATM itu.
3. Klik grup (V,R) → detail terfilter V,R; jumlah ATM distinct = kolom ATM di ringkasan.
4. Buat Vendor Request untuk sebagian ATM grup → kembali → "Sudah" naik, "Belum" turun; baris ATM tsb berbadge
   "Sudah di-request" dan tidak bisa dicentang.
5. Vendor Request dibatalkan/ditolak → ATM-nya kembali "Belum".
6. Pilih ATM dari dua vendor → tombol Buat disabled dengan teks bantu.
7. `GET /forecast` tanpa vendor/region → 200; dengan `unassigned=true&flm_vendor=X` → 400.

## Out of scope
- Ekspor CSV ringkasan; pembatasan per cabang; Manual Request dari grup tanpa vendor.
- Filter status "Belum di-request saja" di tabel detail (bisa ditambah bila diminta).
- Mencegah request dobel di server (create saat ini tidak mengecek item yang sudah di-request) — hanya penanda UI.
  Catat sebagai risiko; bila perlu jadi intent terpisah.
