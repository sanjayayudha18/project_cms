# Steering: AI Collaboration Rules, AI-Native SDLC & Scope-Driven Dev

> **Scope**: AI Collaboration Rules ("the leash" Sec 4), 4 Metode Pengembangan di `.agents/`, 6 Tahap AI-Native SDLC (Sec 4a), Scope-Driven Development (ScDD), Gerbang Sec 4 #7, Graphify navigation, dan Tech-Debt backlog.  
> **Source of Truth**: Disinkronkan dari `.claude/CLAUDE.md` (Sec 1a, 4, 4a, 15) & standar `.agents/`.  
> **Referenced by**: [GEMINI.md](file:///c:/Users/RB%20Yudha%20Rangga/OneDrive/Documents/Development/CMS2/.agents/rules/GEMINI.md)

---

## 1. AI Collaboration Rules — "The Leash" (Sec 4)

1.  **Plan First for Non-Trivial Work**: Buat daftar file + langkah pengerjaan, lalu tunggu persetujuan ("OK") dari user sebelum mengeksekusi.
2.  **Small Diffs, One Concern**: Perubahan kecil dan terfokus. Dilarang melakukan *drive-by refactoring* di luar cakupan tugas.
3.  **Ask, Don't Guess**: Tanyakan jika ada keraguan mengenai nama, kontrak API, atau aturan bisnis. **Jangan pernah mengarang endpoint, tabel, kolom, env var, atau modul.**
4.  **Respect Stack & Module Map**: Patuhi tech stack (Sec 2) dan peta modul/tabel (Sec 3). Tidak boleh menambah dependency baru tanpa izin.
5.  **Read Before Edit**: Baca dan ikuti pola eksisting di `pkg/` dan `internal/`.
6.  **Zero Hallucination**: Jangan pernah menghalusinasikan file atau fungsi yang tidak ada.
7.  **STOP & Flag on Critical Areas (Sec 4 #7)**:
    *   Berhenti dan beri penanda khusus jika menyentuh: **autentikasi (`auth`), perhitungan uang/jurnal (`money/journal`), rekonsiliasi (`reconciliation`), migrasi data/skema (`data migrations`), atau penghapusan data (`deletes`)**.
    *   Area ini **wajib** melalui rantai ber-gate penuh: **Spec-Driven Dev** (6 tahap) atau **Scope-Driven Dev** dengan bagian `## Gerbang Sec 4 #7` yang terisi lengkap (formula + contoh numerik, persetujuan skema, tes money/RBAC/maker-checker, dan security review).
8.  **State Tradeoffs Concisely**: Jelaskan setiap tradeoff desain dalam 1–2 baris secara lugas.
9.  **Every Feature Ships with Tests**: Setiap fitur wajib disertai automated test sesuai standar Sec 8.

---

## 2. Langkah 0: Pemilihan 4 Metode Pengembangan (Wajib Tanya Dahulu)

Sebelum menulis kode atau membuat artefak apa pun, agen **wajib bertanya terlebih dahulu** kepada user dengan menyajikan **4 pilihan metode pengembangan** beserta evaluasi checklist dan rekomendasinya:

| # | Metode | Lokasi Artefak (Gemini `.agents/`) | Kapan Digunakan |
|---|---|---|---|
| **1** | **Spec-Driven Dev (6-Stage Spec)** | `.agents/specs/<feature>/` *(Selesai: `.agents/specs/done/<feature>/`)* | Fitur baru yang besar/kompleks, persyaratan belum tertulis utuh di FSD/URS, atau menyentuh **Sec 4 #7** (auth, money, recon, migrasi DB, delete). |
| **2** | **Scope-Driven Dev (ScDD)** | `.agents/scopes/<feature>/` *(Selesai: `.agents/scopes/done/<feature>/`)* | Fitur yang persyaratannya sudah jelas di dokumen sumber resmi (DSR/FSD/URS) atau batasan MVP/MoSCoW yang terdefinisi jelas. |
| **3** | **Bug Fix** | `.agents/bugfixes.md` *(urutan terbaru di atas)* | Perbaikan bug/error/failing test/lint finding. Wajib mengikuti alur: *diagnosa → tulis failing test → minimal fix → verifikasi → catat di `bugfixes.md`* (skill `cms2-bugfix`). |
| **4** | **Small Change Dev** | `.agents/changes.md` | Perubahan kecil (<2 file, tweak UI minor, typo, copy text) yang **tidak** menyentuh Sec 4 #7 atau skema DB. |

> **Interoperabilitas Lintas Direktori**: Artefak yang dikerjakan di Antigravity/Gemini dicatat di dalam `.agents/` agar rapi dan terpisah. Namun jika user secara eksplisit meminta melanjutkan spec/scope yang sudah dimulai di `.kiro/specs/<feature>/`, `.claude/dev-plan/plan-perscope/dev-phase-perscope/<n>-<scope>/`, atau `.claude/sdlc/<feature>/` (legacy), kerjakan langsung di lokasi yang diminta user tersebut tanpa memigrasikan foldernya.

---

## 3. AI-Native SDLC — Rantai 6 Tahap (Sec 4a)

Setiap fitur non-trivial bergerak melewati 6 tahap berurutan. Setiap file artefak wajib mencantumkan baris status di bawah judulnya:
`> Status: draft | accepted YYYY-MM-DD (<role>)`
*(Hanya manusia/user yang berhak mengubah status menjadi `accepted` — AI dilarang melakukan self-approve).*

| Tahap | Artefak (`.agents/specs/<feature>/`) | Human Gate (Wajib Tunggu User) | Tugas Agen AI |
|---|---|---|---|
| **1. Plan** | `requirements.md` → `## Introduction` | PO menyetujui Introduction; pertanyaan terbuka dijawab | Tangkap masalah, hasil yang diharapkan, dan batasan; daftarkan pertanyaan terbuka (*open questions*) tanpa menjawabnya sendiri. |
| **2. Design** | `requirements.md` (EARS) + `design.md` | PO menyetujui requirements; Engineer menyetujui design | Tulis requirements berformat EARS, kontrak API, model data, *Correctness Properties*, out-of-scope; tandai item **Sec 4 #7**. |
| **3. Build** | `tasks.md` | Engineer menyetujui tasks | Susun daftar task dengan `_Requirements:_`, *Model Split*, dan *Task Dependency Graph* → lalu kerjakan dalam diff kecil; jaga `tasks.md` tetap sinkron. |
| **4. Test** | `tasks.md` → `## Notes` → `### Test Results` | Build + seluruh automated test hijau | Jalankan `go test ./...`, `pnpm test`, typecheck, `pnpm build`, perbaiki hingga hijau; petakan tes ke requirements/properties. Catat verifikasi browser sebagai `outstanding (user)`. |
| **5. Deploy** | `review.md` | Code owner menyetujui merge | Lakukan review menyeluruh (bug, security, compliance), pisahkan *Important vs nit*, dan periksa 10 poin checklist Definition of Done (Sec 11). |
| **6. Maintain** | `incidents.md` | On-call | Catat insiden pasca-rilis, tambahkan regression test per insiden; pelanggaran/breach baru membuka siklus spec baru. |

### 3.1 Heading Kanonis Literal Validator (Kiro Spec Sync)
Validator spec mencocokkan teks heading `##` **secara literal**. Konten yang diletakkan di bawah heading yang salah akan gagal validasi. Gunakan heading baku bahasa Inggris berikut persis seperti tertulis:

*   **`requirements.md`**:
    *   `# Requirements Document` (judul)
    *   `## Introduction` (latar belakang, kondisi kode saat ini + tanggal, scope in/out, area Sec 4 #7, open questions)
    *   `## Glossary` (definisi entitas/istilah yang menjadi subjek kriteria)
    *   `## Requirements` (tiap item: `### Requirement N: <judul>` $\to$ `**User Story:**` $\to$ `#### Acceptance Criteria` format EARS: `WHEN/IF/WHILE … THE <Komponen> SHALL …`)
*   **`design.md`**:
    *   `## Overview`
    *   `## Architecture` (diagram Mermaid + aliran data)
    *   `## Components and Interfaces` (komponen BE/FE/API sebagai `###`, detail `####`)
    *   `## Data Models` (skema migrasi, constraint, atau tulis "Tidak ada tabel baru")
    *   `## Correctness Properties` (tiap property: `### Property N: <judul>`, kalimat *For any …*, lalu `**Validates: Requirements X.Y, …**`)
    *   `## Error Handling`
    *   `## Testing Strategy`
*   **`tasks.md` (Tepat 4 Heading `##` Wajib)**:
    1.  `## Overview`: Ringkasan implementasi + sub-heading wajib `### Model Split` (pemetaan model reasoning vs execution).
    2.  `## Tasks`: Checkbox `- [ ] N. Judul` dan sub-task `- [ ] N.M Judul` (indentasi 2 spasi; test opsional: `- [ ]* N.M`). Setiap task wajib memiliki `_Requirements: X.Y_`; setiap top-level task wajib memuat `_Model: <Tier>, Effort: <Level> — alasan_`. Dilarang menggunakan heading `## Task N`.
    3.  `## Notes`: Catatan implementasi, DoD, serta sub-heading `### Test Results` setelah build.
    4.  `## Task Dependency Graph`: Blok ```json``` format objek:
        ```json
        { "waves": [{ "id": 0, "tasks": ["1", "2.1"] }, { "id": 1, "tasks": ["2.2"] }] }
        ```
        *Aturan Graph*: Wajib memuat **seluruh leaf task tepat satu kali** (termasuk checkpoint/gate, test opsional `*`, dan task penutup). Parent yang memiliki sub-task diwakili oleh leaf task-nya. Task yang menyentuh file yang sama wajib diserialkan ke wave berbeda. Di atas blok JSON sertakan ringkasan prosa (`**Wave N:** ...`).

### 3.2 Checklist Traceability Wajib
Sebelum spec atau scope dinyatakan siap di-build, verifikasi 100% keterhubungan:
- [ ] **Acceptance Criteria Terpetakan**: Setiap acceptance criterion (`N.M` atau `AC-x.y-zz`) dirujuk minimal oleh satu `_Requirements:_` / `_AC:_` di `tasks.md`.
- [ ] **Tidak Ada ID Hantu**: Setiap kriteria yang dirujuk benar-benar ada di dokumen requirement/scope.
- [ ] **Design Coverage**: Setiap requirement di-cover oleh section Components/Data Models, dan setiap Correctness Property memiliki baris `**Validates:**`.
- [ ] **Property Test Trace**: Setiap Correctness Property memiliki task automated test yang menyebut nomor property terkait.
- [ ] **Graph Completeness**: Gabungan ID task di seluruh wave cocok persis dengan seluruh leaf task di `## Tasks` (selisih dua arah harus kosong).
- [ ] **Migration Check**: Nomor migrasi yang direncanakan belum pernah terpakai di `backend/migrations/` (cek nomor tertinggi di folder).

### 3.3 Aturan Ketat Eksekusi SDLC (Sequential Gate)
1.  **Satu Dokumen per Giliran (Sequential Gate)**: Tulis artefak SDLC **satu per satu secara berurutan**. Jangan pernah membuat dua file tahap berbeda (misalnya `requirements.md` dan `design.md`) sekaligus dalam satu giliran respon. Jangan pernah melompat ke tahap berikutnya sebelum artefak tahap saat ini berstatus `accepted`.
2.  **Single Source of Truth Artefak**: Bekerja hanya dari artefak terakhir yang sudah `accepted`. Jika kode dan artefak berbeda, **BERHENTI dan tanyakan ke user** (jangan diam-diam mengubah salah satunya).
3.  **Update Progress & Lessons Learned**:
    *   Setiap kali satu tahap selesai, perbarui baris fitur terkait di [`.agents/development-progress.md`](file:///c:/Users/RB%20Yudha%20Rangga/OneDrive/Documents/Development/CMS2/.agents/development-progress.md).
    *   Pelajaran penting dari insiden/review dicatat di [`.agents/lessons-learned.md`](file:///c:/Users/RB%20Yudha%20Rangga/OneDrive/Documents/Development/CMS2/.agents/lessons-learned.md).

---

## 4. Scope-Driven Development / ScDD (Sec 4a)

Digunakan untuk scope yang kebutuhannya sudah jelas (mis. mengacu pada baris resmi DSR/FSD/URS atau batasan scope MVP):
*   **Struktur di `.agents/scopes/<feature>/`**:
    *   `README.md`: Roadmap scope, daftar periksa per-fase, status fase, dan dependensi antar-fase.
    *   `<n>.0-keputusan.md` (opsional): Fase 0 tanpa kode jika ada konflik/pertanyaan PO terbuka yang memblokir.
    *   `<n>.<m>-<slug>.md`: Satu file per fase yang memuat AC, Design, Gerbang Sec 4 #7, Tasks, Test Results, Review, dan Incidents.
*   **Daftar Periksa Masuk (Entry Checklist per Fase)**:
    1.  *Sumber resmi*: Ada dokumen resmi (DSR/FSD/URS) yang menjelaskan alur dan aturan.
    2.  *Bisa ditunjuk*: Setiap aturan bisnis fase ini dapat dirujuk ke baris/bab dokumen sumber.
    3.  *Konflik PO diputuskan*: Tidak ada pertanyaan terbuka yang mengubah perilaku; jika ada, selesaikan di fase 0 keputusan.
    4.  *Tabel disetujui*: Tabel/kolom baru yang dibutuhkan telah disetujui user.
*   **Wajib Pengisian `## Gerbang Sec 4 #7` (Area Kritis)**:
    Fase yang menyentuh auth, uang/jurnal, rekonsiliasi, migrasi DB, atau delete wajib mengisi:
    *   *Rumus & Contoh Angka*: Formula matematika tertulis lengkap beserta minimal satu contoh angka konkret (input $\to$ output), termasuk uji kasus ambang/batas.
    *   *Persetujuan Skema*: Tabel, kolom, constraint, dan nomor migrasi disetujui user sebelum file migrasi dibuat.
    *   *Test Wajib*: Tes aritmatika uang (presisi, tanpa float), tes penolakan RBAC, tes maker-checker (`Maker != Checker`), dan tes kegagalan audit rollback tx.
    *   *Security Review*: Jalankan review kepatuhan keamanan sebelum gerbang review/merge ditutup.
*   **3 Human Gates ScDD**: (Gate 1) Rencana fase disetujui (`accepted`), (Gate 2) Build + seluruh automated test hijau (`## Test Results`), (Gate 3) Review & DoD selesai (`done <sha>`). Jika terjadi ambiguitas di tengah build: tandai `blocked`, buat AC baru, dan ulangi dari Gate 1.

---

## 5. Navigasi Kode dengan Graphify (Sec 1a)

Proyek ini memiliki knowledge graph di `graphify-out/` (god nodes, struktur komunitas, dan relasi lintas file):
1.  Untuk pertanyaan tentang arsitektur atau simbol kode, jalankan terlebih dahulu:
    *   `graphify query "<pertanyaan>"`
    *   `graphify path "<Simbol A>" "<Simbol B>"` (untuk relasi antar komponen)
    *   `graphify explain "<konsep>"` (untuk penjelasan terfokus)
2.  Jika `graphify-out/wiki/index.md` tersedia, gunakan untuk navigasi luas alih-alih membaca banyak file mentah.
3.  Baca `graphify-out/GRAPH_REPORT.md` hanya untuk tinjauan arsitektur tingkat tinggi atau jika `query`/`path`/`explain` belum memberikan konteks yang cukup.
4.  **Wajib Setelah Modifikasi Kode**: Jalankan `graphify update .` untuk memperbarui graph (hanya berbasis AST lokal, tanpa biaya API).

---

## 6. Peta Lokasi Dokumentasi & Tech-Debt Backlog (Sec 15)

*   **Artefak & Rules Antigravity (Gemini)**:
    *   Master Rules & Steering: `.agents/rules/GEMINI.md` & `.agents/rules/steering/*.md`
    *   Specs, Scopes, Bugfixes, Changes: `.agents/specs/`, `.agents/scopes/`, `.agents/bugfixes.md`, `.agents/changes.md`
    *   Progress & Lessons: `.agents/development-progress.md`, `.agents/lessons-learned.md`
    *   Skills & Subagents: `.agents/skills/*/SKILL.md` (`cms2-master-data`, `cms2-sdlc`, `cms2-bugfix`, `db-migration-sqlc`, `go-chi-endpoint`, `openapi-frontend-sync`, `react-ui-ux-craft`, `sql-query-tuning`, `gemini-cms-orchestrator`, `graphify`) & `.agents/agents/*.md`.
*   **Dokumentasi Referensi Mendalam (*Read on Demand*)**:
    *   Penjelasan sederhana konsep bisnis CMS: [.claude/eli5.md](file:///c:/Users/RB%20Yudha%20Rangga/OneDrive/Documents/Development/CMS2/.claude/eli5.md)
    *   Ringkasan URS lengkap: [.claude/docs/requirements.md](file:///c:/Users/RB%20Yudha%20Rangga/OneDrive/Documents/Development/CMS2/.claude/docs/requirements.md)
    *   Catatan tabel & kolom detail per migrasi: [.claude/docs/data-map.md](file:///c:/Users/RB%20Yudha%20Rangga/OneDrive/Documents/Development/CMS2/.claude/docs/data-map.md)
    *   Log keputusan arsitektur & PO lengkap: [.claude/docs/decisions.md](file:///c:/Users/RB%20Yudha%20Rangga/OneDrive/Documents/Development/CMS2/.claude/docs/decisions.md)
    *   Panduan deployment GCP & produksi: [.claude/docs/deployment.md](file:///c:/Users/RB%20Yudha%20Rangga/OneDrive/Documents/Development/CMS2/.claude/docs/deployment.md)
    *   Aturan palet & komponen UI frontend: [.claude/rules/frontend-ui.md](file:///c:/Users/RB%20Yudha%20Rangga/OneDrive/Documents/Development/CMS2/.claude/rules/frontend-ui.md)
*   **Tech-Debt Backlog (*Read on Demand*)**:
    *   Frontend Tech Debt: [.claude/FE_debt.md](file:///c:/Users/RB%20Yudha%20Rangga/OneDrive/Documents/Development/CMS2/.claude/FE_debt.md) (`FE-xx`)
    *   Backend Tech Debt: [.claude/BE_debt.md](file:///c:/Users/RB%20Yudha%20Rangga/OneDrive/Documents/Development/CMS2/.claude/BE_debt.md) (`BE-xx`)
    *   *Aturan*: Perbarui kolom `Status` item (`open` → `in-progress` → `done <sha>`) saat Anda mengerjakan item tech-debt tersebut.
