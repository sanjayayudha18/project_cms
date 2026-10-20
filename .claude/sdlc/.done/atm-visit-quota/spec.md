# Spec: Kuota kunjungan replenish per ATM + laporan selesai Vendor Request

Status: **accepted 2026-10-01** (user: "spec diterima, lanjut plan.md"). Stage: 2 Design — selesai.
Input: `intent.md` (accepted 2026-10-01, user: "diterima, lanjut").
Tambahan keputusan saat spec (2026-10-01): reset massal per vendor diletakkan di **ringkasan Forecast Browser**
(bukan detail vendor — halaman itu hanya ADMIN/ADMIN_PARAM/APPACCESS, `cmd/api/main.go:158`). Mengganti keputusan intent #17 untuk reset massal.

## Definisi
- **Kuota** = `package_frequencies.cr_frequency` untuk `(vendor_packages_branch.package_code, atms.price_machine_group)`
  dari paket **aktif hari ini (WIB)** ATM itu — resolusi paket aktif sama dengan `BrowseForecast`
  (`atm_vendor_packages` aktif, tie-break `avp.id DESC`). Tidak cocok / tidak ada paket / `price_machine_group` NULL → **kuota tidak diketahui**.
  `cr_frequency` hanya dibaca, tidak pernah diubah.
- **Sisa** (`remaining`) = angka tersimpan per ATM. Dikurangi 1 per kunjungan, boleh **negatif**.
  Tampil: sisa = `max(remaining, 0)`, **kelebihan** = `max(-remaining, 0)`.
- **Kunjungan** = satu ATM berstatus **berhasil** pada laporan selesai yang di-approve SPV.
  Satu ATM = satu kunjungan per Vendor Request, walau ATM itu punya beberapa baris item (denom/periode berbeda).
- **Periode berjalan** = sejak `reset_at` terakhir ATM itu (tanpa reset otomatis).

## Functional requirements

### FR1 — State machine Vendor Request: laporan selesai
Transisi baru (tabel `transitions`, `service/vendor_request.go`):

| Dari | Aksi | Ke | Aktor |
|---|---|---|---|
| `approved` | `submit_completion` | `completion_pending` | maker: ATM-USER / BRANCH-ATM-USER / ADMIN |
| `completion_pending` | `approve_completion` | `completed` | checker: ATM-SPV / BRANCH-ATM-SPV / ADMIN, **≠ `completion_submitted_by`** |
| `completion_pending` | `reject_completion` | `approved` | checker, ≠ `completion_submitted_by`; alasan wajib |

- FR1.1 `submit_completion` membawa hasil **per ATM** (`terminal_id` → `success` | `failed`). Wajib mencakup **setiap**
  `terminal_id` distinct di `vendor_request_items` request itu, tidak boleh terminal lain → 400 jika tidak.
  Pengajuan ulang setelah ditolak menimpa hasil sebelumnya.
- FR1.2 `approve_completion` dalam **satu transaksi**: ubah status → `completed`; untuk setiap ATM `success` buat
  kunjungan (FR3) dan kurangi sisa (FR2); tulis `audit_logs`. Gagal sebagian → rollback semua.
- FR1.3 `reject_completion`: status kembali `approved`, simpan alasan; tidak ada kunjungan; kuota tidak berubah.
- FR1.4 `cancel` dari `completion_pending` **tidak diizinkan** (409 `ErrInvalidTransition`); `cancel` dari `approved` tetap seperti sekarang.
- FR1.5 Request `approved` yang dibatalkan tidak pernah membuat kunjungan.

### FR2 — Sisa kunjungan per ATM
- FR2.1 Saat kunjungan dibuat dan ATM **belum punya** baris kuota: buat baris dengan `quota_total = kuota` dan
  `remaining = kuota − 1`, `reset_at = now()`. Kuota tidak diketahui → **tidak** buat baris kuota; kunjungan tetap dicatat (`quota_known = false`).
