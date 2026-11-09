# Go Backend Specialist (Subagent Instruction)

Subagent ini bertanggung jawab khusus atas pengembangan layer backend Go (`backend/`, `backend-exq/`, `pkg/`) menggunakan **Go + Chi v5**, **PostgreSQL (`db` & `dbRead`)**, dan **Redis**.

## Aturan Wajib Domain Go Backend

1. **Arsitektur & Pemisahan Layer**:
   - Ikuti pola `handler` -> `service` -> `repository` (`sqlc`).
   - Jangan menambah library/dependency baru di `go.mod` tanpa persetujuan user.
2. **Maker-Checker & Audit Trail**:
   - Setiap perubahan state pada data master atau finansial **wajib** melalui mekanisme maker-checker (`approval_requests`) dan mengembalikan HTTP `202 Accepted` saat pengajuan (Submit).
   - Setiap mutasi state wajib mencatat `audit_logs` di dalam **transaksi database (`tx`) yang sama**.
   - Jika menyentuh entitas master data, baca dan ikuti checklist `.agents/skills/cms2-master-data/SKILL.md` (termasuk pendaftaran Applier di `main.go`).
3. **Bentuk Response JSON per Service**:
   - `backend/` (ATM / Core, port 8080): Gunakan **Flat JSON** demi kompatibilitas dengan frontend.
   - `backend-exq/` (Async export & monitoring, port 8081): Gunakan envelope `pkg/response` (`{success, data}`).
4. **Topologi Database**:
   - Gunakan **Primary pool (`db`)** untuk seluruh operasi write/update dan *read-after-write*.
   - Gunakan **Read replica pool (`dbRead`)** untuk endpoint reporting, dashboard, monitoring, dan ekspor data.
5. **Tipe Data Finansial**:
   - Dilarang menggunakan `float32` / `float64` untuk nominal uang. Gunakan `int64` (minor units IDR) atau tipe `numeric`/`decimal` yang sudah terstandar di proyek.
6. **Kontrak OpenAPI**:
   - Jika menambah atau mengubah request/response endpoint di `backend/`, perbarui spesifikasi/handler terkait dan jalankan:
     ```bash
     cd backend && go test ./internal/handler -run OpenAPI
     ```
7. **Verifikasi Wajib**:
   - Jalankan test dan linter pada modul yang diubah:
     ```bash
     cd backend && go test ./...
     cd backend && golangci-lint run --build-tags integration ./...
     ```
   - Jika menyentuh `backend-exq/` atau `pkg/`, jalankan juga `go test ./...` di modul tersebut.
