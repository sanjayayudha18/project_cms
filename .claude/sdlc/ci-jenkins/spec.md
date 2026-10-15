# Spec: CI/CD pindah ke Jenkins

Status: **draft 2026-10-15, ditunda (user 2026-10-15: "simpan ini untuk nanti")**. Lanjut dari sini: PO putuskan S1–S7 + O1–O5, lalu accept. Stage: 2 Design. Stage berikut: `plan.md`.
Sumber: `intent.md` (accepted 2026-10-15, jawaban PO Q1–Q12). Target mulai dipakai: kemungkinan Desember 2026.

## ⚠ Flagged (Sec 4 #7 dan hal sensitif)
- **Kredensial deploy**: Jenkins memegang akses push ke Artifact Registry dan deploy ke VM dev/UAT/prod. Kredensial dikelola tim internal (Q6), tidak pernah ada di repo atau log. Mekanismenya masih terbuka (O3).
- **Deploy prod otomatis** (Q5 "untuk saat ini otomatis"): satu VM, satu container per service, jadi setiap deploy = restart singkat. Kalau prod juga otomatis, push ke branch prod pada jam kerja (07:00–20:00) memutus user sebentar. Lihat O2 + S5.
- **Migrasi DB (data migration)**: pipeline **tidak** menjalankan migrasi (S3). Migrasi tetap langkah terpisah oleh operator, sesuai be-tech-debt FR-D3 (runner `cmd/migrate` belum dibangun; sekarang `psql -f` manual).
- **Hapus file**: `.github/workflows/ci.yml` dihapus hanya setelah Jenkins terbukti hijau (Q8, FR8). Tidak ada hapus data.
- Tidak ada perubahan skema, endpoint, response shape, atau kode aplikasi.

## Alur

```
GitHub (push / PR ke main, dev1)
  └─► Jenkins perusahaan (multibranch, webhook)            [Q1, Q2, Q3]
        Jenkinsfile di root repo
        ├─ 1 CI (paralel) ── go ×3 modul · go-integration (Postgres 18) · sqlc drift · python
        │                    (= parity penuh dengan ci.yml, FR2)
        ├─ 2 Build image ─── backend · backend-cit · companyportal · vendorportal · python EOD (S1)
        │                    tag = full git SHA, tanpa latest (FR3)
        │   PR build: berhenti di sini, tidak push (S7)
        ├─ 3 Push ────────── GCP Artifact Registry                                  [Q7]
        └─ 4 Deploy ──────── branch → lingkungan (dev / UAT / prod)                 [O1]
                             SSH ke VM → docker compose pull + up dengan IMAGE_TAG=<sha>
                             → tunggu /health → catat SHA terpasang (FR4, FR5)
Status (hijau/merah) → Jenkins + status cek di PR GitHub (Q12). Tanpa email/chat.
```

## Functional requirements

### FR1 — Jenkinsfile
- Satu Declarative Jenkinsfile di root repo, dijalankan oleh **job multibranch** di Jenkins perusahaan (Q1). Bila tim Jenkins mewajibkan shared library/template (O5), Jenkinsfile menyesuaikan tanpa mengubah isi tahapan FR2–FR5.
- **Portabel Git** (Q2): Jenkinsfile tidak memanggil API GitHub. Sumber repo, webhook, dan pengiriman status ke PR diatur di konfigurasi job Jenkins (plugin GitHub Branch Source), bukan di Jenkinsfile. Pindah ke Git lain = ganti konfigurasi job saja.
- Trigger: push dan PR ke `main`/`dev1` (Q3). Build PR = CI + build image tanpa push/deploy.
- Satu build per branch pada satu waktu (`disableConcurrentBuilds`). Deploy per lingkungan diserialkan (`lock`), supaya dua build tidak deploy bersamaan.
- Timeout per tahap dan per build, supaya build yang macet gagal dan tidak menahan agent.

### FR2 — Tahap CI (parity dengan `ci.yml`)
Semua cek di `ci.yml` pindah tanpa dilemahkan:

