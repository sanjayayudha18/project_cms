# Plan: Pola auth & util frontend antar portal (FE-09, opsi (b))

Status: **accepted 2026-10-15** (engineer/PO: "Terima kedua plan.md. P1: ikut rename ke formatDecimalThousands."). Stage: 3 Build. Stage berikut: `tests.md`.
Sumber: `spec.md` (accepted 2026-10-15: S1 = opsi (b), S2 = `formatRp` + `formatThousands`, S3 = `zustand` di VP dibiarkan).

## ⚠ Flagged (Sec 4 #7)
Auth: U1 dan U2 mengubah `frontend/VendorPortal-Vite/src/features/auth/AuthContext.tsx`. `lib/api/client.ts` (CP dan VP) **tidak** diubah. Kontrak HTTP tetap (FR-4).

## Urutan
U1 → U2 → U3. Satu concern per langkah, test-first (RED → GREEN). Tiap langkah: `pnpm test`, `pnpm lint`, `pnpm build` hijau di portal yang disentuh.

## U1 — B1: init VP single-flight (FR-1)
**File:** `frontend/VendorPortal-Vite/src/features/auth/AuthContext.tsx`, test `features/auth/__tests__/AuthContext.test.tsx`.

1. **RED.** Test baru:
   - render `<StrictMode><AuthProvider><TestConsumer/></AuthProvider></StrictMode>`, tunggu `loading=false` → `fetch` ke `/api/v1/auth/refresh` dipanggil **tepat 1×**. Dengan kode sekarang hasilnya 2×, jadi test gagal.
   - sesudah promise selesai, unmount lalu render ulang → `/refresh` dipanggil lagi (single-flight di-reset, bukan cache permanen).
   - `/refresh` 200 dengan role `VENDOR-USER` di StrictMode → user ter-set satu kali, `authenticated=true`.
2. **GREEN.** Pola CP `initializePromise` (`CompanyPortal-Vite/src/lib/auth/store.ts:61-64, 281-336`):
   - `let initPromise: Promise<LoginSuccessResponse | null> | null = null;` di level modul.
   - `fetchInitialSession()`: `POST /api/v1/auth/refresh` dengan `credentials: "include"` → `data` bila `ok`, `null` bila tidak ok atau error jaringan.
   - effect: `initPromise ??= fetchInitialSession().finally(() => { initPromise = null; })`, lalu `await` promise itu. Bila `cancelled`, berhenti. Bila tidak: terapkan guard `VENDOR-USER` dan state yang sama seperti sekarang, `setIsAuthLoading(false)` di `finally` (tetap dicek `!cancelled`).
   - Perilaku lain tidak berubah: URL, method, header, `credentials`, guard role.

## U2 — B2: token tersedia tanpa jeda effect (FR-2)
**File:** sama dengan U1.

1. **RED.** Test baru: consumer memanggil `await login(...)` lalu langsung `api.get("/api/v1/vendor/x")` di handler yang sama. Request kedua harus membawa `Authorization: Bearer <token dari login>`, tanpa 401 dan tanpa `/refresh`. Dengan kode sekarang `accessTokenRef` masih `null` (diisi di `useEffect`), jadi test gagal.
2. **GREEN.**
   - Helper lokal `applyToken(t: string | null)`: `accessTokenRef.current = t; setAccessToken(t);`.
   - Ganti semua `setAccessToken(...)` di `AuthContext.tsx` dengan helper itu: init, login, logout, refresh (sukses, non-vendor, gagal, catch), dan `onFailure` di `configureApiAuth`.
   - Hapus `useEffect` yang menyinkronkan `accessTokenRef` (baris 85-87), karena sudah tidak dibutuhkan.
   - API `configureApiAuth` dan `client.ts` VP tidak berubah.

## U3 — B3: nama formatter jujur di CP (FR-3)
**Rename saja. Output setiap layar harus persis sama.**

