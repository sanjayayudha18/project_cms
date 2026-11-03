# Intent: CI/CD pindah ke Jenkins (gantikan GitHub Actions + Cloud Build)

Author: user (product owner), ditulis bersama Claude. Status: **accepted 2026-10-15** (user: "Intent ci-jenkins diterima; 5 pertanyaan sisa dibawa ke spec.md.").
Stage: 1 Plan. Stage berikut: `spec.md`.
Asal: keputusan user 2026-10-15.
Menyentuh Sec 4 #7: **tidak langsung** (tidak ada auth aplikasi, uang, rekonsiliasi, migrasi skema, atau hapus data). Tapi ada dua hal sensitif: **kredensial deploy** (akses Jenkins ke GCP, Artifact Registry, VM, Secret Manager) dan **gerbang deploy ke produksi**. Keduanya dibahas di spec.

## Problem
Kata user (2026-10-15): *"CI/CD kita nantinya akan menggunakan JENKIN"*. Cakupan yang dipilih user: **CI + deploy**. Jenkins menjalankan lint/test/scan, lalu build image → Artifact Registry → deploy. `.github/workflows/ci.yml` dihapus, dan Cloud Build dicoret dari dokumen.

Kondisi sekarang (dicek di repo, 2026-10-15):
- **CI** = GitHub Actions, `.github/workflows/ci.yml` (be-tech-debt Fase B, commit `8b3a32b` + `077ce05`). Trigger push/PR ke `main`/`dev1`. Isinya empat job:
  - `go`: matrix `backend`/`backend-cit`/`pkg`, menjalankan vet + golangci-lint v2.13.0 `--build-tags integration` + govulncheck v1.8.0 + `go test -race`.
  - `go-integration`: Postgres 18 service, `backend/scripts/apply-migrations.sh`, gagal bila ada test skip, lalu gate coverage `backend/scripts/check-coverage.sh`.
  - `sqlc`: v1.31.1, perbaikan `UserLeafe`, cek drift dengan `git diff`.
  - `python`: Python 3.14, Postgres 18, unittest, gagal bila ada test skip.