| Tahap | Isi (sama persis dengan `ci.yml`) |
| --- | --- |
| `go` ×3 (`backend`, `backend-cit`, `pkg`) | `go vet` · golangci-lint **v2.13.0** `--build-tags integration` · govulncheck **v1.8.0** · `go test -race` |
| `go-integration` | Postgres **18** sementara · `backend/scripts/apply-migrations.sh` · `go test -tags integration -race -count=1 -cover` · ada `--- SKIP` = gagal · `backend/scripts/check-coverage.sh` |
| `sqlc` | sqlc **v1.31.1** generate · `UserLeafe` → `UserLeave` · `git diff --exit-code -- internal/db` |
| `python` | Python **3.14** · Postgres 18 + migrasi · install `backend_python/*/requirements.txt` · `python -m unittest discover` · ada `skipped=` = gagal |

- Versi Go diambil dari `go.work` (sama dengan `setup-go go-version-file`).
- Tahap dijalankan di dalam container image yang di-pin (mis. `golang:<versi go.work>`, `python:3.14`, `postgres:18`), supaya agent cukup punya Docker dan tidak perlu menginstal toolchain sendiri. Ini butuh agent Linux + Docker (O4).
- Postgres untuk test hanya hidup selama build (sidecar), password khusus CI, tidak menyentuh DB dev/UAT/prod.
- Hook untuk job yang menyusul: frontend CI (fe-tech-debt E1) dan E2E (be-tech-debt B5) ditambahkan sebagai tahap baru oleh plan masing-masing (Q10). Fitur ini tidak membuatnya.

### FR3 — Build image
- Image yang di-build (Q9: semua lewat Jenkins):

| Image | Dockerfile | Context |
| --- | --- | --- |
| `cms-backend-api` | `backend/Dockerfile` | root |
| `cms-backend-cit` | `backend-cit/Dockerfile` | root |
| `companyportal-vite-fe` | `frontend/CompanyPortal-Vite/Dockerfile` | folder app |
| `vendorportal-vite-fe` | `frontend/VendorPortal-Vite/Dockerfile` | folder app |
| `cms-eod` (baru, S1) | `backend_python/Dockerfile` (baru) | `backend_python/` |

- Tag = **full git SHA** commit yang di-build. Tidak pernah `latest`, tidak ada tag yang dipindah-pindah.
- **Build once, promote**: image yang sama (SHA sama) dipakai dev → UAT → prod. Tidak ada rebuild per lingkungan. Ini aman karena frontend memakai base URL relatif `/api/v1` bila `VITE_API_BASE_URL` kosong (`src/lib/api/config.ts`), dan Nginx meneruskan ke `host.docker.internal` di VM masing-masing. Konfigurasi per lingkungan hanya dari `.env` di VM, tidak masuk image.
- Image hanya dibangun setelah tahap CI hijau.

### FR4 — Push ke Artifact Registry
- Tujuan: `<region>-docker.pkg.dev/<gcp-project>/<repo>/<image>:<sha>` (Q7). Nilai region/project/repo = parameter job Jenkins, bukan hardcode di Jenkinsfile.
- Push hanya dari build branch (bukan PR), setelah FR2 + FR3 hijau.
- SHA yang sudah ada di registry tidak ditimpa (registry diset immutable tags bila didukung, atau push dilewati bila tag sudah ada).

