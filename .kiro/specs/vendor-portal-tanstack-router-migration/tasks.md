# Implementation Plan: Migrasi Routing Vendor Portal ke TanStack Router

## Overview

Rencana implementasi ini memigrasikan routing `frontend/VendorPortal-Vite/` dari `react-router` (`^8.3.0`) ke `@tanstack/react-router`, meniru konvensi CompanyPortal, secara **behavior-preserving**. Urutan kerja disusun agar pohon route TanStack Router baru, guard berbasis komponen, dan wiring `main.tsx` dibangun dan tersambung lebih dulu; `react-router` baru dihapus di akhir sehingga build tetap hijau selama transisi jika memungkinkan.

Keputusan desain kritis yang wajib diikuti:
- Guard diletakkan di **komponen layout** (pathless route) memakai `useAuth()` + `<Navigate>`, BUKAN di `beforeLoad`. `AuthContext` tidak diubah ke Zustand (Requirement 4.1).
- Provider (`QueryClientProvider` + `AuthProvider`) dipindahkan ke komponen `rootRoute`.
- Search param `redirect` bertipe dan divalidasi Zod, menolak URL eksternal (`http://`, `https://`, `//`) → fallback `/orders`.
- Timeout inisialisasi auth 10 detik di guard terproteksi (Requirement 3.7).

Semua perintah dijalankan dari `frontend/VendorPortal-Vite/`.

## Tasks

- [ ] 1. Tambah dependensi TanStack Router dan siapkan skema search param
  - [ ] 1.1 Tambah dependensi `@tanstack/react-router` (pertahankan `react-router` sementara agar test lama tetap hijau selama transisi)
    - Jalankan `pnpm add @tanstack/react-router` di `frontend/VendorPortal-Vite/`
    - Pastikan versi selaras dengan CompanyPortal (`^1.93.0` atau kompatibel)
    - Verifikasi `package.json` memuat entri baru dan `react-router` masih ada untuk sementara
    - _Requirements: 1.1_
    - _Model: Claude Haiku, Effort: Low_
  - [ ] 1.2 Buat skema dan helper sanitasi `redirect` di `src/routes/login.tsx` (skema saja, tanpa route dulu)
    - Definisikan `isSafeInternalPath(value)` yang menolak string tak diawali `/` dan yang diawali `//`
    - Definisikan `redirectSearchSchema` (Zod) dengan `redirect` opsional + `transform` ke `undefined` bila tak aman
    - Ekspor `RedirectSearch = z.infer<typeof redirectSearchSchema>`
    - _Requirements: 3.4, 3.8_
    - _Model: Claude Sonnet, Effort: Medium_
  - [ ]* 1.3 Tulis unit test untuk `redirectSearchSchema` dan `isSafeInternalPath`
    - Kasus eksplisit: `/orders` lolos, `/orders/123/evidence?x=1` lolos, `//evil.com` ditolak, `http://x` dan `https://x` ditolak, `undefined` → default
    - _Requirements: 3.3, 3.4_
    - _Model: Claude Sonnet, Effort: Medium_

- [ ] 2. Bangun akar route tree dan komponen provider
  - [ ] 2.1 Buat `src/routes/__root.tsx` (`rootRoute`)
    - `createRootRoute` dengan `component: RootComponent` dan `notFoundComponent: NotFound`
    - `RootComponent` membungkus `<Outlet/>` dengan `QueryClientProvider` (pakai `queryClient` dari `@/lib/queryClient`) lalu `AuthProvider` (dari `@/features/auth/AuthContext`) tanpa modifikasi
    - _Requirements: 1.2, 2.6, 4.1, 5.1, 5.2, 6.5_
    - _Model: Claude Sonnet, Effort: Medium_
  - [ ]* 2.2 Tulis unit test bahwa `rootRoute` merender `QueryClientProvider` terluar lalu `AuthProvider` membungkus `Outlet`
    - Verifikasi urutan provider dan bahwa `notFoundComponent` terpasang
    - _Requirements: 5.1, 2.6_
    - _Model: Claude Sonnet, Effort: Low_

