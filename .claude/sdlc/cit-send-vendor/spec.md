# Spec: Kirim Vendor Request ke vendor + terima/tolak di VendorPortal (Phase 2.2b)

Status: **accepted — 2026-10-09** (user: "diterima, lanjut plan"; S1–S5 dikonfirmasi). Stage: 2 Design — selesai. Stage berikut: `plan.md`.
Sumber: `intent.md` (accepted 2026-10-09, Q1–Q12).

## ⚠ Flagged (Sec 4 #7)
- **Auth/RBAC**: endpoint baru untuk role `VENDOR-USER`; scoping per vendor + `users.vendor_branch_id`; satu request terlihat
  oleh beberapa vendor (vault vendor lain) → kebocoran data antar vendor = risiko utama.
- **Alur approval Vendor Request berubah** (status baru, gate laporan selesai pindah ke `vendor_accepted`, alur balik ke ACM /
  ATM-SPV) — alur yang sudah live.
- **Skema**: migrasi `029` (`022` tetap dicadangkan untuk vendor-pks-cis-limit).
- **Deviasi terdokumentasi** (dicatat di CLAUDE.md Sec 12 saat diterapkan): keputusan vendor memakai state machine sendiri pada
  tabel keputusan (pola Vendor Request / rencana vault 2.2a), bukan `approval_requests` — vendor bukan maker-checker internal.

## Alur & status

```
Vendor Request (vendor_requests.status)
vault_review --ATM-SPV vault-approve--> sent_to_vendor        (S1: langsung, `ready` tidak disimpan untuk request baru)
sent_to_vendor --semua pihak accepted--> vendor_accepted
sent_to_vendor --pihak VAULT reject--> vault_assignment       (plan area terdampak → draft; lalu ACM → ACM-SPV → vault_review
                                                                → ATM-SPV → sent_to_vendor, kirim ulang hanya pihak berubah)
sent_to_vendor --pihak REPLENISH reject--> vendor_rejected
vendor_rejected --ATM-SPV return (alasan)--> rejected         (pola reject yang ada → ATM-USER revise → draft → edit → submit
                                                                → pending_approval → ATM-SPV approve → vault_assignment …)
sent_to_vendor | vendor_accepted | vendor_rejected --ATM-SPV cancel--> cancelled
vendor_accepted --laporan selesai--> completion_pending --reject--> vendor_accepted
ready / approved = legacy (sudah ada sebelum rilis): tidak dikirim, laporan selesai tetap boleh (FR6.3)

Keputusan vendor per pihak (vendor_request_vendor_parties.status)
pending --accept--> accepted
pending --reject(alasan)--> rejected
(kirim ulang) rejected / isi berubah → pending      pihak hilang dari request → withdrawn
```

## Functional requirements

### FR1 — Pihak vendor (party) per request
- FR1.1 **Pihak** = (request, `vendor_branch_id`, `role` ∈ `replenish`,`vault`), diturunkan dari `vendor_request_vault_assignments`
  request itu: tiap `replenish_branch_id` distinct → pihak `replenish`; tiap `vault_branch_id` distinct → pihak `vault`.
- FR1.2 Branch yang menjadi **replenish sekaligus vault** (F7 2.2a, `ATM_CASH` sendiri) punya **dua** pihak dan memutuskan
  dua kali (S2) — rute penolakannya berbeda (FR4).
- FR1.3 Tiap pihak menyimpan **snapshot isi** (`content` jsonb) yang dilihat vendor saat dikirim: daftar ATM + nominal per
  denom + pihak lawan (FR3). Snapshot = bukti apa yang diterima/ditolak; tidak diubah setelah keputusan.

### FR2 — Kirim otomatis
- FR2.1 Di tx ATM-SPV `vault-approve` (2.2a FR5.2), request langsung `vault_review → sent_to_vendor` (S1), `sent_at` diisi, pihak
  dibuat/disinkronkan (FR2.2), notifikasi vendor (FR7.1). Tidak ada tombol kirim (Q2).
