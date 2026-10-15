# Tests: Coverage frontend naik ke 80% (FE-13)

Status: **partial — C0–C4 green 2026-10-15** (uncommitted). Stage: 4 Test (per slice). C5 belum dimulai.
Sumber: `plan.md` (accepted 2026-10-15).

## C0 — Config + threshold baseline

**Diubah**
- `frontend/CompanyPortal-Vite/vite.config.ts` dan `frontend/VendorPortal-Vite/vite.config.ts`: `test.coverage` (v8, `include: src/**`, exclude non-kode per Q2, `thresholds`).
- Kedua `package.json`: script baru `test:coverage` = `vitest run --coverage`.
- VP `package.json` + `pnpm-lock.yaml`: `@vitest/coverage-v8 ^4.1.10` (resolved 4.1.10 = versi `vitest`; `pnpm peers check` bersih).

**Angka terukur** (setelah exclude, termasuk test baru `fe-auth-unify`) → threshold = floor:

| Portal | Statements | Branches | Functions | Lines | Threshold |
|---|---|---|---|---|---|
| CompanyPortal | 61,46 % (3506/5704) | 62,40 % (2707/4338) | 54,73 % (1272/2324) | 61,84 % (3205/5182) | 61 / 62 / 54 / 61 |
| VendorPortal | 68,58 % (727/1060) | 56,71 % (439/774) | 63,83 % (233/365) | 69,66 % (682/979) | 68 / 56 / 63 / 69 |

VP naik dari baseline intent (63,0/54,7/58,8/63,7) karena `src/test/routerTestUtils.tsx` dikeluarkan (Q2) dan ada test baru `fe-auth-unify` U1/U2.

**Bukti**
- `pnpm run test:coverage` lulus di kedua portal (CP 1019 test, VP 151 test).
- Negatif: `pnpm run test:coverage --coverage.thresholds.statements=90` → exit 1, `ERROR: Coverage for statements (…) does not meet global threshold (90%)` di kedua portal.
- `pnpm lint` dan `pnpm build` hijau di kedua portal.

**Deviasi dari plan**
- Plan (dan `fe-tech-debt` E2) menulis `pnpm test -- --coverage`. Dengan pnpm 11 perintah itu menjadi `vitest run "--" "--coverage"`: coverage **diam-diam tidak jalan** dan exit 0. Karena itu ditambahkan script `test:coverage`. Perintah gate yang benar: `pnpm run test:coverage` (untuk CI/Jenkins juga).

**Catatan**
- VP: 1 dari 8 run `test:coverage` gagal satu test (nama tidak tertangkap); 7 run berikutnya hijau. Kemungkinan flaky timing di bawah instrumentasi coverage. Pantau; bila berulang, tangkap namanya dan catat lewat `cms2-bugfix`.
- Pesan `Error: Not implemented: window.scrollTo` di output VP adalah log jsdom yang sudah ada sebelumnya, bukan kegagalan.

## C1 — Auth + API client

**Test baru / diperluas**
| File test | Menutup |
|---|---|
| VP `src/lib/api/__tests__/client.test.ts` (baru, 12 test) | Bearer + `credentials`, `skipAuth`/tanpa token, FormData tanpa Content-Type, PUT/PATCH/DELETE/GET, 204, 401 → refresh → retry dengan token baru, 401 paralel = satu refresh, refresh gagal → `onAuthFailure` + throw 401, 401 sebelum auth di-wire, `skipAuth` tidak refresh, error `message`/`details`, body non-JSON → `statusText` |
| VP `features/auth/__tests__/AuthContext.test.tsx` (+17) | pemetaan error login 403 (`account_inactive`, `portal_mismatch`, lainnya, body rusak)/422/503/500/network; 429 `Retry-After` valid & tidak valid; logout tetap clear saat `POST /logout` gagal; `refreshToken` sukses, 401, role non-vendor, network; `useAuthRefresh` (true / false + logout) |
| VP `features/auth/__tests__/LoginPage.test.tsx` (baru, 8 test) | wajib isi email/password (termasuk spasi saja), pesan error server, tampil/sembunyi password, peringatan Caps Lock, 429 → form terkunci + countdown "1 menit 15 detik", countdown selesai → form aktif lagi, login sukses → `/orders` |
| CP `lib/auth/store.test.ts` (+9) | login 403 lainnya/body rusak, 422 tanpa `details`/body rusak, 500 dengan/tanpa body; refresh `VENDOR-USER` ditolak (redirect tanpa `reason`); refresh network error; `initialize` mengabaikan sesi `VENDOR-USER` |
| CP `lib/api/client.test.ts` (+3) | PUT/PATCH, `errors` → `details`, body non-JSON → `statusText` |

