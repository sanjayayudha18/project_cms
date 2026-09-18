# Feature Flow — Cash Count Vault Vendor

Sumber: URS v0.3 Phase 1 · To-be "Cash count Vault vendor" · FNC 002
Modul: `internal/cashcount` (proposal, belum approved) + `internal/notification`, `internal/document`, `internal/corebanking`

## Aturan
- Bulanan. Jadwal **acak / tidak berpola**, abaikan tanggal merah & hari libur, pertimbangkan ketersediaan PIC wilayah.
- Kategori risiko dari analisa saldo kelolaan + histori (escrow SIBS/MIS).
- Template BA: Vault ATM, Vault Cash, Valas + checklist parameter.
- Status per escrow: Complete · On-progress · Not Complete.

## Flow: penjadwalan & penugasan

```mermaid
flowchart TD
    A([Awal bulan]) --> B[Analisa saldo kelolaan vault<br/>dari escrow SIBS/MIS + histori]
    B --> C[Tentukan kategori risiko per vault]
    C --> D[Susun jadwal visit acak<br/>skip hari libur]
    D --> E[Pilih PIC wilayah yang tersedia]
    E --> F[Email notifikasi ke PIC]
    F --> G{PIC Accept?}
    G -->|Reject| H[(Simpan history reject)]
    H --> I{Tindak lanjut}
    I -->|Reschedule| D
    I -->|Ganti PIC| E
    G -->|Accept| J[Terbitkan Surat Tugas otomatis]
    J --> K([PIC unduh Surat Tugas])
```

## Flow: pelaksanaan & e-sign

```mermaid
flowchart TD
    A([PIC di lokasi vault]) --> B[Status: On-progress]
    B --> C[Isi BA digital - hasil hitung fisik per denom]
    C --> D[Kolom DSR auto-fill dari DSR vendor]
    D --> E[Hitung selisih otomatis]
    E --> F[Isi checklist kondisi vault]
    F --> G[Upload foto / dokumen pendukung]
    G --> H[E-sign vendor]
    H --> I[E-sign PIC bank]
    I --> J[Generate dokumen final<br/>BA + checklist + foto]
    J --> K[Status: Complete]
    K --> L([Unduh / cetak])
```

## Flow: rekap & rekonsiliasi 3 arah

```mermaid
flowchart LR
    A[Hasil cash count] --> R{Rekonsiliasi 3 arah}
    B[Saldo escrow H-1<br/>otomatis MIS/SIBS] --> R
    C[Hasil proofing<br/>input manual] --> R
    D[Nilai DSR] --> S
    R -->|Cocok| S[Rekap per vault ATM & Cash]
    R -->|Selisih| T[Tandai temuan untuk tindak lanjut] --> S
    S --> U[Ringkasan checklist]
    U --> V[Evaluasi performa vendor]
    V --> W([Dashboard progres & temuan per periode])
```

## Catatan / open questions
- E-sign tersertifikasi opsional (NFR) — vendor e-sign apa?
- Tabel `cash_count_schedules`, `cash_count_evidences` masih proposal (CLAUDE.md §3).
- Formula kategori risiko belum didefinisikan di URS.
