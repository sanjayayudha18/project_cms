---
name: react-ui-ux-craft
description: >-
  UI/UX and React implementation checklist for CompanyPortal-Vite and
  VendorPortal-Vite. Enforces the 4 mandatory UI states (loading, empty, error,
  202 maker-checker pending), IDR currency formatting without float precision
  loss, accessible tables/modals/forms, and portal isolation. Use when building
  or updating frontend screens and components.
---

# React + Vite UI/UX Craft (`react-ui-ux-craft`)

Gunakan skill ini saat merancang, membuat, atau memodifikasi halaman, tabel, modal, atau form di `frontend/CompanyPortal-Vite` (Internal) maupun `frontend/VendorPortal-Vite` (Vendor).

---

## Standar Wajib Implementasi UI/UX

### 1. Empat State UI Wajib (4 Mandatory States)
Setiap komponen yang memuat data dari API atau melakukan mutasi wajib menangani 4 kondisi berikut:
1. **Loading State**:
   - Tampilkan skeleton/spinner yang proporsional pada tabel atau card.
   - Nonaktifkan (*disable*) tombol submit saat request sedang berlangsung (`isSubmitting` / `isPending`) untuk mencegah klik ganda (*double submission*).
2. **Empty State**:
   - Bedakan antara **"Belum ada data sama sekali"** dan **"Data tidak ditemukan untuk filter/pencarian saat ini"** (sertakan tombol *Reset Filter* jika akibat filter).
3. **Error State**:
   - Tampilkan pesan error berbahasa Indonesia yang mudah dipahami pengguna (petakan pesan `ValidationError` HTTP `422` ke field terkait atau banner form, dan HTTP `409` untuk konflik perubahan pending).
   - Sediakan tombol **Coba Lagi (Retry)** saat gagal memuat data.
4. **Maker-Checker Pending State (`HTTP 202 Accepted`)**:
   - Untuk form yang melewati mekanisme maker-checker: saat server merespons `202 Accepted`, tampilkan pesan konfirmasi (`pendingApprovalMessage()`) bahwa perubahan telah diajukan dan **Menunggu Persetujuan Checker**.
   - Tandai baris tabel yang sedang memiliki perubahan pending menggunakan `usePendingEntityIds` / `usePendingCreates` dan badge status yang jelas, serta cegah pengajuan duplikat pada baris tersebut.

### 2. Penanganan Input & Tampilan Uang (IDR)
- **Tampilan**: Selalu format nominal uang IDR dengan pemisah ribuan yang konsisten dan rata kanan (*right-aligned*) di dalam kolom tabel agar mudah dibandingkan secara visual.
- **Input & Kalkulasi**:
  - Jangan menggunakan aritmatika `float` untuk menghitung total/selisih nominal uang. Gunakan bilangan bulat (`number` integer aman / `BigInt` / string numerik sesuai kontrak API).
  - Validasi batas nilai (tidak boleh negatif kecuali diizinkan spesifikasi, tidak melebihi batas maksimum kolom DB) sebelum dikirim ke API.

### 3. Ergonomi Tabel, Filter & Modal Konfirmasi
- **Tabel Data**:
  - Pertahankan state filter, pencarian, dan halaman (*pagination*) agar tidak hilang secara tiba-tiba saat modal ditutup.
- **Modal Aksi Kritis**:
  - Aksi *Disable*, *Enable*, *Approve*, *Reject*, atau *Submit Jurnal/Rekonsiliasi* wajib menampilkan dialog konfirmasi yang menyebutkan identitas item yang akan diproses.
- **Aksesibilitas Dasar**:
  - Pastikan input form memiliki `<label>` yang terhubung, tombol ikon memiliki `title` atau `aria-label`, dan modal dapat ditutup dengan aman.

### 4. Pemisahan Konteks Kedua Portal
- **`CompanyPortal-Vite`**: Perhatikan visibilitas tombol berdasarkan role pengguna (`ADMIN`, `ADMIN_PARAM`, Maker vs Checker).
- **`VendorPortal-Vite`**: Pastikan komponen tidak menampilkan filter atau referensi ke vendor lain di luar otorisasi pengguna vendor yang sedang login.

### 5. Verifikasi Otomatis & Batas Verifikasi Manual
- Jalankan test, lint, dan build pada portal yang dikerjakan:
  ```bash
  pnpm --dir frontend/CompanyPortal-Vite run test
  pnpm --dir frontend/CompanyPortal-Vite run lint
  pnpm --dir frontend/CompanyPortal-Vite run build
  ```
  *(Atau pada `frontend/VendorPortal-Vite` sesuai target task).*
- **Golden Rule #7**: Jangan pernah mengklaim pengecekan tampilan browser secara manual selesai oleh agen; catat verifikasi visual di browser sebagai `outstanding (user)`.
