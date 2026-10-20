# Spec: Perbaikan tech debt backend (BE-01 – BE-12)

Status: **accepted 2026-10-12** (user: *"terima spec, S1–S8 sesuai usulan, lanjut plan"*).
Stage: 2 Design selesai. Stage berikut: [`plan.md`](./plan.md). Dasar: [`intent.md`](./intent.md) (accepted 2026-10-12, jawaban PO Q1–Q9). Backlog: [`BE_debt.md`](../../BE_debt.md). Pasangan: [`fe-tech-debt/spec.md`](../fe-tech-debt/spec.md) (FR-E1 workflow CI, S3 E2E di CI ditambahkan di sini).

## Flag Sec 4 #7

| Flag | Di mana | Penanganan |
|---|---|---|
| Data migration | FR-D1/D2: runner + tabel baru `schema_migrations`, baseline dev | Tabel baru diusulkan di sini (bagian Data model) lalu dicatat di CLAUDE.md Sec 3 setelah diterapkan. Baseline dev = tulis ke DB dev, dijalankan setelah user OK di langkah itu. |
| Auth | FR-F2 (`python-jose` → PyJWT); FR-E3 (middleware global di depan semua route) | Perilaku verifikasi token Python tidak berubah, dibuktikan test negatif. Middleware baru tidak menyentuh `RequireAuth`/`RequireRoles`. |
| Money / maker-checker | FR-C1 (test `service`, `approval`, `rolemgmt`) | Hanya menambah test. Kalau test menemukan bug di logika uang/maker-checker: STOP, laporkan, catat di `bugfixes.md` atau AI-DLC sendiri. Tidak diperbaiki diam-diam di sini. |
| Delete | FR-A2 (hapus `backend_python/dmaa/bak/`) | File kode, bukan data. Dipastikan tidak di-import sebelum dihapus. |

Tidak ada perubahan perilaku bisnis, response shape, atau endpoint bisnis. Endpoint baru hanya `GET /ready` (FR-E2).

## Baseline (2026-10-12)
- Coverage unit (`go test -cover`, tanpa `DATABASE_URL`): `approval` 54,2 · `auth` 90,1 · `handler` 79,5 · `notification` 81,2 · `repository` 15,4 · `rolemgmt` 35,8 · `service` 44,6 · `pkg/*` 83,9–100.
- 29 file test Go membaca `DATABASE_URL` dan skip tanpa itu (total 55 file `//go:build integration`). Test integration **mengandaikan skema sudah ada** di DB (`handler/integration_test.go` hanya menerapkan 001+002 kalau skema belum ada).
- Python: 79 test lulus, 53 skip (butuh `DATABASE_URL`).
- Migrasi di `backend/migrations/`: `001`–`029` tanpa `022` dan `024`. 23 file mengatur `BEGIN`/`COMMIT` sendiri; `006` memakai `CREATE INDEX CONCURRENTLY` (tidak boleh di dalam transaksi).
- Runtime image `alpine:3.20` (EOL) di `backend/Dockerfile` + `backend-cit/Dockerfile`. `golang.org/x/crypto v0.54.0` di `backend/go.mod`.
- Python: 4 `requirements.txt` (`dsr`, `eod_retry_scheduler`, `service_dsr_etl`, `itm/replenish`) memakai `>=`; `dmaa/` dan `itm/cashpos/` tidak punya requirements sendiri. `python-jose` dipakai di `lib/dependencies.py:9` dan `lib/test_eod_api.py:22`.
- Belum ada `.github/`; FE plan (E1) akan membuat `.github/workflows/ci.yml` dengan job `frontend` + `frontend-image`.

## Functional requirements

### Fase A — Supply chain (BE-06, BE-12)
- **FR-A1** Runtime image `backend/Dockerfile` dan `backend-cit/Dockerfile` memakai `alpine:3.N` terbaru yang masih didukung saat implementasi (dicek di alpinelinux.org/releases), dipin ke minor, bukan `latest`. User non-root, `TZ`, dan `HEALTHCHECK` berbasis `wget` tetap.
- **FR-A2** `golang.org/x/crypto` naik ke ≥ v0.56.0 di semua module yang me-require-nya, lalu `go mod tidy`. `govulncheck ./...` di ketiga module: 0 vuln terjangkau. GO-2026-5932 (belum ada fix, tidak terjangkau) dicatat sebagai *accepted* di `BE_debt.md` BE-12.
- **FR-A3** `backend_python/dmaa/bak/` dihapus dari git setelah dipastikan tidak ada yang meng-import-nya.