**Angka** (`pnpm run test:coverage`) → threshold baru = floor, tidak ada yang turun:

| Portal | Statements | Branches | Functions | Lines | Threshold (sebelum → sesudah) |
|---|---|---|---|---|---|
| CompanyPortal | 61,79 % | 62,65 % | 54,90 % | 62,17 % | 61/62/54/61 → **61/62/54/62** |
| VendorPortal | 75,00 % | 64,47 % | 67,94 % | 75,99 % | 68/56/63/69 → **75/64/67/75** |

CP hanya naik sedikit: gap auth/API CP memang kecil (20 statement). Gap besar CP ada di C2–C4.

**Bukti:** `pnpm run test:coverage` exit 0 (CP 1031 test, VP 188 test); `pnpm lint` + `pnpm build` hijau di kedua portal.

**⚠ Bug ditemukan (diperbaiki terpisah 2026-10-15, lihat `bugfixes.md` "VendorPortal: redirect setelah login hilang"):** VP — login dari `/login?redirect=%2Fdsr` dengan `LoginPage` asli berakhir di **`/orders`**, bukan `/dsr` (router test: `BEFORE /login?redirect=%2Fdsr {"redirect":"/dsr"}` → `AFTER /orders`). Test property "Route-Guard Round-Trip" memakai login page tiruan, jadi kasus ini tidak pernah teruji. Dugaan: setelah `navigate("/dsr")`, effect `LoginPage` (`state.isAuthenticated` → `navigate(redirectTo)`) berjalan lagi saat `useSearch` loginRoute sudah tidak punya `redirect` → fallback `/orders`. Belum diverifikasi di browser. Test untuk kasus ini sengaja tidak dimasukkan (komentar di `LoginPage.test.tsx`); perlu keputusan PO → `cms2-bugfix`.

## C2 — Maker-checker + admin (CP)

**Test baru / diperluas** (pola: hanya `@/lib/api/client` di-mock, sehingga fetcher + hook TanStack asli ikut teruji; halaman: hook data di-mock, dialog anak di-stub)
| File test | Menutup |
|---|---|
| `approvals/__tests__/apiHooks.test.tsx` (baru, 8) | path inbox/detail/approve/reject; `useMasterDataDetail` idle tanpa id; `useApprove` me-refresh approvals + admin-vendors + admin-atms setelah sukses **dan** gagal (409 stale); `useReject` hanya setelah sukses |
| `approvals/__tests__/ApprovalInboxPage.test.tsx` (baru, 4) | loading, error + Coba Lagi, kosong (non-approver), hanya baris `master_data` punya tombol Tinjau → dialog review untuk request itu |
| `admin-users/__tests__/apiHooks.test.tsx` (baru, 6) | query string list (filter hanya yang diisi, `vendor_id=0` tetap terkirim), endpoint get/create/update/disable/enable, invalidasi per mutation, error 409 tanpa refresh |
| `admin-users/__tests__/UserFormDialog.test.tsx` (+6) | create LDAP (tanpa password sementara), local wajib vendor + password sementara, create local, edit (payload tanpa username/auth_source/password), error field 422 → field, error lain → toast, dialog tetap terbuka |
| `admin-users/__tests__/AdminUsersPage.test.tsx` (+6) | loading/error/kosong, paging via URL, toast sukses/gagal nonaktifkan, aktifkan user nonaktif, form create/edit, filter → halaman 1 |
| `admin-regions/__tests__/apiHooks.test.tsx` (baru, 6) | query string, endpoint CRUD/enable/disable, invalidasi, disable 409 (region dipakai lokasi aktif) |
| `admin-regions/__tests__/AdminRegionsPage.test.tsx` (baru, 11) | default URL (page 1, 20 baris, active), loading/error/kosong, paging, filter status → halaman 1, search debounce → URL, konfirmasi nonaktif/aktif + toast sukses/409, batal, form create/edit |
| `admin-holidays/__tests__/api.test.tsx` (baru, 10) | `useHolidays` (tahun + status, 100 baris), create/rename/enable/disable + refresh, 409 tanggal duplikat, import CSV (multipart, Bearer, tanpa token, pesan error server/default) |
| `admin-acm-areas/api.test.tsx` (baru, 12) | 4 query list + detail idle, dispatcher mutation create/rename/regions/members/enable/disable + refresh, 409 tanpa refresh |
| `role-management/__tests__/api.test.ts` (baru, 3) + `RoleManagementPage.test.tsx` (baru, 6) | path roles/catalog/create/permissions; halaman loading/error/kosong, daftar peran, editor izin untuk peran terpilih, dialog buat peran |

