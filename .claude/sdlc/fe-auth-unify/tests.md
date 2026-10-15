# Tests: Pola auth & util frontend antar portal (FE-09, opsi (b))

Status: **green 2026-10-15** (U1–U3 built, uncommitted). Stage: 4 Test. Stage berikut: `review.md`.
Sumber: `plan.md` (accepted 2026-10-15; P1 di-rename ke **`formatDecimalIDR`**, bukan `formatDecimalThousands`, lihat Deviasi).

## Hasil otomatis

| Portal | `pnpm test` | `pnpm lint` | `pnpm build` |
|---|---|---|---|
| CompanyPortal | 125 file / 1019 test hijau | Biome bersih (466 file) | hijau; initial JS 172,5 KB gzip (budget 300) |
| VendorPortal | 20 file / 151 test hijau | Biome bersih (100 file) | hijau |

## Trace test → FR

| FR | Test | RED sebelum fix | GREEN |
|---|---|---|---|
| FR-1 (B1) | VP `features/auth/__tests__/AuthContext.test.tsx` › "AuthContext init under StrictMode (FR-1)": satu `/refresh` per load di StrictMode; restore sesi `VENDOR-USER` satu kali; role non-vendor tetap ditolak; load berikutnya mengirim `/refresh` lagi | ✅ `expected 2 to be 1` (2 test merah) | ✅ |
| FR-2 (B2) | same file › "AuthContext token wiring (FR-2)": request pertama setelah `login()` membawa `Bearer fresh-token` tanpa `/refresh`; request pertama setelah `logout()` tanpa `Authorization` | ✅ `expected null to be 'Bearer fresh-token'` | ✅ |
| FR-3 (B3) | CP `lib/utils/formatCurrency.test.ts` (baru, mengunci output sebelum rename: `0`, `1.000`, `1.250.000`, `-1.250.000`, `1.500`, round-trip `parseIDR`); `format.test.ts` + `format.property.test.ts` (→ `formatRp`); `admin-vendors/lib/packagePrice.test.ts` (→ `formatDecimalIDR`) | n/a (rename; output dikunci dulu, hijau dengan nama lama) | ✅ |
| FR-4 | Regresi VP `AuthContext.test.tsx` (13 test lama), `routeGuard.property.test.tsx`, `app/__tests__/integration.test.tsx`, `routes/__tests__/routeMapping.property.test.tsx`; CP seluruh suite | – | ✅ |

## Bukti rename hanya rename
- `git diff -U0` CP: semua baris berubah memuat `formatRp`/`formatThousands`/`formatDecimalIDR`/`formatIDR`, kecuali pembungkusan JSX oleh Biome (nama baru lebih panjang).
- `grep -rn "\bformatIDR\b" CompanyPortal-Vite/src` = 0. VP `formatIDR` tidak disentuh.
- VP `AuthContext.tsx` `git diff -w`: hanya `setAccessToken` → `applyToken`, deps `[applyToken]`, blok init. Re-indent `login`/`refreshToken` berasal dari Biome karena array deps tidak muat satu baris.

## Deviasi dari plan
- **P1 nama:** saat implementasi ternyata `packagePrice.ts` `formatIDR("1000.50")` menghasilkan `"IDR 1.000,50"` (ADA mata uang; plan/intent salah menulis tanpa mata uang). Atas pilihan PO 2026-10-15 nama menjadi **`formatDecimalIDR`**, bukan `formatDecimalThousands`.
- U2 memakai `applyToken` lewat `useCallback` dan ditambahkan ke deps hook (Biome `useExhaustiveDependencies`); identitasnya stabil, jadi perilaku tidak berubah.

## Temuan di luar scope (tidak diperbaiki)
- `tsc -b` (tanpa `tsconfig.app.json`) melaporkan error tipe di CP `src/styles/contrast.property.test.ts` (baris 310, 451–452). File ini tidak disentuh; gate `pnpm build` memakai `tsconfig.app.json`, yang tidak mencakup test.

## Manual (user) — OUTSTANDING
- VP (dev, `pnpm --dir frontend/VendorPortal-Vite dev`): login → reload → Network tab hanya **satu** `/api/v1/auth/refresh`, vendor tetap login.
- VP: login → request data pertama tanpa 401 dan tanpa `/refresh` tambahan.
- Kedua portal: login, logout, sesi habis (refresh 401 → kembali ke login) berperilaku sama seperti sebelumnya.
- CP: angka di dashboard (MetricCard `Rp…`), vendor request/forecast (`1.000` tanpa mata uang), harga paket vendor (`IDR 1.000,50`) tampil sama seperti sebelum rename.
