# Plan: Penetapan branch vault per ATM oleh ACM (Phase 2.2a)

Status: **accepted 2026-10-08** (user: "Diterima, mulai S1"; P1 `vault_flow` disetujui). Review di akhir tiap slice. Reads: `spec.md` (accepted 2026-10-08).
Stage: 3 Build. Kerja dipecah menjadi **8 slice**; tiap slice = satu diff kecil yang build + test hijau sebelum lanjut.

## Satu tambahan di luar spec (minta OK)
- **Kolom `vendor_requests.vault_flow boolean NOT NULL DEFAULT false`** — di-set `true` saat ATM-SPV approve setelah rilis.
  Dipakai untuk: (a) laporan selesai yang ditolak kembali ke `ready` (vault_flow) atau `approved` (request lama, F12/N2);
  (b) membedakan request lama vs baru tanpa menebak dari tanggal. Alternatif (cek "punya vault plan?") lebih rapuh.

## Titik sentuh yang sudah dicek di kode
- State machine: `backend/internal/service/vendor_request.go:480` (`transitions`), `checkActor` :510, `isChecker` :34.
- `approved_at/approved_by` di-set via `CASE status='approved'` — `backend/queries/vendor_request.sql:75` → harus ikut `vault_assignment`.
- Laporan selesai: `vendor_request_completion.go` (`approved → completion_pending`, reject → `approved`) + `vendor_request.sql:491-495`.
- Filter status list: `vendor_request_actions.go:23`. Frontend: `features/vendor-request/types.ts`, `VendorRequestDetail.tsx`, `StatusBadge.tsx`.
- Pola master-data maker-checker: `masterdata_change.go`, `masterdata_applier_vendor_vault.go`, registry `cmd/api/main.go:216`.
- Pola immediate-apply + audit: `region_admin.go` + `admin_region_handler.go`.
- Notifikasi: `internal/notification.Send(ctx, q, msg, Recipients)` dalam tx; `ListActiveUserRecipientsByRoles`.
- DSR ETL: `backend_python/dsr/dsr_etl.py` (`SALDO AKHIR` hanya disimpan total untuk blok tanpa lokasi, ±baris 290).

## Slices (urutan kerja)

### S1 — Migrasi 027 + sqlc — ✅ DONE 2026-10-08
> `backend/migrations/027_acm_vault_assignment.sql` applied to dev (localhost); sqlc v1.31.1 regenerated (diff: `models.go`,
> `vendor_request.sql.go` only; UserLeafe fix applied); `go build` OK; `go test` service/handler/repository vs dev DB:
> 496 pass, 0 skip, 0 fail. Uncommitted. Next: S2 (wait for user OK per slice).
- `backend/migrations/027_acm_vault_assignment.sql` (satu tx): status baru + `vault_flow` + kolom `vault_reviewed_*` di
  `vendor_requests`; `dsr_daily_rows_flow_chk` + `saldo_akhir`; tabel `dsr_location_vault_maps`, `acm_areas`,
  `acm_area_branches`, `acm_area_members`, `vendor_request_vault_plans`, `vendor_request_vault_assignments`;
  seed roles `ACM-USER`/`ACM-SPV`, `menu_features` (`cit`, `cit.vault-plan`, `settings.acm-areas`), `role_permissions`.
- Terapkan ke dev (`localhost:5432`), `sqlc generate` (v1.31.1) + fix `UserLeafe → UserLeave`.
- **Proof:** migrasi jalan bersih; `go build ./...`.

### S2 — DSR ETL saldo akhir per vault (Python) — ✅ DONE 2026-10-08
> `read_daily_rows` menambah baris `flow='saldo_akhir'` (d0) per blok berlokasi; blok total tetap hanya
> `saldo_akhir_0000_total_idr`. Temuan: baris tidak pernah membawa `location` (insert selalu NULL) → kini tiap baris
> diberi lokasi bloknya (dibutuhkan mapping S3). `python -m unittest discover -s dsr`: 11 OK. Uncommitted.
- `dsr_etl.py`: simpan baris `SALDO AKHIR` tiap blok berlokasi sebagai `flow='saldo_akhir'`.
- Test di `backend_python/dsr/` (unittest): workbook 2 blok vault → 2 baris `saldo_akhir` per denom; blok total tidak disimpan.
- **Proof:** `python -m unittest discover -s dsr` hijau.