1. **Kunci output dulu.** `lib/utils/formatCurrency.test.ts` (baru): `formatIDR(0)="0"`, `1000→"1.000"`, `1250000→"1.250.000"`, negatif, `parseIDR` round-trip. Test ini ditulis dengan nama lama dan harus hijau sebelum rename.
2. **`lib/utils/format.ts`:** `formatIDR` → `formatRp` (output `"Rp1.000"`). Pemakai:
   - `features/dashboard/components/MetricCard.tsx`
   - `lib/utils/format.test.ts`
   - `lib/utils/format.property.test.ts`
   - barrel `lib/utils/index.ts`
3. **`lib/utils/formatCurrency.ts`:** `formatIDR` → `formatThousands` (output `"1.000"`); `parseIDR` tetap. Pemakai:
   - `components/ui/SummaryCard.tsx`
   - `features/cit/CitTable.tsx`
   - `features/dsr/{DsrSummary,DsrTable}.tsx`
   - `features/forecast/{ForecastTable,ScheduleList}.tsx`
   - `features/invoice/{InvoiceDetail,InvoiceFlow}.tsx`
   - `features/vendor-request/{ForecastBrowser,ForecastSummary,ForecastTable,VendorRequestCreate,VendorRequestDetail,VendorRequestList}.tsx`
   - barrel `index.ts` (`formatIDR as formatIDRRaw` → `formatThousands`)
   - komentar `formatIDR(...)` di `features/cit/__tests__/CitTracker.test.tsx` dan `features/dsr/__tests__/DsrDashboard.test.tsx`
4. `tsc` (via `pnpm build`) dan `pnpm lint` menangkap import yang terlewat. `grep -rn "formatIDR\b" src` sesudahnya hanya boleh menyisakan P1 di bawah.
5. VP `formatIDR` (`"IDR …"`) **tidak disentuh**.

## Butuh keputusan engineer/PO sebelum U3
- **P1 — `formatIDR` ketiga di CP.** Saat menyusun plan ditemukan `features/admin-vendors/lib/packagePrice.ts:35` `formatIDR(decimalString: string)` → `"1.000,50"` (tanpa mata uang; input decimal-string). Intent hanya mencatat dua. Usul: ikut di-rename menjadi `formatDecimalThousands` (pemakai: `components/PackagePricesPanel.tsx`, `lib/packagePrice.test.ts`), supaya FR-3 ("tidak ada dua `formatIDR` dengan output berbeda di CP") benar-benar terpenuhi. Pilihan lain: biarkan, karena signature-nya `string` sehingga salah pakai ditangkap `tsc`.

## Bukti
- Test baru U1/U2 merah sebelum fix dan hijau sesudahnya (dicatat di `tests.md`).
- Regresi hijau: VP `AuthContext.test.tsx`, `routeGuard.property.test.tsx`, `app/__tests__/integration.test.tsx`; CP `store.test.ts`, `client.test.ts`, `format*.test.ts`, test fitur pemakai formatter.
- Kedua portal: `pnpm test`, `pnpm lint`, `pnpm build` hijau.
- **Manual (user, outstanding):** VP dev: reload saat login → di Network tab hanya satu `/refresh`, vendor tetap login; login → request pertama tanpa 401. Kedua portal: login, logout, dan sesi habis tetap seperti sebelumnya. CP: angka di dashboard, vendor request, dan forecast tampil sama seperti sebelum rename.

## Penutup
`FE_debt.md` FE-09 → `done <sha>` dengan catatan bahwa opsi (a) ditunda (pemicu ada di spec) dan (c) ditolak. Update baris tech-debt frontend di `development-progress.md`, jalankan `graphify update .`, lalu commit hanya bila user bilang "commit".

## Risiko
- U1: promise level modul bisa bocor antar test. Mitigasinya `finally` me-reset promise, dan test menunggu `loading=false` sebelum selesai.
- U2: semua jalur clear token harus memakai helper. Bila satu terlewat, token lama tetap terkirim setelah logout. Mitigasi: `grep setAccessToken` hanya menyisakan definisi helper, plus test logout → request berikutnya tanpa `Authorization`.
- Overlap `fe-coverage` C1 (file test yang sama): kerjakan `fe-auth-unify` dulu, lalu C1.
