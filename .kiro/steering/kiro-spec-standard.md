---
inclusion: auto
name: kiro-spec-standard
description: Standar format spec Kiro (.kiro/specs/<feature>/ requirements.md, design.md, tasks.md, .config.kiro) dan checklist traceability requirement. Use when creating, editing, reviewing, or validating a spec, requirements, design, or tasks document.
---

# Standar Spec Kiro (CMS2)

Berlaku untuk setiap spec di `.kiro/specs/<feature>/`. Validator Kiro mencocokkan **teks heading `##` secara literal**, bukan isi. Konten yang benar di bawah heading yang salah tetap gagal validasi. Contoh spec yang lolos: `.kiro/specs/done/replenishment-request-enhancements/`, `.kiro/specs/done/cms-app-foundation/`.

Spec ditulis dalam bahasa Indonesian (mengikuti spec yang ada); heading kanonis di bawah tetap bahasa Inggris persis seperti tertulis.

## 1. Struktur folder

```
.kiro/specs/<feature>/
  .config.kiro      {"specId": "<uuid>", "workflowType": "requirements-first", "specType": "feature"}
  requirements.md
  design.md
  tasks.md
  review.md         (tahap 5, ditambahkan setelah build; lihat Sec 7)
  incidents.md      (tahap 6, dibuat saat insiden pertama; lihat Sec 7)
```

Spec selesai dipindah ke `.kiro/specs/done/`.

**Baris status (wajib, semua file di atas):** tepat di bawah judul `#`, satu baris `> Status: draft` atau `> Status: accepted YYYY-MM-DD (<peran penyetuju>)`. Hanya manusia yang mengubah ke `accepted`; AI tidak pernah menyetujui sendiri. Baris ini bukan heading, jadi tidak mengganggu validator.

## 2. `requirements.md`

Heading wajib (`##`, persis):
- `# Requirements Document` (judul)
- `## Introduction` — masalah, kondisi kode saat ini (dicek, dengan tanggal), ruang lingkup + tidak termasuk, area sensitif (project-context Sec 3 rule 7), keputusan/asumsi PO.
- `## Glossary` — setiap istilah/komponen yang dipakai sebagai subjek acceptance criteria (mis. `VendorOrderAPI`) didefinisikan di sini.
- `## Requirements` — tiap requirement `### Requirement N: <judul>` berisi `**User Story:**` lalu `#### Acceptance Criteria` dengan daftar bernomor format EARS: `WHEN/IF/WHILE … THE <Komponen> SHALL …`.

Extra `##` (mis. `## Open Questions`, `## Dependency & Scope Note`) boleh. Open question/asumsi harus ditandai status (✅ DIPUTUSKAN PO <tanggal> / mengikuti saran) sebelum design ditulis.

## 3. `design.md`

Heading wajib (`##`, persis, termasuk huruf besar):
- `## Overview`
- `## Architecture` — diagram (Mermaid) + alur data.
- `## Components and Interfaces` — backend/frontend/API sebagai `###`, detail sebagai `####`.
- `## Data Models` — migrasi, kolom, constraint (tulis "Tidak ada tabel baru" bila memang tidak ada).
- `## Correctness Properties` — tiap property `### Property N: <judul>`, kalimat *For any …*, lalu `**Validates: Requirements X.Y, …**`.
- `## Error Handling`
- `## Testing Strategy`

Extra `##` (mis. `## Files touched`, `## Documented deviation`, `## Seed dev`) boleh, diletakkan setelah section wajib yang relevan.

## 4. `tasks.md`

Hanya empat heading `##`, urut:
1. `## Overview` — ringkasan urutan implementasi, referensi pola, lalu `### Model Split` (wajib, `model-recommendation.md`): **Sonnet-runnable** dan **Opus-required** (dengan alasan zona bahaya), plus dependensi lintas grup.
2. `## Tasks` — setiap task berupa checkbox `- [ ] N. Judul`, sub-task `- [ ] N.M Judul` (indentasi 2 spasi). **Jangan** pakai heading `## Task N` / `### Task N`. Test opsional ditandai `- [ ]* N.M`. Setiap task/sub-task punya `_Requirements: X.Y, …_`; setiap top-level task punya `_Model: <Tier>, Effort: <Level> — alasan_`.
3. `## Notes` — catatan bebas; konten tambahan (Definition of Done, sequencing) sebagai `###` di sini.
4. `## Task Dependency Graph` — blok ```json``` format objek:
   ```json
   { "waves": [{ "id": 0, "tasks": ["1", "2.1"] }, { "id": 1, "tasks": ["2.2"] }] }
   ```
   ID berupa string, memakai ID leaf (sub-task bila ada, top-level bila tanpa sub-task). **Semua** leaf task wajib masuk tepat satu kali: termasuk task checkpoint/gate kualitas, task test opsional (`*`), dan task penutup (dokumentasi, cek browser). Parent yang punya sub-task diwakili oleh seluruh sub-task-nya (ID parent tidak ditulis terpisah). Graph = urutan pengerjaan lengkap, jadi checkpoint ditaruh di wave setelah semua task yang di-gate-nya. Task yang menyentuh file yang sama diserialkan ke wave berbeda. Array polos (`[["1"]]`) membuat parser gagal.
   Tulis juga ringkasan urutan per wave dalam prosa (bullet `**Wave N:** …` + dependensinya) tepat di atas blok JSON.

