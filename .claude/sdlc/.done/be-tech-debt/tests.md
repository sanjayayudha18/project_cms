# Tests: Perbaikan tech debt backend (BE-01 – BE-12)

Status: **partial — Fase A (A1, A2, A3) green 2026-10-15** (Stage 4). Input: `spec.md` (FR-A1–A3), `plan.md` Fase A (accepted 2026-10-15). Committed `fb3b6fd` (A1, A2) + `9a5af6d` (A3). **Fase B (B1–B4, B6) green lokal 2026-10-15**, committed `8b3a32b` (FR-B6 test) + `077ce05` (ci.yml) + docs; B5 ditunda. Fase C–G belum dikerjakan (plan masih draft untuk fase itu).

## Commands (run 2026-10-15)
| Command | Result |
|---|---|
| alpinelinux.org/releases | stable terbaru v3.24 (3.24.2, support s.d. 2028-06-01); 3.20 EOL 2026-04-01 |
| `FROM alpine:3.20` → `alpine:3.24` di `backend/Dockerfile` + `backend-cit/Dockerfile` | — |
| `docker build -f backend/Dockerfile .` / `-f backend-cit/Dockerfile .` | sukses keduanya; `/etc/alpine-release` = 3.24.2 |
| `docker run --env-file backend/.env` (+ Redis sementara dengan alias `cms-redis`, DB dev via `host.docker.internal`) | `docker inspect` → `running healthy`; `id -un` = `appuser`; `GET /health` 200 (T-A1) |
| `cd backend && go get golang.org/x/crypto@v0.56.0 && go mod tidy` | x/crypto v0.54.0 → v0.56.0; `go` directive 1.25.0 → 1.26.0 (dipaksa x/crypto v0.56.0); `go get` ikut menaikkan `go.work` 1.25.0 → 1.26.0 (workspace harus ≥ module); toolchain tetap go1.26.9 = builder `golang:1.26.9`. `backend-cit`/`pkg` tidak me-require x/crypto → tidak berubah |
| `cd backend && go build ./... && go vet ./... && go test ./...` | clean / all ok |
| `cd backend && golangci-lint run --build-tags integration ./...` | 0 issues |
| `go test ./...` di `backend-cit`, `pkg` | all ok |
| `govulncheck ./...` ×3 (T-A2) | backend: 0 terjangkau; sisa 1 di module = **GO-2026-5932** (`x/crypto/openpgp`, tanpa fix, tidak di-import → *accepted*); backend-cit + pkg: no vulnerabilities |
| `git rm -r backend_python/dmaa/bak` (2 file, tidak di-import siapa pun) | — |
| `cd backend_python && py -3 -m unittest discover` | 77 tests OK (55 skip integration tanpa `DATABASE_URL`) |

## Test → requirement
| Test | Covers |
|---|---|
| T-A1 docker build + health `healthy` + non-root | FR-A1 base image didukung, HEALTHCHECK + `USER appuser` tetap (backend) |
| T-A2 govulncheck ×3 + `go test` ×3 | FR-A2 x/crypto ≥ v0.56.0, tanpa regresi |
| unittest Python | FR-A3 `dmaa/bak` dihapus tanpa memutus import |

## Catatan
- `backend-cit/Dockerfile` hanya diganti base image sesuai A1. Image itu **belum** non-root dan **tanpa** HEALTHCHECK (sudah begitu sebelumnya; Sec 9). Di luar Fase A → follow-up (atau sekalian saat rename ke `backend-exq`).

## Outstanding
- Tidak ada manual browser check untuk Fase A.
- T-A1/T-A2 sebagai job CI menunggu fase B.

## Fase B — CI (run lokal 2026-10-15; run GitHub menunggu push user)
| Command | Result |
|---|---|
| `SHOW server_version` dev (lewat subprocess, URL tidak dicetak) | 18.3 → service `postgres:18` |
| B1 per module `backend`, `backend-cit`, `pkg`: `go vet ./...` · `golangci-lint run --build-tags integration ./...` (v2.13.0) · `go run golang.org/x/vuln/cmd/govulncheck@v1.8.0 ./...` | ok · 0 issues · no vulnerabilities (×3) |
| B1 `go test -race ./...` ×3 (container `golang:1.26.9`; Windows tanpa gcc) | exit 0 ×3 |
| B2 `CREATE DATABASE cms_ci` (izin user) → `PG*` env → `bash scripts/apply-migrations.sh` | semua migrasi `001`–`031` sukses, 66 tabel |
| B2 `go test -tags integration -race -count=1 -v ./...` ke `cms_ci` (run 1) | exit 0, 0 skip `DATABASE_URL`, tetapi **2 SKIP data dev** → FR-B6 |
| FR-B6: `TestATMAssignmentApplier_AutoPeriod_HandsOverRunningPeriod_Integration` (cari ATM tanpa kelolaan + 2 paket dari data dev) dan `TestIntegration_ApplyVaultTickets` (cari 2 vendor_request dari data dev) → seed data sendiri di tx (helper `seedForecast*` yang sudah ada / `INSERT vendor_requests`) | lulus di `cms_ci` **dan** di DB dev |
| B2 run 2 ke `cms_ci` | exit 0, **0 SKIP**, 1727 PASS (termasuk subtest) |
| B2 `DROP DATABASE cms_ci` (izin user) | dihapus |
| B3 `sqlc generate` (v1.31.1) → `sed UserLeafe→UserLeave` → `git diff --exit-code -- internal/db` | diff kosong |
| B4 `py -3 -m unittest discover` (Python 3.14.7) dengan `DATABASE_URL` → `cms_ci` | 77 tests OK, **0 skipped** (dsr 11, itm/cashpos 3, replenish_classify 12, lib import_jobs+eod_api 34 termasuk) |
| `ci.yml` di-parse PyYAML | jobs `go`, `go-integration`, `sqlc`, `python`; `permissions: contents: read` |

### Test → requirement (Fase B)
| Test | Covers |
|---|---|
| job `go` (matrix 3 module) | FR-B1 |
| job `go-integration` + gagal bila ada `--- SKIP` | FR-B2, FR-B6 (lebih ketat dari plan: semua skip, bukan hanya `DATABASE_URL`) |
| job `sqlc` | FR-B3 |
| job `python` (`discover` = superset dsr/itm/lib + `replenish_classify` yang ditambah 2.2c) + gagal bila `skipped=` | FR-B4 |
| `docs/deployment.md` #8 + Pipeline | FR-B7 (B6) |

### Deviasi dari plan (Fase B)
- B4 memakai `python -m unittest discover` (77 test) alih-alih 3 perintah terpisah (60 test): superset, ikut `replenish_classify` (2.2c).
- Job integration/python gagal pada skip apa pun (plan hanya mengecek `DATABASE_URL not set`).
- Action major terbaru: `checkout@v7`, `setup-go@v7`, `setup-python@v7`, `golangci-lint-action@v9` (linter tetap `v2.13.0`).

