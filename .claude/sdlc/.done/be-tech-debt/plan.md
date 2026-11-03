# Plan: Perbaikan tech debt backend (BE-01 – BE-12)

Status: **accepted 2026-10-15, all phases** (draft 2026-10-12): Fase A (A1–A3) **accepted 2026-10-15** (user: "Fase A plan be-tech-debt (A1, A2, A3) diterima"), Fase B (B1–B6, B5 ditunda) **accepted 2026-10-15** (user: "Saya terima Fase B plan BE (B1–B6, B5 ditunda). Boleh buat dan hapus DB cms_ci di Postgres dev."); Fase C–G **accepted 2026-10-15** (user: "accept Fase C–G of the BE plan"). Stage: 3 Build.
Sumber: `spec.md` (accepted 2026-10-12, S1–S8 sesuai usulan), `intent.md` (accepted 2026-10-12).
Singkatan path: `BE/` = `backend/`, `PY/` = `backend_python/`, `CI` = `.github/workflows/ci.yml`.

## Titik sentuh yang sudah dicek di kode (2026-10-12)
- **Bootstrap**: `BE/cmd/api/main.go:99-112`:
  - Urutan middleware: `RequestID` → `TrustedRealIP` → chi `middleware.Logger` (teks) → `Recoverer`.
  - `/health` statis.
  - Pool: `pgxpool.New` di baris 47 (primary) dan 66 (replica, opsional; tanpa replica `dbReadPool = dbPool`). Redis di baris 88.
  - `http.Server` `ReadTimeout` 10 s, `WriteTimeout` 30 s.
  - `backend-cit/cmd/api/main.go:62-66`: urutan sama tanpa `TrustedRealIP`, tanpa replica.
- **`pkg/`**: `auth`, `config`, `middleware`, `response`. `pkg/go.mod` sudah me-require go-redis + miniredis, tapi tidak pgx. Jadi readiness memakai closure `func(ctx) error`, tanpa import pgx di `pkg`.
- **Upload dengan limit sendiri**, satu-satunya pemakai `MaxBytesReader`:
  - `POST /api/v1/dsr/uploads` (`dsr_upload_handler.go:92`)
  - `POST /api/v1/admin/master-data/import/{entity}/dry-run|confirm` (`admin_master_data_import_handler.go:77`)
  - Handler JSON lain men-decode `r.Body` sendiri. Body yang melewati batas → error decode → 4xx yang sudah ada.
- **Migrasi**:
  - 23 file punya `BEGIN`/`COMMIT` sendiri; `006` memakai `CONCURRENTLY`.
  - `.gitattributes` memaksa `eol=lf` (`029` = `i/lf w/lf`), jadi checksum sama di Windows dan CI tanpa normalisasi.
  - `BE/Dockerfile` sudah meng-`COPY migrations`, tidak dipakai runner (S1: runner jalan dari host).
- **Test integration**:
  - 29 file membaca `DATABASE_URL`, dan mengandaikan skema sudah ada.
  - Redis di test = miniredis (`handler/*_test.go`), jadi job integration tidak butuh service Redis.
- **Tool lokal**: golangci-lint 2.13.0, govulncheck v1.8.0, Go toolchain 1.26.9 (`go.work`: `go 1.25.0` + `toolchain go1.26.9`), psql client 18.3, Node 24 lokal (FE CI memakai 22), Python 3.14.7.
- **Python terpasang** (lulus test hari ini): fastapi 0.141.1, uvicorn 0.52.4, pydantic 2.13.5, pydantic-settings 2.15.0, asyncpg 0.31.0, APScheduler 3.11.3, python-dotenv 1.0.1, pandas 3.0.5, psycopg(-binary) 3.3.4, openpyxl 3.1.5, xlrd 2.0.2, python-jose 3.5.0.
  - `PY/dmaa/` dan `PY/itm/cashpos/` meng-import `pandas`, `psycopg`, `dotenv` tanpa requirements sendiri.
  - `PY/dmaa/bak/` tidak di-import siapa pun.
- **File service besar**:
  - `service/vendor_request_actions.go` (1210): query baca baris 40–376, create + penomoran 377–733, transisi + helper 734–1210.
  - `service/masterdata_import.go` (862): pembaca CSV baris 116–258, validator field 676–862.

