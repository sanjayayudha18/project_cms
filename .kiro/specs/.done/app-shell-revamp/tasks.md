# Implementation Plan

## Overview

Urutan: fondasi (font, env, token) → logika murni (`navigation.ts`, `lib/shell/*`, `initials`) → data badge → komponen Sidebar / Topbar / CommandPalette secara paralel → wiring `AppShell` → test integrasi + aksesibilitas → dokumentasi → checkpoint gate.

Referensi pola: design `## Components and Interfaces`; konvensi test di `src/lib/config/navigation.property.test.ts` dan `src/styles/contrast.property.test.ts` (konverter OKLCH inline); pola menu/escape di `NotificationBell.tsx`. Semua path relatif ke `frontend/CompanyPortal-Vite/`.

Keputusan PO D1–D14 ada di `design.md` (`## Overview`). D2 menyetujui dua dependency font (exact pin `5.3.0`). D14: `--shell-accent` hanya untuk shell, `--red-500` tidak disentuh.

### Model Split

**Sonnet-runnable**
- 1. Font + env build — Sonnet, Low
- 2. Token shell + style global — Sonnet, Medium (needs 1.1)
- 3. NavConfig + helper navigasi — Sonnet, Medium
- 4. Logika shell murni (prefs, time, shortcuts, initials) — Sonnet, Medium
- 5. Data NavBadge — Sonnet, Medium (needs 3.2)
- 6. Sidebar — Sonnet, Medium (needs 2.1, 3.2, 5.2)
- 7. Topbar + UserMenu — Sonnet, Medium (needs 2.1, 3.2, 4.2, 4.4)
- 8. CommandPalette — Sonnet, Medium (needs 3.2)
- 9. Wiring AppShell + test integrasi — Sonnet, Medium (needs 4.1, 4.3, 6.5, 7.6, 8.1)
- 10. Dokumentasi steering + progress — Haiku, Low (Haiku)
- 11. Checkpoint gate — Sonnet, Medium

**Opus-required**
- none — frontend only; tidak menyentuh money, auth, approval, reconciliation, migrasi, atau delete. Visibilitas menu dijaga Property 1 (task 3.3) dan route guard/backend tidak diubah.

## Tasks

- [x] 1. Font self-hosted dan konfigurasi env build
  - [x] 1.1 Pasang font dan muat di CSS
    - `pnpm add @fontsource-variable/plus-jakarta-sans@5.3.0 @fontsource-variable/jetbrains-mono@5.3.0 --save-exact`
    - `src/styles/index.css`: `@import` `wght.css` kedua font sebelum `tokens.css`; tidak ada URL eksternal
    - _Requirements: 14.2, 14.3_
  - [x] 1.2 `APP_ENV` dan `APP_VERSION`
    - `vite.config.ts`: `define.__APP_VERSION__` dari `package.json`; deklarasi di `src/vite-env.d.ts`
    - `src/lib/config/appEnv.ts`: `parseAppEnv`, `APP_ENV`, `APP_VERSION`, `ENV_LABEL`
    - `VITE_APP_ENV=dev` di `.env.example` + `.env.local`; build arg `VITE_APP_ENV` (default `prod`) di `Dockerfile`
    - _Requirements: 5.2, 5.3, 5.4_
  - [x]* 1.3 `src/lib/config/appEnv.test.ts`: nilai valid, kosong/tak dikenal → `dev`, label per env
    - _Requirements: 5.2, 5.3, 5.4_
  - _Model: Sonnet, Effort: Low — konfigurasi mekanis; satu-satunya risiko adalah default env di Docker._

