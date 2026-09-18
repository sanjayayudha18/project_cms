# Feature Flow — Pemenuhan Dana, Pengambilan Dana & Serah Terima

Sumber: URS v0.3 Phase 1 · ATM Cash Forecasting "Pemenuhan Dana", "Pengambilan Dana", "Surat Tugas dan Serah Terima"
Modul: `internal/replenishment`, `internal/approval`, `internal/document`, `internal/notification`

## Data
- **Pemenuhan dana** (Cabang / Cash Management): lokasi pengambilan, nominal per denom, tanggal, jam, vault penyedia dana
- **Pengambilan dana** (Vendor FLM): nama petugas, no. KTP, no. NIP, nama perusahaan, keperluan, nominal, nominal per denom, jumlah lembar, no. kendaraan, tanggal

## Flow

```mermaid
sequenceDiagram
    autonumber
    actor CM as Cabang / Cash Management
    participant SYS as CMS
    actor FLM as Vendor FLM
    actor MK as Maker
    actor CK as Checker

    Note over SYS: Order ATM sudah approved
    CM->>SYS: Input pemenuhan dana<br/>(lokasi, nominal/denom, tanggal, jam, vault)
    SYS-->>FLM: Notifikasi lokasi & jadwal pengambilan
    FLM->>SYS: Input data petugas & kendaraan
    SYS->>MK: Request review data pengambilan
    MK->>SYS: Submit (maker)
    SYS->>CK: Request approval
    alt Reject
        CK->>SYS: Reject + alasan
        SYS-->>FLM: Notifikasi revisi data
    else Approve
        CK->>SYS: Approve (checker ≠ maker)
        SYS->>SYS: Generate Surat Tugas
        SYS-->>FLM: Surat Tugas siap diunduh
        SYS-->>CM: Surat Tugas siap diunduh
        FLM->>CM: Datang ambil dana (bawa Surat Tugas)
        CM->>SYS: Verifikasi petugas, kendaraan, nominal
        CM->>SYS: Konfirmasi serah terima
        FLM->>SYS: Konfirmasi serah terima
        SYS->>SYS: Status: diserahterimakan + audit_logs
    end
```

## Catatan / open questions
- Maker & Checker di sini dari pihak bank atau vendor? URS: "harus mendapatkan persetujuan Maker dan Checker".
- No. KTP = data pribadi petugas vendor → masuk cakupan DGCC/TPRA (lihat CLAUDE.md §3a).
- Serah terima perlu e-sign atau cukup konfirmasi in-app?
