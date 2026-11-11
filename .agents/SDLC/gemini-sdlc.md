# AI-Native SDLC

> **Note:** Folder ini menyimpan artefak siklus hidup pengembangan fitur (6-Stage AI-Native SDLC) di dalam `.agents/SDLC/`. Untuk format Hybrid Kiro/Gemini (`requirements.md` -> `design.md` -> `tasks.md`), lihat `.agents/specs/<feature>/`. Progress pelacakan fitur dicatat di `.agents/development-progress.md`.

Folder ini menampung satu sub-folder per fitur. Setiap fitur melewati enam tahap (*stages*), dan setiap tahap menghasilkan satu artefak *machine-readable* yang dibaca oleh tahap berikutnya. Rangkaian file yang di-commit menjadi *audit trail*: siapa yang meminta apa, apa yang dihasilkan agen, dan siapa yang menyetujuinya.

Reference: https://claude.com/blog/the-ai-native-sdlc-playbook

## Layout

```text
.agents/SDLC/
├── gemini-sdlc.md         <- Anda berada di sini (cara kerja workflow)
└── <feature-name>/        <- satu folder per fitur (mis. vendor-upload-dsr/)
    ├── intent.md          Stage 1 Plan
    ├── spec.md            Stage 2 Design
    ├── plan.md            Stage 3 Build
    ├── tests.md           Stage 4 Test
    ├── review.md          Stage 5 Deploy
    └── incidents.md       Stage 6 Maintain
```

## Artifact chain (isi secara berurutan; setujui & commit setiap tahap sebelum memulai tahap berikutnya)

| Stage | Artifact | Who drives | Trigger to next stage |
|-------|----------|------------|-----------------------|
| 1 Plan | `intent.md` | Product owner + Gemini (Antigravity) | product owner menyetujui (`accepted`) intent |
| 2 Design | `spec.md` | Product owner + Gemini (skills applied) | product owner menyetujui (`accepted`) spec |
| 3 Build | `plan.md` | Engineer + Gemini plan mode | engineer menyetujui (`accepted`) plan |
| 4 Test | `tests.md` | Engineer (feedback loop) | build + automated tests hijau (`green`) |
| 5 Deploy | `review.md` | Reviewer agent + code owner | PR merged melewati seluruh gate |
| 6 Maintain | `incidents.md` | Monitoring / on-call | pelanggaran control-band membuat `intent.md` baru |

Siklus tertutup (*closed loop*): pelanggaran, tiket, atau *alert* pada Stage 6 menghasilkan `intent.md` baru dan siklus dimulai kembali.

## How to start a new feature

1. Buat folder baru: `.agents/SDLC/<feature-name>/` (gunakan format `kebab-case`).
2. Salin enam file artefak dari fitur yang sudah ada sebagai template, atau buat menggunakan Gemini berdasarkan daftar template di bawah.
3. Isi `intent.md` terlebih dahulu sesuai kata-kata peminta (*originator*). Biarkan file lainnya sebagai *stub* sementara.
4. Maju satu tahap dalam satu waktu. Setiap file diawali `# <Judul>` diikuti `> Status: draft`. Hanya manusia (PO/Engineer) yang berhak mengubahnya menjadi `> Status: accepted YYYY-MM-DD (<role>)`. Jangan memulai tahap berikutnya sebelum artefak tahap sebelumnya disetujui (`accepted`).

## Rules that apply to every feature

Diambil dari `.agents/rules/GEMINI.md` (*source of truth* proyek untuk Gemini/Antigravity). Cantumkan kembali aturan spesifik yang relevan pada setiap `intent.md` / `spec.md`:

- **Tech Stack Tetap**: Go + Chi v5 (`backend/`, `backend-exq/`, `pkg/`), sqlc `v1.31.1`, Python EOD (`backend_python/`), React + Vite (`frontend/CompanyPortal-Vite`, `frontend/VendorPortal-Vite`), PostgreSQL (primary + read replica), Redis. Jangan menambah endpoint/tabel/env/library di luar kesepakatan.
- **Rencanakan Sebelum Eksekusi**: Buat rencana sebelum pekerjaan non-trivial; hasilkan diff kecil dengan satu fokus (*one concern*).
- **Tipe Data Uang & Waktu**: Gunakan `numeric` atau integer minor units (`bigint` IDR), **dilarang keras** menggunakan `float` / `double`. Timestamp wajib `timestamptz` (UTC).
- **Topologi Database**: Operasi write/update dan read-after-write pada primary (`db`); reporting, dashboard, monitoring, dan ekspor pada read replica (`dbRead`).
- **Maker-Checker & Audit**: Setiap perubahan state pada data finansial atau master wajib melalui maker-checker (`approval_requests`) dan mencatat `audit_logs` dalam transaksi yang sama.
- **Idempotensi Ingest File**: Proses ingest file wajib idempoten per checksum.
- **STOP dan Tanyakan User (Gate Manusia)**: Pada perubahan yang menyentuh autentikasi, perhitungan uang/jurnal, rekonsiliasi, migrasi skema database, atau penghapusan data.
- **Pengujian & Verifikasi**: Setiap fitur wajib disertai automated test (target coverage >= 80% pada `internal/*`). Verifikasi UI di browser secara manual dilakukan oleh **user** dan dicatat sebagai `outstanding (user)`.

## Artifact templates (kegunaan setiap file)

- `intent.md` — Masalah dalam bahasa peminta (*originator*): deskripsi masalah, hasil yang diharapkan (*proposed outcome*), pengguna/sistem terdampak, batasan (*constraints*), dan daftar pertanyaan terbuka (*open questions*).
- `spec.md` — Kebutuhan fungsional & non-fungsional, kontrak API (`backend/` flat JSON vs `backend-exq/` envelope `pkg/response`), model data, batasan di luar cakupan (*out-of-scope*), serta isu kritis yang harus diputuskan pemilik kebijakan sebelum tahap build.
- `plan.md` — Daftar file yang berubah, urutan pengerjaan, risiko, bukti pengujian (*proof* yang dipetakan ke test), serta alternatif yang tidak diambil. Tetap disinkronkan dengan perubahan kode.
- `tests.md` — Perintah build/test/lint, target terukur (*quantifiable targets*), dan kasus uji (*test cases*) yang ditelusuri kembali ke kebutuhan di `spec.md`.
- `review.md` — Tahapan review (bugs / security / compliance), klasifikasi temuan (*Important* vs *nit*), gate *Definition of Done*, dan persetujuan merge.
- `incidents.md` — Sinyal yang dipantau (*monitored signals*), tier respons, log insiden, serta regression test / eval yang ditambahkan dari insiden.

## Current features

Status fitur yang dikerjakan oleh Gemini dipelihara juga di `.agents/development-progress.md`.

| Feature | Folder | Stage | Status |
|---------|--------|-------|--------|
| *(Belum ada fitur)* | — | — | — |
