# Spec: Pisah akun vendor per peran (vault vs pengisian ATM) + cakupan cabang di VendorPortal

> **⚠ SUPERSEDED 2026-10-20** oleh `.kiro/specs/vendor-party-scope/` (`requirements.md`, `design.md`, `tasks.md`). PO memutuskan
> berbeda dari usulan di bawah (role baru `VENDOR-ATM-USER`/`VENDOR-VAULT-USER`, bukan kolom; `VENDOR-USER` tetap dua peran;
> PIC vendor dipisah per peran). Jangan dipakai untuk build; disimpan sebagai riwayat.

Status: **draft — 2026-10-20**, menunggu PO accept. Stage: 2 Design. Stage berikut: `plan.md`.
Sumber: `intent.md` (draft 2026-10-20). Q1–Q8 intent belum dijawab PO; spec ini mengusulkan jawabannya sebagai
**S1–S9** di bawah. Accept spec = Q1–Q8 terjawab sesuai S1–S9 (atau koreksi PO).

## ⚠ Flagged (Sec 4 #7)
- **Auth/RBAC**: cakupan data user vendor berubah di semua endpoint `/api/v1/vendor/*` (peran pihak + cabang). Akun
  vendor tanpa peran kehilangan akses Orders/Realisasi (fail closed, S3). Risiko utama: tiket peran lain atau cabang lain
  bocor ke akun yang salah.
- **Skema**: migrasi `032_users_vendor_party_role.sql` (kolom + constraint baru di `users`, unique komposit di
  `vendor_branches`). `022` tetap dicadangkan untuk vendor-pks-cis-limit. Tidak ada backfill data produksi.
- **Admin users**: field baru di `/api/v1/admin/users` (create/update) + satu endpoint lookup baru. Apply langsung +
  `audit_logs` (pola admin users yang sudah ada, Sec 12). Tidak ada deviasi GR#3 baru.
- **Tidak ada perubahan JWT** dan tidak ada role baru (S1).

## Kondisi sekarang (dicek di kode, 2026-10-20)
| Area | Fakta | Lokasi |
|---|---|---|
| Cakupan vendor | `GetUserVendorScope` mengembalikan `vendor_id`, `vendor_branch_id` (primary, per call); `scope()` cek role `VENDOR-USER` + claim `vendor_id` = DB. Filter daftar/detail: vendor + cabang opsional, **tanpa peran pihak** | `backend/queries/vendor_parties.sql` (`GetUserVendorScope`, `ListVendorOrders`, `CountVendorOrders`, `GetVendorOrder`); `backend/internal/service/vendor_order.go` `scope()` |
| Realisasi | `SubmitRealization` hanya pihak `replenish` berstatus `accepted`; `checkReportsWholeRequest` mewajibkan semua pihak replenish request ada di cakupan, jadi request multi-cabang butuh akun vendor-wide. `ListRealizations` difilter vendor + cabang | `service/vendor_order_realization.go`, `service/replenishment_read.go` |
| Notifikasi vendor | `notifyParties` mengirim `Recipients{VendorBranchIDs}` → user vendor cabang itu **atau** vendor-wide + email PIC cabang itu atau vendor-wide. `partyNotice` tidak membawa peran | `service/vendor_party.go`; `queries/notifications.sql` (`ListActiveUserRecipientsByVendorBranches`, `ListVendorBranchNotificationPicEmails`) |
| Admin users | `/api/v1/admin/users` (route: `APPACCESS`, `ADMIN`, `ADMIN_PARAM`) hanya mengatur `vendor_id`. **`vendor_branch_id` tidak bisa diatur lewat API/UI**, hanya lewat DB. Tidak ada cek cabang milik vendor (FK hanya ke `vendor_branches.id`) | `handler/admin_user_handler.go`, `auth/user_admin.go`, `queries/users_admin.sql` |
| Lookup vendor di form user | Form memakai `useVendorsList` → `/api/v1/admin/vendors` yang di-guard `ADMIN`/`ADMIN_PARAM`. **APPACCESS tidak bisa memuat daftar vendor** di form user (gap yang sudah ada) | `features/admin-users/components/UserFormDialog.tsx`, `cmd/api/main.go` `masterDataAdmin` |
| Role `VENDOR-USER` di-hardcode | Guard login/refresh VendorPortal, `RequireRoles("VENDOR-USER")` pada `/vendor/replenish-orders` dan `/vendor/replenish-realizations`, `vendorUserRole` di service | `VendorPortal-Vite/src/features/auth/AuthContext.tsx`, `handler/vendor_order_handler.go`, `handler/replenishment_handler.go` |
| Region vs cabang | `vendor_branches.region_code` (mig 026) mengelompokkan beberapa cabang, mis. "Abacus Jawa Barat" = Bandung, Ciamis, Subang | `migrations/026_replenish_ticket_region.sql`, `002_baseline_seed.sql` |
| Seed dev | `seed_vendor_users.sql`: satu `vendor.<code lower>` vendor-wide per vendor FLM aktif. Baseline: `vendor.ssi` (vendor-wide), `vendor.bijak` (cabang 27) | `backend/scripts/dev/seed_vendor_users.sql`, `002_baseline_seed.sql` |

