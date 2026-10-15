# Backend Tech Debt

Hasil review arsitektur BE (`backend/`, `backend-cit/`, `pkg/`, `backend_python/`) terhadap standar aplikasi modern, 2026-10-12. Satu baris per item; ubah **Status** saat dikerjakan (`open` → `in-progress` → `done <sha>`). Sebelum mengerjakan item, tanya dulu: AI-DLC baru atau cukup `bugfixes.md` (CLAUDE.md Sec 4a rule 0). Item yang menyentuh migrasi/auth/uang selalu full chain (Sec 4 #7).

Pasangan: [FE_debt.md](./FE_debt.md). AI-DLC perbaikan: [`sdlc/be-tech-debt/`](./sdlc/be-tech-debt/intent.md). Riwayat QC sebelumnya: `backend_QC_result.md`, `backend_QC_fixes.md` (root repo).

## Baseline saat review

| | backend | backend-cit | pkg | backend_python |
|---|---|---|---|---|
| Test | lulus | – (belum ada kode) | lulus | 79 lulus, 53 skip (butuh `DATABASE_URL`) |
| `go vet` / golangci-lint (+gosec, gocyclo) | 0 issue | 0 | 0 | – |
| govulncheck | 0 terjangkau (4 di module, tidak dipanggil) | 0 | 0 | – |
| Coverage unit | approval 54,2 · auth 90,1 · handler 79,5 · notification 81,2 · repository 15,4 · rolemgmt 35,8 · service 44,6 | – | 83,9–100 | – |

Sudah baik (jangan dirombak): Go 1.26 + chi v5 + pgx v5 + sqlc pinned, minim dependency; workspace 3 module acyclic; handler → service → repository → sqlc; maker-checker + audit dalam satu tx; pool primary/replica; tidak ada SQL via `Sprintf`; error 500 tidak bocor ke client; `JWT_SECRET` ≥ 32 byte; rate limit Redis; trusted-proxy IP; `slog` JSON, graceful shutdown, server timeouts; Docker multi-stage non-root + HEALTHCHECK; Python: `Decimal`, idempotent SHA-256, `hmac.compare_digest`, `create_subprocess_exec`.

## HIGH

| ID | Item | Bukti | Perbaikan | Status |
|---|---|---|---|---|
| BE-01 | Coverage modul uang/maker-checker di bawah 80% | Lihat baseline: `service` 44,6%, `approval` 54,2%, `rolemgmt` 35,8%, `repository` 15,4%. 55 file test `//go:build integration` ter-skip tanpa `DATABASE_URL` | Jalankan integration test di CI (BE-02) lalu ukur ulang; tambah test pada gap yang tersisa. | in-progress: baseline baru 2026-10-15 (unit + integration, DB bermigrasi penuh, `-race`): `service` 81,6% → 81,7% (C4.1 masterdata_approval `82ae252`) → 82,0% (C4.2 vendor_request `f9f66e8`) → 82,2% (C4.3 vault plan + vendor order `fb92794`), `approval` 81,3% → 88,6% (C3 `993540b`), `rolemgmt` 85,4% → 93,5% (C2 `8013dd6`), `repository` 86,9%, `handler` 80,7%, `auth` 90,1%, `audit` 86,7%, `notification` 83,9% — semua ≥ 80%; ambang anti-turun `backend/coverage-thresholds.txt` dicek di CI (C1, `ddefb12`). Test prioritas FR-C3 = C2–C4 |
| BE-02 | Tanpa CI pipeline | Tidak ada `.github/workflows` / Cloud Build; lint, test, govulncheck, sqlc diff semuanya manual | Pipeline per module: `go vet`, `golangci-lint --build-tags integration`, `go test` + service Postgres, `govulncheck`, `sqlc diff`; Python unittest. Gabung dengan FE-06: tambah job ke `.github/workflows/ci.yml` yang dibuat `sdlc/fe-tech-debt`, termasuk job E2E Playwright (`frontend/CompanyPortal-Vite/e2e/`) di atas Postgres + backend (keputusan S3 FE). | in-progress (ci.yml job go/go-integration/sqlc/python green lokal 2026-10-15, committed `077ce05` (+ `8b3a32b`), belum push; job e2e menunggu FE E3) |
| BE-03 | Migrasi manual tanpa tracking | `psql -f backend/migrations/0NN_*.sql`; tidak ada tabel pencatat → drift dev/staging/prod | Keputusan stack ("no migrate tool") → **butuh approval**. Opsi minimal: tabel `schema_migrations` + runner stdlib; alternatif golang-migrate. | open |
| BE-04 | Observability belum siap produksi | Tanpa metrics/tracing (NFR p95 ≤ 3 s, 300 user tidak terukur); `middleware.Logger` chi menulis teks, bercampur dengan `slog` JSON (`backend/cmd/api/main.go:104`); `/health` hanya liveness, tidak cek DB/Redis (`main.go:108`) | Logger request berbasis `slog`; endpoint readiness (ping DB + Redis); metrics latency/error (Cloud Monitoring). | open |
| BE-05 | Dependency Python tidak di-pin + `python-jose` | `backend_python/*/requirements.txt` memakai `>=`, tanpa lockfile; `python-jose` minim maintenance, riwayat CVE algorithm-confusion; layanan Python tanpa Dockerfile | Pin versi (lock), ganti ke PyJWT (perlu approval dependency), Dockerfile untuk `eod_retry_scheduler` + `service_dsr_etl`. | open |
| BE-06 | Base image runtime EOL | `backend/Dockerfile`: `FROM alpine:3.20` (support berakhir April 2026) | Naik ke Alpine terbaru yang didukung atau distroless (CLAUDE.md Sec 9). Cek juga `backend-cit/Dockerfile`. | done fb3b6fd |

## MEDIUM

| ID | Item | Bukti | Perbaikan | Status |
|---|---|---|---|---|
| BE-07 | Package dan file terlalu besar | `internal/service` 44 file / 13,5k LOC, `internal/handler` 38 file; `service/vendor_request_actions.go` 1210 baris, `service/masterdata_import.go` 862 baris (batas 800) | Pecah file > 800 saat disentuh; package per domain (mis. `internal/vendorrequest`) hanya bila disetujui — layering flat adalah keputusan terdokumentasi. | open |
| BE-08 | Tanpa limit body JSON global + timeout per request | `MaxBytesReader` hanya di handler upload (`admin_master_data_import_handler.go`, `dsr_upload_handler.go`); tanpa middleware timeout → query tetap jalan setelah client putus | Middleware `http.MaxBytesReader` (mis. 1 MB) untuk route non-upload; `middleware.Timeout` chi. | open |
| BE-09 | Ukuran pool DB default | `pgxpool.New(ctx, cfg.DatabaseURL)` (`main.go:47,66`) → `max(4, NumCPU)` koneksi | Set `pool_max_conns` di `DATABASE_URL` / `DATABASE_REPLICA_URL` dan dokumentasikan di `.env.example` + `docs/deployment.md`. Tanpa ubah kode. | open |
| BE-10 | Tanpa kontrak OpenAPI | 138+ endpoint; tipe FE ditulis manual → drift | Spec OpenAPI untuk endpoint yang dipakai FE (mulai dari modul baru); generate tipe FE bila disetujui. | open |

## LOW

| ID | Item | Bukti | Perbaikan | Status |
|---|---|---|---|---|
| BE-11 | `backend-cit` masih kosong tapi dijadwalkan deploy | `backend-cit/internal/*` = 8 file `doc.go` satu baris; rename ke `backend-exq` belum dilakukan | Jangan deploy container-nya sampai `backend-exq` berisi; rename sesuai keputusan 2026-10-08. | open |
| BE-12 | Vuln di module yang di-require (tidak terjangkau) + file usang ter-track | govulncheck: 4 vuln di module `backend`; `backend_python/dmaa/bak/` ter-track di git | `go get -u` module terkait + `go mod tidy`; hapus `dmaa/bak/`. | x/crypto v0.56.0; GO-2026-5932 (openpgp, tanpa fix, tidak di-import) accepted. done fb3b6fd (go) + 9a5af6d (dmaa/bak) |