- [x] 2. Token shell dan style global
  - [x] 2.1 Token dan utilitas
    - `src/styles/tokens.css`: semua `--shell-*` (tabel design Data Models) + blok `[data-sidebar-theme="dark"|"light"]`
    - `src/styles/index.css`: `@theme` `--font-sans`/`--font-mono` dengan fallback stack; keyframes `.shell-pop`; `.shell-scroll` (6px); `.shell-focus:focus-visible`; blok `prefers-reduced-motion` (ping, pulse, chevron, width, pop)
    - Tidak mengubah `--red-*` (D14)
    - _Requirements: 5.5, 14.1, 14.2, 15.1, 15.2, 15.3_
  - [x] 2.2 Property 10: tambah pasangan `--shell-*` ke `src/styles/contrast.property.test.ts` (kedua tema, Topbar, palette, dropdown, tooltip, badge, chip; 4.5:1 teks, 3:1 ikon)
    - _Requirements: 14.4_
  - _Model: Sonnet, Effort: Medium — banyak token per tema; kontras harus diverifikasi lewat test, bukan mata._

- [x] 3. NavConfig dan helper navigasi
  - [x] 3.1 Perubahan konfigurasi `src/lib/config/navigation.ts`
    - Tipe `NavGroup` + `"system"`, `NavPlacement`, `NavBadgeSource`, field `placement?`/`badge?`; `GROUP_LABELS.system = "Sistem"`; `SIDEBAR_GROUP_ORDER`
    - Ikon: Dashboard `LayoutGrid`, EOD `HeartPulse`, Kalender Libur `CalendarDays`, Pengaturan `Settings2` + `group: "system"`, `placement: "topbar"`
    - Badge: Persetujuan `approvals-inbox`, Request CIT `cit-requests`, EOD `eod-health`; tidak menambah item Vendor Request (D11)
    - Hapus import ikon yang jadi tidak terpakai (`noUnusedLocals`)
    - _Requirements: 2.2, 2.3, 13.1, 13.2, 13.3_
  - [x] 3.2 Helper `getVisibleNav`, `findActiveNavItem`, `filterPaletteItems`
    - `filterNavByRoles` tidak diubah; `getVisibleNav` membuang `disabled` (D13) dan grup kosong
    - Prefix segmen terpanjang, `/` hanya exact, normalisasi trailing slash
    - _Requirements: 2.1, 2.4, 2.7, 3.2, 7.2, 7.3, 7.4, 10.2, 12.3, 12.4, 12.9_
  - [x] 3.3 Test navigasi
    - `navigation.property.test.ts`: Property 1 (visibilitas per role, termasuk `null`), Property 2 (active match), Property 3 (filter palette)
    - `navigation.test.ts`: ikon per item, Pengaturan `system/topbar`, `/replenishment/vendor-requests/42` → Request CIT, `/settings/admin/holidays` → Kalender Libur, `/settings/admin/users` → Pengaturan
    - _Requirements: 2.1, 2.2, 2.3, 2.4, 2.7, 3.2, 7.2, 7.3, 7.4, 10.2, 12.3, 12.4, 12.9, 16.5_
  - _Model: Sonnet, Effort: Medium — helper kecil, tapi Property 1 menjaga visibilitas menu per role tidak melebar._

- [x] 4. Logika shell murni
  - [x] 4.1 `src/lib/shell/prefs.ts`: zustand `persist` key `crown.shell.v1`, `sanitizeShellPrefs`, `safeLocalStorage`, aksi `toggleRail`/`toggleTheme`/`toggleGroup`; default `{ rail: false, sidebarTheme: "dark", collapsedGroups: [] }`
    - _Requirements: 2.5, 4.6, 6.3_
  - [x] 4.2 `src/lib/shell/time.ts`: `CIT_CUTOFF_WIB`, `wibParts`, `formatOpsDate`, `todayWib`, `cutoffState`, `formatHms`, `useNow` (D10)
    - _Requirements: 9.2, 9.3, 9.4, 9.5, 9.6_
  - [x] 4.3 `src/lib/shell/shortcuts.ts`: `isEditableTarget`, `matchShellShortcut`, `useShellShortcuts`
    - _Requirements: 4.1, 4.2, 8.2_
  - [x] 4.4 `src/lib/utils/initials.ts`: `getInitials`
    - _Requirements: 11.1_
  - [x] 4.5 Property test: Property 5 (`time.property.test.ts`, termasuk proses dengan `TZ` non-WIB), Property 6 (`prefs.property.test.ts`, `fc.jsonValue()` + round-trip), Property 7 (`shortcuts.property.test.ts`), Property 8 (`initials.property.test.ts`)
    - _Requirements: 2.5, 4.1, 4.2, 4.6, 6.3, 8.2, 9.2, 9.3, 9.4, 9.5, 9.6, 11.1_
  - _Model: Sonnet, Effort: Medium — perhitungan zona waktu WIB dan sanitasi storage punya edge case, tapi tercakup property test._