- [ ] 3. Bangun guard berbasis komponen (auth + guest) dan shell layout
  - [ ] 3.1 Tulis ulang `src/features/auth/ProtectedRoute.tsx` menjadi komponen layout TanStack Router
    - Ekspor `ProtectedLayout` (auth guard) dan `AuthLayout` (guest guard) memakai `useAuth()`
    - `ProtectedLayout`: `isAuthLoading` → spinner `Loader2` (markup identik dengan saat ini); `!isAuthenticated` → `<Navigate to="/login" search={{ redirect: currentPath }} replace/>` dengan `currentPath = pathname + search` via `useRouterState`/`useLocation`; selain itu `<Outlet/>`
    - `AuthLayout`: `isAuthLoading` → spinner; `isAuthenticated` → `<Navigate to="/orders" replace/>`; selain itu `<Outlet/>`
    - Terapkan timeout 10 detik di `ProtectedLayout`: bila `isAuthLoading` masih true setelah 10 detik → `<Navigate to="/login" .../>`; bersihkan timer saat unmount atau saat `isAuthLoading` menjadi false. Jangan ubah `AuthContext`
    - _Requirements: 3.1, 3.5, 3.6, 3.7, 4.4, 4.5, 4.6_
    - _Model: Claude Sonnet, Effort: High_
  - [ ] 3.2 Adaptasi `src/app/AppShell.tsx` ke TanStack Router
    - Ganti `NavLink`/`Outlet` `react-router` → `Link`/`Outlet` `@tanstack/react-router`; styling `isActive` via render prop / `activeProps` TanStack Router
    - Pertahankan seluruh markup, token warna/tema, dan atribut aksesibilitas persis
    - _Requirements: 2.7, 6.1, 6.2, 6.3_
    - _Model: Claude Sonnet, Effort: Medium_
  - [ ] 3.3 Buat `src/routes/_protected.tsx` dan `src/routes/_auth.tsx` (pathless layout) + shell layout
    - `_auth` (`id: "_auth"`) → `component: AuthLayout`
    - `_protected` (`id: "_protected"`) → `component: ProtectedLayout`
    - `shellRoute` (child `_protected`, pathless `id: "_shell"`) → komponen `ShellLayout` merender `<AppShell><Outlet/></AppShell>`
    - _Requirements: 2.1, 2.2, 2.7, 2.8_
    - _Model: Claude Sonnet, Effort: Medium_
  - [ ]* 3.4 Tulis unit test guard: spinner saat `isAuthLoading`, dan timeout 10 detik → redirect `/login`
    - Gunakan fake timers untuk memverifikasi transisi setelah 10 detik
    - _Requirements: 3.6, 3.7, 4.4_
    - _Model: Claude Sonnet, Effort: Medium_

- [ ] 4. Definisikan route halaman, redirect, dan masking
  - [ ] 4.1 Buat `src/routes/login.tsx` (`loginRoute`)
    - Child `_auth`, `path: "/login"`, `validateSearch: redirectSearchSchema`, `component: LoginPage`
    - _Requirements: 2.1, 3.8_
    - _Model: Claude Sonnet, Effort: Low_
  - [ ] 4.2 Buat route halaman terproteksi di bawah `shellRoute`
    - `src/routes/orders.tsx` (`/orders`), `src/routes/orders.$id.evidence.tsx` (`/orders/$id/evidence`), `src/routes/invoices.tsx`, `src/routes/schedule.tsx`, `src/routes/dsr.tsx`, `src/routes/notifications.tsx`
    - Setiap route memetakan ke komponen halaman yang sudah ada di `src/features/*`
    - _Requirements: 2.2_
    - _Model: Claude Haiku, Effort: Low_
  - [ ] 4.3 Buat route redirect `src/routes/index.tsx` (`/`) dan `src/routes/dashboard.tsx` (`/dashboard`)
    - `beforeLoad: () => { throw redirect({ to: '/orders' }) }` untuk keduanya (redirect murni, tetap melewati auth guard di `/orders`)
    - _Requirements: 2.3, 2.4_
    - _Model: Claude Haiku, Effort: Low_
  - [ ] 4.4 Buat route internal-masking `src/routes/admin.tsx`, `src/routes/forecasting.tsx`, `src/routes/reconciliation.tsx`
    - Masing-masing `component: NotFound` sehingga output identik dengan path tak dikenal
    - _Requirements: 2.5_
    - _Model: Claude Haiku, Effort: Low_
  - [ ] 4.5 Adaptasi `src/components/NotFound.tsx` ke TanStack Router
    - Ganti `Link` `react-router` → `Link` `@tanstack/react-router`; pertahankan teks "Halaman tidak ditemukan" dan tautan "Kembali ke beranda"
    - _Requirements: 6.5_
    - _Model: Claude Haiku, Effort: Low_