## Keputusan spec (usulan = jawaban Q1–Q8, perlu konfirmasi PO)
- **S1 (Q1): kolom `users.vendor_party_role`, bukan role baru.** Nilainya sama dengan `vendor_request_vendor_parties.role`
  (`replenish` | `vault`), jadi filter cukup `p.role = u.vendor_party_role`. Alasan: peran pihak adalah **cakupan data**
  seperti `vendor_branch_id`, bukan hak menu. Role baru akan menyentuh guard login/refresh VendorPortal, semua
  `RequireRoles("VENDOR-USER")`, `role_permissions`, dan upload DSR. Kolom dibaca dari DB per call (pola 2.2b FR5.2),
  jadi berlaku tanpa login ulang dan JWT tidak berubah.
- **S2 (Q2): tepat satu peran per akun.** Konsekuensi: `users.email` dan `users.username` unik, jadi vendor kecil
  yang satu orangnya memegang dua peran butuh **dua akun dengan dua email**. Alternatif yang tidak diambil: nilai `both`.
  Nilai ini mengembalikan masalah tiket tercampur. Bila PO lebih memilih `both`, cukup tambah nilai di CHECK; FR lain tetap.
- **S3 (Q3): akun lama tanpa peran (`NULL`) = fail closed.** Orders dan Realisasi menjawab `403` `vendor_scope_unset`,
  dan akun tidak menerima notifikasi pihak. Fitur lain (DSR upload, notifikasi umum, profil) tidak berubah. Migrasi
  tidak mem-backfill. Akun dev diatur oleh seed (S5); akun produksi diatur admin sebelum rilis.
- **S4 (Q4): cakupan per cabang vendor (`vendor_branch_id`, sudah ada), bukan per region.** Pihak dialamatkan per
  cabang, dan satu region bisa berisi beberapa cabang (Jawa Barat = 3 cabang). **Akun vendor-wide tetap diizinkan**
  sebagai pilihan eksplisit "Semua cabang". Alasannya: realisasi request multi-cabang wajib dilaporkan akun vendor-wide
  (`checkReportsWholeRequest`), dan kantor pusat vendor butuh pandangan penuh. Cakupan per region = out of scope.
- **S5 (Q5): seed dev dipecah per peran, ditambah satu pasang akun cabang untuk uji kebocoran.**
  `vendor.<code>` yang sudah ada menjadi `replenish` vendor-wide. Ditambah `vendor.<code>.vault` (vault, vendor-wide).
  Untuk ABACUS ditambah akun berbatas cabang: Medan × {replenish, vault} + satu cabang non-Medan × {replenish, vault}.
  `vendor.ssi` dan `vendor.bijak` (baseline) diberi peran `replenish` oleh seed. Seed tetap idempoten, tanpa audit (data dev).
