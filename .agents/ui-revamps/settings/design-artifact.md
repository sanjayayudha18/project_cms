# Desain Artefak UI Revamp: Settings Hub (/settings)

> **Layar**: `http://localhost:3001/settings` (`CompanyPortal-Vite`)  
> **Komponen Target**: `frontend/CompanyPortal-Vite/src/features/rbac-settings/components/SettingsHubPage.tsx`  
> **Dokumen Terkait**: [Audit](audit.md) · [Brain Artifact Interaktif](file:///C:/Users/RB%20Yudha%20Rangga/.gemini/antigravity/brain/621000c0-409e-48f2-aa61-22fe6d637dea/settings-revamp-plan.md) · [UI Spec](ui-spec.md) · [Tasks](tasks.md)  
> **Status**: draft  

---

## 1. Rangkuman Artefak Desain (Brain Artifact Reference)

Artefak visual interaktif utama telah di-generate di folder Antigravity Brain:
* 📋 **Dokumen Rencana Desain**: [`settings-revamp-plan.md`](file:///C:/Users/RB%20Yudha%20Rangga/.gemini/antigravity/brain/621000c0-409e-48f2-aa61-22fe6d637dea/settings-revamp-plan.md)
* 🎨 **Pratinjau Visual Prototype Interaktif**: [`settings-preview.html`](file:///C:/Users/RB%20Yudha%20Rangga/.gemini/antigravity/brain/621000c0-409e-48f2-aa61-22fe6d637dea/settings-preview.html)
* **Metadata**: `RequestFeedback: true`, `UserFacing: true`.
* **Fitur Prototipe**: Simulasi peran (ADMIN vs ATM-SPV vs ADMIN_PARAM), filter pencarian instan dengan shortcut ⌘K, cross-tab suggestion banner saat kata kunci ada di tab lain, dynamic count badges, dan arsitektur visual baru spotlight card *"Hierarki Pengguna"*.

---

## 2. Intisari Keputusan Desain & Visual Layout

### 2.1 Proporsi Visual 60 / 30 / 10 ("Merah Sirih")
- **60% Neutral Surfaces**: `--n-0` untuk kartu modul, container tab aktif, dan input bar; `--n-50` untuk kanvas halaman.
- **30% Borders & Typography**: `--n-200` border pembatas halus, `--n-600` deskripsi modul, `--n-900` judul seksi & kartu.
- **10% Brand Red Accent**: `--red-500` aksen penanda kartu featured & icon hover; `--red-600` untuk tombol CTA primer ("Kelola hierarki"). Tidak ada penggunaan `--danger-500` di halaman pengaturan.

### 2.2 Wireframe Layout Baru
1. **Header & Context Zone**:
   - `PageHeader` standar CMS2 dengan eyebrow `"Administrasi"` dan judul `"Pengaturan"`.
   - Action: Role context badge (`ADMIN · Akses Penuh` / `ATM-SPV · Khusus PKS & Vendor`) + tombol Bantuan.
2. **Category Tabs & Search Controls**:
   - Segmented tab control pills dengan count chips: `Akses & persetujuan (6)`, `Data master (8)`, `Export & Import (2)`.
   - Input pencarian cerdas dengan tombol clear `(X)` dan deteksi kecocokan lintas-tab (*Cross-Tab Search Prompt*).
3. **Architectural Spotlight: "Hierarki Pengguna"**:
   - Format spotlight card beraksen border kiri `--red-500`, badge *"FONDASI OTORISASI"*, micro-skema approver $\rightarrow$ maker, dan CTA link *"Kelola hierarki"* ke `/settings/rbac/users`.
4. **Module Cards Grid (2-Kolom)**:
   - Menggantikan baris vertikal kaku `divide-y` menjadi grid 2-kolom responsif (`grid-cols-1 md:grid-cols-2 gap-4`).
   - Icon container dengan smooth hover background tint (`--red-50`), teks semi-bold, dan panah `ArrowUpRight` dengan micro-interaction translation.
5. **Governance Footer**:
   - Informasi kepatuhan maker-checker dan audit log atomik dengan icon `Lock`.

---

## 3. Human Gate & Status Persetujuan
- [x] Artefak desain dibuat di Antigravity Brain directory.
- [ ] User mereview dan menyetujui rancangan desain (`Proceed` / `accepted`).
