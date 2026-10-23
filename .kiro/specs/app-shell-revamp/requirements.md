# Requirements Document

## Introduction

### Masalah
Shell aplikasi internal (`frontend/CompanyPortal-Vite`) masih versi awal: brand hanya teks "CMS", tidak ada breadcrumb, kolom pencarian di header tidak berfungsi, tidak ada badge pada menu, tidak ada indikator lingkungan, dan menu user hanya berisi email + logout. PO meminta tampilan shell baru "CROWN – Cash Management System" (sidebar kiri, topbar, command palette Ctrl+K) sesuai brief visual tanggal 2026-10-23.

### Kondisi kode saat ini (dicek 2026-10-23)
- Shell: `src/components/layout/AppShell.tsx` (grid, sidebar 256/64px, collapse via `useState` tanpa persistensi, auto-collapse < 1024px, fokus pindah ke `<main>` saat ganti route), `Sidebar.tsx` (grup + roving tabindex, aktif = `currentPath === href` exact match, indikator `border-left` 3px, tooltip ad-hoc saat collapsed), `Header.tsx` (input pencarian dekoratif, dropdown user tanpa navigasi keyboard/Escape). Dipasang di `src/routes/_protected.tsx` dengan slot `<NotificationBell/>`.
- Data menu: `src/lib/config/navigation.ts` (`NAV_CONFIG`, `NavGroup`, `GROUP_LABELS`, `filterNavByRoles`). Tidak ada field badge. "Pengaturan" masih di grup Umum. Item Tagihan/Perhitungan Kas berstatus `disabled`.
- Route guard per halaman memakai `requireRoles([...])` yang terpisah dari `NAV_CONFIG`.
- Token: `src/styles/tokens.css` (OKLCH, `--red-*`, `--n-*`, semantic, spacing, radius, shadow). Font = system stack. Tidak ada web font.
- CSP (`security-headers.conf`): `style-src 'self'; font-src 'self' data:` sehingga Google Fonts CDN **akan diblokir**.
- Sumber data badge yang ada: `useUnreadCount()` (notifikasi, poll 60 dtk), `useInbox()` (approval inbox, tanpa endpoint count), `useVendorRequests()` (`pagination.total_count`), `useEodSummary()` / `useEodStatus()` (EOD, ADMIN/ADMIN_PARAM). Tidak ada indikator versi/lingkungan.
- Tidak ada route ganti password, profil, atau bantuan di CompanyPortal. User internal login via LDAP.
- `AuthUser` = `{id, username, fullName, email, role, isKaryawan, vendorId}`; tidak ada kantor/region.

### Ruang lingkup
- Rebuild `AppShell`, `Sidebar`, `Header` (menjadi Topbar) dan tambah `CommandPalette` di CompanyPortal-Vite (TypeScript), memakai router + `NAV_CONFIG` yang sudah ada.
- Tambah field badge/indikator pada `NAV_CONFIG`, pindahkan Pengaturan ke topbar.
- Token warna/font shell baru di `tokens.css` (OKLCH).
- Persistensi preferensi shell (rail, tema sidebar, grup terlipat) di `localStorage`.

### Tidak termasuk
- Konten halaman, VendorPortal-Vite, perubahan backend/endpoint baru, perubahan RBAC/route guard, halaman Profil/Ganti password/Bantuan (lihat Q7).
- Penggantian `NotificationBell` panel (hanya tampilan tombolnya yang disesuaikan).

### Area sensitif (project-context Sec 3 rule 7)
Tidak menyentuh money, journal, reconciliation, migrasi, atau delete. Menyentuh tampilan menu per role: visibilitas menu **tidak boleh** melebar dari `filterNavByRoles` saat ini (keamanan tetap di route guard + backend).

### Dependency & scope
Item tercatat di `development-plan.md` sebagai cross-cutting **X.1 CompanyPortal app-shell revamp** (Q1 ✅). Frontend only, tidak menunggu Phase 2.

## Glossary