### Fase B — CI GitHub Actions (BE-02, + E2E FE)
Semua job ditambahkan ke `.github/workflows/ci.yml` yang sama dengan FE (trigger `push`/`pull_request` ke `main`, `dev1`).
- **FR-B1 `go`**, matrix module `[backend, backend-cit, pkg]`, versi Go dari `go.mod`:
  `go vet ./...` → golangci-lint v2 dengan config root `.golangci.yml` (`--build-tags integration`) → `govulncheck ./...` → `go test -race ./...`.
- **FR-B2 `go-integration`** (module `backend`). Service container Postgres, versi major sama dengan dev/Cloud SQL (dicek di plan dengan `SELECT version()`).
  1. Skema dibangun dari `backend/migrations/` (fase B: loop `psql -v ON_ERROR_STOP=1 -f` urut nomor; fase D: diganti runner FR-D1).
  2. `go test -tags integration -race` dengan `DATABASE_URL` → test integration benar-benar jalan, tidak skip.
- **FR-B3 `sqlc`**: pasang sqlc **v1.31.1** → `sqlc generate` → hand-fix `UserLeafe` → `UserLeave` (quirk di CLAUDE.md Sec 2a) → `git diff --exit-code backend/internal/db`. `sqlc diff` biasa tidak dipakai karena akan selalu gagal akibat hand-fix itu.
- **FR-B4 `python`**: versi Python sama dengan image FR-F3 → `pip install` semua requirements → `python -m unittest` untuk `dsr`, `itm/cashpos`, dan `lib` (termasuk `lib.test_import_jobs` + `lib.test_eod_api`) dengan `DATABASE_URL` ke Postgres job yang sudah bermigrasi.
- **FR-B5 `e2e`** (keputusan S3 FE):
  - Services: Postgres (migrasi + seed E2E, lihat S4) + Redis.
  - Backend `go run ./cmd/api` dengan `JWT_SECRET` acak per run (≥ 32 byte), lalu kedua FE (build + `vite preview`, atau dev server; dipilih di plan).
  - Menjalankan `pnpm e2e` di CompanyPortal (`e2e/{auth,vendor-request,vendor-order}.spec.ts` dari FE E3).
  - Kredensial `E2E_*` dari GitHub Actions secrets, tidak ditulis di workflow.
  - Artefak trace/screenshot Playwright di-upload saat gagal.
- **FR-B6** Kalau test integration/Python gagal saat pertama dijalankan di DB yang bermigrasi penuh:
  - Masalah di test itu sendiri (setup, asumsi data): diperbaiki di fase ini.
  - Bug produk: STOP, laporkan, dicatat terpisah (lihat flag money di atas).
  - Test tidak boleh di-skip atau dihapus untuk membuat CI hijau.
- **FR-B7** `docs/deployment.md` #8 + "Pipeline": GitHub Actions = CI (lint/test/scan/E2E); Cloud Build = build image → push Artifact Registry (tag git SHA) → deploy.

### Fase C — Coverage (BE-01)
- **FR-C1** Coverage diukur dari unit + integration (job `go-integration`, `-cover` per paket `internal/*`). Angka baseline baru dicatat di `BE_debt.md` + `tests.md`.
- **FR-C2** CI gagal kalau coverage per paket turun di bawah ambang. Ambang awal = baseline terukur (dibulatkan ke bawah), lalu dinaikkan per slice.
  - File ambang: `backend/coverage-thresholds.txt` (`<paket> <persen>`).
  - Dicek dengan skrip shell/awk di CI, tanpa tool baru.
- **FR-C3** Target ≥ 80% untuk `internal/service`, `internal/approval`, `internal/rolemgmt`; `internal/repository` cukup lewat integration test (tanpa target 80% sendiri).
  - Prioritas test: maker-checker gates (maker ≠ checker), RBAC denial di service, money math (decimal string, kuota), applier stale/409, rolemgmt audit-in-tx rollback.
  - Ruang lingkup: keputusan S5.

### Fase D — Migrasi (BE-03)
- **FR-D1** Runner baru `backend/cmd/migrate` (Go, stdlib + pgx yang sudah ada). Subcommand:
  - `status`: daftar file + applied/pending + checksum cocok/tidak.
  - `up`: terapkan semua file pending.
  - `baseline --through NNN`: tandai file ≤ NNN sebagai applied tanpa menjalankan SQL-nya.
  - Membaca `DATABASE_URL`.
