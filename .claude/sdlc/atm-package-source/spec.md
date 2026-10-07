# Spec: Pilihan sumber paket pada Kelolaan ATM (paket khusus cabang vs paket vendor-wide)

Author: Claude, dari `intent.md` (accepted 2026-10-07). Status: **accepted 2026-10-07** (user: "diterima, setuju endpoint baru, lanjut plan.md"; ekspor/impor CSV dua mode dipilih user 2026-10-07 → FR12).
Stage: 2 Design. Stage berikut: `plan.md`.

## Sec 4 #7 flags (wajib dibaca PO)
- **Skema / migrasi**: `atm_vendor_packages` berubah (kolom baru, `vendor_package_id` nullable, CHECK, unique parsial). Nomor migrasi = nomor
  bebas berikutnya saat plan (`022` sudah dicadangkan `vendor-pks-cis-limit`) → kemungkinan `023`.
- **Maker-checker**: payload `atm_assignment` berubah; `ATMAssignmentApplier` dan service ikut berubah.
- **Uang/harga**: tidak ada perubahan nilai harga. Validasi tarif hanya *membaca* `vendor_package_prices`.
- **Endpoint baru** (satu, read-only) diajukan di bawah — perlu persetujuan.

## Ruang lingkup
Dialog Kelolaan ATM bisa menetapkan paket dari dua sumber, cabang vendor pengelola selalu dipilih:

| Mode (`source`) | Paket dari | Disimpan di `atm_vendor_packages` |
|---|---|---|
| `branch` (default, perilaku sekarang) | baris `vendor_packages_branch` | `vendor_package_id` |
| `vendor` (baru) | label paket vendor (`vendor_package_prices.package`, mis. "PAKET 4") | `vendor_id`, `vendor_branch_id`, `package` |

## Functional requirements
- **FR1** Dialog menampilkan Vendor → Cabang (wajib, tanpa "Semua cabang") → Jenis paket (*Khusus cabang* / *Seluruh vendor*) → Paket.
  Mode `branch`: opsi paket = `vendor_packages_branch` aktif milik cabang terpilih (sumber data sekarang). Mode `vendor`: opsi = label paket
  vendor terpilih yang punya tarif aktif untuk `price_machine_group`/`price_class` ATM (FR9).
- **FR2** Ganti vendor/cabang/jenis mengosongkan pilihan paket. Prefill dari kelolaan aktif ATM (termasuk `source`).
- **FR3** `POST /api/v1/admin/atms/{atmId}/assignments` menerima `source` (`branch`|`vendor`; kosong = `branch`).
  `branch`: `vendor_package_id` wajib, field vendor-wide dilarang. `vendor`: `vendor_id`, `vendor_branch_id`, `package` wajib, `vendor_package_id` dilarang.
  Pelanggaran → `ValidationError` per field (kode HTTP mengikuti handler yang ada).
- **FR4** Periode tetap otomatis (tanpa tanggal): mulai pada tanggal disetujui (WIB), terbuka; periode berjalan ditutup H-1 / dinonaktifkan bila mulai hari yang sama
  (`applyAutoAssignment` apa adanya, untuk kedua mode). Jalur dengan tanggal eksplisit tetap berlaku untuk kedua mode.
- **FR5** Maker-checker, audit, RBAC (`ADMIN`/`ADMIN_PARAM` di route **dan** service) tidak berubah; respons 202 + pending.
- **FR6** Daftar kelolaan (`GET .../assignments`) mengembalikan, per baris: `source`, `package_code` (mode `branch` = `vendor_packages_branch.package_code`;
  mode `vendor` = label `package`), `vendor_id`, `vendor_branch_id`, dan field yang sudah ada. Tabel di dialog menampilkan badge jenis ("Cabang"/"Vendor").
- **FR7** Semua pembaca paket aktif ATM menerima kedua sumber (lihat Dampak query). Label untuk join `package_frequencies.package_code` = `COALESCE(p.package_code, avp.package)`;
  cabang pengelola = `COALESCE(p.vendor_branch_id, avp.vendor_branch_id)`.
