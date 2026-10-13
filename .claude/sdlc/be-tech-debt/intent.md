# Intent: Perbaikan tech debt backend (BE-01 – BE-12)

Author: user (product owner), ditulis bersama Claude. Status: **accepted 2026-10-12** (user: *"setuju semua saran Claude, lanjut spec"*). Jawaban di bagian "Jawaban PO" di bawah.
Stage: 1 Plan selesai. Stage berikut: [`spec.md`](./spec.md).
Asal: review arsitektur BE 2026-10-12 → backlog [`.claude/BE_debt.md`](../../BE_debt.md). Pasangan: [`sdlc/fe-tech-debt/`](../fe-tech-debt/intent.md) (plan accepted; job CI frontend + E2E bergantung pada item ini).
Menyentuh Sec 4 #7:
- **data migrations** (BE-03: cara migrasi diterapkan + tabel pencatat baru)
- **auth** (BE-05: library JWT di Python EOD; BE-08: middleware request global)
- **money / maker-checker** (BE-01: test di `service`, `approval`, `rolemgmt`)

Maka item ini wajib full chain. Tidak ada perubahan perilaku bisnis, response shape, atau endpoint publik.

## Problem
Kata user (2026-10-12): *"buat sdlc untuk perbaikan di .claude\BE_debt.md"*.

Arsitektur BE sudah matang: Go 1.26 + chi + pgx + sqlc, layering rapi, maker-checker + audit dalam satu tx, golangci-lint (+gosec) 0 issue, govulncheck 0 vuln terjangkau. Masalahnya ada di operasional dan jaminan kualitas. Kondisi saat ini sudah dicek di kode (detail per item ada di `BE_debt.md`):

- **Jaminan kualitas**
  - Coverage unit di modul uang dan maker-checker di bawah 80%: `service` 44,6%, `approval` 54,2%, `rolemgmt` 35,8%, `repository` 15,4%. (BE-01)
  - 55 file test `integration` ter-skip karena tidak ada yang menjalankannya dengan `DATABASE_URL`.
  - Belum ada CI; lint, test, vuln scan, dan `sqlc diff` dijalankan manual. (BE-02)
- **Migrasi**: SQL diterapkan manual lewat `psql -f`, tanpa catatan migrasi mana yang sudah jalan di environment mana. Risiko drift antara dev, staging, dan prod. (BE-03)
- **Observability** (BE-04)
  - Tidak ada metrics/tracing, jadi NFR p95 ≤ 3 s dan 300 user tidak bisa diukur.
  - Log request chi berbentuk teks, bercampur dengan `slog` JSON.
  - `/health` hanya liveness; tidak ada readiness yang mengecek DB/Redis.
- **Python EOD** (BE-05)
  - Dependency di-pin dengan `>=` dan tanpa lockfile.
  - `python-jose` (dipakai di `lib/dependencies.py` untuk auth JWT `eod_retry_scheduler`) minim maintenance dan punya riwayat CVE.
  - Belum ada Dockerfile.
- **Supply chain**
  - Runtime image `alpine:3.20` sudah EOL, dipakai di `backend/Dockerfile` dan `backend-cit/Dockerfile`. (BE-06)
  - `golang.org/x/crypto` v0.54.0 punya 4 vuln (GO-2026-6355, -6354, -6303, -5932), tidak terjangkau dari kode kita; perbaikannya ada di v0.56.0, kecuali -5932 yang belum ada fix. (BE-12)
- **Ketahanan**
  - Belum ada limit body JSON global dan timeout per request. (BE-08)
  - Pool DB memakai default `max(4, NumCPU)`. (BE-09)
- **Pemeliharaan**
  - File service besar: `vendor_request_actions.go` 1210 baris, `masterdata_import.go` 862 baris. (BE-07)
  - Belum ada kontrak OpenAPI. (BE-10)
  - `backend-cit` masih kosong tapi dijadwalkan deploy. (BE-11)
  - `backend_python/dmaa/bak/` masih ter-track di git. (BE-12)