- [x] 5. Data NavBadge
  - [x] 5.1 Opsi additive `options?: { enabled?: boolean; refetchInterval?: number | false }` pada `useInbox`, `useVendorRequests`, `useEodSummary`; default perilaku lama tidak berubah
    - _Requirements: 13.4_
  - [x] 5.2 `src/components/layout/useNavBadges.ts`: `NavBadge`, `badgeRequestStatuses` (tabel D8), query hanya `enabled` untuk item terlihat, EOD hanya ADMIN/ADMIN_PARAM (`counts.failed + max_retries_exhausted + late > 0`), `formatBadgeCount`, error/loading → tanpa badge
    - _Requirements: 13.1, 13.2, 13.3, 13.4, 13.5, 13.6_
  - [x] 5.3 `useNavBadges.test.tsx`: Property 9 (`fc.constantFrom(ROLES)` + hasil query acak, assert `enabled` dan nilai), mapping D8, sembunyi saat 0/error; test hook lama tetap lulus
    - _Requirements: 13.1, 13.2, 13.3, 13.4, 13.5, 13.6_
  - _Model: Sonnet, Effort: Medium — gating query per role harus tepat agar shell tidak menembak endpoint yang 403._

- [x] 6. Sidebar
  - [x] 6.1 `sidebar/SidebarBrand.tsx`: logo 36px, dot status + teks sr-only, rail = logo saja
    - _Requirements: 1.1, 1.2, 1.3_
  - [x] 6.2 `sidebar/SidebarNavGroup.tsx`: tombol label `aria-expanded`/`aria-controls`, chevron, animasi grid-rows, item aktif tetap tampil saat terlipat, rail = divider 24px tanpa lipat
    - _Requirements: 2.5, 2.6, 4.3_
  - [x] 6.3 `sidebar/SidebarNavItem.tsx`: `<Link>` 40px, state aktif (gradient, ikon tebal, dot 6px D4, `aria-current`), hover 200ms via kelas, badge angka/dot + nama aksesibel, rail (dot pojok, tooltip `role="tooltip"` + `aria-describedby` saat hover/focus)
    - _Requirements: 3.1, 3.3, 3.4, 3.5, 3.6, 3.7, 3.8, 4.3, 4.4_
  - [x] 6.4 `sidebar/SidebarFooter.tsx` + `EnvBadge`: border-top, eyebrow LINGKUNGAN, `{ENV_LABEL} · v{APP_VERSION}`, chip gold non-prod, tombol Ciutkan menu (`ChevronsLeft`/`ChevronsRight`, `<kbd>[</kbd>`, label aksesibel), EnvBadge tersembunyi saat rail
    - _Requirements: 4.3, 4.5, 5.1, 5.2, 5.3, 5.4_
  - [x] 6.5 `Sidebar.tsx`: susun subkomponen dari `VisibleNav.sidebar` + `SIDEBAR_GROUP_ORDER`, roving tabindex ArrowUp/Down, area nav `.shell-scroll`, `data-sidebar-theme`, lebar `--sb-w` 264/76 dengan transisi 300ms (off reduced-motion); hapus handler hover inline dan tooltip lama
    - _Requirements: 2.1, 3.8, 4.1, 5.5, 6.1_
  - [x] 6.6 `Sidebar.test.tsx`: brand + rail, urutan grup per role, Pengaturan tidak di sidebar, lipat grup + `aria-expanded` + item aktif tetap tampil, state aktif `aria-current`, badge + nama aksesibel, pulse dot, rail (divider, dot, tooltip hover/focus), footer prod vs non-prod, ikon collapse, keyboard + `href` link
    - _Requirements: 1.1, 1.2, 1.3, 2.1, 2.3, 2.5, 2.6, 3.3, 3.5, 3.6, 3.7, 3.8, 4.3, 4.4, 4.5, 5.1, 5.3, 5.4, 16.5_
  - _Model: Sonnet, Effort: Medium — komponen visual dalam pola yang sudah ada; aksesibilitas rail/tooltip perlu teliti._

