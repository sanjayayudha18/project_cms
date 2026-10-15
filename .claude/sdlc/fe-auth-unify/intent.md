# Intent: Pola auth & util frontend antar portal (FE-09)

Author: user (product owner), ditulis bersama Claude. Status: **accepted 2026-10-15** (PO: opsi (b), S2 formatRp/formatThousands, S3 zustand dibiarkan).
Stage: 1 Plan. Stage berikut: `spec.md` (draft sudah disiapkan bersama intent ini, menunggu intent diterima).
Asal: backlog [`.claude/FE_debt.md`](../../FE_debt.md) FE-09, ditunda di `fe-tech-debt` (intent Q8, spec "Out of scope"). Keputusan PO 2026-10-15: AI-DLC terpisah.
Menyentuh Sec 4 #7: **auth** (store/context auth, API client, refresh 401, logout). **Full chain wajib.** Tidak ada migrasi, tidak ada perubahan backend.

## Problem
FE-09: *"Pola auth berbeda antar portal + UI/util terduplikasi. Samakan saat salah satu disentuh; shared package hanya bila duplikasi mulai merepotkan."*

Pertanyaan pertama untuk AI-DLC ini adalah **apakah perlu dikerjakan sekarang**, bukan bagaimana caranya. Kondisi terukur 2026-10-15:

**Auth.** CompanyPortal (CP) memakai Zustand (`lib/auth/store.ts`, 341 baris, 19 file pemakai). VendorPortal (VP) memakai React Context (`features/auth/AuthContext.tsx`, 313 baris, 7 file pemakai). Kontrak backend-nya sama (`/api/v1/auth/login|refresh|logout`, cookie `refresh_token` yang di-rotate, access token di memori). Perbedaan perilaku yang nyata:

| Aspek | CP (Zustand) | VP (Context) |
|---|---|---|
| Init `/refresh` saat load | single-flight `initializePromise` (komentar: StrictMode menjalankan effect 2×, token di-rotate, request kedua 401 dan menimpa sesi) | `useEffect` + flag `cancelled`; **dua request `/refresh` tetap terkirim** di StrictMode (dev) |
| Refresh gagal | clear state + hard redirect `/login?redirect=…&reason=session_expired` | clear state + `error = "Sesi berakhir…"`; `ProtectedRoute` yang me-redirect |
| Logout | clear state, `POST /logout`, `window.location = /login` (reload membuang cache query) | clear state + `queryClient.clear()`, `POST /logout`, tanpa reload |
| Login gagal | set `error`, tidak throw; 422 menampilkan detail per field | set `error` **dan throw**; 422 hanya `message` |
| Guard portal | tolak `VENDOR-USER` | hanya terima `VENDOR-USER` |
| Header login | `X-Portal-Type: company` | `X-Portal-Type: vendor` |
| API client | baca token langsung `useAuthStore.getState()` | token di-inject lewat `configureApiAuth()` + `useRef` yang disinkronkan di `useEffect` |
| API client lain | `baseURL`, `responseType: "blob"`, `headers` di response | path mentah, JSON saja |
| Session max lifetime | ditegakkan server (`SESSION_MAX_LIFETIME` → `/refresh` 401) → jalur "refresh gagal" | sama |

Temuan yang mungkin bug (belum direproduksi; cek browser adalah tugas user):
- **B1 — VP double `/refresh` di dev.** `Refresh` di backend mem-blacklist token lama, dan bila validasi gagal memanggil `clearRefreshCookie`. Dua request paralel dengan cookie yang sama bisa membuat request kedua 401 dan menghapus cookie baru dari request pertama. Akibatnya vendor ter-logout saat reload di **dev** (StrictMode). Produksi tidak terdampak karena StrictMode tidak menjalankan effect dua kali di build produksi. Ini kasus yang sama dengan yang sudah diperbaiki di CP.
- **B2 — VP token ref terlambat satu effect.** `accessTokenRef` diperbarui di `useEffect` milik `AuthProvider`. Effect anak berjalan lebih dulu, jadi query pertama setelah login bisa berangkat tanpa token → 401 → refresh → retry. Hasil akhirnya benar, tapi ada satu round-trip ekstra plus rotasi token. Perlu dicek di Network tab.

**UI/util.** File `components/ui` yang namanya sama di kedua portal (`Badge`, `Button`, `Card`, `DataTable`, `EmptyState`) **sudah berbeda hampir di setiap baris**, karena tema brand-nya memang beda (CP "Merah Sirih" vs VP "Merah Menyala", `rules/frontend-ui.md`). Yang benar-benar identik hanya `lib/utils/cn.ts` (1 fungsi).
- **B3 — `formatIDR` punya tiga arti.** CP `lib/utils/format.ts` → `"Rp1.000"`, CP `lib/utils/formatCurrency.ts` → `"1.000"` (tanpa mata uang), VP `lib/formatters.ts` → `"IDR 1.000"`. Ditambah CP `formatIDRFull` → `"IDR 1.000"`. Nama sama dengan output beda adalah risiko tampilan uang (Sec 13: mata uang wajib eksplisit). Duplikasi ini ada **di dalam CP**, bukan antar portal.
- VP mencantumkan `zustand ^5` di `package.json` tapi tidak memakainya di `src/`.

## Proposed outcome
PO memilih satu dari opsi di `spec.md`:
- (a) VP pindah ke Zustand,
- (b) pola dibiarkan, hanya perbaiki B1–B3 dan rapikan util,
- (c) shared package.

Apa pun pilihannya, kedua portal tetap app terpisah (Sec 5: internal LDAP vs vendor lokal), dan kontrak request/response JSON tidak berubah.

## Affected users and systems
- Users: tidak ada perubahan alur bagi user. Opsi (b) memperbaiki logout tak terduga di dev (B1).
- Systems: `frontend/VendorPortal-Vite/src/features/auth/*`, `src/lib/api/client.ts`. Untuk B3: `frontend/CompanyPortal-Vite/src/lib/utils/format*.ts` dan pemakainya. Opsi (a)/(c) melebar, lihat spec.

## Constraints
- Sec 5: dua SPA terpisah, login route terpisah, guard role per portal tetap.
- Request JSON yang ada tidak boleh berubah (header, body, `credentials: "include"`, retry 401 satu kali).
- Tidak ada dependency baru (opsi (a) memakai `zustand` yang sudah ada di VP).
- Cek manual login/logout/refresh di kedua portal = tugas user, dicatat outstanding.

## Open questions (untuk PO)
- **Q1 — Kerjakan sekarang?** Usul: **ya, tapi hanya opsi (b)**. B1 dan B3 adalah risiko nyata (sesi & tampilan uang). Menyamakan pola (a) tidak memberi manfaat bagi user, menambah risiko auth, dan sesuai backlog cukup dilakukan "saat salah satu disentuh".
- **Q2 — B3: nama mana yang menang?** Usul: hilangkan nama `formatIDR` yang ambigu di CP. `format.ts` diganti `formatRp`, sedangkan `formatCurrency.ts` diganti nama yang menyatakan "tanpa mata uang" (mis. `formatThousands`). Output setiap layar **tidak berubah** (rename saja). Mengubah tampilan agar mata uang selalu eksplisit adalah keputusan UI terpisah.
- **Q3 — `zustand` di VP.** Dibiarkan (dipakai bila nanti opsi (a)) atau dihapus sekarang?
