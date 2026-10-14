# Spec: Replenishment — realisasi pengisian ATM vs order (Phase 2.2c)

Author: Claude, dari `intent.md` (accepted 2026-10-14). Status: **draft 2026-10-14** — menunggu PO accept.
Stage: 2 Design. Stage berikut: `plan.md`.

## ⚠ Flagged (Sec 4 #7)
- **Uang**: nominal realisasi (ITM + laporan vendor) vs order per denom, selisih. `numeric`, IDR, tidak pernah float.
- **Skema baru**: migrasi `031_replenish_realization.sql` (tabel `holidays`, laporan realisasi vendor, hasil klasifikasi).
- **Alur approval berubah**: maker laporan selesai pindah dari ATM-USER ke vendor (pihak `replenish`).
- **Deviasi GR#3**: `holidays` langsung berlaku + `audit_logs` dalam tx yang sama (pola Role Management, intent Q4b).
- **Python menulis notifikasi** (diusulkan, K3) — belum pernah ada; EOD alert Python semula Phase 7.

## Alur & status
Alur Vendor Request tidak berubah, hanya maker laporan selesai:
```
vendor_accepted ──(vendor: lapor realisasi)──► completion_pending ──(ATM-SPV/BRANCH-ATM-SPV approve)──► completed
        ▲                                              │
        └──────────────(reject, alasan)────────────────┘
legacy approved/ready ─(vendor lapor)─► completion_pending   (request lama yang tidak lewat alur vendor, Q2b)
```
Klasifikasi berjalan terpisah, per tiket replenish aktif, di Python EOD:
```
pending ──(jendela ±2 hari kerja tertutup)──► final: on_schedule | early | late | not_done
baris itm_replenish tanpa tiket cocok setelah jendelanya tertutup ──► no_order
tiket nonaktif / request dibatalkan ──► void
```

## Functional requirements

### FR1 — Hari kerja & `holidays` (Q4, Q4b, Q14, keputusan isi awal 2026-10-14)
1.1 Hari kerja = Senin–Jumat yang tidak ada di `holidays` aktif.
1.2 ADMIN saja: lihat (filter tahun), tambah (tanggal + nama 1–100), ubah nama, nonaktif/aktifkan. Tanggal unik di antara baris aktif.
1.3 Import CSV `holiday_date,name` (stdlib): semua-atau-tidak; tanggal yang sudah aktif dilewati dan dilaporkan.
1.4 Setiap perubahan langsung berlaku + `audit_logs` (`holiday_created|updated|disabled|enabled|imported`) dalam tx yang sama; gagal audit → rollback. `ADMIN` dicek di route **dan** service.
1.5 Migrasi tidak mengisi data; libur resmi diinput ADMIN.
1.6 Perubahan hari libur hanya mempengaruhi klasifikasi **belum final** (job berikutnya); yang final tidak dihitung ulang.

### FR2 — Laporan realisasi vendor (Q1, Q6, Q6b–d, Q13)
2.1 Pelapor = `VENDOR-USER` dengan scope pihak `replenish` request itu (sama dengan Orders 2.2b: `users.vendor_id` dari JWT + `vendor_branch_id` dari DB per call; di luar scope → 404). Pihak `vault` tidak melapor.
2.2 Boleh dari `vendor_accepted` (atau legacy `approved`/`ready`). Isi per ATM (tiket aktif): hasil `success|failed`; bila `success`: tanggal realisasi + nominal per denom yang ada di order ATM itu (bulat ≥ 0, kelipatan denom, minimal satu > 0). `failed`: tanpa tanggal/nominal.
2.3 Semua ATM request wajib diisi dalam satu submit → `completion_pending`; `vendor_request_tickets.result` diisi seperti sekarang. Resubmit setelah reject menimpa laporan sebelumnya (riwayat di `audit_logs`).
2.4 Checker = ATM-SPV / BRANCH-ATM-SPV lewat endpoint approve/reject yang ada (tidak berubah): approve → `completed` + kuota terpotong per ATM `success` (idempoten); reject (alasan 1–500) → `vendor_accepted`, vendor melapor ulang.
2.5 `POST /api/v1/vendor-requests/{id}/completion` (input ATM-USER) **ditutup** → 409 `completion_by_vendor`; form di CompanyPortal diganti tampilan laporan vendor. Request yang sudah `completion_pending`/`completed` saat rilis tidak disentuh; request terbuka langsung dilapor vendor (keputusan transisi 2026-10-14).
2.6 Laporan vendor tidak mengubah klasifikasi resmi (Q6c): ITM menang; beda ditandai (FR3.6).
2.7 Tanpa batas waktu melapor (keputusan 2026-10-14); telat ditandai (FR3.7).
2.8 Notifikasi (Go, `notification.Send` dalam tx): vendor submit → ATM-SPV/BRANCH-ATM-SPV aktif; approve/reject → user vendor pelapor.

