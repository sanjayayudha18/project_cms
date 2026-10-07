# Plan: Kuota kunjungan replenish per ATM + laporan selesai Vendor Request

Status: **accepted 2026-10-01** (user: "plan diterima, lanjut implementasi"). Stage: 3 Build — **selesai 2026-10-01** (M1, B1–B4, F1–F4, D; deviasi dicatat di "Catatan implementasi" di bawah). D1–D4 disetujui bersama plan.
Input: `spec.md` (accepted 2026-10-01, user: "spec diterima, lanjut plan.md"). Trigger ke stage berikut: plan diterima → implementasi → `tests.md`.

## Keputusan yang perlu disetujui (code ≠ spec)
**D1 — Pool baca kuota.** Spec NFR: `GET /atm-visit-quotas/atms/{id}` "boleh replica". Di kode, `VendorRequestService` hanya
punya pool primary (`cmd/api/main.go:433`); replica opsional dan hanya dipakai export master data.
**Usulan: primary**, sama dengan keputusan D1 `forecast-browser-summary` — angka sisa dibaca tepat setelah approve/reset/batal
(read-after-write), replica lag akan menampilkan angka lama. Beban kecil (1 baris kuota + ≤ puluhan kunjungan per ATM).

**D2 — Aksi laporan selesai tidak lewat `transition()`.** `checkActor` membandingkan dengan `req.CreatedBy`, sedangkan spec
FR1 mensyaratkan checker ≠ `completion_submitted_by`; approve juga harus menulis kunjungan + kuota. **Usulan:** tiga method
terpisah (pola sama dengan `Cancel`, `vendor_request_actions.go:805`): tx → `GetVendorRequestForUpdate` → `nextState` →
guard aktor sendiri → query khusus → audit → commit. `transitions` map tetap satu-satunya sumber transisi.

**D3 — Working tree belum bersih.** Ada perubahan belum di-commit di `ForecastBrowser.tsx`, `ForecastSummary.tsx`,
`ForecastTable.tsx`, `VendorRequestCreate.tsx` (+ tests) yang juga disentuh F4. **Usulan:** commit/stash perubahan itu
oleh user sebelum F4, supaya diff fitur ini terpisah.

**D4 — Nama tabel ke CLAUDE.md Sec 3.** `atm_visit_quotas`, `atm_visits`, `vendor_request_atm_results` ditambahkan ke grup
**ATM** di Sec 3 + `docs/data-map.md` + catatan deviasi Golden Rule #3 di Sec 12 / `docs/decisions.md` — dikerjakan di langkah D.

## Urutan kerja
Migrasi → SQL/sqlc → service (TDD: test dulu) → handler → frontend → docs. Setiap langkah diakhiri `go test ./...` / `pnpm test` hijau.
Migrasi diterapkan ke dev **oleh user** (atau setelah user OK), bukan otomatis.

### M1 — Migrasi `backend/migrations/021_atm_visit_quota.sql`
Persis DDL spec "Data model", dibungkus `BEGIN; … COMMIT;`, plus `COMMENT ON TABLE/COLUMN` (gaya migrasi 009/016),
`updated_at` trigger untuk `atm_visit_quotas` bila trigger function baseline tersedia (cek nama fungsi di `001_baseline_schema.sql`).
Additive kecuali penggantian `vendor_requests_status_chk` (drop+add dalam tx yang sama; nilai lama tetap valid).
Tambah ke `CMS_DB_dbdiagramio.dbml`.

### B1 — SQL
`backend/queries/vendor_request.sql`:
1. `UpdateVendorRequestCompletion :one` — set `status` + kolom `completion_*` sesuai target (`completion_pending` →
   submitted_by/at, reset rejected_*; `completed` → approved_by/at; `approved` (tolak) → rejected_by/at/reason).
2. `UpsertVendorRequestAtmResult :exec` — `INSERT … ON CONFLICT (vendor_request_id, terminal_id) DO UPDATE SET result, updated_at`.
   FR1.1 mewajibkan hasil untuk setiap terminal, jadi upsert selalu menimpa semua baris — tidak perlu query hapus.