- **AppShell**: komponen frame layout (`src/components/layout/AppShell.tsx`) yang menyusun Sidebar, Topbar, dan area `<main>`.
- **Sidebar**: navigasi kiri; mode **Expanded** (264px) atau **Rail** (76px).
- **Topbar**: bar atas 64px (pengganti `Header.tsx`).
- **CommandPalette**: modal pencarian menu yang dibuka dengan Ctrl+K / ⌘K atau tombol pencarian.
- **NavConfig**: `NAV_CONFIG` + `GROUP_LABELS` + `filterNavByRoles` di `src/lib/config/navigation.ts`.
- **ActiveNavItem**: item NavConfig yang cocok dengan pathname saat ini (prefix terpanjang).
- **NavBadge**: angka atau dot pada item menu, diisi dari hook data yang sudah ada.
- **ShellPrefs**: preferensi UI shell yang dipersistenkan di `localStorage` (rail, tema sidebar, grup terlipat).
- **SidebarTheme**: varian warna sidebar, `dark` (oxblood) atau `light`.
- **OpsStatusStrip**: kapsul status operasional di Topbar (tanggal ops + hitung mundur cut-off CIT).
- **UserMenu**: tombol avatar + dropdown di Topbar.
- **EnvBadge**: kartu lingkungan + versi di footer Sidebar.
- **WIB**: zona waktu Asia/Jakarta.

## Requirements

### Requirement 1: Header brand Sidebar

**User Story:** Sebagai pengguna internal, saya ingin identitas CROWN terlihat jelas di sidebar, agar saya tahu sedang berada di aplikasi yang benar.

#### Acceptance Criteria

1. THE Sidebar SHALL menampilkan header setinggi 64px berisi kotak logo 36px (radius 12px, warna CIMB Red, huruf "C" bold), teks "CROWN" (15px, extrabold) dan "Cash Management" (12px, muted).
2. THE Sidebar SHALL menampilkan dot status kecil berwarna success di pojok kanan atas logo, dengan teks alternatif untuk pembaca layar.
3. WHILE Sidebar dalam mode Rail, THE Sidebar SHALL hanya menampilkan kotak logo di tengah header.

### Requirement 2: Grup dan item navigasi

**User Story:** Sebagai pengguna internal, saya ingin menu dikelompokkan dan bisa dilipat per grup, agar saya cepat menemukan fitur.

#### Acceptance Criteria

1. THE Sidebar SHALL merender grup berurutan Umum, Replenish, CIT, Monitoring, Peramalan, dengan item dari NavConfig yang lolos `filterNavByRoles` untuk role user.
2. THE NavConfig SHALL memuat item dengan ikon: Dashboard (LayoutGrid), Cash Flow Monitoring (Activity), Persetujuan (ClipboardCheck), Rekomendasi CIT (Search), Request CIT (ClipboardList), Jadwal CIT (Truck), Penetapan Vault (Landmark), ATM Portal (Monitor), EOD Monitoring (HeartPulse), Log Audit (ScrollText), DMAA Forecast (BarChart3), Kalender Libur (CalendarDays).
3. THE Sidebar SHALL tidak menampilkan item Pengaturan; item Pengaturan tetap ada di NavConfig dengan penanda lokasi topbar agar tersedia untuk Topbar dan CommandPalette.
4. THE Sidebar SHALL tidak menampilkan grup yang tidak memiliki item terlihat dan tidak menampilkan item `disabled`.
5. WHEN user mengklik label grup, THE Sidebar SHALL melipat atau membuka grup tersebut dengan chevron berputar dan animasi tinggi berbasis grid-rows, serta menyetel `aria-expanded` pada tombol label.
6. IF grup yang berisi ActiveNavItem dilipat, THEN THE Sidebar SHALL tetap menampilkan ActiveNavItem tersebut.
7. THE Sidebar SHALL mempertahankan visibilitas item per role persis sama dengan `filterNavByRoles` saat ini (tidak ada item yang menjadi terlihat untuk role yang sebelumnya tidak melihatnya).

### Requirement 3: State item menu

**User Story:** Sebagai pengguna internal, saya ingin melihat menu mana yang aktif dan indikator jumlah pekerjaan, agar saya tahu posisi dan prioritas saya.

#### Acceptance Criteria