- [x] 7. Topbar dan UserMenu
  - [x] 7.1 `topbar/Breadcrumb.tsx`: `nav aria-label="Breadcrumb"`, grup muted › item semibold `aria-current`, "Sistem › Pengaturan", kosong bila tanpa item aktif
    - _Requirements: 7.2, 7.3, 7.4_
  - [x] 7.2 `topbar/SearchTrigger.tsx`: `button` `aria-haspopup="dialog"` + `aria-keyshortcuts`, lebar 260/340px (2xl), kbd Ctrl K / ⌘ K
    - _Requirements: 8.1, 8.3_
  - [x] 7.3 `topbar/OpsStatusStrip.tsx`: `hidden lg:flex`, dot ping + `formatOpsDate`, `Clock3` + countdown `useNow(1000)` mono tabular, warning (tint + angka merah + teks sr-only), closed "ditutup", label statis tanpa `aria-live`
    - _Requirements: 9.1, 9.2, 9.3, 9.4, 9.5, 9.6, 9.7_
  - [x] 7.4 `topbar/TopbarIconButton.tsx` + tombol tema (`Sun`/`Moon`, label tema tujuan), Pengaturan (render hanya bila `visible.settings`, aktif tint 8% + `aria-current`), divider 1×32px; restyle tombol `NotificationBell.tsx` 40×40 + badge 18px (`--shell-accent`, ring) tanpa ubah logika panel
    - _Requirements: 6.2, 10.1, 10.2, 10.3, 10.4, 10.5_
  - [x] 7.5 `topbar/UserMenu.tsx`: avatar gradient + `getInitials`, nama + label role, chevron rotate; panel `role="menu"` w-60 rounded-2xl `.shell-pop`; info email + divider + "Keluar" (`--danger-fg` + ikon) → `logout()` (D7); fokus item pertama, ArrowUp/Down/Home/End, Escape + restore fokus, `pointerdown` di luar; pertahankan `data-testid` lama
    - _Requirements: 11.1, 11.2, 11.3, 11.4, 11.5, 11.6_
  - [x] 7.6 `Topbar.tsx`: sticky 64px, `--shell-topbar-bg` solid (D6), border bawah, `px-6`, susunan kiri→kanan. `Header.tsx` **belum** dihapus di sini (masih diimpor `AppShell` sampai 9.1)
    - _Requirements: 7.1, 10.1_
  - [x] 7.7 `Topbar.test.tsx` (menggantikan `Header.test.tsx` + `Header.property.test.tsx`) + update `NotificationBell.test.tsx`: breadcrumb (3 kasus), search trigger ARIA, OpsStatusStrip dengan fake timers (open/warning/closed, tanpa `aria-live`, tersembunyi < lg via kelas), toggle tema, Pengaturan per role + aktif, badge notifikasi tersembunyi saat 0 + panel tetap jalan, UserMenu (inisial, keyboard, Escape, klik luar, logout, warna danger)
    - _Requirements: 6.2, 7.1, 7.2, 7.3, 7.4, 8.1, 8.3, 9.1, 9.3, 9.4, 9.5, 9.7, 10.1, 10.2, 10.3, 10.4, 10.5, 11.1, 11.2, 11.3, 11.4, 11.5, 11.6, 16.5_
  - _Model: Sonnet, Effort: Medium — beberapa komponen kecil; pola WAI-ARIA menu button dan countdown perlu test teliti._

