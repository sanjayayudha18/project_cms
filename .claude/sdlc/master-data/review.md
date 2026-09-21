# Review — master data (Fase 0–6)

Dua review independen (security + Go correctness) atas commit `2173a5a..HEAD`, plus verifikasi manual terhadap kode.

## Temuan yang diperbaiki
| # | Sev | Temuan | Perbaikan |
|---|-----|--------|-----------|
| 1 | High | `Submit` tidak atomik: gagal routing approval meninggalkan baris `pending` yatim yang memblokir entitas (pending-unique) | `MasterDataChangeService.abandon` menolak baris itu bila routing/link gagal (+ tes) |
| 2 | High/Med | Batch apply gagal → head tetap `approved`; `FindOpen` menganggap batch masih terbuka, file yang sama tidak bisa di-upload ulang | Saat batch di-rollback semua baris ditandai `stale` (+ assert integrasi) |
| 3 | Med | `Reject` menelan error `GetByID`; baris batch bisa tertinggal `pending` | Error dikembalikan |
| 4 | Med | `ErrVendorVaultCodeConflict`, `ErrATMNotFound`, `ErrATMInvalidReference` saat approve jadi 500 | Dipetakan ke 409 di `approval_handler.handleError` |
| 5 | Low | `finish` memakai maker pemanggil, bukan maker batch, saat submit approval diulang | Memakai `batch.MakerID` |

## Diperiksa, tanpa masalah
Batas upload (`MaxBytesReader` + `LimitReader`), SQL parameterized, formula injection (`csvSafe` / satu apostrof), race upload sama (advisory lock + re-`FindOpen`), RBAC dua lapis (`masterDataAdmin` + `authorizeMasterData*`), `GET /approvals/{id}/master-data` hanya maker atau approver dengan step pending (tanpa IDOR).

## Risiko sisa (diterima / tindak lanjut)
- **Tidak ada re-apply** untuk change `approved` yang gagal apply karena error transien (bukan stale/konflik); approval sudah final sehingga `Approve` kedua → 409. Perlu operasi "re-apply" idempoten bila ini terjadi di produksi.
- **Beban impor**: maks 10.000 baris/5 MiB, "before" dibaca per baris, satu transaksi dengan advisory lock; belum ada rate limit per-user di `/import`. Pertimbangkan turunkan batas (~2.000) + batch read + rate limit.
- `Reject` (batch) belum transaksional (retry aman/idempoten).
- 403 vs 404 pada endpoint detail approval membocorkan keberadaan id (rendah).
- Pending badge FE hanya membaca 100 pending terbaru; create pending tidak ter-badge.
- UI belum diverifikasi visual di browser (hanya tes komponen, tsc, biome).