## Slices (urutan kerja; satu commit per slice, commit hanya saat user bilang "commit")

### Fase A — Supply chain

**A1 — Base image Alpine (FR-A1)**
- Cek versi Alpine terbaru yang masih didukung di alpinelinux.org/releases (web) saat slice dikerjakan. `FROM alpine:3.20` → `alpine:3.N` di `BE/Dockerfile` + `backend-cit/Dockerfile`.
- Bukti: T-A1, yaitu `docker build` keduanya, lalu `docker run` + `docker inspect --format '{{.State.Health.Status}}'` = `healthy` (butuh `.env` dev; dijalankan bila Docker + DB tersedia, kalau tidak dicatat outstanding).

**A2 — `x/crypto` + govulncheck (FR-A2)**
- `cd backend && go get golang.org/x/crypto@v0.56.0 && go mod tidy`. Ulangi di `pkg`/`backend-cit` bila ikut me-require (indirect).
- `BE_debt.md` BE-12: GO-2026-5932 dicatat *accepted* (tanpa fix, tidak terjangkau).
- Bukti: T-A2 (`govulncheck ./...` ×3) + `go test ./...` ×3.

**A3 — Hapus `PY/dmaa/bak/` (FR-A3)**
- `git rm -r backend_python/dmaa/bak`. Sebelumnya dicek ulang dengan grep `bak` di import.
- Bukti: Python unittest tetap lulus.

### Fase B — CI (bergantung pada A; file `CI` dipakai bersama FE E1)
Siapa pun yang lebih dulu dikerjakan (FE E1 atau B1) membuat `CI` dengan trigger `push`/`pull_request` ke `main`, `dev1` dan `permissions: contents: read`. Yang berikutnya hanya menambah job.

**B1 — Job `go` (FR-B1)**
- Matrix `module: [backend, backend-cit, pkg]`, `working-directory: ${{ matrix.module }}`.
- `actions/setup-go` dengan `go-version-file: go.work`.
- Langkah:
  1. `go vet ./...`
  2. `golangci/golangci-lint-action` versi `v2.13.0`, `args: --build-tags integration`, config root.
  3. `go run golang.org/x/vuln/cmd/govulncheck@v1.8.0 ./...`
  4. `go test -race ./...`
- Bukti: lokal, perintah yang sama ×3 hijau. GitHub: T-B1 setelah user push.

**B2 — Job `go-integration` (FR-B2, S3)**
- Versi major Postgres = dev. Saat slice dikerjakan, **minta user** menjalankan `SHOW server_version` di dev (Claude tidak membaca `.env`). Image `postgres:<major>`, `POSTGRES_DB=cms`, healthcheck `pg_isready`.
- Skrip `BE/scripts/apply-migrations.sh` (sementara, dihapus di D3):
  - `for f in $(ls migrations/[0-9][0-9][0-9]_*.sql | sort); do psql -v ON_ERROR_STOP=1 -X -q -f "$f"; done`
  - Koneksi lewat env `PG*`, tidak di argv.
- Lalu `go test -tags integration -race -count=1 ./...` dengan `DATABASE_URL`.
- Langkah verifikasi: `go test -tags integration -v ./... | grep -c "DATABASE_URL not set"` harus 0.
- **Uji lokal dulu**: buat DB kosong `cms_ci` di Postgres dev (izin user, karena ini membuat database baru), jalankan skrip + test, catat hasilnya. Kegagalan ditangani per FR-B6. DB `cms_ci` dihapus setelahnya atas izin user.
- Bukti: T-B2.

**B3 — Job `sqlc` (FR-B3)**
- `go install github.com/sqlc-dev/sqlc/cmd/sqlc@v1.31.1` → `cd backend && sqlc generate`
- → `sed -i 's/UserLeafe/UserLeave/g' internal/db/*.go` → `git diff --exit-code -- internal/db`.
- Bukti: lokal sama (diff kosong). T-B3: satu kali ubah `queries/*.sql` tanpa generate di branch uji → job gagal (dicatat di `tests.md`, tidak di-commit).