- Job frontend (fe-tech-debt **E1**) dan E2E (be-tech-debt **B5**) belum ada. Rencana keduanya masih ditulis untuk GitHub Actions.
- **CD** = Cloud Build. Ini **hanya di dokumen**: belum ada `cloudbuild.yaml` di repo. `.claude/docs/deployment.md` (#8 + "Pipeline") dan CLAUDE.md Sec 10 menulis: build image `backend/Dockerfile` dan `backend-cit/Dockerfile` (context root) → push ke Artifact Registry dengan tag git SHA → deploy ke Compute Engine (satu VM, dua container) → rollback ke SHA sebelumnya per service.
- Repo di GitHub (`sanjayayudha18/project_cms`).
- Script `backend/scripts/apply-migrations.sh` dan `check-coverage.sh` tidak terikat GitHub Actions, jadi bisa dipakai ulang di Jenkins.
- Dokumen yang bertentangan dengan keputusan ini: `.claude/docs/deployment.md`, CLAUDE.md Sec 10, `.claude/sdlc/be-tech-debt/` (intent Q4, spec FR-B7, plan B6), `.claude/sdlc/fe-tech-debt/` (E1), dan salinan lama di `.kiro/steering/project-context.md` + `.opencode/opencode.md`.

## Proposed outcome
1. Satu pipeline Jenkins (Jenkinsfile di repo) menjalankan **semua cek yang sekarang ada di `ci.yml`** dengan aturan yang sama: lint, vuln scan, unit test, integration test di Postgres 18 tanpa skip, gate coverage, drift sqlc, dan Python unittest tanpa skip.
2. Pipeline yang sama menjalankan **build image** kedua backend (dan frontend bila masuk scope), push ke registry dengan tag git SHA (tidak pernah `latest`), lalu **deploy** ke VM.
3. Rollback tetap bisa ke SHA sebelumnya per service.
4. `.github/workflows/ci.yml` dihapus dan Cloud Build hilang dari dokumen. Kapan tepatnya mengikuti jawaban Q8.
5. Dokumen sinkron: `deployment.md`, CLAUDE.md Sec 10, dan catatan di be-tech-debt/fe-tech-debt bahwa target CI mereka sekarang Jenkins.

## Affected users and systems
- Users aplikasi: tidak ada perubahan.
- Developer/engineer: status cek PR berpindah dari GitHub Actions ke Jenkins.
- Systems: `Jenkinsfile` baru, `.github/workflows/ci.yml` (dihapus), `backend/scripts/*` (dipakai ulang), `.claude/docs/deployment.md`, `.claude/CLAUDE.md` Sec 10 + Sec 2a bila perlu, `.claude/sdlc/be-tech-debt/*`, `.claude/sdlc/fe-tech-debt/*`. Dockerfile kemungkinan tidak berubah.

## Constraints
- Stack aplikasi tetap. Jenkins adalah alat pipeline, bukan library aplikasi. Plugin Jenkins yang dipakai dicatat di spec.
- Kekuatan cek tidak boleh turun dibanding `ci.yml`: versi tool tetap di-pin (golangci-lint, govulncheck, sqlc v1.31.1, Postgres 18, Python 3.14), test skip tetap = gagal, dan gate coverage tetap ada.
- Tag image = git SHA, tanpa `latest`. Database tetap eksternal, tidak didockerkan di produksi. Postgres di CI hanya untuk test.
- Tidak ada secret di Jenkinsfile maupun di repo. Semua kredensial ada di credential store Jenkins atau Secret Manager.
- Deploy produksi tidak boleh tanpa kontrol (bentuk gerbangnya: Q5).
- Manual verification (UI aplikasi setelah deploy, tampilan Jenkins) = tugas user (GR#10).

## Open questions (untuk PO / spec)
1. **Jenkins di mana?** Pakai Jenkins perusahaan yang sudah ada (on-prem/bersama) atau instance baru (mis. di GCP)? Agent Linux dengan Docker tersedia? (Integration test butuh container Postgres 18, dan build image butuh Docker.)
2. **Sumber kode**: repo tetap di GitHub (Jenkins multibranch + webhook, status cek dikirim balik ke PR) atau pindah ke Git internal (Bitbucket/GitLab)?
3. **Trigger**: tetap push/PR ke `main`/`dev1` seperti `ci.yml`? Branch lain?
4. **Lingkungan deploy**: ada berapa (dev/staging/prod)? Branch mana deploy ke lingkungan mana?
5. **Gerbang deploy**: deploy otomatis setelah CI hijau, atau perlu approval manual (input step Jenkins) untuk produksi? Perlu maker ≠ approver?
6. **Akses Jenkins ke GCP**: service account key di credential Jenkins, Workload Identity Federation, atau Jenkins agent di GCP dengan service account VM? Siapa pemilik kredensial itu?
7. **Registry**: tetap GCP Artifact Registry, atau registry perusahaan (Nexus/Harbor)?
8. **Transisi `ci.yml`**: dihapus di slice pertama, atau dibiarkan jalan paralel sampai pipeline Jenkins terbukti hijau?
9. **Cakupan build/deploy**: hanya `backend` + `backend-cit`, atau juga kedua frontend (image Nginx) dan Python EOD (`eod_retry_scheduler`, `service_dsr_etl`)? Python EOD sekarang tidak ada di pipeline mana pun.
10. **Job yang belum dibuat**: frontend CI (fe-tech-debt E1) dan E2E (be-tech-debt B5) masuk ke Jenkins di fitur ini, atau tetap di plan masing-masing dengan target Jenkins?
11. **Format pipeline**: Declarative Jenkinsfile di root repo, atau perusahaan punya shared library/template Jenkins yang wajib dipakai?
12. **Notifikasi hasil pipeline**: email (SMTP perusahaan), chat, atau cukup status di Jenkins/PR?

## Jawaban PO (2026-10-15, dijawab satu per satu)
1. **Jenkins**: pakai **Jenkins perusahaan yang sudah ada**. Agent Linux + Docker: belum dijawab.
2. **Sumber kode**: saat ini **tetap GitHub**. Mungkin nanti pindah ke Git lain, tapi belum pasti. Implikasi untuk spec: Jenkinsfile tidak boleh bergantung pada fitur khusus GitHub, kecuali bagian trigger/status cek yang dibatasi di satu tempat.
3. **Trigger**: **tidak berubah**, tetap push/PR ke `main`/`dev1`. **Jadwal**: Jenkins mulai aktif dipakai **kemungkinan Desember 2026**. Sampai saat itu `ci.yml` tetap menjadi CI yang berjalan (kapan dihapus: Q8).
4. **Lingkungan deploy**: tiga, yaitu **dev, UAT, prod**. Pemetaan branch → lingkungan: belum dijawab.
5. **Gerbang deploy**: **untuk saat ini otomatis** setelah CI hijau, tanpa approval manual. Catatan: ini bertentangan dengan constraint "Deploy produksi tidak boleh tanpa kontrol" yang ditulis Claude di atas. Perlu konfirmasi apakah prod juga otomatis.
6. **Akses Jenkins ke GCP**: kredensial dimiliki dan dikelola **tim internal**. Mekanisme (SA key / Workload Identity / SSH): belum dijawab.
7. **Registry**: **tetap GCP Artifact Registry** (tag git SHA, sesuai `deployment.md`).
8. **Transisi `ci.yml`**: **biarkan dulu**. `ci.yml` tetap jalan dan baru dihapus setelah pipeline Jenkins terbukti hijau di repo ini.
9. **Cakupan build/deploy**: **semua lewat Jenkins**, yaitu `backend`, `backend-cit` (rencana rename `backend-exq`), kedua frontend, dan Python EOD (`eod_retry_scheduler`, `service_dsr_etl`). Fakta repo 2026-10-15: kedua frontend sudah punya Dockerfile, Python EOD **belum punya Dockerfile**. Cara deploy Python EOD dibahas di spec. Syarat host-nya `TZ=Asia/Jakarta` (CLAUDE.md Sec 14).
10. **Job yang belum dibuat**: frontend CI (fe-tech-debt E1) dan E2E (be-tech-debt B5) **tetap di plan masing-masing**, hanya targetnya diubah dari GitHub Actions ke Jenkins. Fitur ini cukup menyediakan Jenkinsfile tempat job itu nanti ditambahkan.
11. **Format pipeline**: **belum tahu**, perlu ditanyakan ke tim Jenkins perusahaan (apakah ada shared library/template wajib).
12. **Notifikasi**: **cukup status** di Jenkins dan PR. Tidak ada email atau chat.

### Masih terbuka setelah sesi tanya-jawab
- Q1: agent Jenkins Linux + Docker? (tanya tim Jenkins)
- Q4: pemetaan branch → dev/UAT/prod.
- Q5: apakah **prod** juga deploy otomatis (bertentangan dengan constraint "deploy produksi tidak boleh tanpa kontrol").
- Q6: mekanisme akses ke GCP (SA key / Workload Identity / SSH), bersama tim internal.
- Q11: format pipeline / shared library wajib (tanya tim Jenkins).