**Cakupan per fitur sesudah C2 (statements):** approvals 96,6 % · admin-users 94,8 % · admin-regions 93,9 % · role-management 88,3 % · admin-acm-areas 84,8 % · admin-holidays 83,8 %. Sisa kecil (dialog/panel ACM, halaman libur, `CreateRoleDialog`, `RegionFormDialog`) tidak dikejar di slice ini.

**Angka** → threshold baru = floor (VP tidak disentuh di C2):

| Portal | Statements | Branches | Functions | Lines | Threshold (sebelum → sesudah) |
|---|---|---|---|---|---|
| CompanyPortal | 66,67 % | 65,62 % | 61,01 % | 67,27 % | 61/62/54/62 → **66/65/61/67** |

**Bukti:** `pnpm run test:coverage` CP exit 0 (134 file, 1115 test); `pnpm lint` (475 file) + `pnpm build` hijau.

**Temuan (tidak diperbaiki, di luar scope test):**
- `admin-holidays/api.ts` `importHolidays` memakai `fetch` langsung + token dari store, bukan API client → upload CSV hari libur **tidak ikut refresh 401** (pola yang diseragamkan FE-08). Kandidat item FE_debt / bugfix.
- `admin-users/components/UserFormDialog.tsx` `Field` memakai `<span>` sebagai label, tidak terhubung ke input (`getByLabelText` tidak bisa) → aksesibilitas pembaca layar. Kandidat perbaikan UI kecil.

## C3 — Form master-data (CP)

**Test baru** (pola sama seperti C2; uang selalu dicek sebagai string desimal, bukan float)
| File test | Menutup |
|---|---|
| `admin-vendors/__tests__/apiHooks.test.tsx` (baru, 41, table-driven) | semua query vendor (list + filter, detail idle tanpa id, children per jenis, branch/vault/PIC/paket/tarif/ATM cabang dengan query string lengkap) dan **semua 22 submission maker-checker** (create/update/disable/enable vendor, cabang, vault, PIC, paket cabang, tarif vendor-wide): endpoint benar, hasil `change_request_id` diteruskan, lalu refresh pohon `admin-vendors` + badge `master-data/pending`; submission ditolak (409) tidak me-refresh apa pun; DSR location → vault map (list, unmapped, create/update/disable/enable) |
| `master-data-io/api.test.ts` (baru, 6) | export/template CSV sebagai blob, nama file dari `Content-Disposition` atau fallback, object URL di-revoke; dry-run/confirm import multipart; error → `ImportApiError` dengan error per baris (422), default "Permintaan gagal", error non-API diteruskan apa adanya |
| `admin-vendors/lib/formSchemas.test.ts` (baru, 27) | skema paket cabang, tarif vendor-wide, PIC, vault: harga ≤ 2 desimal & tidak negatif & tanpa koma, tingkat bulat, mata uang 3 huruf, tanggal akhir ≥ mulai, override cabang XOR ATM, email PIC opsional tapi valid, kategori vault ATM/CASH/ATM_CASH |
| `admin-vendors/__tests__/PackageFormDialogs.test.tsx` (baru, 9) | dialog harga khusus cabang + tarif vendor-wide: payload create (cabang dari konteks, opsional kosong → `null`, `base_price` tetap string, tanpa `package_code` di tarif), edit hanya kirim harga/SLA/tanggal akhir dengan field grain terkunci, hanya cabang aktif sebagai override, toast "menunggu persetujuan (#id)", 422 → field, error lain → toast |
| `admin-vendors/__tests__/PackagesPanel.test.tsx` (baru, 8) | daftar harga khusus cabang: label tingkat, `IDR <string desimal>`, status Berlaku/Berakhir per tanggal akhir, baris pending di-badge + terkunci, pengajuan baru hanya untuk cabang ini, Akhiri → konfirmasi → toast sukses/gagal, form create/edit, loading/error/kosong |
| `admin-vendors/__tests__/VendorBranchEditPage.test.tsx` (baru, 7) | halaman ubah cabang: loading/tidak ditemukan, prefill + kode cabang terkunci, payload (region code di-trim + upper-case, opsional kosong → `null`), toast + kembali ke vendor, validasi nama, 422 → field, error lain → toast, Batal |