- FR2.2 ATM **sudah punya** baris: `remaining = remaining − 1` secara atomik (`UPDATE … SET remaining = remaining - 1 RETURNING remaining`,
  row lock dalam tx — aman untuk dua approve paralel pada ATM yang sama).
- FR2.3 `remaining` setelah dikurangi `< 0` → kunjungan ditandai `is_over_quota = true`.
- FR2.4 Ganti paket **tidak** mengubah `quota_total`/`remaining` (berlaku setelah reset).

### FR3 — Log kunjungan + pembatalan
- FR3.1 Satu baris per (Vendor Request, ATM) berhasil; **unik** `(vendor_request_id, atm_id)` → approve ganda tidak mengurangi dua kali.
- FR3.2 Pembatalan kunjungan oleh ATM-SPV / BRANCH-ATM-SPV / ADMIN, alasan wajib (1–500 karakter): soft-cancel
  (`cancelled_at/by/reason`), `remaining + 1`, audit, satu tx. Tidak dihapus.
- FR3.3 Hanya kunjungan **periode berjalan** (`created_at >= reset_at` kuota ATM) yang bisa dibatalkan; lainnya / sudah batal → 409.
  Kunjungan dengan `quota_known = false` bisa dibatalkan tanpa mengubah sisa.

### FR4 — Reset kuota
- FR4.1 Per ATM: `quota_total = remaining = kuota` paket aktif sekarang, `reset_at = now()`, `reset_by` = aktor; buat baris bila belum ada.
  Kuota tidak diketahui → 422 "Paket ATM tidak dikenali, kuota tidak dapat di-reset".
- FR4.2 Massal per vendor: semua ATM dengan paket aktif milik vendor itu (`vendor_branches.vendor_id`). ATM dengan kuota
  tidak diketahui **dilewati** dan dihitung. Satu tx; satu `audit_logs` per ATM yang di-reset.
  Respons: `{ "reset_count": n, "skipped_count": m }`.
