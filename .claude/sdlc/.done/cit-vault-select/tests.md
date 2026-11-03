# Tests: Pemenuhan vault ACM — bagi nominal request ke vault + tiket pemenuhan vault (v3)

Status: **green 2026-10-13** (Stage 4). Input: `spec.md` v3 (FR1–FR9, NFR1–NFR5), `plan.md` v3 (S0–S8). Belum di-commit.

## Commands (run 2026-10-13)
| Command | Result |
|---|---|
| `psql … -f backend/migrations/030_vault_allocation.sql` (dev) | COMMIT (S1); unik parsial `vendor_request_vault_plans_live_uq` dibuat di S3 setelah cek 0 duplikat |
| psql CHECK/FK di tx rollback (S1) | kelipatan, denom, FK komposit vault↔branch, tier 3 wajib urgent, satu tiket per (request, vault), inactive butuh `deactivated_at`, format + unik region, cascade amounts — semua ditolak/diterima sesuai |
| `cd backend && sqlc generate` (v1.31.1) + fix `UserLeafe` | hanya file query terkait berubah |
| `cd backend && go build ./... && go vet -tags integration ./...` | clean |
| `cd backend && go test ./...` | all ok |
| `cd backend && DATABASE_URL=… go test -tags integration ./...` | all packages ok, **0 skip** (8 test 2.2b yang di-skip di S3 aktif lagi sejak S4) |
| coverage `internal/service` (integration) | **81.3 %**; fungsi baru 65–100 % (sisa = cabang error DB) |
| `golangci-lint run --build-tags integration ./...` | 0 issues |
| sisa data test di dev | 0 baris `vendor_request_vault_allocations` / `_tickets` / `acm_area_regions`; 1 plan dev lama tetap |
| `pnpm --dir frontend/CompanyPortal-Vite run test` / `lint` / `build` | 127 files / 1080 tests / clean / built |
| `pnpm --dir frontend/VendorPortal-Vite run test` / `lint` / `build` | 18 files / 128 tests / clean / built |

## Test → requirement
| Test | Covers |
|---|---|
| `vault_allocation_test.go` (7, table-driven) | FR4.2 eligible, dobel, nominal bulat > 0, denom, kelipatan, urgent 10–500 / hanya bila urgent, tier 3 wajib urgent; FR4.3 kurang 1 rupiah → error, lebih tanpa alasan → error, lebih + alasan OK, denom tak dipesan = lebih; FR5.3 kapasitas, tidak diketahui ≠ 0; FR3.4 urutan kandidat; NFR3 tanpa float |
| `vault_ticket_integration_test.go` | FR6.1/FR6.2: `_001` per vault, kirim ulang tetap, vault dilepas nonaktif, request lain vault+tanggal sama `_002`, vault kembali = nomor lama aktif; NFR4 advisory lock |
| `TestIntegration_VaultPlan_FullFlow` | FR2.1 plan per region saat approve; FR4.1/FR4.2 simpan replace-all, RBAC non-anggota 404; FR4.3 kurang/lebih/buffer 422; FR4.4 snapshot; FR4.5 submit + tiket; FR6.3 reject tidak ubah tiket; FR4.6 approve → `vault_review`; ATM-SPV reject → draft |
| `TestIntegration_VaultPlan_CapacityAndTicketsAcrossRequests` | FR5.2/FR5.3 alokasi request lain mengurangi kapasitas, plan sendiri tidak; cancel tidak ubah tiket dan kapasitas kembali |
| `vault_plan_guards_integration_test.go` | FR8.2 RBAC, maker ≠ checker, transisi tidak sah → 409 |
| `TestIntegration_VaultFlow_RegionWithoutAreaGetsPlanOnMapping` (+ `vault_flow_…`) | FR1.3 map region → plan untuk request menunggu; FR1.4 warning; FR1.5 cabang tidak membuat plan; pindah region tidak memindah plan |
| `TestAcmAreaAdmin_Regions_Integration` | FR1.1 unik region (409 konflik + pemegang), FR1.2 audit dalam tx, disable melepas region |
| `TestBuildPartyContents`, `TestSameContent` | FR7.1–FR7.4: dua vault satu branch = satu pihak dua tiket; replenish melihat semua vault; vault melihat cabang pengisi; tanpa saldo/tier/urgent/buffer |
| `TestIntegration_VendorParty_SendAndResync` | FR7 isi pihak dari alokasi; resync: nominal berubah → hanya pihak yang melihatnya; vault pindah branch → pihak lama withdrawn |
| `vendor_order_integration_test.go` (ScopeAndNoLeak, VaultRejectBackToAcm, ReturnEditResend, …) | FR7.5 vendor vault tolak → plan satu-satunya draft, tiket tetap; edit request tidak menghapus alokasi, submit lebih tanpa alasan → 422; scope vendor, `vault_count` |
| handler tests (`vault_plan_handler_test.go`, `admin_acm_area_handler_test.go`, `vendor_request_vault_handler_test.go`) | API: `PUT …/allocations` (route lama 404), kandidat `{branches}`, detail baru, `{plan}`, region routes + 409 `conflicts`, warnings `{requests_without_area}` |
| `VaultPlan.test.tsx` (CompanyPortal, 11) | FR9.1: tambah/hapus baris, kelipatan, angka nol di depan dibuang, kurang memblokir submit (alasan tertulis), lebih → kolom alasan buffer wajib, tiket per baris, urgent membuka tier 3, kapasitas warning; FR9.2 panel ATM-SPV; FR9.5 status berlabel |
| `AdminAcmAreasPage.test.tsx` (CompanyPortal, 4) | FR9.3 region milik area lain terkunci, simpan kode, kode lama tetap tampil; FR1.4 daftar request menunggu |
| `OrdersPages.test.tsx` (VendorPortal, 9) | FR9.4 pihak vault (tiket vault, alamat, cabang pengisi), pihak replenish (ATM + semua vault request), kolom jumlah per peran |

## Catatan
- **Manual browser check: outstanding (user, Golden Rule #10).** Langkah: (1) jalankan `backend/scripts/dev/seed_acm_users.sql` (memetakan semua
  region bebas ke "Area ACM Dev"), atau Pengaturan → Area ACM → Kelola "Area ACM Dev" → centang region → Simpan Region; (2) ATM-SPV approve request
  → Penetapan Vault: pilih branch → vault, isi nominal, kurang/lebih/buffer, Simpan, Kirim; (3) ACM-SPV approve; (4) ATM-SPV review di
  detail Vendor Request; (5) VendorPortal Orders sebagai user cabang pengisi dan user branch vault.
- Seed dev `backend/scripts/dev/seed_acm_users.sql` masih memetakan **cabang** (lihat `review.md` R1).
- Build Docker belum dijalankan. `-race` tidak tersedia lokal (CGO_ENABLED=0).