1. THE Sidebar SHALL merender item setinggi 40px, radius 12px, ikon 18px, jarak ikon-label 12px, teks 14px.
2. THE AppShell SHALL menentukan ActiveNavItem dari pathname router dengan pencocokan prefix segmen terpanjang (mis. `/replenishment/vendor-requests/42` → Request CIT, bukan Jadwal CIT; `/` hanya cocok persis).
3. WHILE sebuah item adalah ActiveNavItem, THE Sidebar SHALL menampilkan latar gradient merah horizontal, teks kuat, ikon merah terang dengan stroke lebih tebal, indikator aktif sesuai keputusan Q4, dan `aria-current="page"`.
4. WHEN pointer berada di atas item non-aktif, THE Sidebar SHALL menampilkan latar hover dengan transisi 200ms.
5. WHERE item memiliki NavBadge angka, THE Sidebar SHALL menampilkan angka berfont mono 12px pada pill netral, dan pill merah dengan teks terang ketika item aktif.
6. WHERE item memiliki NavBadge dot, THE Sidebar SHALL menampilkan dot berwarna warning dengan animasi pulse, dan pulse dinonaktifkan WHEN `prefers-reduced-motion: reduce`.
7. THE Sidebar SHALL menyertakan nilai NavBadge dalam nama aksesibel item (mis. "Persetujuan, 12 menunggu").
8. THE Sidebar SHALL mempertahankan navigasi keyboard ArrowUp/ArrowDown/Enter antar item yang ada saat ini dan memakai elemen link sehingga klik tengah/Ctrl+klik membuka tab baru.

### Requirement 4: Mode Rail (sidebar diciutkan)

**User Story:** Sebagai operator yang bekerja seharian, saya ingin menciutkan sidebar, agar area kerja lebih lebar.

#### Acceptance Criteria

1. WHEN user menekan tombol "Ciutkan menu" atau tombol `[`, THE Sidebar SHALL berpindah antara Expanded (264px) dan Rail (76px) dengan transisi 300ms ease-out.
2. IF fokus berada di input, textarea, select, atau elemen contenteditable, THEN THE AppShell SHALL mengabaikan tombol `[`.
3. WHILE Sidebar dalam mode Rail, THE Sidebar SHALL menampilkan ikon di tengah, mengganti label grup dengan garis pemisah 24px, mengganti NavBadge dengan dot merah kecil di pojok ikon, dan menyembunyikan EnvBadge.
4. WHILE Sidebar dalam mode Rail, WHEN pointer atau fokus keyboard berada pada item, THE Sidebar SHALL menampilkan tooltip gelap berisi nama menu (dan nilai badge bila ada) dengan `role="tooltip"` yang terhubung via `aria-describedby`.
5. THE Sidebar SHALL menampilkan ikon ChevronsLeft pada mode Expanded dan ChevronsRight pada mode Rail, dengan hint kbd `[` dan label aksesibel yang sesuai.
6. THE ShellPrefs SHALL menyimpan mode Rail pilihan user dan memulihkannya saat aplikasi dimuat ulang.
7. WHILE lebar viewport < 1024px, THE AppShell SHALL memakai mode Rail tanpa menimpa preferensi tersimpan user untuk viewport lebar.

### Requirement 5: Footer Sidebar dan indikator lingkungan

**User Story:** Sebagai pengguna UAT, saya ingin tahu lingkungan dan versi aplikasi, agar saya tidak tertukar dengan produksi.

#### Acceptance Criteria

1. THE Sidebar SHALL menampilkan footer dengan garis atas, berisi EnvBadge dan tombol "Ciutkan menu".
2. THE EnvBadge SHALL menampilkan label "LINGKUNGAN", nama lingkungan, dan versi aplikasi yang diambil dari konfigurasi build (bukan teks hardcode).
3. WHERE lingkungan bukan produksi, THE EnvBadge SHALL menampilkan chip lingkungan berwarna warning (teks + latar 20%).
4. WHERE lingkungan adalah produksi, THE EnvBadge SHALL tidak menampilkan chip peringatan (sesuai keputusan Q9).
5. THE Sidebar SHALL memakai scrollbar tipis 6px pada area navigasi.

### Requirement 6: Tema Sidebar

**User Story:** Sebagai pengguna, saya ingin memilih sidebar gelap atau terang, agar nyaman dipakai di berbagai kondisi cahaya.

#### Acceptance Criteria