### Outstanding (Fase B)
- **T-B1/T-B2/T-B4 di GitHub**: run hijau setelah user push.
- **T-B3 negatif**: ubah satu `queries/*.sql` tanpa generate di branch uji → job `sqlc` harus gagal (belum dijalankan; dilakukan setelah workflow aktif di GitHub).
- **B5 (job `e2e`) ditunda**: FE E3 (`frontend/CompanyPortal-Vite/e2e/`) belum ada. Secrets `E2E_*` belum diperlukan.
- Branch protection (wajib lulus CI) di GitHub = user.

## Fase C — Coverage, slice C1 (run 2026-10-15)
| Command | Result |
|---|---|
| `go test -tags integration -count=1 -p 1 -cover ./internal/...` ke DB dev | lihat tabel baseline |
| Ulang cara CI: `CREATE DATABASE cms_ci` → `apply-migrations.sh` → container `golang:1.26.9` `go test -tags integration -race -count=1 -cover -v ./...` → `DROP DATABASE cms_ci` | exit 0, 0 SKIP, angka **sama persis** dengan run di dev |
| `scripts/check-coverage.sh <log>` (ambang asli) | 8 paket ok, exit 0 |
| `scripts/check-coverage.sh <log> <ambang uji: service 99 + paket fiktif>` | `FAIL … service 81.6% < 99%` + `MISSING … nosuch`, exit 1 |

### Baseline coverage (unit + integration) — FR-C1
| Paket `internal/*` | 2026-10-12 (unit saja) | 2026-10-15 | Ambang |
|---|---|---|---|
| `service` | 44,6% | 81,6% | 81 |
| `approval` | 54,2% | 81,3% | 81 |
| `rolemgmt` | 35,8% | 85,4% | 85 |
| `repository` | 15,4% | 86,9% | 86 |
| `handler` | — | 80,7% | 80 |
| `auth` | — | 90,1% | 90 |
| `audit` | — | 86,7% | 86 |
| `notification` | — | 83,9% | 83 |
| `db` | — | 0% (sqlc, tanpa test) | — |

- **S5 tidak terpicu**: `service` 81,6% ≥ 65%, tidak perlu AI-DLC `be-coverage-service`.
- Target FR-C3 (≥ 80% `service`, `approval`, `rolemgmt`) **sudah tercapai** secara angka. C2–C4 tetap ada untuk test prioritas (maker ≠ checker, RBAC service, money math, applier stale/409, audit-in-tx rollback) dan menaikkan ambang.
- CI: `go-integration` kini `-cover` + step `bash scripts/check-coverage.sh test.log`. Committed `ddefb12`; bukti GitHub menunggu push.

## Fase C — slice C2 `rolemgmt` (run 2026-10-15, committed `8013dd6`)
Prioritas plan sudah punya test integration sebelum slice ini: grant/revoke full-set (`TestUpdateRolePermissions_FullSetReplace`), audit di tx yang sama + rollback saat audit gagal (`TestCreateRole_AuditFailureRollsBack`, `TestUpdateRolePermissions_AuditFailureRollsBack`), re-check `APPACCESS`/`ADMIN` (`*_AuthorizationMatrix`). C2 menutup sisa celah (jalur error + race nama):

| Test baru | File | Covers |
|---|---|---|
| `TestPermissionService_DependencyErrorsArePropagated` (6 kasus) | `service_test.go` (unit, fake `Repo`/`Pool`) | error repo/`Begin` dibungkus (`errors.Is`), tidak ada tx dibuka bila pre-check gagal |
| `TestPermissionService_UnauthorizedActorNeverTouchesRepoOrTx` | `service_test.go` | re-check peran di service terjadi sebelum repo/tx disentuh (`""`, `ATM-USER`, `VENDOR-USER`, `ADMINX`) |
| `TestIsUniqueViolation` (5 kasus) | `service_test.go` | deteksi `23505`, termasuk error terbungkus |
| `TestCreateRole_ConcurrentDuplicateCaughtByUniqueConstraint` | `service_integration_test.go` | pre-check nama terlewat (create bersamaan) → constraint unik DB → `ErrRoleNameConflict` (409), tetap 1 baris |

| Command | Result |
|---|---|
| `go test ./internal/rolemgmt/` (unit) | ok |
| `go test -tags integration -cover ./internal/rolemgmt/` (DB dev) | ok, 0 SKIP, **85,4% → 93,5%** |
| `go test -race ./internal/rolemgmt/` (container) | ok |
| `golangci-lint run --build-tags integration ./internal/rolemgmt/...` | 0 issues |
| `coverage-thresholds.txt` `rolemgmt` | 85 → **93** |

Sisa tidak tercakup (≈6,5%): error `db.CreateRole` non-unik, `tx.Commit` gagal, error dari `ReplaceRolePermissions`/`distinctInt64s` repo, dan cabang 500 middleware — butuh fake `pgx.Tx` penuh; tidak sebanding.

## Fase C — slice C3 `approval` (run 2026-10-15, committed `993540b`)
Sebelum slice: happy path naik L2→L3, maker approve (hanya cek `err != nil`), reject oleh orang asing, double submit, error submit. C3 menambah `internal/approval/orchestrator_rules_test.go` (unit, fake yang sudah ada + 5 hook injeksi error di `fakeStore`):

| Test | Covers (prioritas plan) |
|---|---|
| `TestOrchestrator_MakerCannotActOnOwnRequest` (4 subtest) | maker ≠ checker untuk approve **dan** reject, termasuk approver cuti yang mendelegasikan ke maker; step + request tetap pending, tidak ada audit |
| `TestOrchestrator_OnlyCurrentStepApproverMayAct` | actor = approver efektif step **saat ini**: L3 tidak bisa lompat saat L2 pending (approve/reject → `ErrNotAuthorized`); approver L2 tidak bisa bertindak di step L3 |
| `TestOrchestrator_DelegateActsWhileApproverOnLeave` | delegasi/cuti: approver yang cuti ditolak, delegate menyetujui; `acted_by` = delegate, `acted_at` = jam orkestrator, `assigned_approver` tetap; audit actor = delegate |
| `TestOrchestrator_ApproverOnLeaveWithoutDelegateBlocksStep` | cuti tanpa delegate → semua aktor ditolak, step tetap pending |
| `TestOrchestrator_PolicyLevelDecidesChainLength` | policy berjenjang: level 2 → satu step, approve langsung `approved` + `final_approve` |
| `TestOrchestrator_RejectAtLastLevelAfterEarlierApproval` | reject di L3 setelah L2 approve → `rejected`; reject kedua → `ErrRequestNotPending`; urutan audit |
| `TestOrchestrator_ApproveRejectErrorBranches` (9) + `TestOrchestrator_FinalApproveAuditFailureIsReturned` | error availability/update step/audit/list steps/update request dibungkus dan dikembalikan |