- FR2.2 **Sinkronisasi saat kirim ulang**: pihak baru → `pending`; pihak yang ada dengan `content` berbeda **atau** status
  `rejected` → `pending` (keputusan lama ke riwayat); pihak `accepted` dengan `content` sama → tetap `accepted` (tidak
  dinotifikasi); pihak yang tidak lagi ada → `withdrawn` (dinotifikasi "ditarik"). Hanya pihak yang menjadi `pending` /
  `withdrawn` yang dinotifikasi (Q6 "hanya branch yang berubah").
- FR2.3 Setelah edit request (alur `rejected → draft`), **semua** pihak kembali `pending` saat dikirim lagi (Q12), walau isi sama.
- FR2.4 Request `vault_flow=false` atau sudah `ready`/`approved` sebelum rilis tidak pernah dikirim.

### FR3 — Data yang dilihat vendor (Q7)
- FR3.1 **Pihak replenish**: nomor request, tanggal replenish, status request + status pihak; per ATM: terminal ID, lokasi
  (`vendor_request_items.lokasi_atm`), nomor tiket aktif, nominal order per denom (`amount_replenish`), **branch vault**
  (nama branch, nama vendor, alamat dari `vendor_branches.location_id → locations`).
- FR3.2 **Pihak vault**: nomor request, tanggal replenish, status; hanya ATM yang `vault_branch_id`-nya branch itu: terminal ID,
  lokasi, nominal per denom, **branch replenish** pengambil (nama branch, vendor); **total per denom** = Σ ATM tersebut.
- FR3.3 Tidak pernah dikirim ke vendor: saldo/kapasitas snapshot, tier, `is_urgent`/`urgent_reason`, alasan/catatan internal
  ACM & ATM-SPV, harga paket, data ATM pihak lain.
- FR3.4 Uang = `amount_replenish` apa adanya (bigint IDR penuh), mata uang `IDR` eksplisit di respon.

### FR4 — Terima / tolak (Q4, Q5, Q6)
- FR4.1 Hanya saat request `sent_to_vendor` dan pihak `pending`. User `VENDOR-USER` dalam cakupan pihak (FR5) → **accept**
  atau **reject** + alasan 10–500 karakter. Konkuren: `SELECT … FOR UPDATE` request + pihak; keputusan kedua → 409.
- FR4.2 **Accept**: pihak `accepted` (`decided_by/at`). Bila semua pihak aktif `accepted` → request `vendor_accepted`
  (`vendor_accepted_at`) + notifikasi FR7.2.
- FR4.3 **Reject pihak vault**: request → `vault_assignment`; plan area yang memuat ATM dengan `vault_branch_id` = branch itu →
  `draft` (`rejection_reason` = alasan vendor, ditampilkan ke ACM sebagai "Ditolak vendor"); plan lain tetap `acm_approved`.
  Penetapan vault ATM terdampak **tidak dihapus** (ACM menggantinya). Notifikasi FR7.2.
- FR4.4 **Reject pihak replenish**: request → `vendor_rejected` + notifikasi FR7.2. Bila pihak vault juga menolak di request yang
  sama, `vendor_rejected` didahulukan (ATM-SPV memutuskan).
- FR4.5 Setelah request keluar dari `sent_to_vendor`, pihak `pending` yang tersisa tidak bisa memutuskan (409) sampai dikirim ulang;
  keputusan `accepted` tetap tercatat (Q6).

### FR5 — Cakupan vendor (Q3)
- FR5.1 User `VENDOR-USER` dengan `vendor_branch_id` → hanya pihak dengan branch itu. Tanpa `vendor_branch_id` → semua pihak yang
  branch-nya milik `users.vendor_id`.
- FR5.2 `vendor_id` dari claim JWT; `vendor_branch_id` **dibaca dari `users` (primary) per request** — tidak ditambahkan ke JWT
  (S3), sehingga perubahan cakupan berlaku tanpa login ulang.
- FR5.3 Request/pihak di luar cakupan → **404** (bukan 403). Daftar & detail hanya menampilkan pihak dalam cakupan; bila user
  mencakup dua pihak satu request (replenish + vault), keduanya tampil terpisah.
- FR5.4 RBAC di route (`RequireRoles("VENDOR-USER")`) **dan** service (filter cakupan di query, bukan di handler).