- **S6 (Q6): notifikasi pihak hanya ke akun dengan peran = peran pihak** dan cabang = cabang pihak atau vendor-wide.
  **PIC vendor (`vendor_pics`) tidak dipisah per peran.** `vendor_pics` tidak punya kolom peran, dan menambahnya =
  fitur master data baru (dilarang scope guard development-plan rule 2). PIC tetap menerima email per cabang seperti
  sekarang → known gap, dicatat.
- **S7 (Q7): laporan dan daftar realisasi hanya untuk akun `replenish`.** Akun vault: menu Realisasi disembunyikan,
  `GET /vendor/replenish-realizations` → `403` `vendor_scope_role`. Pihak replenish sudah di luar cakupan akun vault,
  sehingga `POST …/realization` oleh akun vault → `404`.
- **S8 (Q8): yang mengatur peran/cabang = guard `/api/v1/admin/users` yang ada** (`APPACCESS`, `ADMIN`, `ADMIN_PARAM`),
  tidak diubah. Intent menyebut "ADMIN/APPACCESS". `ADMIN_PARAM` sudah punya akses hari ini; mencabutnya di luar
  cakupan spec ini kecuali PO minta.
- **S9 (baru): lookup vendor + cabang khusus form user**, `GET /api/v1/admin/users/vendor-options`, dengan guard yang
  sama dengan `/admin/users`. Tanpanya APPACCESS tidak bisa memilih vendor/cabang (gap tabel di atas). Read-only,
  replica, hanya field yang dibutuhkan form.

## Functional requirements

### FR1 — Data akun vendor
- FR1.1 `users.vendor_party_role` ∈ `replenish`, `vault`; `NULL` = belum diatur (S3).
- FR1.2 Hanya boleh terisi bila `vendor_id` terisi (CHECK DB + validasi service). User non-vendor → `NULL`.
- FR1.3 `vendor_branch_id` opsional (`NULL` = semua cabang vendor, S4). Bila diisi: cabang **aktif** milik `vendor_id`
  (validasi service, 400 `vendor_branch_id`) dan dijaga FK komposit `(vendor_branch_id, vendor_id)` → `vendor_branches(id, vendor_id)`.
- FR1.4 Ganti `vendor_id` pada update: `vendor_branch_id` lama yang bukan milik vendor baru → 400 (tidak dikosongkan diam-diam).

### FR2 — Admin users (backend + CompanyPortal)
- FR2.1 `POST` / `PUT /api/v1/admin/users` menerima `vendor_party_role` dan `vendor_branch_id`. `GET` list/detail
  mengembalikan keduanya + `vendor_branch_name`.
- FR2.2 Role `VENDOR-USER`: `vendor_party_role` **wajib** pada create **dan** update (akun baru tidak pernah `NULL`;
  akun lama ikut diatur saat diedit). Role lain: kedua field harus kosong (400).
- FR2.3 Audit `user_created` / `user_updated` (sudah ada) memuat kedua field di before/after. Baris `GetUserAdminByID`,
  `CreateUserAdmin`, `UpdateUserAdmin` ikut memuat kolom baru.
- FR2.4 `GET /api/v1/admin/users/vendor-options` (S9): vendor aktif `{id, code, name}` + cabang aktif `{id, vendor_id,
  branch_code, branch_name, region_code}`. Replica.
- FR2.5 Form CompanyPortal (`features/admin-users`): saat role `VENDOR-USER`, select **Peran** (Pengisian ATM / Penyedia
  uang, wajib) dan select **Cabang** ("Semua cabang" + cabang aktif vendor terpilih, dari FR2.4). Ganti vendor →
  cabang di-reset. Tabel user: kolom Peran dan Cabang. Vendor di form dan filter memakai FR2.4 (bukan `/admin/vendors`).
- FR2.6 Perubahan berlaku pada request berikutnya tanpa login ulang (scope dibaca dari DB).