## 5. Checklist traceability (wajib sebelum spec dianggap selesai)

Jalankan saat menulis maupun me-review spec:
- [ ] **Setiap acceptance criterion** (`N.M`) di `requirements.md` dirujuk minimal satu `_Requirements:_` di `tasks.md`. Termasuk criteria "tidak berubah"/regresi (dirujuk oleh task test regresi) dan criteria coverage/testing (dirujuk oleh task checkpoint).
- [ ] Setiap `_Requirements:_` di `tasks.md` menunjuk criterion yang benar-benar ada (tidak ada ID hantu setelah renumber).
- [ ] Setiap requirement di-cover design (section Components/Data Models menyebut `Req N`), dan setiap property punya `**Validates:**`.
- [ ] Setiap Correctness Property punya task test (property test atau integration test) yang menyebut nomor property-nya.
- [ ] Setiap keputusan PO (D1…Dn) tercermin di requirement dan task yang relevan.
- [ ] Model Split cocok dengan catatan `_Model:_` per task; setiap task muncul tepat di satu grup.
- [ ] Task Dependency Graph memuat **setiap** leaf task (termasuk checkpoint, test opsional `*`, dan task penutup) tepat satu kali, tanpa ID hantu, dan setiap task berada di wave setelah semua dependensinya. Cek: daftar semua checkbox `- [ ] N.` / `- [ ] N.M` di `## Tasks` (parent dengan sub-task diganti sub-task-nya), bandingkan dengan gabungan ID di `waves`; selisih harus kosong di dua arah.
- [ ] Nomor migrasi yang dirujuk belum terpakai di `backend/migrations/` (cek file terakhir; `development-plan.md` bisa basi).
- [ ] Grep folder spec untuk istilah/ID lama setelah rename. Catatan: sub-nomor criteria tampil sebagai nomor list (`5.`), jadi `grep "14.5"` bisa kosong walau criterion ada; hitung item di bawah heading requirement.

Cara cepat memeriksa cakupan: daftar semua ID `N.M` dari requirements (hitung item list per `### Requirement N`), lalu bandingkan dengan gabungan semua angka di baris `_Requirements:_` tasks.md. Selisihnya = requirement yang missing.

## 6. Konvensi proses

- Tulis berurutan requirements → design → tasks, tunggu persetujuan user di antara dokumen kecuali user meminta lanjut (lihat memory `cms2-kiro-spec-location`).
- Saat review "pastikan tidak ada section yang missing", periksa **ketiga** sumber aturan: validator Kiro (section ini), `model-recommendation.md`, dan checklist traceability. Lolos satu tidak berarti lolos yang lain.

## 7. Standar hybrid CMS2: tahap SDLC + gate (keputusan user 2026-11-09)

Spec Kiro adalah standar SDLC CMS2 untuk fitur baru (CLAUDE.md §4a). Enam tahap AI-Native SDLC dipetakan ke file berikut; setiap gate dicatat di baris `> Status:` file-nya.

| Tahap | File | Gate manusia | Isi minimal |
|---|---|---|---|
| 1 Plan | `requirements.md` → `## Introduction` | PO menyetujui Introduction + open questions dijawab | Masalah dengan kata-kata pemohon, outcome, batasan, open questions (AI mendaftar, tidak menjawab) |
| 2 Design | `requirements.md` (EARS) + `design.md` | PO menyetujui requirements; engineer menyetujui design | Sec 2–3 di atas; tandai area CLAUDE.md §4 #7 (auth/uang/rekon/migrasi/delete) |
| 3 Build | `tasks.md` | Engineer menyetujui tasks | Sec 4 di atas |
| 4 Test | `tasks.md` → `## Notes` → `### Test Results` | Build + test hijau | Perintah yang dijalankan + hasil (tanggal, angka lulus/coverage), trace ke Requirement/Property; cek browser manual = **outstanding (user)**, tidak pernah ditandai selesai |
| 5 Deploy | `review.md` | Code owner merge | `## Review Passes` (bugs / security / compliance), `## Findings` (R1…Rn, Important vs nit, status), `## Definition of Done` (checklist CLAUDE.md §11) |
| 6 Maintain | `incidents.md` | On-call | `## Signals`, `## Incident Log` (tanggal, gejala, akar masalah, test regresi yang ditambahkan); pelanggaran → spec baru (loop tertutup) |

Aturan:
- Satu tahap pada satu waktu; tahap berikutnya baru mulai setelah file sebelumnya `accepted`.
- Kode dan spec tidak cocok → STOP, tanya. Jangan diam-diam memperbaiki salah satunya.
- Saat tahap selesai: perbarui `.claude/development-progress.md`.
- Pelajaran dari review/insiden → `.claude/lessons-learned.md`.
- Fitur yang dimulai di `.claude/sdlc/<feature>/` (format lama) diselesaikan di sana; tidak dimigrasi.
