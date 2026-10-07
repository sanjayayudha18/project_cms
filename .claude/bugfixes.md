# Bug Fix Log

Small fixes that the user decided don't need a full AI-DLC chain (CLAUDE.md Sec 4a rule 0). Newest first. One entry per fix.

Template:

```markdown
## YYYY-MM-DD — <short title>
- **Symptom**: what the user saw
- **Root cause**: why it happened
- **Fix**: what changed (files)
- **Tests**: automated check added/run; manual browser verification: outstanding
- **Commit**: <sha>
```

---

## 2026-10-07 — Kelolaan ATM: dropdown "Paket seluruh vendor" hanya label + tabel terpotong
- **Symptom**: di `/settings/admin/atms` → Kelolaan ATM, mode "Paket seluruh vendor" hanya menampilkan label (PAKET 3/4/5) tanpa kode paket/tingkat/harga; modal terlalu sempit sehingga kolom tabel riwayat kelolaan terpotong.
- **Root cause**: `ListATMPackageOptions` mengembalikan `DISTINCT package` saja; `ATMAssignmentsDialog` memakai lebar default `Dialog` (`max-w-lg`).
- **Fix**: query `ListATMPackageOptions` (`backend/queries/atm_assignments_admin.sql` + sqlc regen) kini mengembalikan baris tarif (`package`, `package_code`, `tier_min`, `tier_max`, `base_price` sebagai teks, `currency`); signature repo/service/handler ikut. `PackageSourceFields.tsx` menampilkan `kode · label · tingkat · harga` per opsi; yang dikirim saat submit tetap label (`avp.package`) — kontrak API create tidak berubah. `ATMAssignmentsDialog.tsx` → `max-w-4xl` + tabel dibungkus `overflow-x-auto`. Endpoint `GET .../assignment-package-options` berubah shape: `packages` kini array objek (satu-satunya konsumen adalah dialog ini).
- **Tests**: Go handler/service/repository unit + `-tags integration TestIntegration_VendorWideAssignment*` lulus; vitest `admin-atms` 84/84, lint + build lulus; manual browser verification: outstanding.
- **Commit**: _(belum)_

---

## 2026-09-30 — Halaman bisa di-scroll ke area kosong (forecast-browser)
- **Symptom**: `/replenishment/forecast-browser` masih bisa scroll ke bawah melewati konten; seluruh shell (sidebar + main) naik dan menyisakan area kosong.
- **Root cause**: elemen `sr-only` (`position: absolute`) di `ForecastSummary.tsx`/`ForecastBrowser.tsx` tidak punya ancestor ber-`position`, jadi containing block-nya = viewport. Elemen itu lolos dari clipping `overflow` `<main>`/grid dan memperpanjang tinggi dokumen.
- **Fix**: `AppShell.tsx` — `<main>` diberi `relative` (berlaku untuk semua halaman protected). Grid rows `auto minmax(0, 1fr)` ikut di diff yang sama.
- **Tests**: `AppShell.test.tsx` assert `<main>` punya `relative` — 11/11 lulus; manual browser verification: outstanding.
- **Commit**: _(belum)_
