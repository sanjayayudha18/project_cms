# Spec: Coverage frontend naik ke 80% (FE-13)

Status: **accepted 2026-10-15** (PO; Q1–Q4 sesuai usulan). Stage: 2 Design. Stage berikut: `plan.md` (daftar file test per slice).
Sumber: `intent.md` (draft 2026-10-15). Angka baseline: lihat intent.

## ⚠ Flagged (Sec 4 #7)
Tidak ada. Hanya test + config. Test untuk auth (C1) dan maker-checker (C2) **membaca** perilaku yang ada dan tidak mengubahnya. Bila test membuktikan ada bug, bug itu masuk `bugfixes.md` lewat `cms2-bugfix` dengan keputusan PO terpisah.

## Functional requirements

**FR-1 — Config coverage (slice C0).** Kedua `vite.config.ts`:
```ts
test: {
  // …yang sudah ada
  coverage: {
    provider: "v8",
    include: ["src/**"],
    exclude: [/* Q2: test, setup, main.tsx, *.d.ts, data/ */],
    thresholds: { statements, branches, functions, lines },
  },
}
```
Nilai awal = baseline setelah `exclude` diterapkan, dibulatkan ke bawah ke bilangan bulat. Script `test` tidak berubah (`vitest run`). Coverage dijalankan lewat `pnpm test -- --coverage` (lokal, nanti juga di CI/Jenkins).

**FR-2 — Ratchet.** Setelah tiap slice, threshold = floor(angka terukur), dan tidak pernah lebih rendah dari nilai sebelumnya. Satu commit per slice: test + kenaikan threshold.

**FR-3 — Urutan slice (prioritas risiko × statement belum tercover).** Perkiraan statement belum tercover diambil dari baseline 2026-10-15. Target threshold di bawah adalah perkiraan; angka final = angka terukur.

| Slice | Isi | CP belum tercover | VP belum tercover | Perkiraan threshold sesudah (CP / VP, statements) |
|---|---|---|---|---|
| C0 | Pasang config + threshold baseline (FR-1) | – | – | ~61 / ~63 |
| C1 | **Auth + API client.** CP `lib/auth` (17), `lib/api` (3). VP `features/auth` (79: AuthContext, ProtectedRoute, LoginPage), `lib/api/client.ts` (23: refresh 401 single-flight, `onAuthFailure`, FormData) | 20 | 102 | ~61 / ~72 |
| C2 | **Maker-checker + admin.** CP `approvals` (33), `role-management` (40), `admin-users` (85), `admin-holidays` (34), `admin-regions` (111), `admin-acm-areas` (36) | 339 | – | ~66 / ~72 |
| C3 | **Form master-data.** CP `admin-vendors` (530), `master-data-io` (38), `admin-atms` (42) | 610 | – | ~73 / ~72 |
| C4 | **Uang / kuota / order.** CP `vendor-request` (249), `atm-portal` (249), `replenishment` (35), `vault-plan` (48). VP `dsr` (46), `orders` (23), `realizations` (6), `components/ui` (26), `notifications` (12) | 581 | 113 | ~80 / ~82 |
| C5 | **Sisa.** CP `eod-monitoring` (145), `routes` (159), `dashboard`, `components/layout`, `lib/utils`, layar prototipe (Q1). VP `schedule`/`evidence`/`invoices` (Q1) | sisa | sisa | ≥ 80 keempat metrik |

Catatan:
- VP sudah bisa mencapai 80 % statements setelah C4. **Branches VP** (54,7 %) adalah gap terbesar di VP, jadi test C1/C4 VP harus menutup cabang error (401/403/422/429/503, network error, role guard), bukan hanya happy path.
- C2–C4 menguji perilaku yang terlihat oleh user (render, submit, error server, tombol disabled untuk maker = checker, 202 "menunggu persetujuan"), bukan detail implementasi.
- Isi per file di tiap slice ditentukan di `plan.md`.

**FR-4 — Selesai.** Setelah C5: threshold 80/80/80/80 (atau lebih) di kedua portal, `FE_debt.md` FE-13 = `done <sha>`.

## Non-functional
- Durasi `vitest run` tidak boleh naik drastis. Sekarang CP ~35 s, VP ~25 s. Bila sebuah slice menambah > 30 % durasi, catat di `tests.md`.
- Tidak ada test yang di-`skip`/`todo` untuk mengejar angka.
- Tidak ada dependency baru (coverage-v8 VP sudah dipasang dengan persetujuan PO).

## Out of scope
- E2E/Playwright dan menjalankan coverage di Jenkins (FE-06 / `ci-jenkins`).
- Refactor komponen besar demi testability (FE-12).
- Perubahan pola auth (FE-09 → `fe-auth-unify`).

## Keputusan spec (dikonfirmasi PO 2026-10-15)
Q1–Q4 di `intent.md`. Usulan default: Q1 tetap dihitung, Q2 exclude non-kode saja, Q3 C0 memindahkan threshold dari `fe-tech-debt` E2, Q4 target 80 % untuk keempat metrik.