### S3 — Mapping lokasi DSR → vault (master data, maker-checker) — ✅ DONE 2026-10-08
> Backend: `queries/dsr_location_maps_admin.sql` (insert/update ber-guard: vault aktif milik vendor saat apply),
> `repository/dsr_location_map_admin_repository.go` (list di replica), `service/dsr_location_map_admin.go`,
> `service/masterdata_applier_dsr_location_map.go` (registry `dsr_location_map`), `handler/admin_dsr_location_map_handler.go`
> (`GET /`, `GET /unmapped`, `POST /`, `PUT /{id}` ganti vault saja, `POST /{id}/disable|enable`; label immutable).
> `no_hard_delete_test` + tabel baru. Frontend: tab "Mapping DSR" (`DsrLocationMapsPanel.tsx`, `dsrLocationMaps.ts`),
> label diff approval, pending badge. Proof: `go test -tags integration` service/handler/repository vs dev DB OK
> (applier lifecycle: create, duplikat beda huruf ditolak, vault vendor lain ditolak, update, disable);
> `pnpm test` 1058/1058, `lint`, `build` OK. Cek manual browser = outstanding (user). Uncommitted.
- `queries/dsr_location_maps_admin.sql`; repo read-only; `service/dsr_location_map_admin.go` (`Submit` via
  `MasterDataChangeService`); `service/masterdata_applier_dsr_location_map.go`; registrasi di `main.go`;
  `handler/admin_dsr_location_map_handler.go` (202) + endpoint lokasi belum terpetakan.
- Frontend `features/admin-vendors`: tab "Mapping DSR".
- **Proof:** unit + integration test applier (create/update/disable, vault milik vendor lain ditolak, unik case-insensitive);
  RBAC test; `pnpm test` + `build`.

### S4 — Area ACM (ADMIN, langsung + audit) — ✅ DONE 2026-10-08
> Backend: `queries/acm_areas_admin.sql`, `service/acm_area_admin.go` (role ADMIN dicek ulang di service; tiap mutasi
> + `audit_logs` satu tx; nonaktif area **melepas cabangnya**, anggota tetap), `handler/admin_acm_area_handler.go`
> (`/api/v1/admin/acm-areas`: list, detail, create 201, rename, disable/enable, `PUT /{id}/branches|members` replace-all,
> `GET /warnings` FR7.4, `/branch-options`, `/eligible-users`), mount `RequireRoles("ADMIN")`. Tanpa repo layer (sqlc langsung).
> Frontend: `features/admin-acm-areas` + route `/settings/admin/acm-areas` + kartu hub (hanya ADMIN).
> **FR7.3 (assign cabang → buat rencana untuk request di `vault_assignment`) dipindah ke S5** (fungsi pembuat rencana lahir di S5; TODO di `SetBranches`).
> Proof: integration (lifecycle: RBAC, nama unik case-insensitive, cabang unik antar area + 409 menyebut area pemegang,
> anggota harus ACM-USER/ACM-SPV aktif, disable melepas cabang, audit tertulis; audit gagal → rename di-rollback);
> handler test; `go test -tags integration ./...` hijau; `pnpm test` 1062/1062, lint, build OK. Cek manual browser = outstanding (user).
- `queries/acm_areas_admin.sql`; `service/acm_area_admin.go` (pola `region_admin.go`: audit dalam tx, RBAC di service);
  `handler/admin_acm_area_handler.go` (CRUD, branches, members, warnings).
- Frontend `features/admin-acm-areas` (menu Pengaturan → Area ACM) + panel peringatan.
- **Proof:** integration test (branch unik per area → 409, anggota harus ACM-USER/ACM-SPV, audit tertulis, rollback bila audit gagal).

### S5 — Vendor Request: status baru — ✅ DONE 2026-10-08
> `transitions`: approve → `vault_assignment`; `vault_assignment` -(vault_ready)→ `vault_review` -(vault_approve)→ `ready`
> / -(vault_reject)→ `vault_assignment`; `ready` → completion/cancel; `approved` tetap (legacy). Aksi `vault_ready/approve/reject`
> sudah di tabel, endpoint-nya S6. Cancel checker-only di `approved|vault_assignment|vault_review|ready` + rencana → `cancelled`.
> Laporan ditolak → `ready` bila `vault_flow` (di `completionTx`), else `approved`. SQL: `approved_at/by` + `vault_flow=true`
> saat `vault_assignment`; completion reject CASE ikut `ready`. `queries/vault_plans.sql` + `service/vault_plan_create.go`
> (`createVaultPlans`: set-based, audit `vault_plan_created`, notifikasi ACM-USER area) dipanggil di tx approve **dan**
> `AcmAreaAdminService.SetBranches` (FR7.3; service kini `WithNotifier`, mount dipindah setelah `notificationService`).
> Status filter list + 3 status. **Frontend minimal ditarik dari S7** supaya UI tidak crash: tipe/badge/label 3 status,
> cancel checker & laporan selesai dari `ready`. Tests diperbarui: `TestNextState`, property 8/10 (seluruh tabel baru),
> cancel-integration (before state `vault_assignment`). Proof: `vault_flow_integration_test.go` (approve → plan+notif,
> completion diblok di `vault_assignment`, cancel cascade, cabang tanpa area → warning → SetBranches buat plan,
> completion dari `ready` & reject kembali `ready`); kuota-integration (legacy `approved` → reject → `approved`) tetap hijau;
> `go test -tags integration ./...` hijau; `pnpm test` 1062, lint, build OK. Cek manual browser = outstanding (user).
- `transitions`: `pending_approval → vault_assignment` (approve); `vault_review → ready` (vault-approve) / `→ vault_assignment`
  (vault-reject); `ready → completion_pending`; cancel dari `vault_assignment`/`vault_review`/`ready`; completion reject →
  `ready` bila `vault_flow` else `approved`. `approved` tetap untuk request lama.
