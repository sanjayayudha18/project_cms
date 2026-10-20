# Plan: Pemenuhan vault ACM — bagi nominal request ke vault + tiket pemenuhan vault

Status: **v3 accepted — 2026-10-13** ("accept plan, mulai S0"). Reads: `spec.md` v3 (accepted 2026-10-13).
Stage: 3 Build. Kerja dipecah menjadi **9 slice** (S0–S8); tiap slice = satu diff kecil, build + test hijau, berhenti & minta OK user.
(Plan v1/v2 per-ATM: superseded. S1 v1 = migrasi 030 kolom `vendor_vault_id` per ATM, uncommitted, applied to dev, 0 baris → di-revert di S0.)

## Titik sentuh yang sudah dicek di kode
- Pembuatan plan: `service/vault_plan_create.go` `createVaultPlans` :18 (dipanggil ATM-SPV approve + ADMIN assign branch),
  `resetVaultPlansForReapproval` :65; query `CreateMissingVaultPlans` (`queries/vault_plans.sql`, join `acm_area_branches`).
- Detail/kandidat/kapasitas: `service/vault_plan.go` (`detail`, `loadMetrics`, `Candidates`, `recomputeSnapshots`, `ForRequest` :368),
  `service/vault_capacity.go`.
- Aksi: `service/vault_plan_actions.go` (`SaveAssignments`, `validateAssignments`, `Submit`, `Approve` + gate
  `CountRequestAtmsWithoutApprovedVault` :224, `ReviewRequest`, `ResetVaultPlansToDraft` :323).
- Edit request 2.2b: `vendor_request_actions.go:811` `DeleteVaultAssignmentsOfRemovedAtms`.
- Area admin: `service/acm_area_admin.go` (`SetBranches` :225, `BranchOptions`, `Warnings` :96), `handler/admin_acm_area_handler.go`.
- Pihak vendor: `queries/vendor_parties.sql` `ListRequestPartyRows`, `service/vendor_party.go` (`buildPartyContents`, `sameContent`).
- Tiket replenish (pola): `service/vendor_request_ticket.go` (`planTickets`, `applyTicketPlan`, advisory lock).
- Handler: `handler/vault_plan_handler.go`, `handler/vendor_request_vault_handler.go`.
- Frontend: CompanyPortal `features/vault-plan/` (`VaultPlanDetail.tsx`, `RequestVaultPanel.tsx`, `api.ts`, `VaultPlan.test.tsx`),
  `features/admin-acm-areas/` (`AcmAreaDetailPanel.tsx`, `AdminAcmAreasPage.tsx`, `api.ts`, test), `features/vendor-request/
  VendorPartiesPanel.tsx`; VendorPortal `features/orders/` (`api.ts`, `OrderDetailPage.tsx`, `__tests__/OrdersPages.test.tsx`).
- Dev data: 1 plan `draft`, 0 penetapan per ATM, 0 pihak vendor, 1 request `vault_flow` di `vault_assignment` dengan
  `region_code` NULL.

## Slices

### S0 — Revert S1 v1 (perlu izin user: ubah dev DB) — ✅ DONE 2026-10-13
> Izin user: "accept plan, mulai S0". Dev DB: satu tx dengan guard (abort bila `vendor_vault_id` berisi) → FK, kolom, UNIQUE
> dilepas, COMMENT `saldo_snapshot` dikembalikan ke teks 027; dicek: kolom/constraint/index = 0. Repo: 6 file `git checkout`,
> `030_vault_assignment_vault.sql` dihapus → backend bersih (hanya `.claude/` berubah). `go test ./...` +
> `go test -tags integration ./...` hijau.
- Dev DB (localhost), satu tx: `ALTER TABLE vendor_request_vault_assignments DROP CONSTRAINT vendor_request_vault_assignments_vault_fk,
  DROP COLUMN vendor_vault_id` (index ikut terhapus); `ALTER TABLE vendor_vaults DROP CONSTRAINT vendor_vaults_id_branch_uq`;
  kembalikan COMMENT `saldo_snapshot`. Kolom kosong (0 baris).
- Repo: hapus `migrations/030_vault_assignment_vault.sql`; `git checkout` `queries/vault_plans.sql`, `internal/db/{models,vault_plans.sql}.go`,
  `service/vault_capacity.go`, `service/vault_plan.go`, `service/vault_plan_integration_test.go` (semua hanya berisi diff S1 v1).
