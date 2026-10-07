# Feature Flow — Upload DSR Harian & Rekap Keterlambatan

Sumber: URS v0.3 Phase 1 · ATM Cash Forecasting §2 "Pengelolaan Data Pendukung" · FNC 001, FNC 003
Modul: `internal/dsr`, `internal/notification`, `internal/export`

## Aturan
- DSR = laporan saldo fisik di vault/kantor FLM, wajib upload **setiap hari sebelum 09:00**.
- Upload idempotent per file hash (`import_jobs`).
- Target NFR: upload ≤ 30 detik/dokumen.
- Akhir bulan: report DSR telat/tidak dikirim per vendor → dasar penalty FLM.

## Flow: upload DSR harian

```mermaid
flowchart TD
    A([Vendor FLM login Vendor Portal]) --> B[Upload file DSR]
    B --> C{File hash sudah pernah diproses?}
    C -->|Ya| C1[Tolak: duplikat] --> Z([Selesai])
    C -->|Tidak| D{Validasi format & isi}
    D -->|Gagal| D1[Tampilkan error] --> B
    D -->|Lolos| E[(atm_dsr_uploads + atm_dsr_rows)]
    E --> F{Diterima sebelum 09:00?}
    F -->|Ya| G[Status: OK]
    F -->|Tidak| H[Status: TELAT]
    G --> I[(audit_logs)]
    H --> I
    I --> Z
```

## Flow: monitoring & notifikasi keterlambatan

```mermaid
flowchart TD
    A([09:00 setiap hari]) --> B[Cek vault yang belum upload DSR]
    B --> C{Ada yang belum?}
    C -->|Tidak| Z([Selesai])
    C -->|Ya| D[Notifikasi email / in-app<br/>ke vendor PIC & ATM Support]
    D --> Z
```

## Flow: rekap bulanan (dasar penalty)

```mermaid
flowchart LR
    A([Akhir bulan]) --> B[Generate report DSR telat / tidak dikirim]
    B --> C[Kolom: Tanggal Laporan · Vendor & Area Vault · Tanggal Terima · Status]
    C --> D[Export CSV / XLSX / PDF]
    D --> E([Dipakai sebagai acuan penalty FLM])
```

## Catatan / open questions
- "Tanggal Laporan" = tanggal posisi DSR, bukan tanggal kirim.
- Apakah hari libur/weekend mengubah deadline 09:00? (contoh URS: laporan Jumat diterima Sabtu 08:28 = OK, diterima Senin 13:03 = TELAT)