- **FR-D2** Aturan runner:
  - Hanya file `NNN_*.sql` di level atas `backend/migrations/` (bukan `archives/`), diurutkan numerik.
  - **Pending = setiap file yang belum tercatat**, termasuk nomor yang lebih kecil dari yang sudah applied. Ini supaya `022` yang dibuat belakangan tetap diterapkan.
  - Setiap file dijalankan apa adanya lewat `psql -v ON_ERROR_STOP=1 -f`, semantik sama dengan proses manual sekarang: file mengatur `BEGIN`/`COMMIT` sendiri, dan `006` memakai `CONCURRENTLY`. Baris `schema_migrations` ditulis hanya setelah psql sukses.
  - File yang sudah applied tetapi checksum SHA-256-nya berubah → runner berhenti dengan error, tidak menjalankan apa pun.
  - Satu run memegang `pg_advisory_lock` (kunci tetap) supaya dua run tidak berjalan bersamaan.
  - Gagal di file X → berhenti, file sesudahnya tidak dijalankan, exit code ≠ 0.
- **FR-D3** Dev di-baseline `--through 029`. File yang tidak ada (`022`, `024`) tidak ditandai. Runner tidak dijalankan otomatis saat API start; ini langkah deploy terpisah dari host operator yang punya `psql` (seperti sekarang), dijalankan di luar jam 07:00–20:00 (Sec 14).
- **FR-D4** Dokumen diperbarui:
  - CLAUDE.md Sec 2a (perintah migrasi) dan Sec 12 "Migration baseline".
  - `docs/deployment.md` (langkah migrasi saat deploy).
  - Tabel `schema_migrations` dicatat di Sec 3 (Core) setelah diterapkan.

### Fase E — Observability + ketahanan (BE-04, BE-08, BE-09)
- **FR-E1** `middleware.Logger` chi diganti logger request berbasis `slog` (JSON, satu baris per request, `msg="http_request"`).
  - Field: `method`, `route` (pola chi, mis. `/api/v1/vendor-requests/{id}`), `path`, `status`, `bytes`, `duration_ms`, `request_id`, `ip` (dari `r.RemoteAddr` sesudah `TrustedRealIP`).
  - **Tidak** dicatat: query string, header, body, token.
  - Level: 5xx = error, 4xx = warn, lainnya info.
  - Berlaku di `backend` dan `backend-cit`.
- **FR-E2** `GET /ready` (tanpa auth): ping primary, replica (kalau beda dari primary), dan Redis, timeout total 2 s.
  - Semua OK → 200 `{"status":"ok"}`.
  - Ada yang gagal → 503 `{"status":"unavailable","checks":{"db":"ok|fail","db_replica":"ok|fail","redis":"ok|fail"}}`, tanpa teks error. Detail error ke log.
  - `/health` tetap liveness (dipakai `HEALTHCHECK` Docker).
- **FR-E3** Middleware global di `backend`:
  - Batas body **1 MB** untuk semua route kecuali route upload yang sudah punya limit sendiri (master-data import, DSR upload). Body lebih besar → 413 atau 4xx dari decoder yang ada; server tidak membaca lebih dari batas.
  - Timeout request **30 s** (sama dengan `WriteTimeout`) lewat context, supaya query DB berhenti saat waktu habis atau client putus.
- **FR-E4** Pool DB diatur lewat parameter URL pgx (`pool_max_conns`, opsional `pool_min_conns`) di `DATABASE_URL` / `DATABASE_REPLICA_URL`.
  - Didokumentasikan di `backend/.env.example` + `docs/deployment.md` dengan angka usulan per environment.
  - Startup mencatat `max_conns` efektif per pool (satu baris log).
- **FR-E5** `docs/deployment.md`: definisi log-based metric Cloud Monitoring.
  - Distribusi `duration_ms` per `route` untuk p95.
  - Counter `status>=500` untuk error rate.
  - Filter `jsonPayload.msg="http_request"`.
  - Pembuatan metric/dashboard/alert di GCP dilakukan user (outstanding).

### Fase F — Python EOD (BE-05)
- **FR-F1** Semua `requirements.txt` di `backend_python/` di-pin `==` ke versi yang sekarang terpasang dan lulus test. `dmaa/` + `itm/cashpos/` diberi requirements sesuai import-nya, atau dirujuk ke file yang ada; dipilih di plan.
- **FR-F2** `python-jose` → `PyJWT` di `lib/dependencies.py` + `lib/test_eod_api.py`.
  - Algoritma dipaku `HS256`, `exp` wajib diverifikasi, klaim dan role yang dicek sama dengan sekarang.
  - Token tanpa algoritma (`alg: none`), algoritma lain, secret salah, dan kedaluwarsa → ditolak (test).
