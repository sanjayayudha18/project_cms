# Requirements Document

## Introduction

Vendor Portal (`frontend/VendorPortal-Vite/`) saat ini menggunakan library routing `react-router` (`^8.3.0`) dengan pola `createBrowserRouter` + array konfigurasi route dan `RouterProvider` yang dipasang di `src/app/App.tsx` di dalam `QueryClientProvider` dan `AuthProvider`. Steering doc teknologi (`tech.md`) menetapkan TanStack Router sebagai router resmi untuk kedua frontend, dan CompanyPortal (`frontend/CompanyPortal-Vite/`) sudah memakai `@tanstack/react-router` (`^1.93.0`) dengan pola route programatik (`createRootRoute`/`createRoute`), guard `beforeLoad` + `redirect`, search param bertipe (Zod), serta `createRouter`/`RouterProvider` di `main.tsx`.

Fitur ini adalah migrasi arsitektur yang **mempertahankan perilaku (behavior-preserving)**: mengganti `react-router` dengan `@tanstack/react-router` pada Vendor Portal tanpa mengubah satu pun path route, redirect, perilaku guard autentikasi, integrasi dengan `AuthContext`/`useAuth`, integrasi TanStack Query, aksesibilitas, maupun kontrak API (`X-Portal-Type: vendor`). Konvensi TanStack Router yang diadopsi harus konsisten dengan CompanyPortal agar kedua frontend seragam.

Konteks tooling saat ini (fakta terverifikasi): dependensi `react-router` `^8.3.0` terpasang, `@tanstack/react-router` belum terpasang, linter adalah `oxlint` (bukan Biome), test memakai Vitest + fast-check + `@testing-library`, test terletak co-located di direktori `__tests__/`. Migrasi ini membingkai perilaku observable saat ini sebagai baseline yang harus dipertahankan sambil menukar library routing.

## Glossary

- **Vendor_Portal**: Aplikasi SPA di `frontend/VendorPortal-Vite/`, React 19 + TypeScript + Vite, untuk pengguna vendor (login lokal).
- **TanStack_Router**: Library routing `@tanstack/react-router` yang menjadi target migrasi.
- **React_Router**: Library routing `react-router` `^8.3.0` yang menjadi sumber migrasi (harus dihapus di akhir).
- **Route_Tree**: Pohon route TanStack Router yang dibangun dari objek route programatik (`createRootRoute`/`createRoute`), meniru pola CompanyPortal.
- **Router_Provider**: Komponen `RouterProvider` dari TanStack Router yang memasang instance router ke aplikasi.
- **Router_Instance**: Instance yang dibuat via `createRouter({ routeTree })`, didaftarkan ke sistem tipe via `declare module` untuk keamanan tipe compile-time.
- **Protected_Guard**: Guard route untuk halaman yang wajib autentikasi (padanan `ProtectedRoute` saat ini). Mengarahkan pengguna tak terautentikasi ke `/login`.
- **Guest_Guard**: Guard route untuk halaman khusus tamu (padanan `GuestRoute` saat ini), mis. `/login`. Mengarahkan pengguna terautentikasi ke tujuan default.
- **Auth_Context**: Konteks autentikasi yang sudah ada di `src/features/auth/` (`AuthContext`/`useAuth`) yang mengekspos `login`, `logout`, dan `state` (termasuk `isAuthenticated`, `isAuthLoading`, `error`, `rateLimitRetryAfter`).
- **App_Shell**: Komponen `AppShell` yang membungkus route terautentikasi utama (`/orders`, `/orders/:id/evidence`, `/invoices`, `/schedule`, `/dsr`, `/notifications`).
- **Redirect_Param**: Search param bernama `redirect` yang menyimpan path tujuan awal pengguna yang diminta sebelum autentikasi, agar dapat dikembalikan setelah login.
- **Internal_Route_Masking**: Perilaku menampilkan halaman NotFound untuk path internal-only (`/admin`, `/forecasting`, `/reconciliation`) sehingga keberadaannya tidak terungkap di Vendor Portal.
- **Query_Provider**: `QueryClientProvider` dari TanStack Query yang harus tetap membungkus aplikasi.
- **Route_Guard_Round_Trip**: Properti bahwa akses tak terautentikasi ke path terproteksi menghasilkan redirect ke `/login` dengan path awal terpreservasi, dan setelah login sukses pengguna dikembalikan ke path awal tersebut.

