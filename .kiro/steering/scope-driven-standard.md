---
inclusion: auto
name: scope-driven-standard
description: Standar Scope Driven Development (ScDD) CMS2 — rencana implementasi per fase di .claude/dev-plan/plan-perscope/dev-phase-perscope/<n>-<scope>/ (README roadmap + satu file per fase berisi AC, design, tasks, test, review, insiden), tanpa requirements EARS. Use when creating, editing, reviewing, or executing a ScDD scope, roadmap README, or phase file.
---

# Standar Scope Driven Development (ScDD) — CMS2

> Status: draft — hasil grilling Q1–Q21 dengan user, 2026-11-12.

ScDD adalah metode kedua di samping spec Kiro (`kiro-spec-standard.md`). Dipakai saat **requirement sudah jelas** di dokumen sumber resmi (DSR/FSD/URS). Requirement tidak ditulis ulang dalam format EARS; gantinya tiap fase memuat **acceptance criteria (AC) bernomor** yang menunjuk baris sumber, lalu design + tasks langsung di file yang sama. Singkatan: **ScDD** (bukan "SDD", yang biasa berarti *Spec* Driven Development).

Ditulis dalam Bahasa Indonesia. Heading `##` di template Sec 5–6 dipakai persis (alat `cms-task-runner` / `claude-cms-orchestrator` mencarinya secara literal).

## 1. Kapan dipakai

- Hanya untuk **scope baru**. Spec Kiro yang sudah berjalan diselesaikan di Kiro; fitur `.claude/sdlc/` di sana. Tidak ada migrasi antar metode.
- Pemilihan metode (CLAUDE.md §4a rule 0): AI mengecek daftar periksa Sec 2 lalu **menyarankan** Kiro / ScDD / dokumentasi bugfix beserta poin yang lolos/gagal. User yang memutuskan.

## 2. Daftar periksa masuk (per fase)

Dinilai **per fase**, sebelum gerbang 1 fase itu (Sec 4). Hasilnya dicatat di README (Sec 5).

1. **Sumber resmi** — ada dokumen (DSR/FSD/URS) yang menjelaskan alur, rumus, dan aturan fase ini.
2. **Bisa ditunjuk** — setiap aturan bisnis fase ini bisa dirujuk ke bagian/baris sumbernya.
3. **Konflik PO diputuskan** — tidak ada pertanyaan PO terbuka yang mengubah perilaku fase ini.
4. **Tabel disetujui** — tabel/kolom baru yang dibutuhkan sudah disetujui (CLAUDE.md §3).

Poin gagal → dikerjakan di **fase 0 keputusan** (`<n>.0-keputusan.md`): tanpa kode, berisi daftar pertanyaan + jawaban (tanggal, pemberi keputusan). Hanya fase yang bergantung pada poin itu yang menunggu; fase lain boleh jalan. Bila poin tidak bisa diselesaikan dalam scope ini → sarankan Kiro untuk fase itu.

## 3. Area CLAUDE.md §4 #7 (auth, uang/journal, rekonsiliasi, migrasi, delete)

Boleh memakai ScDD. Fase yang menyentuh area ini **wajib** mengisi `## Gerbang Sec 4 #7`:
- **Rumus** tertulis lengkap + minimal satu contoh angka per rumus (input → output), termasuk kasus batas (mis. tepat di ambang).
- **Persetujuan skema**: tabel/kolom/constraint baru disetujui user sebelum file migrasi ditulis; nomor migrasi dicek belum terpakai.
- **Test wajib**: uang (presisi, tanpa float), RBAC (role ditolak), maker-checker (maker ≠ checker, efek hanya setelah approve), audit dalam tx — sesuai CLAUDE.md §8.
- **Review security** (`/security-review`) sebelum gerbang 3.
- Deviasi GR#3 (bila ada) ditulis dan dijustifikasi.

## 4. Gerbang

Hanya manusia yang menyetujui; AI tidak pernah menulis `accepted` sendiri.

| Gerbang | Kapan | Bukti |
|---|---|---|
| R. Roadmap | sekali per scope, sebelum fase pertama ditulis detail | `README.md` → `> Status: accepted …` |
| 1. Rencana fase | AC + design + tasks lengkap, daftar periksa fase ✅ | file fase → `> Status: accepted …` |
| 2. Build + test | semua task `[x]`, perintah test hijau | `## Test Results` terisi |
| 3. Review / merge | review selesai, code owner merge | `## Review` terisi, status `done` |

**Baris status** (tepat di bawah judul `#`): `> Status: draft` · `accepted YYYY-MM-DD (<peran>)` · `blocked YYYY-MM-DD (<alasan singkat>)` · `done YYYY-MM-DD <sha>`.

**Ambiguitas di tengah build**: fase → `blocked`, pertanyaan dicatat di `## Pertanyaan Terbuka & Risiko`, task terdampak ditahan. Setelah dijawab → AC baru/revisi (nomor lama tidak dipakai ulang) → gerbang 1 diulang. Jawaban yang mengubah >1 fase atau pembagian fase → gerbang R diulang juga. AI tidak menafsirkan sendiri (CLAUDE.md §4 rule 3).

## 5. Struktur folder + `README.md`