### FR6 — Internal: ATM-SPV, laporan selesai, batal
- FR6.1 **`vendor_rejected`**: ATM-SPV / BRANCH-ATM-SPV melihat alasan vendor; aksi **Kembalikan ke pembuat** (alasan 1–500) →
  `rejected` (pola reject yang ada; ATM-USER pembuat revise → `draft` → edit → submit), atau **Batal**. Maker ≠ checker: ATM-SPV ≠
  pembuat (aturan reject yang ada).
- FR6.2 Saat request kembali lewat `pending_approval → vault_assignment` (approve ulang): plan area yang ada → `draft` (pakai
  `ResetVaultPlansToDraft`), plan area baru dibuat (`CreateMissingVaultPlans`); penetapan ATM yang sudah tidak ada di request
  dihapus saat edit (baris draft, diaudit); penetapan ATM yang masih ada tetap sebagai usulan awal ACM.
- FR6.3 **Laporan selesai** (`completion_pending`) hanya dari `vendor_accepted` (request kirim), `ready` (legacy 2.2a sebelum rilis)
  atau `approved` (legacy). Reject laporan → kembali ke status asal (`vendor_accepted` bila `vendor_sent=true`).
- FR6.4 **Batal** (aturan cancel yang ada, checker) juga dari `sent_to_vendor` / `vendor_accepted` / `vendor_rejected`: plan →
  `cancelled` (sudah ada), pihak tidak diubah (riwayat), notifikasi FR7.1 "dibatalkan". Vendor tetap melihat request dengan
  status "Dibatalkan" (tidak dihapus), tanpa aksi.
- FR6.5 Request terkirim **tidak bisa diedit** selain lewat FR6.1 (edit sudah hanya mungkin di `draft`; tidak ada perubahan kode
  edit, hanya ditegaskan di test).
- FR6.6 Detail Vendor Request (CompanyPortal): panel **"Status Vendor"** per pihak (branch, vendor, peran, status, waktu, oleh,
  alasan tolak) + riwayat keputusan. Daftar Vendor Request: filter status baru.

### FR7 — Notifikasi (Q8) — `notification.Send` dalam tx pemicu, in-app + email
- FR7.1 **Ke vendor** per pihak yang dinotifikasi (FR2.2, FR6.4): user `VENDOR-USER` aktif vendor itu dengan `vendor_branch_id` =
  branch pihak **atau NULL**, + email PIC `is_notification_recipient` vendor itu dengan `vendor_branch_id` = branch **atau NULL**
  (S4). Butuh recipient baru `VendorBranchIDs` di `internal/notification` (perluasan kecil, `VendorIDs` tidak berubah).
  Tipe: terkirim, dikirim ulang, ditarik (`withdrawn`), dibatalkan. Link VendorPortal `/orders/{id}`.
- FR7.2 **Ke internal**: reject vault → ACM-USER + ACM-SPV anggota area terdampak; reject replenish → role `ATM-SPV`,
  `BRANCH-ATM-SPV` + pembuat; semua diterima → pembuat + role `ATM-SPV`. Tidak ada notifikasi per accept.

### FR8 — Audit
- FR8.1 Kirim/kirim ulang, accept, reject, return, cancel → `audit_logs` dalam tx: entity `vendor_request` untuk status request,
  `vendor_request_vendor_party` untuk keputusan (before/after status, alasan, actor = user vendor). Kirim otomatis diaudit dengan
  actor = ATM-SPV yang approve (aksi pemicunya).
- FR8.2 Riwayat keputusan: tabel `vendor_request_vendor_party_events` (append-only) — dipakai panel riwayat (FR6.6) tanpa parse
  `audit_logs`.

### FR9 — VendorPortal (Q11)
- FR9.1 Halaman **Orders** diganti data nyata (hapus pemakaian `orders.json`): daftar pihak dalam cakupan (filter status pihak,
  status request, tanggal replenish; paging server), kolom: nomor request, tanggal replenish, peran (Replenish/Vault), branch,
  jumlah ATM, total per denom, status.
- FR9.2 Detail `/orders/{id}`: isi FR3 per pihak + tombol **Terima** / **Tolak** (dialog alasan) saat aktif; status pakai label +
  ikon, bukan warna saja; uang `tabular-nums`, rata kanan, IDR.