## Requirements

### Requirement 1: Adopsi TanStack Router sebagai library routing

**User Story:** Sebagai developer platform, saya ingin Vendor Portal memakai TanStack Router alih-alih React Router, sehingga frontend selaras dengan steering doc teknologi dan seragam dengan CompanyPortal.

#### Acceptance Criteria

1. THE Vendor_Portal SHALL menggunakan TanStack_Router sebagai satu-satunya library routing untuk seluruh routing aplikasi, tanpa menyisakan dependensi, impor, atau komponen React_Router (`react-router`/`react-router-dom`) di seluruh basis kode.
2. THE Vendor_Portal SHALL mendefinisikan Route_Tree menggunakan objek route programatik TanStack_Router (`createRootRoute` dan `createRoute`) mengikuti konvensi CompanyPortal, di mana setiap route yang saat ini dapat diakses di React_Router terpetakan tepat satu ke route TanStack_Router dengan path yang sama.
3. THE Vendor_Portal SHALL membuat Router_Instance melalui `createRouter` dengan Route_Tree sebagai masukan.
4. THE Vendor_Portal SHALL mendaftarkan tipe Router_Instance melalui deklarasi modul TanStack_Router sehingga route dan parameter bertipe pada waktu kompilasi, dan proses kompilasi TypeScript SHALL selesai tanpa error tipe terkait routing.
5. WHEN aplikasi Vendor_Portal dijalankan pada titik masuk aplikasi, THE Vendor_Portal SHALL memasang Router_Instance melalui Router_Provider TanStack_Router sehingga route awal ter-render sesuai path yang diminta.
6. IF navigasi ditujukan ke path yang tidak terdaftar dalam Route_Tree, THEN THE Vendor_Portal SHALL menampilkan tampilan not-found dengan indikasi yang dapat diamati pengguna tanpa mengubah URL ke path lain.

### Requirement 2: Preservasi seluruh path route dan redirect

**User Story:** Sebagai pengguna vendor, saya ingin semua URL dan pengalihan tetap sama setelah migrasi, sehingga bookmark dan alur navigasi saya tidak berubah.

#### Acceptance Criteria

1. THE Vendor_Portal SHALL menyediakan route `/login` yang di-guard oleh Guest_Guard.
2. THE Vendor_Portal SHALL menyediakan route terproteksi `/orders`, `/orders/:id/evidence`, `/invoices`, `/schedule`, `/dsr`, dan `/notifications` di dalam App_Shell.
3. WHEN pengguna terautentikasi mengakses `/`, THE Vendor_Portal SHALL mengalihkan pengguna ke `/orders`.
4. WHEN pengguna terautentikasi mengakses `/dashboard`, THE Vendor_Portal SHALL mengalihkan pengguna ke `/orders`.
5. WHEN pengguna mengakses `/admin`, `/forecasting`, atau `/reconciliation`, THE Vendor_Portal SHALL menampilkan respons NotFound (Internal_Route_Masking) yang identik dengan respons untuk path yang tidak dikenali, tanpa perbedaan yang dapat diamati pengguna yang mengungkap bahwa route internal tersebut ada.
6. WHEN pengguna mengakses path yang tidak dikenali, THE Vendor_Portal SHALL menampilkan halaman NotFound.
7. THE Vendor_Portal SHALL mempertahankan struktur App_Shell sebagai pembungkus route terproteksi utama sebagaimana perilaku saat ini.
8. WHEN pengguna tak terautentikasi mengakses salah satu path terproteksi (`/orders`, `/orders/:id/evidence`, `/invoices`, `/schedule`, `/dsr`, `/notifications`), THE Vendor_Portal SHALL mengalihkan pengguna ke `/login` (detail preservasi return URL diatur pada Requirement 3).

### Requirement 3: Preservasi guard autentikasi dan return URL

**User Story:** Sebagai pengguna vendor, saya ingin diarahkan ke login saat belum masuk dan dikembalikan ke halaman yang saya tuju setelah login, sehingga alur autentikasi tetap mulus seperti sebelumnya.

#### Acceptance Criteria