**Cakupan sesudah C3 (statements):** admin-vendors 42,7 % → **80,4 %** (tidak ada file 0 % lagi) · master-data-io 47,9 % → 84,9 % · admin-atms 88,0 % (tidak disentuh; sudah di atas 80 %).

**Angka** → threshold baru = floor (VP tidak disentuh, kecuali config di bawah):

| Portal | Statements | Branches | Functions | Lines | Threshold (sebelum → sesudah) |
|---|---|---|---|---|---|
| CompanyPortal | 73,26 % | 69,68 % | 68,14 % | 74,15 % | 66/65/61/67 → **73/69/68/74** |

**Bukti:** `pnpm run test:coverage` CP exit 0 (141 file, 1213 test — termasuk perubahan test `admin-holidays` dari session paralel "Fix: holidays CSV import bypasses 401 refresh"); `pnpm lint` (482 file) + `pnpm build` hijau; VP `pnpm lint` hijau.

**Perbaikan config (kekurangan C0):** `pnpm run test:coverage` tanpa override reporter menulis laporan ke `coverage/` di folder app, dan Biome ikut me-lint-nya (lint merah: `coverage-final.json` 1,8 MiB). Ditambahkan `"coverage"` ke `files.ignore` di `biome.json` kedua portal, dan `coverage` ke `.gitignore` VP (CP sudah ada) supaya laporan tidak ikut ter-commit.

**Temuan (tidak diperbaiki):** pola `Field` dengan label `<span>` yang tidak terhubung ke input (lihat C2) juga ada di `VendorPackageFormDialog`, `VendorPackagePriceFormDialog` dan `VendorBranchEditPage` — dan dua dialog harga hampir identik (±290 baris masing-masing). Kandidat perbaikan a11y sekaligus FE-12.

## C4 — Uang / kuota / order (CP + VP)

