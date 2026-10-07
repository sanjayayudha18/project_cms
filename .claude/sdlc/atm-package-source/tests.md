# Tests: Pilihan sumber paket pada Kelolaan ATM

Status: build + automated tests **green** (2026-10-07). Stage: 4 Test.
Input: `plan.md` (diterima 2026-10-07). Trigger ke stage berikut: `review.md` → code owner merge.
**Manual browser check: OUTSTANDING — dilakukan user** (Golden Rule #10). Tidak ditandai selesai di sini.
Prasyarat uji browser: backend `cd backend && go run ./cmd/api`, frontend `pnpm --dir frontend/CompanyPortal-Vite run dev`; login ADMIN/ADMIN_PARAM; buka Pengaturan → Manajemen ATM → ATM → Kelolaan. Perlu satu vendor FLM dengan tarif aktif di `vendor_package_prices` yang cocok dengan jenis mesin/kelas ATM (mis. PAKET 4 untuk ATM REGULAR).

## Perintah & hasil (2026-10-07)
| Perintah | Hasil |
|---|---|
| `psql … -f backend/migrations/023_atm_assignment_vendor_source.sql` (dev, localhost) | OK (disetujui user); 1163 baris lama tetap `vendor_package_id` terisi, kolom baru nullable |
| `cd backend && sqlc generate` (v1.31.1) + fix `UserLeafe`→`UserLeave` | OK |
| `cd backend && go build ./... && go vet ./...` | OK |
| `cd backend && go test -count=1 ./...` | OK |
| `cd backend && DATABASE_URL=<dev, localhost> go test -count=1 -tags integration ./internal/...` | OK (seluruh paket, termasuk test integrasi lama: regresi mode `branch`) |
| `go test -tags integration ./internal/service -run TestIntegration_VendorWideAssignment -v` | PASS; semua data di transaksi yang di-rollback |
| `pnpm --dir frontend/CompanyPortal-Vite run test` | 122 file, 1045 test lulus |
| `pnpm --dir frontend/CompanyPortal-Vite run lint` (biome, seluruh proyek) | bersih (6 file diformat dengan `biome check --write`) |
| `pnpm --dir frontend/CompanyPortal-Vite run build` | OK (warning chunk >500 kB sudah ada sebelumnya) |

## Traceability test → spec
| Spec | Test |
|---|---|
| FR3 bentuk payload per mode; payload lama tanpa `source` = `branch` | `service/atm_assignment_source_test.go` `TestNormalizeSource`; handler `…_Create_VendorSourcePayloadReachesService` |
| FR4 periode otomatis untuk kedua mode, tanpa cek overlap saat submit | `TestATMAssignmentAdminService_VendorSource_Create`; handover otomatis dicakup test integrasi lama `masterdata_approval_integration_test` |
| FR5 RBAC/maker-checker/202 | handler tests (`…_WrongRole_Forbidden`, `…_PackageOptions/non-admin`, `…_Create_Returns202`) |
| FR6 list/get mengembalikan `source`, label, cabang | handler `…_List_ExposesSource`; integrasi `TestIntegration_VendorWideAssignment` (list/get) |
| FR7 semua pembaca menerima dua sumber | integrasi `TestIntegration_VendorWideAssignment`: ATM list, `ResolveAtmQuota`, `ListBranchATMs`/`CountBranchATMs`, `ListForecastForDate`, export |
| FR8 Update tidak boleh pindah mode; label boleh diganti dalam mode | `…_Update_RejectsModeSwitch`, `…_Update_VendorLabelWithinMode` |
| FR9 validasi tarif/vendor/cabang saat submit **dan** apply | `…_VendorSource_Validate` (4 kasus), `TestValidateSourceAtApply`, handler `…_HandleError_MasterDataApplyConflicts` (409); integrasi `VendorTariffExistsForATM`, `CheckAssignmentVendorBranch` |
| FR10 kuota memakai label dari kedua sumber | integrasi `ResolveAtmQuota` + `packageLabel`; test kuota lama tetap hijau |
| FR11 endpoint `assignment-package-options` | `…_PackageOptions` (service + handler: 200, 400 tanpa vendor, 404, 403); integrasi `ListATMPackageOptions`; frontend `api.test.ts` |
| FR12 CSV dua mode, file lama tetap valid, `package_source` tak bisa diubah | `TestImport_AssignmentTwoModes`, `…_AssignmentSourceIsImmutable`, `…_AssignmentLegacyHeaderStillAccepted`, `TestAssignmentFields_BySource`, `TestExport_HeaderContract`; integrasi export row |
| NFR1 baris lama valid tanpa backfill | migrasi diterapkan ke dev (1163 baris), seluruh test integrasi lama hijau |
| NFR2 integritas di DB | integrasi: `avp_source_chk` (23514: kedua mode & tidak ada), overlap lintas mode (23P01) |
| NFR4 regresi nol mode `branch` | seluruh test integrasi lama (`forecast_query`, `atm_visit_quota`, `vendor_request_*`, `branch_atm`, `masterdata_approval`) tanpa diubah, hijau |
| FR1/FR2/NFR5 UI | `ATMAssignmentsDialog.test.tsx` (cabang wajib, tanpa "Semua cabang", mode seluruh vendor, payload), `ATMAssignmentsDialog.currentAssignment.test.ts` (`isSameAsCurrent`) |

## Outstanding / tidak diuji otomatis
- **Verifikasi browser (user)**: buka dialog Kelolaan; (1) pilih vendor → cabang → "Paket khusus cabang" → ajukan; (2) ganti ke "Paket seluruh vendor" → dropdown berisi label bertarif untuk mesin ATM itu → ajukan; (3) setujui kedua permintaan di Approval, cek tabel periode (kolom Jenis) dan Profil ATM; (4) ekspor CSV kelolaan lalu impor ulang tanpa perubahan (harus no-op).
- Apply-time 409 untuk mode `vendor` diuji pada level unit (`validateSourceAtApply`) dan pemetaan HTTP; skenario "tarif dihapus di antara submit dan approve" belum diuji end-to-end di browser.
- Tidak ada validasi baru untuk mode `branch` (gap diterima 2026-09-23).