| Command | Result |
|---|---|
| `go test ./internal/approval/` (unit) | ok |
| `go test -tags integration -cover ./internal/approval/` (DB dev) | ok, 0 SKIP, **81,3% → 88,6%** (`Approve`/`Reject` 100%) |
| `go test -race` (container) · `golangci-lint` | ok · 0 issues |
| `coverage-thresholds.txt` `approval` | 81 → **88** |

Sisa tidak tercakup: cabang error DB di `repository.go` (not-found/scan error) — dicakup sebagian oleh integration test yang ada.

### Temuan produk — **DIPERBAIKI `448e870`** (2026-10-15, bugfix: (a) → 403 `ErrNotAuthorized`, (b) → 409 `ErrApproverOnLeaveNoDelegate`; test C3 dikencangkan ke sentinel, lihat `.claude/bugfixes.md`)
`ResolveEffectiveApprover` (`internal/approval/effective_approver.go`) mengembalikan error **polos** (tidak dibungkus `ErrNotAuthorized`) untuk dua kasus: (a) approver efektif = maker (mis. approver cuti mendelegasikan ke maker), (b) approver cuti tanpa delegate. `ApprovalHandler.handleError` (`internal/handler/approval_handler.go`) jatuh ke `default` → **HTTP 500 "Terjadi kesalahan internal"**. Persetujuan tetap **ditolak** (fail-closed, tidak ada celah keamanan), tetapi user mendapat 500, bukan 403/409 yang bisa ditindaklanjuti. Test C3 sengaja hanya mengecek "ditolak + tidak berubah", tidak mengunci kode 500.

## Fase C — slice C4.1 `service` / `masterdata_approval` (run 2026-10-15, committed `82ae252`)
Sudah ada sebelumnya: stale → `ErrMasterDataChangeStale` + entitas tidak berubah, unchanged → applied, applier hilang → approved-not-applied, batch satu baris gagal → seluruh batch rollback, reject batch, RetryApply (+ RBAC, kedua kali `NotRetryable`); handler stale → 409 (`admin_atm_assignment_handler_test.go`).

| Test baru | File | Covers |
|---|---|---|
| `TestMasterDataApprovalService_StaleChange_NotRetryable_NoAppliedAudit_Integration` | `masterdata_approval_rules_integration_test.go` | stale bersifat final: `RetryApply` → `ErrMasterDataNotRetryable`, edit bersamaan tetap, 0 audit `master_data_applied` |
| `TestMasterDataApprovalService_EntityGoneAtApply_ApprovedNotApplied_Integration` | idem | entitas hilang saat apply → error bukan stale, status `approved` + `error` tercatat (bisa di-retry), tidak ada tulisan |
| `TestMasterDataApprovalService_OrchestratorRefusal_LeavesChangePending_Integration` | idem | penolakan rantai approval (`ErrNotAuthorized`, maker ≠ checker) diteruskan; change tetap `pending`, tidak ada apply/reject |
| `TestStatesEqual` (7) | `masterdata_states_equal_test.go` (unit) | urutan field tidak berpengaruh; nilai/flag berubah → beda; field hilang di snapshot → beda; snapshot rusak / current tak bisa di-marshal → error |

| Command | Result |
|---|---|
| `go test -tags integration -cover ./internal/service/` (DB dev) | ok, 0 SKIP; paket 81,6% → 81,7%; `Approve` 100%, `applyOne` 75% → 81,2%, `statesEqual` 66,7% → 88,9% |
| `golangci-lint` · `go test -race` (unit, container) | 0 issues · ok |
| `coverage-thresholds.txt` `service` | tetap 81 (paket besar; kenaikan per sub-slice C4.2–C4.4) |

## Fase C — slice C4.2 `service` / vendor_request + completion + kuota (run 2026-10-15, committed `f9f66e8`)
Sudah ada sebelumnya: state machine + `checkActor` (unit + property), four-eyes approve/reject, reject → rejected + 1 audit, cancel (union rule, checker-only setelah approved, idempotent), UpdateItems guard, kuota (decrement, over quota tetap approve + notifikasi, reject + resubmit, reset/cancel, rollback, paralel, terminal tak dikenal), completion transitions/actor (unit).

`internal/service/vendor_request_rules_integration_test.go`:

| Test | Covers |
|---|---|
| `TestIntegration_VendorRequestTransition_Guards` (5) | `ErrNotFound` untuk submit/approve/reject; **state guard sebelum actor guard** (approve draft oleh creator → `ErrInvalidTransition`, bukan `ErrSelfApproval`; 0 audit); hanya creator yang submit (`ErrNotCreator`); submit tanpa item → `ErrEmptyItems`; role non-checker approve → `ErrNotChecker` (RBAC di service); status tidak berubah |
| `TestIntegration_Completion_Guards` (4) | request tak ada → `ErrNotFound`; request dibatalkan tidak bisa dilaporkan; `VENDOR-USER` di jalur internal → `ErrNotAuthorized`; alasan reject > 500 → `ValidationError` |
| `TestIntegration_VisitQuota_ServiceRBACAndEdges` (4) | **RBAC di service**: `ATM-USER`/`VENDOR-USER`/kosong tidak bisa `ResetATM`/`ResetVendor`/`CancelVisit` (`ErrNotChecker`); reset ATM tak dikenal → `ErrAtmNotFound`; cancel visit tak ada / alasan > 500; ATM tanpa kuota diketahui → kunjungan tercatat `quota_known=false`, tidak ada baris kuota, bisa dibatalkan tanpa menyentuh counter |

| Command | Result |
|---|---|
| `go test -tags integration -cover ./internal/service/` (DB dev) | ok, 0 SKIP; paket 81,7% → **82,0%**; `transition` 72,7 → 81,8%, `runCompletionTx` 77,4 → 87,1%, `ResetATM` 76,9 → 92,3%, `CancelVisit` 74,1 → 85,2% |
| `golangci-lint` | 0 issues |
| `coverage-thresholds.txt` `service` | 81 → **82** |

## Fase C — slice C4.3 `service` / vault plan + vendor order (run 2026-10-15, committed `fb92794`)
Sudah ada sebelumnya: validasi alokasi (duplikat, vault/denom tak dikenal, urgent/buffer reason), submit kosong, reject draft, maker reject/approve sendiri, reviewer = pembuat/penyetuju plan, non-member Get, ADMIN save, alur penuh + tiket; vendor order scope/no-leak (branch/vendor lain → 404, internal user / klaim basi → 403), alasan pendek, keputusan kedua, keputusan setelah request pindah status, vault/replenish reject, return-edit-resend, realisasi.