- **Proof:** `\d vendor_request_vault_assignments` tanpa `vendor_vault_id`; `git status` bersih kecuali `.claude/`; `go test ./...` hijau.

### S1 — Migrasi 030 v3 + query + sqlc — ✅ DONE 2026-10-13
> `030_vault_allocation.sql` applied to dev. **Penyimpangan dari rencana di bawah** (alasan: slice tetap hijau tanpa
> menyentuh service 2.2a): (1) unik parsial "satu plan per request" **ditunda ke S3** (TODO di 030; index sempat dibuat lalu
> di-drop di dev) — service 2.2a + fixture integration masih membuat plan per area; (2) query baru di file baru
> `queries/vault_allocations.sql` + `queries/vault_tickets.sql` (+ region di `acm_areas_admin.sql`), query lama per-ATM di
> `vault_plans.sql` **dibiarkan** sampai S3 mengganti service; `CreateMissingVaultPlansByRegion` = query baru (yang lama tetap).
> Bukti psql (tx rollback, sisa 0 baris): kelipatan, denom, FK komposit, tier3-urgent, satu tiket per (request, vault), inactive
> butuh `deactivated_at`, format + unik region, cascade amounts. Query baca dijalankan di dev (39 vault eligible, 36 region).
> sqlc v1.31.1 (diff: `models.go`, `acm_areas_admin.sql.go`, baru `vault_allocations.sql.go`, `vault_tickets.sql.go`; tanpa
> `UserLeafe`); `no_hard_delete_test` + `vendor_request_vault_tickets`. `go test ./...` + `-tags integration` hijau; lint 0.
- `030_vault_allocation.sql` (satu tx, additive): `acm_area_regions`; `vendor_request_vault_plans.buffer_reason` + unik parsial
  `(vendor_request_id) WHERE status <> 'cancelled'`; `vendor_vaults UNIQUE (id, vendor_branch_id)`; `vendor_request_vault_allocations`
  + `_amounts` (CHECK denom + kelipatan); `vendor_request_vault_tickets` (spec Data model).
- `queries/vault_plans.sql`: `CreateMissingVaultPlans` → join `acm_area_regions` pada `vendor_requests.region_code`;
  `ListVaultCandidateVaults` (vault eligible + branch + vendor + region); `ListVaultSaldo` (per vault); `ListVaultAllocatedTotals`
  (Σ alokasi per vault per denom tanggal X, kecuali plan tertentu); `ClearVaultPlanAllocations`, `InsertVaultAllocation`,
  `InsertVaultAllocationAmount`, `ListVaultPlanAllocations` (+ amounts, vault, branch, tiket aktif), `UpdateVaultAllocationSnapshot`,
  `SetVaultPlanBufferReason`, `ListRequestOrderTotals`; gate approve = plan `acm_approved` (ganti `CountRequestAtmsWithoutApprovedVault`).
- `queries/vault_tickets.sql` (baru): list per request, `NextVaultTicketSeq` (vault, tanggal), insert / reactivate / deactivate.
- `queries/acm_areas.sql`: list/set region per area, region options (kode dari `vendor_branches` aktif), warnings FR1.4.
- Dev: terapkan 030; `sqlc generate` v1.31.1 + fix `UserLeafe`; `no_hard_delete_test` + `vendor_request_vault_tickets`.
- **Proof:** migrasi bersih; CHECK kelipatan + FK komposit + unik parsial diuji di psql (tx rollback); `go build ./...`.

