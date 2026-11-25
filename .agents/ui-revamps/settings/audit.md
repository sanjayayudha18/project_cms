# UI Revamp Audit: Settings Hub (/settings)

> **Layar**: `http://localhost:3001/settings` (`CompanyPortal-Vite`)  
> **Komponen Target**: `frontend/CompanyPortal-Vite/src/features/rbac-settings/components/SettingsHubPage.tsx`  
> **Dokumen Terkait**: [Desain Artefak](design-artifact.md) · [UI Spec](ui-spec.md) · [Tasks](tasks.md)  
> **Status**: accepted (Tahap 1 selesai)  

---

## 1. Scope & Guardrails
- **Strictly `/settings` only**: Hanya merombak halaman hub `/settings` (`SettingsHubPage.tsx`) dan sub-komponen presentasional langsungnya.
- **Zero Regression**: Seluruh 21 automated test Vitest di `src/features/rbac-settings/__tests__/` wajib tetap hijau.
- **Brand Identity**: Mengikuti standar *Merah Sirih* (Light Mode OKLCH, Brand Red hue 29 $\le 10\%$, rasio visual 60/30/10).

---

## 2. Temuan Audit (Visual & UX Pain Points)
1. **Layout Kaku**: Lebar `max-w-[960px]` dengan kartu deret satu kolom vertikal di dalam border box `divide-y`, terkesan seperti direktori link statis biasa.
2. **Featured Card Tidak Efisien**: Kartu *Hierarki Pengguna* memakan ruang vertikal ~280px dengan diagram dummy pohon ("Supervisor" $\rightarrow$ "Pengguna A, Pengguna B") yang tidak interaktif.
3. **Pencarian Terisolasi & Dead-End**: Pencarian hanya memfilter tab aktif. Jika mencari "ATM" saat berada di tab "Akses", muncul "Tidak ada pengaturan yang cocok" tanpa memberitahu bahwa modul ATM ada di tab "Data master".
4. **Navigasi Tab Tanpa Metrik**: Tab tidak menampilkan jumlah modul yang tersedia di bawahnya.
5. **Konteks Peran Belum Eksplisit**: Pengguna tidak melihat indikator peran aktif mereka di halaman pengaturan untuk memahami visibilitas modul yang dibatasi hak akses.
