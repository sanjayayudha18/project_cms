# Backend QC Result

**Tanggal**: 2026-10-09 · **Branch**: `dev1` (commit `42e7b08`) · **Toolchain**: Go 1.26.3, golangci-lint 2.13.0
**Scope**: Go modules `backend/`, `backend-cit/`, `pkg/` (Python `backend_python/` tidak termasuk QC ini).
**Ukuran**: ±28.6k LOC non-test (tanpa `internal/db` hasil sqlc), ±33.8k LOC test.

## Ringkasan

| Cek | Hasil |
| --- | --- |
| `go build ./...` (3 modul) | ✅ Bersih |
| `go vet ./...` (3 modul) | ✅ Bersih |
| Unit test (tanpa DB) | ✅ Semua lulus |
| Integration test (`-tags integration`, DB dev localhost) | ✅ Semua lulus |
| `gofmt -l` | ⚠️ 12 file belum terformat |
| golangci-lint (default: errcheck + staticcheck) | ⚠️ 37 isu, hampir semua minor/di file test |
| gosec | ⚠️ 11 temuan, 1 false positive, tidak ada yang kritis |
| Coverage ≥ 80% per `internal/*` (CLAUDE.md Sec 8) | ❌ 3 package di bawah target |
| Aturan uang = numeric, bukan float (Sec 6) | ❌ Ada `float64` di response read-only |
| SQL injection (string concat / Sprintf query) | ✅ Tidak ditemukan, semua lewat sqlc |
| Secret hardcoded / secret di log | ✅ Tidak ditemukan |
| Kebocoran error internal ke client | ✅ Aman: hanya error validasi/sentinel; error lain lewat `writeUnexpectedError` |
| Pagination dibatasi | ✅ mis. `PageSize` 1–100 |
| Upload body dibatasi + sanitasi nama file | ✅ `MaxBytesReader` + `filepath.Base(Clean)` |
| Replica routing (Sec 6) | ✅ `dbReadPool` sudah di-wire di `main.go` (lihat M3 soal sisa repo) |

**Kesimpulan**: kode sehat dan siap dilanjutkan. Tidak ada isu **CRITICAL**. Ada 2 isu **HIGH** (gap coverage, float untuk uang) yang sebaiknya dijadwalkan; sisanya MEDIUM/LOW.

---

## Coverage (integration test aktif)

| Package | Coverage | Target 80% |
| --- | --- | --- |
| `backend/internal/auth` | 88.0% | ✅ |
| `backend/internal/audit` | 86.7% | ✅ |
| `backend/internal/rolemgmt` | 85.4% | ✅ |
| `backend/internal/notification` | 83.8% | ✅ |
| `backend/internal/approval` | 81.3% | ✅ |
| `backend/internal/service` | 72.5% | ❌ |
| `backend/internal/handler` | 68.0% | ❌ |
| `backend/internal/repository` | 33.5% | ❌ |
| `pkg/config` | 100% | ✅ |
| `pkg/middleware` | 85.9% | ✅ |
| `pkg/auth` | 83.9% | ✅ |
| `pkg/response` | 0% | ❌ (kecil, dipakai `backend-cit` nanti) |
| `backend-cit/internal/*` | tidak ada test | — (scaffold, rencana rename ke `backend-exq`) |

Catatan: tanpa `-tags integration`, coverage `service` turun ke 37.8% dan `rolemgmt` ke 35.8%. CI **wajib** menjalankan tag `integration` dengan DB, kalau tidak, regresi tidak akan terdeteksi.

---

## Temuan

### HIGH

**H1. Coverage di bawah 80% pada `service`, `handler`, `repository`**
- `repository` 33.5% paling rendah. Banyak repo admin read-only tanpa integration test langsung.
- Prioritaskan path Sec 4 #7 (money/recon/maker-checker): `vendor_request_actions.go`, `vault_plan_actions.go`, `vendor_party.go`, applier master-data.

