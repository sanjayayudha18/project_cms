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

### Temuan produk (STOP, belum diperbaiki — keputusan user)
`ResolveEffectiveApprover` (`internal/approval/effective_approver.go`) mengembalikan error **polos** (tidak dibungkus `ErrNotAuthorized`) untuk dua kasus: (a) approver efektif = maker (mis. approver cuti mendelegasikan ke maker), (b) approver cuti tanpa delegate. `ApprovalHandler.handleError` (`internal/handler/approval_handler.go`) jatuh ke `default` → **HTTP 500 "Terjadi kesalahan internal"**. Persetujuan tetap **ditolak** (fail-closed, tidak ada celah keamanan), tetapi user mendapat 500, bukan 403/409 yang bisa ditindaklanjuti. Test C3 sengaja hanya mengecek "ditolak + tidak berubah", tidak mengunci kode 500.