| Test baru | File | Covers |
|---|---|---|
| `TestIntegration_VaultPlan_RoleScopeAndStateGuards` (8) | `vault_plan_rbac_integration_test.go` | plan tak ada → `ErrVaultPlanNotFound`; **role salah per langkah** (ACM-SPV save/submit, ACM-USER approve/reject, ADMIN dan ATM-SPV approve → `ErrNotAuthorized`); **non-member area** tidak bisa save/submit/approve/reject (404, tanpa bocor), status request tetap; approve draft → `ErrInvalidTransition`; review sebelum ACM approve → `ErrInvalidTransition`; review oleh ACM-SPV → `ErrNotChecker`, request tak ada → `ErrNotFound`, alasan kosong / > 500; request dibatalkan tidak bisa direview |
| `TestIntegration_VendorOrder_DecisionGuards` (3) | `vendor_order_rules_integration_test.go` | alasan reject > 500 → `ValidationError`; role vendor tanpa baris users → `ErrNotAuthorized`; accept/reject pada request dibatalkan → `ErrInvalidTransition`, party tetap `pending` |

| Command | Result |
|---|---|
| `go test -tags integration -cover ./internal/service/` (DB dev) | ok, 0 SKIP; paket 82,0% → **82,2%**; `planFor` 76,9 → 84,6%, `planTx` 81 → 85,7%, `Approve` 72,2 → 77,8%, `ReviewRequest` 72,9 → 81,2%, `scope` 80 → 90%, `decide` 80,5 → 82,9% |
| `golangci-lint` | 0 issues |
| `coverage-thresholds.txt` `service` | tetap 82 |

## Fase C — slice C4.4 `service` / sisanya: money + import CSV (run 2026-10-15, committed `56d0a8f`; failing import test diserahkan ke sesi perbaikan)
Pemilihan berdasarkan prioritas FR-C3 dari daftar fungsi < 70%: **money math** (harga paket, desimal ke DB) dan **maker-checker impor CSV** (baris update/toggle). Sisa fungsi < 70% kebanyakan cabang error DB atau read/list tanpa aturan bisnis.

