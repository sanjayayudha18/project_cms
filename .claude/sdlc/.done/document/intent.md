# Intent: Penyimpanan dokumen (`documents`) — Phase 0.4

Author: user (product owner), ditulis bersama Claude. Status: **accepted 2026-10-07, DEFERRED ke Phase 3/5 (PO)** — spec ditulis saat fitur pemakai pertama (invoice / cash count) dimulai.
Stage: 1 Plan. Stage berikut: `spec.md` (setelah intent diterima).
Menyentuh Sec 4 #7: **skema (tabel baru `documents`, migrasi baru)** + **file upload (trust boundary)** → wajib AI-DLC penuh.

## Problem
Beberapa modul berikutnya perlu menyimpan file lampiran, tapi belum ada tempat penyimpanan bersama (`.kiro/steering/development-plan.md` 0.4):

- **Cash count** (Phase 5, `docs/requirements.md`): Berita Acara digital, checklist, **foto bukti**, dual e-sign; dokumen final bisa diunduh/dicetak.
- **Invoice** (Phase 3): vendor upload invoice + dokumen pendukung.
- **Escrow** (Phase 4): file batch Corebanking (ingest sudah diatur `import_jobs`, tapi file aslinya perlu disimpan).
- **CIT** (Phase 6): bukti serah terima (`cit_handover_evidences`).

Kondisi sekarang (dicek di kode 2026-10-07):
- Tabel `documents` **belum ada** (nama sudah tercantum di CLAUDE.md Sec 3, grup Core); `internal/document` belum ada.
- Upload yang sudah jalan menyimpan file ke disk lokal masing-masing: DSR → `FTP_DATA/DSR/` (`DSR_UPLOAD_DIR`, nama `<vendor>__<user>__<file>`), CSV master data → diproses di memori + `master_data_import_batches` (SHA-256).
- Env `GCS_BUCKET` ada di CLAUDE.md Sec 9 tapi belum dipakai kode. Prod di GCP (GCS, Sec 10).

## Proposed outcome
1. Satu modul `internal/document`: simpan file → dapat `document_id`; metadata (nama asli, tipe, ukuran, checksum SHA-256, pengunggah, waktu, entitas pemilik) di DB.
2. Backend penyimpanan bisa diganti lewat config: **direktori lokal di dev**, **GCS di prod**, tanpa ubah kode pemakai.
3. Unduh file lewat endpoint yang mengecek hak akses (vendor hanya dokumen miliknya; internal sesuai role).
4. Validasi di batas upload: ukuran maksimum, tipe file yang diizinkan (cek magic bytes, bukan hanya ekstensi), nama file aman (tidak ada path traversal).
5. Modul ini dipakai pertama kali oleh fitur yang dipilih di spec (lihat open question 7).

## Affected users and systems
- Users: Vendor (upload invoice/bukti), operator internal (cash count, CIT), auditor (unduh).
- Systems: `backend/` (migrasi, `queries/`, `internal/document`, handler), `pkg/config` (storage env), `docker-compose`/deploy (volume dev, bucket GCS), frontend pemakai.

## Constraints
- Stack tetap (Sec 2). Klien GCS (`cloud.google.com/go/storage`) = **dependency baru → butuh persetujuan** (Golden Rule #1); bisa ditunda dengan hanya membangun backend lokal dulu.
- Tidak ada hard delete (Sec 12): dokumen dinonaktifkan, file fisik tidak langsung dihapus.
- Idempotensi per checksum bila relevan (Sec 5); jangan duplikasi `import_jobs` — file ingest tetap dilacak di sana, `documents` hanya menyimpan file/metadata.
- Metadata tulis ke primary; daftar dokumen boleh dari replica (Sec 6).
- Audit setiap upload / nonaktifkan (`audit_logs`, Sec 5).
- Jangan log isi file atau URL bertanda tangan (signed URL).

## Open questions (dijawab PO sebelum spec)
1. **Batas ukuran & tipe file**: maksimum berapa MB? Tipe yang diizinkan (PDF, JPG/PNG, XLSX, CSV)?
2. **GCS sekarang atau nanti**: setujui dependency klien GCS sekarang, atau bangun backend lokal saja dulu sampai deploy?
3. **Akses unduh**: lewat backend (stream) atau signed URL GCS berbatas waktu?
4. **Kepemilikan**: dokumen ditautkan ke entitas lewat kolom generik (`owner_type` + `owner_id`) atau FK per tabel pemakai?
5. **Retensi**: berapa lama dokumen disimpan (kebutuhan audit/regulasi bank)?
6. **Scan antivirus**: perlu dipindai sebelum disimpan/diunduh? Ada layanan perusahaan yang harus dipakai?
7. **Fitur pemakai pertama**: modul ini belum punya pemakai di Phase 0–2 — dibangun sekarang, atau ditunda sampai Phase 3/5 yang membutuhkannya (YAGNI)?
8. **Migrasi DSR**: upload DSR yang ada pindah ke `documents`, atau tetap di `FTP_DATA/DSR/` (kontrak nama file dipakai ETL Python)?

## Resolved decisions (PO, 2026-10-07)
- **Q7**: **tunda** — belum ada pemakai di Phase 0–2 (YAGNI). Phase 0 dianggap selesai tanpa 0.4.
- **Q1**: maks **10 MB**; PDF, JPG, PNG, XLSX, CSV; tipe dicek lewat magic bytes.
- **Q2/Q3**: backend **lokal dulu** (satu implementasi); dependency GCS diajukan saat deploy. Unduh di-stream lewat backend dengan cek akses.
- **Q4**: kepemilikan generik `owner_type` + `owner_id`.
- **Q5**: disimpan tanpa batas sampai kebijakan retensi bank ditentukan; hanya soft-disable.
- **Q6**: tanpa antivirus dulu — dicatat sebagai risiko di spec.
- **Q8**: DSR tetap di `FTP_DATA/DSR/` (kontrak nama file ETL Python tidak diubah).

## Out of scope
- E-sign (Phase 5), pembuatan PDF Berita Acara / export XLSX/PDF (0.5, ditunda).
- Versi dokumen / editing.
