# Feature Flow — Rekonsiliasi Invoice Vendor FLM

Sumber: URS v0.3 Phase 1 · ATM Cash Forecasting §2 "Pembayaran invoice vendor FLM"
Modul: `internal/invoice`, `internal/reconciliation`, `internal/approval`

## Aturan
- CMS hanya **validasi & approve**, TIDAK mengeksekusi pembayaran.
- Upload idempotent per file hash.

## Flow

```mermaid
flowchart TD
    A([Vendor FLM]) --> B[Upload invoice ATM + dokumen pendukung]
    C([Tim internal CIMB Niaga]) --> D[Upload data referensi:<br/>ATM aktif, ATM terminasi, harga,<br/>paket trip, kategori VIP/Industri/Regular]
    B --> E[(invoice_uploads + invoice_items)]
    D --> F[(data referensi invoice)]
    E --> G[Rekonsiliasi otomatis]
    F --> G
    G --> H[(invoice_reconciliation_results)]
    H --> I{Ada selisih?}
    I -->|Tidak| M
    I -->|Ya| J[Tampilkan selisih ke vendor]
    J --> K{Vendor sanggah?}
    K -->|Tidak| M
    K -->|Ya| L[Tim internal adjust manual<br/>+ alasan] --> L1[(audit_logs)] --> M
    M[Ajukan approval invoice] --> N{Checker approve?}
    N -->|Reject| J
    N -->|Approve| O([Status approved → diteruskan ke proses pembayaran di luar CMS])
```

## Catatan / open questions
- Tabel data referensi (harga, paket trip, kategori) belum ada di table map — perlu proposal.
- Periode invoice: bulanan?
