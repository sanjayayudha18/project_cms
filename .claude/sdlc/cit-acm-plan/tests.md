# Tests: Penetapan branch vault per ATM oleh ACM (Phase 2.2a)

Status: **green 2026-10-08** (Stage 4). Input: `spec.md` (FR1–FR9, NFR1–NFR4), `plan.md` (S1–S8). Belum di-commit.

## Commands (run 2026-10-08)
| Command | Result |
|---|---|
| `psql … -f backend/migrations/027_acm_vault_assignment.sql` (dev) | COMMIT (S1) |
| `psql … -f backend/migrations/028_dsr_flow_comment.sql` (dev) | COMMIT — komentar kolom `dsr_daily_rows.flow` saja (S8) |
| `cd backend && sqlc generate` (v1.31.1) + fix `UserLeafe` | hanya file query terkait berubah |
| `cd backend && go build ./... && go vet -tags integration ./...` | clean |
| `cd backend && DATABASE_URL=… go test -tags integration -count=1 ./...` | all packages ok (S6); parallel-approve test 3x ok |
| coverage file service baru (`vault_capacity.go`, `vault_plan*.go`) | 80.2% statement |
| `cd backend_python && python -m unittest discover -s dsr` | 11 OK (S2) |
| `pnpm --dir frontend/CompanyPortal-Vite run test` | 127 files / 1071 tests passed (S8) |
| `pnpm --dir frontend/CompanyPortal-Vite run lint` + `run build` | clean / built |

## Test → requirement
| Test | Covers |
|---|---|
| `DsrSaldoAkhirPerVaultTests` (`backend_python/dsr/test_dsr_etl.py`) | FR1.1 baris `saldo_akhir` per blok berlokasi + `location`; FR1.2 blok total tidak disimpan |
| `dsr_location_map_admin_test.go`, `masterdata_applier_dsr_location_map_integration_test.go`, `admin_dsr_location_map_handler_test.go` | FR2.1–FR2.3: create/update/disable lewat maker-checker, label unik case-insensitive, vault vendor lain ditolak, RBAC |
| `DsrLocationMapsPanel.test.tsx`, `VendorDetailPage.test.tsx` (5 tab) | FR2.4, FR9.3 tab Mapping DSR |
| `TestVaultDemandCapacity`, `TestCapacityWarning`, `TestSortCandidates`, `TestSnapshotRoundTrip` (`vault_capacity_test.go`) | FR3.1–FR3.4 (saldo tidak diketahui, beban ke vault terpilih / branch replenish, kapasitas tanpa ATM sendiri, warning), FR4.2 urutan |
| `TestVaultTier` | FR4.2 tier 1/2/3 |
| `TestIntegration_VaultPlan_FullFlow` (`vault_plan_integration_test.go`) | FR3 dengan DSR + mapping nyata; FR4.2 tier 3 hanya urgent; FR4.3 replace-all + snapshot; FR4.4 submit; FR4.5 maker≠checker ACM, reject; FR4.6 auto `vault_review`; FR5.2 approve → `ready`, reject → semua area `draft`; FR5.3 lintas lapis; FR6.1 audit; FR6.2 non-anggota 404, ADMIN read-only; FR6.3 notifikasi |
| `TestIntegration_VaultPlan_BranchMovedArea` (review R1/R2) | FR7.3 branch pindah area: ATM yang sudah ditetapkan tetap di rencananya, yang belum ikut area baru, rencana lain tidak bisa mengambilnya (422); notifikasi review ATM-SPV ke ACM-USER berlink rencana, ke pembuat berlink request |
| `TestIntegration_VaultPlan_ParallelApprove` | NFR3 dua area approve paralel → tepat satu `vault_ready` |
| `vault_flow_integration_test.go` (3 tests) | FR4.1 approve → `vault_assignment` + rencana per area + notifikasi; FR5.4 laporan selesai dari `ready` dan reject kembali ke `ready`; FR5.5 cancel cascade; FR7.3 assign branch → rencana dibuat; FR7.4 peringatan |
| `vendor_request_state_machine_property_test.go`, `vendor_request_test.go`, `vendor_request_cancel_integration_test.go` | transisi baru + regresi request lama (`approved`) |
| `acm_area_admin_integration_test.go`, `admin_acm_area_handler_test.go` | FR7.1–FR7.2 CRUD area, satu branch satu area, nonaktif melepas branch, audit dalam tx, ADMIN only |
| `AdminAcmAreasPage.test.tsx` | FR9.3 layar Area ACM + peringatan |
| `vault_plan_handler_test.go` | API wiring: uang sebagai string, status error 404/403/409/422 |
| `VaultPlan.test.tsx` (8 tests) | FR9.1: default kandidat teratas tier 1 (FR4.2), warning kapasitas, simpan, urgent + alasan 10–500, ACM-SPV approve bukan kiriman sendiri, banner alasan tolak, filter status + tanggal; FR9.2/FR5.1 panel saldo/kapasitas + approve/tolak wajib alasan |
| `VendorRequestDetail.test.tsx` "vault panel" | FR9.2 gating: aksi review hanya `vault_review` + checker; tidak tampil untuk request lama |
| `navigation.test.ts` | FR8 menu CIT → Penetapan Vault, role ACM valid |
| `no_hard_delete_test.go` | tabel baru tanpa `DELETE FROM` |

## Keputusan yang mengubah spec
- FR3.2 (user 2026-10-08, opsi a): beban branch menghitung **semua** request `vault_flow` tanggal X yang tidak batal/ditolak
  (termasuk `completion_pending`/`completed`), bukan hanya `vault_assignment|vault_review|ready`. Dicatat di `spec.md` FR3.2.

## Not covered by automated tests
- NFR1 (≤ 3 s p95 untuk 1000 ATM) tidak diukur; kandidat dihitung set-based di SQL + Go, frontend memanggil kandidat per baris ATM.
- Email notifikasi nyata (SMTP) — hanya baris outbox dicek.
- **Manual browser check: outstanding (user, Golden Rule #10)** — Pengaturan → Area ACM, tab Mapping DSR di detail vendor,
  CIT → Penetapan Vault (pilih vault, urgent, simpan/kirim/setujui/tolak), panel Penetapan Vault + setujui/tolak di detail
  Vendor Request, link notifikasi ke `/cit/vault-plans/{id}`.