- [x] 8. CommandPalette
  - [x] 8.1 `CommandPalette.tsx` (default export untuk `React.lazy`): overlay blur + panel dialog 560px @12vh, input combobox 16px + kbd Esc, listbox dengan `aria-activedescendant`, baris (kotak ikon 32px aktif/normal, label, grup, `CornerDownLeft`), `nextIndex` melingkar + `scrollIntoView`, reset query/indeks, focus trap, Escape/klik overlay → `onClose`, Enter/klik → `onSelect`, empty state `Tidak ada menu "{q}"`
    - _Requirements: 12.1, 12.2, 12.3, 12.4, 12.5, 12.6, 12.7, 12.8, 12.9, 12.10_
  - [x] 8.2 `CommandPalette.test.tsx`: Property 4 (`nextIndex`), render ARIA dialog/combobox/listbox, filter + empty state, ArrowUp/Down + Enter, klik baris, Escape, klik overlay, Pengaturan grup "Sistem", reset saat dibuka ulang
    - _Requirements: 12.1, 12.2, 12.3, 12.5, 12.6, 12.7, 12.8, 12.9, 12.10_
  - _Model: Sonnet, Effort: Medium — pola combobox + listbox standar; fokus dan keyboard perlu dicakup test._

- [x] 9. Wiring AppShell dan test integrasi
  - [x] 9.1 `AppShell.tsx`: skip link, grid + `--sb-w`, `effectiveRail = prefs.rail || isNarrow` tanpa menulis prefs, `data-sidebar-theme`, `getVisibleNav` + `findActiveNavItem` + `useNavBadges` ke Sidebar/Topbar/palette, `useShellShortcuts` (nonaktif `[` saat palette terbuka), `paletteOpen` + `returnFocusRef` + `Suspense`/error boundary untuk `React.lazy(CommandPalette)`, `onSelect` → `router.navigate`, tema hanya di sidebar, `<main id="main-content">` `--shell-canvas` scroll sendiri, efek fokus ke `<main>` saat pathname berubah dipertahankan; props tetap `{ children, notifications? }`. Setelah `AppShell` memakai `Topbar`: hapus `Header.tsx`, update `src/components/layout/index.ts`, pastikan tidak ada import tersisa
    - _Requirements: 4.1, 4.2, 4.6, 4.7, 6.1, 6.3, 6.4, 8.2, 12.7, 12.8, 15.4, 16.1, 16.2, 16.3_
  - [x] 9.2 `AppShell.test.tsx`: fokus ke `<main>` saat route berubah (bukan render pertama), skip link, `[` toggle + diabaikan di input/textarea/select/contenteditable, Ctrl+K dan ⌘K membuka palette + fokus kembali setelah tutup, palette Enter menavigasi, rail paksa < 1024px tanpa mengubah prefs, persistensi rail/tema setelah remount, tema tidak memengaruhi Topbar/konten, main scroll terpisah; `_protected.hydration.test.tsx` dan `_protected.property.test.ts` tetap lulus
    - _Requirements: 4.1, 4.2, 4.6, 4.7, 6.1, 6.3, 6.4, 8.2, 12.7, 12.8, 15.4, 16.1, 16.2, 16.3, 16.5_
  - [x] 9.3 `accessibility.property.test.tsx`: update selector; setiap elemen interaktif shell punya nama aksesibel dan kelas focus-visible di kedua tema dan mode rail; animasi punya varian `motion-reduce`/`motion-safe`
    - _Requirements: 3.6, 15.1, 15.2, 15.3, 16.5_
  - _Model: Sonnet, Effort: Medium — integrasi banyak bagian; ingat lesson "spec task execution creates components in isolation", verifikasi semua komponen benar-benar dirender._

