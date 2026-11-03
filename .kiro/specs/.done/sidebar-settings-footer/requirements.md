# Requirements Document

## Introduction

Penyesuaian kecil pada shell CompanyPortal-Vite hasil `app-shell-revamp` (X.1 di `development-plan.md`):

1. Menu **Pengaturan** pindah dari grup ikon Topbar ke **bagian paling bawah Sidebar** (footer, di atas tombol "Ciutkan menu").
2. Kartu **EnvBadge** ("LINGKUNGAN" / "PRODUKSI · v0.1.0") di footer Sidebar dihapus.

### Kondisi kode saat ini (dicek 2026-10-27)
- `src/lib/config/navigation.ts`: item `settings` (`/settings`, roles `ADMIN`/`ADMIN_PARAM`/`APPACCESS`, group `system`) memakai `placement: "topbar"`. `getVisibleNav` mengeluarkannya dari `sidebar` dan menaruhnya di `VisibleNav.settings`.
- `src/components/layout/AppShell.tsx` meneruskan `visible.settings` ke `Topbar`; `topbar/TopbarIconButton.tsx` (`TopbarIconGroup`) merender tombol ikon Pengaturan (varian `Link`, `aria-current` saat aktif). Varian `Link` dari `TopbarIconButton` hanya dipakai oleh Pengaturan.
- `src/components/layout/sidebar/SidebarFooter.tsx`: `EnvBadge` (label "Lingkungan", chip non-prod, `PRODUKSI · v{version}`) + tombol "Ciutkan menu". `APP_ENV`/`APP_VERSION` dari `src/lib/config/appEnv.ts` hanya dikonsumsi oleh `SidebarFooter`.
- Breadcrumb "Sistem › Pengaturan" dan CommandPalette (memuat Pengaturan) tidak bergantung pada lokasi tombol.

### Ruang lingkup
- `navigation.ts` (penanda lokasi), `AppShell.tsx`, `Sidebar.tsx`, `sidebar/SidebarFooter.tsx`, `Topbar.tsx`, `topbar/TopbarIconButton.tsx`, dan test terkait.

### Tidak termasuk
- VendorPortal-Vite, backend, route guard `/settings`, isi halaman Pengaturan, CommandPalette, Breadcrumb.
- Perubahan role yang boleh melihat Pengaturan.

### Area sensitif
Tidak ada (bukan auth/money/rekonsiliasi/migrasi). Visibilitas Pengaturan tetap ditentukan `filterNavByRoles`; route guard `/settings` tidak berubah.

### Keputusan / asumsi
- **D1** — Pengaturan dirender di footer Sidebar, di luar area scroll menu, sehingga selalu terlihat di paling bawah. ✅ mengikuti permintaan user 2026-10-27.
- **D2** — EnvBadge dihapus di **semua** lingkungan (termasuk chip UAT di non-prod); tidak ada teks info lingkungan di Sidebar. ✅ DIPUTUSKAN PO 2026-10-27.
- **D3** — `appEnv.ts` dan test-nya dibiarkan (tidak dihapus); konfigurasi build `VITE_APP_ENV`/`__APP_VERSION__` tetap ada untuk pemakaian berikutnya. Mengikuti saran.
- **D4** — Ini adalah deviasi dari `app-shell-revamp` Req 2.3, 4.3 (bagian EnvBadge), 5.1–5.4, 10.1–10.3. Spec ini menggantikan criteria tersebut; catatan supersede ditambahkan di `app-shell-revamp/requirements.md` (task 4). ✅ DIPUTUSKAN PO 2026-10-27.

## Glossary

- **Sidebar**: navigasi kiri CompanyPortal (`Sidebar.tsx`).
- **SidebarFooter**: area bawah Sidebar dengan garis atas (`sidebar/SidebarFooter.tsx`).
- **Topbar**: bar atas 64px (`Topbar.tsx` + `topbar/TopbarIconButton.tsx`).
- **NavConfig**: `NAV_CONFIG` + `getVisibleNav` di `navigation.ts`.
- **SettingsItem**: item NavConfig `id: "settings"` (label "Pengaturan", href `/settings`).
- **EnvBadge**: kartu "LINGKUNGAN" + nama lingkungan + versi di SidebarFooter.
- **Rail**: mode Sidebar ciut (76px, ikon saja).
- **ActiveNavItem**: hasil `findActiveNavItem(visible.all, pathname)`.