- [ ] 5. Adaptasi LoginPage ke search param TanStack Router
  - [ ] 5.1 Ubah `src/features/auth/LoginPage.tsx`
    - Ganti `useLocation`/`URLSearchParams`/`useNavigate` (`react-router`) → `useSearch({ from: loginRoute.id })` dan `useNavigate` (`@tanstack/react-router`)
    - `redirectTo` dibaca dari search tervalidasi, default `/orders`; efek "sudah authenticated → redirect" mengarah ke `/orders`
    - Pertahankan seluruh markup, atribut a11y (`aria-live`, `role="alert"`, `aria-invalid`, label form), token warna, dan logika rate-limit
    - _Requirements: 3.2, 3.3, 3.5, 4.3, 6.1, 6.2_
    - _Model: Claude Sonnet, Effort: Medium_
  - [ ] 5.2 Verifikasi `src/features/auth/useAuth.ts` (`useAuthRefresh`) tetap konsisten
    - Pastikan `window.location.href = '/login?redirect=...'` memakai nama param `redirect` dan semantik `pathname + search` yang sama; tanpa hook router
    - _Requirements: 3.1, 3.8, 4.1_
    - _Model: Claude Sonnet, Effort: Low_

- [ ] 6. Wiring router di titik masuk aplikasi
  - [ ] 6.1 Ubah `src/main.tsx`
    - Bangun `routeTree` via `rootRoute.addChildren([...])` sesuai hierarki desain (`_auth`>login; `_protected`>shell>halaman; index; dashboard; admin/forecasting/reconciliation)
    - `createRouter({ routeTree })`; `declare module "@tanstack/react-router" { interface Register { router: typeof router } }`
    - Render `<StrictMode><RouterProvider router={router} /></StrictMode>`
    - _Requirements: 1.3, 1.4, 1.5, 8.1_
    - _Model: Claude Sonnet, Effort: High_
  - [ ] 6.2 Hapus `src/app/routes.tsx` dan `src/app/App.tsx`
    - Provider sudah pindah ke `rootRoute`; `main.tsx` merender `RouterProvider` langsung; pastikan tidak ada impor yang menggantung
    - _Requirements: 1.2_
    - _Model: Claude Sonnet, Effort: Low_

- [ ] 7. Checkpoint - Pastikan aplikasi ter-wiring dan build sementara hijau
  - Ensure all tests pass, ask the user if questions arise.
  - _Model: Claude Sonnet, Effort: Low_

- [ ] 8. Adaptasi dan tambah test property-based (fast-check, min. 100 iterasi)
  - [ ] 8.1 Siapkan util test TanStack Router memory-history
    - Helper membuat `createRouter({ routeTree, history: createMemoryHistory({ initialEntries: [path] }) })` lalu render `<RouterProvider router={testRouter} />` dengan `fetch` di-mock; pastikan `AuthProvider` aktif via root
    - _Requirements: 7.1_
    - _Model: Claude Sonnet, Effort: Medium_
  - [ ]* 8.2 Adaptasi `src/features/auth/__tests__/routeGuard.property.test.tsx` ke TanStack Router
    - **Property 1: Route-Guard Round-Trip** (`protectedPathArb` termasuk `/orders/$id/evidence` + query string acak; unauth akses → `/login?redirect={path}` → login sukses → kembali ke path awal identik)
    - **Validates: Requirements 3.1, 3.2, 7.1, 7.2, 7.3**
    - _Model: Claude Sonnet, Effort: High_
  - [ ]* 8.3 Tulis property test sanitasi redirect param
    - **Property 2: Sanitasi Redirect Param** (generator `http://…`/`https://…`/`//…`/string non-path acak → target akhir `/orders`)
    - **Validates: Requirements 3.3, 3.4**
    - _Model: Claude Sonnet, Effort: Medium_
  - [ ]* 8.4 Tulis property test preservasi pemetaan path route
    - **Property 3: Preservasi Pemetaan Path Route** (generator seluruh path lama → halaman/redirect yang sama; `/` & `/dashboard` → `/orders`; setiap path terproteksi → halaman sama; `/login` → LoginPage)
    - **Validates: Requirements 1.2, 2.2, 2.3, 2.4**
    - _Model: Claude Sonnet, Effort: High_
  - [ ]* 8.5 Tulis property test internal-route masking tak terbedakan
    - **Property 4: Internal-Route Masking Tak Terbedakan** (generator himpunan internal + path acak → keluaran ter-render sama-sama `NotFound`)
    - **Validates: Requirements 2.5, 2.6**
    - _Model: Claude Sonnet, Effort: Medium_

