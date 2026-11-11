---
name: sql-query-tuning
description: >-
  Guide and checklist for optimizing PostgreSQL queries, indexing strategies,
  preventing N+1 queries, pagination on large tables, and routing heavy
  reporting/export workloads to dbRead in CMS2. Use when diagnosing slow
  endpoints, designing reporting/dashboard queries, or handling large batch EOD
  and export operations.
---

# PostgreSQL Query Tuning & Performance (`sql-query-tuning`)

Gunakan skill ini saat merancang query laporan/dashboard berkapasitas besar, mengoptimalkan proses rekonsiliasi/EOD, atau memperbaiki endpoint yang lambat.

---

## Checklist Optimasi Database CMS2

### 1. Routing Beban Kerja (`db` vs `dbRead`)
- **Wajib `dbRead` (Read Replica)**:
  - Seluruh query dashboard, ringkasan monitoring, laporan historis, dan ekspor CSV/XLSX di `backend/` maupun `backend-exq/` wajib diarahkan ke pool `dbRead`.
- **Wajib `db` (Primary)**:
  - Hanya operasi mutasi (`INSERT`, `UPDATE`) dan pembacaan *read-after-write* yang membutuhkan konsistensi instan dalam alur transaksi/maker-checker.

### 2. Pencegahan N+1 Query
- **Dilarang** memanggil query SQL di dalam perulangan `for` di Go atau Python (misalnya mengambil detail cabang/vault satu per satu untuk setiap baris DSR atau ATM).
- **Solusi**:
  - Gunakan `JOIN` eksplisit jika relasinya 1:1 atau N:1.
  - Gunakan parameter array `WHERE id = ANY($1::uuid[])` (atau `bigint[]` / `text[]`) untuk mengambil data relasi secara *batch* dalam satu kali round-trip.
  - Untuk batch insert/update besar, gunakan `COPYFrom` (`pgx`) atau `UNNEST` array parameter.

### 3. Strategi Indexing & Filter SARGable
- **Filter SARGable (Search Argument Able)**:
  - Hindari membungkus kolom dengan fungsi di sisi kiri operator (contoh buruk: `WHERE DATE(created_at) = $1` atau `WHERE LOWER(code) = LOWER($1)` tanpa functional index).
  - Gunakan rentang waktu eksplisit agar B-Tree index terpakai:
    ```sql
    WHERE created_at >= $1 AND created_at < $2
    ```
- **Composite Index**:
  - Urutkan kolom pada composite index berdasarkan: **(1) Kesamaan (`=`)** -> **(2) Pengurutan (`ORDER BY`) / Rentang (`>=`, `<`)**.
  - Contoh: untuk query `WHERE vendor_id = $1 AND is_active = true ORDER BY created_at DESC`, indeks yang optimal adalah `(vendor_id, is_active, created_at DESC)`.

### 4. Pagination & Streaming untuk Ekspor Data Besar
- Jangan pernah memuat puluhan ribu baris sekaligus ke slice memori tanpa batas (`LIMIT`) pada endpoint HTTP biasa.
- Untuk fitur ekspor asinkron di `backend-exq/`:
  - Gunakan pendekatan *chunking / keyset pagination* (`WHERE id > $last_id ORDER BY id ASC LIMIT $batch_size`) atau *cursor/rows iteration* dari `dbRead` agar konsumsi memori tetap datar dan tidak menyebabkan *Out-of-Memory (OOM)*.

### 5. Akurasi Agregasi Finansial di SQL
- Saat melakukan `SUM(...)` pada kolom nominal uang (`bigint` / `numeric`), gunakan `COALESCE(SUM(amount), 0)` agar tidak mengembalikan `NULL` saat tidak ada baris yang cocok.
- Pastikan hasil agregasi `SUM(bigint)` di-cast atau dipetakan ke tipe yang aman dari overflow di `sqlc`.