**H2. Nilai uang dibawa sebagai `float64` (melanggar CLAUDE.md Sec 6)**
- `internal/service/atm_portal.go:131-137` + `numericToFloat64Ptr` (`:327`): threshold, refund, replenish, escrow.
- `internal/service/dsr_upload.go:338-357`, `internal/handler/dsr_upload_handler.go:326-344`: denominasi & total IDR DSR.
- `internal/handler/atm_portal_handler.go:295-301`.
- Hanya untuk tampilan (read-only, tidak ada perhitungan atau tulis ke DB). Nilai IDR bulat aman sampai 2^53, jadi risiko nyata rendah. Tetapi ini tetap pelanggaran aturan dan akan menular kalau nanti dipakai untuk menghitung. **Fix**: kirim sebagai string desimal (pola yang sudah dipakai di `atm_admin.go:454`) dan sesuaikan frontend.

### MEDIUM

**M1. Fungsi dengan kompleksitas tinggi (gocyclo > 20)**
| Fungsi | Lokasi | Cyclo |
| --- | --- | --- |
| `importEnv.validateFields` | `internal/service/masterdata_import.go:669` | 47 |
| `importEnv.validateRow` | `internal/service/masterdata_import.go:490` | 40 |
| `UserAdminService.Create` / `Update` | `internal/auth/user_admin.go:154`, `:294` | 34 / 34 |
| `VaultPlanService.ReviewRequest` | `internal/service/vault_plan_actions.go:283` | 25 |
| `syncVendorParties` | `internal/service/vendor_party.go:132` | 24 |
| `VendorRequestService.UpdateItems` / `transition` | `internal/service/vendor_request_actions.go:734`, `:958` | 23 / 23 |
| `readImportCSV`, `buildEnv`, `ATMAssignmentApplier.Apply` | `masterdata_import.go:127`, `:338`, `masterdata_applier_atm_assignment.go:88` | 21–22 |

`masterdata_import.go` akan ditulis ulang untuk flow FSD replace-all (Sec 12), jadi refactor di sana sebaiknya sekalian saat itu. Jangan refactor terpisah.

**M2. File > 800 baris**: `internal/service/vendor_request_actions.go` (1210 baris). Kandidat dipecah per state transition (create/edit, approve/reject, completion), sebaiknya saat fitur berikutnya menyentuh file ini.

**M3. Komentar TODO replica basi + sebagian repo admin belum pakai replica**
- `NewUserAdminRepository`, `NewVendorVaultAdminRepository`, `NewVendorPicAdminRepository`, `NewVendorPackageAdminRepository`, `NewVendorPackagePriceAdminRepository`, `NewVendorBranchAdminRepository`, dan ATM assignment repo masih dibuat hanya dengan `dbPool` (`cmd/api/main.go:152, 309, 321, 328, 338, 348`). Komentarnya bilang "TODO saat `DATABASE_REPLICA_URL` di-wire", padahal wiring sudah ada (`main.go:60-80`) dan sudah dipakai `vendor`, `atm`, `region`, `dsr_location_map`.
- **Fix**: tambah parameter `dbRead` dan arahkan list/get ke replica (read-after-write tetap ke primary), lalu hapus TODO.

**M4. Duplikasi handler admin master-data (dupl: 24 blok)**
- Blok parse/submit/202 hampir identik di `admin_atm_assignment_handler.go`, `admin_vendor_*_handler.go`, `admin_atm_handler.go`; juga `auth_repository.go:39/72/106`, `atm_portal_cashpos.go` ↔ `dmaa_forecast.go:32-61`, `atm_portal_handler.go:420` ↔ `:567`.
- Wajar untuk pola "setiap entity punya handler sendiri". Ekstrak helper hanya kalau ada perubahan perilaku yang harus diterapkan ke semua handler sekaligus.

**M5. `main()` 329 baris** (`backend/cmd/api/main.go:32`). Semua wiring ada di satu fungsi. Masih mudah dibaca karena linear; pecah per domain (`mountAdminRoutes`, `mountVendorRoutes`, …) kalau terus bertambah.

**M6. Permission file upload DSR terlalu longgar** (`internal/service/dsr_upload.go:191,195`): `MkdirAll 0o755`, `WriteFile 0o644`. File vendor berisi data cash. Turunkan ke `0o750` / `0o640`. Pastikan user container Python ETL masih bisa membaca (grup sama).

### LOW

