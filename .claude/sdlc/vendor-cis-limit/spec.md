# Spec: Limit CIS per area vault vendor + escrow vault + BM + Detail inquiry

Author: Claude, dari `intent.md` (accepted 2026-11-06, user). Status: **accepted 2026-11-06** (user: "Terima, buat plan"); S1–S7 disetujui 2026-11-06.
Stage: 2 Design. Stage berikut: `plan.md` (setelah spec diterima).

## ⚠ Flagged (Sec 4 #7)
- **Skema**: migrasi baru `038_vendor_cis_limit.sql` (nomor bebas berikutnya). **File belum dibuat.** Dibuat di Build,
  diterapkan ke dev lewat `go run ./cmd/migrate up` dengan OK user.
- **Uang**: `vendor_branches.cis_limit_amount` adalah limit asuransi (CIS). Disimpan `numeric`, di API sebagai string
  desimal (QC F7), tidak pernah float.
- **Master data maker-checker**: Limit CIS dan escrow vault ikut alur `MasterDataChangeService.Submit` → `Applier` yang
  sudah ada (GR#3 dipertahankan, A3). Peran PIC `bm` ikut alur peran PIC yang sudah ada, yaitu **deviasi GR#3 D6e**
  (langsung berlaku + audit dalam tx, vendor-party-scope). Tidak ada deviasi baru.
- Tidak ada delete, auth, rekonsiliasi, atau journal.

## Temuan verifikasi kode (2026-11-06)
1. `vendor_branches` sudah punya `category` (ATM / CASH / ATM_CASH, migrasi 013) = Service Type per area. Ini lebih pas
   dengan grain inquiry (per area) daripada `vendor_vaults.category` (O2). → keputusan **S1**.
2. Update cabang dan vault adalah **full replace** (`UpdateVendorBranchAdmin`, `UpdateVendorVaultAdmin`: semua kolom ditulis
   dari payload). Kolom baru harus selalu dikirim form; payload tanpa kolom itu akan mengosongkannya. CSV memakai pola
   `LegacyHeaders` + `LegacyFill: KeepCurrentCell` (seperti `region_code`/`category` cabang).
3. `vendor_pics.roles` CHECK `⊆ {replenish, vault, cashcount}` (032); Go `picRoleCodes` + FE `picRoles.ts` meniru daftar itu.
   Notifikasi pihak vendor memilih PIC dengan `pic_role = ANY(p.roles)` untuk `replenish`/`vault`, jadi menambah `bm`
   tidak mengubah penerima notifikasi. Direktur (`is_director`) wajib `roles = '{}'` (037), jadi direktur tidak bisa BM.
4. `atms.escrow_account` (text, tanpa validasi format) sudah ada per ATM. Fitur ini tidak mengubahnya.
5. Ekspor master data (`backend-exq`) dan impor CSV memakai kontrak bersama `pkg/csvexport/entities.go`; kolom baru di
   header ikut dua-duanya.
6. Endpoint `/api/v1/admin/vendors/...` tidak ada di `backend/api/openapi.yaml` (pilot hanya vendor-requests), jadi tidak
   ada kontrak OpenAPI yang wajib diubah.

## Functional requirements

### FR1 — Limit CIS per area vault vendor (A1, A2, A3, intent #1)
1.1 Setiap `vendor_branches` (area vault vendor) punya **Limit CIS** opsional: `cis_limit_amount` + `cis_limit_currency`.
    "Limit Asuransi" (xlsx) dan "Limit CIS" (FSD) adalah nilai yang sama; label layar: **"Limit CIS"** (S3).
1.2 Validasi (service, saat submit **dan** apply): kosong (NULL) = belum diisi; bila diisi → desimal > 0, maksimal 2 angka
    desimal, muat di `numeric(20,2)`. Mata uang hanya `IDR` untuk sekarang (S4), disimpan eksplisit.
1.3 Isi / ubah / kosongkan limit = op `update` (atau ikut `create`) cabang lewat maker-checker yang sudah ada → 202; baris
    berubah hanya saat approve, dalam tx yang sama dengan `audit_logs` (perilaku Applier sekarang).
1.4 Form cabang CompanyPortal (buat + ubah) punya field Limit CIS (IDR, `tabular-nums`, rata kanan, pemisah ribuan) dan
    selalu mengirim nilai sekarang (temuan 2). Detail cabang menampilkan Limit CIS ("—" bila kosong).
1.5 CSV `vendor-branches` (impor + ekspor) mendapat kolom `cis_limit_amount` (desimal titik, kosong = NULL). File tanpa
    kolom ini tetap valid dan **mempertahankan** nilai sekarang (`LegacyHeaders` + `KeepCurrentCell`).

### FR2 — Nomor escrow per vault (A2, Q7 ditinjau, intent #4)
2.1 `vendor_vaults.escrow_account` opsional: tepat **12 digit angka** (setelah `btrim`; kosong = NULL). **Tidak unik**
    (FSD cash count: satu escrow bisa dikelola beberapa vendor).
2.2 Isi / ubah / kosongkan = op `create`/`update` vault lewat maker-checker yang sudah ada (202, apply + audit dalam tx).
    Validasi di submit **dan** apply.
2.3 Form vault CompanyPortal punya field "No Rekening Escrow" dan selalu mengirim nilai sekarang. Panel vault
    menampilkannya.
2.4 CSV `vendor-vaults` mendapat kolom `escrow_account`; file lama tanpa kolom itu mempertahankan nilai sekarang. Sel
    diekspor sebagai teks (nol di depan tidak hilang; tetap lewat `CSVSafe`).

### FR3 — Peran PIC `bm` / Branch Manager (intent #2)
3.1 `vendor_pics.roles` boleh berisi `bm` (CHECK diperluas jadi `⊆ {replenish, vault, cashcount, bm}`). Default PIC baru
    tetap tiga peran lama (tanpa `bm`).
3.2 Peran `bm` diatur lewat jalur peran PIC yang ada: layar `PUT …/admin/vendors/{vendorID}/pics/{id}/roles` (langsung +
    audit dalam tx, D6e), create PIC dan impor CSV PIC (maker-checker). Go `picRoleCodes` + FE `picRoles.ts` + label
    "BM (Branch Manager)".
3.3 `bm` hanya bermakna untuk PIC yang terikat cabang (`vendor_branch_id` terisi). PIC tingkat vendor dengan `bm` ditolak
    (422, pesan jelas) di ketiga jalur 3.2 (S5).
3.4 `bm` tidak memengaruhi notifikasi, login, atau cakupan pihak vendor.

### FR4 — Layar Detail inquiry Limit CIS (intent #3, #4, #7)
4.1 Layar baru CompanyPortal "Inquiry Limit CIS": cari per **Nama Perusahaan** (pilih vendor FLM aktif; kosong = semua),
    paging server-side. Satu baris = satu area vault vendor (`vendor_branches`) aktif dari vendor `kind = 'FLM_VENDOR'`.
4.2 Kolom:

| Kolom layar | Sumber |
|---|---|
| Nama Perusahaan | `vendors.name` |
| Area Vault Vendor | `vendor_branches.branch_code` + `branch_name` |
| Region | `vendor_branches.region` |
| Location | lokasi `vendor_branches.location_id` (nama lokasi) |
| Nama BM / Contact BM | PIC aktif cabang itu dengan peran `bm`: nama / telepon (lebih dari satu → semua, dipisah koma; tidak ada → "—") |
| Service Type | `vendor_branches.category` (S1) |
| No Rek ATM | `escrow_account` vault aktif cabang itu dengan kategori `ATM` atau `ATM_CASH` (S2) |
| No Rek Cash | `escrow_account` vault aktif dengan kategori `CASH` atau `ATM_CASH` (S2) |
| Limit CIS | `cis_limit_amount` + mata uang ("—" bila kosong) |

4.3 Read-only, dibaca dari **replica**. Tidak ada kolom Posisi Saldo / Status, filter Status Asuransi, atau tombol
    generate report (ditunda, intent #5–#7).
4.4 Akses: role `ADMIN`, `ADMIN_PARAM` (grup route `masterDataAdmin`, dicek di route + service) (S6). Layar ini
    berupa tab di halaman admin vendor yang ada, tanpa entri `menu_features` baru (S7).

### FR5 — Audit
5.1 Tidak ada jalur tulis baru: limit + escrow ditulis Applier cabang/vault (audit dalam tx), peran `bm` ditulis jalur
    peran PIC (audit `vendor_pic_roles_updated` dalam tx). Inquiry read-only, tanpa audit.

## Non-functional
- Uang: `numeric(20,2)`, string desimal di API, `tabular-nums` + IDR eksplisit di layar (Sec 6, Sec 13).
- Inquiry: satu query dengan join + agregasi (`string_agg`), tanpa N+1; ≤ 3 s p95 (dashboard NFR), paging maksimal 100.
- Tes: Go unit (validasi limit/escrow/bm), integration (apply + legacy CSV keep-current + CHECK), handler (RBAC 403,
  202), FE component (form field, inquiry), coverage `internal/*` ≥ 80 %.

## API (flat JSON, handler `backend/` yang ada)
| Perubahan | Endpoint |
|---|---|
| Payload create/update + DTO baca cabang: `cis_limit_amount` (string desimal \| null), `cis_limit_currency` (`"IDR"`) | `…/admin/vendors/{vendorID}/branches` (POST, PUT `/{id}`, GET) |
| Payload create/update + DTO vault: `escrow_account` (string 12 digit \| null) | `…/admin/vendors/{vendorID}/vaults` |
| `roles` menerima `"bm"` | `PUT …/admin/vendors/{vendorID}/pics/{id}/roles`, create PIC |
| **Baru**: `GET /api/v1/admin/cis-limit-inquiry?vendor_id=&page=&page_size=` → `{items:[…], total}` (replica) | grup `masterDataAdmin` |

Error validasi: 422 `ValidationError` (pola sekarang) dengan field `cis_limit_amount` / `escrow_account` / `roles`.

## Data model (migrasi `038_vendor_cis_limit.sql`, file belum dibuat)
```sql
ALTER TABLE vendor_branches
  ADD COLUMN cis_limit_amount   numeric(20,2) NULL CHECK (cis_limit_amount > 0),
  ADD COLUMN cis_limit_currency character(3) NOT NULL DEFAULT 'IDR';
ALTER TABLE vendor_vaults
  ADD COLUMN escrow_account text NULL CHECK (escrow_account ~ '^[0-9]{12}$');
ALTER TABLE vendor_pics
  DROP CONSTRAINT vendor_pics_roles_chk,
  ADD CONSTRAINT vendor_pics_roles_chk CHECK (roles <@ ARRAY['replenish','vault','cashcount','bm']::text[]);
-- + COMMENT ON COLUMN untuk ketiga kolom baru
```
Additive (kolom NULL / default konstan = tanpa rewrite tabel). Baris lama: limit NULL, escrow NULL, roles tidak berubah.
Rollback: drop dua kolom cabang + kolom vault; CHECK roles dikembalikan setelah `bm` dihapus dari baris yang memakainya.
Setelah apply: update CLAUDE.md Sec 3 (pindahkan dari "Still NOT migrated") + `docs/data-map.md`.

## Keputusan
### Dari intent (accepted 2026-11-06)
#1 Limit Asuransi = Limit CIS · #2 BM = PIC cabang dengan peran `bm` · #3 Location = lokasi area · #4 No Rek = escrow vault
ATM / CASH · #5–#7 saldo, status, parameter, filter status, generate report ditunda · Q7 escrow tidak unik.

### Keputusan spec (disetujui user 2026-11-06: "Ikuti semua saran")
- **S1** Service Type = `vendor_branches.category` (bukan kategori vault, menggantikan O2 untuk inquiry), karena grain
  inquiry per area dan kolomnya sudah ada (temuan 1).
- **S2** Vault `ATM_CASH` dengan escrow muncul di kolom No Rek ATM **dan** No Rek Cash (A2 menganjurkan vault terpisah,
  tapi data lama bisa `ATM_CASH`).
- **S3** Label layar "Limit CIS" (bukan "Limit Asuransi").
- **S4** Mata uang limit hanya IDR: kolom `cis_limit_currency` disimpan (Sec 6) tapi tidak bisa diubah dari layar/CSV.
- **S5** Peran `bm` ditolak untuk PIC tingkat vendor.
- **S6** Akses inquiry `ADMIN` + `ADMIN_PARAM` ("Ops team" FSD). Role lain menyusul bila PO minta.
- **S7** Inquiry = tab di halaman admin vendor, tanpa menu/permission baru.

## Risiko
- Full-replace update (temuan 2): klien lama yang tidak mengirim kolom baru akan mengosongkan limit/escrow. Mitigasi: form
  selalu mengirim; CSV legacy keep-current; tes integration untuk keduanya.
- Change request cabang/vault yang **pending sebelum** rilis tidak punya kolom baru dalam payload → saat approve akan
  menulis NULL. Mitigasi di plan: Applier memperlakukan kunci yang **tidak ada** di payload sebagai "pertahankan nilai
  sekarang" (beda dengan `null` eksplisit = kosongkan).
- Data dev: limit + escrow semua NULL setelah migrasi; inquiry akan banyak "—" sampai Ops mengisi.

## Out of scope
- Posisi Saldo SIBS, status Covered / Over, "Rules Parameterize" + Menu Input Parameter, filter Status Asuransi, generate
  report CSV inquiry → fase monitoring setelah ingest saldo escrow (A6).
- Ingest saldo escrow (`escrow_batch_rows`), rekonsiliasi escrow.
- Perubahan `atms.escrow_account`.
- PKS vendor (`.kiro/specs/vendor-pks/`).