## Requirements

### Requirement 1: Pengaturan di paling bawah Sidebar

**User Story:** Sebagai admin, saya ingin menu Pengaturan ada di bawah Sidebar, agar letaknya konsisten dengan menu lain dan mudah ditemukan.

#### Acceptance Criteria

1. THE NavConfig SHALL menandai SettingsItem dengan lokasi footer Sidebar (bukan topbar) dan tetap mengeluarkannya dari grup menu Sidebar, tersedia lewat `VisibleNav.settings`.
2. WHERE role user lolos `filterNavByRoles` untuk SettingsItem, THE SidebarFooter SHALL merender link Pengaturan (ikon Settings2 + label) ke `/settings`, di atas tombol "Ciutkan menu"; selain itu link tidak dirender.
3. THE Sidebar SHALL menempatkan link Pengaturan di luar area menu yang bisa di-scroll, sehingga tetap terlihat di bagian paling bawah Sidebar berapa pun panjang menu.
4. WHILE ActiveNavItem adalah SettingsItem, THE SidebarFooter SHALL menampilkan link Pengaturan dengan gaya aktif yang sama dengan item Sidebar lain dan `aria-current="page"`.
5. WHILE Sidebar dalam mode Rail, THE SidebarFooter SHALL menampilkan Pengaturan sebagai ikon di tengah dengan tooltip "Pengaturan" (`role="tooltip"`, `aria-describedby`), sama seperti item Sidebar lain.
6. THE SidebarFooter SHALL membuat link Pengaturan bisa dicapai dengan Tab dan memiliki focus ring yang terlihat.

### Requirement 2: Tombol Pengaturan dihapus dari Topbar

**User Story:** Sebagai pengguna, saya ingin hanya ada satu tempat untuk Pengaturan, agar tidak membingungkan.

#### Acceptance Criteria

1. THE Topbar SHALL tidak merender tombol Pengaturan untuk role apa pun; grup ikon Topbar berisi toggle tema Sidebar dan Notifikasi, lalu divider.
2. WHEN ActiveNavItem adalah SettingsItem, THE Topbar SHALL tetap menampilkan breadcrumb "Sistem › Pengaturan".
3. THE CommandPalette SHALL tetap menampilkan Pengaturan dengan grup "Sistem" untuk role yang lolos `filterNavByRoles`.

### Requirement 3: EnvBadge dihapus

**User Story:** Sebagai pengguna, saya tidak butuh teks lingkungan dan versi di Sidebar, agar footer lebih bersih.

#### Acceptance Criteria

1. THE SidebarFooter SHALL tidak menampilkan teks "LINGKUNGAN", nama lingkungan, chip lingkungan, maupun versi aplikasi, di lingkungan apa pun dan di mode Expanded maupun Rail.
2. THE SidebarFooter SHALL tetap memiliki garis atas dan tombol "Ciutkan menu"/"Lebarkan menu" dengan ikon, hint kbd `[`, dan perilaku yang sama seperti sebelumnya.

### Requirement 4: Tidak ada regresi

**User Story:** Sebagai tim QA, saya ingin perubahan ini tidak merusak navigasi lain.

#### Acceptance Criteria

1. THE Sidebar SHALL tetap merender grup dan item menu lain (urutan, badge, lipat grup, roving tabindex dengan Arrow/Home/End) tanpa perubahan; link Pengaturan tidak ikut dalam roving tabindex grup menu.
2. THE CompanyPortal SHALL lolos `pnpm run lint`, `pnpm run test:coverage` (threshold 84/80/80/85), dan `pnpm run build`, serta `tsc -b tsconfig.test.json` tidak menambah error di file yang disentuh spec ini.