### FR3 — Cakupan VendorPortal Orders
- FR3.1 Cakupan = `users.vendor_id` = claim ∧ `party.role = users.vendor_party_role` ∧ (`users.vendor_branch_id IS NULL`
  ∨ `party.vendor_branch_id = users.vendor_branch_id`). Diterapkan di SQL untuk daftar, hitung, detail, terima, tolak,
  realisasi (FR5.4 2.2b: filter di query, bukan handler).
- FR3.2 `vendor_party_role IS NULL` → `403` dengan `error` `vendor_scope_unset` di semua endpoint Orders/Realisasi.
  Ini 403 (bukan 404) karena menyangkut akun, bukan objek, jadi tidak membocorkan keberadaan tiket.
- FR3.3 Pihak di luar cakupan (peran lain, cabang lain, vendor lain) → `404` (tetap).
- FR3.4 Kasus intent `REP-ABA-MDN-20261014-001`: Abacus Medan sebagai replenish **dan** vault (S2 2.2b) kini ditangani
  dua akun berbeda. Akun replenish hanya melihat baris replenish, akun vault hanya baris vault.
- FR3.5 Alur keputusan pihak, status Vendor Request, dan `content` snapshot **tidak berubah** (constraint intent).

### FR4 — Realisasi
- FR4.1 `POST /vendor/replenish-orders/{id}/realization`: hanya akun `replenish` (akun vault → 404 lewat FR3.3).
- FR4.2 `checkReportsWholeRequest` tetap: semua pihak replenish request harus di cakupan. Request multi-cabang → akun
  replenish vendor-wide (pesan yang ada tetap).
- FR4.3 `GET /vendor/replenish-realizations`: akun vault → `403` `vendor_scope_role`; akun replenish → vendor + cabang (tetap).

### FR5 — Notifikasi
- FR5.1 `partyNotice` membawa peran pihak. Penerima in-app/email = user aktif `VENDOR-USER` vendor itu dengan
  `vendor_party_role` = peran pihak dan (`vendor_branch_id` = cabang pihak ∨ `NULL`). Akun `NULL` peran tidak menerima.
- FR5.2 Email PIC (`vendor_pics`) tetap per cabang tanpa peran (S6).
- FR5.3 `internal/notification`: recipient baru berisi pasangan (cabang, peran) untuk user. PIC tetap per cabang.
  `VendorBranchIDs` hanya dipakai `notifyParties`, jadi plan boleh menggantinya (keputusan di plan).
- FR5.4 Notifikasi keputusan realisasi (ke pelapor) dan notifikasi internal tidak berubah.

### FR6 — VendorPortal UI
- FR6.1 Header/sidebar menampilkan cakupan akun: "<vendor> · <cabang | Semua cabang> · <Pengisian ATM | Penyedia uang>"
  dari `GET /api/v1/auth/me`, yang ditambah `vendor_branch_id`, `vendor_branch_name`, `vendor_party_role` (nullable;
  field lama tidak berubah).
- FR6.2 Akun vault: menu Realisasi tidak tampil. Akses langsung `/realizations` → pesan "Tidak tersedia untuk peran Anda".
- FR6.3 `vendor_scope_unset` → halaman Orders menampilkan pesan dengan ikon + teks: "Akun Anda belum diatur peran
  vendornya. Hubungi admin CIMB Niaga." (bukan tabel kosong).
- FR6.4 Label peran memakai label yang ada (`ROLE_LABEL`, `features/orders/labels.ts`). Kolom "Peran" di Orders tetap.

### FR7 — Seed dev (S5)
- FR7.1 `backend/scripts/dev/seed_vendor_users.sql` diperbarui sesuai S5, tetap idempoten. Bukan migrasi.

## Non-functional
- NFR1 Tanpa query tambahan per request: `vendor_party_role` dibaca di `GetUserVendorScope` yang sama (primary).
- NFR2 Daftar tetap di replica; scope, detail, dan aksi di primary (pola 2.2b NFR2). Index baru hanya bila `EXPLAIN`
  di plan menunjukkan perlu (peran = kardinalitas rendah; index `(vendor_branch_id, status)` yang ada tetap dipakai).
