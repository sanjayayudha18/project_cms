# Implementation Plan

## Overview

Urutan: ubah penanda lokasi di NavConfig dulu (type `NavPlacement` dipakai komponen dan fixture test), lalu Sidebar dan Topbar secara paralel karena file-nya terpisah, terakhir gate kualitas. Pola komponen mengikuti `app-shell-revamp` (pakai ulang `SidebarNavItem`, token `--shell-*`).

### Model Split

**Sonnet-runnable**
- 1. NavConfig: placement `sidebar-footer` — Sonnet, Low
- 2. Sidebar footer: link Pengaturan + hapus EnvBadge — Sonnet, Medium (needs 1)
- 3. Topbar: hapus tombol Pengaturan — Sonnet, Low (needs 1)
- 4. Checkpoint lint/coverage/build — Sonnet, Medium (needs 2, 3)

**Opus-required**
- none

## Tasks

- [x] 1. NavConfig: pindahkan penanda lokasi Pengaturan ke footer Sidebar
  - Ubah `NavPlacement` menjadi `"sidebar" | "sidebar-footer"`, item `settings` → `placement: "sidebar-footer"`, `getVisibleNav` memfilter dan mengisi `settings` berdasarkan nilai baru.
  - Perbarui `navigation.test.ts` ("placement sidebar-footer") dan `navigation.property.test.ts` (Property 2), serta fixture `placement` di `CommandPalette.test.tsx`.
  - _Requirements: 1.1, 2.3_
  - _Model: Sonnet, Effort: Low — perubahan union type + satu field, jalurnya jelas._

- [ ] 2. Sidebar footer: link Pengaturan di paling bawah, EnvBadge dihapus
  - [x] 2.1 Implementasi `SidebarFooter` + `Sidebar`
    - Hapus `EnvBadge`, `EnvBadgeProps`, prop `env`/`version`, import `appEnv` (file `appEnv.ts` dibiarkan, D3).
    - Tambah prop `settings?: NavItem` + `settingsActive: boolean`; render `SidebarNavItem` sebelum tombol Ciutkan bila `settings` ada.
    - `Sidebar` meneruskan `visible.settings` dan `settingsActive`; footer tetap di luar `<nav>` scroll.
    - _Requirements: 1.2, 1.3, 1.4, 1.5, 1.6, 3.1, 3.2_
  - [x] 2.2 Test `Sidebar.test.tsx`
    - Ganti test Req 2.3 lama: link Pengaturan ada di footer dan tidak di dalam `navigation` "Menu navigasi".
    - Test "every link has the right href": ganti filter `i.placement !== "topbar"` menjadi `!== "sidebar-footer"` (setelah task 1 perbandingan lama jadi TS2367), lalu tambah asersi href link Pengaturan di footer.
    - Aktif: `aria-current="page"` saat `active` = settings. Rail: tooltip "Pengaturan" via hover/focus + `aria-describedby`. Tab mencapai link.
    - Property 1 (sisi Sidebar): untuk setiap role, link ada ⇔ `getVisibleNav(role).settings` terdefinisi.
    - Property 3: untuk setiap rail × role, tidak ada "Lingkungan", nilai `ENV_LABEL` mana pun, atau `v{APP_VERSION}`; tombol Ciutkan/Lebarkan tetap ada. Hapus dua test EnvBadge lama (`env="uat"`/`env="prod"`) dan test "rail hides EnvBadge".
    - Test keyboard roving tabindex yang ada tetap hijau tanpa perubahan.
    - _Requirements: 1.2, 1.3, 1.4, 1.5, 1.6, 3.1, 3.2, 4.1_
  - _Model: Sonnet, Effort: Medium — komponen + a11y (tooltip Rail, roving tabindex) perlu dicek, tapi polanya sudah ada._