```
.claude/dev-plan/plan-perscope/
  <n>-<scope>.md                    # file area (ringkasan): checklist area + konflik PO — HANYA di sini
  dev-phase-perscope/<n>-<scope>/
    README.md                       # roadmap scope
    <n>.0-keputusan.md              # opsional, fase 0 tanpa kode
    <n>.1-<slug>.md                 # satu file per fase
```

Template `README.md`:

```markdown
# <n> <Nama scope> — Roadmap ScDD
> Status: draft

Area: [`../../<n>-<scope>.md`](../../<n>-<scope>.md) (checklist area + konflik PO). Sumber: <dokumen §bagian>.

## Daftar Periksa
| Fase | Sumber | Bisa ditunjuk | Konflik PO | Tabel | Catatan |
|---|---|---|---|---|---|
| <n>.1 | ✅ | ✅ | ❌ #5 | ✅ | menunggu <n>.0 |

## Fase
| Fase | Nama | File | Status | Ukuran |
|---|---|---|---|---|
| <n>.1 | … | [<n>.1-<slug>.md](<n>.1-<slug>.md) | draft | M |

Status: `draft` / `accepted` / `in-build` / `blocked` / `done <sha>`.

## Ketergantungan
- <n>.2 setelah <n>.1; <n>.3 ∥ <n>.4 setelah <n>.2.

## Keputusan Scope
- YYYY-MM-DD (<pemberi>): <keputusan yang berlaku untuk >1 fase>.
```

Keputusan yang hanya berlaku untuk satu fase ditulis di file fase itu.

## 6. Template file fase

````markdown
# <n>.<m> <Judul fase>
> Status: draft

Sumber: <dokumen §bagian, baris L…–L…> · Prasyarat: <fase / keputusan> · Ukuran: S|M|L

## Tujuan & Scope
<1–2 kalimat hasil bisnis.>
- Masuk: …
- Tidak termasuk: …

## Acceptance Criteria
- **AC-<n>.<m>-01** (DSR L512–520): <perilaku yang bisa diuji, kalimat biasa>.
- **AC-<n>.<m>-02** (FSD §4.2): …

## Design
### Data
<migrasi/tabel/kolom/constraint, atau "Tidak ada tabel baru".>
### Backend / API
### Frontend
### Python / EOD
### Kontrol
<role + RBAC (route + service), maker-checker atau deviasi GR#3, audit, primary vs replica.>

## Gerbang Sec 4 #7
<Sec 3 di atas, atau "Tidak berlaku: <alasan>".>

## Tasks
- [ ] 1. <Judul>
  _AC: AC-<n>.<m>-01_ · _Model: Opus, Effort: High — <alasan>_
  - [ ] 1.1 <Sub-task> — _AC: AC-<n>.<m>-01_

### Task Dependency Graph
- **Wave 0:** 1.1 …
```json
{ "waves": [{ "id": 0, "tasks": ["1.1"] }] }
```

## Pertanyaan Terbuka & Risiko

## Test Results
### Catatan eksekusi

## Review

## Incidents
````

Aturan tasks = `kiro-spec-standard.md` Sec 4 (checkbox, tanpa heading per task, `- [ ]*` untuk test opsional, `_Model:_` di setiap top-level task, format JSON waves, semua leaf tepat satu kali, task yang menyentuh file sama diserialkan), dengan `_AC:_` menggantikan `_Requirements:_`. Tanpa `### Model Split` terpisah.

Isi bagian setelah build:
- **Test Results**: perintah (CLAUDE.md §2a), tanggal, angka lulus/coverage, trace ke AC. Cek browser manual = **outstanding (user)**, tidak pernah ditandai selesai.
- **Review**: pass bugs / security / compliance, temuan R1…Rn (Important vs nit, status), checklist DoD CLAUDE.md §11.
- **Incidents**: tanggal, gejala, akar masalah, test regresi yang ditambahkan. Pelanggaran besar → fase baru atau spec baru.

## 7. Checklist traceability (sebelum gerbang 1 dan 2)

- [ ] Setiap AC dirujuk minimal satu `_AC:_` di Tasks; setiap `_AC:_` menunjuk AC yang ada.
- [ ] Setiap AC menunjuk sumbernya; AC tanpa sumber = keputusan → harus ada di `## Keputusan Scope` atau fase 0.
- [ ] Task Dependency Graph memuat setiap leaf task tepat sekali, tanpa ID hantu, dependensi di wave sebelumnya.
- [ ] Fase §4 #7: `## Gerbang Sec 4 #7` terisi lengkap (Sec 3).
- [ ] Nomor migrasi yang dirujuk belum terpakai di `backend/migrations/`.

## 8. Konvensi proses

- Penamaan: folder `<n>-<scope>` (kebab-case), file `<n>.<m>-<slug>.md`, fase keputusan `<n>.0-keputusan.md`.
- Commit hanya saat user bilang "commit"; pesan menyebut fase + task (`feat: cash count 5.1 T3 …`).
- Standar test CLAUDE.md §8 tidak berubah.
- `.claude/development-progress.md`: **satu baris per scope**, ringkasan status fase + tautan ke README; diperbarui di setiap gerbang.
- Kode dan file fase tidak cocok → STOP, tanya.
- Pelajaran dari review/insiden → `.claude/lessons-learned.md`.
- Alat: skill `cms2-scope` (membuat + memandu gerbang), `claude-cms-orchestrator` + agent `cms-task-runner` (eksekusi task; input = path file fase).