| Test baru | File | Covers |
|---|---|---|
| `TestValidatePriceContent_BasePrice` (14) | `masterdata_money_test.go` (unit) | harga = desimal ≥ 0, maks 18 digit bulat + 2 desimal, di-trim; ditolak: negatif, 3 desimal, 19 digit, pemisah ribuan, eksponen, teks, titik di akhir; kosong = warisi |
| `TestValidatePriceContent_OptionalFields` | idem | SLA note kosong → nil, di-trim; tanggal akhir `YYYY-MM-DD` (salah format → error, kosong → nil) |
| `TestNumericFromDecimalPtr` (5) | idem | desimal → `numeric` **persis** (dicek via `big.Rat`), nil/kosong → NULL, sampah → error (tidak pernah nol diam-diam) |
| `TestTierMaxInt32` | idem | tier terbuka tetap nil |
| `TestMasterDataImportConfirm_UpdateAndToggleRows_EffectOnlyAfterApproval_Integration` | `masterdata_import_update_toggle_integration_test.go` | **GAGAL — menemukan bug produk (lihat Temuan #2); belum di-commit, diserahkan ke sesi perbaikan** |

| Command | Result |
|---|---|
| `go test -tags integration -cover -skip UpdateAndToggleRows… ./internal/service/` (DB dev) | ok, 0 SKIP; paket 82,2% → **82,3%**; `validatePriceContent` 68,4 → 100%, `numericFromDecimalPtr` → 100%, `tierMaxInt32` 50 → 100% |
| `golangci-lint` | 0 issues |
| `coverage-thresholds.txt` `service` | tetap 82 |

### Temuan produk #2 (diperbaiki `1f2e8a3`, bugfixes.md 2026-10-15)
Impor CSV master-data yang berisi baris **toggle** (hanya `is_active` berubah → enable/disable) gagal saat **Confirm**: `ERROR: null value in column "payload" of relation "master_data_change_requests" violates not-null constraint (23502)` → HTTP 500. Penyebab: `MasterDataImporter.Confirm` (`internal/service/masterdata_import_confirm.go`, loop `params[n] = …`) memakai `marshalOrNilJSON(c.req.Payload)` tanpa fallback `{}`; jalur biasa `MasterDataChangeService.Submit` (`masterdata_change.go:144`) sudah punya fallback itu untuk op enable/disable. Berlaku untuk semua entitas yang bisa di-toggle lewat CSV (vendors, branches, vaults, PIC, ATM, kelolaan). `TestImport_StageToggle` memakai fake sehingga tidak menangkapnya. Test integration di atas adalah failing test untuk perbaikan.

### Status Fase C
Target FR-C3 tercapai: `service` 82,3%, `approval` 88,6%, `rolemgmt` 93,5% (semua ≥ 80%), ambang anti-turun aktif di CI. Test prioritas (maker ≠ checker, RBAC service, money math, applier stale/409, audit-in-tx rollback) tercakup. Temuan #2 diperbaiki di `1f2e8a3` (helper bersama `marshalPayloadJSON`).

## Fase F — Python EOD / BE-05 (run 2026-10-15, committed `c34a038`)

| Perubahan | File |
|---|---|
| F1: semua requirements `>=` → `==` (versi terpasang, plan baris 27); baru `dmaa/requirements.txt`, `itm/cashpos/requirements.txt` | `backend_python/{dsr,itm/replenish,eod_retry_scheduler,service_dsr_etl,dmaa,itm/cashpos}/requirements.txt` |
| F2: `python-jose[cryptography]` → `PyJWT==2.15.1` (diinstal dengan izin user); `jwt.decode(..., algorithms=["HS256"], options={"verify_aud": False, "require": ["exp"]})`, `jwt.InvalidTokenError`. Klaim/role yang dicek tidak berubah (`role` wajib, identitas `sub`/`id`, `EOD_ROLES`) | `lib/dependencies.py`, `lib/test_eod_api.py`, requirements kedua service |
| F3: `python:3.14-slim`, satu `pip install --no-cache-dir` untuk 6 requirements (konflik versi = build gagal), user non-root 10001, `TZ=Asia/Jakarta`, `WORKDIR /app`, HEALTHCHECK `urllib` ke `HEALTHCHECK_PORT` (8091; DSR override 8090), ENV path script ETL relatif ke `backend_python/` (default config relatif ke root repo → salah di container) | `backend_python/Dockerfile`, `backend_python/.dockerignore` |
| CI: job `python-image` (`docker build`); loop install job `python` ikut `backend_python/itm/*/requirements.txt` | `.github/workflows/ci.yml` |

| Test | Covers |
|---|---|
| `test_jwt_role_gate_and_token_hardening` (diperluas, 2 service × 5 route) | T-F2: secret salah, kedaluwarsa (`exp` −60 s), HS512 (secret benar), `alg none` (tanpa signature), header RS256 dengan signature HMAC secret (algorithm confusion) → 401; tanpa `exp`, tanpa `role` → 401; VENDOR → 403; ADMIN/APPACCESS → 200 |
| Mutasi: `algorithms=["HS256","HS512"]` | test di atas gagal (`200 != 401 : /status HS512`) → dipulihkan |

| Command | Result |
|---|---|
| `py -3 -m unittest lib.test_eod_api -v` (DB dev, `localhost`) | 9 OK |
| `py -3 -m unittest discover` (DB dev) | T-F1: **77 OK, 0 skip** |
| `py -3 -m lib.dependencies` | demo OK |
| `docker build -t cms-eod:local backend_python` (Docker 29.8.2) | OK; di image: uid 10001, `date` = WIB, semua import + PyJWT 2.15.1, tanpa `python-jose`, tanpa `.env`/test |
| `docker run` (DB dev, `FTP_DATA` kosong di-mount) default + override `python -m service_dsr_etl.run`, `HEALTHCHECK_PORT=8090` | T-F3: keduanya `healthy`, `/health` 200; path script ETL ada |

### Catatan (Fase F)
- `/health` mengembalikan 503 bila `FTP_DATA` (induk direktori input) tidak ada → deploy wajib me-mount `/app/FTP_DATA` (atau set `*_FTP_DATA_ROOT`).
- PyJWT memvalidasi `iat` di masa depan (python-jose tidak); Go dan Python satu VM sehingga clock skew diabaikan. PyJWT memberi warning bila secret HS256 < 32 byte; `JWT_SECRET` prod harus ≥ 32 byte.
- CI direncanakan pindah ke Jenkins (`.claude/sdlc/ci-jenkins/`, on hold); job `python-image` ditambahkan di `ci.yml` sesuai plan dan perlu dipindahkan bersama job lain.

### Outstanding (Fase F)
- Run GitHub Actions (`python`, `python-image`) setelah user push.
- Wiring deploy image Python (registry, compose/VM) — di luar plan.

## Fase E — slice E3 + E4 / BE-08, BE-09 (run 2026-10-15, committed `9f54d5a`; timeout 30 s diterima user apa adanya)

| Perubahan | File |
|---|---|
| E3: `BodyLimit(max, exempt)` membungkus `http.MaxBytesReader` (exempt `nil` = semua dibatasi) | `pkg/middleware/body_limit.go` (+ test) |
| E3: `JSONBodyLimit` = 1 MiB + `IsBodyLimitExempt`: `POST /api/v1/dsr/uploads` (exact; `/confirm` JSON tetap dibatasi) dan prefix `POST /api/v1/admin/master-data/import/` | `backend/internal/handler/body_limit.go` (+ test) |
| E3: urutan `RequestID` → `TrustedRealIP` → `middleware.Logger` → `Recoverer` → `middleware.Timeout(30s)` → `BodyLimit` | `backend/cmd/api/main.go` |
| E3: `Timeout(30s)` + `BodyLimit(1<<20, nil)` | `backend-cit/cmd/api/main.go` |
| E4: `?pool_max_conns=20` di contoh `DATABASE_URL` + komentar replica; usulan per env + rumus | `backend/.env.example`, `.claude/docs/deployment.md` ("Ukuran pool DB") |
| E4: `slog.Info("db pool", "pool", "primary"\|"replica", "max_conns", …)` | `backend/cmd/api/main.go` |

| Test | Covers |
|---|---|
| `TestBodyLimit` (pkg, 4 kasus) | T-E3: tepat di batas terbaca penuh; batas + 1 → `*http.MaxBytesError`; path exempt tidak dibatasi; exempt `nil` membatasi semua |
| `TestIsBodyLimitExempt` (7 kasus) | T-E3: hanya DSR upload + master-data import yang dikecualikan; `dsr/uploads/confirm`, `GET dsr/uploads`, holiday import, roles PUT tidak |
| `TestBodyLimit_Router` | T-E3: `PUT /api/v1/admin/roles/7/permissions` body JSON 1 MiB + 1 → 400; tepat 1 MiB → 200; upload DSR 2 MiB → lolos batas, sampai dry-run (502 ETL stand-in tak terjangkau) |

| Command | Result |
|---|---|
| `go build ./... && go vet ./...` (pkg, backend, backend-cit) | OK |
| `go test ./...` ×3 | OK (`-race` tidak bisa lokal: butuh cgo; jalan di CI) |
| `go test -tags integration -cover ./...` (backend, DB dev `localhost`) | OK; approval 88.6, audit 86.7, auth 90.1, handler 80.7, notification 83.9, repository 86.9, rolemgmt 93.5, service 82.4 — semua ≥ `coverage-thresholds.txt` |
| `golangci-lint run --build-tags integration ./...` ×3 | 0 issues |
| Start `backend/cmd/api` lokal (`pool_max_conns=20`, replica = primary) | T-E4: log `{"msg":"db pool","pool":"primary","max_conns":20}` + `{"msg":"db pool","pool":"replica","max_conns":20}` (start berhenti setelahnya karena Redis lokal mati — tidak terkait) |

### Deviasi / catatan (E3, E4)
- **`RequestLogger` belum ada** (E1 belum dikerjakan): urutan dipertahankan dengan chi `middleware.Logger` di posisinya; Timeout + BodyLimit disisipkan setelah `Recoverer`. Saat E1, `middleware.Logger` diganti di tempat yang sama.
- **Grep endpoint multipart**: hanya tiga handler membaca multipart — DSR upload (10 MiB, exempt), master-data import (5 MiB, exempt), dan holiday import `POST /api/v1/admin/holidays/import`. Holiday import **tidak** ditambahkan ke pengecualian: handler-nya sendiri membatasi seluruh body 1 MiB (`holidayImportMaxBytes`), jadi batas global tidak mengubah perilakunya. Laporan realisasi vendor dan VendorPortal lain = JSON. Tidak ada endpoint upload yang hilang dari plan.
- **`json.Decoder` berhenti setelah nilai JSON pertama**: padding setelah objek JSON tidak pernah dibaca sehingga tidak memicu batas (server juga tidak membacanya). Test menaruh padding di dalam nilai JSON.
- **Long-running vs Timeout 30 s** (dilaporkan ke user, tidak diputuskan sendiri): `http.Server.WriteTimeout` sudah 30 s di kedua backend, jadi plafon waktu tidak berubah; yang baru adalah context dibatalkan (query DB berhenti) + 504 bila belum ada byte terkirim. Terdampak: DSR upload (`POST /api/v1/dsr/uploads`: baca 10 MiB + dry-run ke `service_dsr_etl` dengan client 30 s → total bisa > 30 s, NFR ≤ 30 s/dok), export CSV master-data (streaming, `ErrAbortHandler` bila putus di tengah), export di `replenishment_handler.go` (≤ 31 hari). Tidak ada SSE/streaming lain. `ReadTimeout` 10 s untuk upload 10 MiB di koneksi lambat juga sudah ada sebelumnya.
- `backend-cit` tidak diberi log pool (di luar tugas; modul akan di-rename ke `backend-exq`).

### Outstanding (E3, E4)
- Uji manual browser: upload DSR > 1 MiB dan import master-data > 1 MiB di UI (user, Golden Rule #10).
- Nilai `pool_max_conns` per env diterapkan di Secret Manager/`.env` VM sesudah `SHOW max_connections` di Cloud SQL (user).

## Fase E — slice E1 + E2 + E5 / BE-04 (run 2026-10-15, committed `180d9f8`)

| Perubahan | File |
|---|---|
| E1: `RequestLogger(logger)` — chi `NewWrapResponseWriter`, `RoutePattern()` (kosong → `unmatched`), `slog.LogAttrs` msg `http_request` (method, route, path, status, duration_ms, bytes, request_id, remote_ip tanpa port); 5xx ERROR / 4xx WARN / lainnya INFO; `/health` + `/ready` 2xx dilewati; header/query/body tidak pernah dicatat | `pkg/middleware/request_logger.go` (+ test) |
| E1: `middleware.Logger` → `custommw.RequestLogger(slog.Default())` di posisi yang sama: `RequestID` → `TrustedRealIP` → `RequestLogger` → `Recoverer` → `Timeout` → `BodyLimit` (cit: tanpa `TrustedRealIP`, seperti sebelumnya) | `backend/cmd/api/main.go`, `backend-cit/cmd/api/main.go` |
| E1: `github.com/go-chi/chi/v5 v5.3.0` masuk `pkg/go.mod` (versi sama dengan `backend`; bukan dependency baru di stack) | `pkg/go.mod`, `pkg/go.sum` |
| E2: `health.Ready(timeout, checks)` — cek paralel, status per nama, 200/503, error di-`slog.Warn` saja | `pkg/health/ready.go` (+ test) |
| E2: `GET /ready` publik (2 s): backend `db`, `db_replica` (hanya bila `dbReadPool != dbPool`), `redis`; cit `db`, `redis` | kedua `main.go` |
| E5: filter, metric distribusi `duration_ms` label `route`, counter `status>=500`, contoh alert p95 > 3 s | `.claude/docs/deployment.md` ("Request logs, `/ready` and log-based metrics") |

| Test | Covers |
|---|---|
| `TestRequestLogger` (pkg, 6 kasus) | T-E1: 2xx INFO + route pattern `/api/v1/items/{id}` + bytes; 4xx WARN; panic → Recoverer 500 ERROR tercatat; route tak dikenal → `unmatched`; `/health` 2xx tidak dicatat; `/ready` 503 tetap dicatat; tiap kasus: header `Authorization` + query `token=` tidak muncul di log, `request_id` terisi, `duration_ms` numerik, `remote_ip` tanpa port |
| `TestReady` (pkg, 4 kasus) | T-E2: semua ok → 200; satu gagal → 503 + `fail`, pesan error (berisi password DSN) tidak bocor ke body; check menggantung → dipotong timeout → 503; tanpa check → 200 |

| Command | Result |
|---|---|
| `go build ./... && go vet ./...` (pkg, backend, backend-cit) | OK |
| `go test ./...` ×3 | OK (`-race` tidak bisa lokal: butuh cgo; jalan di CI). Coverage pkg: `middleware` 89.1%, `health` 100% |
| `go test -tags integration -count=1 -cover ./...` (backend, DB dev `localhost`) | OK, 0 SKIP, 0 FAIL; `scripts/check-coverage.sh` semua `ok` (approval 88.6, audit 86.7, auth 90.1, handler 80.7, notification 83.9, repository 86.9, rolemgmt 93.5, service 82.4) |
| `golangci-lint run --build-tags integration ./...` ×3 | 0 issues |
| `backend` lokal (`cms-redis` dinyalakan sementara lalu dimatikan lagi) | T-E2: `curl /ready` → 200 `{"checks":{"db":"ok","redis":"ok"},"status":"ok"}`; Redis dimatikan → 503 `redis:"fail"` + log `readiness check failed` (EOF) + `http_request` ERROR route `/ready`. T-E1: request `GET /api/v1/approvals/?token=zzz` + `Authorization: Bearer eyJfake` → baris `http_request` WARN 401 route `/api/v1/approvals`; `grep -c 'eyJ\|zzz'` log = 0 |
| `backend-cit` lokal (Redis mati) | T-E2: `curl /ready` → 503 `{"db":"ok","redis":"fail"}` |
| T-E5 | docs saja (`deployment.md`); pembuatan metric + alert di GCP = user |

### Catatan (E1, E2, E5)
- `RoutePattern()` untuk `r.Post("/")` di dalam `r.Route("/api/v1/items")` adalah `/api/v1/items` (chi memangkas `/` akhir) — label metric jadi tanpa slash akhir.
- Log memakai `context.Background()`: context request bisa sudah dibatalkan oleh `Timeout`/client pergi, baris log tetap harus tertulis.
- `remote_ip` = `r.RemoteAddr` sesudah `TrustedRealIP` (backend), port dibuang lewat `parseRemoteAddr` yang sudah ada.
- Docker `HEALTHCHECK` **tidak** diubah (tetap `/health`); pindah ke `/ready` perlu keputusan user (restart container tiap DB/Redis bermasalah).
- slog menulis `level`, bukan `severity` → di Cloud Logging severity `DEFAULT`; dicatat di `deployment.md`, tidak diubah (di luar E1).

### Outstanding (E1, E2, E5)
- GCP: buat metric `http_request_latency` + `http_5xx`, alert p95 > 3 s + kanal notifikasi; pastikan log container ter-parse JSON ke `jsonPayload` (user).
- Keputusan: LB/uptime check memakai `/ready`; Docker `HEALTHCHECK` tetap `/health` atau pindah (user).

## Fase D — runner migrasi / BE-03 (run 2026-10-15, committed `88a3139`)

| Perubahan | File |
|---|---|
| D1: runner `status` / `up` / `baseline --through NNN`, flag `-dir` (default `migrations`), `DATABASE_URL`; `pg_advisory_lock(hashtext('cms.schema_migrations'))` di koneksi runner; `CREATE TABLE IF NOT EXISTS schema_migrations` (kolom sesuai spec); hanya `^\d{3}_.*\.sql$` level atas, urut nama, SHA-256; checksum beda → exit ≠ 0 sebelum apa pun jalan; `psql -v ON_ERROR_STOP=1 -X -q -f` per file, env `PGHOST/PGPORT/PGUSER/PGPASSWORD/PGDATABASE/PGSSLMODE` dari `pgx.ParseConfig` (password tidak di argv/log); `status`/`up` cek `psql` di PATH | `backend/cmd/migrate/main.go`, `runner.go` |
| D1: test | `backend/cmd/migrate/runner_test.go` (unit), `runner_integration_test.go` (`//go:build integration`) |
| D1: ambang coverage paket baru `cmd/migrate 81` | `backend/coverage-thresholds.txt` |
| D3: `go-integration` + `python` → `go run ./cmd/migrate up` (job `python` dapat `setup-go`); env `PG*` dihapus dari kedua job (hanya dipakai skrip lama); `scripts/apply-migrations.sh` dihapus. Tidak ada job `e2e` (B5 ditunda) | `.github/workflows/ci.yml` |
| D3: Sec 2a perintah migrasi, Sec 3 Core `schema_migrations`, Sec 12 "Migration runner" | `.claude/CLAUDE.md` |
| D3: langkah migrasi saat deploy (host operator, di luar 07:00–20:00, `status` → `up` → `status`) | `.claude/docs/deployment.md` |

| Test | Covers |
|---|---|
| `TestUpAppliesAllThenIsNoop` | T-D1: `up` di DB kosong menerapkan 3 file (termasuk `CREATE INDEX CONCURRENTLY` → bukti tidak dibungkus transaksi, seperti `006`); baris `baseline=false`, `applied_by=current_user`; `up` kedua = no-op (seed tetap 1 baris); `run(... status)` lewat CLI → `3 applied, 0 pending` |
| `TestChecksumMismatchStopsBeforeRunningAnything` | T-D1: file applied diubah + file baru pending → `up` error `checksum mismatch … 001_a.sql`, file baru **tidak** jalan; `status` exit ≠ 0 + `MISMATCH` |
| `TestBaselineMarksWithoutRunningSQL` | T-D1: `baseline --through 2` atas file `SELECT 1/0` → 2 baris `baseline=true`, SQL tidak jalan; `up` sesudahnya hanya menjalankan `003` |
| `TestLowerNumberAddedLaterIsApplied` | T-D1 / FR-D2: `002` yang muncul setelah `001`+`003` applied ikut diterapkan (kasus `022`) |
| `TestFailureStopsAndIsNotRecorded` | FR-D2: `002` gagal → error menyebut file, stderr psql diteruskan, `002` tidak tercatat, `003` tidak jalan |
| `TestConcurrentUpAppliesOnce` | T-D1: dua goroutine `up` paralel (file pertama `pg_sleep(1)`) → keduanya sukses, satu `2 applied` satu `0 applied`, tabel seed 1 baris, 2 baris `schema_migrations` |
| `TestSSLModeRoundTrip`, `TestPsqlEnvCarriesConnectionAndOverridesInherited`, `TestReadMigrationsFiltersAndSorts`, `TestRunRejectsBadArgs`, `TestRunParseErrorDoesNotLeakPassword` (unit) | mapping `sslmode` ×6 sama dengan pgconn; `PG*` menimpa env warisan; filter `NNN_*.sql` (tanpa `archives/`, `02_`, `.bak`); argumen salah → usage; error parse URL tanpa password |

| Command | Result |
|---|---|
| `go build ./... && go vet ./... && go vet -tags integration ./...` (backend) | OK |
| `go test -count=1 ./...` (backend) | OK (`-race` butuh cgo, jalan di CI) |
| `go test -tags integration -count=1 -cover -v ./cmd/migrate` (DB dev `localhost`, DB sementara `cms_migrate_test_*` via `CREATE DATABASE`, di-`DROP … WITH (FORCE)`) | 11 PASS, 0 SKIP, coverage 81.8%; sesudahnya `pg_database LIKE 'cms_migrate_%'` = 0 |
| Paritas CI: DB sementara kosong → `go run ./cmd/migrate up` atas `backend/migrations/` asli → `status` → `DROP` | `29 applied` (001–031 tanpa 022/024), `status` semua `ok`, `29 applied, 0 pending`, exit 0 |
| `go test -tags integration -count=1 -cover ./...` (backend, DB dev) | OK, 0 SKIP, 0 FAIL; `check-coverage.sh` semua `ok` (cmd/migrate 81.8, approval 88.6, audit 86.7, auth 90.1, handler 80.7, notification 83.9, repository 86.9, rolemgmt 93.5, service 82.4) — tidak ada yang turun |
| `golangci-lint run --build-tags integration ./...` (backend) | 0 issues (G204 `exec` + G304 `ReadFile` `//nolint:gosec` dengan alasan; `Fprintf` ke tabwriter/stdout `_, _ =`) |
| T-D2 pra-cek dev (read-only, `BEGIN READ ONLY` … `ROLLBACK`) | `cms`, PG 18.3; `schema_migrations` belum ada; objek 029 (`vendor_request_vendor_parties`), 030 (`acm_area_regions`, `vendor_request_vault_allocations`/`_amounts`, `vendor_request_vault_tickets`, `vendor_request_vault_plans_live_uq`, kolom `buffer_reason`) dan 031 (`holidays` + `holidays_active_date_uq`, `vendor_request_realizations`/`_amounts`, `replenish_classifications`/`_amounts` + `_open_idx`) ada → baseline yang benar `--through 031` (plan menulis 029, usang) |
| T-D2 baseline dev (izin user 2026-10-15: "ya, jalankan baseline --through 031 di dev lalu status") `go run ./cmd/migrate baseline --through 031` → `status` | `29 baselined`, exit 0; `status` exit 0: `29 applied, 0 pending`, semua `ok`, `baseline=true`, tanpa 022/024 (output di bawah) |
| T-D3 | CI berubah, dibuktikan oleh run GitHub setelah user push (lokal: paritas di atas) |

### Output `status` dev setelah baseline (T-D2, 2026-10-15)

```text
VERSION                                   STATE    CHECKSUM  BASELINE
001_baseline_schema.sql                   applied  ok        true
002_baseline_seed.sql                     applied  ok        true
003_master_data.sql                       applied  ok        true
004_vendor_packages_soft_disable.sql      applied  ok        true
005_master_data_import_batches.sql        applied  ok        true
006_audit_logs_read_indexes.sql           applied  ok        true
007_branch_coverage_areas.sql             applied  ok        true
008_vendor_vault_types.sql                applied  ok        true
009_vendor_package_prices.sql             applied  ok        true
010_vendor_package_prices_data.sql        applied  ok        true
011_rename_vendor_packages_branch.sql     applied  ok        true
012_drop_vendor_package_price_addons.sql  applied  ok        true
013_vendor_branches_category.sql          applied  ok        true
014_vendor_branches_category_seed.sql     applied  ok        true
015_vendor_packages_branch_tiering.sql    applied  ok        true
016_vendor_packages_branch_pricing.sql    applied  ok        true
017_vendor_package_code_split.sql         applied  ok        true
018_regions_soft_delete.sql               applied  ok        true
019_import_jobs.sql                       applied  ok        true
020_import_jobs_current_uq_uploads.sql    applied  ok        true
021_atm_visit_quota.sql                   applied  ok        true
023_atm_assignment_vendor_source.sql      applied  ok        true
025_notifications.sql                     applied  ok        true
026_replenish_ticket_region.sql           applied  ok        true
027_acm_vault_assignment.sql              applied  ok        true
028_dsr_flow_comment.sql                  applied  ok        true
029_vendor_request_send.sql               applied  ok        true
030_vault_allocation.sql                  applied  ok        true
031_replenish_realization.sql             applied  ok        true
29 applied, 0 pending
```

### Deviasi / catatan (Fase D)
- Baseline dev `--through 031`, bukan `029` (plan/spec usang: 030 diterapkan 2026-10-13, 031 2026-10-14; diverifikasi read-only di atas).
- `status` juga gagal (exit ≠ 0) bila ada `MISMATCH`, supaya urutan deploy `status` → `up` berhenti di langkah 1. Versi tercatat yang filenya tidak ada lagi ditampilkan `no file` (tidak error).
- `status` bukan read-only: membuat `schema_migrations` bila belum ada (`CREATE TABLE IF NOT EXISTS`). Karena itu `status` di dev tidak dijalankan sebelum izin user; pra-cek memakai SELECT di atas.
- Hanya host pertama dari `DATABASE_URL` diteruskan ke psql (multi-host tidak didukung, komentar `ponytail:` di `psqlEnv`). `sslrootcert` dsb. tidak diterjemahkan; operator meng-export `PGSSLROOTCERT` (ditulis di `deployment.md`).
- `backend/internal/handler/integration_test.go` `runMigrations` tidak diubah: ia no-op bila `roles` sudah ada (CI sudah `migrate up` sebelum test), dan di DB kosong hanya menjalankan 001+002 lewat `pool.Exec`. Runner tidak bisa di-import (paket `main`); kalau gap ini mau ditutup, test bisa memanggil `go run ./cmd/migrate up` / binary via `exec`, atau logika runner dipindah ke paket yang bisa di-import — keputusan terpisah.
- `.claude/sdlc/ci-jenkins/` (on hold) masih menyebut `apply-migrations.sh` di intent/spec; diperbarui saat ci-jenkins dilanjutkan (Jenkins memakai `go run ./cmd/migrate up`).

### Outstanding (Fase D)
- Run GitHub Actions setelah push (T-D3).
- Deploy pertama dengan runner (staging/prod = DB kosong → `up`, atau baseline bila sudah dimigrasi manual).

## Fase G — slice G1 / BE-07 (run 2026-10-19, `b007b64`)

| File | Sebelum | Sesudah |
|---|---|---|
| `service/vendor_request_actions.go` | 1204 | 502 |
| `service/vendor_request_queries.go` (baru) | — | 349 |
| `service/vendor_request_create.go` (baru) | — | 382 |
| `service/masterdata_import.go` | 862 | 526 |
| `service/masterdata_import_csv.go` (baru) | — | 157 |
| `service/masterdata_import_validate.go` (baru) | — | 195 |

- Hanya blok dipindah (package `service` sama; import dirapikan `gopls imports`, lalu `gofmt`). Tanpa ubah signature, route, response, atau test.
- `go build ./... && go vet ./... && go test ./...` hijau; `golangci-lint run --build-tags integration ./...` = 0 issues; `gofmt -l` bersih.
- `go test -list . ./internal/service ./internal/handler` sebelum vs sesudah: identik (597 test).
- Integration `go test -tags integration ./internal/service` dengan `DATABASE_URL` localhost: ok (45 s).
- Tidak dikerjakan: file test > 800 baris (`masterdata_approval_integration_test.go` 1328, `handler/vendor_request_handler_test.go` 857) — keputusan user: tidak dipecah.

## Fase G — slice G2 / BE-11 (run 2026-10-19, `b007b64`; docs only)

| Perubahan | File |
|---|---|
| Tabel produk #2, "Current state", "Pipeline" (CD hanya `backend/Dockerfile`), tabel ukuran pool (1 instance `backend` hari ini; 2 setelah `backend-exq` di-deploy); rencana split VM 2028 dipertahankan | `.claude/docs/deployment.md` |
| Sec 10 "Today" + Sec 12 "Backend topology": hanya `backend` yang di-deploy | `.claude/CLAUDE.md` |
| Rujukan root `docker-compose.yml` (tidak ada) → `backend/docker-compose.yaml` | `.claude/docs/deployment.md` |

| Test | Result |
|---|---|
| T-G2: `grep -rn "docker-compose\.yml" .claude/docs .claude/CLAUDE.md` + `grep -n "backend-cit" .claude/docs/deployment.md` | tidak ada lagi rujukan root compose; sisa `backend-cit` di `deployment.md` hanya sebagai "tidak di-deploy sampai `backend-exq`" / catatan `TRUSTED_PROXIES` / pool |
| `ci.yml`, Dockerfile, kode `backend-cit/` | tidak diubah |

### Catatan (G2)
- Rujukan lama tetap ada di salinan usang `.kiro/steering/*` dan `.opencode/opencode.md` (bukan sumber kebenaran, tidak disentuh).
- Module `backend-cit` tetap di `go.work` dan CI tetap membangun/mengujinya (FR-B1). Image `backend-cit` belum non-root/HEALTHCHECK (lihat Catatan Fase A) → selesaikan saat rename ke `backend-exq`.

## Fase G — slice G3 / BE-10 (run 2026-10-19, `b007b64`; docs only)

| Perubahan | File |
|---|---|
| BE-10 Status `open` → `deferred (be-tech-debt Q8)`, catatan "mulai dari modul baru berikutnya" | `.claude/BE_debt.md` |

| Test | Result |
|---|---|
| T-G3: `grep -n "BE-10" .claude/BE_debt.md` | Status = `deferred (be-tech-debt Q8, ...)`, kolom lain tidak berubah |
| Kode, spec OpenAPI, dependency | tidak ada (FR-G3 docs only) |
