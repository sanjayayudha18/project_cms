# Intent: Coverage frontend naik ke 80% (FE-13)

Author: user (product owner), ditulis bersama Claude. Status: **accepted 2026-10-15** (PO: "Terima intent + spec fe-coverage (Q1–Q4 sesuai usulan)").
Stage: 1 Plan. Stage berikut: `spec.md` (draft sudah disiapkan bersama intent ini, menunggu intent diterima).
Asal: backlog [`.claude/FE_debt.md`](../../FE_debt.md) FE-13. Di-split dari `fe-tech-debt` (spec S1: kenaikan ke 80% dikerjakan bertahap sebagai item baru). Keputusan PO 2026-10-15: AI-DLC terpisah, bukan addendum `fe-tech-debt`.
Menyentuh Sec 4 #7: **tidak**. Hanya test + config Vitest. Tidak ada perubahan kode produksi, kecuali bila test menemukan bug (bug itu dicatat terpisah di `bugfixes.md`, bukan diperbaiki diam-diam di sini).

## Problem
CLAUDE.md Sec 8 menuntut coverage ≥ 80% di `internal/*`. Untuk frontend target yang sama dipakai (common/testing.md). Kondisi terukur 2026-10-15 (basis **semua file `src/**`**, keputusan PO 2026-10-15):

| | Statements | Branches | Functions | Lines | Test |
|---|---|---|---|---|---|
| CompanyPortal | 61,37 % (3505/5711) | 62,37 % (2707/4340) | 54,64 % (1271/2326) | 61,74 % (3204/5189) | hijau |
| VendorPortal | 63,01 % (697/1106) | 54,74 % (427/780) | 58,80 % (227/386) | 63,70 % (653/1025) | 20 file / 145 test hijau |

Catatan baseline:
- Angka FE-13 lama (CP 69,85/70,11/64,13/70,64) dihitung dengan default v8, yang hanya menghitung file yang di-load test. Dengan basis yang sama hari ini CP = 68,49/69,48/62,33/69,21. Selisih ~7 poin berasal dari file yang sama sekali tidak punya test.
- **Threshold belum terpasang di kedua portal.** FE-13 menulis "threshold dipasang", tapi `vite.config.ts` keduanya belum punya `test.coverage`. Itu bagian FR-E2 `fe-tech-debt`, yang belum dikerjakan (slice E tertahan bersama FE-06/Jenkins). Jadi coverage sekarang bisa turun tanpa ada yang gagal.
- `@vitest/coverage-v8 ^4.1.10` dipasang di VendorPortal 2026-10-15 (persetujuan PO, sama dengan S4 `fe-tech-debt`) agar baseline VP bisa diukur. Versi terkunci 4.1.10 = versi `vitest` yang ter-resolve (peer dependency harus persis sama).

Area dengan risiko tinggi tapi coverage rendah:
- **Auth/API client**: VP `features/auth` 66,9 %, VP `lib/api/client.ts` 59,6 % (refresh 401). CP `lib/auth` 80,9 %, `lib/api` 94,1 %.
- **Maker-checker**: CP `approvals` 62 % (`api.ts`, `hooks.ts`, `ApprovalInboxPage.tsx` 0 %), `role-management` 61 %, `admin-regions` 32 %, `admin-holidays` 50 %, `admin-users` 56 %.
- **Form master-data**: CP `admin-vendors` 42,7 % (530 statement belum tercover, terbesar; `PackagesPanel`, `VendorBranchEditPage` 0 %), `master-data-io` 47,9 %.
- **Uang/kuota**: CP `vendor-request` 68,7 % (249), `atm-portal` 47,2 % (249; 7 file 0 %), `replenishment` 62 %.

## Proposed outcome
1. Kedua portal memasang `test.coverage.thresholds` di `vite.config.ts`, mulai dari baseline (dibulatkan ke bawah). Mulai saat itu coverage tidak bisa turun diam-diam.
2. Coverage naik bertahap per slice, dengan prioritas risiko: auth → maker-checker → form master-data → uang/kuota → sisanya. Setelah tiap slice threshold dinaikkan ke angka terukur (dibulatkan ke bawah). **Threshold hanya boleh naik.**
3. Akhir: keempat metrik ≥ 80 % di kedua portal, `FE-13` = done.

## Affected users and systems
- Users: tidak ada perubahan perilaku.
- Systems: `frontend/*/vite.config.ts`, `frontend/*/src/**/*.test.ts(x)`, `frontend/VendorPortal-Vite/package.json` + lockfile (sudah: coverage-v8).

## Constraints
- Test mengikuti pola yang sudah ada (Vitest + Testing Library + mock `fetch`/`msw` sesuai tetangga, helper `src/test/` di CP, `routerTestUtils` di VP). Tidak ada dependency baru.
- Tidak mengubah kode produksi demi test. Bila sebuah komponen tidak bisa dites tanpa refactor, catat dan lewati, jangan refactor di sini.
- Lint, tsc, build, dan test hijau di kedua portal setiap slice.

## Open questions (untuk PO)
- **Q1 — Layar prototipe/mock.** Layar yang disembunyikan di `fe-tech-debt` S2 (CP `dmaa-forecast`, `forecast`, `dsr`, `cit`; VP `schedule`, `evidence`, `invoices`) tetap dihitung? Usul: **tetap dihitung**, dites paling akhir (slice C5). Mengecualikannya menaikkan angka tanpa menambah jaminan.
- **Q2 — Pengecualian non-kode.** Usul: keluarkan dari hitungan hanya file yang bukan kode aplikasi: `src/**/*.{test,spec}.*`, `src/test/**`, `src/test-setup.ts`, `src/main.tsx`, `src/**/*.d.ts`, `src/data/**`. Contoh: VP `test/routerTestUtils.tsx` sekarang terhitung 0 %.
- **Q3 — Pemilik threshold.** FR-E2 `fe-tech-debt` (threshold di `vite.config.ts`) belum dikerjakan. Usul: bagian `vite.config.ts` dipindah ke slice C0 AI-DLC ini. `fe-tech-debt` E2 tinggal menjalankan `--coverage` di CI (Jenkins, FE-06).
- **Q4 — Functions CP.** Functions CP 54,6 % → 80 % butuh ±590 fungsi tambahan, kebanyakan handler kecil di komponen presentasi. Target tetap 80 % untuk keempat metrik? Usul: **ya**. Bila setelah C4 functions jelas tertinggal, PO memutuskan ulang dengan data.
