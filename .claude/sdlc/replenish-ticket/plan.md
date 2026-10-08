# Plan: Nomor tiket replenish per ATM + nomor request per region

Status: **accepted 2026-10-08 (engineer, via chat: "plan diterima")**. Stage: 3 Build.
Input: `spec.md` (accepted 2026-10-08: FR1–FR11). Menggantikan dua versi plan sebelumnya.

## Keputusan yang perlu disetujui (detail yang spec tidak atur)
**D1 — Logika tiket murni di Go, nomor dirakit di SQL.** `denomCode(denoms) string` (`50K`/`100K`/`MIX`) dan
`planTickets(existing []db.VendorRequestTicket, want map[terminal]code) ticketPlan{Deactivate, Reactivate []int64,
Issue []issue}` (Issue urut `terminal_id`) — diuji unit tanpa DB. Format nomor tiket hanya ada di query insert
(`terminal || '_' || code || '_' || to_char(date,'YYYYMMDD') || '_' || lpad(seq,3,'0')`) dan backfill migrasi.

**D2 — NNN tiket: advisory lock per (ATM, tanggal) + `max(seq)+1`.** `pg_advisory_xact_lock(hashtextextended(
terminal_id || '|' || date, 0))` lalu `INSERT … SELECT COALESCE(max(seq),0)+1 … RETURNING ticket_number`. Urutan
`terminal_id` ascending di semua tx → tanpa deadlock; tanpa retry, tanpa tabel counter. `seq > 999` → CHECK gagal →
`ValidationError` 422. (Counter nomor request tetap memakai pola `vendor_request_number_seq` yang ada — FR10.5.)

**D3 — Resolusi region tanpa query tambahan.** `GetActiveVendorForTerminal` (dipanggil per item oleh
`validateItemsSingleVendor`) diperluas mengembalikan `vendor_id, branch_code, region_code`. Fungsi itu diubah
mengembalikan `map[terminal]branchRegion` sekaligus memvalidasi vendor; fungsi murni `singleRegion(map)` memutuskan
FR10.1/10.2 (pesan 422 dikelompokkan per kode, termasuk cabang tanpa kode).

**D4 — Tiket dikunci `request_number`** (FR4.1). Service sudah memegang nomor: `Create` (dikembalikan oleh
`createWithRetryingNumber`, sekarang hanya id), `UpdateItems`/`Get`/completion (dari header).

**D5 — Interface repo: `ListVendorRequestAtmResults` diganti `ListActiveVendorRequestTickets(ctx, requestNumber)`**
(dipakai `Get` + `requestAtmStatuses`) — jumlah method tetap; stub `fakeForecastRepo` ikut diganti.

**D6 — `region_code` vendor cabang dinormalisasi server**: trim + huruf besar, lalu cek `^[A-Z0-9]{2,10}$` (422
bila gagal); string kosong = NULL. CHECK di DB sebagai jaring terakhir.

**D7 — Satu migrasi `026_replenish_ticket_region.sql`, satu tx**, guard lewat CHECK + `RAISE EXCEPTION` (FR7).
Sebelum apply ke dev: `pg_dump --data-only -t vendor_request_atm_results -t vendor_request_number_seq` ke scratchpad.

## Urutan kerja

### M1 — Migrasi `backend/migrations/026_replenish_ticket_region.sql`
1. `vendor_branches.region_code` + CHECK + `COMMENT`; seed: `UPDATE vendor_branches b SET region_code = m.code
   FROM (VALUES ('Jakarta','JKT'), …) m(region, code) WHERE btrim(b.region, ' Â') = m.region AND b.region_code IS
   NULL` — tabel FR11.3 lengkap (`Jawa barat` & `Jawa Barat` → JABAR; `Jakarta UtaraÂ` → JKTUTR).
2. `vendor_requests.region_code text` + `COMMENT` (snapshot, NULL = format lama).
3. `vendor_request_number_seq`: tambah `region_code`, drop PK, `UNIQUE NULLS NOT DISTINCT (vendor_id, region_code,
   seq_date)`.
4. `vendor_request_tickets` (DDL spec, FK `request_number`) + partial unique index + `COMMENT`; backfill FR7.2
   (`GROUP BY request_number, terminal_id`; kode denom `MIX` bila > 1 denom; tanggal `COALESCE(replenish_date,
   (created_at AT TIME ZONE 'Asia/Jakarta')::date)`; `row_number() OVER (PARTITION BY terminal_id, tanggal ORDER BY
   created_at, id)`).