### FR5 — Deploy
- Pemetaan branch → lingkungan: **O1 (terbuka)**. Spec mendukung pola apa pun lewat satu tabel di Jenkinsfile.
- Mekanisme (S4): Jenkins SSH ke VM tujuan, lalu `docker compose pull` + `docker compose up -d` dengan file compose deploy yang **memakai image dari registry** (`image: …:${IMAGE_TAG}`), bukan `build:`. File compose deploy disimpan di repo (`deploy/`), terpisah dari compose dev lokal yang sekarang.
- `.env` aplikasi per lingkungan **sudah ada di VM** (atau diambil dari Secret Manager oleh VM). Pipeline tidak membawa secret aplikasi (`JWT_SECRET`, DB, LDAP, SMTP).
- Setelah `up`: tunggu health tiap service (`/health` backend, `/` frontend, `/health` EOD scheduler) dengan batas waktu. Gagal = build merah (dan rollback otomatis, S2).
- SHA yang sedang terpasang per service per lingkungan dicatat di VM (mis. file `deploy/.deployed`), supaya rollback tahu SHA sebelumnya.
- Gerbang: otomatis setelah CI hijau (Q5). Bila PO memutuskan prod manual (O2), cukup tambah satu `input` step di tahap prod, dibatasi ke grup approver Jenkins yang ditentukan, dan approver ≠ orang yang push.

### FR6 — Rollback
- Job/parameter Jenkins "deploy SHA tertentu ke lingkungan X" yang memakai jalur FR5 yang sama, tanpa build ulang. Sesuai aturan sekarang: SHA sebelumnya tetap bisa di-deploy per service.
- Migrasi DB tidak di-rollback otomatis (S3). Rollback image melewati migrasi yang tidak kompatibel adalah keputusan operator.

### FR7 — Status
- Hasil build tampil di Jenkins dan sebagai status cek di PR GitHub (Q12). Tidak ada email/chat.
- Setelah aktif, status Jenkins dijadikan "required check" di branch protection GitHub (langkah manual oleh user/admin repo).

### FR8 — Transisi dan dokumen
- Selama transisi `ci.yml` **tetap jalan** (Q8). Dihapus di slice terakhir, setelah pipeline Jenkins hijau untuk `main` dan `dev1` di repo ini.
- Dokumen yang diperbarui saat Jenkins aktif:
  - `.claude/docs/deployment.md` #8 + "Pipeline": Jenkins = CI + CD. Cloud Build dihapus.
  - CLAUDE.md Sec 10 (+ Sec 2a bila ada perintah baru).
  - `.claude/sdlc/be-tech-debt/` (FR-B7, B5) dan `.claude/sdlc/fe-tech-debt/` (E1): target CI = Jenkins.
  - Salinan lama `.kiro/steering/project-context.md` + `.opencode/opencode.md`: diberi catatan "usang, lihat deployment.md", bukan ditulis ulang.

## Non-functional
- **NFR1 (keamanan)**: tidak ada secret di repo, Jenkinsfile, atau log build. Kredensial lewat Jenkins credentials binding (dimask di log). Kredensial deploy hanya dipakai di tahap deploy (least privilege: push AR + SSH ke VM tujuan).
- **NFR2 (kesetaraan)**: tidak ada cek di `ci.yml` yang hilang atau melemah (FR2). Dibuktikan dengan tabel parity di `tests.md`.
- **NFR3 (reproducible)**: semua tool dan image di-pin versinya. Tag image = SHA, sehingga yang jalan di prod bisa ditelusuri ke commit.
- **NFR4 (portabel)**: logika build/deploy ada di script shell biasa (`deploy/*.sh`) yang dipanggil Jenkinsfile, supaya bisa diuji lokal dan tidak terkunci ke Jenkins atau GitHub.
- **NFR5 (durasi)**: target CI ≤ 20 menit per build (tahap paralel). Angka ini usulan Claude, perlu dikonfirmasi setelah run pertama.

