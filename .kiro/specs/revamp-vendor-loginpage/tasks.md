# Implementation Plan: Revamp Halaman Login Vendor Portal (CROWN)

## Overview

Merombak tampilan halaman login Vendor Portal agar sesuai mockup "CROWN" (`vendor-login-ui/crown-vendor-portal-login.html`): layout split-screen dua kolom, panel brand maroon di kiri, panel form di kanan, dengan tema "Merah Menyala". Logika autentikasi yang sudah ada (`AuthContext` / `useAuth` / `ProtectedRoute`) dan router React Router **dipertahankan** — hanya lapisan UI `LoginPage.tsx` yang dirombak dan diintegrasikan ulang. Setiap tugas mengacu pada mockup dan file yang sudah ada, bukan pada nomor requirement (tidak ada `requirements.md`/`design.md`).

## Tasks

- [ ] 1. Tambahkan token tema "Merah Menyala" (login) dan font ke style Vendor Portal
  - Periksa `src/styles/index.css` dan `src/index.css` lebih dulu; token topbar/sidebar/semantic sudah ada — **jangan** duplikasi, tambahkan hanya yang belum ada.
  - Tambahkan token yang dipakai mockup namun belum ada: `--maroon-deep`, `--primary`, `--primary-hover`, `--primary-fg`, `--primary-text`, `--primary-tint`, `--chrome-fg`, `--bg`, `--surface-sunken`, `--text`, `--text-secondary`, `--text-muted`, `--border-c`, `--border-strong` (definisikan via OKLCH di dalam blok `@supports (color: oklch(0% 0 0))` dengan fallback hex, mengikuti pola mockup).
  - Muat font Plus Jakarta Sans + IBM Plex Mono (link `fonts.googleapis.com` di `index.html` atau `@import`), definisikan `--font-sans` dan `--font-mono`.
  - Tambahkan keyframes `crown-rise` + `crown-spin`, aturan `prefers-reduced-motion`, dan penanganan autofill `-webkit-autofill` seperti pada mockup.
  - _Referensi: vendor-login-ui/crown-vendor-portal-login.html; frontend/VendorPortal-Vite/src/styles/index.css; frontend/VendorPortal-Vite/src/index.css_

- [ ] 2. Bangun kerangka layout split-screen sebagai struktur LoginPage baru
  - [ ] 2.1 Ganti markup `LoginPage.tsx` menjadi grid dua kolom `grid-cols-1 lg:grid-cols-[44fr_56fr]`, `min-h-dvh`
    - Panel kiri `<section>` brand (maroon-deep), panel kanan `<section>` form (surface terang).
    - Pertahankan landmark semantik dan tetap merender `null` saat sudah terautentikasi / spinner saat `state.isAuthLoading` (perilaku yang sudah ada).
    - Responsif: satu kolom di bawah `lg`, dua kolom di `lg` ke atas; elemen brand dekoratif disembunyikan di layar kecil.
    - _Referensi: vendor-login-ui/crown-vendor-portal-login.html; frontend/VendorPortal-Vite/src/features/auth/LoginPage.tsx_
  - [ ]* 2.2 Tulis tes render dasar untuk kerangka LoginPage
    - Verifikasi kedua panel ter-render dan heading "Masuk" tampak.
    - _Referensi: frontend/VendorPortal-Vite/src/features/auth/__tests___

- [ ] 3. Panel brand kiri (maroon-deep)
  - Header: ikon crown (SVG inline) + wordmark "CROWN", tagline "CIMB Niaga · Portal Vendor".
  - Headline dan paragraf deskriptif seperti mockup.
  - Baris statistik bawah: "Batas unggah DSR" → "09.00 WIB" dan "Bantuan operasional" → "021 1500 800" (angka pakai `--font-mono` / `tabular-nums`).
  - Terapkan animasi masuk yang menghormati `prefers-reduced-motion`.
  - _Referensi: vendor-login-ui/crown-vendor-portal-login.html; frontend/VendorPortal-Vite/src/features/auth/LoginPage.tsx_

- [ ] 4. Panel kanan: pill status, heading, dan shell form
  - Status pill kanan-atas "Layanan normal" (titik hijau + label uppercase).
  - Heading "Masuk" + subheading "Gunakan kredensial vendor yang diterbitkan CIMB Niaga."
  - Siapkan elemen `<form onSubmit={handleSubmit(onSubmit)} noValidate>` sebagai wadah field (diisi di tugas berikutnya), pertahankan pemakaian `useForm` dari react-hook-form.
  - _Referensi: vendor-login-ui/crown-vendor-portal-login.html; frontend/VendorPortal-Vite/src/features/auth/LoginPage.tsx_

