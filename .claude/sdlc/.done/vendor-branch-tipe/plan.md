# Plan: Perbaikan Menu "Branch" — `/settings/admin/vendors/{id}`

Status: **Item 1 selesai diimplementasikan (2026-09-23)** — user approve "pakai default saja" untuk semua pertanyaan §2.

## 0. Ruang Lingkup Sesi Ini

Item 1 (satu-satunya item yang diminta sejauh ini):

> Tambahkan field **tipe** pada Cabang (Branch), dengan value: `ATM`, `ATM & CASH`, `CASH`.

Item lain untuk menu Branch bisa ditambahkan ke plan ini nanti sebagai Item 2, 3, dst — belum ada yang diminta.

## 1. State Saat Ini (hasil baca kode, bukan asumsi)

- Tabel `vendor_branches` (baseline schema, `backend/migrations/001_baseline_schema.sql:1394`) kolom: `id, vendor_id, branch_code, branch_name, location_id, is_active, created_at, updated_at, region`. **Tidak ada kolom tipe/kategori.**
- Enum tipe yang diminta (`ATM` / `ATM & CASH` / `CASH`) sudah ada persis di tabel **`vendor_vaults.category`**, dengan constraint `CHECK (category IN ('ATM','CASH','ATM_CASH'))` (`backend/migrations/008_vendor_vault_types.sql:20`, lihat juga catatan CLAUDE.md Sec 3). Frontend sudah punya type `VendorVaultCategory = "ATM" | "CASH" | "ATM_CASH"` (`frontend/.../admin-vendors/types.ts:111`).
- Halaman `/settings/admin/vendors/2` menampilkan cabang lewat `BranchesPanel.tsx` → tabel kolom: Kode, Nama, Wilayah, Status, Aksi. Klik kode cabang → detail cabang (`VendorBranchDetailPage.tsx`, menampilkan Vaults/PICs/Paket).
- Create cabang: dialog `VendorBranchFormDialog.tsx` (field: branch_code, branch_name, region, location_id).
- Edit cabang: halaman terpisah `VendorBranchEditPage.tsx` (field sama minus branch_code yang immutable).
- Validasi form: `lib/vendorBranchFormSchema.ts` (zod).
- Alur mutasi: **maker-checker wajib** (Golden Rule #3, keputusan D1 di CLAUDE.md Sec 12). Tidak ada satupun handler yang menulis tabel langsung:
  `VendorBranchFormDialog/EditPage` → `useCreateVendorBranch/useUpdateVendorBranch` (hooks) → `admin_vendor_branch_handler.go` → `VendorBranchAdminService.Create/Update` (`backend/internal/service/vendor_branch_admin.go`) → `MasterDataChangeService.Submit` → baris `master_data_change_requests` + `approval_requests` → setelah disetujui, `VendorBranchApplier.Apply` (`backend/internal/service/masterdata_applier_vendor_branch.go`) menjalankan query sqlc (`backend/queries/vendor_branches_admin.sql`) di dalam transaksi apply.
- Query CRUD cabang: `vendor_branches_admin.sql` — `ListVendorBranchesAdmin`, `GetVendorBranchAdminByID`, `CreateVendorBranchAdmin`, `UpdateVendorBranchAdmin` (semua select kolom eksplisit, tidak pakai `SELECT *`).

## 2. Keputusan Desain

| # | Keputusan | Alasan |
|---|---|---|
| D1 | Kolom baru bernama `vendor_branches.category`, tipe `text`, `CHECK (category IN ('ATM','CASH','ATM_CASH'))`, **NOT NULL DEFAULT 'ATM'**. | Nama & value konsisten dengan `vendor_vaults.category` yang sudah ada (bukan bikin enum baru) — mengurangi konsep ganda. Default `'ATM'` supaya baris cabang existing (yang belum re-seed, lihat catatan "Needs re-seeding" di CLAUDE.md Sec 12) tidak NULL saat migrasi. |
| D2 | Reuse type FE `VendorVaultCategory` (`"ATM" \| "CASH" \| "ATM_CASH"`) untuk field baru ini juga — tidak bikin type baru. | DRY; satu tempat kalau nanti ada value ke-4. |
| D3 | Label tampilan: `ATM` → "ATM", `CASH` → "CASH", `ATM_CASH` → "ATM & Cash" (sesuai permintaan user persis: "ATM & CASH"). Mapping label taruh di satu helper `vendorVaultCategoryLabel()` kalau belum ada, dipakai di kolom tabel + form select. | Value DB tetap `ATM_CASH` (konsisten & aman dari spasi/karakter `&` di constraint), label tampilan yang sesuai permintaan user. |
| D4 | Field ini **wajib diisi saat create** (tidak ada opsi kosong), default pilihan `ATM` di form. Saat update, field bisa diubah bebas (tidak immutable, beda dengan `branch_code`). | Tidak ada requirement dari user untuk membatasi; ATM adalah kategori paling umum di data existing. |
| D5 | Mutasi tetap lewat maker-checker existing (tidak ada logic approval baru) — field ini hanya menambah kolom pada payload create/update `vendor_branch` yang sudah ada. | Konsisten dengan D1 Sec 12 CLAUDE.md; tidak menambah kompleksitas approval. |

**Pertanyaan terbuka** — user menjawab "pakai default saja" untuk semua (2026-09-23):
1. Nama kolom `category` ✅ dipakai apa adanya.
2. Baris existing di-default ke `'ATM'` ✅ (tidak backfill manual).
3. Tidak ditambah sebagai filter/search — hanya kolom tabel & field form (default, sesuai §4).

**Deviasi kecil dari D3 saat implementasi**: opsi `<select>` dan kolom tabel menampilkan value mentah (`ATM`, `CASH`, `ATM_CASH`), BUKAN label "ATM & Cash" yang disebut di D3 — mengikuti konvensi yang sudah ada di `VaultsPanel.tsx`/`VendorVaultFormDialog.tsx` untuk `vendor_vaults.category` (keduanya juga tidak punya label mapping, cuma render value mentah). Konsistensi dengan pola existing dinilai lebih penting daripada instruksi rinci di D3 yang belum ada presedennya di codebase. Kalau user mau label "ATM & Cash" tetap ditampilkan, perlu instruksi eksplisit (nanti berlaku juga untuk VaultsPanel supaya konsisten).

## 3. Rencana Implementasi (setelah keputusan di atas dikonfirmasi)

### Backend — DB & sqlc
- [x] `backend/migrations/013_vendor_branches_category.sql`: `ALTER TABLE vendor_branches ADD COLUMN category text NOT NULL DEFAULT 'ATM'`, lalu `ADD CONSTRAINT vendor_branches_category_chk CHECK (category IN ('ATM','CASH','ATM_CASH'))`. Comment kolom menjelaskan asal keputusan (link ke plan ini).
- [x] Update `backend/queries/vendor_branches_admin.sql`: tambah `category` ke kolom SELECT di `ListVendorBranchesAdmin`, `GetVendorBranchAdminByID`, `CreateVendorBranchAdmin` (INSERT + RETURNING), `UpdateVendorBranchAdmin` (SET + RETURNING). `CountVendorBranchesAdmin`/`FindVendorBranchAdminByCode` tidak perlu berubah (tidak select kolom data).
- [x] `sqlc generate` (pinned v1.31.1, lihat `backend/CLAUDE.md`) → cek `git diff --stat backend/internal/db/` hanya menyentuh file yang diharapkan.

### Backend — service & applier
- [x] `backend/internal/service/vendor_branch_admin.go`: tambah `Category string` ke `VendorBranchPayload` dan `VendorBranchUpdatePayload`; validasi di `Create`/`Update` — wajib salah satu dari `ATM`, `CASH`, `ATM_CASH` (cek dulu apakah ada validator category yang reusable di service package vault sebelum menulis baru).
- [x] `backend/internal/service/masterdata_applier_vendor_branch.go`: teruskan `Category` ke `CreateVendorBranchAdminParams`/`UpdateVendorBranchAdminParams`.
- [x] `backend/internal/handler/admin_vendor_branch_handler.go`: pastikan request struct create/update meng-decode field `category` dari JSON body (cek nama field JSON yang dipakai handler, biasanya langsung reuse `VendorBranchPayload`/`VendorBranchUpdatePayload`).

### Backend — tests
- [x] Test service/applier terkait (nama file persis perlu dicek saat eksekusi) — update fixture create/update payload untuk sertakan `category`, tambah test case validasi (reject value di luar enum, reject kosong).
- [x] `backend/internal/repository/no_hard_delete_test.go`: tidak berubah (tabel `vendor_branches` sudah ada di guarded list, kolom baru tidak butuh perubahan test ini).

### Frontend
- [x] `frontend/CompanyPortal-Vite/src/features/admin-vendors/types.ts`: tambah `category: VendorVaultCategory` ke `AdminVendorBranch`, `CreateVendorBranchPayload`, `UpdateVendorBranchPayload`.
- [x] `frontend/CompanyPortal-Vite/src/features/admin-vendors/lib/vendorBranchFormSchema.ts`: tambah `category: z.enum(["ATM", "CASH", "ATM_CASH"])`.
- [x] `components/VendorBranchFormDialog.tsx`: tambah `<select>` Tipe (default `ATM`), masukkan ke `EMPTY_VALUES` dan payload submit.
- [x] `components/VendorBranchEditPage.tsx`: tambah field yang sama, masukkan ke `toDefaultValues()` dan payload submit.
- [x] `components/BranchesPanel.tsx`: tambah kolom "Tipe" di tabel (pakai `Badge` atau teks, dengan label sesuai D3).
- [x] Cek apakah ada helper label kategori vault yang sudah ada di `VaultsPanel.tsx` / vendor vault form — reuse, jangan duplikasi mapping label.

### Tests frontend
- [x] `__tests__/VendorBranchDetailPage.test.tsx` dan test form dialog/edit page terkait (perlu dicek file mana yang ada saat eksekusi) — update fixture + assert kolom/field baru muncul.

### Verifikasi
- [x] `go build ./...` + `go test ./...` di `backend/`.
- [x] `npm run build` / `tsc --noEmit` + test runner FE di `CompanyPortal-Vite`.
- [x] Manual browser check **dilakukan oleh user** (Golden Rule #10) — bukan oleh AI.

## 5. Seed Data Tipe & Pembersihan Region (2026-09-23)

User memberi daftar master `<Vendor> <Kota>	<Tipe>` (99 baris) dan minta nama vendor dihapus dari field `region`.

Cross-check: daftar yang di-paste user identik dengan data yang sudah dipakai untuk seed `vendor_vaults.category` di migrasi 008 (96/99 exact match by branch_name; 3 sisanya cuma beda nama karena branch itu di-rename ke nama lebih lengkap di step 2 migrasi 008 — value kategorinya sama). Jadi `migrations/014_vendor_branches_category_seed.sql` **tidak** re-parse teks yang di-paste user, melainkan copy `vendor_vaults.category` ke `vendor_branches.category` lewat join `vendor_branch_id` (satu vault per cabang) — lebih aman daripada matching by name (rawan typo/beda spasi) dan sudah diverifikasi identik dengan sumber sama.

Region: strip prefix nama vendor dari `vendor_branches.region` (regex `^<vendor_name>\s+`), hanya menyentuh baris yang benar-benar diawali nama vendornya sendiri.

Migrasi 013 dan 014 sudah **diterapkan ke dev DB** (`localhost:5432/cms`, sebelumnya migrasi 013 belum ter-apply dari sesi lalu). Hasil verifikasi live:
- Distribusi kategori: 44 `ATM_CASH` / 43 `CASH` / 12 `ATM` (dari 99 cabang FLM) + 320 sisanya (ROH + 8 cabang tanpa vault) tetap default `ATM` — cocok persis dengan catatan CLAUDE.md Sec 3 ("44 ATM_CASH / 43 CASH / 12 ATM").
- Tidak ada lagi `region` yang diawali nama vendornya sendiri (query verifikasi 0 baris).

## 4. Di Luar Lingkup Item 1 Ini

- Tidak menyentuh `vendor_vaults.category` yang sudah ada.
- Tidak menambah filter/search berdasarkan tipe di `BranchesPanel` kecuali dikonfirmasi di §2 Q3.
- Tidak mengubah alur maker-checker/approval.
- Item-item lain "perbaikan menu Branch" di luar field tipe ini belum didefinisikan — tunggu instruksi lanjutan dari user untuk ditambahkan sebagai Item 2, dst di plan ini.