- [ ] 9. Test unit/integration dan parity a11y/visual
  - [ ]* 9.1 Pertahankan `src/features/auth/__tests__/AuthContext.test.tsx` tanpa perubahan logika
    - Sesuaikan hanya impor jika menyentuh router; logika auth tidak diubah
    - _Requirements: 4.1_
    - _Model: Claude Haiku, Effort: Low_
  - [ ]* 9.2 Tambah test pemetaan route eksplisit dan parity a11y/visual
    - Setiap path lama meresolusi ke route/redirect yang benar
    - Verifikasi atribut `aria-live`, `role="alert"`, `aria-invalid`, label form pada `LoginPage` tetap ada; snapshot markup untuk memastikan tidak ada regresi
    - _Requirements: 6.1, 6.2, 6.3_
    - _Model: Claude Sonnet, Effort: Medium_

- [ ] 10. Hapus React Router dan finalisasi
  - [ ] 10.1 Hapus dependensi dan seluruh impor `react-router`
    - `pnpm remove react-router`; hapus setiap `import` `react-router`/`react-router-dom` yang tersisa di kode sumber
    - _Requirements: 7.4, 7.5_
    - _Model: Claude Sonnet, Effort: Medium_

- [ ] 11. Checkpoint akhir - Jalankan gerbang kualitas di `frontend/VendorPortal-Vite`
  - Jalankan `oxlint` (exit 0, 0 error), `vitest --run` (exit 0, 0 gagal, 0 skipped), `pnpm build` (exit 0, tanpa error TypeScript/build)
  - Verifikasi 0 kemunculan impor `react-router` di kode sumber dan tidak ada entri `react-router` di `package.json` (dependencies maupun devDependencies)
  - Ensure all tests pass, ask the user if questions arise.
  - _Requirements: 7.4, 7.5, 8.1, 8.2, 8.3, 8.4, 8.5_
  - _Model: Claude Sonnet, Effort: High_

## Notes

- Tugas bertanda `*` bersifat opsional (test) dan dapat dilewati untuk MVP lebih cepat, namun tetap disertakan dalam dependency graph.
- Setiap tugas mereferensikan requirement spesifik dan/atau property desain untuk keterlacakan.
- Property test memvalidasi property kebenaran universal (Property 1-4); unit test memvalidasi contoh spesifik dan edge case.
- Urutan menjaga build tetap hijau: route tree + guard + wiring dibangun dulu (Tugas 1-6), `react-router` baru dihapus di akhir (Tugas 10) sebelum checkpoint kualitas akhir (Tugas 11).
- Guard hidup di level komponen (`useAuth()` + `Navigate`), bukan `beforeLoad`; `AuthContext` tidak diubah.
- Setiap task menyertakan rekomendasi model Claude dan tingkat effort (Low/Medium/High) sebagai panduan eksekusi; sesuaikan bila perlu.

## Task Dependency Graph

```json
{
  "waves": [
    { "id": 0, "tasks": ["1.1", "1.2"] },
    { "id": 1, "tasks": ["1.3", "2.1", "3.1", "3.2"] },
    { "id": 2, "tasks": ["2.2", "3.3", "4.5", "5.2"] },
    { "id": 3, "tasks": ["3.4", "4.1", "4.2", "4.3", "4.4", "5.1"] },
    { "id": 4, "tasks": ["6.1"] },
    { "id": 5, "tasks": ["6.2"] },
    { "id": 6, "tasks": ["8.1", "9.1"] },
    { "id": 7, "tasks": ["8.2", "8.3", "8.4", "8.5", "9.2"] },
    { "id": 8, "tasks": ["10.1"] }
  ]
}
```
