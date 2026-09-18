# Feature Flow — Cash Count Selektif Mesin

Sumber: URS v0.3 Phase 1 · To-be "Cash count selektif Mesin" · FNC 002
Modul: `internal/cashcount` (proposal, belum approved)

## Aturan
- Dipicu oleh **instruksi pengawasan** untuk mesin ATM tertentu (bukan jadwal acak bulanan).
- Mekanisme sama dengan cash count vault: surat tugas → BA → checklist → foto → e-sign → dokumen final.
- Rekap untuk evaluasi performa vendor **level mesin**.

## Flow

```mermaid
flowchart TD
    A([Instruksi pengawasan]) --> B[Pilih ATM target]
    B --> C[Tentukan jadwal & PIC]
    C --> D[Email notifikasi ke PIC]
    D --> E{PIC Accept?}
    E -->|Reject| E1[(History reject)] --> C
    E -->|Accept| F[Terbitkan Surat Tugas]
    F --> G[PIC di mesin: isi BA hasil hitung fisik]
    G --> H[Checklist selektif mesin]
    H --> I[Upload foto]
    I --> J[E-sign vendor + PIC bank]
    J --> K[Generate dokumen final]
    K --> L[Rekap per mesin]
    L --> M([Evaluasi performa vendor level mesin])
```

## Catatan / open questions
- Siapa yang berhak membuat instruksi pengawasan (role)?
- Apakah perlu rekonsiliasi terhadap saldo mesin (journal/switching), atau cukup BA vs DSR?
