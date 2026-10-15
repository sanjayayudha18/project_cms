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
