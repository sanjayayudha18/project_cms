---
name: db-migration-sqlc
description: >-
  Checklist and workflow for PostgreSQL schema migrations, writing sqlc queries
  in backend/queries/*.sql, enforcing soft-delete and numeric money rules,
  separating primary (db) vs read-replica (dbRead) queries, and running sqlc
  v1.31.1 codegen cleanly. Use when adding/altering tables, indexes, or SQL
  queries in backend/.
---

# Database Migration & `sqlc` Workflow (`db-migration-sqlc`)

Gunakan skill ini setiap kali task menyentuh skema tabel PostgreSQL, pembuatan indeks, atau penulisan/perubahan file SQL di `backend/queries/*.sql`.

> **Gate Area Kritis**: Perubahan skema database wajib memiliki spec yang disetujui di `.agents/specs/<feature>/` sebelum dieksekusi. Menulis file migrasi diperbolehkan, tetapi menjalankan `go run ./cmd/migrate up` memerlukan izin eksplisit user. **Dilarang keras** menjalankan `psql -f` secara manual.

---

## Checklist 6 Langkah

### 1. Desain Skema & File Migrasi
- Pastikan penamaan file migrasi berurutan sesuai penomoran terakhir di folder migrasi `backend/`.
- **Aturan Tipe Data Uang**: Gunakan `numeric` atau integer minor units (`bigint` IDR). **Dilarang keras** menggunakan `float`, `real`, atau `double precision`.
- **Aturan Soft Delete**: Tabel bisnis/master tidak boleh dihapus permanen (`DELETE FROM`). Gunakan kolom `is_active boolean NOT NULL DEFAULT true` dan `deleted_at timestamptz` (atau `effective_end_date` untuk tabel *effective-dated*).
- Jika menambahkan tabel baru yang menggunakan soft-delete, daftarkan nama tabel ke dalam `backend/internal/repository/no_hard_delete_test.go`.
- **Indeks Wajib**:
  - Tambahkan index pada setiap kolom Foreign Key (`vendor_id`, `branch_id`, `vault_id`, `atm_id`, dll.).
  - Tambahkan composite index pada kolom filter yang sering dipakai bersama (`status`, `is_active`, `created_at`).

### 2. Cek Status Migrasi
- Untuk memeriksa status migrasi saat ini:
  ```bash
  cd backend && go run ./cmd/migrate status
  ```
- **Jangan** jalankan `cd backend && go run ./cmd/migrate up` tanpa persetujuan eksplisit dari user.

### 3. Penulisan Query `sqlc` (`backend/queries/*.sql`)
- Hindari `SELECT *`; tuliskan daftar kolom secara eksplisit agar perubahan urutan kolom di masa depan tidak merusak pemetaan struct Go.
- Gunakan anotasi `sqlc` yang tepat (`:one`, `:many`, `:exec`, `:execrows`).
- Untuk operasi mutasi pada applier maker-checker, sertakan guard kepemilikan/status aktif langsung di klausa `WHERE` (misalnya `WHERE id = $1 AND vendor_id = $2 AND is_active = true`).

### 4. Jalankan `sqlc generate` & Post-Fix Wajib
1. Jalankan generator `sqlc` (wajib versi `v1.31.1`):
   ```bash
   cd backend && sqlc generate
   ```
2. **Post-Fix Wajib (`UserLeafe` -> `UserLeave`)**:
   - `sqlc` mengubah bentuk tunggal dari `user_leaves` menjadi `UserLeafe`. Setelah `sqlc generate`, periksa `backend/internal/db/` dan ubah kembali `UserLeafe` menjadi `UserLeave` agar kode di package lain tetap terkompilasi.
3. Periksa diff hasil generate:
   ```bash
   cd backend && git diff --stat internal/db/
   ```
   Pastikan hanya file yang relevan dengan perubahan Anda yang terpengaruh.

### 5. Pemisahan Koneksi di Repository (`db` vs `dbRead`)
- Saat membungkus query `sqlc` di `backend/internal/repository/`:
  - Gunakan **Primary pool (`db` / `queries`)** untuk seluruh operasi write/update, transaksi (`tx`), dan pembacaan *read-after-write* (termasuk pembacaan snapshot `Before` saat submit maker-checker).
  - Gunakan **Read Replica pool (`dbRead` / `readQueries`)** untuk daftar tabel (List), dashboard, monitoring, dan laporan ekspor.
- Petakan `pgx.ErrNoRows` menjadi `nil, nil` (atau sentinel error domain sesuai pola repository terkait).

### 6. Verifikasi Kompilasi & Test
- Jalankan unit test repository dan pastikan test pencegahan hard-delete lulus:
  ```bash
  cd backend && go test ./internal/db/... ./internal/repository/...
  ```
