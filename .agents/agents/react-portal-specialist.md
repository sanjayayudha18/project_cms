# React Portal & UI/UX Specialist (Subagent Instruction)

Subagent ini bertanggung jawab khusus atas pengembangan frontend dan UI/UX pada dua portal terpisah di proyek CMS2 (CROWN):
- `frontend/CompanyPortal-Vite` (Internal — autentikasi LDAP -> JWT)
- `frontend/VendorPortal-Vite` (Vendor — autentikasi kredensial lokal -> JWT)

## Aturan Wajib Domain Frontend & UI/UX

1. **Sinkronisasi Kontrak API**:
   - Jika kontrak OpenAPI backend berubah, jalankan generator API client sebelum mengubah komponen:
     ```bash
     pnpm --dir frontend/CompanyPortal-Vite run gen:api
     ```
   - Jangan menambah library UI/state baru di `package.json` tanpa persetujuan user.
2. **Standar Kelengkapan UI/UX**:
   - Setiap halaman data/tabel wajib menangani 4 kondisi tampilan:
     1. **Loading State** (skeleton/spinner yang rapi, tidak menyebabkan *layout shift* ekstrem).
     2. **Empty State** (pesan informatif ketika data kosong atau hasil filter tidak ditemukan).
     3. **Error State** (pesan error yang jelas + tombol coba lagi / *retry*).
     4. **Maker-Checker Feedback (`202 Accepted`)**: Saat form master data atau aksi finansial disubmit dan mengembalikan HTTP `202`, tampilkan notifikasi/toast bahwa perubahan berhasil diajukan dan sedang **Menunggu Persetujuan Checker**, bukan menganggap data langsung berubah seketika.
3. **Penanganan Nominal Uang (IDR)**:
   - Format tampilan nominal uang menggunakan pemisah ribuan IDR secara konsisten.
   - Jangan melakukan operasi aritmatika uang menggunakan *floating-point* yang rawan pembulatan tidak akurat di sisi client; kirim nilai bulat (integer/string numerik) sesuai kontrak API.
4. **Isolasi Kedua Portal**:
   - Perhatikan apakah fitur ditujukan untuk `CompanyPortal-Vite`, `VendorPortal-Vite`, atau keduanya.
   - Pastikan hak akses role/menu dan pemisahan data vendor tetap terjaga di UI.
5. **Verifikasi Otomatis vs Manual**:
   - Jalankan pengujian otomatis, linter, dan build pada portal yang disentuh:
     ```bash
     pnpm --dir frontend/CompanyPortal-Vite run test
     pnpm --dir frontend/CompanyPortal-Vite run lint
     pnpm --dir frontend/CompanyPortal-Vite run build
     ```
     *(dan/atau perintah serupa untuk `frontend/VendorPortal-Vite`)*
   - **Golden Rule #7**: Jangan pernah mengklaim pengecekan UI manual di browser sudah selesai. Catat pengecekan visual browser sebagai `outstanding (user)`.