**Test baru / diperluas**
| File test | Menutup |
|---|---|
| CP `vendor-request/__tests__/apiHooks.test.tsx` (baru, 32) | semua query (opsi vendor, browse forecast dengan semua filter, summary, list dengan semua filter, detail, audit log) + kapan idle; select-all lintas halaman per 100 baris, berhenti di halaman 1 bila melewati cap (tidak pernah seleksi parsial); semua aksi (create, edit items, submit, approve, reject `rejection_reason`, revise, cancel `cancellation_reason`, laporan selesai submit/approve/reject) + refresh; approve laporan selesai ikut refresh `atm-visit-quota`; aksi ditolak tidak refresh; `getVendorRequestErrorMessage` |
| CP `vault-plan/api.test.tsx` (baru, 15) | list (filter kosong tidak dikirim), detail, kandidat selalu `urgent=true`, review ATM-SPV (plan / null), save alokasi + buffer reason, submit/approve/reject, vault-approve/reject refresh plans + vendor-requests, save ditolak tidak refresh; `formatAmount` (string IDR utuh, sampai 2^53) dan `formatDenoms` (denom besar → kecil, nol dibuang) |
| CP `replenishment/__tests__/api.test.tsx` (baru, 10) | `rangeDays`/`rangeError` (maks 31 hari, inklusif), query klasifikasi (filter hanya yang diisi, `vendor_id=0` tetap terkirim, paging 20), rentang tidak valid tidak di-query, summary |
| CP `atm-portal/__tests__/dataHooks.test.tsx` (baru, 14) | `visitSisa` (sisa vs kelebihan), kuota per ATM (terminal id di-encode), reset ATM / reset vendor / batal kunjungan + refresh kuota & vendor-requests, 403 tidak refresh; query string portal + cashpos (hanya mode cashpos); profil ATM + riwayat (hanya bila aktif), 404 tidak di-retry / 500 di-retry sekali; URL state profil (default 30 hari, merge ke URL) |
| CP `atm-portal/__tests__/AtmProfileScreen.test.tsx` (baru, 10) | halaman profil ATM dengan header, tab, dan dua tabel riwayat asli: tidak ditemukan / 404 / error + retry, Rupiah dari string desimal (`Rp 250.000.000,50`), tanggal Indonesia, filter tanggal → halaman 1, tab via klik + panah, cashpos angka utuh tanpa pembulatan, cashpos menolak tanggal akhir < awal, live region pembaca layar |
| VP `features/dsr/__tests__/DsrDetailDialog.test.tsx` (baru, 4) | detail DSR harian + rencana isi (angka desimal tepat, baris dilewati), hanya sheet yang ada yang di-fetch, tutup via tombol/Escape/overlay; `listDsrUploads` query string |
| VP `features/dsr/__tests__/DsrPage.test.tsx` (baru, 6) | DSR Monitor: loading, kosong, status per sheet sebagai teks, filter tanggal + Tampilkan Semua, buka detail, upload sukses → refresh list |
| VP `features/notifications/__tests__/NotificationsPage.test.tsx` (baru, 6) | loading/error/kosong, waktu WIB, isi dipotong, status Read/Unread teks, buka = tandai dibaca + ikuti link (klik/Enter/Spasi), Mark all hanya saat ada unread |
| VP `components/ui/__tests__/FileUpload.test.tsx` (+7) | drop valid ditambahkan, drop kosong, antrean penuh, drop campuran (yang valid tetap masuk), hapus file membersihkan error, Enter/Spasi membuka pemilih file, error dari parent |

**Angka** → threshold baru = floor:

| Portal | Statements | Branches | Functions | Lines | Threshold (sebelum → sesudah) |
|---|---|---|---|---|---|
| CompanyPortal | 78,21 % | 73,69 % | 74,25 % | 79,26 % | 73/69/68/74 → **78/73/74/79** |
| VendorPortal | 81,43 % | 73,03 % | 77,80 % | 82,44 % | 75/64/67/75 → **81/73/77/82** |

VP statements/lines sudah ≥ 80 %. Sisa gap branches VP terutama layar prototipe (`evidence`, `invoices`, `schedule`) = C5.

**Bukti:** `pnpm run test:coverage` exit 0 (CP 146 file / 1294 test, VP 25 file / 214 test); `pnpm lint` + `pnpm build` hijau di kedua portal.

**⚠ Temuan (tidak diperbaiki di slice ini, sengaja tidak dikunci oleh test):**
1. **CP `vendor-request/vendorSide.ts` path ganda `/api/v1`.** `BASE = "/api/v1/vendor-requests"`, padahal API client sudah menambah `apiConfig.baseURL` (`/api/v1`). Akibatnya panel pihak vendor (`GET /{id}/vendor-parties`) dan aksi ATM-SPV "kembalikan ke vendor" (`POST /{id}/vendor-return`) memanggil `/api/v1/api/v1/vendor-requests/...` → 404 (backend: `vendor_request_handler.go:79-80`). Kandidat bugfix prioritas tinggi (fitur cit-send-vendor).
2. **CP `replenishment/api.ts` `downloadReport` memakai `fetch` langsung** + token dari store → unduh laporan CSV realisasi tidak ikut refresh 401 (pola sama dengan bug impor hari libur yang sudah diperbaiki 2026-10-15).
3. **VP `features/dsr/DsrPage.tsx` tanggal awal di-hardcode `"2024-01-15"`.** Vendor yang membuka DSR Monitor melihat filter tanggal 2024 (hampir pasti kosong) sampai tanggal diganti. Kandidat bugfix kecil (default ke hari ini WIB atau tanpa filter — keputusan PO).

## C5
Belum dimulai.