**B4 — Job `python` (FR-B4)**
- `actions/setup-python` 3.14 + Postgres service yang dimigrasi dengan skrip B2.
- `pip install -r` untuk semua requirements `PY/`.
- `cd backend_python`, lalu:
  - `python -m unittest discover -s dsr`
  - `python -m unittest discover -s itm/cashpos`
  - `python -m unittest lib.test_import_jobs lib.test_eod_api -v`
- `DATABASE_URL` ke Postgres job (localhost).
- Bukti: lokal perintah sama hijau; jumlah skip karena `DATABASE_URL` = 0 di CI.

**B5 — Job `e2e` (FR-B5, S4)** — bergantung pada FE E3 (spec Playwright). Kalau FE E3 belum ada, slice ini ditunda dan dicatat.
- `BE/testdata/e2e_seed.sql`: data minimum untuk alur di `CP/e2e/*.spec.ts`, idempotent (`ON CONFLICT DO NOTHING`), tanpa password baru (memakai user `002`).
  - Satu vendor FLM + cabang ber-`region_code` + vault.
  - ATM ber-kelolaan aktif + paket.
  - Baris `dmaa_files`/`dmaa_atm_forecast` untuk tanggal relatif hari run.
  - Area ACM + anggota.
  - Satu Vendor Request siap `sent_to_vendor` untuk alur terima/tolak vendor.
  - Isi tepatnya diturunkan dari spec E2E saat slice dikerjakan.
- Job:
  1. Services Postgres + Redis.
  2. Migrasi (skrip B2) + `psql -f testdata/e2e_seed.sql`.
  3. `JWT_SECRET=$(openssl rand -hex 32)`.
  4. `go build ./cmd/api` → jalan di background, tunggu `/health`.
  5. Kedua FE: `pnpm install --frozen-lockfile && pnpm build && pnpm preview` di port 5173/5174 dengan proxy `/api` ke :8080. Kalau `preview` tidak memakai proxy dev, pakai `pnpm dev`; dipilih saat slice.
  6. `pnpm exec playwright install --with-deps chromium` → `pnpm e2e`.
  7. `actions/upload-artifact` untuk `playwright-report/` + `test-results/` saat gagal.
- Secrets `E2E_*` dibuat user di GitHub repo settings (outstanding).
- Bukti: T-B1 (job e2e hijau) setelah user push + set secrets.

**B6 — Dokumen CI vs Cloud Build (FR-B7)**
- `.claude/docs/deployment.md` #8 + "Pipeline": GitHub Actions = CI; Cloud Build = build image → push AR (tag SHA) → deploy.
- `BE_debt.md` BE-02 → `in-progress`/`done`.

### Fase C — Coverage (bergantung pada B2)

**C1 — Ukur + ambang anti-turun (FR-C1, FR-C2)**
- Lokal (DB dev atau `cms_ci`): `go test -tags integration -count=1 -cover ./internal/...` → catat persen per paket ke `BE_debt.md` (baseline baru) + `tests.md`.
- `BE/coverage-thresholds.txt`: `<import path> <persen dibulatkan ke bawah>` untuk semua paket `internal/*` yang punya test.
- `BE/scripts/check-coverage.sh`: parse baris `ok <pkg> ... coverage: N%`; gagal bila `N < ambang` atau paket di file tidak muncul. Dipanggil di job `go-integration` (output `go test` di-`tee`).
- **Titik keputusan S5**: bila `service` < 65% (gap > 15 poin), laporkan ke user dan usulkan memecah `service` ke AI-DLC `be-coverage-service`.

**C2 — `rolemgmt` → ≥ 80%** (prioritas: grant/revoke + audit di tx yang sama, rollback saat audit gagal, re-check `APPACCESS`/`ADMIN` di service).
**C3 — `approval` → ≥ 80%** (maker ≠ checker, actor = approver efektif, delegasi/cuti, policy berjenjang, reject).
**C4 — `service` → ≥ 80%** (bila tidak dipecah). Satu sub-slice per area, mulai dari yang paling berisiko:
  1. `masterdata_approval` (apply stale/409)
  2. `vendor_request` (transisi + completion + kuota)
  3. vault plan / vendor order
  4. sisanya
- Setiap slice C2–C4: tulis test (table-driven, mock repo untuk unit; integration untuk jalur DB), naikkan ambang di `coverage-thresholds.txt`. Bug produk yang ditemukan → STOP (flag money).
- Bukti: T-C1.