1. THE Sidebar SHALL mendukung SidebarTheme `dark` (latar oxblood dengan radial-gradient halus di kiri atas) dan `light` (latar terang, teks netral, aktif gradient merah 10%).
2. WHEN user mengklik toggle tema di Topbar, THE AppShell SHALL mengganti SidebarTheme dan ikon toggle berganti Sun/Moon dengan label aksesibel yang menyebut tema tujuan.
3. THE ShellPrefs SHALL menyimpan SidebarTheme dan memulihkannya saat aplikasi dimuat ulang; nilai default adalah sesuai keputusan Q5.
4. THE AppShell SHALL membatasi tema hanya pada Sidebar; Topbar, CommandPalette, dan area konten tetap light.

### Requirement 7: Breadcrumb Topbar

**User Story:** Sebagai pengguna, saya ingin melihat posisi saya di menu, agar orientasi jelas.

#### Acceptance Criteria

1. THE Topbar SHALL setinggi 64px, sticky, dengan latar terang, border bawah netral, dan padding horizontal 24px.
2. THE Topbar SHALL menampilkan breadcrumb "{Label grup} › {Label ActiveNavItem}" dalam elemen `nav` berlabel "Breadcrumb", grup berwarna muted dan item aktif semibold dengan `aria-current="page"`.
3. WHEN ActiveNavItem adalah Pengaturan, THE Topbar SHALL menampilkan breadcrumb "Sistem › Pengaturan".
4. IF tidak ada ActiveNavItem untuk pathname saat ini, THEN THE Topbar SHALL tidak menampilkan breadcrumb.

### Requirement 8: Pemicu pencarian

**User Story:** Sebagai pengguna, saya ingin membuka pencarian menu dari keyboard, agar berpindah fitur lebih cepat.

#### Acceptance Criteria

1. THE Topbar SHALL menampilkan tombol pencarian (lebar 260px, 340px pada breakpoint 2xl) dengan ikon Search, teks "Cari menu atau fitur…", dan kbd "Ctrl K" (atau "⌘ K" pada macOS).
2. WHEN user mengklik tombol pencarian atau menekan Ctrl+K / ⌘K di mana pun dalam shell, THE AppShell SHALL membuka CommandPalette dan mencegah aksi default browser.
3. THE tombol pencarian SHALL merupakan elemen `button` dengan `aria-haspopup="dialog"`, bukan input dekoratif.

### Requirement 9: Strip status operasional

**User Story:** Sebagai operator CIT, saya ingin melihat tanggal operasional dan sisa waktu menuju cut-off, agar saya tidak terlambat mengirim request.

#### Acceptance Criteria

1. WHILE lebar viewport ≥ 1024px, THE OpsStatusStrip SHALL tampil sebagai kapsul ber-border dengan dua segmen; di bawah 1024px disembunyikan.
2. THE OpsStatusStrip SHALL menampilkan dot success dengan animasi ping dan tanggal operasional hari ini di WIB berformat `id-ID` (mis. "20 Okt").
3. THE OpsStatusStrip SHALL menampilkan ikon Clock3, label "Cut-off CIT", dan hitung mundur `HH:MM:SS` (font mono, `tabular-nums`) menuju jam cut-off WIB sesuai keputusan Q10, diperbarui tiap detik.
4. WHILE sisa waktu < 1 jam, THE OpsStatusStrip SHALL mewarnai segmen cut-off dengan latar merah tipis dan angka merah, disertai ikon/teks sehingga status tidak hanya dibedakan warna.
5. WHEN waktu sudah melewati cut-off, THE OpsStatusStrip SHALL menampilkan "ditutup" menggantikan hitung mundur.
6. THE OpsStatusStrip SHALL menghitung waktu berdasarkan WIB, bukan zona waktu perangkat.
7. THE OpsStatusStrip SHALL tidak mengumumkan setiap detik ke pembaca layar (tanpa `aria-live` pada angka berjalan) dan menyediakan label aksesibel statis.

### Requirement 10: Grup ikon Topbar

**User Story:** Sebagai pengguna, saya ingin akses cepat ke tema, pengaturan, dan notifikasi.

#### Acceptance Criteria

