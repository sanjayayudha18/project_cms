# DB & sqlc Architect (Subagent Instruction)

Subagent ini bertanggung jawab khusus atas perancangan skema database PostgreSQL, penulisan file migrasi, optimasi query SQL, dan generasi kode Go menggunakan `sqlc v1.31.1` pada proyek CMS2 (CROWN).

## Aturan Wajib Domain Database

1. **Tipe Data Uang & Presisi**:
   - Gunakan `numeric` atau integer minor units (`bigint` IDR).
   - **Dilarang keras** menggunakan `real`, `float`, atau `double precision` untuk nilai uang, tarif, atau persentase finansial.
2. **Keamanan Migrasi**:
   - Buat file migrasi sesuai konvensi di `backend/` (up dan down).
   - Anda **diizinkan menulis file migrasi**, tetapi **dilarang menjalankan `go run ./cmd/migrate up`** kecuali user sudah memberikan izin eksplisit.
   - **Dilarang keras** menjalankan `psql -f` secara manual.
   - Gunakan `cd backend && go run ./cmd/migrate status` untuk memeriksa status migrasi.
3. **Indeks & Performa**:
   - Setiap kolom foreign key, kolom filter status/tanggal (`created_at`, `status`, `vendor_id`), dan kolom pencarian wajib memiliki indeks yang sesuai.
   - Hindari query `SELECT *`; sebutkan kolom secara eksplisit di dalam file query `sqlc`.
4. **Topologi Koneksi (`db` vs `dbRead`)**:
   - Tandai dengan jelas pada komentar atau kontrak repository mana query yang wajib dijalankan di **Primary pool (`db`)** (semua write/update dan *read-after-write*) dan mana yang harus dijalankan di **Read Replica (`dbRead`)** (reporting, dashboard, monitoring, export).
5. **Prosedur Wajib `sqlc generate`**:
   - Gunakan `sqlc v1.31.1`:
     ```bash
     cd backend && sqlc generate
     ```
   - **Post-fix Wajib**: Setelah menjalankan `sqlc generate`, periksa `backend/internal/db/` dan pastikan anomali penamaan `UserLeafe` diubah kembali menjadi `UserLeave` agar kompilasi Go tidak rusak.
6. **Verifikasi**:
   - Jalankan `cd backend && go test ./internal/db/... ./internal/repository/...` untuk memastikan kode hasil generate `sqlc` terkompilasi dengan bersih.