### Fase D — Runner migrasi (bergantung pada B2)

**D1 — `BE/cmd/migrate` (FR-D1, FR-D2, S1, S2)**
- File:
  - `cmd/migrate/main.go`: subcommand `status|up|baseline --through NNN`, flag `-dir` default `migrations`.
  - `cmd/migrate/runner.go`: logika.
  - `cmd/migrate/runner_integration_test.go`: `//go:build integration`.
  - Tidak ada paket `internal/` baru.
- Alur:
  1. pgx connect (`DATABASE_URL`).
  2. `pg_advisory_lock(hashtext('cms.schema_migrations'))` di koneksi itu.
  3. `CREATE TABLE IF NOT EXISTS schema_migrations (...)` (kolom sesuai spec).
  4. Baca file `^\d{3}_.*\.sql$` di level atas `-dir`, urut nama, SHA-256 isi file.
  5. Bandingkan dengan tabel: checksum beda → error keluar ≠ 0 sebelum menjalankan apa pun.
  6. `up`: untuk setiap file belum tercatat, jalankan `exec.CommandContext("psql", "-v", "ON_ERROR_STOP=1", "-X", "-q", "-f", path)`, lalu `INSERT` baris (`applied_by = current_user`).
     - Env anak: `PGHOST`/`PGPORT`/`PGUSER`/`PGPASSWORD`/`PGDATABASE`/`PGSSLMODE` dari `pgx.ParseConfig` (password tidak di argv atau log).
     - Gagal → stop, exit ≠ 0, stderr psql diteruskan.
  7. `baseline`: `INSERT ... baseline=true` untuk file ≤ NNN yang ada di dir dan belum tercatat.
  8. `status`: tabel teks (`version`, `applied|pending`, `checksum ok|MISMATCH`, `baseline`).
- Test (DB sementara via `CREATE DATABASE` di `DATABASE_URL` admin, dir migrasi sementara `t.TempDir()`): semua kasus T-D1. Lock diuji dengan dua goroutine `up` → keduanya sukses, file hanya diterapkan sekali.
- Bukti: T-D1 + `golangci-lint` (gosec G204 untuk `exec`: argumen konstan + path dari dir sendiri, beri `//nolint:gosec` dengan alasan bila perlu).

**D2 — Baseline dev (FR-D3)** — tulis ke DB dev, **minta izin user di langkah ini**.
- `go run ./cmd/migrate baseline --through 029` → `status`: 001–029 applied (tanpa 022/024), 0 pending.
- Bukti: T-D2 (output `status` di `tests.md`).

**D3 — CI pakai runner + dokumen (FR-D4)**
- Job `go-integration`/`python`/`e2e`: `go run ./cmd/migrate up` menggantikan `scripts/apply-migrations.sh` (dihapus).
- CLAUDE.md:
  - Sec 2a: perintah migrasi `cd backend && go run ./cmd/migrate status|up`; `psql -f` manual tidak dipakai lagi.
  - Sec 3 Core: `schema_migrations`.
  - Sec 12 "Migration baseline": runner + baseline dev sampai 029.
- `deployment.md`: langkah migrasi saat deploy (host operator, di luar 07:00–20:00, `status` → `up` → `status`).

### Fase E — Observability + ketahanan

**E1 — Logger request `slog` (FR-E1)**
- `pkg/middleware/request_logger.go` + `request_logger_test.go`.
  - `middleware.NewWrapResponseWriter` (chi) untuk status/bytes.
  - Sesudah `next.ServeHTTP`: `chi.RouteContext(r.Context()).RoutePattern()`, lalu `slog.LogAttrs` dengan level per status.
  - Tidak mencatat `/health` dan `/ready` yang 2xx (healthcheck Docker tiap 10 s).
- Kedua `main.go`: `middleware.Logger` → `custommw.RequestLogger`.
- Bukti: T-E1.

**E2 — `/ready` (FR-E2)**
- Paket baru `pkg/health`: `ready.go` = `Ready(timeout time.Duration, checks map[string]func(context.Context) error) http.HandlerFunc` + test. Isinya:
  - cek paralel
  - status per nama
  - 200/503
  - error di-`slog.Warn`, tidak dikirim ke client