- **FR-F3** Dockerfile Python (keputusan S6): satu image untuk `eod_retry_scheduler` dan `service_dsr_etl` (keduanya berbagi `lib/`), service dipilih lewat `CMD`.
  - Base `python:<versi>-slim` dipin, non-root, `TZ=Asia/Jakarta` (prasyarat deploy Sec 14), `HEALTHCHECK` ke `/health`.
  - `FTP_DATA` lewat volume.
  - CI hanya mem-build image ini; wiring deploy di luar scope.

### Fase G — Pemeliharaan (BE-07, BE-10, BE-11)
- **FR-G1** `service/vendor_request_actions.go` (1210 baris) dan `service/masterdata_import.go` (862) dipecah menjadi file ≤ 800 baris di package yang sama.
  - Hanya memindahkan kode: tanpa ubah signature atau logika.
  - Test lulus tanpa diubah.
- **FR-G2** `backend-cit` dikeluarkan dari rencana deploy:
  - `docs/deployment.md` + CLAUDE.md Sec 10: hari ini satu VM menjalankan `backend` saja; `backend-cit` menyusul setelah rename ke `backend-exq` dan berisi.
  - Module tetap di `go.work` dan tetap diuji CI (FR-B1).
- **FR-G3** BE-10 (OpenAPI) dicatat *deferred* di `BE_debt.md`, dengan catatan "mulai dari modul baru berikutnya".

## Non-functional requirements
- **NFR1** Tidak ada dependency Go baru. Satu dependency Python baru: `PyJWT` (disetujui Q6). Action GitHub hanya dari `actions/*` + `golangci/golangci-lint-action`, dipin versi (S7).
- **NFR2** Tidak ada perubahan response shape atau endpoint bisnis. ATM `backend` tetap flat JSON; `/ready` mengikuti bentuk `/health`.
- **NFR3** Seluruh workflow CI ≤ 15 menit per run (target; dicatat aktual di `tests.md`).
- **NFR4** Rahasia CI hanya lewat GitHub secrets atau dibangkitkan per run; log CI tidak mencetak `DATABASE_URL`, JWT, atau password.
- **NFR5** Overhead logger + middleware per request diabaikan (< 1 ms).
- **NFR6** Small diffs, satu fase per commit/PR. Fase B–C bergantung pada A (govulncheck bersih). Fase D menggantikan loop psql di B. Fase lain independen.

## Data model
- **Tabel baru `schema_migrations`** (Core, dibuat oleh runner dengan `CREATE TABLE IF NOT EXISTS`, bukan file migrasi, supaya tidak ada ayam-dan-telur):
  - `version text PRIMARY KEY` (nama file, mis. `029_vendor_request_send.sql`)
  - `checksum text NOT NULL` (SHA-256 hex)
  - `applied_at timestamptz NOT NULL DEFAULT now()`
  - `applied_by text NOT NULL` (`current_user` DB)
  - `baseline boolean NOT NULL DEFAULT false` (true = ditandai lewat `baseline`, SQL-nya tidak dijalankan runner)
- Tidak ada perubahan pada tabel bisnis.

## API
- `GET /ready`: lihat FR-E2. Tanpa auth, di `backend` dan `backend-cit`.
- Semua endpoint lain tidak berubah. Request JSON > 1 MB dan request > 30 s sekarang ditolak (FR-E3); hari ini keduanya tidak dibatasi.

## Tests (trace ke FR)

| ID | Bukti | FR |
|---|---|---|
| T-A1 | `docker build` kedua backend + container sehat (`HEALTHCHECK` lulus) | A1 |
| T-A2 | `govulncheck ./...` ×3 module: 0 terjangkau | A2 |
| T-B1 | Run GitHub Actions hijau untuk semua job (setelah user push) | B1–B5 |
| T-B2 | Log job `go-integration`: test integration jalan (jumlah `SKIP` karena `DATABASE_URL` = 0) | B2 |
| T-B3 | Job `sqlc` gagal kalau `queries/*.sql` diubah tanpa generate (dicoba sekali di branch) | B3 |
| T-C1 | Coverage per paket ≥ ambang; `service`/`approval`/`rolemgmt` ≥ 80% (sesuai S5) | C1–C3 |
| T-D1 | Integration test runner di Postgres kosong: `up` menerapkan semua; `up` kedua = no-op; checksum diubah → error; `baseline --through` tidak menjalankan SQL; file nomor kecil yang muncul belakangan ikut diterapkan; dua run paralel → satu menunggu | D1–D2 |
| T-D2 | `status` di dev setelah baseline: 001–029 applied (tanpa 022/024), 0 pending | D3 |
| T-E1 | Unit test logger: field ada, query string tidak tercatat, level per status | E1 |
| T-E2 | Unit test `/ready`: 200 semua OK; 503 + checks saat DB/Redis mati, tanpa teks error | E2 |
| T-E3 | Unit test: body 1 MB + 1 byte di route JSON ditolak; route upload tetap menerima file besar sesuai limit-nya; handler yang menunggu ctx berhenti setelah timeout | E3 |
| T-F1 | `python -m unittest` lulus dengan versi ter-pin | F1 |
| T-F2 | Test negatif JWT Python: `alg none`, HS512/RS256, secret salah, kedaluwarsa → 401; token valid ADMIN → lulus | F2 |
| T-F3 | `docker build` image Python + `/health` 200 di container | F3 |
| T-G1 | `go test ./...` + integration lulus tanpa perubahan test; `wc -l` file service ≤ 800 | G1 |
| Manual (user, outstanding) | Baseline dev disetujui; log-based metric + dashboard di GCP; branch protection "required checks" di GitHub; deploy pertama dengan runner | D3, E5, B1 |