Konflik dokumen yang perlu diselesaikan: `docs/deployment.md` (#8 + "Pipeline") menyebut **Cloud Build** sebagai CI, sedangkan PO memutuskan **GitHub Actions** untuk CI (FE intent Q5).

## Proposed outcome
1. Setiap push/PR ke `main`/`dev1` menjalankan CI backend di workflow `.github/workflows/ci.yml` yang sama dengan FE:
   - `go vet`, golangci-lint (`--build-tags integration`), `go test` dengan Postgres sungguhan (integration test ikut jalan), `govulncheck`, dan pengecekan `sqlc` (kode generated sinkron dengan query).
   - Python unittest (termasuk integration test-nya).
   - Job E2E FE (keputusan S3 di `fe-tech-debt`).
2. Coverage modul uang/maker-checker diukur dari unit + integration, lalu dinaikkan menuju 80%. CI memasang ambang yang mencegah coverage turun.
3. Setiap environment tahu persis migrasi mana yang sudah diterapkan. Menerapkan migrasi berikutnya adalah satu langkah yang bisa diulang tanpa efek ganda.
4. Backend punya log request JSON terstruktur (`slog`), endpoint readiness (DB + Redis), dan metrics latency/error per route yang bisa dibaca Cloud Monitoring.
5. Layanan Python EOD bisa di-build ulang dengan hasil yang sama (dependency di-pin), memakai library JWT yang terawat, dan punya Dockerfile.
6. Image runtime memakai base yang masih didukung; `x/crypto` dinaikkan ke versi yang sudah diperbaiki.
7. Request JSON dibatasi ukurannya dan punya batas waktu. Pool DB diatur eksplisit dan terdokumentasi.
8. File service > 800 baris dipecah, kontrak API mulai terdokumentasi, `backend-cit` tidak ikut di-deploy selama masih kosong, dan file usang dihapus.

## Affected users and systems
- **Users**: tidak ada perubahan tampilan atau alur. Dampak tidak langsung: error 413/timeout yang jelas untuk request yang terlalu besar atau lambat, dan deploy yang lebih aman.
- **Systems**:
  - `backend/cmd/api/main.go` (middleware, health/readiness, metrics), `backend/internal/service/*` (pemecahan file + test), `backend/internal/{approval,rolemgmt,repository}` (test).
  - `backend/migrations/` + mekanisme menjalankannya (BE-03).
  - `backend/Dockerfile`, `backend-cit/Dockerfile`, `go.mod`/`go.sum` ketiga module.
  - `backend_python/` (requirements, `lib/dependencies.py`, Dockerfile baru).
  - `.github/workflows/ci.yml` (job backend + E2E), `.env.example`, `.claude/docs/deployment.md`.

## Constraints
- Stack tetap (CLAUDE.md Sec 2). Library baru, alat migrasi, client metrics, dan library JWT Python butuh approval (Sec 4 #4). Pilihan stdlib atau yang sudah terpasang diutamakan.
- ATM `backend` tetap flat JSON (Sec 3/5); tidak ada perubahan response shape atau endpoint publik.
- Migrasi yang sudah diterapkan di dev (`001`–`029`, kecuali `022` yang direservasi) **tidak boleh dijalankan ulang** oleh mekanisme baru; dev harus bisa ditandai "sudah sampai 029" tanpa menjalankan SQL-nya lagi.
- Small diffs, satu concern per perubahan; dikerjakan per fase.
- Heavy work tidak berjalan di jam kantor 07:00–20:00 (Sec 14). Ini relevan bila migration runner dijalankan saat deploy.
- Manual verification yang butuh environment nyata (deploy, Cloud Monitoring) dilakukan user dan dicatat outstanding.

## Open questions (untuk PO / spec)
1. **Scope & urutan**: semua 12 item dalam satu AI-DLC, dibagi fase? Saran Claude: **ya**.
   - Fase A supply chain (BE-06, BE-12)
   - Fase B CI (BE-02), sekaligus melengkapi job E2E FE
   - Fase C coverage (BE-01, butuh CI)
   - Fase D migrasi (BE-03)
   - Fase E observability + ketahanan (BE-04, BE-08, BE-09)
   - Fase F Python (BE-05)
   - Fase G pemeliharaan (BE-07, BE-10, BE-11)
2. **Mekanisme migrasi (BE-03)**: opsi (a) runner kecil berbasis stdlib + tabel `schema_migrations` (nama file + checksum + waktu); opsi (b) golang-migrate (dependency baru, format `up/down`, berarti mengganti nama file). Saran Claude: **(a)**. Formatnya tetap seperti sekarang, tanpa dependency, dan dev ditandai "baseline sampai 029". Runner dijalankan manual saat deploy (bukan otomatis saat API start) atau otomatis? Saran Claude: **manual / langkah deploy terpisah**.
3. **Target coverage (BE-01)**: 80% untuk paket mana? Saran Claude: hitung dari unit + integration; wajib ≥ 80% untuk `service`, `approval`, `rolemgmt`; `repository` cukup lewat integration; ambang CI dinaikkan bertahap seperti FE (mulai dari baseline). Ini butuh banyak test baru. Apakah tetap satu fase dalam AI-DLC ini, atau target 80% dipisah menjadi AI-DLC sendiri sementara di sini cukup "ukur + pasang ambang anti-turun"?
4. **CI vs Cloud Build**: GitHub Actions untuk CI (test/lint/scan), sedangkan build image + push Artifact Registry + deploy tetap Cloud Build seperti `docs/deployment.md`? Atau semuanya pindah ke GitHub Actions? Saran Claude: **GitHub Actions = CI, Cloud Build = build/deploy**; `docs/deployment.md` diperbarui sesuai keputusan.
5. **Metrics (BE-04)**: (a) `prometheus/client_golang` (dependency baru, standar industri, di-scrape Ops Agent/Managed Prometheus); (b) OpenTelemetry (dependency lebih besar, juga tracing); (c) tanpa dependency: log request terstruktur (latency, status, route) yang diubah menjadi log-based metrics di Cloud Monitoring. Saran Claude: **(c) dulu**, karena cukup untuk mengukur p95 dan error rate tanpa library baru. Tracing ditunda.
6. **Library JWT Python (BE-05)**: ganti `python-jose` dengan `PyJWT` (dependency baru, terawat) dan pin semua versi dengan lockfile (`pip-tools`/`requirements.lock`)? Saran Claude: **ya, PyJWT + file requirements yang di-pin (`==`)**, tanpa tool lock tambahan. Dockerfile Python ikut dibuat di sini, atau menunggu keputusan cara deploy Python (container vs langsung di VM)?
7. **Base image (BE-06)**: `alpine` versi terbaru yang didukung atau `gcr.io/distroless/static` (lebih kecil, tanpa shell; `HEALTHCHECK` berbasis `wget` harus diganti, misalnya binary memeriksa dirinya sendiri)? Saran Claude: **alpine terbaru**, perubahan minimal, healthcheck tetap jalan.
8. **OpenAPI (BE-10)**: masuk scope sekarang atau ditunda? Menulis spec untuk 138+ endpoint itu besar. Saran Claude: **ditunda**, dicatat; mulai dari modul baru berikutnya.
9. **`backend-cit` (BE-11)**: cukup dikeluarkan dari pipeline/rencana deploy sampai rename ke `backend-exq` dan ada isinya? Rename-nya sendiri dikerjakan user (keputusan 2026-10-08). Saran Claude: **ya, hanya keluarkan dari deploy + catat**; module tetap di workspace dan CI tetap mem-build-nya.

## Jawaban PO (2026-10-12)
User: *"setuju semua saran Claude, lanjut spec"*. Artinya:

1. Satu AI-DLC, 12 item, fase A–G sesuai urutan Q1.
2. Runner migrasi stdlib + tabel `schema_migrations` (nama + checksum + waktu); dev di-baseline "sampai 029"; dijalankan sebagai langkah deploy terpisah (manual), bukan saat API start.
3. Coverage dihitung dari unit + integration; ≥ 80% untuk `service`, `approval`, `rolemgmt`; `repository` lewat integration; ambang CI mulai dari baseline lalu naik bertahap. Sub-pertanyaan "fase di sini atau AI-DLC terpisah" tidak punya saran → diputuskan di spec (S3).
4. GitHub Actions = CI; Cloud Build = build image + push + deploy. `docs/deployment.md` diperbarui.
5. Metrics lewat log request terstruktur → log-based metrics Cloud Monitoring; tanpa library baru; tracing ditunda.
6. `python-jose` → `PyJWT`; semua requirements di-pin `==`, tanpa tool lock. Sub-pertanyaan Dockerfile Python tidak punya saran → diputuskan di spec (S6).
7. Base image: Alpine terbaru yang didukung (bukan distroless).
8. OpenAPI (BE-10) ditunda; dicatat, mulai dari modul baru berikutnya.
9. `backend-cit` hanya dikeluarkan dari deploy + dicatat; module tetap di workspace dan tetap di-build CI.
