# Review: Replenishment — realisasi pengisian ATM vs order (Phase 2.2c)

Status: **2026-10-14** (Stage 5) — R1–R5 **fixed** (R3 dengan keputusan PO), tidak ada Important terbuka. Reviewer: Claude (bugs / security / compliance pass atas diff
belum di-commit, input `spec.md`, `plan.md`, `tests.md`). Code owner merge, bukan Claude.

## Important

| # | Area | Temuan | Usulan perbaikan |
|---|---|---|---|
| R1 | Security / auth (Sec 4 #7) | `SubmitRealization` (`service/vendor_order_realization.go:58`) hanya mengecek pihak yang dibuka ada dalam scope. Status pihak tidak dicek, dan `checkReportsWholeRequest` hanya berlaku untuk user ber-branch. User **vendor-wide** vendor X yang memegang pihak replenish berstatus `withdrawn`, atau pihak replenish salah satu dari beberapa vendor dalam request, bisa melapor realisasi **semua ATM**, termasuk ATM kelolaan vendor Y. | Pihak harus `accepted`. `checkReportsWholeRequest` berlaku untuk semua scope: setiap pihak replenish non-`withdrawn` harus `vendor_id = sc.vendorID` (dan branch = `sc.branchID` bila ada). Tambah tes integration lintas vendor. |
| R2 | Bug (job) | `replenish_classify/job.py`: pengisian ITM yang pindah antar tiket pending melanggar `UNIQUE (itm_replenish_id)`. Contoh: tiket B baru dengan jadwal lebih dekat, padahal baris A masih memegangnya. Urutan loop tidak terjamin, sehingga seluruh job gagal di setiap run. Selain itu, baris `void` tetap memegang `itm_replenish_id`. Karena `void` final, pengisian itu dikecualikan selamanya: tidak dicocokkan ke tiket lain dan tidak jadi `no_order`. | Di awal run: `UPDATE … SET itm_replenish_id = NULL WHERE NOT is_final`. Saat void, `itm_replenish_id = NULL`. Tes: pengisian pindah ke tiket lain + tiket di-void. |
| R3 | Spec ≠ kode | FR3.2: request `rejected`/`vendor_rejected` seharusnya membuat baris belum final menjadi `void`. Kode hanya mem-void tiket nonaktif / request batal. Request yang ditolak sementara (revisi lalu dikirim ulang) memang lebih aman tidak di-void (void = final). Tetapi request yang tidak pernah kembali tetap `pending` selamanya. | **Butuh keputusan PO**: void hanya bila jendela tertutup dan request tidak berstatus instruksi. Spec FR3.2 ikut diubah. |
| R4 | Spec gap (FR3.6) | Laporan vendor tidak punya batas waktu (FR2.7), padahal baris final tidak pernah dihitung ulang. Akibatnya laporan yang masuk setelah H+2 tidak pernah mengisi `vendor_amount` / `vendor_mismatch`. Ini kasus umum, sehingga flag "beda laporan vendor" hampir selalu kosong. | Job memperbarui **hanya** kolom vendor (`vendor_mismatch`, `replenish_classification_amounts.vendor_amount`) pada baris final yang laporannya lebih baru dari `classified_at`. Kelas dan offset tetap final. |
| R5 | Security | CSV export (`handler/replenishment_handler.go` `writeCSV`) tidak menetralkan formula injection, padahal nama vendor dan cabang diketik admin. `service.csvSafe` sudah dipakai export master data. | Ekspor `csvSafe` dan pakai di `writeCSV`. Angka negatif (`-10000000`) tetap lolos lewat `plainNumberRe`. |

## Perbaikan (2026-10-14, belum di-commit)
| # | Perbaikan | Bukti |
|---|---|---|
| R1 | `SubmitRealization`: pihak harus `accepted`; `checkReportsWholeRequest` untuk semua scope — setiap pihak replenish non-`withdrawn` = vendor user (+ branch bila ber-branch) | `TestIntegration_VendorOrder_Realization`: user vendor lain dengan pihak replenish sendiri → ValidationError; pihak `withdrawn` → ErrInvalidTransition |
| R2 | Job: baris pending melepas `itm_replenish_id` sebelum matching; `void` juga melepas | `test_fill_moves_and_rejected_request_voids_after_window`: pengisian pindah dari tiket nonaktif ke tiket request lain |
| R3 | PO pilih "void saat jendela tutup": request di luar status instruksi tetap `pending` selama jendela terbuka, lalu `void`. Spec FR3.2, CLAUDE.md Sec 12, `docs/decisions.md` diperbarui | tes yang sama: `vendor_rejected` → pending (Kamis) → void (Kamis berikutnya), pengisian lepas → `no_order` |
| R4 | Job `_refresh_vendor_columns`: laporan dengan `submitted_at` > `classified_at` pada baris final → hanya `vendor_amount` + `vendor_mismatch`; logika beda dipindah ke `rules.is_vendor_mismatch` | `test_classify_pending_then_final_idempotent` (laporan telat → `not_done` tetap, mismatch + nominal vendor terisi), `test_vendor_mismatch` |
| R5 | `csvSafe` → `CSVSafe` (exported), dipakai di `writeCSV` | `TestReplenishmentHandler` trips CSV: nama vendor `=HYPERLINK(1)` → `'=HYPERLINK(1)`; diff negatif tetap angka |

Tes ulang: Go unit + integration (0 skip) + golangci-lint 0; Python 77 OK; frontend tidak berubah.

## Nits
- N1: Kontrak notifikasi Python↔Go (K3) dicek manual: kolom `notifications` / `notification_emails` sama dengan `InsertNotification` / `InsertNotificationEmail`, judul ≤ 120, body ≤ 1000. Belum ada tes silang otomatis.
- N2: `ReportReplenishTrips` ikut menghitung kecocokan sementara dari baris `pending` (bisa berubah sampai final).
- N3: Nominal di JSON = angka `int64` (pola 2.2b `DenomAmount`), bukan string desimal QC F7. F7 hanya untuk `numeric`/float; batas presisi JS 9e15 IDR tidak relevan.
- N4: BRANCH-* tidak disaring per cabang di layar Replenishment. Ini sama dengan list Vendor Request (dicatat di plan S6), tetapi FR4.3 perlu dibaca begitu.
- N5: Coverage `VendorOrderHandler.ListRealizations` 53,8 % (cabang error).
- N6: `LOOKBACK_DAYS = 31` (ponytail): job yang mati > 31 hari tidak melihat ITM lama lagi.

## Definition of Done (CLAUDE.md Sec 11)
- [x] Sesuai requirement + module/table map (031 di Sec 3); R3/R4 fixed, spec diperbarui
- [x] Auth path + scoped RBAC (R1 fixed)
- [x] Maker-checker + audit: laporan selesai tetap maker-checker (maker vendor, checker ATM-SPV), audit dalam tx; holidays = deviasi GR#3 terdokumentasi; export diaudit; job → `retry_audit_logs`
- [x] Read di replica, write di primary
- [x] Uang `bigint`, tanpa float (Go int64, Python int/Decimal, frontend digit string → integer); timestamptz
- [x] Tes hijau termasuk auth/RBAC/uang (`tests.md`)
- [x] Tanpa secret hardcoded; `.env.example` scheduler diperbarui (`RETRY_REPLENISH_CLASSIFY_START_DATE`, `RETRY_NOTIFY_EMAIL_ENABLED`)
- [x] Response flat JSON seperti handler tetangga
- [ ] Build Docker — belum dijalankan

## Gates
- Important R1–R5 ditutup 2026-10-14; tes ulang hijau.
- Manual browser check + uji ITM dev: **outstanding (user)**.
- Commit hanya saat user bilang "commit"; merge oleh code owner.