### S2 — Logika murni: total, validasi, kapasitas, tiket (unit test, tanpa DB) — ✅ DONE 2026-10-13
> `service/vault_allocation.go`: `validateAllocationLines` (eligible, dobel, nominal bulat > 0, denom, kelipatan, urgent
> 10–500 / hanya bila urgent, tier 3 wajib urgent), `checkCoverage` (kurang → 422 dengan selisih; lebih termasuk denom tak
> dipesan → alasan buffer 10–500, di-trim; tanpa lebih → NULL), `allocationTotals`, `vaultSaldoFromRow`, `allocatedByVault`,
> `vaultCapacity`, `VaultOption`/`VaultBranchOption` + `groupVaultOptions`. Memakai ulang `denomAmounts`, `vaultTier`,
> `capacityWarning`, `amountsJSON` dari `vault_capacity.go` (sisa per-ATM di file itu dihapus di S3). Uang `strconv.ParseInt`
> ketat (bukan `big.Rat`): alokasi = rupiah bulat.
> `service/vault_ticket.go`: `planVaultTickets` (keep / deactivate / reactivate / issue urut id = urutan lock) +
> `applyVaultTickets` (advisory lock per (vault, tanggal), seq > 999 → 422) + `vaultTicketAudit` (FR6.4).
> Tests: `vault_allocation_test.go` (7 test, table-driven) — coverage fungsi baru 80–100 %; **tambahan**
> `vault_ticket_integration_test.go` (Postgres nyata, tx rollback, tanggal 2099-01-01): `_001` per vault, kirim ulang = tetap,
> vault dilepas = nonaktif, request lain di vault+tanggal sama = `_002`, vault kembali = nomor lama aktif lagi. Sisa di DB 0.
> `go test ./...` + `-tags integration` hijau; lint 0.
- `service/vault_allocation.go` (baru): `validateAllocationLines` (FR4.2: dobel, kosong, kelipatan, denom, urgent/tier),
  `allocationShortfall` / `allocationExcess` per denom vs order (FR4.3), `vaultCapacity` (FR5.3), `capacityWarning` per baris,
  sort kandidat (FR3.4). `vault_capacity.go` lama diganti/diringkas.
- `service/vault_ticket.go` (baru): `planVaultTickets(existing, wantedVaultIDs)` → issue/keep/reactivate/deactivate (FR6.2).
- **Proof:** table-driven: kurang 1 rupiah → error; lebih tanpa alasan → error; lebih + alasan OK; denom tak dipesan = lebih;
  bukan kelipatan; tier 3 tanpa urgent; kapasitas unknown; tiket: tetap/reaktivasi/nonaktif/baru; tidak ada float.

### S3 — Service + handler: area per region, plan per request, kandidat, simpan, submit + tiket — ✅ DONE 2026-10-13
> Migrasi 030: TODO diganti index `vendor_request_vault_plans_live_uq` (unik parsial), diterapkan di dev (cek dulu: 0 duplikat).
> Query: `vault_plans.sql` dibuang semua query per-ATM (`CreateMissingVaultPlans`, `ListVaultCandidateBranches`, `ListVaultBranchSaldo`,
> `ListVaultDemand`, `…Assignment…`, `CountRequestAtmsWithoutApprovedVault`, `DeleteVaultAssignmentsOfRemovedAtms`,
> `ListAllVaultPlansForRequest`); `GetVaultPlan` + `buffer_reason`/`region_code`, `LockVaultPlan` + `buffer_reason`, `ListVaultPlans`
> hitung alokasi, `ListVaultPlanAtms` = ATM request (tanpa penetapan); `acm_areas_admin.sql`: `region_count`, warning lama dibuang.
> Service: `vault_plan.go` (detail baru: ATM + order, `OrderTotals`, `Allocations` + tiket, `Tickets`; `Candidates` per branch → vault;
> `refreshAllocationSnapshots`; `ForRequest` = detail plan hidup / nil), `vault_plan_actions.go` (`SaveAllocations` replace-all + alasan
> buffer opsional 10–500; `Submit` = validasi ulang + `checkCoverage` + snapshot + `syncVaultTickets` + audit `vault_tickets`; `Approve`
> langsung `vault_review`), `vault_plan_create.go` (plan per region; re-approve = `ResetVaultPlansToDraft` plan hidup),
> `acm_area_admin.go` (`SetRegions`/`RegionOptions`/`Warnings` FR1.4, disable melepas region, `SetBranches` tidak membuat plan lagi),
> `vendor_request_actions.go` (edit tidak menghapus penetapan per ATM), `vault_capacity.go` diringkas ke helper bersama.
> Handler: `PUT /vault-plans/{id}/allocations` (`…/assignments` dihapus → 404), kandidat `{branches}` tanpa `terminal_id`, detail +
> `region_code`/`buffer_reason`/`order_totals`/`allocations`/`vault_tickets`; `GET /vendor-requests/{id}/vault-assignments` → `{plan}`;
> admin `GET /region-options`, `GET|PUT /{id}/regions`, warnings `{requests_without_area}`, 409 konflik region.
> Tests: integration ditulis ulang — `vault_plan_integration_test.go` (alur penuh: RBAC, kandidat, kurang/lebih/buffer, kelipatan, tier 3,
> tiket _001, reject tetap, vault dilepas = nonaktif, vault kembali = nomor lama, ATM-SPV reject; kapasitas lintas request, _002, cancel
> tidak ubah tiket + kapasitas kembali), `vault_plan_guards_…`, `vault_flow_…` (region tanpa area → warning → map → plan; branch tidak
> membuat plan; pindah region tidak memindah plan), `acm_area_admin_…` (+ `Regions`); handler test 3 file.
> **Penyimpangan (disengaja):** 8 test 2.2b (`vendor_order_…` 7, `vendor_party_…` 1) **di-skip** lewat `sentToVendor` — isi pihak vendor
> masih membaca penetapan per ATM (S4). Sampai S4, ATM-SPV vault-approve gagal "no vault assignments to send" (belum rilis, S3–S7
> bersama). Coverage `resetVaultPlansForReapproval` 25 % sampai test 2.2b aktif lagi di S4.
> `go test ./...` + `-tags integration ./...` hijau (8 skip di atas), lint 0, sisa data test di DB 0.
- `createVaultPlans`: area dari region request (S1 query); notifikasi tetap.
- `AcmAreaAdminService.SetRegions` (+ `RegionOptions`; FR1.3 membuat plan untuk request menunggu), `Warnings` FR1.4; handler
  `GET/PUT /admin/acm-areas/{id}/regions`.