3. `ListVendorRequestAtmResults :many` (untuk detail + approve).
4. `ListDistinctRequestTerminals :many` — `SELECT DISTINCT terminal_id FROM vendor_request_items WHERE vendor_request_id = $1`.
5. `ListForecastForDate`: `LEFT JOIN atm_visit_quotas q ON q.atm_id = a.id` → `q.remaining AS visit_remaining`,
   `q.quota_total AS visit_quota_total` (LEFT JOIN tabel nyata → nullable, alasan sqlc sama dengan kolom `paket`).

`backend/queries/atm_visit_quota.sql` (baru):
1. `ResolveAtmQuota :one` — per `terminal_id` + `as_of_date`: `atms.id`, paket aktif (LATERAL sama dengan
   `ListForecastForDate`, tie-break `avp.id DESC`), `pf.cr_frequency` via
   `LEFT JOIN package_frequencies pf ON pf.package_code = pkg.package_code AND pf.machine_group = a.price_machine_group`.
2. `GetAtmVisitQuotaForUpdate :one` — `SELECT … WHERE atm_id = $1 FOR UPDATE`.
3. `InsertAtmVisitQuota :one` (FR2.1) · `DecrementAtmVisitQuota :one` (`remaining = remaining - 1 … RETURNING remaining`) ·
   `IncrementAtmVisitQuota :one` (pembatalan) · `ResetAtmVisitQuota :one` (`INSERT … ON CONFLICT (atm_id) DO UPDATE` set
   `package_code, quota_total, remaining = quota_total, reset_at = now(), reset_by`).
4. `InsertAtmVisit :one` — `ON CONFLICT (vendor_request_id, atm_id) DO NOTHING RETURNING *` (idempotensi FR3.1).
5. `GetAtmVisitForUpdate :one` · `CancelAtmVisit :one`.
6. `GetAtmVisitQuotaByTerminal :one` + `ListAtmVisitsSince :many` (kunjungan `created_at >= reset_at`, join nama user).
7. `ListAtmsForVendorQuotaReset :many` — semua ATM dengan paket aktif milik `vendor_id`, beserta `cr_frequency` (nullable).
8. `ListVisitQuotaForTerminals :many` — `terminal_id = ANY(@terminals)` → remaining/quota_total (untuk detail request).
Lalu `sqlc generate` (v1.31.1) + hand-fix `UserLeafe` → `UserLeave`; cek `git diff --stat backend/internal/db/`.

### B2 — Service kuota `backend/internal/service/atm_visit_quota.go` (baru)
- `AtmVisitQuotaService{pool}`; tipe `AtmVisitQuota`, `AtmVisit`, `VendorQuotaResetResult`.
- Helper **dalam tx** (dipakai juga oleh B3): `recordVisit(ctx, q, actor, requestID, terminalID, today) (overQuota, created bool, err)`
  — resolve kuota → insert visit (conflict → `created=false`, tidak mengurangi) → lock kuota → insert/decrement → set `is_over_quota`.
  ATM tidak ada di `atms` → `created=false` + catatan audit.
- `Get(terminalID)`, `ResetATM(actor, terminalID)` (422 `ErrQuotaUnknown`), `ResetVendor(actor, vendorID)`,
  `CancelVisit(actor, visitID, reason)` (409 `ErrVisitNotCancelable`: sudah batal / sebelum `reset_at`).
- RBAC service: `isChecker(actor.Role)` untuk reset/batal (`ErrNotChecker`).
- Audit per aksi, entity `atm_visit_quota` / `atm_visit`, dalam tx yang sama.
- "Hari ini" = tanggal Asia/Jakarta (pakai helper yang sudah ada bila ada; kalau tidak, `time.LoadLocation("Asia/Jakarta")` sekali).

### B3 — Laporan selesai `backend/internal/service/vendor_request_completion.go` (baru)
- `vendor_request.go`: tambah action `submitCompletion/approveCompletion/rejectCompletion` ke `transitions`:
  `approved: {…, submit_completion: completion_pending}`, `completion_pending: {approve_completion: completed, reject_completion: approved}`.
  `cancel` tidak ditambahkan untuk `completion_pending` (FR1.4).
