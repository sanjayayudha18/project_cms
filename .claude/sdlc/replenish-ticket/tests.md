# Tests: Nomor tiket replenish per ATM + nomor request per region

Status: **green 2026-10-08** (Stage 4). Input: `spec.md` (FR1–FR11), `plan.md` (accepted 2026-10-08).

## Commands (all run 2026-10-08)
| Command | Result |
|---|---|
| `psql … -f backend/migrations/026_replenish_ticket_region.sql` (dev, after `pg_dump --data-only` backup) | COMMIT; 174 tickets = 174 (request, ATM) pairs, 0 dup numbers, 67 MIX, max seq 3; 0 results to move (old table empty); 356 cabang coded, 66 NULL (51 no region text + 15 test rows, 0 unexpected); `vendor_request_atm_results` gone |
| `cd backend && sqlc generate` (v1.31.1) | only `master_data_export`, `models`, `vendor_branches_admin`, `vendor_request` changed |
| `cd backend && go build ./... && go vet -tags integration ./...` | clean |
| `cd backend && DATABASE_URL=… go test -tags integration -count=1 ./...` | all packages ok |
| `pnpm --dir frontend/CompanyPortal-Vite run test` | 124 files / 1055 tests passed |
| `pnpm --dir frontend/CompanyPortal-Vite exec tsc -b tsconfig.app.json` + `run build` | clean / built |
| `pnpm … exec biome check src/features/admin-vendors src/features/vendor-request` | clean |

## Test → requirement
| Test | Covers |
|---|---|
| `TestDenomCode`, `TestWantedCodes` (`service/vendor_request_ticket_test.go`) | FR1.1 code `50K`/`100K`/`MIX`, one ticket per ATM |
| `TestPlanTickets` (6 cases) | FR1.2 order, FR2.1 unchanged, FR2.2 deactivate, FR2.3 code change, FR2.4/2.5 reactivate |
| `TestSingleRegion` (5 cases) | FR10.1, FR10.2 (mixed codes, cabang without code), FR10.4 (update vs request code) |
| `TestNormalizeBranchRegionCode` | FR11.1, plan D6 |
| `TestIntegration_Ticket_CreateNumbersAndRegion` | FR10.3 `REP-<p>-ITEST-<date>-001`, FR1 `_50K_…_001`, NNN global per ATM+date (`MIX _002`), FR4.1 FK on `request_number`, FR5.1 item + atm ticket |
| `TestIntegration_Ticket_SeqExhaustedRejects` (review R1) | FR1.3 seq 999 → 422, create rolled back |
| `TestTicketDate` (review R2) | FR7.3 date rule for old `VR-` requests in `UpdateItems` |
| `TestIntegration_Ticket_RegionRules` | FR10.2 cabang without code → 422, FR10.5 counter per region (`ITESTB` restarts at 001) |
| `TestIntegration_Ticket_UpdateItemsDeactivatesAndReactivates` | FR2.1/2.3/2.5 against Postgres (1 active / 2 total) |
| `atm_visit_quota_integration_test.go` (7 tests, fixtures now seed a ticket per ATM) + `vendor_request_notification_integration_test.go` | FR9 regression: laporan selesai on the ticket, approve/visits/kuota unchanged |
| `vendor_request_number_integration_test.go` (regex + exhaustion + retry updated) | FR10.5 atomic counter, 999 exhaustion, unique-violation retry still work with region |
| `TestImport_VendorBranchRegionCode`, `TestExport_HeaderContract` | FR11.2 CSV column, old file without the column keeps the code, invalid code = row error |
| `TestVendorRequestHandler_Get_TicketAndRegion` | FR5.1 wire fields |
| `VendorRequestDetail.test.tsx` "shows the ticket number per ATM" | FR8.1 |
| `vendorBranchFormSchema.test.ts` | FR8.2 / FR11.1 form validation |

## Not covered by automated tests
- FR1.5 two parallel creates for the same ATM+date: covered by the advisory lock design (D2); the existing concurrent-create number test uses one ATM, so it exercises the lock, but does not assert ticket numbers.
- UpdateItems adding an ATM from another region (FR10.4) end-to-end: needs a second DMAA-backed ATM in another cabang; covered by `TestSingleRegion` "update other code".
- **Manual browser check: outstanding (user, Golden Rule #10)** — detail request "No. Tiket" column, laporan selesai table, vendor cabang form/table "Kode Region", 422 message on Forecast Browser create for a cabang without a code.