- `VaultPlanService`: `Candidates` (per vault, tanpa terminal), `SaveAllocations` (replace-all, snapshot), `Submit` (FR4.3 + tiket +
  audit), `Approve` (gate satu plan), `detail`/`ForRequest` bentuk baru; hapus `SaveAssignments`/per-ATM. Handler `PUT …/allocations`
  (hapus `…/assignments`).
- `vendor_request_actions.go:811`: edit request tidak lagi menghapus penetapan per ATM (alokasi tidak per ATM; total dicek ulang
  saat submit) — panggilan dihapus.
- **Proof:** integration (Postgres nyata): region → area → plan dibuat saat approve; region tanpa area → warning, lalu map region →
  plan dibuat; simpan + submit; kurang → 422; buffer tanpa alasan → 422; tiket `<vault>_<YYYYMMDD>_001`; reject + submit ulang ganti
  vault → lama nonaktif, nomor tidak dipakai ulang; vault sama nominal berubah → nomor tetap; cancel tidak ubah tiket; kapasitas
  berkurang oleh alokasi request lain; ACM-SPV approve → `vault_review`; RBAC non-anggota 404; handler test bentuk JSON.

### S4 — Pihak vendor (FR7) — ✅ DONE 2026-10-13
> `queries/vendor_parties.sql`: `ListRequestPartyRows` (per-ATM) diganti `ListRequestPartyAtmRows` (ATM × denom, cabang pengisi =
> kelolaan aktif pada tanggal replenish, tiket replenish) + `ListRequestPartyVaultRows` (alokasi plan hidup × denom, tiket vault aktif,
> alamat `COALESCE(vault.location_id, branch.location_id)`); `ReturnVaultPlansForVaultBranch` membaca alokasi. Tidak ada query yang
> membaca `vendor_request_vault_assignments` lagi.
> `vendor_party.go`: `PartyContent` = `atms[]` (tanpa `counterpart`), `vaults[] {ticket_number, vault{id,code,address}, branch, denoms}`,
> `replenish_branches[]`, `totals`; pihak replenish = ATM sendiri + semua vault (V6), total = order ATM-nya; pihak vault = vault
> sendiri + daftar cabang pengisi, total = alokasinya. `VendorOrderSummary` + `vault_count`. Kunci pihak, state machine, resend
> (berubah/ditolak/re-approve) tidak berubah.
> Konsekuensi V6 (dicatat di test): perubahan vault/nominal mana pun mengirim ulang **semua** pihak replenish.
> Tests: unit `TestBuildPartyContents` (dua vault satu branch → satu pihak, dua tiket; pihak replenish lihat semua vault; tanpa
> saldo/tier/urgent/buffer); 8 test 2.2b aktif lagi (skip S3 dibuang) dan disesuaikan: isi pihak, resync (nominal berubah → hanya yang
> melihatnya; vault pindah branch → pihak lama withdrawn), vault reject → plan satu-satunya draft → pindah alokasi → tiket vault2
> nonaktif, return → edit (alokasi tetap) → re-approve → plan sama draft → submit kelebihan ditolak tanpa alasan → trim → resend.
> `go test ./...` + `-tags integration ./...` hijau tanpa skip, lint 0, sisa data 0. Frontend (VendorPortal Orders, panel pihak) masih
> membaca bentuk lama `counterpart` → S7.
- `ListRequestPartyRows` dipecah: ATM pihak replenish (tanpa vault per ATM) + baris alokasi (vault, branch, tiket, amounts, alamat
  vault fallback branch).