5. FR7.4: `UPDATE tickets SET result … FROM vendor_request_atm_results r JOIN vendor_requests vr ON vr.id =
   r.vendor_request_id WHERE …is_active`; `DO $$` cek jumlah → `RAISE EXCEPTION`; `DROP TABLE
   vendor_request_atm_results`.
Apply ke dev (`psql … localhost:5432`); bukti: jumlah tiket = jumlah (request, ATM) unik, jumlah `result` = baris
lama, jumlah cabang ber-kode. Update `CMS_DB_dbdiagramio.dbml`.

### B1 — SQL → `sqlc generate` (v1.31.1, fix `UserLeafe` → `UserLeave`, cek `git diff --stat internal/db/`)
- `vendor_branches_admin.sql`: `region_code` di SELECT/INSERT/UPDATE/RETURNING.
- `master_data_export.sql`: kolom `region_code`.
- `vendor_request.sql`:
  - `GetActiveVendorForTerminal` → + `vb.branch_code`, `vb.region_code` (D3).
  - `NextRequestNumberSeq` → + `region_code`, `ON CONFLICT (vendor_id, region_code, seq_date)`.
  - `CreateVendorRequest` → + `region_code`; header SELECT (`GetVendorRequest*`) → + `region_code`.
  - Tiket: `ListVendorRequestTickets(request_number)`, `LockTicketSeq(terminal_id, date)`,
    `IssueVendorRequestTicket(request_number, terminal_id, denom_code, date) RETURNING ticket_number`,
    `SetVendorRequestTicketActive(id, is_active) RETURNING ticket_number`,
    `ListActiveVendorRequestTickets(request_number)` (`terminal_id, ticket_number, result`),
    `SetVendorRequestTicketResult(request_number, terminal_id, result) RETURNING id` (0 baris → error).
  - Hapus `UpsertVendorRequestAtmResult`, `ListVendorRequestAtmResults`.

### B2 — Master data vendor cabang (FR11)
- `service/vendor_branch_admin.go`: `RegionCode *string` di payload create/update + normalisasi D6.
- `service/masterdata_applier_vendor_branch.go`: teruskan `region_code`.
- `service/masterdata_import.go` + `masterdata_import_confirm.go`: kolom opsional `region_code` (file lama tanpa
  kolom tetap valid). `service/masterdata_export.go`: kolom ikut.
- `handler/admin_vendor_branch_handler.go`: `region_code` di respons.

### B3 — Nomor request per region (FR10)
- `service/vendor_request_actions.go`:
  - `validateItemsSingleVendor` → kembalikan `map[terminal]branchRegion` (D3); `singleRegion` → 422 FR10.2.
  - `Create`: region tunggal → `createWithRetryingNumber(…, regionCode)` → nomor
    `REP-<prefix>-<region>-<YYYYMMDD>-<NNN>`, simpan `region_code`, kembalikan `(id, requestNumber)`.
  - `UpdateItems`: `req.RegionCode != nil` → ATM harus kode sama (FR10.4); NULL → tanpa cek.
  - `Get`/`mapDetail`: `RegionCode` di header.

### B4 — Tiket (FR1, FR2, FR6)
- `service/vendor_request_ticket.go` (baru): `denomCode`, `wantedCodes(items)`, `planTickets` (D1),
  `applyTicketPlan(ctx, q, requestNumber, replenishDate, plan) (ticketAudit, error)` — Deactivate → Reactivate →
  Issue (lock D2 per Issue); CHECK seq → `ValidationError`.
- `Create`: setelah item → plan + apply → audit `after.region_code`, `after.tickets`.
- `UpdateItems`: setelah `resolveItems` → `ListVendorRequestTickets` → plan + apply → audit
  `tickets_added/deactivated/reactivated`.
- `Get`: `ListActiveVendorRequestTickets` → `TicketNumber` per item.

### B5 — Laporan selesai di tiket (FR9)
- `service/vendor_request_completion.go`: submit (~146) → `SetVendorRequestTicketResult`; approve (~175) dan
  `requestAtmStatuses` (~244) → `ListActiveVendorRequestTickets` (`result` + `TicketNumber`).
- `service/vendor_request.go`: field `TicketNumber` (item, ATM), `RegionCode` (detail); interface D5.

### B6 — Handler `handler/vendor_request_response.go`
`ticket_number` di `vendorRequestItemResponse` + `requestAtmStatusResp`; `region_code` di detail.

### F1 — CompanyPortal vendor request (`src/features/vendor-request/`)
`types.ts` (`ticket_number`, `region_code`); `VendorRequestDetail.tsx` kolom "No. Tiket" setelah "ATM ID";
`CompletionPanels.tsx` kolom "No. Tiket"; fixture tes. Pesan 422 FR10.2 sudah tampil lewat penanganan error yang ada
(dicek, bukan diubah).