- `SubmitCompletion(actor, id, results)` — maker role; validasi set terminal = distinct items (FR1.1, `ValidationError`);
  upsert hasil; update status; audit (before/after + hasil).
- `ApproveCompletion(actor, id) (detail, overQuotaTerminals)` — checker, ≠ `completion_submitted_by` (`ErrSelfApproval`);
  untuk setiap `success` → `recordVisit`; update status `completed`; audit memuat daftar kunjungan + over quota.
- `RejectCompletion(actor, id, reason)` — checker ≠ pelapor; alasan 1–500 (pola `Reject`); status → `approved`.
- `mapDetail`/`Get`: tambah per item `completion_result`, `visit_remaining`, `visit_quota_total`, `is_over_quota`;
  header `completion_*` + nama user.
- `BrowseForecast`: map dua kolom baru.

### B4 — Handler
- `vendor_request_handler.go`: routes `POST /{id}/complete` (maker), `/{id}/complete/approve`, `/{id}/complete/reject` (checker);
  `vendor_request_response.go`: field baru (flat JSON, snake_case); `handleError` peta `ErrQuotaUnknown`→422, `ErrVisitNotCancelable`→409.
- `atm_visit_quota_handler.go` (baru) + mount di `cmd/api/main.go`: `r.With(RequireAuth).Mount("/api/v1/atm-visit-quotas", …)`,
  GET dengan `vendorRequestViewerRoles`, POST dengan `vendorRequestCheckerRoles`.

### F1 — Tipe/API/hooks
`features/vendor-request`: `types.ts` (status `completion_pending`, field baru), `api.ts` + `hooks.ts` (3 mutation baru,
invalidate detail/list/forecast), `StatusBadge.tsx` (label "Menunggu persetujuan laporan", "Selesai"), filter status di list.
`features/atm-portal`: api/hooks `useAtmVisitQuota`, `useResetAtmQuota`, `useCancelVisit`, `useResetVendorQuota`.

### F2 — Detail Vendor Request (`VendorRequestDetail.tsx`)
Tombol + dialog "Laporkan selesai" (FR6.1), "Setujui laporan"/"Tolak laporan" + peringatan kelebihan kuota (FR6.2),
badge per ATM (FR6.3, teks + ikon). Tombol tampil sesuai role + pelapor (pola tombol approve yang ada).

### F3 — Profil ATM (`AtmProfileScreen.tsx` + komponen baru `components/VisitQuotaCard.tsx`)
Kartu kuota, daftar kunjungan, reset + batalkan (dialog alasan) untuk checker; "Paket tidak dikenali" bila kuota null (FR6.5).

### F4 — Forecast Browser (setelah D3)
`ForecastTable.tsx`: kolom "Sisa kunjungan" (`tabular-nums`, kanan, "—", penanda ≤ 0 teks+ikon).
`ForecastSummary.tsx`: tombol "Reset kuota vendor" per baris vendor (checker) + dialog konfirmasi + hasil (FR6.6).
Baris ringkasan perlu `vendor_id` — cek apakah `SummarizeForecastForDate` sudah mengirimnya; bila belum, tambah kolom `v.id` (additive).