- Approve juga: set `vault_flow=true`, buat `vendor_request_vault_plans` per area (branch replenish via kelolaan), notifikasi
  ACM-USER area — semua dalam tx approve.
- FR7.3 (dari S4): `AcmAreaAdminService.SetBranches` memanggil fungsi pembuat rencana yang sama untuk request di
  `vault_assignment` yang ATM-nya dikelola cabang yang baru ditambahkan.
- `vendor_request.sql`: `approved_at/by` untuk `vault_assignment`; completion reject CASE ikut `ready`.
- Update `vendor_request_state_machine_property_test.go`, completion tests, status list.
- **Proof:** semua test Vendor Request lama tetap hijau + test transisi baru + test request lama (`approved`) tidak berubah.

### S6 — Rencana vault (service + API) — ✅ DONE 2026-10-08
> `queries/vault_plans.sql` (+S6: plan read/lock/list, ATM per plan, kandidat, saldo per branch dari DSR+mapping, demand per
> tanggal, replace assignment, transisi plan, review request). `service/vault_capacity.go` (logika murni: tier, demand
> dibebankan ke vault terpilih / branch replenish bila belum, kapasitas tanpa ATM yang diedit, warning, sort) +
> `vault_plan.go` (RBAC area: non-anggota 404, ADMIN read-only; detail, kandidat, snapshot) + `vault_plan_actions.go`
> (save replace-all draft, submit, approve → auto `vault_review` bila semua ATM tercakup plan `acm_approved`, reject,
> `ReviewRequest` ATM-SPV approve → `ready` / reject → `vault_assignment` + semua plan `draft`; lock request lalu plan;
> notifikasi N1; audit). Handler: `vault_plan_handler.go` (`/api/v1/vault-plans`, roles ACM-USER/ACM-SPV/ADMIN) +
> `vendor_request_vault_handler.go` (`/vendor-requests/{id}/vault-assignments|vault-approve|vault-reject`).
> **Keputusan implementasi (lapor ke user):** demand FR3.2/3.3 menghitung semua request `vault_flow` tanggal X yang tidak
> batal/ditolak (termasuk `completion_pending`/`completed`), bukan hanya `vault_assignment|vault_review|ready`; saldo dari DSR
> `daily_status='completed'`, dibulatkan ke rupiah penuh. **Disetujui user 2026-10-08 (opsi a).** Proof: unit `vault_capacity_test.go`; integration
> `vault_plan_integration_test.go` (2 area, kandidat tier 1/2/3 + saldo/kapasitas nyata dari DSR, tier 3 wajib urgent,
> replace-all, maker≠checker ACM, reject ACM, auto `vault_review`, ATM-SPV lintas-lapis ditolak, reject reset → draft,
> approve → `ready`, list scoped, notifikasi, audit; approve paralel 2 area → tepat 1 `vault_ready`, 3x run); handler test;
> `go test -tags integration ./...` hijau; coverage statement file baru 80.2%.
- `queries/vault_plans.sql`: kandidat bertier + saldo/kapasitas (set-based), simpan penetapan, transisi.
- `service/vault_plan.go`: candidates, SaveAssignments (validasi tier/urgent, snapshot, `capacity_warning`), Submit,
  Approve (→ request `vault_review` bila semua area approved, `FOR UPDATE`), Reject; ATM-SPV vault-approve/reject di
  `vendor_request_*`; notifikasi N1; audit.
- `handler/vault_plan_handler.go` (`/api/v1/vault-plans`) + endpoint `/vendor-requests/{id}/vault-*`; mount di `main.go`.
- **Proof:** table-driven unit test (tier, urgent, kapasitas: saldo NULL, kurang, cukup); integration test alur penuh
  (2 area → keduanya approve → `vault_review` → `ready`; reject ACM-SPV; reject ATM-SPV → semua area `draft`);
  RBAC (non-anggota 404, maker = checker 403, ADMIN read-only); approve paralel dua area.