- `BE` main: `db`, `db_replica` (hanya bila `dbReadPool != dbPool`), `redis`. `backend-cit`: `db`, `redis`.
- Bukti: T-E2.

**E3 — Batas body + timeout (FR-E3, S8)**
- `pkg/middleware/body_limit.go`: `BodyLimit(max int64, exempt func(*http.Request) bool)` membungkus `http.MaxBytesReader` bila tidak dikecualikan. Plus test.
- `BE` main:
  - Pengecualian: `POST /api/v1/dsr/uploads` dan prefix `/api/v1/admin/master-data/import/`.
  - Urutan: `RequestID` → `TrustedRealIP` → `RequestLogger` → `Recoverer` → `middleware.Timeout(30*time.Second)` (chi) → `BodyLimit(1<<20, isUpload)`.
- `backend-cit`: `Timeout` + `BodyLimit` tanpa pengecualian.
- Test handler: body 1 MB + 1 byte ke route JSON (mis. `PUT /api/v1/admin/roles/...` lewat router uji) → 4xx. Upload > 1 MB tetap diterima.
- Bukti: T-E3.

**E4 — Pool DB (FR-E4)**
- `BE/.env.example`: contoh `?pool_max_conns=20` (+ komentar untuk replica).
- `deployment.md`: usulan per environment, dengan rumus `(max_connections Cloud SQL − cadangan) / jumlah instance`.
- Main: satu `slog.Info("db pool", "pool", "primary", "max_conns", dbPool.Config().MaxConns)` per pool.
- Bukti: log startup lokal.

**E5 — Log-based metric (FR-E5)**
- `deployment.md`: filter `jsonPayload.msg="http_request"`; metric distribusi `duration_ms` label `route`; counter `status>=500`; contoh alert p95 > 3 s.
- Pembuatan di GCP: user (outstanding).

### Fase F — Python EOD

**F1 — Pin requirements (FR-F1)**
- Keempat `requirements.txt`: `>=` → `==` sesuai versi terpasang di atas (`uvicorn[standard]`, `psycopg[binary]` dipertahankan extras-nya).
- Baru: `PY/dmaa/requirements.txt` (`pandas`, `psycopg[binary]`, `openpyxl`, `python-dotenv`) dan `PY/itm/cashpos/requirements.txt` (`pandas`, `psycopg[binary]`, `python-dotenv`), di-pin sama.
- Bukti: T-F1.

**F2 — PyJWT (FR-F2)** — **minta izin user sebelum `py -3 -m pip install PyJWT`** (unduh paket).
- `lib/dependencies.py`: `jwt.decode(token, secret, algorithms=["HS256"], options={"require": ["exp"]})`; `jwt.InvalidTokenError` menggantikan `JWTError`. Klaim dan role yang dicek tidak berubah.
- `lib/test_eod_api.py`: pembuatan token → PyJWT. Tambah test: `alg none`, HS512, RS256 (header dipalsukan), secret salah, kedaluwarsa → 401; ADMIN valid → 200.
- requirements kedua service: `python-jose[cryptography]` → `PyJWT==<versi terpasang>`.
- Bukti: T-F2.

**F3 — Dockerfile Python (FR-F3, S6)**
- `PY/Dockerfile`:
  - `python:3.14-slim` (dipin minor)
  - `pip install --no-cache-dir` semua requirements
  - user non-root, `TZ=Asia/Jakarta`
  - `WORKDIR /app` (= `backend_python/`)
  - `CMD ["python","-m","eod_retry_scheduler.run"]`; DSR via override `python -m service_dsr_etl.run`
  - `HEALTHCHECK` memakai `python -c "urllib.request.urlopen(...)"`, karena slim tanpa wget/curl; port dari env.
- `PY/.dockerignore`: `.env*`, `__pycache__`, `*/backup/`, test.
- `CI`: job `python-image` → `docker build`.
- Bukti: T-F3 (lokal bila Docker tersedia).

### Fase G — Pemeliharaan

**G1 — Pecah file service (FR-G1)**
- `vendor_request_actions.go` → `vendor_request_queries.go` (baris 40–376: browse/summary/options/list/get/audit) + `vendor_request_create.go` (377–733: create + validasi + penomoran). Sisa transisi + helper tetap (~480 baris).
- `masterdata_import.go` → `masterdata_import_csv.go` (116–258: pembaca CSV + span) + `masterdata_import_validate.go` (676–862: validator field).
- Hanya memindahkan blok; import per file dirapikan `goimports`.
- Bukti: T-G1 (`go test ./...` + integration + lint hijau, test tidak diubah, `wc -l` ≤ 800).