### FR3 — Klasifikasi harian (Q3, Q5, Q11, keputusan runtime + matching 2026-10-14)
3.1 Runtime **Python EOD**: modul baru `backend_python/replenish_classify/`, dijalankan `eod_retry_scheduler` setelah file `itm_replenish` sukses, juga manual `python -m replenish_classify.run [--date YYYY-MM-DD]`. Idempoten: menghitung ulang semua baris **belum final**; baris final tidak disentuh.
3.2 Yang diklasifikasi: tiket replenish **aktif** dari request `vendor_accepted`, `completion_pending`, `completed` (+ legacy `approved`/`ready`). Tiket yang kemudian nonaktif atau request `rejected`/`cancelled`/`vendor_rejected` → baris belum final jadi `void` (K2).
3.3 Pencocokan: `itm_replenish.terminal_id = tiket.terminal_id`, `replenish_date` dalam ±2 hari kerja dari `tiket.replenish_date`. Lebih dari satu baris → **terdekat ke jadwal** (seri → tanggal lebih awal, lalu `replenish_time` lebih awal). Satu baris ITM hanya untuk satu tiket (tiket dengan jadwal terdekat menang; seri → nomor tiket lebih kecil).
3.4 Offset = jumlah hari kerja dari jadwal ke realisasi (negatif = maju). 0 → `on_schedule`; −1..−2 → `early`; +1..+2 → `late`; tidak ada pengisian dalam jendela → `not_done`.
3.5 Final saat jendela tertutup (hari ini > jadwal + 2 hari kerja). Sebelum itu `pending` dengan hasil sementara (boleh berubah).
3.6 Nominal per denom: `order` (Σ `vendor_request_items.amount_replenish` ATM itu) vs `itm` (`replenish_denom_10k/20k/50k/100k`); `diff = itm − order`; `has_amount_diff` = ada diff ≠ 0 (tanpa toleransi, Q11). Denom order di luar 10K/20K/50K/100K → itm 0. Bila ada laporan vendor: simpan nominal + tanggal vendor; `vendor_mismatch` = tanggal atau nominal vendor ≠ ITM.
3.7 `vendor_report_late` = jendela tertutup dan belum ada laporan vendor ter-submit. Saat pertama kali true → notifikasi ATM-SPV/BRANCH-ATM-SPV aktif (K3).
3.8 `no_order`: baris `itm_replenish` yang setelah jendelanya (±2 hari kerja dari tanggal pengisian) tertutup tidak dipakai tiket mana pun → satu baris `no_order`; vendor/cabang dari kelolaan aktif ATM pada tanggal itu (tidak ada → kosong).
3.9 Hanya pengisian sejak `REPLENISH_CLASSIFY_START_DATE` (env) — tidak ada backfill historis (K4).
3.10 Aksi sistem → `retry_audit_logs` + structured log (pola S4 `import_jobs`); tidak menulis `audit_logs`.

### FR4 — Tampilan internal (intent #7)
4.1 Layar **Replenishment** (`features/replenishment`, ganti mock): per tiket — tanggal jadwal, terminal, vendor, cabang, klasifikasi + offset, order vs ITM per denom, selisih, penanda `beda laporan vendor` / `laporan vendor telat`. Filter tanggal (wajib, maks 31 hari), vendor, region, klasifikasi, hanya-selisih. Paging server-side. Replica.
4.2 Dashboard `ReplenishmentSummary` (ganti mock): hitungan per klasifikasi + jumlah ATM selisih untuk rentang tanggal; nominal instruksi harian + terminal (FNC 003). Replica.
4.3 Role: ATM-USER, ATM-SPV, BRANCH-ATM-SPV, ADMIN (route + service); BRANCH-* dibatasi ke cabangnya seperti layar Vendor Request.

### FR5 — VendorPortal (Q6, Q7)
5.1 Orders detail (2.2b) mendapat form **Lapor Realisasi** (FR2) untuk pihak replenish saat status mengizinkan; tampil status laporan + alasan reject.
5.2 Halaman **Realisasi** baru: klasifikasi + selisih milik vendor sendiri (scope FR2.1, termasuk `no_order` ATM kelolaannya). Replica.

### FR6 — Report + export CSV (Q9, Q12)
6.1 **Amount ≠ order**: klasifikasi final dengan `has_amount_diff`, detail terminal + denom.
6.2 **Trip/realisasi per vendor per ATM**: jumlah pengisian ITM per vendor × terminal dalam rentang, urut terbanyak.
6.3 Export CSV (stdlib, nominal bulat tanpa format, kolom `currency`=IDR), maks 31 hari, replica, `audit_logs` `report_exported` (filter).

### FR7 — Audit
Vendor submit / SPV approve / reject: `audit_logs` dalam tx yang sama (before/after status + laporan). Holidays: FR1.4. Export: FR6.3. Job: FR3.10.

## Non-functional
- Uang `numeric(20,0)` (IDR penuh); Python `Decimal`.
- List/report/dashboard/VendorPortal baca replica; tulis primary; read-after-write di alur yang sama pakai primary.
- Job ringan: hanya baris belum final + ITM baru; jendela EOD 05:00–09:00.
- Dashboard ≤ 3 s p95. Tidak ada dependency baru.