### S7 — Frontend ACM + review ATM — ✅ DONE 2026-10-08
> `features/vault-plan/`: `api.ts` (tipe + hooks list/detail/kandidat/mutasi + review request, uang = string desimal),
> `VaultPlanList.tsx` (filter status, peringatan kapasitas), `VaultPlanDetail.tsx` (per ATM: dropdown kandidat dikelompokkan
> per tier + kapasitas, badge peringatan, urgent → tier 3 + alasan 10–500; simpan sebagian, kirim hanya bila semua ATM
> tersimpan; ACM-SPV setujui/tolak, tidak untuk kiriman sendiri; banner alasan tolak ACM-SPV/ATM-SPV),
> `RequestVaultPanel.tsx` (panel di `VendorRequestDetail` untuk status vault_*/ready/completion_*; tombol setujui/tolak
> penetapan hanya `vault_review` + checker). Route `/cit/vault-plans` + `/$id` (ACM-USER/ACM-SPV/ADMIN = guard backend;
> link notifikasi sudah cocok), menu grup baru "CIT" → "Penetapan Vault" (ACM-USER/ACM-SPV). `DbRole` + `ACM-USER`/`ACM-SPV`.
> Proof: `VaultPlan.test.tsx` (7 test: pilih kandidat + warning + simpan, urgent + alasan, ACM-SPV approve & bukan kiriman
> sendiri, banner tolak ATM-SPV, panel approve/tolak wajib alasan/tanpa aksi) + gating panel di `VendorRequestDetail.test.tsx`;
> `pnpm test` 1070/1070, lint + build hijau. Cek manual browser = **outstanding (user)**.
> Catatan: `ReasonModal` diimpor dari `VendorRequestDetail` (import melingkar vendor-request ↔ vault-plan, aman karena hanya
> dipakai saat render); kandidat diambil satu request per baris ATM (cukup untuk puluhan ATM per area).
- Feature baru `features/vault-plan` (menu CIT → Penetapan Vault): list, detail per ATM, dropdown bertier + kapasitas,
  urgent + alasan, submit/approve/reject. Mock `features/cit` tidak disentuh.
- `features/vendor-request`: status baru di `types.ts`/`StatusBadge.tsx`, panel "Penetapan Vault" + approve/reject di
  `VendorRequestDetail.tsx`, laporan selesai dari `ready`.
- **Proof:** component test alur kritis; `pnpm --dir frontend/CompanyPortal-Vite run test|lint|build` hijau.
  Cek manual browser = **outstanding (user)**.

### S8 — Docs & graph — ✅ DONE 2026-10-08
> Cek S7 vs spec menemukan 3 celah, diperbaiki: FR4.2 kandidat teratas Tier 1 dipilih otomatis untuk ATM kosong,
> FR9.1 filter tanggal replenish di daftar, FR5.1 kolom saldo di panel ATM-SPV (+ test). `028_dsr_flow_comment.sql`
> (komentar kolom saja) applied to dev. Diperbarui: `spec.md` FR3.2 (opsi a), `tests.md` (baru), CLAUDE.md Sec 3 + Sec 12,
> `docs/data-map.md`, `docs/decisions.md`, `development-progress.md`, `sdlc/README.md`; `graphify update .`.
> `pnpm test` 1071/1071, lint + build hijau.
- CLAUDE.md Sec 3 (tabel baru, status baru) + Sec 12 (dua deviasi: state machine sendiri, Area ACM immediate-apply);
  `docs/data-map.md`; `tests.md`; development-progress; `graphify update .`.

## Risiko
| Risiko | Mitigasi |
|---|---|
| Alur Vendor Request live berubah (approve tidak lagi ke `approved`) | `vault_flow` + test regresi request lama; transisi lama tetap ada |
| Branch replenish tanpa area → request macet di `vault_assignment` | Peringatan ADMIN (S4); assign branch membuat rencana (FR7.3) |
| Saldo "tidak diketahui" di awal (mapping kosong, DSR lama) | Peringatan saja (tidak memblokir); tab "lokasi belum terpetakan" |
| Dua ACM-SPV approve area terakhir bersamaan | `SELECT … FOR UPDATE` pada request di tx approve |
| `dsr_uploads.vendor` = nama vendor (bukan id) | Join via `vendors.name`; dicatat di data-map |

## Alternatif tidak diambil
- Pakai `approval_requests` untuk rencana vault — tidak, pola Vendor Request lebih cocok (dokumen transaksional, sudah ada `checkActor`).
- Satu slice besar — tidak, 8 slice supaya tiap langkah bisa di-review dan dites.