**G2 — `backend-cit` keluar dari deploy (FR-G2)**
- `deployment.md` (tabel produk #2, "Current state", "Pipeline") + CLAUDE.md Sec 10: hari ini satu VM menjalankan `backend` saja.
- Rujukan root `docker-compose.yml` yang tidak ada → diganti `backend/docker-compose.yaml`.
- `BE_debt.md` BE-11 `done`.

**G3 — OpenAPI deferred (FR-G3)**
- `BE_debt.md` BE-10: Status `deferred (be-tech-debt Q8)`, mulai dari modul baru berikutnya.

## Bukti selesai (per slice dan di akhir)
- Lokal per module: `go vet`, `golangci-lint run --build-tags integration ./...`, `govulncheck ./...`, `go test -race ./...`, dan `go test -tags integration ./...` dengan `DATABASE_URL`. Python: unittest dengan `DATABASE_URL`.
- GitHub Actions: semua job hijau setelah user push (T-B1).
- `graphify update .` setelah perubahan kode (CLAUDE.md 1a).
- Status BE-01 … BE-12 di `BE_debt.md` diperbarui per fase (`done <sha>` setelah commit). README sdlc + `development-progress.md` diperbarui per stage.
- Outstanding untuk user:
  - secrets `E2E_*` + branch protection di GitHub
  - izin baseline dev (D2)
  - log-based metric/dashboard di GCP (E5)
  - deploy pertama dengan runner

## Risiko
- **Test integration yang lama tidak jalan bisa gagal di DB bermigrasi penuh** (asumsi data dev, urutan, isolasi). Mitigasi: B2 diuji lokal di `cms_ci` dulu; perbaikan test di fase B, bug produk STOP (FR-B6). Estimasi fase B bisa melebar karena ini.
- **Data dari migrasi berbeda dari dev.** `010`/`014` mengisi data, dan seed `024` sudah diarsip. Test yang diam-diam bergantung pada data dev akan gagal di CI. Mitigasi: sama dengan di atas; data yang dibutuhkan test dibuat test itu sendiri.
- **`middleware.Timeout` chi menulis 504 hanya bila handler tidak menulis.** Stream export CSV yang sudah mulai menulis tidak terpotong rapi. Mitigasi: `WriteTimeout` 30 s sudah membatasi hal yang sama hari ini; tidak ada perubahan perilaku untuk export.
- **Pengecualian upload berbasis path** harus ikut diperbarui bila route upload baru ditambah. Mitigasi: komentar di `isUpload` + test yang menyebut kedua route; dicatat di CLAUDE.md Sec 12 bersama E3.
- **Runner bergantung pada `psql`** di host. Mitigasi: `status`/`up` memeriksa `exec.LookPath("psql")` di awal dengan pesan jelas.
- **Versi Postgres CI ≠ Cloud SQL prod** bila dev pun berbeda dari prod. Mitigasi: catat di `deployment.md`; samakan saat Cloud SQL dibuat.
- **CI belum bisa dibuktikan sebelum push** (sama dengan FE E1). Workflow divalidasi oleh run GitHub setelah user push.
- **Fase C bisa besar.** Titik keputusan S5 di C1.

## Alternatif yang tidak diambil
- golangci-lint/govulncheck lewat `go install` di job (lebih lambat, tanpa cache) → action resmi untuk lint, `go run …@versi` untuk govulncheck.
- Runner mengeksekusi SQL lewat pgx (gagal untuk `006`, S1); golang-migrate (dependency baru, ganti nama file; Q2).
- Prometheus/OTel (Q5); distroless (Q7).
- Body limit berdasarkan `Content-Type` (bisa diakali dengan header multipart palsu ke route JSON) → pengecualian per path.

## Di luar plan
- OpenAPI, tracing, `cloudbuild.yaml` + wiring deploy (termasuk Python), rename `backend-exq`, coverage FE (FE-13), TLS.
- Bug produk yang ditemukan test; branch protection GitHub.
