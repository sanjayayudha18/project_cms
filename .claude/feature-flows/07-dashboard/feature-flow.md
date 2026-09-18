# Feature Flow — Dashboard & Pelaporan

Sumber: URS v0.3 Phase 1 · To-be "Dashboard & Pelaporan" · FNC 003
Modul: dashboard read-only di atas `internal/forecast`, `internal/replenishment`, `internal/dsr`, `internal/cashcount`, `internal/export`

## Konten (FNC 003)
- **ATM Forecasting**: daily instruction (amount & term ID), rekap keterlambatan DSR
- **Cash Count**: daily progress pelaksanaan, monthly report
- Umum: status, aging, SLA, tren historis, indikator exception

## Flow

```mermaid
flowchart TD
    A([User login]) --> B{Role}
    B -->|ATM Support & Monitoring| C[Dashboard ATM Forecasting]
    B -->|Vendor Operations & Control| D[Dashboard Cash Count]
    B -->|Vendor| V[Hanya data kelolaan sendiri]

    C --> C1[Daily instruction: amount & term ID]
    C --> C2[Rekap keterlambatan DSR]
    D --> D1[Daily progress cash count]
    D --> D2[Monthly report cash count]
    D --> D3[Temuan & evaluasi performa vendor]

    C1 & C2 & D1 & D2 & D3 --> F[Filter: periode, vendor, vault, region]
    F --> G[Drill-down ke detail]
    G --> H[Export CSV / XLSX / PDF]
```

## NFR
- Load dashboard ≤ 3 detik (p95), data dari **read replica**.
- Responsive (bisa dipakai di mobile).

## Catatan / open questions
- REP.001 "Delivery Status Reporting" di URS menyebut "RFP transactions" — kemungkinan sisa template, perlu konfirmasi.