- [ ] 5. Field username dan password + validasi inline (wiring ke useAuth)
  - [ ] 5.1 Implementasi field "Nama pengguna" dan "Kata sandi" dengan `register` react-hook-form
    - Aturan validasi + pesan Bahasa Indonesia (wajib diisi, non-whitespace, batas panjang) seperti pada `LoginPage.tsx` saat ini.
    - Pesan error inline dengan ikon peringatan (Warn/AlertCircle), styling danger dari token tema.
    - `onSubmit` memanggil `login(username, password)` dari `useAuth` lalu `navigate(redirectTo)` — **jangan** mengimplementasikan ulang panggilan API atau header `X-Portal-Type`.
    - _Referensi: vendor-login-ui/crown-vendor-portal-login.html; frontend/VendorPortal-Vite/src/features/auth/LoginPage.tsx; frontend/VendorPortal-Vite/src/features/auth/useAuth.ts; frontend/VendorPortal-Vite/src/features/auth/AuthContext.tsx_
  - [ ]* 5.2 Tulis tes validasi field
    - Submit kosong menampilkan pesan "wajib diisi"; input valid memicu pemanggilan `login`.
    - _Referensi: frontend/VendorPortal-Vite/src/features/auth/__tests___

- [ ] 6. Toggle tampil/sembunyi password
  - Tombol ikon (Eye / eye-off) di dalam field password, `type="button"`.
  - `aria-label` dinamis: "Tampilkan kata sandi" / "Sembunyikan kata sandi"; nonaktif saat form terkunci/loading.
  - _Referensi: vendor-login-ui/crown-vendor-portal-login.html; frontend/VendorPortal-Vite/src/features/auth/LoginPage.tsx_

- [ ] 7. Checkbox "Ingat perangkat ini" dan link "Lupa kata sandi?"
  - Checkbox remember-device (kontrol UI; belum ada perilaku persist backend — cukup state lokal).
  - Link "Lupa kata sandi?" dengan target stub (`#reset`). **Flag:** tidak ada alur reset password di kode saat ini — konfirmasi target ke user sebelum menghubungkan ke rute nyata.
  - _Referensi: vendor-login-ui/crown-vendor-portal-login.html; frontend/VendorPortal-Vite/src/features/auth/LoginPage.tsx_

- [ ] 8. Deteksi Caps Lock + peringatan di bawah password
  - Pakai `getModifierState('CapsLock')` pada `onKeyUp`/`onKeyDown` field password.
  - Tampilkan peringatan "Caps Lock aktif." dengan ikon peringatan dan warna `--warning`.
  - _Referensi: vendor-login-ui/crown-vendor-portal-login.html; frontend/VendorPortal-Vite/src/features/auth/LoginPage.tsx_

- [ ] 9. Penanganan state UI (idle / loading / done / error / locked)
  - [ ] 9.1 Petakan state dari `useAuth` ke tampilan tombol dan alert
    - loading: spinner + label "Memverifikasi"; done: sukses hijau + "Berhasil masuk" dan pesan "Mengalihkan ke dasbor vendor" (`role="status"`).
    - error: alert danger-tint (`role="alert"`) dengan penghitung sisa percobaan; ambil pesan dari `state.error`.
    - locked: pakai `state.rateLimitRetryAfter` yang sudah ada — pesan lockout 30 menit + nomor bantuan "021 1500 800", tombol dinonaktifkan; pertahankan countdown timer yang sudah ada di `LoginPage.tsx`.
    - _Referensi: vendor-login-ui/crown-vendor-portal-login.html; frontend/VendorPortal-Vite/src/features/auth/LoginPage.tsx; frontend/VendorPortal-Vite/src/features/auth/AuthContext.tsx_
  - [ ]* 9.2 Tulis tes state loading/error/locked/success
    - Simulasikan `state.error` dan `state.rateLimitRetryAfter` via mock context; verifikasi alert & disabled state.
    - _Referensi: frontend/VendorPortal-Vite/src/features/auth/__tests___

- [ ] 10. Redirect pasca-login dan integrasi guard
  - Pertahankan parsing `?redirect=` dari `useLocation().search` (default `/dashboard`) dan `navigate(redirectTo, { replace: true })`.
  - Pertahankan `GuestRoute` yang mengalihkan user terautentikasi ke dashboard; jangan ubah `ProtectedRoute.tsx` maupun beralih ke TanStack Router.
  - **Flag:** target default `/dashboard` vs menu DSR `ROUTES.DSR = '/dsr'` — konfirmasi ke user, jangan diubah diam-diam.
  - _Referensi: frontend/VendorPortal-Vite/src/features/auth/ProtectedRoute.tsx; frontend/VendorPortal-Vite/src/lib/constants.ts; frontend/VendorPortal-Vite/src/features/auth/LoginPage.tsx_