- FR9.3 Schedule tetap mock. `orders.json` + tipe `CITOrder` dihapus bila tidak dipakai lagi.

## Non-functional
- NFR1 Daftar vendor ≤ 3 s p95 (replica); detail ≤ 1000 ATM/request.
- NFR2 Tulis primary; daftar dari replica; detail + cakupan (`users.vendor_branch_id`) + aksi dari primary.
- NFR3 Transisi `FOR UPDATE` pada request lalu pihak (urutan kunci tetap) — aman terhadap dua accept paralel yang sama-sama
  "terakhir".
- NFR4 Coverage ≥ 80% service baru; integration test Postgres nyata, termasuk **test kebocoran**: user vendor A tidak melihat
  pihak/ATM vendor B pada request yang sama.

## API (flat JSON, pola handler `backend/`)
| Method | Path | Role |
|---|---|---|
| GET | `/api/v1/vendor/replenish-orders?party_status=&request_status=&from=&to=&page=` | VENDOR-USER (cakupan) |
| GET | `/api/v1/vendor/replenish-orders/{partyID}` | idem — isi FR3 |
| POST | `/api/v1/vendor/replenish-orders/{partyID}/accept` · `/reject` | idem |
| GET | `/api/v1/vendor-requests/{id}/vendor-parties` | role Vendor Request yang ada — status + riwayat (FR6.6) |
| POST | `/api/v1/vendor-requests/{id}/vendor-return` | ATM-SPV, BRANCH-ATM-SPV (dari `vendor_rejected`) |

ID di URL vendor = `partyID` (bukan request ID), sehingga satu URL = satu cakupan yang diperiksa.

## Data model (migrasi `029_vendor_request_send.sql`)
- `vendor_requests_status_chk` + `sent_to_vendor`, `vendor_accepted`, `vendor_rejected`; kolom `vendor_sent boolean NOT NULL
  DEFAULT false` (true = request lewat langkah vendor; menentukan status kembali saat laporan selesai ditolak, pola
  `vault_flow`), `sent_at`, `vendor_accepted_at` timestamptz.
- `vendor_request_vendor_parties` — `id`, `vendor_request_id` FK, `vendor_branch_id` FK, `role` CHECK (`replenish`,`vault`),
  `status` CHECK (`pending`,`accepted`,`rejected`,`withdrawn`), `content` jsonb, `sent_at`, `decided_by` FK users, `decided_at`,
  `rejection_reason`, timestamps; UNIQUE (`vendor_request_id`, `vendor_branch_id`, `role`); index (`vendor_branch_id`, `status`).
- `vendor_request_vendor_party_events` — `id`, `party_id` FK, `event` (`sent`,`resent`,`accepted`,`rejected`,`withdrawn`,
  `cancelled`), `actor_id` FK users, `reason`, `content` jsonb (snapshot saat event), `created_at`. Append-only.
- Tidak ada hard delete; tidak ada backfill (request lama tetap `ready`/`approved`).

## Keputusan spec (diusulkan, perlu konfirmasi PO saat accept)
- **S1** — Request baru tidak berhenti di `ready`: `vault-approve` langsung ke `sent_to_vendor`. `ready` yang tersisa di DB =
  legacy sebelum rilis → otomatis "melewati" tanpa penanda migrasi (menjawab open question intent #3).
- **S2** — Branch replenish + vault sekaligus = dua pihak, dua keputusan.
- **S3** — Cakupan cabang dibaca dari DB, JWT tidak diubah.
- **S4** — PIC vendor (email saja) ikut menerima notifikasi bila branch-nya cocok atau vendor-wide.
- **S5** — Tolak replenish → status baru `vendor_rejected`, lalu ATM-SPV pakai "Kembalikan" (→ `rejected`, alur revise yang ada)
  atau Batal (menjawab open question intent #1). Tolak vault → hanya plan area terdampak kembali `draft` (open question #2).

## Out of scope
Alur cash pickup FSD (petugas, nopol, validasi Provider, serah terima); halaman Schedule; batas waktu/SLA/penalti; pindah buku /
escrow; realisasi vs order (2.2c); ubah kelolaan; perubahan JWT.