1. THE Topbar SHALL menampilkan tombol ikon 40x40 radius 12px: toggle tema Sidebar, Pengaturan (Settings2), dan Notifikasi (Bell), lalu divider vertikal 1px x 32px.
2. WHERE role user lolos `filterNavByRoles` untuk item Pengaturan, THE Topbar SHALL menampilkan tombol Pengaturan yang menavigasi ke `/settings`; selain itu tombol tidak dirender.
3. WHILE ActiveNavItem adalah Pengaturan, THE Topbar SHALL menampilkan tombol Pengaturan dengan latar merah 8%, ikon merah, dan `aria-current="page"`.
4. THE Topbar SHALL menampilkan tombol Notifikasi lewat `NotificationBell` yang sudah ada, dengan badge angka bulat 18px (latar CIMB Red, ring terang) dari `useUnreadCount()`, tanpa badge bila nol.
5. THE Topbar SHALL mempertahankan perilaku panel `NotificationBell` yang ada (buka, tandai baca, Escape menutup).

### Requirement 11: Menu user

**User Story:** Sebagai pengguna, saya ingin melihat identitas saya dan bisa keluar dari sistem.

#### Acceptance Criteria

1. THE UserMenu SHALL menampilkan avatar bulat 36px bergradient merah ke maroon berisi inisial dari `fullName` (maksimal 2 huruf), nama (14px semibold), dan baris kedua berisi label role (12px muted), serta ChevronDown yang berputar saat terbuka.
2. WHEN user mengklik tombol UserMenu, THE UserMenu SHALL membuka dropdown lebar 240px, radius 16px, shadow lembut, dengan animasi fade + scale dari kanan atas 150ms.
3. THE UserMenu SHALL berisi item sesuai keputusan Q7, divider, lalu "Keluar" (LogOut) yang memanggil `logout` dari auth store.
4. WHEN user mengklik di luar dropdown atau menekan Escape, THE UserMenu SHALL menutup dropdown dan mengembalikan fokus ke tombol pemicu (khusus Escape).
5. THE UserMenu SHALL mendukung navigasi ArrowUp/ArrowDown/Home/End antar item dengan pola WAI-ARIA menu button.
6. THE item "Keluar" SHALL memakai warna danger (hue 12) disertai ikon, bukan brand red.

### Requirement 12: Command Palette

**User Story:** Sebagai pengguna, saya ingin mengetik nama fitur dan langsung berpindah ke sana.

#### Acceptance Criteria

1. WHEN CommandPalette dibuka, THE CommandPalette SHALL menampilkan overlay oxblood 40% dan modal lebar 560px pada 12vh dari atas, radius 16px, dengan `role="dialog"`, `aria-modal="true"`, fokus otomatis pada input, dan fokus terkunci di dalam modal.
2. THE CommandPalette SHALL menampilkan input 16px berplaceholder "Ke menu mana? Ketik nama fitur…" dan kbd "Esc".
3. THE CommandPalette SHALL menampilkan semua item NavConfig yang lolos `filterNavByRoles` dan tidak `disabled`, termasuk Pengaturan dengan grup "Sistem".
4. WHEN user mengetik, THE CommandPalette SHALL memfilter item berdasarkan label atau label grup, tidak peka huruf besar/kecil dan tanpa memperhitungkan spasi di awal/akhir.
5. THE CommandPalette SHALL menampilkan tiap baris berisi kotak ikon 32px, label, label grup di kanan, dan ikon CornerDownLeft pada baris terpilih; ActiveNavItem bertanda kotak ikon merah.
6. WHEN user menekan ArrowUp/ArrowDown, THE CommandPalette SHALL memindahkan baris terpilih secara melingkar dan menggulir baris tersebut ke area terlihat, memakai pola combobox + listbox (`aria-activedescendant`).
7. WHEN user menekan Enter atau mengklik baris, THE CommandPalette SHALL menavigasi ke `href` item tersebut lewat router dan menutup palette.
8. WHEN user menekan Escape atau mengklik overlay, THE CommandPalette SHALL menutup dan mengembalikan fokus ke elemen yang memicu pembukaan.
9. IF tidak ada item yang cocok, THEN THE CommandPalette SHALL menampilkan teks `Tidak ada menu "{q}"`.
10. WHEN CommandPalette dibuka ulang, THE CommandPalette SHALL mengosongkan query dan memilih baris pertama.

### Requirement 13: Sumber data NavBadge

**User Story:** Sebagai pengguna, saya ingin angka badge mencerminkan data nyata, bukan angka contoh.

#### Acceptance Criteria