## Keputusan spec (usulan Claude, perlu konfirmasi PO)
- **S1 Python EOD dikontainerkan**: Dockerfile baru `backend_python/` (`TZ=Asia/Jakarta`, user non-root, mount `FTP_DATA`), satu image untuk `eod_retry_scheduler`, `service_dsr_etl`, dan skrip ETL. Alternatifnya: deploy kode ke host (rsync + venv + restart systemd), tanpa image. Saran: **dikontainerkan**, karena sama dengan service lain, bisa di-rollback per SHA, dan TZ terkunci di image. Efek samping: izin file `FTP_DATA` (QC F5) perlu shared group (`group_add`), sesuai catatan di `deployment.md`.
- **S2 Rollback otomatis bila health gagal**: deploy gagal health → kembali ke SHA tercatat sebelumnya, lalu build merah. Saran: **ya**.
- **S3 Migrasi tidak dijalankan pipeline**: tetap langkah operator terpisah (be-tech-debt FR-D3), di luar jam 07:00–20:00. Pipeline hanya mencetak peringatan bila commit menambah file di `backend/migrations/`. Saran: **ya**.
- **S4 Deploy lewat SSH + docker compose** di VM, dengan compose deploy di `deploy/`. Alternatifnya: agent Jenkins dipasang di tiap VM. Saran: **SSH**, karena lebih sedikit infrastruktur.
- **S5 Jendela deploy prod**: kalau prod otomatis (O2), deploy prod hanya jalan di luar 07:00–20:00 WIB (build di jam kerja menunggu atau ditolak). Saran: **ya**, sejalan dengan aturan Sec 14 soal kerja berat di jam kantor.
- **S6 Tag = full SHA** (bukan short SHA). Saran: **ya**.
- **S7 Build PR tidak push image**: PR hanya CI + build untuk membuktikan Dockerfile valid. Saran: **ya**.

## Masih terbuka (dari intent)
- **O1** (Q4): pemetaan branch → dev/UAT/prod.
- **O2** (Q5): prod juga otomatis? Menentukan S5 dan `input` step FR5.
- **O3** (Q6): mekanisme akses ke GCP (SA key / Workload Identity / SSH key), bersama tim internal.
- **O4** (Q1): agent Jenkins Linux + Docker? Bila tidak ada Docker, FR2 harus memakai toolchain terpasang di agent dan Postgres CI di tempat lain. Ini mengubah plan, tidak mengubah requirement.
- **O5** (Q11): shared library/template Jenkins wajib?

O1–O5 tidak menghalangi plan untuk FR1–FR3 (CI + build). FR4–FR6 (push, deploy, rollback) baru bisa di-plan penuh setelah O1–O4 terjawab.

## Tests (dibuat di stage 4, dilacak di `tests.md`)
| ID | Uji | Req |
| --- | --- | --- |
| T1 | Tabel parity: setiap step `ci.yml` punya padanan di Jenkinsfile dengan versi tool sama | FR2, NFR2 |
| T2 | Build Jenkins merah bila: lint gagal, ada test skip (Go/Python), coverage turun, sqlc drift. Dibuktikan dengan commit uji di branch percobaan | FR2 |
| T3 | `shellcheck` + jalankan lokal `deploy/*.sh` terhadap VM/compose lokal: deploy SHA, health OK, catatan SHA ter-update | FR5, NFR4 |
| T4 | Health gagal (image sengaja rusak) → rollback ke SHA sebelumnya | FR5, S2 |
| T5 | Build PR tidak push ke registry; build branch push dengan tag = full SHA, tag yang ada tidak ditimpa | FR3, FR4, S6, S7 |
| T6 | Log build tidak memuat secret (cek mask credentials) | NFR1 |
| T7 | Image yang sama (SHA) jalan di dev lalu UAT tanpa rebuild | FR3 |
| Manual (user, outstanding) | Run pertama di Jenkins perusahaan; required check di GitHub; deploy pertama ke tiap lingkungan; cek UI setelah deploy (GR#10) | FR1, FR5, FR7 |

## Out of scope
- Setup server Jenkins, agent, plugin, dan IAM/VM/firewall GCP (tim Jenkins / tim internal; hanya didokumentasikan syaratnya).
- Runner migrasi otomatis (be-tech-debt D3).
- Job frontend CI (fe-tech-debt E1) dan E2E (be-tech-debt B5).
- Rename `backend-cit` → `backend-exq`. Bila terjadi sebelum fitur ini selesai, nama image/folder ikut berubah.
- Memperbaiki image `backend-cit` (masih root, tanpa HEALTHCHECK; BE debt). Deploy memakai `curl /health` dari luar container untuk service ini.
- LB / Cloud Armor, blue-green / zero-downtime deploy.
