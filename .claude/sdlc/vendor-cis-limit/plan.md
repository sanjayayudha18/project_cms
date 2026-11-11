# Plan: Limit CIS + escrow vault + PIC `bm` + Inquiry Limit CIS

Author: Claude, dari `spec.md` (accepted 2026-11-06, user: "Terima, buat plan"). Status: **accepted 2026-11-06** (engineer);
P1 selesai. Stage: 3 Build. Stage berikut: `tests.md`.
Skill saat build: `ecc:golang-patterns`, `ecc:golang-testing`, `cms2-master-data` (9 langkah), `ui-ux-pro-max` (UI).

## Urutan kerja (satu langkah = satu diff kecil; tes di langkah yang sama)

### P1 — Migrasi `038_vendor_cis_limit.sql` (FR1.1, FR2.1, FR3.1)
- File baru `backend/migrations/038_vendor_cis_limit.sql`: isi persis "Data model" di spec + `COMMENT ON COLUMN` + header
  rollback (pola 032/037). `BEGIN`/`COMMIT` di file.
- `go run ./cmd/migrate status` → `up` ke dev **hanya dengan OK user** (Sec 4 #7).
- `sqlc generate` (v1.31.1) + fix `UserLeafe` → `UserLeave`.
- ✅ **Selesai 2026-11-06**: plan diterima engineer ("Terima, mulai P1"); `038` applied to dev lewat `migrate up` (OK user,
  37 applied, 0 pending), 4 constraint ada; sqlc hanya mengubah `models.go`; build + vet hijau. Tambahan dari spec:
  CHECK `vendor_pics_bm_branch_chk` (S5 di DB juga).

### P2 — Limit CIS di cabang (FR1.1–1.3, risiko "payload pending")
- `queries/vendor_branches_admin.sql`: `CreateVendorBranchAdmin` + `UpdateVendorBranchAdmin` menulis `cis_limit_amount`;
  update memakai `CASE WHEN sqlc.arg('set_cis_limit')::bool THEN sqlc.narg('cis_limit_amount') ELSE cis_limit_amount END`;
  semua `SELECT`/`RETURNING` cabang ikut `cis_limit_amount, cis_limit_currency`.
- `internal/service/vendor_branch_admin.go`: `CisLimitAmount *string` di `VendorBranchPayload` + `VendorBranchUpdatePayload`;
  `normalizeCisLimit` (trim, kosong → nil, `big.Rat` > 0, ≤ 2 desimal, ≤ 18 digit bulat) dipanggil di Submit.
- `masterdata_applier_vendor_branch.go`: validasi ulang saat apply; `set_cis_limit` = kunci `cis_limit_amount` ada di
  payload (cek lewat `map[string]json.RawMessage`). Kunci tidak ada (change request lama) → nilai sekarang tetap.
- `internal/handler/admin_vendor_branch_handler.go`: DTO baca + request ikut `cis_limit_amount` (string \| null) dan
  `cis_limit_currency`.
- ✅ **Selesai 2026-11-06**, dengan dua penyimpangan dari rencana di atas (keduanya lebih sederhana):
  1. **Tanpa `set_cis_limit` / cek kunci-ada.** Cek stale (`statesEqual`) membandingkan seluruh `Before` dengan
     `CurrentState` (DeepEqual). Karena `GetVendorBranchAdminByID` kini memuat `cis_limit_*`, update yang di-submit
     sebelum 038 otomatis **stale** (tidak diterapkan, maker submit ulang). Dibuktikan di integration test.
  2. **Applier tidak validasi ulang** (aturan skill `cms2-master-data` #4); validasi di submit, CHECK DB = jaring terakhir.
  - sqlc tidak bisa memberi `*string` untuk `numeric::text` (NOT NULL / bool), jadi kolom tetap `pgtype.Numeric` dan
    handler memakai `service.DecimalStringPtr` (wrapper `numericToDecimalStringPtr`, tanpa float).
  - Tes: `vendor_branch_cis_limit_test.go` (unit, 16 kasus validasi), `admin_vendor_branch_handler_test.go`
    (GET list/detail + POST/PUT), `masterdata_applier_vendor_branch_cis_integration_test.go`. `go test -tags integration
    ./...` + `golangci-lint` hijau.
  - ⚠ **Celah sampai P5**: impor CSV `vendor-branches` (update) belum membawa `cis_limit_amount`, jadi impor akan
    mengosongkan limit. Jangan rilis/commit-terpisah P2 tanpa P5.

### P3 — Escrow vault (FR2.1–2.2)
- `queries/vendor_vaults_admin.sql`: create/update/select + `escrow_account`, pola `set_escrow_account` yang sama.
- `vendor_vault_admin.go`: `EscrowAccount *string` di `VendorVaultUpdatePayload`; `validateVaultFields` menambah cek
  `^[0-9]{12}$` setelah trim (kosong → nil). DTO `VendorVault` + handler vault ikut.
- `masterdata_applier_vendor_vault.go`: validasi ulang + kunci-ada → set.
- ✅ **Selesai 2026-11-06**, penyimpangan sama dengan P2: tanpa kunci-ada (update lama jadi stale), applier tanpa
  validasi ulang. Validasi escrow di `validateVaultFields` (trim, kosong → NULL, `escrowAccountRe` = CHECK 038).
  Request body sudah meng-embed `VendorVaultUpdatePayload`, jadi hanya respons (`vaultToResponse`) yang diubah.
  Tes: `TestValidateVaultFields` (+6 kasus) + `TestValidateVaultFields_EscrowNormalized`, handler list,
  `masterdata_applier_vendor_vault_escrow_integration_test.go` (tidak unik, nol di depan tetap, kosongkan, CHECK).
  `go test -tags integration ./...` + lint hijau. ⚠ Celah sampai P5 sama: impor CSV vault mengosongkan escrow.

### P4 — Peran PIC `bm` (FR3)
- `vendor_pic_roles.go`: `picRoleCodes` + `"bm"`, pesan error ikut. Cek baru: `bm` + PIC tanpa `vendor_branch_id` →
  `ValidationError{Field:"roles"}` (S5), dipanggil di `PUT …/roles`, create PIC (`vendor_pic_admin.go`) dan validasi
  impor PIC (`masterdata_import_validate.go`).
- ✅ **Selesai 2026-11-06.** Temuan: `AllPicRoles()` (default PIC baru) meng-clone `picRoleCodes`, jadi menambah `bm`
  saja akan memberi semua PIC baru peran `bm`. Dipisah: `picRoleCodes` (4, untuk validasi) vs `defaultPicRoles` (3);
  `AllPicRoles` diganti nama `DefaultPicRoles` (applier + 3 file tes). `checkBmNeedsBranch` dipanggil di `SetRoles`
  (query `GetVendorPicRolesForUpdate` + `vendor_branch_id`), `VendorPicAdminService.validate` (juga update yang
  memindah PIC ber-`bm` ke tingkat vendor tanpa mengirim roles) dan `validatePIC` impor (setelah `resolvePICKeeps`).
  Tes: `TestNormalizePicRoles` (+bm, default tanpa bm), `TestVendorPicAdminService_BmNeedsBranch` (6 kasus),
  `TestImport_VendorPICRoleBm`, `TestVendorPicRoleService_SetRolesBm_Integration`. `go test -tags integration ./...`
  + lint hijau. Handler `PUT …/roles` sudah memetakan `ValidationError` → 422.

### P5 — CSV impor + ekspor (FR1.5, FR2.4)
- `pkg/csvexport/entities.go`: header `vendor-branches` + `cis_limit_amount`, `vendor-vaults` + `escrow_account`;
  header lama masuk `LegacyHeaders` dengan `LegacyFill: KeepCurrentCell`.
- `backend/internal/service/masterdata_import.go`: keep-current untuk `cis_limit_amount` (cabang, daftar kolom di baris
  ~511) dan `escrow_account` (vault; peta nilai sekarang `e.cur` untuk vault ditambah bila belum ada);
  `masterdata_import_validate.go` + `masterdata_import_csv.go` memetakan kolom ke payload, validasi sama dengan P2/P3.
- `backend-exq/queries/vendor_export.sql` (+ `sqlc generate` di `backend-exq`) dan producer ekspor: pilih dua kolom baru;
  escrow sebagai teks lewat `CSVSafe`.
- `masterdata_import_export_header_test.go` menjaga header impor == ekspor.
- ✅ **Selesai 2026-11-06.** Kolom baru diletakkan sebelum `is_active` (header, `*Cells`, field row, query
  `master_data_export.sql`) karena `csvexport.VendorBranchRow(x)` adalah konversi struct langsung dari baris sqlc;
  `backend-exq` memakai file query yang sama (sqlc di dua modul). `e.cur` importer diisi dari query ekspor yang sama,
  jadi nilai sekarang otomatis tersedia. Keep-current dibuat tabel `keepCurrentCols` (cabang: region_code, category,
  cis_limit_amount; vault: escrow_account). Dry-run memakai `normalizeCisLimit` / `validateVaultFields`; confirm
  meneruskan ke `Branches.Create/Update` + `vaultFields`. **Celah impor dari P2/P3 tertutup.**
  - Tes: `masterdata_import_cis_escrow_test.go` (header lama = tetap, round trip = no-op termasuk nol di depan, kosong
    di header baru = kosongkan, nilai salah = error baris); `TestExport_HeaderContract` + golden (`-update`, fixture
    kini berisi limit `2500000000.00` + escrow `001234567890`) + `pkg/csvexport/testdata/headers.golden`.
  - `pkg`, `backend`, `backend-exq`: `go test -tags integration ./...` + lint hijau. ⚠ `backend-exq`
    `TestW11Property11SingleDownload` gagal 1× dari 5 run (lulus 3× sendiri + 1× suite penuh); tes lifecycle unduhan,
    tidak menyentuh kolom CSV — kemungkinan flaky lama, belum diselidiki.

### P6 — Endpoint inquiry (FR4, S6)
- `queries/cis_limit_inquiry.sql` (baru): `ListCisLimitInquiry` + `CountCisLimitInquiry`. Cabang aktif × vendor aktif
  `kind='FLM_VENDOR'`, filter `vendor_id` opsional, `LEFT JOIN locations`, subquery `string_agg` untuk PIC `bm` aktif
  cabang (nama, telepon) dan escrow vault aktif (ATM/ATM_CASH, CASH/ATM_CASH, S2); `ORDER BY vendors.name, branch_code`;
  `LIMIT/OFFSET`.
- `internal/repository/cis_limit_inquiry.go` (read-only, `dbRead` replica) → `internal/service/cis_limit_inquiry.go`
  (cek role ADMIN/ADMIN_PARAM di service, page_size 1–100) → `internal/handler/admin_cis_limit_inquiry_handler.go`
  (`GET /`, flat JSON `{items,total}`) → mount `/api/v1/admin/cis-limit-inquiry` di grup `masterDataAdmin`
  (`cmd/api/main.go`).

### P7 — CompanyPortal (FR1.4, FR2.3, FR3.2, FR4, S3, S7)
- `features/admin-vendors/`: `VendorBranchFormDialog.tsx` + `VendorBranchEditPage.tsx` (field Limit CIS, selalu
  dikirim), `VendorBranchDetailPage.tsx` (tampil), `VendorVaultFormDialog.tsx` + `VaultsPanel.tsx` (escrow),
  `lib/picRoles.ts` (+ `bm` "BM (Branch Manager)", disabled untuk PIC vendor-wide di `PicRolesDialog.tsx` /
  `VendorPicFormDialog.tsx`), `types.ts` + `hooks.ts`.
- Tab "Inquiry Limit CIS" di `AdminVendorsPage.tsx` + komponen baru `CisLimitInquiryPanel.tsx` (pilih vendor, tabel,
  `ServerPager`, uang `formatRp` + `tabular-nums` rata kanan, "—" untuk kosong).
- Tes komponen: form kirim limit/escrow, validasi 12 digit, peran `bm` disabled vendor-wide, panel inquiry render + paging.

### P8 — Dokumen
- CLAUDE.md Sec 3: pindahkan Limit CIS/escrow dari "Still NOT migrated" ke daftar migrasi (038 applied), Sec 12
  baseline nomor migrasi; `docs/data-map.md` bagian baru; `tests.md`; README + `development-progress.md`;
  `graphify update .`.

## Risiko
| Risiko | Penanganan |
|---|---|
| Update full replace mengosongkan limit/escrow dari klien/CSV lama | form selalu kirim; CSV `KeepCurrentCell`; Applier kunci-tidak-ada = tetap (P2/P3); tes integration |
| `vendor_pics_roles_chk` di-drop lalu dibuat ulang | dalam satu tx migrasi; baris lama sudah ⊆ set baru |
| Escrow nol di depan hilang di Excel | sel teks + `CSVSafe`; catat di tests sebagai cek manual (user) |
| Inquiry lambat bila banyak cabang | satu query, subquery per baris dibatasi page ≤ 100; indeks `vendor_vaults_vendor_branch_idx` sudah ada; PIC per cabang memakai indeks yang ada (dicek `EXPLAIN` di dev) |
| Dua header CSV (impor vs ekspor) tidak sinkron | `masterdata_import_export_header_test.go` |

## Bukti (→ `tests.md`)
- Unit: `normalizeCisLimit` (kosong, 0, negatif, 3 desimal, terlalu besar, valid), validasi escrow (11/12/13 digit,
  huruf, spasi), `bm` vendor-wide ditolak, `picRoleCodes`.
- Integration (`//go:build integration`): apply create/update limit + escrow; payload tanpa kunci → nilai tetap; CSV
  header lama → nilai tetap; CHECK DB menolak limit ≤ 0 / escrow salah; inquiry (filter vendor, agregasi BM/escrow,
  ATM_CASH di dua kolom, vendor INTERNAL tidak muncul).
- Handler: 403 untuk role selain ADMIN/ADMIN_PARAM, 202 submit, 422 validasi.
- `cd backend && go test ./...`, `golangci-lint run --build-tags integration ./...`; `backend-exq` + `pkg` `go test`;
  `pnpm --dir frontend/CompanyPortal-Vite run test|lint|build`.
- Cek browser manual: **outstanding (user)**.

## Alternatif yang tidak diambil
- Tabel `vendor_branch_cis_limits` berhistori: belum diminta; audit maker-checker sudah mencatat before/after.
- Kolom `bm_name/bm_phone` di `vendor_branches`: ditolak di intent #2.
- Menu + `menu_features` baru untuk inquiry: ditolak S7.
- Mengubah update cabang/vault jadi partial (PATCH): perubahan kontrak lebih besar; kunci-ada hanya untuk dua kolom baru.