### F2 — CompanyPortal vendor cabang (`src/features/admin-vendors/`)
`types.ts`, `lib/vendorBranchFormSchema.ts` (opsional, 2–10 huruf/angka, auto huruf besar),
`VendorBranchFormDialog.tsx` + `VendorBranchEditPage.tsx` field "Kode Region", `BranchesPanel.tsx` kolom.

### T1 — Tes (detail di `tests.md`)
- Unit: `denomCode`, `planTickets` (create/tetap/hapus/tambah/kode berubah/aktif kembali), `singleRegion`
  (satu/campur/tanpa kode), normalisasi `region_code`, format nomor request.
- Integration (skip bila `DATABASE_URL` kosong): `REP-BJK-JKT-20261008-001`; counter terpisah per region; lintas
  region → 422; cabang tanpa kode → 422; edit request baru tambah ATM region lain → 422; edit request lama → lolos;
  tiket `MIX`, `_002`, `_003`, aktif kembali, dua create paralel, 999 → 422; FK tiket → request; laporan selesai di
  tiket + regresi `atm_visit_quota_integration_test.go`; import CSV vendor branch dengan/tanpa `region_code`.
- Handler: `ticket_number`, `region_code`. Frontend: kolom "No. Tiket", field "Kode Region".
- `go test ./...`, `pnpm --dir frontend/CompanyPortal-Vite run test|lint|build`.

### D — Docs
CLAUDE.md Sec 3 (`vendor_request_tickets` baru, `vendor_request_atm_results` dihapus, `vendor_branches.region_code`)
+ Sec 12 (migrasi sampai `026`, format nomor request & tiket); `docs/data-map.md`; README sdlc +
`development-progress.md`; `tests.md` + `review.md`; `graphify update .`.

## Risiko
- **Format nomor request berubah** untuk request baru — tidak ada parsing nomor di backend/frontend (dicek grep), tapi
  pihak luar yang membaca nomor (laporan manual, vendor) perlu tahu.
- **51 vendor cabang dev tanpa kode** → request untuk ATM-nya ditolak sampai diisi (diterima PO).
- **DROP TABLE + seed master data lewat migrasi** (Sec 4 #7, disetujui PO): satu tx, guard jumlah, `pg_dump` cadangan.
  Seed master data tidak lewat maker-checker (data awal migrasi, sama seperti seed lain); perubahan selanjutnya wajib
  lewat maker-checker.
- Fitur kuota (selesai) berganti sumber data hasil laporan → regresi wajib hijau.
- Advisory lock hash bisa bertabrakan → hanya saling menunggu, tidak salah.
- Manual browser check: **outstanding** (user, Golden Rule #10).

## Catatan implementasi (2026-10-08, sinkron dengan diff)
- M1–T1 selesai; bukti di `tests.md`. Cadangan `pg_dump` sebelum migrasi: scratchpad sesi (`pre026_backup.sql`).
- `singleRegion`/`branchRegion` tinggal di `service/vendor_request_ticket.go` bersama logika tiket (bukan di
  `vendor_request_actions.go`, yang sudah 1100+ baris).
- `UpdateItems` sebelumnya **tidak** memanggil `validateItemsSingleVendor`; sekarang dipanggil hanya untuk request
  ber-`region_code` (sekalian cek vendor). Request lama tetap tanpa cek (FR10.4 "seperti sekarang").
- `UpdateItems` request `VR-` tanpa `replenish_date`: tanggal tiket = `created_at` Asia/Jakarta (`ticketDate`, sama
  dengan backfill FR7.3).
- Nama `normalizeBranchRegionCode` (bukan `normalizeRegionCode`: sudah dipakai `region_admin.go`).
- CSV vendor-branches: header lama (tanpa `region_code`) diterima lewat `legacyHeader`; sel diisi `keepRegionCode`
  lalu diganti kode saat ini → baris tak berubah = "unchanged", tidak menghapus kode.
- Tambahan CHECK `vendor_request_tickets_inactive_chk` (non-aktif ⇒ `deactivated_at` terisi, `result` NULL) — FR9.4
  dijaga DB.
- Ditemukan, **tidak** diperbaiki (di luar lingkup): impor CSV vendor-branches tidak mengirim `category`, sehingga
  create/update lewat CSV selalu gagal validasi "harus ATM, CASH, atau ATM_CASH" (sudah ada sebelum fitur ini).

## Proof (Stage 4)
`sqlc generate` bersih; output `psql` migrasi 026 + angka verifikasi; `go test ./...`; frontend test/lint/build.