1. THE AppShell SHALL mengisi NavBadge Persetujuan dari jumlah item `useInbox()` dan menyembunyikan badge bila nol atau gagal dimuat.
2. THE AppShell SHALL mengisi NavBadge Request CIT dari `pagination.total_count` `useVendorRequests()` dengan filter status sesuai keputusan Q8.
3. WHERE role user adalah ADMIN atau ADMIN_PARAM, THE AppShell SHALL menampilkan dot EOD Monitoring hanya ketika `useEodSummary()` untuk tanggal hari ini (WIB) melaporkan file gagal, retry habis, atau terlambat sesuai keputusan Q8.
4. THE AppShell SHALL hanya memanggil hook data badge untuk item yang terlihat oleh role user (tidak ada request ke endpoint yang akan menolak role tersebut).
5. THE AppShell SHALL memformat angka badge dengan `formatBadgeCount` yang sudah ada (mis. "99+").
6. IF query badge gagal, THEN THE Sidebar SHALL menyembunyikan badge tanpa menampilkan error di shell.

### Requirement 14: Token visual dan tipografi

**User Story:** Sebagai tim frontend, saya ingin warna dan font shell mengikuti design system, agar konsisten dan mudah dirawat.

#### Acceptance Criteria

1. THE AppShell SHALL mengambil semua warna shell (oxblood, maroon, gold, teks sidebar, border topbar, latar konten) dari CSS custom properties OKLCH di `tokens.css`, tanpa literal hex/rgba di komponen.
2. THE AppShell SHALL memakai font sesuai keputusan Q2, dengan ukuran teks minimum 12px dan teks menu 14px.
3. THE AppShell SHALL tidak memuat aset dari domain luar (sesuai CSP `font-src 'self' data:` dan `style-src 'self'`).
4. THE AppShell SHALL memakai warna teks ink (bukan hitam/putih murni) dan setiap pasangan teks/latar di shell (kedua SidebarTheme, Topbar, palette, dropdown, badge) SHALL memenuhi kontras WCAG AA (4.5:1 teks normal, 3:1 teks ≥ 18.66px bold dan ikon non-dekoratif).
5. THE `design.md` steering SHALL diperbarui untuk setiap deviasi yang disetujui pada Q2–Q6.

### Requirement 15: Motion dan aksesibilitas

**User Story:** Sebagai pengguna dengan kebutuhan aksesibilitas, saya ingin shell tetap bisa dipakai dengan keyboard dan pembaca layar.

#### Acceptance Criteria

1. THE AppShell SHALL memakai transisi hover 200ms dan fade-in dropdown/palette 150ms dengan easing `cubic-bezier(0.22, 1, 0.36, 1)`.
2. WHEN `prefers-reduced-motion: reduce` aktif, THE AppShell SHALL menonaktifkan animasi ping/pulse, rotasi chevron beranimasi, dan transisi lebar sidebar.
3. THE AppShell SHALL menampilkan focus ring `:focus-visible` pada semua elemen interaktif shell di kedua SidebarTheme.
4. THE AppShell SHALL menyediakan skip link "Lewati ke konten utama" yang menuju `#main-content`.

### Requirement 16: Regresi

**User Story:** Sebagai tim, saya ingin revamp shell tidak merusak perilaku yang sudah ada.

#### Acceptance Criteria

1. THE AppShell SHALL tetap memindahkan fokus ke `<main id="main-content">` saat pathname berubah (kecuali render pertama).
2. THE AppShell SHALL tetap dirender oleh `_protected.tsx` dengan perilaku auth guard dan `requireRoles` yang tidak berubah.
3. THE area konten utama SHALL berlatar warna konten shell dan tetap dapat di-scroll sendiri tanpa menggulir Sidebar/Topbar.
4. THE CompanyPortal-Vite SHALL lolos `pnpm run lint`, `pnpm run test`, threshold coverage (84/80/80/85), dan bundle budget `scripts/check-bundle.mjs`.
5. THE test layout yang ada (`AppShell.test.tsx`, `Sidebar.test.tsx`, `Header*.test.tsx`, `accessibility.property.test.tsx`, `navigation*.test.ts`, `contrast.property.test.ts`, `NotificationBell.test.tsx`) SHALL diperbarui dan tetap lulus.

## Open Questions

Status: Q1 ✅ DIPUTUSKAN PO 2026-10-23 (masuk plan sebagai X.1). Q2–Q13 ✅ mengikuti saran (PO meminta lanjut ke design 2026-10-23 tanpa perubahan). Keputusan dirujuk di design sebagai D1–D13 (Dn = Qn). Rincian status Request CIT pada Q8 ditetapkan di design (D8) dan tetap bisa dikoreksi PO saat review design.

