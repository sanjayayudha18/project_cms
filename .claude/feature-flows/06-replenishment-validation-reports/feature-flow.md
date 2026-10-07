# Feature Flow — Validasi Realisasi Replenishment & Report

Sumber: URS v0.3 Phase 1 · Business Rules ATM Cash Forecasting
Modul: `internal/replenishment`, `internal/export`

## Flow: klasifikasi realisasi vs order

```mermaid
flowchart TD
    A([Data realisasi pengisian ATM]) --> B{Ada order untuk ATM ini?}
    B -->|Tidak| X[Pengisian tanpa order]
    B -->|Ya| C[Hitung selisih hari kerja<br/>tanggal realisasi vs jadwal<br/>- disesuaikan hari libur]
    C --> D{Selisih hari}
    D -->|0| E[Sesuai jadwal]
    D -->|1-2 hari lebih awal| F[Pengisian maju]
    D -->|1-2 hari lebih lambat| G[Pengisian mundur]
    D -->|lebih dari 2 hari / tidak diisi| H[Pengisian tidak dilakukan]
    E --> I{Nominal = order?}
    F --> I
    G --> I
    I -->|Tidak| J[Masuk report amount tidak sesuai order]
    I -->|Ya| K([Simpan hasil klasifikasi])
    J --> K
    H --> K
    X --> K
```

## Daftar report

```mermaid
flowchart LR
    R([Menu Report]) --> R1[Refund per ID<br/>filter vendor / denom]
    R --> R2[Amount pengisian ≠ order<br/>+ detail ID]
    R --> R3[Transaksi tarik & setor]
    R --> R4[Profile ATM<br/>ID, lokasi, denom]
    R --> R5[Order vs Transaksi]
    R --> R6[Report User]
    R --> R7[Log User - last login]
    R --> R8[Trip / Realisasi pengisian<br/>per vendor per ID, tertinggi]
    R1 & R2 & R3 & R4 & R5 & R6 & R7 & R8 --> EX[Export CSV / XLSX / PDF]
```

## Catatan / open questions
- Sumber data realisasi (transaksi mesin / journal) dari mana — Corebanking, switching, atau DSR?
- Report dibaca dari read replica.