- NFR3 Test integrasi Postgres nyata, termasuk **test kebocoran**: (a) akun vault Medan tidak melihat pihak replenish
  Medan di request yang sama, (b) akun Medan tidak melihat pihak cabang lain, (c) akun `NULL` peran → 403, (d)
  notifikasi pihak hanya ke akun peran + cabang yang cocok, (e) admin: cabang bukan milik vendor → 400 + ditolak FK.
  Coverage ≥ 80% kode baru di `internal/*`.
- NFR4 `backend/api/openapi.yaml`: kode error `vendor_scope_unset` / `vendor_scope_role` didokumentasikan pada respons
  403 path `/vendor/replenish-orders*`. Skema `VendorOrder*` tidak berubah. `/auth/me` dan `/admin/users` di luar
  pilot OpenAPI (BE-10), tetap hand-typed.

## API (flat JSON, pola handler `backend/`)
| Method | Path | Role | Perubahan |
|---|---|---|---|
| GET | `/api/v1/vendor/replenish-orders` | VENDOR-USER | cakupan + peran (FR3), 403 `vendor_scope_unset` |
| GET · POST | `/api/v1/vendor/replenish-orders/{partyID}` · `/accept` · `/reject` · `/realization` | idem | idem |
| GET | `/api/v1/vendor/replenish-realizations` | idem | vault → 403 `vendor_scope_role` (FR4.3) |
| POST · PUT · GET | `/api/v1/admin/users` · `/{id}` | APPACCESS, ADMIN, ADMIN_PARAM (tetap) | field `vendor_party_role`, `vendor_branch_id` (+ `vendor_branch_name` di respons) |
| GET | `/api/v1/admin/users/vendor-options` | idem | **baru** (S9) |
| GET | `/api/v1/auth/me` | semua | + `vendor_branch_id`, `vendor_branch_name`, `vendor_party_role` (nullable) |

## Data model (migrasi `032_users_vendor_party_role.sql`)
```sql
-- pre-check: tidak ada users.vendor_branch_id yang bukan milik users.vendor_id (RAISE bila ada)
ALTER TABLE public.vendor_branches
    ADD CONSTRAINT vendor_branches_id_vendor_uq UNIQUE (id, vendor_id);        -- target FK komposit (pola 030)
ALTER TABLE public.users
    ADD COLUMN vendor_party_role text
        CONSTRAINT users_vendor_party_role_chk CHECK (vendor_party_role IN ('replenish', 'vault')),
    ADD CONSTRAINT users_vendor_party_role_vendor_chk CHECK (vendor_party_role IS NULL OR vendor_id IS NOT NULL),
    ADD CONSTRAINT users_vendor_branch_owner_fk
        FOREIGN KEY (vendor_branch_id, vendor_id) REFERENCES public.vendor_branches (id, vendor_id);
COMMENT ON COLUMN public.users.vendor_party_role IS
    'Peran pihak vendor yang dilihat akun di VendorPortal (= vendor_request_vendor_parties.role). NULL = belum diatur, tanpa akses Orders.';
```
- Satu tx, forward-only, tanpa backfill. FK komposit MATCH SIMPLE: hanya dicek bila kedua kolom terisi.
  `users_vendor_branch_fk` yang lama tetap.
- Catat di CLAUDE.md Sec 3 + `docs/data-map.md` saat diterapkan.

## Out of scope
Cakupan per region · role vendor baru · PIC vendor per peran (S6) · scoping upload DSR per peran/cabang (tetap per
vendor, kedua peran) · laporan realisasi parsial per cabang · mencabut akses `ADMIN_PARAM` ke `/admin/users` ·
halaman mock VendorPortal (Schedule, Invoices, Evidence) · perubahan JWT · tampilan internal CompanyPortal selain form/tabel user.

## Known gaps (diterima bila spec di-accept)
- PIC vendor menerima email pihak replenish dan vault cabangnya (S6).
- Vendor kecil yang satu orangnya memegang dua peran butuh dua akun + dua email (S2).
- Akun vendor produksi yang ada (bila ada saat rilis) tidak bisa membuka Orders sampai admin mengatur perannya (S3).