| # | Pertanyaan | Saran (= keputusan) |
| --- | --- | --- |
| Q1 | Item ini di luar Phase 2 (`development-plan.md` scope guard). Boleh ditambahkan ke plan sebagai item UI terpisah? | ✅ Masuk plan sebagai X.1, jalan paralel Phase 2. |
| Q2 | Brief meminta Plus Jakarta Sans + JetBrains Mono via Google Fonts. CSP memblokir Google Fonts, dan `design.md` menetapkan system stack. Pilih: (a) self-host via `@fontsource-variable/plus-jakarta-sans` + `@fontsource-variable/jetbrains-mono` (dependency baru, Golden Rule #1, + update `design.md`), atau (b) tetap system stack + `ui-monospace`. | (a) self-host, versi dipin exact, subset latin saja agar bundle budget aman. |
| Q3 | Brief memakai hex/rgba. Design system mewajibkan OKLCH. | Konversi semua ke token OKLCH baru (`--shell-*`) di `tokens.css`; visual setara. |
| Q4 | Indikator aktif "garis vertikal 3px di tepi kiri" melanggar ban side-stripe (`design.md` Sec 10). | Ganti dengan gradient + ikon merah + dot 6px sebelum label. Alternatif: izinkan sebagai deviasi terdokumentasi. |
| Q5 | Sidebar gelap oxblood + toggle tema bertentangan dengan "light mode only, no theme toggle". | Izinkan sebagai deviasi terbatas pada Sidebar (konten tetap light); default `dark`. Update `design.md`. |
| Q6 | `backdrop-blur` Topbar/overlay = glassmorphism (bukan default) dan transisi `width` = animasi properti layout (dilarang). | Topbar solid `--n-0` tanpa blur; overlay palette blur boleh (modal sementara). Transisi lebar sidebar diizinkan sebagai deviasi (sudah ada di kode sekarang), dimatikan saat reduced-motion. |
| Q7 | Profil saya, Ganti password, Bantuan belum punya halaman. User internal login LDAP sehingga ganti password tidak berlaku. Baris kedua "Admin · Kantor Pusat": kantor tidak ada di `AuthUser`. | Tampilkan hanya "Keluar" + baris info (email) sampai halaman ada; baris kedua = label role saja. Item lain ditambahkan di spec terpisah. |
| Q8 | Status apa yang dihitung untuk badge Request CIT? Kapan dot EOD menyala? | Request CIT = status yang menunggu aksi role user (mis. `pending_spv_approval` untuk ATM-SPV); EOD dot = run hari ini `failed` atau belum `success`. Perlu konfirmasi daftar status. |
| Q9 | Sumber nama lingkungan dan versi ("UAT · v4.0.2")? `package.json` versi `0.1.0`, belum ada `VITE_APP_ENV`. | Tambah `VITE_APP_ENV` (dev/uat/prod) + inject `version` dari `package.json` lewat `define` di `vite.config.ts`. Chip warning tampil selain prod. |
| Q10 | Cut-off CIT 15:00 belum tercatat sebagai aturan bisnis. Berlaku setiap hari termasuk libur? Tanggal ops = tanggal kalender WIB? | Konstanta `CIT_CUTOFF = "15:00"` WIB di config frontend, berlaku tiap hari, tanggal ops = tanggal kalender WIB. Aturan libur menyusul. |
| Q11 | Brief memuat menu "Vendor Request (FileStack)" terpisah dari "Request CIT", padahal keduanya route `/replenishment/vendor-requests`. | Tidak menambah item baru; Request CIT tetap satu-satunya. Konfirmasi bila maksudnya halaman lain. |
| Q12 | Brief meminta satu file `App.jsx` + state menu aktif di App (default "vendor"). Proyek memakai TypeScript + TanStack Router. | Implementasi TSX di komponen layout yang ada; menu aktif diturunkan dari route (Req 3.2), bukan state lokal. |
| Q13 | Grup Tagihan dan Perhitungan Kas (item `disabled`, "Soon") tidak ada di brief. | Disembunyikan dari Sidebar dan palette sampai fiturnya dibangun (Req 2.4). |
