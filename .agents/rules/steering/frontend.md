# Steering: Frontend Architecture, UI/UX Brand & OpenAPI Sync

> **Scope**: Dual SPA isolation (`CompanyPortal-Vite` & `VendorPortal-Vite`), OpenAPI 3.0.3 contract sync, brand palette ("Merah Sirih" vs "Merah Menyala"), `/api/eod/*` proxy routing, 4 mandatory UI states, dan browser verification rule.  
> **Source of Truth**: Disinkronkan dari `.claude/CLAUDE.md` (Sec 2, 2a, 3, 8, 11, 13, 14) & [frontend-ui.md](file:///c:/Users/RB%20Yudha%20Rangga/OneDrive/Documents/Development/CMS2/.claude/rules/frontend-ui.md).  
> **Referenced by**: [GEMINI.md](file:///c:/Users/RB%20Yudha%20Rangga/OneDrive/Documents/Development/CMS2/.agents/rules/GEMINI.md)

---

## 1. Dual Portal Isolation (Sec 3 & Sec 5)

CMS2 memiliki dua Single-Page Application (SPA) React + Vite (disajikan via Nginx di Docker) yang **wajib dijaga terpisah**:

| Aplikasi | Path | Pengguna & Autentikasi | Port Dev | Identitas Brand (Sec 13) |
|---|---|---|---|---|
| **CompanyPortal-Vite** | `frontend/CompanyPortal-Vite/` | Internal Bank (LDAP bind → JWT) | `:5173` | **"Merah Sirih"** (aksen merah $\le 10\%$) |
| **VendorPortal-Vite** | `frontend/VendorPortal-Vite/` | Vendor Eksternal (Kredensial lokal DB → JWT) | `:5174` | **"Merah Menyala"** (brand-forward) |

*   **Perintah Utama (pnpm)**:
    ```bash
    pnpm --dir frontend/CompanyPortal-Vite run dev|build|test|lint     # dev :5173
    pnpm --dir frontend/VendorPortal-Vite  run dev|build|test|lint     # dev :5174
    ```
*   **EOD Monitoring Proxy Routing (Sec 14 S6)**:
    *   Fitur `features/eod-monitoring` di `CompanyPortal-Vite` memanggil `/api/eod/*` (bukan route Go).
    *   `vite.config.ts` (saat dev) dan `frontend/CompanyPortal-Vite/nginx.conf` (saat prod/Docker) memetakan `/api/eod/<path>` → `eod_retry_scheduler` `:8091/<path>` (prefix `/api/eod` dihapus).
    *   Layanan `service_dsr_etl` (`:8090`) **tidak pernah** diekspos ke browser.

---

## 2. OpenAPI 3.0.3 Contract Synchronization (Sec 2, 2a, 11)

*   **Spesifikasi Hand-Written**: `backend/api/openapi.yaml` (menggunakan `kin-openapi` **v0.151.0** di Go test dan `openapi-typescript` **7.13.0** di `CompanyPortal-Vite`).
*   **Alur Wajib Saat Menambah/Mengubah Endpoint**:
    1. Setiap perubahan pada endpoint di modul yang sudah tercakup di `backend/api/openapi.yaml` (atau saat memulai modul baru) wajib diperbarui di `openapi.yaml`.
    2. Jalankan tes kontrak di backend (memastikan spec valid, route cocok, dan shape response sesuai):
       ```bash
       cd backend && go test ./internal/handler -run OpenAPI
       ```
    3. Regenerasi tipe TypeScript di frontend dan **komit hasilnya** (tes Vitest akan gagal jika `schema.d.ts` stale):
       ```bash
       pnpm --dir frontend/CompanyPortal-Vite run gen:api
       ```
       *(Menghasilkan `frontend/CompanyPortal-Vite/src/lib/api/schema.d.ts`)*.

---

## 3. UI/UX, Brand Rules & Formatting (Sec 13)

> Aturan lengkap palet dan komponen ada di [.claude/rules/frontend-ui.md](file:///c:/Users/RB%20Yudha%20Rangga/OneDrive/Documents/Development/CMS2/.claude/rules/frontend-ui.md) dan skill `react-ui-ux-craft`. Aturan brand di bawah ini selalu menang di atas saran generik.

### 3.1 Design System Tokens & Color Strategy (Kiro Steering Sync)
*   **Mode Tampilan**: **Light mode only** (tanpa dark mode, tanpa theme toggle). Ruang warna menggunakan **OKLCH**.
*   **Strategi Proporsi Visual (60 / 30 / 10)**:
    *   **60%** Neutral surfaces (`--n-0` cards/inputs, `--n-50` page background).
    *   **30%** Borders, typography, inactive elements (`--n-200` hingga `--n-900`). Seluruh neutral memiliki sedikit rona brand warm (hue 29, chroma 0.003–0.009). Jangan pernah gunakan `#000` atau `#fff` murni.
    *   **10%** Brand red accents, focus halos, dan tombol aksi primer/CTA.
*   **THE HARD RULE: Brand Red (Hue 29) vs Error Red (Hue 12)**:
    *   Aplikasi perbankan ber-brand merah berisiko tinggi membingungkan pengguna jika warna aksi primer sama dengan bahaya/destruktif.
    *   **Brand Red = Hue 29** (`--red-500`, `oklch(0.552 0.205 29)`): Khusus aksi primer (Submit, Save, Approve), navigasi, fokus halo, dan identitas brand.
    *   **Error / Danger Red = Hue 12** (`--danger-500`, `oklch(0.545 0.205 12)`): Khusus aksi destruktif (Hapus, Reject, Void), status gagal, dan error alert.
    *   **Larangan Mutlak**: Dilarang menggunakan `--red-500` untuk tombol delete/reject, dan dilarang menggunakan `--danger-500` untuk tombol CTA.
    *   **Aksesibilitas Ganda**: Status error/bahaya **TIDAK PERNAH** hanya disampaikan melalui warna saja — wajib dipasangkan dengan teks label dan ikon pendamping.

### 3.2 Typography & Spacing Scale (Kiro Steering Sync)
*   **Self-Hosted Fonts (No External URLs)**:
    *   `CompanyPortal-Vite` menggunakan font self-hosted via npm packages `@fontsource-variable/plus-jakarta-sans` (antarmuka umum) dan `@fontsource-variable/jetbrains-mono` (angka finansial, kode, tabular data).
    *   Dilarang keras memuat font via tag `<link>` eksternal / Google Fonts CDN (wajib lolos environment tertutup/on-premise bank).
*   **Sistem Skala Spacing (4pt Base Scale)**:
    *   Gunakan kelipatan 4pt: 4, 8, 12, 16, 24, 32, 48, 72px (`--space-xs` hingga `--space-3xl`).
    *   Gunakan properti `gap` pada Flexbox/Grid ketimbang margin acak.
    *   Border radius: 4px (`sm` - inputs/badges), 6px (`md` - buttons), 10px (`lg` - cards/tables).

### 3.3 Format Finansial & 4 Status Wajib UI
*   **Format Uang & Angka (Wajib di Seluruh Layar & Ekspor)**:
    *   Gunakan `tabular-nums` pada setiap nominal uang dan angka tabel.
    *   Gunakan font monospace (`JetBrains Mono`) untuk sel nilai uang dan rata kanan (`text-right`).
    *   Tampilkan kode mata uang (**IDR**) secara eksplisit.
    *   **Dilarang keras** melakukan kalkulasi nominal uang dengan `float` JavaScript yang berisiko kehilangan presisi.
*   **Aksesibilitas Status**: **Jangan pernah** mengodekan status hanya dengan warna semata (selalu sertakan teks label dan/atau ikon pendamping).
*   **4 Status Wajib UI (4 Mandatory UI States)**:
    1. **Loading**: Skeleton / indikator muat yang proporsional.
    2. **Empty**: Pesan jelas saat data kosong beserta panduan tindakan.
    3. **Error**: Pesan kesalahan yang jelas dengan opsi coba lagi (*retry*).
    4. **202 Maker-Checker Pending**: Menampilkan feedback jelas bahwa perubahan telah diajukan ke antrean persetujuan Checker (`202 Accepted`).

---

## 4. Frontend Testing & Browser Verification (Golden Rule #10 & Sec 8)

*   **Component Tests**: Wajib menyertakan pengujian komponen untuk alur-alur kritis di **KEDUA** frontend (`CompanyPortal-Vite` dan `VendorPortal-Vite`).
*   **Batas Verifikasi Browser Manual (Golden Rule #10)**:
    *   Pengecekan/pengujian manual di browser (mengklik UI, memverifikasi alur secara visual end-to-end) adalah tugas **USER**, bukan AI.
    *   Agen AI bertanggung jawab mengirimkan kode + menjalankan pemeriksaan otomatis (`pnpm test`, `pnpm lint`, typecheck, `pnpm build`).
    *   Setelah semua pemeriksaan otomatis hijau, nyatakan secara lugas bahwa verifikasi manual di browser berstatus **`outstanding (user)`** — jangan pernah mencoba mengklaim verifikasi browser telah selesai.