**L1. gofmt**: 12 file belum terformat (sebagian besar format list di doc comment):
`backend/cmd/hashpw/main.go`, `internal/auth/service.go`, `internal/auth/service_test.go`, `internal/handler/admin_region_handler.go`, `internal/repository/auth_repository.go`, `internal/rolemgmt/repository.go`, `internal/service/atm_visit_quota_integration_test.go`, `internal/service/vendor_request_actions.go`, `internal/service/vendor_request_ticket.go`, `backend/tools.go`, `pkg/auth/repository.go`, `pkg/auth/token_service_test.go`. **Fix**: `gofmt -w` (satu commit terpisah).

**L2. errcheck di kode produksi**:
- `internal/handler/admin_master_data_import_handler.go:106` `defer file.Close()`
- `internal/notification/smtp.go:51,56,59` `conn.Close()` / `c.Close()`
- `pkg/middleware/rbac.go:113`, `pkg/response/response.go:42` `json.Encoder.Encode`
- `backend-cit/cmd/api/main.go:51,70`

Semuanya pada jalur close/write response, jadi dampaknya kecil. Sisanya (±25) ada di file test.

**L3. gosec G115 (int → int32)**: `atm_portal.go:213-214` aman karena PageSize ≤ 100 sudah divalidasi. `masterdata_import_batch_repository.go:92` dan `masterdata_applier_vendor_package*.go:45,57,142` (int64 → int32) perlu dicek bahwa nilai sumbernya dibatasi. Kalau ya, beri komentar; kalau tidak, tambahkan guard.

**L4. staticcheck ST1005** `pkg/auth/errors.go:10-21`: pesan error diawali huruf kapital / diakhiri titik. Disengaja karena pesan ditampilkan langsung ke user. Abaikan atau tambahkan `//nolint:staticcheck` dengan alasan.

**L5. Hard `DELETE FROM` di queries**: `vendor_request_items`, `vendor_request_vault_assignments`, `acm_area_branches`, `acm_area_members` (replace-set child/link rows), `notifications`/`notification_emails` (retensi 90 hari, deviasi terdokumentasi). Tidak ada yang menyentuh master data yang wajib soft-delete (`no_hard_delete_test.go` lulus). Pastikan replace-set child rows selalu dalam tx yang menulis `audit_logs` dengan before/after.

**L6. Tooling**: `govulncheck` belum terpasang, jadi scan CVE dependensi belum dilakukan. Belum ada `.golangci.yml` di repo, sehingga linter berjalan dengan default yang berbeda per developer.

---

## False positive / sudah OK (tidak perlu tindakan)
- gosec G120 `dsr_upload_handler.go:93`: body sudah dibatasi `http.MaxBytesReader` di baris 92.
- `panic(http.ErrAbortHandler)` di `admin_master_data_export_handler.go:125`: idiom stdlib untuk membatalkan streaming response.
- 151 pemanggilan `writeError(..., err.Error())`: semuanya error parsing/validasi/sentinel 4xx. Tidak ada `err.Error()` pada respons 5xx.

---

## Rekomendasi urutan tindakan
1. **Sekarang (murah)**: `gofmt -w` (L1), tambah `.golangci.yml` + pasang `govulncheck` (L6), pastikan CI menjalankan `go test -tags integration` dengan DB.
2. **Sebelum fitur money/recon berikutnya**: H2 (float → string desimal), M6 (permission file), M3 (replica untuk repo admin).
3. **Bertahap, ikut fitur yang menyentuh file**: H1 (naikkan coverage `repository`/`handler`/`service`), M1/M2 (refactor fungsi kompleks / pecah `vendor_request_actions.go`).

## Belum dicek / di luar scope
- Python `backend_python/` (ETL + FastAPI).
- Frontend.
- Review logika bisnis per-fitur terhadap spec `.claude/sdlc/*` (QC ini fokus ke kualitas kode & aturan lintas-modul).
- Manual browser verification (tugas user, Golden Rule #10).

## Cara reproduksi
```bash
cd backend && go build ./... && go vet ./... && gofmt -l .
cd backend && DATABASE_URL=postgres://…@localhost:5432/cms go test -tags integration -count=1 -p 1 -cover ./internal/...
cd backend && golangci-lint run --build-tags integration ./...
cd backend && golangci-lint run --default=none --enable gosec --tests=false ./...
```
