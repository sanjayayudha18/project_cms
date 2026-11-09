---
name: openapi-frontend-sync
description: >-
  End-to-end workflow for synchronizing OpenAPI contracts from the Go backend
  to the React + Vite frontends (CompanyPortal-Vite and VendorPortal-Vite) and
  verifying type safety across both portals. Use whenever backend request or
  response structs/routes change.
---

# OpenAPI Contract & Frontend Sync (`openapi-frontend-sync`)

Gunakan skill ini setiap kali terjadi penambahan atau perubahan endpoint, query parameter, atau bentuk JSON request/response di `backend/` yang dikonsumsi oleh frontend React.

---

## Langkah Eksekusi

### 1. Verifikasi & Update Spesifikasi OpenAPI di Backend Go
1. Pastikan struct request/response dan dokumentasi route OpenAPI pada `backend/internal/handler/` sudah sesuai dengan perubahan terbaru.
2. Jalankan pengujian kontrak OpenAPI di `backend/`:
   ```bash
   cd backend && go test ./internal/handler -run OpenAPI
   ```
3. Pastikan test tersebut **PASS** dan file spesifikasi OpenAPI terbarui dengan bersih.

### 2. Generate Ulang Client API di Frontend
Jalankan perintah codegen dari root proyek untuk memperbarui tipe TypeScript dan client API:
```bash
pnpm --dir frontend/CompanyPortal-Vite run gen:api
```
*(Periksa juga apakah `frontend/VendorPortal-Vite` mengonsumsi endpoint yang sama atau memiliki definisi tipe/service terkait yang perlu diselaraskan).*

### 3. Perbarui Pemanggilan API & Hooks di React
1. Periksa seluruh file `src/features/**` atau `src/services/**` yang memanggil endpoint tersebut.
2. Pastikan tipe request payload dan response yang digunakan pada komponen/hooks menggunakan tipe hasil generate terbaru (hindari penggunaan `any` atau casting manual yang menyembunyikan perubahan kontrak).
3. Jika endpoint mengembalikan HTTP `202 Accepted` (Maker-Checker), pastikan handler di frontend membaca response pengajuan dengan benar dan menampilkan status menunggu persetujuan.

### 4. Verifikasi Type-Check, Lint, Test & Build
Jalankan perintah verifikasi pada portal yang terdampak:
```bash
pnpm --dir frontend/CompanyPortal-Vite run lint
pnpm --dir frontend/CompanyPortal-Vite run test
pnpm --dir frontend/CompanyPortal-Vite run build
```
Jika `VendorPortal-Vite` turut terdampak, jalankan juga:
```bash
pnpm --dir frontend/VendorPortal-Vite run lint
pnpm --dir frontend/VendorPortal-Vite run test
pnpm --dir frontend/VendorPortal-Vite run build
```

### 5. Catat Verifikasi Browser Manual
- Sesuai **Golden Rule #7**, jangan mengklaim verifikasi UI di browser selesai. Catat pengecekan integrasi UI di browser sebagai `outstanding (user)`.