- **FR8** Update/disable/enable kelolaan lama dan baru tetap berfungsi; update tidak boleh memindahkan baris antar-mode (buat periode baru untuk pindah mode).
- **FR9** Validasi mode `vendor` saat submit **dan** saat apply (keadaan bisa berubah di antaranya; gagal apply → 409 seperti pola sekarang):
  1. ATM aktif; `price_machine_group` dan `price_class` ATM tidak NULL.
  2. `vendor_id` aktif dan `kind = 'FLM_VENDOR'` (bukan INTERNAL/ROH — vendor internal tidak punya tarif).
  3. `vendor_branch_id` milik `vendor_id` dan aktif.
  4. Ada baris `vendor_package_prices` untuk `(vendor_id, package, machine_group, price_class)` yang efektif pada tanggal mulai periode (hari ini WIB untuk periode otomatis).
  Gagal → `ValidationError` pada field `package` ("vendor tidak punya tarif aktif untuk paket ini pada jenis mesin ATM ini").
  Mode `branch` **tidak** diberi validasi baru (gap diterima 2026-09-23).
- **FR10** Kuota kunjungan (`atm-visit-quota`): label `COALESCE(p.package_code, avp.package)` dipakai join `package_frequencies`; label tanpa baris frekuensi tetap = "kuota tidak diketahui" (tidak ditebak).
- **FR11** (Disetujui 2026-10-07) `GET /api/v1/admin/atms/{atmId}/assignment-package-options?vendor_id=…` → daftar label unik `{package}` yang lolos FR9 poin 4 pada hari ini untuk ATM itu. Read-only, replica, RBAC sama dengan route assignments.
  Alternatif tanpa endpoint baru: dedupe di klien dari `/admin/vendors/{id}/package-prices` — ditolak karena tak bisa memfilter menurut mesin ATM dan terpotong paginasi.

- **FR12** Ekspor/impor CSV kelolaan ATM (`atm-assignments`) mendukung kedua mode. Format: tambah satu kolom **`package_source`** (`branch`|`vendor`) setelah `package_code`
  (header: `id, terminal_id, vendor_code, branch_code, package_source, package_code, effective_start_date, effective_end_date, is_active`).
  Ekspor: `vendor_code`/`branch_code` = cabang pengelola (COALESCE), `package_code` = kode paket cabang (mode `branch`) atau label (mode `vendor`).
  Impor: kolom `package_source` boleh kosong/tidak ada → `branch` (file lama tetap valid). Mode `vendor`: `vendor_code` → `vendor_id`, `vendor_code|branch_code` → `vendor_branch_id`, `package_code` → label;
  dry-run memvalidasi pasangan vendor/cabang dan label (ada tarif aktif untuk mesin ATM, FR9) per baris sebelum staging; staging memakai `ATMAssignmentAdminService` yang sama (FR3/FR9).
  Update lewat `id` tidak boleh mengganti `package_source` baris (FR8) → baris ditolak dengan pesan jelas.
- **NFR1** Baris lama tetap valid tanpa backfill: setelah migrasi semua baris existing punya `vendor_package_id` terisi dan kolom baru NULL.
- **NFR2** Integritas di DB, bukan hanya aplikasi: CHECK tepat-satu-mode; unique parsial mode `vendor`; constraint `atm_vendor_packages_no_overlap` (atm + rentang tanggal) tetap menjadi penjaga tumpang tindih lintas mode.
- **NFR3** Pembacaan paket aktif dari replica (Sec 6); penulisan di primary. Tanpa float; tanpa hard delete.
- **NFR4** Hasil query forecast/Vendor Request untuk ATM mode `branch` identik sebelum/sesudah (regresi nol).
- **NFR5** Dialog: target sentuh ≥ 44px, jenis/status tidak hanya dengan warna (Badge + teks), `tabular-nums` pada tanggal (`frontend-ui.md`).

## Data model (usulan — dianggap disetujui saat spec diterima, lalu dicatat di CLAUDE.md Sec 3 sebelum migrasi)
```sql
ALTER TABLE public.atm_vendor_packages
    ALTER COLUMN vendor_package_id DROP NOT NULL,
    ADD COLUMN vendor_id        bigint REFERENCES public.vendors(id),
    ADD COLUMN vendor_branch_id bigint REFERENCES public.vendor_branches(id),
    ADD COLUMN package          text;

ALTER TABLE public.atm_vendor_packages
    ADD CONSTRAINT avp_source_chk CHECK (
        (vendor_package_id IS NOT NULL AND vendor_id IS NULL AND vendor_branch_id IS NULL AND package IS NULL)
     OR (vendor_package_id IS NULL AND vendor_id IS NOT NULL AND vendor_branch_id IS NOT NULL AND package IS NOT NULL AND btrim(package) <> '')
    );

CREATE UNIQUE INDEX avp_vendor_mode_uq
    ON public.atm_vendor_packages (atm_id, vendor_id, vendor_branch_id, package, effective_start_date)
    WHERE vendor_package_id IS NULL;

CREATE INDEX avp_vendor_branch_idx ON public.atm_vendor_packages (vendor_branch_id) WHERE vendor_branch_id IS NOT NULL;
```
- `package` = label persis seperti `vendor_package_prices.package` (join ke `package_frequencies.package_code`). Tidak ada FK ke label (tidak ada tabel label); divalidasi di service (FR9).
- Unique lama `(atm_id, vendor_package_id, effective_start_date)` tetap untuk mode `branch` (NULL tidak bentrok).
- Pasangan vendor↔cabang dijaga service (FR9.3), bukan FK komposit.
- Rollback ditulis di berkas migrasi (drop index/constraint/kolom + `SET NOT NULL`), hanya aman bila belum ada baris mode `vendor`.