- [ ] 3. Topbar: hapus tombol Pengaturan
  - [x] 3.1 Implementasi
    - `Topbar`: hapus prop `settings`. `TopbarIconGroup`: hapus `settings`/`settingsActive`. `TopbarIconButton`: hapus varian `Link` (`to`, `active`, import `Link`/`Settings2`).
    - `AppShell`: hapus `settings={visible.settings}` pada `<Topbar>`.
    - _Requirements: 2.1, 2.2, 2.3_
  - [x] 3.2 Test `Topbar.test.tsx` + `AppShell.test.tsx`
    - Hapus `settings: settingsItem` dari default props `renderTopbar` (setelah 3.1 jadi excess property). `settingsItem` tetap dipakai untuk test breadcrumb.
    - Property 1 (sisi Topbar): tidak ada link "Pengaturan" di Topbar untuk semua role (ganti tabel "Pengaturan for %s"); hapus test "Pengaturan is aria-current only when…".
    - Pertahankan test breadcrumb "Sistem › Pengaturan".
    - Pastikan test palette → `/settings` di `AppShell.test.tsx` tetap hijau.
    - _Requirements: 2.1, 2.2, 2.3_
  - _Model: Sonnet, Effort: Low — penghapusan prop dan dead code, hampir mekanis._

- [ ] 4. Checkpoint: gate kualitas CompanyPortal
  - `pnpm --dir frontend/CompanyPortal-Vite run lint`, `run test:coverage` (84/80/80/85), `run build`.
  - `pnpm --dir frontend/CompanyPortal-Vite exec tsc -b tsconfig.test.json`: tidak ada error baru di file yang disentuh (baseline di design.md Testing Strategy; 138 error lama di file lain bukan cakupan spec ini).
  - Tambah catatan "disupersede oleh `sidebar-settings-footer`" di `app-shell-revamp/requirements.md` untuk Req 2.3, 4.3 (EnvBadge), 5.1–5.4, 10.1–10.3.
  - Perbarui baris X.1 di `development-plan.md` (Status snapshot) bahwa follow-up ini selesai.
  - Cek browser manual oleh user: Expanded/Rail, ADMIN vs ATM-USER, tooltip Pengaturan di Rail tidak terpotong tepi Sidebar.
  - Status 2026-10-27: ✅ lint, test:coverage (85.7/81.62/82.17/86.77), build + `check-bundle.mjs`, tsc test 138 (= baseline), catatan supersede, baris X.1. ⏳ **Outstanding (user, GR#10): cek browser manual** — karena itu task 4 belum dicentang.
  - _Requirements: 4.2_
  - _Model: Sonnet, Effort: Medium — triage kegagalan lint/coverage bila ada._

## Notes

- D2 ✅ (PO 2026-10-27): EnvBadge dihapus di semua lingkungan, tanpa teks info lingkungan.
- D4 ✅ (PO 2026-10-27): catatan supersede di `app-shell-revamp/requirements.md` dikerjakan di task 4.
- Masuk di bawah X.1 (app-shell-revamp) di `development-plan.md`; tidak perlu item baru.
- Tidak menyentuh VendorPortal, backend, atau route guard `/settings`.

### Definition of Done
- Semua test hijau, coverage di atas threshold, build sukses.
- Pengaturan terlihat di paling bawah Sidebar untuk ADMIN/ADMIN_PARAM/APPACCESS, tidak ada di Topbar.
- Tidak ada teks "Lingkungan"/versi di Sidebar.

## Task Dependency Graph

Urutan pengerjaan per wave (task dalam satu wave boleh paralel, file-nya terpisah):

- **Wave 0:** 1 (NavConfig `sidebar-footer`), dasar type untuk semua task berikutnya.
- **Wave 1:** 2.1 (SidebarFooter + Sidebar) dan 3.1 (Topbar + AppShell), sesudah 1.
- **Wave 2:** 2.2 (test Sidebar) sesudah 2.1, dan 3.2 (test Topbar/AppShell) sesudah 3.1.
- **Wave 3:** 4 (gate lint/coverage/build + catatan supersede + cek browser), sesudah semua task di atas.

```json
{
  "waves": [
    { "id": 0, "tasks": ["1"] },
    { "id": 1, "tasks": ["2.1", "3.1"] },
    { "id": 2, "tasks": ["2.2", "3.2"] },
    { "id": 3, "tasks": ["4"] }
  ]
}
```