- `PartyContent`: pihak vault `vaults[]` + `replenish_branches[]`; pihak replenish `atms[]` + `vaults[]` (semua vault request).
- Vendor vault menolak → plan satu-satunya `draft` (`ResetVaultPlansToDraft` sudah per request — dicek).
- **Proof:** unit `buildPartyContents` (dua vault satu branch → satu pihak, dua tiket); integration kirim → terima → vault tolak →
  submit ulang → resend hanya pihak berubah.

### S5 — CompanyPortal: Penetapan Vault + review (FR9.1, FR9.2, FR9.5) — ✅ DONE 2026-10-13
- `api.ts` tipe baru; `VaultPlanDetail.tsx` editor alokasi (baris: branch → vault, nominal per denom, urgent), ringkasan order vs
  alokasi vs selisih, alasan buffer, tiket per baris, submit dinonaktifkan bila kurang; `RequestVaultPanel.tsx` bentuk baru.
- **Proof:** `VaultPlan.test.tsx` (tambah/hapus baris, kelipatan, kurang memblokir submit, lebih memunculkan alasan, tiket tampil);
  `pnpm test`, `lint`, `build`.
> `api.ts`: tipe `VaultAllocationView`, `VaultTicket`, `VaultBranchOption{vaults[]}`, `AllocationInput`; detail + `region_code`,
> `buffer_reason`, `order_totals`, `allocations`, `vault_tickets`; list `allocation_count`; save = `PUT …/allocations
> {allocations, buffer_reason}`; kandidat sekali per plan dengan `urgent=true` (editor menyembunyikan tier 3 di baris non-urgent);
> review = `{plan}`. Aturan editor murni di `allocation.ts` (kelipatan, masalah baris, coverage per denom, peringatan kapasitas,
> normalisasi body). `AllocationRows.tsx`: editor baris (vault yang dipakai baris lain tidak ditawarkan; satu vault bebas di branch →
> dipilih otomatis; batal urgent → branch tier 3 dilepas), `CoverageSummary` (Kurang/Buffer/Sesuai = badge berlabel + ikon, bukan warna
> saja), `AllocationTable` read-only (dipakai ACM-SPV + panel ATM-SPV). Kirim dinonaktifkan dengan alasan tertulis: baris belum
> lengkap, denom kurang, alasan buffer belum 10–500, atau ada perubahan belum disimpan (submit memakai baris tersimpan).
> Tests: `VaultPlan.test.tsx` 11 test; CompanyPortal 127 file / 1080 test hijau, `lint` + `build` bersih.
> Tidak dibuat: riwayat tiket nonaktif (`vault_tickets`) di UI — hanya tiket aktif per baris; tambah bila diminta.
> Verifikasi browser manual: **outstanding** (user).

### S6 — CompanyPortal: Area ACM tab Region (FR9.3) — ✅ DONE 2026-10-13
- `admin-acm-areas`: tab Region (pilih kode dari opsi, simpan), tab Cabang disembunyikan, panel peringatan FR1.4.
- **Proof:** `AdminAcmAreasPage.test.tsx`; `pnpm test`, `lint`, `build`.
> Halaman tidak punya tab; bagian "Cabang vendor" di panel Kelola **diganti** bagian "Region" (checkbox per kode dari
> `GET /region-options`, jumlah cabang aktif, kode milik area lain dinonaktifkan + "sudah di area X"; kode milik area ini yang tak lagi
> dipakai cabang aktif tetap tampil). Simpan = `PUT /{id}/regions {region_codes}` (409 konflik → toast pesan server). Daftar area:
> kolom Region (`region_count`). Peringatan FR1.4: daftar request (nomor, region atau "tidak ada (nomor format lama)", tanggal).
> Kode front-end cabang area (`useAcmBranchOptions`, op `branches`) dihapus; route backend `/branches` tetap (tidak dipakai UI).
> Tests: `AdminAcmAreasPage.test.tsx` 4 test; CompanyPortal 1080 test hijau, `lint` + `build` bersih. Browser manual: **outstanding**.