## Keputusan spec (minta konfirmasi PO)
- **S1 — Runner memanggil `psql`.** Usul: runner Go memegang lock + checksum + pencatatan, tetapi mengeksekusi file lewat `psql -f`.
  - Alasan: 23 file punya `BEGIN`/`COMMIT` sendiri dan `006` memakai `CONCURRENTLY`. Menjalankan file sebagai satu string lewat pgx akan membungkusnya dalam transaksi implisit, sehingga `006` gagal. psql = semantik persis seperti proses manual hari ini.
  - Konsekuensi: runner butuh `psql` di PATH (host operator + runner CI sudah punya); image API tidak berubah.
- **S2 — `schema_migrations` dibuat oleh runner**, bukan file `030_`. Alasan: tabel pencatat tidak boleh bergantung pada dirinya sendiri; tetap dicatat di Sec 3.
- **S3 — Urutan B sebelum D.** CI fase B membangun skema dengan loop `psql` sementara, lalu fase D menggantinya dengan runner. Alternatif (D dulu) membuat CI menunggu lebih lama.
- **S4 — Seed E2E.** DB yang dibangun dari migrasi saja tidak punya kelolaan ATM / paket cabang (`vendor_packages_branch`, `atm_vendor_packages` kosong; Sec 12), padahal alur Vendor Request butuh itu.
  - Usul: file `backend/testdata/e2e_seed.sql`, hanya dipakai job E2E, **tidak pernah** di `migrations/` dan tidak dijalankan runner.
  - Akun memakai user dev dari `002_baseline_seed.sql`; password lewat secrets.
- **S5 — Ruang lingkup 80%.** Usul: fase C di AI-DLC ini menutup gap untuk ketiga paket, per slice paket (`rolemgmt` → `approval` → `service`).
  - Titik keputusan di plan: kalau setelah diukur ulang (unit + integration) gap `service` masih > 15 poin, `service` dipisah ke AI-DLC lanjutan `be-coverage-service`, sementara di sini cukup ambang anti-turun.
- **S6 — Dockerfile Python dibuat sekarang** (satu image, dua `CMD`), hanya di-build di CI. Cara deploy Python (container vs langsung di VM) diputuskan saat deploy; Dockerfile tidak mengikat keputusan itu.
- **S7 — Action pihak ketiga.** Hanya `actions/checkout`, `actions/setup-go`, `actions/setup-python`, `actions/setup-node`, `actions/upload-artifact`, `golangci/golangci-lint-action`, dipin ke tag versi mayor. Tool lain (`govulncheck`, `sqlc`) dipasang dengan `go install …@<versi tetap>`.
- **S8 — Batas body & timeout.** 1 MB dan 30 s. 30 s = `WriteTimeout` server yang sudah berlaku, jadi request > 30 s hari ini memang sudah gagal menulis respons; yang berubah hanya query DB ikut dibatalkan.

## Di luar scope
- OpenAPI (BE-10, deferred); tracing / OpenTelemetry / Prometheus; distroless.
- Pemecahan package per domain (layering flat adalah keputusan terdokumentasi).
- File `cloudbuild.yaml` dan wiring deploy (termasuk deploy Python); rename `backend-cit` → `backend-exq` (dikerjakan user).
- Coverage FE (FE-13); TLS.
- Perbaikan bug produk yang ditemukan test (dicatat terpisah, FR-B6).
- Branch protection GitHub (pengaturan akun, dilakukan user).
