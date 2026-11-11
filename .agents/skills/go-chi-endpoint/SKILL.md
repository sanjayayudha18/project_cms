---
name: go-chi-endpoint
description: >-
  Checklist for building or modifying Go + Chi v5 HTTP endpoints in backend/
  (port 8080, flat JSON) or backend-exq/ (port 8081, pkg/response envelope),
  enforcing layer separation (handler -> service -> repository), maker-checker
  + atomic audit_logs, DB topology (db vs dbRead), and table-driven tests.
---

# Go + Chi v5 Endpoint Development (`go-chi-endpoint`)

Gunakan skill ini saat membuat atau memodifikasi endpoint HTTP, service, atau repository di `backend/`, `backend-exq/`, atau `pkg/`.

---

## Checklist 7 Langkah

### 1. Tentukan Target Service & Bentuk Response JSON
Periksa di service mana endpoint berada dan patuhi aturan format JSON di `.agents/rules/GEMINI.md`:
- **`backend/` (ATM / Core, port 8080)**:
  - Wajib mengembalikan **Flat JSON** (tanpa membungkus dengan `{success, data}`) demi kompatibilitas dengan frontend.
- **`backend-exq/` (Async Export & Monitoring, port 8081)**:
  - Wajib menggunakan envelope dari `pkg/response` (`{success, data, ...}`).

### 2. Patuhi Pemisahan Layer (`handler` -> `service` -> `repository`)
- **Repository (`internal/repository/`)**:
  - Hanya berisi pemanggilan query `sqlc` dan pemetaan tipe data DB ke model domain.
  - Gunakan `db` (Primary) untuk mutasi & *read-after-write*, serta `dbRead` (Read Replica) untuk pembacaan daftar/reporting/monitoring.
- **Service (`internal/service/`)**:
  - Berisi validasi bisnis, normalisasi input (`strings.TrimSpace`), pengecekan otorisasi lintas entitas/vendor, pengelolaan transaksi (`tx`), serta pencatatan `audit_logs`.
  - Kembalikan `&ValidationError{Field, Message}` (dalam bahasa Indonesia yang jelas) untuk kesalahan input (HTTP `422`) dan sentinel error (`Err...NotFound`, `Err...Conflict`, `Err...Forbidden`) untuk status HTTP lainnya.
- **Handler (`internal/handler/`)**:
  - Parsing request path/query/body, ekstraksi identitas user dari context JWT dan IP dari `extractClientIP(r)`, pemanggilan service, dan pemetaan sentinel error ke HTTP status code (`400`, `401`, `403`, `404`, `409`, `422`, `500`).

### 3. Kepatuhan Maker-Checker & Transaksi `audit_logs`
- Jika endpoint mengubah state pada data master atau finansial:
  - Wajib melewati mekanisme **Maker-Checker** (`approval_requests`) dan mengembalikan HTTP **`202 Accepted`** saat pengajuan. (Untuk entitas master data, aktifkan juga skill `cms2-master-data`).
  - Setiap perubahan state wajib mencatat `audit_logs` di dalam **transaksi database (`tx`) yang sama** dengan mutasi data utama.

### 4. Keamanan Otorisasi & Nilai Uang
- **Pencegahan IDOR (Vendor Isolation)**: Untuk endpoint yang diakses oleh pengguna Vendor (`VendorPortal`), ambil `vendor_id` dari klaim JWT (bukan percaya mentah-mentah dari body/query parameter) dan perlakukan ID milik vendor lain sebagai `NotFound` (`404`).
- **Tipe Data Uang**: Gunakan `int64` (minor units IDR) atau tipe `numeric` standar proyek. **Dilarang keras** menggunakan `float32` / `float64`.

### 5. Registrasi Route & Dependency Injection
- Daftarkan repository, service, applier (jika ada), dan route Chi v5 di `cmd/api/main.go` pada grup middleware auth/role yang sesuai.

### 6. Pembaruan Kontrak OpenAPI (Jika di `backend/`)
- Jika menambah atau mengubah struktur request/response pada handler `backend/`, jalankan test OpenAPI:
  ```bash
  cd backend && go test ./internal/handler -run OpenAPI
  ```
  *(Jika frontend juga perlu diperbarui pada task yang sama, aktifkan skill `openapi-frontend-sync`).*

### 7. Pengujian (Table-Driven Tests) & Linter
- Buat unit test di `*_test.go` untuk service dan handler mencakup:
  - Skenario sukses (`200` / `201` / `202`)
  - Validasi input gagal (`422`)
  - Akses tanpa izin / lintas vendor (`401` / `403` / `404`)
  - Konflik / perubahan pending ganda (`409`)
- Jalankan verifikasi wajib:
  ```bash
  cd backend && go test ./...
  cd backend && golangci-lint run --build-tags integration ./...
  ```
  *(Atau `cd backend-exq && go test ./...` / `cd pkg && go test ./...` sesuai modul yang diubah).*