- [ ] 11. Pass aksesibilitas
  - `aria-invalid` + `aria-describedby` pada field; wilayah live `role="alert"` / `role="status"` dengan `aria-live`.
  - Focus ring `focus-visible` yang terlihat menggunakan `--primary`; urutan tab logis; label terkait `htmlFor`.
  - Hormati `prefers-reduced-motion` untuk semua animasi.
  - _Referensi: vendor-login-ui/crown-vendor-portal-login.html; frontend/VendorPortal-Vite/src/features/auth/LoginPage.tsx_

- [ ] 12. Sesuaikan dan lengkapi tes ter-kolokasi
  - [ ] 12.1 Sesuaikan tes yang ada bila selector berubah
    - Perbarui `AuthContext.test.tsx` bila query mengacu ke elemen login yang berubah; jaga `routeGuard.property.test.tsx` tetap lulus.
    - _Referensi: frontend/VendorPortal-Vite/src/features/auth/__tests__/AuthContext.test.tsx; routeGuard.property.test.tsx_
  - [ ]* 12.2 Tambah tes komponen untuk LoginPage revamp
    - Render, validasi field, toggle password, deteksi caps-lock, state loading/error/locked/success.
    - _Referensi: vendor-login-ui/crown-vendor-portal-login.html; frontend/VendorPortal-Vite/src/features/auth/LoginPage.tsx_
  - [ ]* 12.3 Tes aksesibilitas dasar
    - Verifikasi atribut aria dan keberadaan focusable controls dalam urutan yang benar.
    - _Referensi: frontend/VendorPortal-Vite/src/features/auth/__tests___

- [ ] 13. Checkpoint akhir — pastikan gate kualitas hijau
  - Jalankan `pnpm lint`, `pnpm test --run`, dan `pnpm build` di `frontend/VendorPortal-Vite`. Pastikan semua lulus; perbaiki temuan sebelum selesai. Tanyakan ke user bila muncul pertanyaan.
  - _Referensi: frontend/VendorPortal-Vite_

## Notes

- Tugas bertanda `*` bersifat opsional (tes) dan boleh dilewati untuk MVP; tugas inti tidak ditandai `*`.
- Traceability mengacu ke mockup dan file yang sudah ada karena tidak ada `requirements.md`/`design.md` untuk spec ini — sumber desain adalah `vendor-login-ui/crown-vendor-portal-login.html`.
- **Router**: proyek memakai **React Router** (`react-router`: `useNavigate`, `useLocation`, `Navigate`), **bukan** TanStack Router. Jangan mengganti router.
- **Wiring auth dipakai ulang tanpa perubahan**: `AuthContext`/`useAuth` menyediakan `login`, `logout`, dan `state` ({ `isAuthenticated`, `isAuthLoading`, `error`, `rateLimitRetryAfter` }) serta header `X-Portal-Type: vendor`. Jangan reimplementasi panggilan API.
- **Diskrepansi target pasca-login**: kode saat ini default ke `/dashboard`, sedangkan menu DSR adalah `ROUTES.DSR = '/dsr'`. Perlu konfirmasi user apakah target seharusnya `/dsr` — jangan diubah diam-diam (Tugas 10).
- **Alur reset password belum ada**: link "Lupa kata sandi?" di-stub (`#reset`) sampai ada rute reset resmi — perlu konfirmasi user (Tugas 7).
- Token topbar/sidebar/semantic sudah ada di `src/styles/index.css`; hanya token khusus login (mis. `--maroon-deep`, `--primary*`, `--chrome-fg`, `--border-*`, `--text-*`) dan font yang perlu ditambahkan — reuse yang sudah ada (Tugas 1).

## Task Dependency Graph

```json
{
  "waves": [
    { "id": 0, "tasks": ["1"] },
    { "id": 1, "tasks": ["2.1", "3", "4"] },
    { "id": 2, "tasks": ["2.2", "5.1"] },
    { "id": 3, "tasks": ["6", "7", "8", "9.1", "10"] },
    { "id": 4, "tasks": ["5.2", "9.2", "11"] },
    { "id": 5, "tasks": ["12.1", "12.2", "12.3"] }
  ]
}
```