## API / payload
- Payload `atm_assignment` (jsonb di `master_data_change_requests`):
  `{atm_id, source?, vendor_package_id?, vendor_id?, vendor_branch_id?, package?, effective_start_date, effective_end_date?}`. `source` kosong = `branch` (kompatibel dengan permintaan pending lama).
- `GET .../assignments` dan `.../assignments/{id}`: tambah `source`, `package`, `vendor_branch_id`, `vendor_id` (kini juga untuk mode vendor); `vendor_package_id` bisa `null`.
- Endpoint baru: FR11 saja.

## Dampak query (tiap item butuh perubahan + test)
| File | Perubahan |
|---|---|
| `atm_assignments_admin.sql` | `LEFT JOIN vendor_packages_branch`; `source`, `package_code`, cabang/vendor = COALESCE; Create/Update/GetByID menerima kolom baru; query validasi tarif + opsi paket baru |
| `atms_admin.sql` | LATERAL kelolaan saat ini: `LEFT JOIN` + COALESCE untuk `package_code`, `branch_code`, vendor |
| `atm_visit_quota.sql` | dua LATERAL: ambil baris `avp`, hitung label COALESCE ke `package_frequencies` |
| `vendor_request.sql` | LATERAL forecast/count/summary (5 tempat): `vendor_branch_id` = COALESCE; `vendor_package_id` boleh NULL — periksa konsumen Go-nya di plan |
| `vendor_branch_atms.sql` | filter "dikelola cabang X" = `COALESCE(vp.vendor_branch_id, avp.vendor_branch_id)`; `package_code` COALESCE |
| `master_data_export.sql` | `ExportATMAssignmentsBatch`: JOIN → LEFT JOIN, `package_source`, `vendor_code`/`branch_code`/`package_code` COALESCE (FR12); `ImportPackageKeys` tetap (mode `branch`), plus daftar label aktif per vendor untuk dry-run mode `vendor` |
Pembaca lain (`internal/repository/*`, `no_hard_delete_test.go`) diaudit di plan dengan graphify/grep sebelum coding.

## Test (jejak ke FR/NFR — diisi di `tests.md`)
- Go unit (service + import/export): validasi payload per mode (FR3), header/parse CSV dua mode + file lama tanpa `package_source` (FR12), validasi tarif (FR9.1–4), payload tanpa `source`, overlap lintas mode.
- Go integrasi (Postgres nyata): CHECK DB (NFR2), unique parsial, no-overlap lintas mode, handover otomatis (FR4), 409 saat apply bila tarif hilang, round-trip list/get (FR6), semua query konsumen untuk ATM mode `vendor` **dan** regresi mode `branch` (NFR4, FR7, FR10).
- Handler: RBAC denial (non-ADMIN → 403), 202 pending, FR11 (RBAC + hasil).
- Frontend (vitest): jenis paket mengganti opsi, cabang wajib, prefill `source`, `canSubmit`, badge jenis, payload mutasi.
- Verifikasi manual di browser: **outstanding, tugas user** (Golden Rule #10).

## Out of scope
- Mengubah harga/tier; layar `package_frequencies`; migrasi massal kelolaan lama ke mode `vendor`.
- Menurunkan harga tagihan per ATM dari kelolaan mode `vendor` (modul invoice) — hanya struktur datanya disiapkan.
- Memindahkan baris antar-mode lewat Update (FR8).

## Keputusan PO
1. FR11 endpoint baru: **disetujui** (2026-10-07).
2. Ekspor/impor CSV: **dua mode** (FR12), bukan hanya mode cabang (2026-10-07).
3. Edit baris mode `vendor` (Update): label boleh diganti dalam mode yang sama (default spec, tidak ada keberatan).