1. WHEN pengguna tak terautentikasi mengakses sebuah path terproteksi, THE Protected_Guard SHALL mengalihkan pengguna ke `/login` dengan Redirect_Param berisi path terproteksi yang diminta beserta query string aslinya.
2. WHEN pengguna berhasil login setelah dialihkan dari path terproteksi, THE Vendor_Portal SHALL mengalihkan pengguna ke path yang tersimpan di Redirect_Param.
3. IF Redirect_Param tidak tersedia setelah login sukses, THEN THE Vendor_Portal SHALL mengalihkan pengguna ke `/orders` sebagai tujuan default.
4. IF Redirect_Param tersedia namun gagal validasi skema Zod, ATAU berisi nilai bukan path internal (mis. diawali `http://`, `https://`, atau `//`), THEN THE Vendor_Portal SHALL mengabaikan Redirect_Param dan mengalihkan pengguna ke `/orders`.
5. WHEN pengguna terautentikasi mengakses `/login`, THE Guest_Guard SHALL mengalihkan pengguna ke tujuan default `/orders`.
6. WHILE Auth_Context melaporkan `isAuthLoading` bernilai benar, THE Vendor_Portal SHALL menampilkan indikator pemuatan (spinner) dan menunda keputusan guard sampai status autentikasi selesai diinisialisasi.
7. IF proses inisialisasi status autentikasi belum selesai dalam 10 detik, THEN THE Vendor_Portal SHALL menghentikan indikator pemuatan dan mengalihkan pengguna ke `/login`.
8. THE Redirect_Param SHALL didefinisikan sebagai search param bertipe yang divalidasi dengan skema Zod, konsisten dengan konvensi search param bertipe pada CompanyPortal.

### Requirement 4: Preservasi integrasi Auth_Context dan kontrak API

**User Story:** Sebagai developer, saya ingin migrasi hanya menukar library routing tanpa menulis ulang autentikasi atau klien API, sehingga risiko regresi minimal.

#### Acceptance Criteria

1. THE Vendor_Portal SHALL mengimpor dan menggunakan modul Auth_Context (`AuthContext`/`useAuth`) yang ada tanpa membuat definisi baru, tanpa menyalin, dan tanpa memodifikasi logika autentikasi di dalamnya.
2. THE Vendor_Portal SHALL mengimpor dan menggunakan modul klien API yang ada tanpa membuat definisi baru dan tanpa mengimplementasikan ulang pemanggilan jaringan (fetch/HTTP request) di luar modul klien API tersebut.
3. WHEN Vendor_Portal mengirim permintaan login, THE Vendor_Portal SHALL menyertakan header `X-Portal-Type` dengan nilai `vendor`.
4. WHILE `state.isAuthLoading` bernilai true, THE Protected_Guard dan Guest_Guard SHALL menahan keputusan pengalihan dan menampilkan indikator status pemuatan tanpa mengalihkan pengguna.
5. IF `state.isAuthLoading` bernilai false dan `state.isAuthenticated` bernilai false, THEN THE Protected_Guard SHALL mengalihkan pengguna ke halaman login.
6. IF `state.isAuthLoading` bernilai false dan `state.isAuthenticated` bernilai true, THEN THE Guest_Guard SHALL mengalihkan pengguna ke halaman terproteksi default.

### Requirement 5: Preservasi integrasi TanStack Query

**User Story:** Sebagai developer, saya ingin caching dan state server tetap berfungsi setelah migrasi, sehingga perilaku data-fetching tidak berubah.

#### Acceptance Criteria

1. WHEN aplikasi Vendor_Portal dirender setelah migrasi, THE Query_Provider SHALL membungkus seluruh pohon komponen aplikasi sebagai penyedia konteks terluar untuk data-fetching.
2. THE Vendor_Portal SHALL mempertahankan seluruh nilai konfigurasi `queryClient` yang ada (termasuk `staleTime`, `gcTime`, `retry`, dan `refetchOnWindowFocus`) identik dengan nilai sebelum migrasi.
3. WHEN sebuah query dieksekusi setelah migrasi, THE Query_Provider SHALL mengembalikan data dari cache untuk query dengan key identik selama entri cache belum melewati batas `staleTime` yang dikonfigurasi.
4. IF sebuah query gagal setelah migrasi, THEN THE Query_Provider SHALL menjalankan ulang percobaan sesuai jumlah `retry` yang dikonfigurasi sebelum menandai query berstatus error.

### Requirement 6: Preservasi aksesibilitas dan pengalaman navigasi

**User Story:** Sebagai pengguna vendor, saya ingin tampilan dan aksesibilitas tetap sama setelah migrasi, sehingga tidak ada regresi visual atau UX saat bernavigasi.