- [x] 10. Dokumentasi
  - `.kiro/steering/design.md`: deviasi dari design `## Documented deviation` (Sec 1/11 sidebar gelap + toggle, Sec 6 font, Sec 10 blur overlay + transisi width, Sec 2 `--shell-accent` D14)
  - `.kiro/steering/development-plan.md` X.1 status, `.claude/development-progress.md` entri baru
  - _Requirements: 14.5_
  - _Model: Haiku, Effort: Low — edit dokumen sesuai daftar yang sudah ditentukan._

- [x] 11. Checkpoint: gate kualitas
  - `pnpm --dir frontend/CompanyPortal-Vite run lint`, `run test:coverage` (84/80/80/85, jangan turunkan), `run build` (termasuk `check-bundle.mjs`)
  - Pastikan tidak ada literal hex/rgba di `src/components/layout/**` (grep) dan tidak ada URL font eksternal
  - Triase kegagalan; jangan skip test
  - _Requirements: 14.1, 14.3, 16.4, 16.5_
  - _Model: Sonnet, Effort: Medium — butuh judgement untuk triase kegagalan lint/coverage/bundle._

## Notes

- Test bertanda `*` (1.3) opsional; sisanya wajib karena memuat Correctness Property atau regresi.
- Pemeriksaan visual, pembaca layar, dan perilaku di browser nyata dilakukan manual oleh user (Golden Rule #10). Kepatuhan WCAG penuh butuh uji manual dengan assistive technology.
- Countdown cut-off memakai jam perangkat (tidak ada waktu server); jam perangkat yang salah menghasilkan countdown salah. Diterima untuk MVP.
- Mapping status badge Request CIT (D8) adalah asumsi; koreksi PO cukup mengubah `badgeRequestStatuses` + test 5.3.
- Route guard (`requireRoles`) dan backend `RequireRoles` tidak diubah. Revamp ini hanya tampilan.
- Antara wave 1 dan wave 6, test lama yang belum di-update (mis. `Sidebar.test.tsx` setelah 3.1/6.5, `AppShell.test.tsx` sebelum 9.2) boleh merah sementara. Tiap task cukup menjalankan test file miliknya (`pnpm vitest run <file>`) + `tsc -b tsconfig.app.json`; full suite + coverage hanya di gate 11.
- Penjadwalan: 4.1 menunggu 3.1 (`NavGroup`/`SIDEBAR_GROUP_ORDER` dipakai `sanitizeShellPrefs`), sehingga 4.5 ikut di wave 2. `Header.tsx` dihapus di 9.1, bukan 7.6, agar `AppShell` tidak kehilangan import di tengah jalan.

### Definition of Done
- Semua task wajib selesai, gate 11 hijau, Property 1–10 punya test yang lulus.
- `Header.tsx` terhapus, tidak ada import yang menunjuk ke sana.
- Dokumentasi task 10 diperbarui.

## Task Dependency Graph

```json
{
  "waves": [
    { "id": 0, "tasks": ["1.1", "1.2", "3.1", "4.2", "4.3", "4.4"] },
    { "id": 1, "tasks": ["1.3", "2.1", "3.2", "4.1", "5.1"] },
    { "id": 2, "tasks": ["2.2", "3.3", "4.5", "5.2", "6.1", "6.2", "7.1", "7.2", "7.3", "8.1"] },
    { "id": 3, "tasks": ["5.3", "6.3", "6.4", "7.4", "7.5", "8.2"] },
    { "id": 4, "tasks": ["6.5", "7.6"] },
    { "id": 5, "tasks": ["6.6", "7.7", "9.1"] },
    { "id": 6, "tasks": ["9.2", "9.3", "10"] }
  ]
}
```