## API (flat JSON, pola handler `backend/`)
| Method | Path | Role | Catatan |
|---|---|---|---|
| POST | `/api/v1/vendor/replenish-orders/{id}/realization` | VENDOR-USER (pihak replenish) | FR2.2–2.3 |
| GET | `/api/v1/vendor/replenish-realizations` | VENDOR-USER | FR5.2 |
| POST | `/api/v1/vendor-requests/{id}/completion` | — | ditutup → 409 (FR2.5) |
| POST | `/api/v1/vendor-requests/{id}/completion/approve`, `/reject` | ATM-SPV, BRANCH-ATM-SPV | tidak berubah |
| GET | `/api/v1/replenishment/classifications` | FR4.3 | FR4.1 |
| GET | `/api/v1/replenishment/summary` | FR4.3 | FR4.2 |
| GET | `/api/v1/replenishment/reports/amount-mismatch` (`?format=csv`) | FR4.3 | FR6.1 |
| GET | `/api/v1/replenishment/reports/trips` (`?format=csv`) | FR4.3 | FR6.2 |
| GET, POST | `/api/v1/admin/holidays` | ADMIN | FR1.2 |
| PUT | `/api/v1/admin/holidays/{date}` | ADMIN | ubah nama |
| POST | `/api/v1/admin/holidays/{date}/disable`, `/enable` | ADMIN | |
| POST | `/api/v1/admin/holidays/import` | ADMIN | FR1.3, multipart CSV |

## Data model (migrasi `031_replenish_realization.sql`)
- **`holidays`**: `id`, `holiday_date date`, `name text (1–100)`, `is_active`, `created_by`, `created_at`, `updated_at`, `deleted_at`; unique partial `(holiday_date) WHERE is_active`.
- **`vendor_request_realizations`**: satu per tiket — `ticket_id → vendor_request_tickets.id` UNIQUE, `realized_date date NULL` (NULL bila failed), `submitted_by → users`, `submitted_at`. Ditimpa saat resubmit.
- **`vendor_request_realization_amounts`**: `realization_id`, `denom integer`, `amount numeric(20,0) ≥ 0` kelipatan denom; PK `(realization_id, denom)`.
- **`replenish_classifications`**: `id`, `ticket_id` NULL (UNIQUE), `itm_replenish_id` NULL (UNIQUE), `terminal_id`, `vendor_id` NULL, `vendor_branch_id` NULL, `scheduled_date` NULL, `realized_date` NULL, `workday_offset` NULL, `classification` CHECK (`pending|on_schedule|early|late|not_done|no_order|void`), `is_final`, `has_amount_diff`, `vendor_mismatch`, `vendor_report_late`, `late_notified_at`, `classified_at`, `finalized_at`. CHECK: `no_order` ⇔ `ticket_id` NULL dan `itm_replenish_id` NOT NULL.
- **`replenish_classification_amounts`**: `classification_id`, `denom integer`, `order_amount numeric(20,0)`, `itm_amount numeric(20,0)`, `vendor_amount numeric(20,0) NULL`, `diff numeric(20,0) GENERATED (itm − order)`; PK `(classification_id, denom)`. Input + selisih tersimpan → reproducible (Sec 6).
- Tidak ada perubahan ke `itm_replenish`, status `vendor_requests`, atau `vendor_request_tickets` (selain `result` yang sudah ada).
- CLAUDE.md Sec 3 + `docs/data-map.md` diperbarui setelah migrasi diterapkan.

## Keputusan spec (diusulkan, konfirmasi PO saat accept)
- **K1** Tanggal realisasi vendor: ≥ jadwal − 7 hari dan ≤ hari ini (cegah salah ketik).
- **K2** `void` untuk tiket nonaktif / request batal, bukan hapus.
- **K3** Notifikasi "laporan vendor telat" ditulis Python langsung ke `notifications` + `notification_emails` (kolom sama dengan `notification.Send`, satu tx dengan `late_notified_at`); email dikirim outbox worker Go yang ada. Alternatif: tanpa notifikasi, hanya penanda.
- **K4** Tidak ada backfill historis.
- **K5** Layar + export maks 31 hari per query.

## Belum dicek (DB dev mati 2026-10-14)
- Rentang tanggal + jumlah `itm_replenish` dan overlap terminal/tanggal dengan tiket dev — dicek di awal `plan.md`; tanpa overlap, uji manual butuh file ITM uji.

## Out of scope
Engine forecast; pemenuhan dana / pengambilan vault + klasifikasi tiket pemenuhan vault; refund per ID, order vs transaksi, transaksi tarik/setor, user log; export XLSX/PDF; exclusion ATM bermasalah (2.4); laporan DSR telat (2.3); invoice/escrow. Status `processing`/`failed` tetap tidak dipakai (Q8).