#### Acceptance Criteria

1. THE Vendor_Portal SHALL mempertahankan layout, warna, dan token desain setiap halaman dan komponen navigasi yang terdampak migrasi sehingga identik dengan tampilan sebelum migrasi (perbedaan piksel visual maksimum 0 persen pada perbandingan snapshot).
2. THE Vendor_Portal SHALL mempertahankan seluruh atribut aksesibilitas yang ada sebelum migrasi (mencakup `aria-live`, `role="alert"`, `aria-invalid`, dan label form) dengan nilai atribut yang sama persis pada setiap halaman yang terdampak migrasi.
3. WHEN pengguna bernavigasi antar route, THE Vendor_Portal SHALL menampilkan halaman tujuan dengan urutan tab fokus dan elemen penerima fokus awal yang sama persis dengan perilaku React_Router sebelum migrasi.
4. IF proses migrasi menghapus atau mengubah nilai salah satu atribut aksesibilitas yang tercantum pada kriteria 2, THEN THE Vendor_Portal SHALL memulihkan atribut tersebut ke nilai sebelum migrasi dan tidak menampilkan halaman dalam kondisi atribut yang berubah.
5. WHEN pengguna bernavigasi ke route yang tidak terdaftar, THE Vendor_Portal SHALL menampilkan halaman penanganan route tidak ditemukan yang sama dengan perilaku React_Router sebelum migrasi disertai indikasi teks bahwa route tidak ditemukan.

### Requirement 7: Preservasi test route-guard dan penghapusan React Router

**User Story:** Sebagai developer, saya ingin properti round-trip route guard tetap terbukti di bawah TanStack Router dan tidak ada sisa React Router, sehingga migrasi selesai bersih dan terverifikasi.

#### Acceptance Criteria

1. THE Vendor_Portal SHALL mempertahankan atau menyesuaikan seluruh test yang memverifikasi properti Route_Guard_Round_Trip (Property 12) sehingga test tersebut lulus (0 kegagalan) di bawah TanStack_Router.
2. FOR ALL path terproteksi, WHEN pengguna tak terautentikasi mengakses path terproteksi kemudian menyelesaikan login dengan sukses, THE Vendor_Portal SHALL mengarahkan pengguna kembali ke path awal terproteksi yang identik dengan yang diminta sebelum login (termasuk query string dan fragment).
3. IF pengguna tak terautentikasi mengakses path terproteksi, THEN THE Vendor_Portal SHALL menyimpan path awal yang diminta hingga login selesai dan mengalihkan pengguna ke halaman login.
4. WHEN migrasi selesai, THE Vendor_Portal SHALL menghapus dependensi `react-router` dari `package.json` sehingga tidak ada entri `react-router` pada bagian dependencies maupun devDependencies.
5. WHEN migrasi selesai, THE Vendor_Portal SHALL tidak menyisakan impor `react-router` apa pun di seluruh berkas kode sumber (0 kemunculan pernyataan impor `react-router`).

### Requirement 8: Kualitas non-fungsional (tipe, lint, build, test)

**User Story:** Sebagai developer, saya ingin hasil migrasi type-safe dan lolos seluruh gerbang kualitas, sehingga kode siap dipakai dan dirawat.

#### Acceptance Criteria

1. THE Vendor_Portal SHALL menyediakan route dan parameter yang aman secara tipe pada waktu kompilasi melalui pendaftaran tipe Router_Instance.
2. WHEN perintah build (`pnpm build`) dijalankan, THE Vendor_Portal SHALL menyelesaikan proses dengan exit code 0 tanpa error TypeScript dan tanpa error build.
3. IF perintah build (`pnpm build`) menghasilkan minimal satu error TypeScript atau error build, THEN THE Vendor_Portal SHALL menghentikan proses build dengan exit code bukan 0 dan menampilkan pesan error yang mengindikasikan lokasi dan jenis error.
4. WHEN perintah lint (`oxlint`) dijalankan, THE Vendor_Portal SHALL menyelesaikan proses dengan exit code 0 tanpa error lint (jumlah error lint = 0).
5. WHEN perintah test (`vitest --run`) dijalankan, THE Vendor_Portal SHALL menyelesaikan proses dengan exit code 0, seluruh test berstatus lulus (jumlah test gagal = 0), dan jumlah test yang dilewati (skipped) = 0.
