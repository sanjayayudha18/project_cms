# Checklist Cek Browser — Impor CSV Data Master (Task 16)

> Dijalankan **user** (Golden Rule #10). Agent hanya menyusun daftar ini; belum ada langkah yang dijalankan atau dicentang. Centang `[x]` dan catat temuan setelah dicoba.
> Rujukan: `requirements.md` (Req 2.6, 4.2, 5.6, 7.1, 8.1, 8.2), `design.md`, `ui-mock.html`.

## 1. Persiapan

- [ ] Backend (host shell, dari `backend/`): `DATABASE_URL` ke `localhost:5432` (bukan `host.docker.internal`) dan `REDIS_URL=redis://localhost:6379`, lalu `go run ./cmd/api` (:8080). Cek `go run ./cmd/migrate status`: `036` harus applied.
- [ ] CompanyPortal: `pnpm --dir frontend/CompanyPortal-Vite run dev` (:5173). VendorPortal (`pnpm --dir frontend/VendorPortal-Vite run dev`, :5174) tidak dibutuhkan spec ini; boleh dilewati.
- [ ] Redis hidup di `localhost:6379`.
- [ ] Akun (ambil dari seed `backend/migrations/002_baseline_seed.sql` / data dev; **jangan tulis password di file**):
  - `maker`: ADMIN atau ADMIN_PARAM
  - `checker`: user yang menjadi approver step `master_data` untuk maker itu (lihat `approval_policies` / `approval_steps.assigned_approver_id`; kebijakan `master_data` satu level)
  - `nonadmin`: satu user non-admin (mis. ATM-USER)
- [ ] CSV kecil per entitas: buka `/settings/admin/master-data-io`, pilih entitas, lalu **Unduh templat** (header benar) atau **Ekspor data ini** (isi nyata). Ubah 2–3 baris di spreadsheet, simpan sebagai CSV (hati-hati Excel merusak nol di depan/format tanggal).
  - Skenario file: (a) baru (kolom `id` kosong); (b) ubah field (isi `id`); (c) hanya `is_active` (aktifkan/nonaktifkan); (d) tanpa perubahan; (e) berisi kesalahan (kode kosong / kolom salah); (f) >200 baris untuk langkah 6 (±250 baris, kode unik; batas 2.000 baris, 5 MiB).

## 2. Impor tiap entitas sampai diterapkan

Login `maker`. Ulangi blok di bawah untuk: [ ] Vendor · [ ] Cabang vendor · [ ] Vault vendor · [ ] PIC vendor · [ ] ATM · [ ] Kelolaan ATM.

- [ ] Judul "Impor CSV Data Master", eyebrow "Data master"; ada tab **Impor baru** dan **Riwayat impor**.
- [ ] Pilih entitas + file, klik **Periksa file** → ringkasan: Total, Baru, Ubah, Aktifkan, Nonaktifkan, Tanpa perubahan, Kesalahan; angka cocok isi file.
- [ ] Tabel perubahan: Baris, Kunci, Operasi, Perubahan (`field: sebelum → sesudah`); badge operasi ikon + label; teks "Baris tanpa perubahan tidak ditampilkan".
- [ ] Filter operasi menyaring tabel; paging 50 jalan (butuh >50 perubahan).
- [ ] Ganti entitas atau file → hasil periksa hilang, kembali ke langkah 2.
- [ ] **Unduh templat**, tautan **Ekspor data ini**, dan catatan aturan impor (pemisah, 5 MiB, 2.000 baris, kolom `id`, peringatan Excel, aturan data vs status aktif) masih ada.
- [ ] **Ajukan untuk persetujuan** → dialog "Ajukan impor untuk persetujuan?" berisi entitas, nama file, jumlah per operasi, kalimat "Data belum berubah sampai pengajuan disetujui."
- [ ] Konfirmasi **Ajukan** → panel hasil: nomor batch, jumlah baris, badge "Menunggu persetujuan", tombol **Lihat di riwayat** dan **Impor file lain**.
- [ ] Data belum berubah: cek layar entitas itu (atau SQL read-only).
- [ ] Login `checker` → `/approvals` → buka batch → **Setujui**.
- [ ] Login `maker` → Riwayat impor → batch **Diterapkan**; data entitas berubah sesuai file; Diputuskan (WIB) terisi.

## 3. File berisi kesalahan; file sama diajukan dua kali

- [ ] Periksa file skenario (e) → tabel kesalahan (Baris, Kolom, Kesalahan) ikon + teks; tabel perubahan hilang; **Ajukan untuk persetujuan** nonaktif.
- [ ] Ajukan file valid, lalu pilih file **identik** dan ajukan lagi (batch pertama masih Menunggu) → "File ini sudah diajukan sebelumnya. Tidak ada batch baru yang dibuat."; riwayat tetap 1 batch.
- [ ] (Bila bisa dipancing) 422 saat ajukan: periksa file valid, ubah data sehingga file jadi tidak valid sebelum klik Ajukan → kesalahan tampil seperti hasil periksa.

## 4. Riwayat dan detail batch

- [ ] Kolom: Batch, Entitas, Nama file, Baris, Diajukan oleh, Diajukan (WIB), Status, Diputuskan (WIB).
- [ ] Filter Entitas ("Semua entitas"), Status ("Semua status"), **Hanya milik saya** → hasil sesuai; URL ikut berubah (`h_entity`, `h_status`, `h_mine`).
- [ ] Paging 20 per halaman (perlu >20 batch; bila tidak ada, catat "tidak bisa diuji").
- [ ] Waktu dalam WIB (bandingkan dengan UTC di DB).
- [ ] Filter tanpa hasil → "Belum ada impor. Ajukan file lewat tab Impor baru."
- [ ] Klik nomor batch → dialog detail (`?tab=history&batch=ID`): ringkasan, jumlah per operasi, tabel perubahan (diff, status), paging 50 untuk batch besar; **Tutup** menghapus `batch` dari URL.
- [ ] `&batch=999999` → pesan "Batch tidak ditemukan".
- [ ] Batch sebelum 036 → Nama file "—".

## 5. Batch gagal diterapkan (stale) dan "Coba terapkan lagi"

Memancing **stale** butuh menulis DB dev. Lewat UI tidak bisa: selama batch masih menunggu, perubahan satu-record atau impor lain atas entitas yang sama ditolak 409 (`master_data_change_requests_pending_uq`).
- [ ] `maker` ajukan batch ≥3 baris `update` (mis. Vendor). Catat `entity_id` baris **selain baris pertama** batch itu:
  ```sql
  -- read-only
  SELECT id, entity_type, entity_id, op FROM master_data_change_requests WHERE batch_id = :b ORDER BY id;
  ```
- [ ] ⚠️ **Hanya bila Anda sendiri memutuskan** (agent tidak menjalankannya), ubah langsung satu kolom entitas itu supaya tidak lagi sama dengan snapshot `before`. Contoh untuk Vendor (`:id` = `entity_id` di atas; pakai kolom yang ada di snapshot `before` baris itu):
  ```sql
  -- ⚠️ TULIS ke dev DB, di luar maker-checker; hanya untuk data uji Anda.
  UPDATE vendors SET contact_phone = coalesce(contact_phone, '') || '9' WHERE id = :id;
  ```
- [ ] Lalu `checker` **Setujui** batch impor.
- [ ] Hasil: batch **Gagal diterapkan**; detail menampilkan banner "Tidak ada data yang diterapkan" + penyebab + kalimat file boleh diajukan ulang; baris penyebab berlabel **Gagal** + pesan, baris lain **Dibatalkan**; tidak ada data batch itu yang masuk.
- [ ] Ajukan ulang file yang sudah diperbaiki → batch baru terbentuk.

**Tombol "Coba terapkan lagi"** (hanya status `approved` / "Disetujui belum diterapkan"):
- [ ] Pada batch Diterapkan / Menunggu / Ditolak / Gagal diterapkan: tombol **tidak** ada.
- [ ] Status `approved` (commit gagal setelah disetujui) **tidak bisa dipancing dengan aman** lewat UI maupun SQL sederhana: mengubah status baris secara manual lalu retry bisa menerapkan batch dua kali. Jangan dipaksa; catat "tombol retry: hanya terverifikasi lewat test otomatis (task 4.3, 8, 12.3)".

## 6. Checker meninjau batch

- [ ] Ajukan batch >200 baris (skenario f). Login `checker` → `/approvals` → buka batch.
- [ ] Kepala batch: "Impor CSV · {entitas} · {nama file}", "Diajukan oleh {maker}", badge Baru/Ubah/Aktifkan/Nonaktifkan, catatan semua-atau-tidak.
- [ ] Semua baris tersedia lewat paging 50 (bukan lagi 200 pertama); total benar.
- [ ] **Lihat perubahan** per baris membuka diff (disclosure, `aria-expanded`).
- [ ] **Setujui** / **Tolak** bisa ditekan dari halaman 2 dan berlaku untuk seluruh batch.
- [ ] Tolak → batch **Ditolak**, data entitas tidak berubah.
- [ ] Review perubahan **satu-record** (non-impor) tampil seperti sebelumnya.
- [ ] `maker` membuka batch miliknya sendiri → **tidak bisa** Setujui/Tolak (403); data tidak berubah.

## 7. Notifikasi

- [ ] `maker` ajukan batch → bell `checker`: "Impor data master menunggu persetujuan", link → `/approvals`.
- [ ] Bila kebijakan >1 level: setelah step 1 disetujui, approver step berikutnya menerima notifikasi yang sama.
- [ ] Setelah diterapkan → bell `maker`: "Impor data master diterapkan"; ditolak → "Impor data master ditolak"; gagal → "Impor data master gagal diterapkan". Link membuka `/settings/admin/master-data-io?tab=history&batch=ID` dengan dialog batch yang benar.
- [ ] Ajukan file identik kedua kali (existing) → **tidak** ada notifikasi baru.
- [ ] SQL read-only (`:b` = ID batch):
  ```sql
  SELECT * FROM notifications
   WHERE entity_type = 'master_data_import_batch' AND entity_id = :b
   ORDER BY id;
  SELECT e.* FROM notification_emails e
   WHERE e.notification_id IN (SELECT id FROM notifications
          WHERE entity_type = 'master_data_import_batch' AND entity_id = :b)
   ORDER BY e.id;
  ```
  Harapan: `master_data_import.submitted` → baris notif **dan** email untuk approver; `applied` / `rejected` / `apply_failed` → baris notif untuk maker **tanpa** baris email. `SMTP_HOST` kosong → email berstatus `skipped` (normal). Nama kolom persisnya: lihat `025_notifications.sql`.

## 8. Peran non-admin

- [ ] `nonadmin` buka `/settings/admin/master-data-io` → halaman Forbidden.
- [ ] Kartu "Impor CSV Data Master" tidak muncul di Pengaturan untuk `nonadmin`.
- [ ] Dengan token `nonadmin`: `GET /api/v1/admin/master-data/import/batches` dan `/batches/1` → 403 (DevTools Network atau curl).

## 9. Aksesibilitas

- [ ] Tab "Impor baru" / "Riwayat impor": panah kiri/kanan berpindah tab; `aria-selected` benar.
- [ ] Indikator langkah: langkah aktif terbaca sebagai langkah saat ini (`aria-current="step"`); langkah selesai berikon centang + teks.
- [ ] **Lihat perubahan**: Enter/Spasi buka-tutup; status expanded terbaca.
- [ ] Dialog konfirmasi dan detail batch: fokus masuk dialog; **Esc** menutup; fokus kembali ke pemicu.
- [ ] Screen reader (NVDA/Narator) membaca status langkah, badge status batch, badge operasi, dan pesan error.
- [ ] Status batch dan operasi tidak hanya warna (selalu ikon + label); status tidak memakai merah brand.

## 10. Hal yang perlu Anda nilai (keputusan, bukan lulus/gagal)

- [ ] Langkah 1–2 wizard selalu terlihat bersamaan (beda dari `ui-mock.html`): terima / minta ubah?
- [ ] Tombol konfirmasi dialog berwarna merah `danger`: terima / minta netral?
- [ ] Isi dialog konfirmasi satu paragraf: cukup jelas?
- [ ] Bulan pada tanggal berbahasa Inggris (mis. "Nov"): terima / minta Indonesia?
- [ ] Di riwayat hanya nomor batch yang bisa diklik (baris tidak): terima?
- [ ] Angka di tabel perubahan tidak rata kanan (Req 3.3 meminta rata kanan): terima atau perbaiki?

## 11. Non-regresi singkat

- [ ] Halaman Export Data (`/exports`) masih membuka dan berfungsi.
- [ ] Approval dokumen lain di `/approvals` masih bisa dibuka, disetujui, ditolak.
- [ ] CRUD satu-record master data (edit vendor/ATM) masih menghasilkan 202 dan masuk approval.

---
**Hasil akhir:** _(diisi user)_ lulus / ada temuan: …
Task 16 di `tasks.md` dicentang oleh user setelah checklist ini selesai.
