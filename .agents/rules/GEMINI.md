# GEMINI.md — Cash Management System (CMS2 / CROWN)

> **Template Standar Minimal (`.agents/rules/GEMINI.md`)**
> File ini otomatis dimuat oleh Google Antigravity di setiap sesi. Batas ukuran per file di `.agents/rules/` adalah **24 KB** — jika aturan berkembang melebihi batas tersebut, pecah menjadi beberapa file `.md` di dalam folder `.agents/rules/`.

---

## 0. Golden Rules

1. **Tech Stack Tetap**: Go + Chi v5 (`backend/`, `backend-exq/`, `pkg/`), sqlc `v1.31.1`, Python EOD (`backend_python/`), React + Vite (`frontend/CompanyPortal-Vite`, `frontend/VendorPortal-Vite`), PostgreSQL (primary + read replica), Redis. Jangan menambah library/framework baru tanpa persetujuan.
2. **Maker-Checker & Audit**: Setiap perubahan state pada data master atau finansial wajib melalui mekanisme maker-checker (`approval_requests`) dan mencatat `audit_logs` dalam transaksi yang sama.
3. **Tipe Data Uang**: Gunakan `numeric` atau integer minor units (`bigint` IDR). **Dilarang keras** menggunakan `float` / `double`.
4. **Topologi Database**:
   - **Primary pool (`db`)**: Semua operasi write/update dan read-after-write.
   - **Read replica pool (`dbRead`)**: Reporting, dashboard, monitoring, dan ekspor.
5. **Dua Frontend Terpisah**:
   - `CompanyPortal-Vite` (Internal — autentikasi LDAP -> JWT)
   - `VendorPortal-Vite` (Vendor — autentikasi kredensial lokal -> JWT)
6. **Bentuk Response JSON**:
   - `backend/` (ATM, port 8080): Flat JSON (kompatibilitas dengan frontend).
   - `backend-exq/` (Async export & monitoring, port 8081): Menggunakan envelope `pkg/response` (`{success, data}`).
7. **Verifikasi Browser Manual**: Pengecekan UI secara manual di browser dilakukan oleh **user**, bukan diklaim selesai oleh agen. Jalankan automated test, lint, dan build, lalu catat verifikasi browser sebagai `outstanding (user)`.

---

## 1. Perintah Utama (Commands)

```bash
# Go Backend (jalankan per modul)
cd backend && go test ./...
cd backend-exq && go test ./...
cd pkg && go test ./...
cd backend && golangci-lint run --build-tags integration ./...

# sqlc (wajib v1.31.1; setelah generate, ubah UserLeafe -> UserLeave di backend/internal/db/)
cd backend && sqlc generate

# OpenAPI Contract
cd backend && go test ./internal/handler -run OpenAPI
pnpm --dir frontend/CompanyPortal-Vite run gen:api

# Frontend
pnpm --dir frontend/CompanyPortal-Vite run dev|build|test|lint
pnpm --dir frontend/VendorPortal-Vite  run dev|build|test|lint

# Python EOD
cd backend_python && python -m unittest discover -s dsr && python -m unittest discover -s itm/cashpos

# Migrasi Database (jangan pernah jalankan psql -f manual)
cd backend && go run ./cmd/migrate status
cd backend && go run ./cmd/migrate up
```

---

## 2. Alur Kerja SDLC & Lokasi Artefak (Khusus Gemini)

Seluruh pekerjaan fitur dan perbaikan bug yang dijalankan menggunakan Gemini/Antigravity dicatat di dalam `.agents/` agar terpisah dari `.claude/` dan `.kiro/`:

1. **Tanya Jalur Terlebih Dahulu**: Sebelum menulis kode untuk fitur atau perbaikan, selalu tanyakan kepada user:
   *"Perlu dibuat spec baru (`.agents/specs/<feature>/`) atau cukup dokumentasi bug fix (`.agents/bugfixes.md`)?"*
2. **Lokasi Artefak Gemini**:
   - **Spec Fitur (6 Tahap)**: `.agents/specs/<feature>/` (`requirements.md` -> `design.md` -> `tasks.md` -> `review.md` -> `incidents.md`)
   - **Spec Selesai**: `.agents/specs/done/<feature>/`
   - **Log Bug Fix**: `.agents/bugfixes.md`
   - **Tracking Progress**: `.agents/development-progress.md`
   - **Lessons Learned**: `.agents/lessons-learned.md`
3. **Area Kritis (Wajib Spec & Gate Manusia)**: Perubahan yang menyentuh autentikasi, perhitungan uang/jurnal, rekonsiliasi, migrasi skema database, atau penghapusan data wajib melalui persetujuan user terlebih dahulu.
4. **Satu Dokumen SDLC per Langkah (Sequential Gate)**: Tulis artefak SDLC **satu per satu** secara berurutan. Selesaikan `requirements.md` terlebih dahulu dan tunggu review/persetujuan user (`accepted`) sebelum menulis `design.md`; selesaikan `design.md` dan tunggu persetujuan user sebelum menulis `tasks.md`, dan seterusnya. **Dilarang keras** membuat dua file artefak tahap berbeda (misalnya `requirements.md` dan `design.md`) sekaligus dalam satu giliran.
5. **Saran Langkah Selanjutnya**: Setelah selesai melakukan task, jika memungkinkan berikan user saran apa yang bisa dilakukan selanjutnya.
6. **Eksplorasi Kode dengan Graphify**: Sebelum membaca banyak file kode atau melakukan grepping luas, manfaatkan knowledge graph di `graphify-out/`. Gunakan `graphify query "<pertanyaan>"`, `graphify path "<A>" "<B>"`, atau `graphify explain "<konsep>"` untuk menelusuri simbol, relasi, dan dependensi kode secara efisien. Setelah mengubah kode, jalankan `graphify update .` untuk memperbarui graph.

---

## 3. Catatan Tambahan Proyek

*(Tambahkan aturan spesifik domain, pemetaan tabel baru, atau keputusan arsitektur di bagian ini sesuai kebutuhan)*