### S7 — VendorPortal Orders + panel pihak CompanyPortal (FR9.4) — ✅ DONE 2026-10-13
- `features/orders`: pihak vault (tiket + nominal per vault, cabang pengisi), pihak replenish (daftar vault request);
  `VendorPartiesPanel.tsx` bila menampilkan isi pihak.
- **Proof:** `OrdersPages.test.tsx`; `pnpm test`, `lint`, `build` (dua app).
> `api.ts`: `PartyAtm` tanpa `counterpart`; `PartyVaultRef`, `PartyVault`; `content.vaults` + `replenish_branches`; summary
> `vault_count`. Daftar: kolom "Jumlah" = `N ATM` (replenish) / `N vault` (vault). Detail: pihak replenish = tabel ATM (tiket
> replenish, nominal per denom) + "Ambil uang di vault" (semua vault request: kode, tiket vault, branch + vendor, alamat, nominal);
> pihak vault = "Vault Anda" (kode, tiket vault, alamat, nominal) + "Diambil oleh (cabang pengisi)". Denom kolom = gabungan denom baris.
> CompanyPortal `VendorPartiesPanel.tsx` tidak menampilkan isi pihak → tidak diubah.
> Tests: `OrdersPages.test.tsx` 9 test (+ pihak replenish melihat semua vault); VendorPortal 128 test hijau, `lint` + `build` bersih.
> Browser manual: **outstanding**.

### S8 — Docs + review — ✅ DONE 2026-10-13
- `CLAUDE.md` Sec 3 (tabel baru, area per region) + Sec 12 (keputusan 2026-10-13, migrasi 030 terbaru); `docs/data-map.md`,
  `docs/decisions.md`; `development-progress.md`; `.claude/sdlc/README.md`; `tests.md`; `review.md`; `graphify update .`.
> Semua dokumen di atas diperbarui; `tests.md` (perintah dijalankan ulang 2026-10-13: Go unit + integration 0 skip, service 81.3 %,
> lint 0, sisa DB 0; CompanyPortal 1080, VendorPortal 128) dan `review.md` dibuat. Review menemukan R2 (nominal dengan angka 0 di depan
> tidak pernah bisa dikirim) → diperbaiki + test; **R1 terbuka**: seed dev `backend/scripts/dev/seed_acm_users.sql` masih memetakan
> cabang, bukan region — perlu keputusan user.

## Risks
- **Dev DB revert (S0)** menyentuh skema dev — hanya objek 030 v1, kolom 0 baris; minta izin eksplisit.
- **Kontrak API berubah** (`…/assignments` → `…/allocations`, kandidat tanpa terminal, isi pihak vendor): konsumen CompanyPortal +
  VendorPortal — S3–S7 dirilis bersama, belum ada produksi.
- **ADMIN wajib isi mapping region** sebelum alur jalan; tanpa itu semua request menunggu (peringatan FR1.4). Dev: "Area ACM Dev"
  perlu diberi region (data seed — di luar kode; disebut di tests.md sebagai langkah manual user).
- **Request dev lama tanpa `region_code`**: tidak pernah dapat plan; tampil di peringatan.
- **2.2a integration test** yang memakai area per cabang / penetapan per ATM perlu ditulis ulang (S3) — volume test besar.
- Uang: semua `bigint` IDR; kelipatan dijaga CHECK DB + validasi service.

## Alternatif yang tidak diambil
- Tambal 030 v1 dengan 031 (drop kolom): ditolak — 030 v1 belum di-commit, 0 baris; satu migrasi bersih lebih mudah dibaca.
- Nominal per denom di jsonb pada baris alokasi: ditolak — CHECK kelipatan per denom dan agregasi kapasitas lebih jelas di tabel
  `_amounts`.
- Drop `acm_area_branches` / `vendor_request_vault_assignments`: ditunda (out of scope spec), tidak dibaca lagi.
