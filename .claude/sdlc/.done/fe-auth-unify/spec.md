# Spec: Pola auth & util frontend antar portal (FE-09)

Status: **accepted 2026-10-15** (PO: S1 = opsi (b), S2 = `formatRp` + `formatThousands`, S3 = `zustand` dibiarkan). Stage: 2 Design. Stage berikut: `plan.md`.
Sumber: `intent.md` (draft 2026-10-15).

## ⚠ Flagged (Sec 4 #7)
- **Auth**: semua opsi menyentuh jalur auth VP (`AuthContext.tsx` dan/atau `lib/api/client.ts`). Opsi (a) menulis ulang inti auth VP.
- Tidak ada perubahan backend, tidak ada migrasi, tidak ada perubahan kontrak HTTP.
- Sec 5 tetap berlaku: dua SPA, dua login route, guard role per portal. Menyamakan pola kode tidak sama dengan menggabungkan app.

## Opsi

| | (a) VP pindah ke Zustand | (b) Biarkan pola, perbaiki B1–B3 | (c) Shared package |
|---|---|---|---|
| Isi | `AuthContext` → `useAuthStore` gaya CP; `client.ts` VP baca store langsung, `configureApiAuth` dihapus; 7 pemakai `useAuth` + 3 test (AuthContext, routeGuard property, integration) ditulis ulang | VP: `initialize` single-flight (B1); token getter langsung dari state yang sama, tanpa jeda effect (B2). CP: rename `formatIDR` yang ambigu (B3). Pola Context vs Zustand tetap | pnpm workspace `frontend/packages/shared` (client core, `cn`, formatter); kedua Dockerfile/`.dockerignore` dan build context harus diubah, karena tiap app sekarang build dari foldernya sendiri |
| Ukuran | M: ±300 baris auth VP + 10 file | S: ±30–60 baris + test | L: tooling + Docker + dua app |
| Manfaat bagi user | Tidak ada langsung; B1/B2 hilang sebagai efek samping | B1 (logout tak terduga di dev) dan B2 (round-trip ekstra) hilang; B3 menutup risiko salah baca uang | Tidak ada langsung |
| Risiko auth | **Tinggi**: semua jalur (init, login 401/403/422/429/503, refresh, logout, guard role) ditulis ulang; perilaku logout VP (tanpa reload + `queryClient.clear()`) harus dipertahankan atau diputuskan ulang | **Rendah**: dua perubahan lokal yang meniru pola CP yang sudah teruji (`store.test.ts`) | Sedang: logic sama, tapi build dan deploy berubah |
| Duplikasi yang hilang | Pola auth (bukan kode; kode tetap dua salinan) | Duplikasi `formatIDR` di dalam CP | `cn` (1 baris), formatter, client core. **Komponen UI tidak**: sudah berbeda karena tema brand |
| Dependency | `zustand` sudah ada di VP | – | Tooling workspace |

**Rekomendasi: (b) sekarang.** (a) ditunda dengan pemicu jelas: dikerjakan saat auth VP diubah untuk alasan fungsional (mis. fitur sesi baru), atau saat B1/B2 berulang meski sudah diperbaiki. (c) **tidak** dikerjakan: kode yang benar-benar sama antar portal hanya ±1 file, jadi biaya workspace + Docker tidak sebanding.

## Functional requirements (bila (b) diterima)

**FR-1 — B1: init VP single-flight.** `/refresh` saat mount hanya dikirim satu kali per load, juga di StrictMode. Pola sama dengan CP: promise level modul dipakai bersama, di-reset di `finally`. Perilaku lain tetap: guard `VENDOR-USER`, `isAuthLoading` → false, error jaringan = belum login.

**FR-2 — B2: token getter tanpa jeda.** Token yang dibaca `client.ts` langsung tersedia begitu `login`/`refresh` sukses. Cukup diset sinkron di jalur sukses itu sendiri (ref diisi bersamaan dengan `setAccessToken`), tanpa mengubah API `configureApiAuth`. Request pertama setelah login membawa `Authorization`.

**FR-3 — B3: nama formatter jujur (CP).** Tidak ada lagi dua fungsi `formatIDR` dengan output berbeda di CP. Rename sesuai Q2 intent; **string output setiap pemanggil persis sama** (dibuktikan dengan test yang sudah ada + snapshot output per fungsi). VP `formatIDR` (`"IDR …"`) tidak disentuh.

**FR-4 — Kontrak tetap.** Untuk kedua portal: URL, method, header (`Content-Type`, `X-Portal-Type`, `Authorization`), body, `credentials: "include"`, retry 401 satu kali, dan pesan error login tidak berubah.

## Test (diturunkan ke `tests.md`)
- FR-1: render `AuthProvider` dalam `<StrictMode>` → `fetch('/api/v1/auth/refresh')` dipanggil tepat 1×. Test ini harus gagal sebelum fix (RED).
- FR-2: setelah `login()` sukses, request `api.get` pertama membawa `Authorization: Bearer <token>` tanpa 401/refresh.
- FR-3: test output per fungsi formatter (nilai 0, 1.000, 1.500.000, negatif); `pnpm lint` dan `tsc` menangkap import lama.
- Regresi: `AuthContext.test.tsx`, `routeGuard.property.test.tsx`, `integration.test.tsx`, CP `store.test.ts`, `client.test.ts` tetap hijau.
- Manual (user, outstanding): login/logout/refresh-reload di kedua portal, cek Network tab: satu `/refresh` per load di VP dev.

## Out of scope
- Opsi (a) dan (c) (lihat rekomendasi).
- Menyamakan perilaku logout (reload vs `queryClient.clear()`) dan login (throw vs tidak). Itu perbedaan desain, bukan bug.
- Mengubah tampilan uang agar selalu menampilkan mata uang (keputusan UI terpisah; FR-3 hanya rename).
- Coverage (FE-13 → `fe-coverage`; test baru di sini ikut menaikkan angka VP auth).

## Keputusan spec (dikonfirmasi PO 2026-10-15)
- **S1** — Opsi: (b) [usul] / (a) / (c).
- **S2** — B3: nama baru `formatRp` + `formatThousands` [usul], atau nama lain.
- **S3** — `zustand` di VP: biarkan [usul, dipakai bila (a) nanti dikerjakan] / hapus.