- FR4.3 Peran: ATM-SPV / BRANCH-ATM-SPV / ADMIN — dicek di route **dan** service. Langsung berlaku (bukan maker-checker, keputusan #9).

### FR5 — API (flat JSON, pola handler ATM tetangga)
Vendor Request (`/api/v1/vendor-requests`, roles mengikuti maker/checker set yang ada):
- `POST /{id}/complete` (maker) body `{ "results": [{ "terminal_id": "...", "result": "success|failed" }] }` → 200 request.
- `POST /{id}/complete/approve` (checker) → 200 `{ ...request, "over_quota_terminals": ["..."] }`.
- `POST /{id}/complete/reject` (checker) body `{ "reason": "..." }` → 200 request.
- `GET /{id}` tambah per item: `completion_result` (`success|failed|null`), `visit_remaining` (int|null),
  `visit_quota_total` (int|null), `is_over_quota` (bool, dari kunjungan bila sudah `completed`).

Kuota (`/api/v1/atm-visit-quotas`, baru):
- `GET /atms/{terminalId}` (viewer roles Vendor Request) → `{ terminal_id, quota_known, quota_total, remaining, over_quota, package_code, reset_at, reset_by, visits: [...] }`
  — `visits` = kunjungan periode berjalan (termasuk yang dibatalkan, ditandai).
- `POST /atms/{terminalId}/reset` (checker) → 200 kuota baru.
- `POST /vendors/{vendorId}/reset` (checker) → 200 `{ reset_count, skipped_count }`.
- `POST /visits/{visitId}/cancel` (checker) body `{ "reason": "..." }` → 200 kunjungan.

Forecast Browser: `GET /forecast` tambah per baris `visit_remaining` (int|null) dan `visit_quota_total` (int|null).

Error: 400 validasi · 403 role / self-approval · 404 tidak ada · 409 transisi/kunjungan tidak valid · 422 kuota tidak diketahui.

### FR6 — UI CompanyPortal
- FR6.1 **Detail Vendor Request** (`approved`): tombol "Laporkan selesai" (maker) → dialog daftar ATM distinct, default
  semua "Berhasil", toggle "Gagal" per ATM → ajukan.
- FR6.2 **Detail** (`completion_pending`, checker ≠ pelapor): tombol "Setujui laporan" / "Tolak laporan" (alasan wajib).
  Dialog setujui menampilkan **peringatan** daftar ATM berhasil yang akan melebihi kuota (`visit_remaining <= 0`) —
  boleh tetap disetujui. Setelah approve, toast berisi `over_quota_terminals` bila ada.
- FR6.3 Badge per ATM di detail: "Berhasil" / "Gagal" dan **"Kelebihan kuota"** (teks + ikon, bukan warna saja) — dilihat ATM-USER pembuat laporan.
- FR6.4 Status baru `completion_pending` → label "Menunggu persetujuan laporan"; `completed` → "Selesai". Filter status di list ikut.
- FR6.5 **Profil ATM**: kartu "Kuota kunjungan" — paket, kuota, sisa, kelebihan, reset terakhir; daftar kunjungan periode berjalan;
  tombol "Reset kuota" dan "Batalkan kunjungan" (alasan wajib) hanya untuk checker. Kuota tidak diketahui → teks "Paket tidak dikenali".
- FR6.6 **Forecast Browser** tabel detail: kolom "Sisa kunjungan" (`sisa/kuota`, "—" bila null, penanda bila ≤ 0).
  Ringkasan vendor × region: tombol "Reset kuota vendor" (checker saja) dengan dialog konfirmasi → tampilkan `reset_count`/`skipped_count`.

## Non-functional
- Semua penulisan di primary dalam satu tx bersama `audit_logs` (Sec 5/6); `GET /atm-visit-quotas/atms/{id}` boleh replica;
  pembacaan untuk dialog approve memakai primary (read-after-write).
- Kolom angka di UI `tabular-nums`, rata kanan (Sec 13). Tidak ada uang di fitur ini.
- Timestamp `timestamptz` UTC; tampil Asia/Jakarta. "Paket aktif hari ini" dihitung dengan tanggal WIB.
- Reset massal satu vendor (ratusan ATM) ≤ 3 s.

## Data model (migrasi baru `021_atm_visit_quota.sql`; nama tabel diajukan ke CLAUDE.md Sec 3, grup ATM)
```sql
-- status baru
ALTER TABLE vendor_requests DROP CONSTRAINT vendor_requests_status_chk,
  ADD CONSTRAINT vendor_requests_status_chk CHECK (status IN
  ('draft','pending_approval','approved','rejected','processing','completed','failed','cancelled','completion_pending'));
ALTER TABLE vendor_requests
  ADD COLUMN completion_submitted_by bigint REFERENCES users(id),
  ADD COLUMN completion_submitted_at timestamptz,
  ADD COLUMN completion_approved_by  bigint REFERENCES users(id),
  ADD COLUMN completion_approved_at  timestamptz,
  ADD COLUMN completion_rejected_by  bigint REFERENCES users(id),
  ADD COLUMN completion_rejected_at  timestamptz,
  ADD COLUMN completion_rejection_reason text;

-- hasil laporan per ATM (ditimpa saat pengajuan ulang)
CREATE TABLE vendor_request_atm_results (
  vendor_request_id bigint NOT NULL REFERENCES vendor_requests(id),
  terminal_id text NOT NULL,
  result text NOT NULL CHECK (result IN ('success','failed')),
  updated_at timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY (vendor_request_id, terminal_id));

-- sisa per ATM
CREATE TABLE atm_visit_quotas (
  atm_id bigint PRIMARY KEY REFERENCES atms(id),
  package_code text NOT NULL,          -- snapshot saat reset/pembuatan
  quota_total integer NOT NULL CHECK (quota_total > 0),
  remaining integer NOT NULL,          -- boleh negatif = kelebihan
  reset_at timestamptz NOT NULL DEFAULT now(),
  reset_by bigint REFERENCES users(id), -- NULL = dibuat otomatis oleh kunjungan pertama
  updated_at timestamptz NOT NULL DEFAULT now());

-- log kunjungan
CREATE TABLE atm_visits (
  id bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
  atm_id bigint NOT NULL REFERENCES atms(id),
  vendor_request_id bigint NOT NULL REFERENCES vendor_requests(id),
  quota_known boolean NOT NULL,
  is_over_quota boolean NOT NULL DEFAULT false,
  created_at timestamptz NOT NULL DEFAULT now(),
  cancelled_at timestamptz, cancelled_by bigint REFERENCES users(id), cancel_reason text,
  CONSTRAINT atm_visits_req_atm_uq UNIQUE (vendor_request_id, atm_id),
  CONSTRAINT atm_visits_cancel_chk CHECK ((cancelled_at IS NULL) = (cancelled_by IS NULL)
                                      AND (cancelled_at IS NULL) = (cancel_reason IS NULL)));
CREATE INDEX atm_visits_atm_created_idx ON atm_visits (atm_id, created_at DESC);
```
- `terminal_id` di hasil laporan tidak bisa di-FK (items memakai teks); ATM tanpa baris `atms` → hasil tersimpan,
  kunjungan **tidak** dibuat (tidak ada `atm_id`), dicatat di audit. `atm_visits` tanpa hard delete (soft-cancel).
- Tidak ada perubahan pada `package_frequencies`.

## Sec 4 #7 flags
- **Migrasi skema**: status CHECK baru + 7 kolom `vendor_requests` + 3 tabel baru → update CLAUDE.md Sec 3 + `docs/data-map.md` setelah apply.
- **Alur approval**: maker-checker baru pada laporan selesai (maker ≠ checker, cek role di route + service).
- **Delete**: tidak ada; pembatalan kunjungan = soft-cancel.
- **Deviasi Golden Rule #3**: reset kuota & pembatalan kunjungan langsung berlaku tanpa `approval_requests` (keputusan #9/#18),
  dengan audit dalam tx yang sama dan RBAC di route + service — pola sama dengan Role Management (Sec 12). Dicatat di `docs/decisions.md`.

## Acceptance criteria
1. PAKET 5 (ATM), request `completed` dengan ATM berhasil → `remaining = 4`; ATM gagal tidak berubah.
2. Kunjungan ke-6 dalam periode yang sama → `remaining = -1`, tampil sisa 0 + kelebihan 1, `is_over_quota = true`, approve tetap sukses.
3. Approve laporan dua kali / paralel → hanya satu kunjungan per ATM, `remaining` turun sekali.
4. Pelapor tidak bisa approve laporan sendiri (403); ATM-USER tidak bisa approve/reset/batalkan (403).
5. Laporan ditolak → status `approved`, kuota tetap; diajukan ulang dengan hasil berbeda menimpa hasil lama.
6. Reset per ATM → `remaining = quota_total = cr_frequency` paket aktif; `cr_frequency` tidak berubah.
7. Reset massal vendor → semua ATM vendor ter-reset, ATM paket tak dikenali masuk `skipped_count`.
8. Batalkan kunjungan periode berjalan → `remaining + 1`; kunjungan sebelum reset terakhir → 409.
9. Setiap aksi tulis `audit_logs` dalam tx yang sama; kegagalan audit me-rollback aksi.
10. Kolom "Sisa kunjungan" tampil di Forecast Browser; kartu kuota tampil di Profil ATM.

## Out of scope
- Laporan selesai dari VendorPortal; FLM; notifikasi in-app/email (diganti peringatan di layar + badge, modul 0.3 menyusul);
  penalti/tagihan kelebihan; layar admin `package_frequencies`; FK/validasi `package_code` ke `package_frequencies`;
  halaman khusus daftar kuota; status `processing`/`failed` (tetap tidak dipakai).
