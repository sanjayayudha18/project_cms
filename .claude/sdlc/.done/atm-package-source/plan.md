# Plan: Pilihan sumber paket pada Kelolaan ATM

Author: Claude, dari `spec.md` (accepted 2026-10-07). Status: **draft 2026-10-07** — menunggu persetujuan engineer/PO sebelum coding.
Stage: 3 Build. Stage berikut: `tests.md`.

## Prinsip
- Urutan kecil, satu concern per fase, tiap fase berakhir hijau (`go test ./...` / `pnpm test` / typecheck). TDD: test dulu (RED) per fase.
- Migrasi **tidak** dijalankan ke DB dev oleh AI tanpa persetujuan; AI menulis file + instruksi `psql`, user yang menerapkan (pola migrasi 019–021).
- Verifikasi browser = tugas user (Golden Rule #10); dicatat "outstanding" di `tests.md`.

## Audit pembaca `atm_vendor_packages` (hasil, 2026-10-07)
Query yang join ke `vendor_packages_branch` (harus diubah): `atm_assignments_admin.sql`, `atms_admin.sql` (1 LATERAL), `atm_visit_quota.sql` (2 LATERAL),
`vendor_request.sql` (5 LATERAL), `vendor_branch_atms.sql` (2), `master_data_export.sql` (export + `ImportPackageKeys`).
Kode Go yang memakai `VendorPackageID`: `handler/admin_atm_assignment_handler.go`, `service/atm_assignment_admin.go`, `service/masterdata_applier_atm_assignment.go`,
`service/masterdata_import_confirm.go` (+ test terkait). `vendor_request.sql` menyelesaikan `vendor_package_id` di LATERAL — konsumen Go-nya diperiksa di Fase 3 (apakah nilai NULL aman).
Pembaca yang hanya menyebut nama tabel di test (`no_hard_delete_test.go`, `branch_atm_repository_test.go`) disesuaikan bila gagal.

## Fase

### Fase 0 — Dokumen & skema disetujui (tanpa kode)
- Tambahkan kolom/constraint baru ke `.claude/CLAUDE.md` Sec 3 (Master) + `.claude/docs/data-map.md` setelah migrasi ditulis (aturan "Propose first" dipenuhi oleh spec yang diterima).
- Nomor migrasi: `022` dicadangkan `vendor-pks-cis-limit` → pakai **`023_atm_assignment_vendor_source.sql`** (konfirmasi lagi saat menulis).

### Fase 1 — Migrasi `023_atm_assignment_vendor_source.sql`
File: `backend/migrations/023_atm_assignment_vendor_source.sql`. Isi = DDL di spec (nullable `vendor_package_id`, `vendor_id`, `vendor_branch_id`, `package`, `avp_source_chk`, unique parsial `avp_vendor_mode_uq`, index cabang) + blok rollback sebagai komentar.
Bukti: diterapkan ke DB lokal/dev **oleh user**; `\d atm_vendor_packages`; test integrasi Fase 2 mengujinya. Update `backend/migrations/CMS_DB_dbdiagramio.dbml`.
Risiko: `atm_vendor_packages` kosong di dev setelah migrasi 011 — tidak ada backfill (NFR1).

### Fase 2 — Query + sqlc (inti `atm_assignments_admin`)
1. `queries/atm_assignments_admin.sql`: `LEFT JOIN vendor_packages_branch`, kolom `source` (`CASE WHEN a.vendor_package_id IS NULL THEN 'vendor' ELSE 'branch' END`), `package_code`/`vendor_branch_id`/`vendor_id` COALESCE; `Create`/`Update`/`GetByID` menerima kolom baru; query baru:
   - `ATMPackageOptions` (FR11): label `DISTINCT vpp.package` dengan tarif efektif pada `as_of` untuk `vendor_id` + `atms.price_machine_group/price_class`.
   - `VendorTariffExists` (FR9.4), `VendorBranchOfVendor` (FR9.2–3 + `kind`).
2. `cd backend && sqlc generate` (**v1.31.1**) lalu hand-fix `UserLeafe` → `UserLeave` (Sec 2a).
3. `internal/repository/atm_assignment_admin_repository.go`: metode baru, read-only.
Bukti: `go build ./...`; integrasi repo mode `vendor` dan regresi `branch`.

### Fase 3 — Service + Applier + handler (alur maker-checker)
- `service/atm_assignment_admin.go`:
  - `ATMAssignmentUpdatePayload` + `Source`, `VendorID`, `VendorBranchID`, `Package` (json `omitempty`; `Source` kosong = `branch`).
  - `validate`/`validatePackage` dipisah per mode (FR3) + validasi tarif FR9 (submit-time). DTO `ATMAssignment` + `Source`, `Package`.
  - `Update`: tolak ganti mode (FR8), boleh ganti label dalam mode `vendor`.
  - Metode baru `PackageOptions(ctx, atmID, vendorID)` untuk FR11.
- `service/masterdata_applier_atm_assignment.go`: `Create`/`Update` meneruskan kolom baru; **re-validasi tarif saat apply** (FR9, gagal → 409); pemetaan error DB tetap (`23P01`, `23505`) + `avp_source_chk` → error validasi, bukan 500.
- `handler/admin_atm_assignment_handler.go`: DTO request/response + field baru; route baru `GET /api/v1/admin/atms/{atmID}/assignment-package-options` di grup `ADMIN`/`ADMIN_PARAM` yang sama (`cmd/api/main.go`; route lain tidak berubah).
- Konsumen Go `vendor_request` (`vendor_package_id` NULL aman?) diperiksa dan disesuaikan.
Bukti: unit table-driven (FR3, FR9.1–4, payload lama, overlap lintas mode, RBAC denial), integrasi apply (handover otomatis, 409), handler 202/403.

### Fase 4 — Pembaca lain (FR7, FR10, NFR4)
`atms_admin.sql`, `atm_visit_quota.sql`, `vendor_request.sql`, `vendor_branch_atms.sql`: `JOIN` → `LEFT JOIN` + `COALESCE(p.package_code, avp.package)` / `COALESCE(p.vendor_branch_id, avp.vendor_branch_id)`; `sqlc generate`.
Bukti: test integrasi existing (`forecast_query_integration_test`, `atm_visit_quota_integration_test`, `vendor_request_*_integration_test`, `branch_atm_repository_test`) **tetap hijau tanpa diubah** (NFR4), plus satu kasus ATM mode `vendor` per konsumen.

### Fase 5 — Ekspor/impor CSV dua mode (FR12)
- `queries/master_data_export.sql`: `ExportATMAssignmentsBatch` → LEFT JOIN + COALESCE + kolom `package_source`; query daftar label aktif per vendor untuk dry-run.
- `service/masterdata_export.go`: header baru `…, branch_code, package_source, package_code, …`; `masterdata_export_test.go` diperbarui.
- `service/masterdata_import.go` + `masterdata_import_confirm.go`: `package_source` opsional (kosong/tidak ada → `branch`; file lama valid); mode `vendor`: resolusi `vendor_code`→`vendor_id`, `vendor|branch`→`vendor_branch_id`, label dicek di dry-run; `assignmentFields` menghasilkan payload sesuai mode; update yang mengubah `package_source` ditolak.
Bukti: unit parse/validate (file lama, dua mode, baris salah), integrasi impor satu file campuran.
Risiko: header ekspor berubah → alat luar yang membaca CSV terpengaruh; dicatat di `docs/decisions.md`.

### Fase 6 — Frontend CompanyPortal (`features/admin-atms`)
- `api.ts` + `types.ts`: `ATMAssignment` + `source`, `package`, `vendor_branch_id`; `CreateATMAssignmentPayload` + `source`, `vendor_id`, `vendor_branch_id`, `package`; `listATMPackageOptions(atmId, vendorId)`.
- `ATMAssignmentsDialog.tsx`: Vendor → Cabang (wajib, hapus "Semua cabang") → segmented **Jenis paket** → Paket. Mode `branch`: dropdown dari `listVendorPackages({branch_id})`; mode `vendor`: dari `listATMPackageOptions`. Prefill `source`; ganti vendor/cabang/jenis mengosongkan paket (FR2); `canSubmit` membandingkan mode + paket aktif (`currentAssignment`); badge jenis di tabel.
- Pecah komponen bila dialog > ~300 baris (mis. `PackageSourceFields.tsx`).
- Test: update `ATMAssignmentsDialog.currentAssignment.test.ts`, tambah test dialog (mode, cabang wajib, payload), `api.test.ts`.
- UI sesuai `rules/frontend-ui.md` (target ≥44px, Badge + teks, tidak hanya warna).
Bukti: `pnpm --dir frontend/CompanyPortal-Vite run test|lint|build` hijau; verifikasi browser **outstanding → user**.

### Fase 7 — Dokumen & penutupan
`.claude/CLAUDE.md` Sec 3 + Sec 12, `.claude/docs/data-map.md`, `.claude/docs/decisions.md`, `.claude/sdlc/README.md` (baris fitur), `.claude/development-progress.md`, `graphify update .`.
Lalu `tests.md` → `review.md` (gerbang manusia: code owner).

## File yang disentuh (ringkas)
Backend: migrasi 023, dbml, 6 file `queries/*.sql` (+ `internal/db/*.sql.go`), `repository/atm_assignment_admin_repository.go`, `service/atm_assignment_admin.go`, `service/masterdata_applier_atm_assignment.go`, `service/masterdata_import*.go`, `service/masterdata_export.go`, `handler/admin_atm_assignment_handler.go`, `cmd/api/main.go`, + test terkait.
Frontend: `admin-atms/{api.ts,types.ts,components/ATMAssignmentsDialog.tsx,…test}`, mungkin satu komponen baru.

## Risiko & mitigasi
| Risiko | Mitigasi |
|---|---|
| Satu query konsumen lupa di-COALESCE → ATM mode `vendor` "tak terkelola" diam-diam | Audit grep di Fase 4 + test integrasi per konsumen untuk mode `vendor` |
| `vendor_request` memakai `vendor_package_id` NULL → crash/NULL deref | Periksa konsumen Go di Fase 3; test forecast dengan ATM mode `vendor` |
| Tarif hilang antara submit dan apply | Re-validasi di Applier (FR9), 409 |
| Header CSV berubah | `package_source` opsional di impor; dokumentasikan di decisions |
| sqlc versi salah / `UserLeafe` | Pin v1.31.1 + hand-fix (Sec 2a) |
| Migrasi sulit dibatalkan | Rollback tertulis; tanpa backfill; aman selama belum ada baris mode `vendor` |

## Di luar plan ini
Tagihan/invoice per ATM dari mode `vendor`; perubahan harga; layar `package_frequencies`.
