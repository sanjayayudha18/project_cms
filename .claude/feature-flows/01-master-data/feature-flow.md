# Feature Flow — Manajemen Vendor & Master Data

Sumber: URS v0.3 Phase 1 · To-be §1 "Manajemen Vendor & Master Data"
Modul: `internal/vendor`, `internal/vendorpic`, `internal/vault`, `internal/location`, `internal/atm`, `internal/assignment`, `internal/approval`, `internal/audit`, `internal/export`

## Cakupan data
- **Vendor**: identitas legal, status aktif/non-aktif, NPWP, kontak notifikasi (PIC)
- **Vault vendor**: alamat, koordinat (opsional), jam operasional, kapasitas, kategori (ATM / Cash)
- **PIC vendor**: nama, jabatan, no. kontak, email
- **Kelolaan**: vendor → mesin ATM dan/atau cabang/nasabah (lokasi, kapasitas)

## Flow: perubahan master data (maker-checker)

```mermaid
flowchart TD
    A([Maker buka menu Master Data]) --> B{Jenis aksi}
    B -->|Create / Edit / Hapus| C[Isi form]
    B -->|Import massal| D[Upload CSV/XLSX]
    D --> E{Validasi struktur & konten}
    E -->|Gagal| E1[Tampilkan error per baris] --> D
    E -->|Lolos| F[Preview data]
    C --> G[Submit]
    F --> G
    G --> H[(approval_requests: pending)]
    H --> I[Notifikasi ke Checker]
    I --> J{Checker review<br/>Checker ≠ Maker}
    J -->|Reject| K[Status rejected + alasan] --> L[Notifikasi ke Maker]
    J -->|Approve| M[Terapkan perubahan ke tabel master]
    M --> N[(audit_logs: siapa, kapan, before/after, ip)]
    K --> N
    N --> O([Selesai])
```

## Flow: pencarian & export

```mermaid
flowchart LR
    A([User]) --> B[Cari & filter data master]
    B --> C{Export?}
    C -->|Ya| D[Pilih format CSV / XLSX / PDF]
    D --> E[(export_jobs)] --> F[Download file]
    C -->|Tidak| G[Lihat detail / unduh dokumen]
```

## Catatan / open questions
- Hapus = soft-delete (`is_active=false` + `deleted_at`), sesuai pola admin CRUD yang sudah ada.
- Field kelolaan cabang/nasabah belum ada di tabel `vendor_assignments` — perlu dicek.