### D — Docs
CLAUDE.md Sec 3 (grup ATM: tiga tabel) + Sec 12 (deviasi GR#3 untuk reset/batal kunjungan, migrasi 021),
`docs/data-map.md`, `docs/decisions.md`, `sdlc/README.md`, `development-progress.md`, `graphify update .`.

## Tests (TDD — ditulis sebelum implementasi tiap langkah)
- Unit (`go test ./...`, table-driven): transisi baru di `transitions`; guard aktor (maker role, checker ≠ pelapor,
  ATM-USER ditolak); validasi set terminal; alasan tolak/batal; perhitungan sisa/kelebihan untuk tampilan.
- Integration (`//go:build integration`, real Postgres, pola `vendor_request_summary_integration_test.go`; jalankan migrasi 021):
  AC1 (PAKET 5 → 4, gagal tidak berubah), AC2 (ke-6 → -1, over quota), AC3 (approve ganda + 2 goroutine paralel → turun sekali),
  AC5 (tolak → approved, ajukan ulang menimpa), AC6/7 (reset ATM/vendor, skipped, `cr_frequency` tetap), AC8 (batal +1 / 409),
  AC9 (audit gagal → rollback), kuota tidak diketahui (kunjungan tercatat, tanpa baris kuota).
- Handler: route role gate 403, mapping 400/404/409/422.
- Frontend (`pnpm test`): dialog laporan (default berhasil, toggle gagal, payload), peringatan approve, badge, kolom sisa,
  kartu kuota (null → "Paket tidak dikenali"), tombol tersembunyi untuk non-checker.
- `pnpm build` + `lint`, `go vet`.
- **Manual browser check: outstanding (user)** — butuh seed ulang `vendor_packages_branch`/`atm_vendor_packages` di dev.

## Risiko
- Data dev kosong (migrasi 011) → integration test men-seed sendiri; uji browser tergantung seed user.
- `package_code` teks bebas → banyak ATM bisa "Paket tidak dikenali"; perilaku sesuai spec (tidak ditebak), tapi perlu dikomunikasikan.
- Approve request besar (ratusan ATM) = ratusan statement dalam satu tx; masih wajar, dicek di integration test (≤ 3 s).
- Status baru `completion_pending` harus dikenali semua tempat yang memfilter status (list; `is_requested` memakai
  `NOT IN ('cancelled','rejected')` → tetap benar).

## Out of scope
Sesuai spec. Tidak menyentuh `package_frequencies`, VendorPortal, notifikasi, status `processing`/`failed`.

## Catatan implementasi (2026-10-01)
Deviasi dari plan/spec — semuanya kecil, tidak mengubah perilaku bisnis yang disetujui:
1. **Pembuatan baris kuota idempoten.** `InsertAtmVisitQuota` diganti `EnsureAtmVisitQuota` (`INSERT … ON CONFLICT (atm_id) DO NOTHING`, `remaining = quota_total`), lalu baris dikunci dan dikurangi lewat jalur yang sama. Alasan: dua approve paralel pada ATM tanpa baris kuota tidak boleh gagal di PK. Terbukti di `TestIntegration_VisitQuota_ParallelApprovals` (5 → 3).
2. **`clock_timestamp()` untuk batas periode** (`atm_visit_quotas.reset_at`, `atm_visits.created_at`), bukan `now()`. `now()` konstan dalam satu tx sehingga `created_at >= reset_at` salah urut; ditemukan oleh integration test (harness satu tx).
3. **Info kuota di detail request = array `atms[]`** (satu baris per ATM distinct: `completion_result`, `visit_remaining`, `visit_quota_total`, `is_over_quota`), bukan field per item seperti tertulis di spec FR5 — item adalah per (terminal, periode, denom), sedangkan hasil laporan & kuota per ATM.
4. **Tombol "Reset Kuota Vendor"** muncul di samping filter Vendor ringkasan setelah satu vendor dipilih (bukan per baris vendor × region), supaya satu vendor tidak punya beberapa tombol reset. Konfirmasi dua langkah, hasil `reset_count`/`skipped_count` di toast.
5. **Placeholder "-"** (bukan "—") untuk sisa kosong — konvensi tabel yang sudah diuji (`ForecastTable.test.tsx`).
6. **`ReasonModal` di-export** dari `VendorRequestDetail.tsx` dan dipakai ulang untuk tolak laporan & batal kunjungan (Profil ATM); dialog laporan memakai elemen `<dialog>` (biome a11y).
7. **Alasan batal kunjungan kosong → 422** (`writeValidationError`, konvensi handler yang ada), bukan 400 seperti tabel error spec.
8. `CMS_DB_dbdiagramio.dbml` hanya ditambah bagian 021 (file itu sudah tertinggal dari migrasi 009–020; tidak dirapikan di sini).
9. **D3 belum terselesaikan**: perubahan Forecast Browser lama (belum di-commit) masih ada di working tree, sehingga diff `ForecastTable.tsx`/`ForecastSummary.tsx`/`VendorRequestCreate*` bercampur dengan fitur ini — perlu dipisah saat commit.
