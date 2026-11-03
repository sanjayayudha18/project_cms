# Graph Report - backend  (2026-11-02)

## Corpus Check
- 458 files · ~696,800 words
- Verdict: corpus is large enough that graph structure adds value.

## Summary
- 5395 nodes · 16518 edges · 266 communities (237 shown, 29 thin omitted)
- Extraction: 85% EXTRACTED · 15% INFERRED · 0% AMBIGUOUS · INFERRED: 2445 edges (avg confidence: 0.85)
- Token cost: 0 input · 0 output

## Graph Freshness
- Built from commit: `f98bc894`
- Run `git rev-parse HEAD` and compare to check if the graph is stale.
- Run `graphify update .` after code changes (no API cost).

## Community Hubs (Navigation)
- context.Context
- Actor
- Queries
- atm_portal_handler.go
- harness
- net/http.Handler
- testing.T
- fakeUserAdminRepo
- DBTX
- Queries
- Queries
- NewMasterDataExportRepository
- NewATMAssignmentAdminService
- parsePathID
- writeError
- adminCtx
- time.Time
- github.com/jackc/pgx/v5/pgtype.Timestamptz
- atm_assignments_admin.sql.go
- Region
- Queries
- AuditLog
- vendor_party.go
- tokenFor
- Role
- DsrLocationMapAdminService
- DsrLateFilter
- testTokenService
- fakeVendorPicAdminServicer
- net/http.ResponseWriter
- 001_baseline_schema.sql
- github.com/jackc/pgx/v5/pgtype.Numeric
- toPgDate
- branch_atm_test.go
- NewDmaaForecastService
- runner
- NewUserAdminService
- DsrLateReportRepository
- net/http.Request
- doRequest
- fakeATMAdminRepo
- fakeVendorBranchAdminServicer
- jakartaCalendarDate
- setupForecastQueryHarness
- mountVendorRequestHandler
- ApprovalRequest
- tokenForRole
- RbacListHandler
- vault_allocation.go
- harness
- fakeVendorAdminRepo
- vendor_request_response.go
- atm_admin.go
- vendor_package_admin.go
- vendor_package_price_admin.go
- GetVendorPackageAdminByIDRow
- vendor_package_prices_admin.sql.go
- formatTimestamptz
- NewChangePasswordService
- Queries
- writeUnauthorized
- fakeDsrRepository
- MasterDataChangeRequest
- openapi_contract_test.go
- holiday_admin.go
- VendorVaultAdminService
- TestMasterDataImport_PicRoles_Integration
- .withFilter
- mountAdminRegionHandler
- newVaultPlanFixture
- user_admin.go
- atm_visit_quota.sql.go
- atm_admin_test.go
- newVendorOrderFixture
- Queries
- writeJSON
- NewVendorAdminService
- admin_vendor_handler_test.go
- newServiceUnderTest
- main
- vault_allocations.sql.go
- .Confirm
- NewAtmPortalService
- newAuditWriter
- setupQuotaFixture
- masterdata_import_history.go
- github.com/cimb-niaga/cms/pkg/auth.TokenService
- NewOrchestrator
- masterdata_import.go
- propertyTestPool
- ApprovalDelegation
- Queries
- fakeVendorAdminServicer
- .Apply
- ApprovalPolicy
- admin_atm_assignment_handler_test.go
- dsr_upload.go
- masterdata_import_confirm.go
- VendorPicAdminService
- dsr.sql.go
- .Get
- mountAdminApprovalHandler
- ApprovalHandler
- mountCashposHandler
- NewService
- newImportFixture
- .actorAndID
- .toggle
- serveNotification
- New
- 027_acm_vault_assignment.sql
- replenish_classifications.sql.go
- AdminMasterDataImportHandler
- mountRbacListHandler
- newHistoryFixture
- importEnv
- regionHarness
- 031_replenish_realization.sql
- DsrService
- NewDsrService
- mountAdminVendorBranchHandler
- worker_test.go
- MasterDataImportBatchRepository
- notificationImportNotifier
- BuildChain
- Orchestrator
- Entry
- NewDeactivateUserService
- Queries
- AuthHandler
- service_property_test.go
- dsr_upload_handler.go
- notification.go
- NewLocalProvider
- .List
- mountAdminVendorPicHandler
- numericFromDecimalPtr
- vendor_vaults_admin.sql.go
- atm_portal_filter_property_test.go
- reasonProbePool
- public.vendor_request_vault_allocations
- GetMasterDataImportBatchRow
- github.com/jackc/pgx/v5/pgconn.CommandTag
- mountAdminMasterDataChangeHandler
- NewRbacReadRepository
- Queries
- ListVaultTicketsForRequestRow
- mountRoleMgmtHandler
- admin_repository_topology_test.go
- dbtxProbe
- readImportCSV
- 003_master_data.sql
- mountAdminBranchATMHandler
- mountAdminVendorPackageHandler
- acmAreaHarness
- IsVendor
- public.vendor_package_prices
- 019_import_jobs.sql
- 021_atm_visit_quota.sql
- mountAdminVendorVaultHandler
- .SetRoles
- public.export_jobs
- mountAdminVendorPackagePriceHandler
- SMTPMailer
- NewMasterDataChangeRepository
- .Apply
- masterdata_import_notify_test.go
- SubmitRequest
- ListDsrDailyRowErrorsRow
- .Apply
- .Apply
- public.notifications
- public.dsr_receipts
- stubProvider
- TestMasterDataExportRepository_KeysetPagingIsCompleteAndExact
- no_hard_delete_test.go
- public.master_data_import_batches
- public.branch_coverage_areas
- CLAUDE.md
- printableASCIIRune
- ValidationError
- AdminStore
- public.vendor_vaults
- public.users
- check-coverage.sh
- public.vendor_packages
- public.vendor_vaults
- public.vendor_packages
- public.vendor_packages
- public.vendor_packages
- public.vendor_package_prices
- public.vendor_packages_branch
- public.vendor_packages_branch
- public.vendor_package_prices
- public.regions
- public.vendor_requests
- github.com/cimb-niaga/cms/backend
- public.atm_vendor_packages
- public.dsr_daily_rows
- public.export_jobs
- public.master_data_import_batches
- public.vendor_pics
- vendor_branches

## God Nodes (most connected - your core abstractions)
1. `doRequest()` - 290 edges
2. `tokenForRole()` - 242 edges
3. `writeError()` - 186 edges
4. `writeJSON()` - 169 edges
5. `MasterDataChangeRequest` - 122 edges
6. `Actor` - 112 edges
7. `New()` - 111 edges
8. `main()` - 96 edges
9. `writeUnauthorized()` - 92 edges
10. `Queries` - 91 edges

## Surprising Connections (you probably didn't know these)
- `refreshParty()` --calls--> `record`  [INFERRED]
  internal/service/vendor_party.go → cmd/migrate/runner.go
- `syncVendorParties()` --calls--> `record`  [INFERRED]
  internal/service/vendor_party.go → cmd/migrate/runner.go
- `TestIntegration_VendorRequestTransition_Guards()` --calls--> `status()`  [INFERRED]
  internal/service/vendor_request_rules_integration_test.go → cmd/migrate/runner_integration_test.go
- `main()` --calls--> `NewOrchestrator()`  [EXTRACTED]
  cmd/api/main.go → internal/approval/orchestrator.go
- `main()` --calls--> `NewRepository()`  [EXTRACTED]
  cmd/api/main.go → internal/approval/repository.go

## Import Cycles
- None detected.

## Communities (266 total, 29 thin omitted)

### Community 0 - "context.Context"
Cohesion: 0.03
Nodes (31): branchChildCounterAdapter, mockUserRepository, stubRateLimiter, stubUserRepo, IsAcmAreaMemberParams, ListAcmAreaMemberIDsByRoleParams, ListVaultPlanAtmsRow, LockVaultPlanRow (+23 more)

### Community 1 - "Actor"
Cohesion: 0.05
Nodes (46): fakeVendorRequestServicer, validateDateBound(), extractMetadata(), extractState(), VendorRequestService, mapDetail(), totalPages(), checkActor() (+38 more)

### Community 2 - "Queries"
Cohesion: 0.04
Nodes (35): AddAcmAreaBranchParams, AddAcmAreaMemberParams, AddAcmAreaRegionParams, CreateAcmAreaParams, ListBranchesInOtherAcmAreasParams, ListRegionsInOtherAcmAreasParams, RenameAcmAreaParams, SetAcmAreaActiveParams (+27 more)

### Community 3 - "atm_portal_handler.go"
Cohesion: 0.05
Nodes (67): math/big.Rat, net/url.Values, atmPortalRow, atmProfileResponse, atmSummaryResponse, auditLogDetailResponse, auditLogListResponse, auditLogListRow (+59 more)

### Community 4 - "harness"
Cohesion: 0.07
Nodes (59): approveChange(), testHelpers, TestDsrLocationMapApplier_Lifecycle_Integration(), batchCauseMessage(), pgx.Tx, approveAssignment(), approveATM(), assignmentFixture() (+51 more)

### Community 5 - "net/http.Handler"
Cohesion: 0.06
Nodes (70): net/http.Cookie, net/http.Handler, net/http/httptest.ResponseRecorder, errorResponseBody, importRateLimiter, loginResponseBody, testATMSpec, testHarness (+62 more)

### Community 6 - "testing.T"
Cohesion: 0.05
Nodes (65): testing.T, TestEndToEnd_ThreeLevelHierarchyWithDelegationOnLeave(), TestRepository_CreateDelegation_NonOverlappingSucceeds(), TestRepository_CreateDelegation_OverlapRejected(), TestRepository_ListInboxForApprover(), TestRepository_RevokeDelegation(), NewRepository(), mountAuditLogHandler() (+57 more)

### Community 7 - "fakeUserAdminRepo"
Cohesion: 0.04
Nodes (23): fakeUserAdminRepo, fakeVendorOptionsReader, FindUserAdminByEmailExcludingIDParams, FindUserAdminByEmployeeIDExcludingIDParams, VendorBranchBelongsToVendorParams, fakeUserAdminServicer, CreateUserRequest, VendorOptions (+15 more)

### Community 8 - "DBTX"
Cohesion: 0.06
Nodes (52): fakeVaultPlanServicer, NewWriter(), TestWriter_Write(), TestWriter_Write_NilBeforeAfter(), DBTX, ListVaultSaldoRow, GetVaultPlanRow, ListVaultPlansForRequestRow (+44 more)

### Community 9 - "Queries"
Cohesion: 0.04
Nodes (27): GetActiveVendorForTerminalParams, GetActiveVendorForTerminalRow, GetVendorForRequestNumberRow, IssueVendorRequestTicketParams, LockTicketSeqParams, SetVendorRequestTicketActiveParams, SetVendorRequestTicketResultParams, UpdateVendorRequestCompletionParams (+19 more)

### Community 10 - "Queries"
Cohesion: 0.04
Nodes (25): CountNotificationsForUserParams, ListNotificationsForUserParams, MarkNotificationReadParams, time.Duration, fakeNotificationStore, noopBlacklist, vendorRequestNoopBlacklist, ClaimPendingNotificationEmailsRow (+17 more)

### Community 11 - "NewMasterDataExportRepository"
Cohesion: 0.05
Nodes (28): ExportATMAssignmentsBatchParams, ExportATMsBatchParams, ExportVendorBranchesBatchParams, ExportVendorPicsBatchParams, ExportVendorsBatchParams, ExportVendorVaultsBatchParams, ExportATMAssignmentsBatchRow, ExportATMsBatchRow (+20 more)

### Community 12 - "NewATMAssignmentAdminService"
Cohesion: 0.06
Nodes (44): fakeATMAssignmentAdminServicer, dateToStringPtr(), ATMAssignment, ATMAssignmentUpdatePayload, newATMAssignment(), NewATMAssignmentAdminService(), date(), ATMAssignmentUpdatePayload (+36 more)

### Community 13 - "parsePathID"
Cohesion: 0.08
Nodes (25): AdminATMAssignmentHandler, AdminVendorBranchHandler, AdminVendorHandler, AdminVendorPackagePriceHandler, AdminVendorPicHandler, createVendorAdminRequestBody, updateVendorAdminRequestBody, VendorAdminServicer (+17 more)

### Community 14 - "writeError"
Cohesion: 0.07
Nodes (27): AdminDsrLocationMapHandler, createRoleRequestBody, DsrLocationMapAdminServicer, DsrUploadHandler, ErrorResponse, FieldError, RoleMgmtHandler, RoleMgmtServicer (+19 more)

### Community 15 - "adminCtx"
Cohesion: 0.06
Nodes (60): NewMasterDataChangeService(), adminCtx(), ctxWithRole(), TestMasterDataChangeService_Submit_ServiceLayerRBAC(), TestMasterDataChangeService_Submit_ApprovalFailureReleasesPendingSlot(), TestMasterDataChangeService_Submit_Create(), TestMasterDataChangeService_Submit_Create_RejectsEntityID(), TestMasterDataChangeService_Submit_Create_SkipsPendingCheck() (+52 more)

### Community 16 - "time.Time"
Cohesion: 0.07
Nodes (40): failingAvailability, fakeAvailabilityRepository, time.Time, fakeReplenishmentReader, goldenReplenishmentReader, T, goldenDailyRows(), goldenMismatchRows() (+32 more)

### Community 17 - "github.com/jackc/pgx/v5/pgtype.Timestamptz"
Cohesion: 0.07
Nodes (62): AcmAreaBranch, AcmAreaMember, AcmAreaRegion, Atm, AtmDenom, AtmVendorPackage, BranchCoverageArea, CimbBranch (+54 more)

### Community 18 - "atm_assignments_admin.sql.go"
Cohesion: 0.05
Nodes (21): CloseATMAssignmentsBeforeStartParams, CreateATMAssignmentAdminParams, CreateATMAssignmentAdminRow, DisableATMAssignmentsStartingOnParams, FindOverlappingATMAssignmentParams, UpdateATMAssignmentAdminParams, UpdateATMAssignmentAdminRow, CheckAssignmentVendorBranchParams (+13 more)

### Community 19 - "Region"
Cohesion: 0.06
Nodes (26): UpdateRegionNameParams, fakeRegionAdminServicer, Region, CountRegionsAdminParams, CreateRegionAdminParams, GetRegionAdminByIDRow, ListRegionsAdminParams, ListRegionsAdminRow (+18 more)

### Community 20 - "Queries"
Cohesion: 0.09
Nodes (33): contractOrderSvc, fakeVendorOrderServicer, Writer, Queries, pgx.Tx, VendorRequest, contractOrderFull(), createVaultPlans() (+25 more)

### Community 21 - "AuditLog"
Cohesion: 0.07
Nodes (35): CountAuditLogsParams, CreateAuditLogParams, ListAuditLogsParams, stubAuditLogService, Queries, ListAuditLogsByEntityParams, Queries, AuditLog (+27 more)

### Community 22 - "vendor_party.go"
Cohesion: 0.07
Nodes (40): CountVendorOrdersParams, DecideVendorPartyParams, GetUserVendorScopeRow, GetVendorOrderParams, InsertVendorPartyEventParams, InsertVendorPartyParams, ListVendorOrdersParams, ListVendorPartiesForRequestRow (+32 more)

### Community 23 - "tokenFor"
Cohesion: 0.08
Nodes (50): ChangePasswordService, ApprovalOrchestrator, ApprovalReader, approvalRequestResponse, fakeApplyRetrier, inboxItemResponse, MasterDataApplyRetrier, submitRequest (+42 more)

### Community 24 - "Role"
Cohesion: 0.05
Nodes (16): HasRolePermissionParams, InsertRolePermissionParams, fakeRoleMgmtServicer, MenuFeature, Role, CreateRoleParams, ListRolesWithPermissionsRow, Queries (+8 more)

### Community 25 - "DsrLocationMapAdminService"
Cohesion: 0.05
Nodes (19): CreateDsrLocationMapParams, CreateDsrLocationMapRow, FindActiveDsrLocationMapParams, UpdateDsrLocationMapParams, UpdateDsrLocationMapRow, fakeDsrLocationMapServicer, GetDsrLocationMapAdminByIDRow, ListDsrLocationMapsAdminParams (+11 more)

### Community 26 - "DsrLateFilter"
Cohesion: 0.07
Nodes (36): github.com/cimb-niaga/cms/pkg/reports.AmountMismatchRow, github.com/cimb-niaga/cms/pkg/reports.DsrLateDailyRow, github.com/cimb-niaga/cms/pkg/reports.DsrLateSummaryRow, github.com/cimb-niaga/cms/pkg/reports.TripRow, contractDsrLate, fakeDsrLateReader, goldenDsrLateReader, pagedGoldenDsrLateReader (+28 more)

### Community 27 - "testTokenService"
Cohesion: 0.07
Nodes (43): math/rand.Rand, cutoverTarget, fakePartyReader, fakeReturner, realizationBody, VendorOrderServicer, TestAdminHolidayHandler(), TestAdminHolidayHandler_Import() (+35 more)

### Community 28 - "fakeVendorPicAdminServicer"
Cohesion: 0.05
Nodes (17): CreateVendorPicAdminParams, CreateVendorPicAdminRow, GetVendorPicRolesForUpdateParams, SetVendorPicRolesParams, SetVendorPicRolesRow, UpdateVendorPicAdminParams, UpdateVendorPicAdminRow, fakeVendorPicAdminServicer (+9 more)

### Community 29 - "net/http.ResponseWriter"
Cohesion: 0.09
Nodes (23): net/http.ResponseWriter, cancelBody, completionBody, createVendorRequestBody, itemInputPayload, reasonBody, rejectBody, updateItemsBody (+15 more)

### Community 30 - "001_baseline_schema.sql"
Cohesion: 0.08
Nodes (50): public.approval_delegations, public.approval_policies, public.approval_requests, public.approval_steps, public.atm_denoms, public.atm_vendor_packages, public.atms, public.audit_logs (+42 more)

### Community 31 - "github.com/jackc/pgx/v5/pgtype.Numeric"
Cohesion: 0.07
Nodes (20): CreateATMAdminRow, github.com/jackc/pgx/v5/pgtype.Numeric, github.com/jackc/pgx/v5/pgtype.Timestamp, CountATMsWithCashPosParams, CountCashposByTerminalParams, CountItmCashposParams, CountReplenishByTerminalParams, GetATMByTerminalIDRow (+12 more)

### Community 32 - "toPgDate"
Cohesion: 0.09
Nodes (41): resolveItems(), validateItemsPayload(), acceptItems(), attemptCreateWithNumber(), categoryOrNil(), createWithRetryingNumber(), fallbackVendorPrefix(), pgx.Tx (+33 more)

### Community 33 - "branch_atm_test.go"
Cohesion: 0.07
Nodes (30): AdminBranchATMHandler, BranchATMServicer, fakeBranchATMServicer, ListBranchATMsParams, ListBranchATMsRow, Queries, chi.Router, managedATMToResponse() (+22 more)

### Community 34 - "NewDmaaForecastService"
Cohesion: 0.08
Nodes (35): DmaaForecastHandler, dmaaForecastPagination, dmaaForecastResponse, dmaaForecastResponseRow, stubDmaaForecastService, CountDmaaForecastParams, ListDmaaForecastParams, Queries (+27 more)

### Community 35 - "runner"
Cohesion: 0.11
Nodes (35): main(), run(), checkChecksums(), pgx.ConnConfig, pgx.ConnConfig, migrate(), queryInt(), status() (+27 more)

### Community 36 - "NewUserAdminService"
Cohesion: 0.14
Nodes (39): branchCall, TestUserAdminService_VendorBranch_Integration(), NewUserAdminService(), samePtr(), activeUserRow(), ptrInt64(), TestConflictError_And_ReferenceError_ErrorStrings(), TestUserAdminService_Count_PassesThroughToRepo() (+31 more)

### Community 37 - "DsrLateReportRepository"
Cohesion: 0.09
Nodes (27): CountDsrLateDailyParams, ListDsrLateDailyParams, ListDsrLateDailyRow, ListDsrLateSummaryParams, ListDsrLateSummaryRow, ListDsrLateUnmappedLabelsParams, ListDsrLateUnmappedLabelsRow, ListDsrLateUnmappedVaultsParams (+19 more)

### Community 38 - "net/http.Request"
Cohesion: 0.12
Nodes (13): github.com/cimb-niaga/cms/pkg/middleware.AuthContext, net/http.Request, AcmAreaAdminServicer, acmAreaNameBody, AdminAcmAreaHandler, acmAreaDetailJSON(), acmAreaJSON(), chi.Router (+5 more)

### Community 39 - "doRequest"
Cohesion: 0.13
Nodes (40): mountAdminUserHandler(), TestAdminUserHandler_Create_ConflictError_409(), TestAdminUserHandler_Create_HappyPath(), TestAdminUserHandler_Create_ReferenceError_400(), TestAdminUserHandler_Create_ValidationError_422(), TestAdminUserHandler_Disable_HappyPath(), TestAdminUserHandler_Disable_NotFound_404_NoDeactivateCall(), TestAdminUserHandler_Disable_SelfDisable_BadRequest() (+32 more)

### Community 40 - "fakeATMAdminRepo"
Cohesion: 0.08
Nodes (13): CreateATMAdminParams, UpdateATMAdminParams, UpdateATMAdminRow, CountATMsAdminParams, GetATMAdminByIDRow, ListATMsAdminParams, ListATMsAdminRow, ListLocationsForSelectRow (+5 more)

### Community 41 - "fakeVendorBranchAdminServicer"
Cohesion: 0.08
Nodes (14): CreateVendorBranchAdminParams, CreateVendorBranchAdminRow, UpdateVendorBranchAdminParams, UpdateVendorBranchAdminRow, fakeVendorBranchAdminServicer, CountVendorBranchesAdminParams, GetVendorBranchAdminByIDRow, ListVendorBranchesAdminParams (+6 more)

### Community 42 - "jakartaCalendarDate"
Cohesion: 0.16
Nodes (34): github.com/jackc/pgx/v5/pgxpool.Pool, countAuditEntries(), fetchUserIDs(), TestIntegration_Cancel_AlreadyCanceledRejectsWithoutDuplicateAudit(), TestIntegration_Cancel_ApprovedIdempotency(), TestIntegration_Cancel_ApprovedIsCheckerOnly(), TestIntegration_Cancel_ApprovedPreservesRowsAndSingleAudit(), TestIntegration_Cancel_AuthorizationUnionRule() (+26 more)

### Community 43 - "setupForecastQueryHarness"
Cohesion: 0.12
Nodes (32): ListForecastForDateRow, TestIntegration_VendorWideAssignment(), pgx.Tx, seedVendorWide(), TestIntegration_VendorWideAssignment_Applier(), TestIntegration_VendorWideAssignment_ForecastCountSummaryAndVendorResolver(), pgx.Tx, linkAtmVendorPackage() (+24 more)

### Community 44 - "mountVendorRequestHandler"
Cohesion: 0.18
Nodes (38): TestVendorRequestHandler_UpdateItemsAndRevise(), mountAtmVisitQuota(), TestAtmVisitQuotaHandler_ErrorMapping(), TestAtmVisitQuotaHandler_RoleGates(), TestVendorRequestHandler_CompletionRoutes(), mountVendorRequestHandler(), TestVendorRequestHandler_Approve_SelfApprovalForbidden(), TestVendorRequestHandler_Approve_WrongRoleForbidden() (+30 more)

### Community 45 - "ApprovalRequest"
Cohesion: 0.08
Nodes (9): fakeStore, Repository, fakeApprovalOrchestrator, fakeApprovalReader, docKey(), InboxItem, ApprovalRequest, ApprovalStep (+1 more)

### Community 46 - "tokenForRole"
Cohesion: 0.14
Nodes (38): TestAdminAcmAreaHandler(), tokenForRole(), mountAdminATMHandler(), TestAdminATMHandler_Create_Accepted202(), TestAdminATMHandler_Create_InvalidReference_400(), TestAdminATMHandler_Create_MalformedBody_400(), TestAdminATMHandler_Create_TerminalIDConflict_409(), TestAdminATMHandler_Create_UnexpectedError_500() (+30 more)

### Community 47 - "RbacListHandler"
Cohesion: 0.10
Nodes (23): AdminApprovalHandler, AdminAuditWriter, AdminStore, ApprovalPolicyWriter, createDelegationRequest, createLeaveRequest, delegationResponse, leaveResponse (+15 more)

### Community 48 - "vault_allocation.go"
Cohesion: 0.10
Nodes (32): ListVaultCandidateVaultsRow, allocationTotals(), allocErr(), checkCoverage(), denomLabel(), VaultBranchOption, groupVaultOptions(), isAllocationDenom() (+24 more)

### Community 49 - "harness"
Cohesion: 0.11
Nodes (29): TestHasPermission_DecisionComesOnlyFromMapping(), CreateRoleRequest, assertInt64Set(), assertPersistedPermissions(), harness(), queryRowScan(), TestCreateRole_AuditFailureRollsBack(), TestCreateRole_AuthorizationMatrix() (+21 more)

### Community 50 - "fakeVendorAdminRepo"
Cohesion: 0.08
Nodes (11): CreateVendorAdminParams, CreateVendorAdminRow, UpdateVendorAdminParams, UpdateVendorAdminRow, CountVendorsAdminParams, ListVendorsAdminParams, ListVendorsAdminRow, Queries (+3 more)

### Community 51 - "vendor_request_response.go"
Cohesion: 0.12
Nodes (33): atmVisitQuotaResponse, AtmVisitQuotaServicer, atmVisitResponse, auditLogEntryResponse, auditLogResponse, forecastResponse, forecastRowResponse, forecastSummaryGroupResponse (+25 more)

### Community 52 - "atm_admin.go"
Cohesion: 0.11
Nodes (18): fakeATMAdminServicer, atmFromGetRow(), atmFromListRow(), decimalStrings(), ATM, CreateATMRequest, LocationOption, UpdateATMRequest (+10 more)

### Community 53 - "vendor_package_admin.go"
Cohesion: 0.11
Nodes (22): fakeVendorPackageAdminServicer, fromGetPackageRow(), fromListPackageRow(), VendorPackage, VendorPackageCreatePayload, NewVendorPackageAdminService(), newVendorPackageFromRow(), baseVendorPackageCreatePayload() (+14 more)

### Community 54 - "vendor_package_price_admin.go"
Cohesion: 0.11
Nodes (20): fakeVendorPackagePriceServicer, TestNumericFromDecimalPtr(), TestValidatePriceContent_BasePrice(), TestValidatePriceContent_OptionalFields(), fromGetRow(), fromListRow(), VendorPackagePrice, VendorPackagePriceContentPayload (+12 more)

### Community 55 - "GetVendorPackageAdminByIDRow"
Cohesion: 0.09
Nodes (13): CreateVendorPackageAdminParams, CreateVendorPackageAdminRow, UpdateVendorPackageAdminParams, UpdateVendorPackageAdminRow, CountVendorPackagesAdminParams, GetVendorPackageAdminByIDRow, ListVendorPackagesAdminParams, ListVendorPackagesAdminRow (+5 more)

### Community 56 - "vendor_package_prices_admin.sql.go"
Cohesion: 0.09
Nodes (14): CreateVendorPackagePriceAdminParams, CreateVendorPackagePriceAdminRow, UpdateVendorPackagePriceAdminParams, UpdateVendorPackagePriceAdminRow, CountVendorPackagePricesAdminParams, GetVendorKindForPackagePriceRow, GetVendorPackagePriceAdminByIDRow, ListVendorPackagePricesAdminParams (+6 more)

### Community 57 - "formatTimestamptz"
Cohesion: 0.11
Nodes (20): AdminUserHandler, createUserRequestBody, DeactivateService, optionalInt64, setInitialPasswordRequest, SetInitialPasswordService, updateUserRequestBody, UserAdminServicer (+12 more)

### Community 58 - "NewChangePasswordService"
Cohesion: 0.11
Nodes (25): AuditWriter, ChangePasswordService, SetInitialPasswordService, fakeChangePasswordService, fakeSetInitialPasswordService, ChangePasswordRequest, NewChangePasswordService(), localUserWithPassword() (+17 more)

### Community 59 - "Queries"
Cohesion: 0.09
Nodes (15): CreateApprovalDelegationParams, CreateApprovalRequestParams, CreateApprovalStepParams, CreateUserLeaveParams, FindActiveDelegateParams, FindActiveLeaveParams, FindApprovalPolicyParams, FindApprovalRequestByDocumentParams (+7 more)

### Community 60 - "writeUnauthorized"
Cohesion: 0.15
Nodes (12): AdminHolidayHandler, AtmVisitQuotaHandler, HolidayAdminer, NotificationHandler, chi.Router, holidayID(), NewAdminHolidayHandler(), chi.Router (+4 more)

### Community 61 - "fakeDsrRepository"
Cohesion: 0.10
Nodes (10): detailFakeDsrRepo, scopeFakeDsrRepo, CountDsrUploadsByVendorParams, ListDsrDailyRowsRow, ListDsrRencanaIsiRowsRow, ListDsrUploadsByVendorParams, ListDsrUploadsByVendorRow, mustNumeric() (+2 more)

### Community 62 - "MasterDataChangeRequest"
Cohesion: 0.11
Nodes (14): github.com/cimb-niaga/cms/backend/internal/service.VendorPackageContentPayload, capturingReader, fakeMasterDataChangeReader, fakeMasterDataDetail, ListMasterDataChangeRequestsParams, MasterDataChangeRequest, VendorBranchPayload, VendorBranchUpdatePayload (+6 more)

### Community 63 - "openapi_contract_test.go"
Cohesion: 0.15
Nodes (26): github.com/getkin/kin-openapi/openapi3.T, github.com/getkin/kin-openapi/routers.Router, net/http.Header, contractCase, contractPartyReader, fakeVaultReviewer, contractCases(), contractDetailFull() (+18 more)

### Community 64 - "holiday_admin.go"
Cohesion: 0.14
Nodes (17): fakeHolidays, authorizeHolidayAdmin(), HolidayImportResult, HolidayList, HolidayView, holidayView(), ptrInt32(), TestIntegration_HolidayAdmin() (+9 more)

### Community 65 - "VendorVaultAdminService"
Cohesion: 0.13
Nodes (16): fakeVendorVaultAdminServicer, VendorVault, VendorVaultPayload, VendorVaultUpdatePayload, newVendorVault(), NewVendorVaultAdminService(), parseDecimalField(), TestValidateVaultFields() (+8 more)

### Community 66 - "TestMasterDataImport_PicRoles_Integration"
Cohesion: 0.13
Nodes (25): CSVSafe(), checkGolden(), exportEntityNames(), TestExportGolden_Bytes(), TestExportGolden_Headers(), NewMasterDataExporter(), TestExportRoundTrip_DryRunIsZeroChanges(), spec() (+17 more)

### Community 67 - ".withFilter"
Cohesion: 0.12
Nodes (14): DsrLateHandler, DsrLateReader, ReplenishmentHandler, ReplenishmentReader, dsrLateError(), chi.Router, parseDsrLateFilter(), rejectCSVFormat() (+6 more)

### Community 68 - "mountAdminRegionHandler"
Cohesion: 0.15
Nodes (28): mountAdminRegionHandler(), regionName(), TestAdminRegionHandler_Create_CodeConflict_409(), TestAdminRegionHandler_Create_Created201(), TestAdminRegionHandler_Create_Forbidden_403(), TestAdminRegionHandler_Create_MalformedBody_400(), TestAdminRegionHandler_Create_UnexpectedError_500(), TestAdminRegionHandler_Create_ValidationError_422() (+20 more)

### Community 69 - "newVaultPlanFixture"
Cohesion: 0.14
Nodes (15): VendorRequestService, newVaultFlowFixture(), TestIntegration_VaultFlow_ApproveCreatesPlanAndCancelCascades(), TestIntegration_VaultFlow_CompletionFromReadyReturnsToReady(), TestIntegration_VaultFlow_RegionWithoutAreaGetsPlanOnMapping(), TestIntegration_VaultPlan_Guards(), vaultPlanFixture, line() (+7 more)

### Community 70 - "user_admin.go"
Cohesion: 0.14
Nodes (19): ConflictError, ReferenceError, UserAdminRepo, UserAdminService, VendorOptionsReader, changesImmutable(), conflictFieldFromPgError(), AuditWriter (+11 more)

### Community 71 - "atm_visit_quota.sql.go"
Cohesion: 0.11
Nodes (15): AdjustAtmVisitQuotaParams, AtmVisit, AtmVisitQuota, CancelAtmVisitParams, EnsureAtmVisitQuotaParams, GetAtmVisitQuotaByAtmRow, InsertAtmVisitParams, ListAtmsForVendorQuotaResetParams (+7 more)

### Community 72 - "atm_admin_test.go"
Cohesion: 0.22
Nodes (27): NewATMAdminService(), activeATM(), atmRepoWith(), disabledATM(), TestATMAdminService_Count_PassesThroughToRepo(), TestATMAdminService_Create_InvalidLocationReference(), TestATMAdminService_Create_MoneyStaysExactAndBlanksBecomeNil(), TestATMAdminService_Create_StagesPayload() (+19 more)

### Community 73 - "newVendorOrderFixture"
Cohesion: 0.20
Nodes (21): deleteVendorUsers(), newVendorOrderFixture(), realizationRows(), TestIntegration_VendorOrder_AcceptAll(), TestIntegration_VendorOrder_CancelSkipsWithdrawnParty(), TestIntegration_VendorOrder_CompletionGateAndCancel(), TestIntegration_VendorOrder_PartyRoleScope(), TestIntegration_VendorOrder_Realization() (+13 more)

### Community 74 - "Queries"
Cohesion: 0.10
Nodes (11): CountMasterDataImportBatchesParams, CreateMasterDataImportBatchParams, FindOpenMasterDataImportBatchParams, FindPendingMasterDataChangeRequestParams, FindPendingMasterDataChangeRequestRow, ListMasterDataImportBatchesParams, MarkMasterDataChangeRequestApplyFailedParams, MarkMasterDataChangeRequestStaleParams (+3 more)

### Community 75 - "writeJSON"
Cohesion: 0.15
Nodes (10): AdminATMHandler, ATMAdminServicer, AtmPortalHandler, createATMAdminRequestBody, updateATMAdminRequestBody, atmToResponse(), chi.Router, NewAdminATMHandler() (+2 more)

### Community 76 - "NewVendorAdminService"
Cohesion: 0.19
Nodes (26): GetVendorAdminByIDRow, NewVendorAdminService(), activeVendor(), disabledVendor(), TestVendorAdminService_Count_PassesThroughToRepo(), TestVendorAdminService_Create_CodeConflict_StagesNothing(), TestVendorAdminService_Create_InvalidNPWP_StagesNothing(), TestVendorAdminService_Create_StagesLegalNameAndNormalizedNPWP() (+18 more)

### Community 77 - "admin_vendor_handler_test.go"
Cohesion: 0.15
Nodes (26): deref(), equalStrPtr(), mountAdminVendorHandler(), strPtr(), TestAdminVendorHandler_Create_Accepted202(), TestAdminVendorHandler_Create_CodeConflict_409(), TestAdminVendorHandler_Create_InvalidNPWP_422(), TestAdminVendorHandler_Create_ValidationError_422() (+18 more)

### Community 78 - "newServiceUnderTest"
Cohesion: 0.20
Nodes (23): noopBlacklist, github.com/cimb-niaga/cms/pkg/auth.TokenConfig, activeKaryawanUser(), newServiceUnderTest(), TestService_Login_DeletedUser_GenericError(), TestService_Login_InactiveUser(), TestService_Login_InvalidCredentials(), TestService_Login_LDAPAccount_NeverLocked() (+15 more)

### Community 79 - "main"
Cohesion: 0.12
Nodes (16): getenvDefault(), loadNotificationConfig(), main(), AdminMasterDataChangeHandler, AuditLogHandler, createVendorBranchRequestBody, MasterDataChangeReader, updateVendorBranchRequestBody (+8 more)

### Community 80 - "vault_allocations.sql.go"
Cohesion: 0.11
Nodes (13): CreateMissingVaultPlansByRegionParams, CreateMissingVaultPlansByRegionRow, InsertVaultAllocationAmountParams, InsertVaultAllocationParams, ListRequestOrderTotalsRow, ListVaultAllocatedTotalsParams, ListVaultSaldoParams, SetVaultPlanBufferReasonParams (+5 more)

### Community 81 - ".Confirm"
Cohesion: 0.13
Nodes (17): io.Reader, fakeImportServicer, FindOpenMasterDataImportBatchRow, authorizeMasterDataMaker(), marshalOrNilJSON(), marshalPayloadJSON(), validateMasterDataOp(), ImportConfirmResult (+9 more)

### Community 82 - "NewAtmPortalService"
Cohesion: 0.13
Nodes (22): pgregory.net/rapid.T, mustNumeric(), sampleCashposRow(), TestListCashpos_CountError(), TestListCashpos_Empty(), TestListCashpos_ForwardsFilters(), TestListCashpos_MapsAllFieldsAndDecimalStrings(), TestListCashpos_RepoError() (+14 more)

### Community 83 - "newAuditWriter"
Cohesion: 0.16
Nodes (15): fakeAtmVisitQuotaServicer, pgx.Tx, AtmVisitQuotaView, VendorQuotaResetResult, NewAtmVisitQuotaService(), packageLabel(), recordVisit(), resetOne() (+7 more)

### Community 84 - "setupQuotaFixture"
Cohesion: 0.20
Nodes (18): pgx.Rows, pgx.Tx, pgx.TxOptions, VendorRequestService, seedTicket(), setupQuotaFixture(), TestIntegration_VisitQuota_ApproveDecrements(), TestIntegration_VisitQuota_FailedApproveRollsBack() (+10 more)

### Community 85 - "masterdata_import_history.go"
Cohesion: 0.16
Nodes (21): authorizeMasterDataAdmin(), changeFrom(), countOps(), T, isFailedRow(), NewMasterDataImportHistoryService(), normalizePaging(), pageSlice() (+13 more)

### Community 86 - "github.com/cimb-niaga/cms/pkg/auth.TokenService"
Cohesion: 0.14
Nodes (17): LoginRequest, LoginResponse, github.com/cimb-niaga/cms/pkg/auth.Provider, github.com/cimb-niaga/cms/pkg/auth.RateLimiter, github.com/cimb-niaga/cms/pkg/auth.TokenService, github.com/cimb-niaga/cms/pkg/auth.UserRepository, loginResponse, meResponse (+9 more)

### Community 87 - "NewOrchestrator"
Cohesion: 0.25
Nodes (22): fakeStore, NewOrchestrator(), stepStatuses(), submitInvoice(), TestOrchestrator_ApproveRejectErrorBranches(), TestOrchestrator_ApproverOnLeaveWithoutDelegateBlocksStep(), TestOrchestrator_DelegateActsWhileApproverOnLeave(), TestOrchestrator_FinalApproveAuditFailureIsReturned() (+14 more)

### Community 88 - "masterdata_import.go"
Cohesion: 0.17
Nodes (10): assignmentSourceCell(), importEnv, MasterDataImporter, importNatKey(), ImportChangePreview, ImportFieldChange, ImportInvalidError, ImportPreview (+2 more)

### Community 89 - "propertyTestPool"
Cohesion: 0.26
Nodes (20): keys(), TestListATMs_DateValidation(), TestProperty_DateRangeFilterCorrectness(), TestProperty4_StatusFilterCorrectness(), TestProperty5_ActiveOnlyInvariant(), TestPreservation_CascadeDeletesReplenishRowsWithFile(), TestPreservation_GetLastUpdated_NoRecordsReturnsNoRows(), TestPreservation_GetLastUpdated_ReturnsGlobalMostRecentRecord() (+12 more)

### Community 90 - "ApprovalDelegation"
Cohesion: 0.13
Nodes (6): fakeAdminStore, fakeRbacReader, ListUserHierarchyRow, UpdateUserHierarchyRow, ApprovalDelegation, UserLeave

### Community 91 - "Queries"
Cohesion: 0.12
Nodes (8): FindUserByEmailRow, FindUserByIDRow, FindUserByUsernameRow, GetUserProfileRow, LockAccountParams, SetInitialPasswordParams, SetPasswordParams, Queries

### Community 92 - "fakeVendorAdminServicer"
Cohesion: 0.15
Nodes (10): fakeVendorAdminServicer, CreateVendorRequest, DisableVendorResult, UpdateVendorRequest, normalizeNPWP(), TestNormalizeNPWP(), valueOrEmpty(), VendorAdminRepo (+2 more)

### Community 93 - ".Apply"
Cohesion: 0.16
Nodes (15): TestDigitsOrFallback(), TestMapPriceAndPackageDBError(), TestVendorPackagePriceApplier_RejectsBadChanges(), pgx.Tx, mapPackageDBError(), digitsOrFallback(), pgx.Tx, mapPriceDBError() (+7 more)

### Community 94 - "ApprovalPolicy"
Cohesion: 0.15
Nodes (9): fakeApprovalPolicyWriter, CreateApprovalPolicyParams, UpdateApprovalPolicyParams, ApprovalPolicy, NewApprovalPolicyStore(), numericParam(), TestApprovalPolicyStore_CreateGetUpdate(), TestApprovalPolicyStore_Get_NotFound() (+1 more)

### Community 95 - "admin_atm_assignment_handler_test.go"
Cohesion: 0.20
Nodes (19): ATMAssignmentAdminServicer, NewAdminATMAssignmentHandler(), assignmentTokenSvc(), mountAdminATMAssignmentHandler(), mountPackageOptions(), TestAdminATMAssignmentHandler_CoMountedWithATMCrudRouter(), TestAdminATMAssignmentHandler_Create_ErrorMapping(), TestAdminATMAssignmentHandler_Create_Returns202() (+11 more)

### Community 96 - "dsr_upload.go"
Cohesion: 0.22
Nodes (17): dailyRowErrorsToDto(), dailyRowsToDto(), dateOrNil(), DsrSheetDetail, DsrSheetResult, DsrUploadListResult, int32OrZero(), newSheetDetail() (+9 more)

### Community 97 - "masterdata_import_confirm.go"
Cohesion: 0.26
Nodes (19): assignmentFields(), atmFields(), ATMAssignmentUpdatePayload, importEnv, optInt64(), picFields(), rowRejection(), stageCreate() (+11 more)

### Community 98 - "VendorPicAdminService"
Cohesion: 0.18
Nodes (12): nilIfEmpty(), VendorPicUpdatePayload, NewVendorPicAdminService(), TestVendorPicAdminService_Create(), TestVendorPicAdminService_UpdateAndToggle_Scoping(), TestVendorPicAdminService_Validate(), TestVendorPicAdminService_Warnings(), trimOptional() (+4 more)

### Community 99 - "dsr.sql.go"
Cohesion: 0.14
Nodes (5): GetDsrUploadByIDForVendorParams, GetDsrUploadByIDForVendorRow, GetVendorByIDRow, ListDsrRencanaIsiRowErrorsRow, Queries

### Community 100 - ".Get"
Cohesion: 0.21
Nodes (11): AdminRegionHandler, createRegionRequestBody, RegionAdminServicer, updateRegionRequestBody, getRegionRowToResponse(), chi.Router, listRegionRowToResponse(), NewAdminRegionHandler() (+3 more)

### Community 101 - "mountAdminApprovalHandler"
Cohesion: 0.20
Nodes (18): AdminStore, mountAdminApprovalHandler(), TestAdminApprovalHandler_AdminRole_SetHierarchy_HappyPath(), TestAdminApprovalHandler_CreateDelegation_BadRequest(), TestAdminApprovalHandler_CreateDelegation_HappyPath(), TestAdminApprovalHandler_CreateDelegation_OverlapConflict(), TestAdminApprovalHandler_CreateLeave_BadRequest(), TestAdminApprovalHandler_CreateLeave_HappyPath() (+10 more)

### Community 102 - "ApprovalHandler"
Cohesion: 0.18
Nodes (7): MasterDataApprovalBatchReader, MasterDataApprovalDetailReader, chi.Router, ApprovalHandler, parseApprovalRequestID(), toApprovalRequestResponse(), ApprovalHandler

### Community 103 - "mountCashposHandler"
Cohesion: 0.19
Nodes (17): mountCashposHandler(), TestListCashpos_BadPage(), TestListCashpos_Defaults(), TestListCashpos_Empty(), TestListCashpos_InternalError(), TestListCashpos_QueryForwarding(), TestListCashpos_SuccessAllFields(), TestListCashpos_ValidationError() (+9 more)

### Community 104 - "NewService"
Cohesion: 0.22
Nodes (17): NewService(), equalInts(), TestSend_DedupesUsersAndEmails(), TestSend_EmailFalseWritesNoOutbox(), TestSend_InsertErrorPropagates(), TestSend_NoRecipientsIsNotAnError(), TestSend_SkipsUnusedRecipientQueries(), TestSend_SMTPDisabledWritesSkipped() (+9 more)

### Community 105 - "newImportFixture"
Cohesion: 0.23
Nodes (11): testHelpers, newImportFixture(), TestMasterDataImportConfirm_InvalidFileStagesNothing_NoChangesIsAnError_Integration(), TestMasterDataImportConfirm_OneBadRowRollsBackTheWholeBatch_Integration(), TestMasterDataImportConfirm_OneBatchOneApproval_IdempotentPerFile_AppliedAtomically_Integration(), TestMasterDataImportConfirm_RejectRejectsWholeBatch_AndFileMayBeReuploaded_Integration(), TestMasterDataImportConfirm_RequiresMakerIdentity_Integration(), TestMasterDataImportConfirm_UpdateAndToggleRows_EffectOnlyAfterApproval_Integration() (+3 more)

### Community 106 - ".actorAndID"
Cohesion: 0.27
Nodes (8): net/http.HandlerFunc, VaultPlanHandler, VaultPlanServicer, formatPgDate(), chi.Router, NewVaultPlanHandler(), parseDateParam(), vaultPlanJSON()

### Community 107 - ".toggle"
Cohesion: 0.19
Nodes (8): AdminVendorVaultHandler, createVendorVaultRequestBody, VendorVaultAdminServicer, parseOptionalIDParam(), chi.Router, NewAdminVendorVaultHandler(), vaultToResponse(), formatTimePtr()

### Community 108 - "serveNotification"
Cohesion: 0.23
Nodes (16): notificationListResponse, notificationResponse, NotificationStore, NewNotificationHandler(), parseBoundedInt(), parseOptionalBool(), notificationRequest(), serveNotification() (+8 more)

### Community 109 - "New"
Cohesion: 0.26
Nodes (14): New(), activeUserIDs(), countFor(), pgx.Tx, integrationTx(), TestIntegration_PurgeDeletesOnlyOldRows(), TestIntegration_RepositoryScopesToOwner(), TestIntegration_SendExpandsVendorUsersAndPICs() (+6 more)

### Community 110 - "027_acm_vault_assignment.sql"
Cohesion: 0.20
Nodes (17): public.acm_area_branches, public.acm_area_members, public.acm_areas, public.dsr_location_vault_maps, public.vendor_request_vault_assignments, public.vendor_request_vault_plans, public, public.set_updated_at (+9 more)

### Community 111 - "replenish_classifications.sql.go"
Cohesion: 0.17
Nodes (10): CountReplenishClassificationsParams, ListReplenishClassificationsParams, ReplenishClassificationAmount, ReportReplenishTripsParams, ReportReplenishTripsRow, SummarizeReplenishClassificationsParams, SummarizeReplenishClassificationsRow, SummarizeReplenishInstructionsParams (+2 more)

### Community 112 - "AdminMasterDataImportHandler"
Cohesion: 0.19
Nodes (11): mime/multipart.File, mime/multipart.FileHeader, MasterDataImportHistoryServicer, MasterDataImportServicer, chi.Router, AdminMasterDataImportHandler, NewAdminMasterDataImportHandler(), queryInt() (+3 more)

### Community 113 - "mountRbacListHandler"
Cohesion: 0.24
Nodes (16): mountRbacListHandler(), numeric(), TestRbacListHandler_APPACCESS_Allowed(), TestRbacListHandler_CreatePolicy_BadBody(), TestRbacListHandler_CreatePolicy_HappyPath(), TestRbacListHandler_CreatePolicy_Validation(), TestRbacListHandler_ListDelegations_HappyPath(), TestRbacListHandler_ListEndpoint_StoreError() (+8 more)

### Community 114 - "newHistoryFixture"
Cohesion: 0.30
Nodes (12): ImportBatch, pgx.Tx, T, historyPtr(), newHistoryFixture(), TestImportHistory_DecidedFromLatestActedStep(), TestImportHistory_FileNameCheckConstraint(), TestImportHistory_FiltersAndPaging() (+4 more)

### Community 115 - "importEnv"
Cohesion: 0.26
Nodes (8): canonicalPicRolesCell(), TestParsePicRoles(), boolCol(), importEnv, parsePicRoles(), requiredCols(), validateVendor(), isValidEmail()

### Community 116 - "regionHarness"
Cohesion: 0.27
Nodes (16): longString(), queryRowScanRegion(), regionHarness(), TestCreateRegion_AuditFailureRollsBack(), TestCreateRegion_AuthorizationMatrix(), TestCreateRegion_CodeConflictRejectsAndCreatesNoRow(), TestCreateRegion_NormalizesCodeAndWritesAudit(), TestCreateRegion_ValidationRejectsBadCodeAndName() (+8 more)

### Community 117 - "031_replenish_realization.sql"
Cohesion: 0.15
Nodes (16): public.holidays, public.replenish_classification_amounts, public.replenish_classifications, public.vendor_request_realization_amounts, public.vendor_request_realizations, public, public.set_updated_at, public.users (+8 more)

### Community 118 - "DsrService"
Cohesion: 0.19
Nodes (6): encoding/json.RawMessage, DsrListUploadsParams, DsrService, DsrUploadResult, sanitizeFilename(), DsrDryRunResult

### Community 119 - "NewDsrService"
Cohesion: 0.19
Nodes (13): net/http.Client, redis.Client, NewDsrService(), TestDsrService_CheckUploadRate(), TestDsrService_ListUploads(), TestDsrService_ResolveVendor(), TestDsrService_SheetDetailIsVendorScoped(), TestDsrService_ValidateUpload() (+5 more)

### Community 120 - "mountAdminVendorBranchHandler"
Cohesion: 0.24
Nodes (13): TestAdminMasterDataUpdate_Returns202And400(), TestAdminVendorBranchHandler_UpdateAndEnableErrors(), mountAdminVendorBranchHandler(), TestAdminVendorBranchHandler_Create_HappyPath_Returns202WithChangeRequest(), TestAdminVendorBranchHandler_Create_ServiceConflict_MapsTo409(), TestAdminVendorBranchHandler_Create_ValidationError_MapsTo400(), TestAdminVendorBranchHandler_Disable_HappyPath_Returns202(), TestAdminVendorBranchHandler_Disable_PendingConflict_MapsTo409() (+5 more)

### Community 121 - "worker_test.go"
Cohesion: 0.20
Nodes (12): NewWorker(), newTestWorker(), TestNewWorker_Defaults(), TestWorker_RunStopsOnCancel(), TestWorker_SendBatch(), TestWorker_TickKeepsRetryingPurgeAfterError(), TestWorker_TickPurgesOncePerDay(), TestWorker_TickWithoutMailerSendsNothing() (+4 more)

### Community 122 - "MasterDataImportBatchRepository"
Cohesion: 0.20
Nodes (6): findOpenBatch(), ImportBatch, NewMasterDataImportBatchRepository(), BatchRowError, ImportBatchPool, MasterDataImportBatchRepository

### Community 123 - "notificationImportNotifier"
Cohesion: 0.23
Nodes (7): importNotifyBody(), NewImportNotifier(), TestImportNotifyBody(), warnImportNotify(), ImportNotifier, importNotifyQuerier, notificationImportNotifier

### Community 124 - "BuildChain"
Cohesion: 0.22
Nodes (8): ApproverInfo, ChainRepository, errNotFound, fakeChainRepository, BuildChain(), int32p(), int64p(), TestBuildChain()

### Community 125 - "Orchestrator"
Cohesion: 0.20
Nodes (7): AuditWriter, AvailabilityRepository, Orchestrator, RequestStore, ResolveEffectiveApprover(), TestResolveEffectiveApprover(), AuditWriter

### Community 126 - "Entry"
Cohesion: 0.20
Nodes (7): countingFailAudit, fakeAuditWriter, fakeAuditWriter, fakeAdminAuditWriter, Entry, marshalOrNil(), fakeMasterDataAuditWriter

### Community 127 - "NewDeactivateUserService"
Cohesion: 0.23
Nodes (10): DeactivateUserService, AuditWriter, NewDeactivateUserService(), TestDeactivateUserService_Deactivate_AuditWriteFails_PropagatesError(), TestDeactivateUserService_Deactivate_TargetNotFound(), TestDeactivateUserService_Deactivate_UserWithAuditHistory_IsSoftDeleted(), TestDeactivateUserService_Deactivate_UserWithoutAuditHistory_StillSoftDeleted(), TestDeactivateUserService_Deactivate_WritesAuditWithoutSecrets() (+2 more)

### Community 128 - "Queries"
Cohesion: 0.19
Nodes (6): CountHolidaysAdminParams, CreateHolidayParams, ListHolidaysAdminParams, SetHolidayActiveParams, UpdateHolidayNameParams, Queries

### Community 129 - "AuthHandler"
Cohesion: 0.22
Nodes (9): github.com/cimb-niaga/cms/pkg/middleware.RateLimiter, AuthHandler, changePasswordRequest, ChangePasswordService, loginRequest, clearRefreshCookie(), chi.Router, NewAuthHandler() (+1 more)

### Community 130 - "service_property_test.go"
Cohesion: 0.35
Nodes (13): pgregory.net/rapid.Generator, genNonWhitespaceString(), genValidPortalType(), genWhitespaceString(), int64Ptr(), nonWSRune(), printableRune(), TestProperty_Service_AuthErrorUniformity() (+5 more)

### Community 131 - "dsr_upload_handler.go"
Cohesion: 0.26
Nodes (13): confirmRequest, dsrDailyRowResponse, dsrRencanaIsiRowResponse, dsrRowErrorResponse, dsrSheetDetailResponse, dsrSheetResponse, dsrSheetSummaryResponse, dsrUploadListItemResponse (+5 more)

### Community 132 - "notification.go"
Cohesion: 0.33
Nodes (10): Message, Querier, Recipients, Service, normalizeEmail(), optional(), resolveUsers(), validate() (+2 more)

### Community 133 - "NewLocalProvider"
Cohesion: 0.26
Nodes (9): LocalProvider, NewLocalProvider(), hashPassword(), TestLocalProvider_Authenticate_NilPasswordHash(), TestLocalProvider_Authenticate_RepoError(), TestLocalProvider_Authenticate_UserNotFound(), TestLocalProvider_Authenticate_ValidPassword(), TestLocalProvider_Authenticate_WrongPassword() (+1 more)

### Community 134 - ".List"
Cohesion: 0.28
Nodes (5): AdminVendorPackageHandler, VendorPackageAdminServicer, chi.Router, NewAdminVendorPackageHandler(), packageToResponse()

### Community 135 - "mountAdminVendorPicHandler"
Cohesion: 0.29
Nodes (12): mountAdminVendorPicHandler(), TestAdminVendorPicHandler_Create_ErrorMapping(), TestAdminVendorPicHandler_Create_Returns202(), TestAdminVendorPicHandler_Get_IncludesRoles(), TestAdminVendorPicHandler_Get_NotFound(), TestAdminVendorPicHandler_List_IncludesWarnings(), TestAdminVendorPicHandler_SetRoles_ErrorMapping(), TestAdminVendorPicHandler_SetRoles_NonAdminForbidden() (+4 more)

### Community 136 - "numericFromDecimalPtr"
Cohesion: 0.22
Nodes (7): atmMoney(), pgx.Tx, pgx.Tx, numericFromDecimalPtr(), vaultNumerics(), ATMApplier, VendorVaultApplier

### Community 137 - "vendor_vaults_admin.sql.go"
Cohesion: 0.24
Nodes (6): CreateVendorVaultAdminParams, CreateVendorVaultAdminRow, UpdateVendorVaultAdminParams, UpdateVendorVaultAdminRow, ListVendorVaultsAdminParams, ListVendorVaultsAdminRow

### Community 138 - "atm_portal_filter_property_test.go"
Cohesion: 0.26
Nodes (11): containsFold(), pgx.Tx, insertTestATMWithLifecycle(), nullableDecimalOrderOK(), nullableTimeOrderOK(), sortedPairOK(), stringOrderOK(), strOrNil() (+3 more)

### Community 139 - "reasonProbePool"
Cohesion: 0.18
Nodes (7): pgx.Row, pgx.Rows, pgx.Tx, pgx.TxOptions, reasonGen(), TestProperty5_ReasonValidation(), reasonProbePool

### Community 140 - "public.vendor_request_vault_allocations"
Cohesion: 0.18
Nodes (11): public.acm_area_regions, public.vendor_request_vault_allocation_amounts, public.vendor_request_vault_allocations, public, public.set_updated_at, public.users, public.vendor_requests, public.vendor_vaults (+3 more)

### Community 141 - "GetMasterDataImportBatchRow"
Cohesion: 0.27
Nodes (4): GetMasterDataImportBatchRow, ListMasterDataImportBatchesRow, ImportBatchFilter, fakeHistoryRepo

### Community 142 - "github.com/jackc/pgx/v5/pgconn.CommandTag"
Cohesion: 0.18
Nodes (6): github.com/jackc/pgx/v5/pgconn.CommandTag, pgx.Row, pgx.Rows, TestRepository_WriteReadTopology(), dbtxProbe, probeRow

### Community 143 - "mountAdminMasterDataChangeHandler"
Cohesion: 0.35
Nodes (10): mountAdminMasterDataChangeHandler(), TestAdminMasterDataChangeHandler_Get_HappyPath_IncludesPayloadAndBefore(), TestAdminMasterDataChangeHandler_Get_InternalError(), TestAdminMasterDataChangeHandler_Get_InvalidID_BadRequest(), TestAdminMasterDataChangeHandler_Get_NotFound(), TestAdminMasterDataChangeHandler_List_EntityTypeAndStatusFilter_PassedThrough(), TestAdminMasterDataChangeHandler_List_HappyPath(), TestAdminMasterDataChangeHandler_List_InvalidStatus_BadRequest() (+2 more)

### Community 144 - "NewRbacReadRepository"
Cohesion: 0.25
Nodes (6): NewRbacReadRepository(), TestRbacReadRepository_ListApprovalPolicies_ExactMoney(), TestRbacReadRepository_ListDelegations_OnlyActive(), TestRbacReadRepository_ListLeaves(), TestRbacReadRepository_ListUserHierarchy(), RbacReadRepository

### Community 145 - "Queries"
Cohesion: 0.24
Nodes (4): InsertVendorRealizationAmountParams, ListRequestTicketOrdersRow, UpsertVendorRealizationParams, Queries

### Community 146 - "ListVaultTicketsForRequestRow"
Cohesion: 0.27
Nodes (5): IssueVaultTicketParams, LockVaultTicketSeqParams, SetVaultTicketActiveParams, ListVaultTicketsForRequestRow, Queries

### Community 147 - "mountRoleMgmtHandler"
Cohesion: 0.38
Nodes (9): mountRoleMgmtHandler(), TestRoleMgmtHandler_CreateRole_ErrorMapping(), TestRoleMgmtHandler_CreateRole_HappyPath(), TestRoleMgmtHandler_ListCatalog_HappyPath(), TestRoleMgmtHandler_ListRoles_GroupsPermissionsByRole(), TestRoleMgmtHandler_NoToken_Unauthorized(), TestRoleMgmtHandler_RBAC(), TestRoleMgmtHandler_UpdateRolePermissions_ErrorMapping() (+1 more)

### Community 148 - "admin_repository_topology_test.go"
Cohesion: 0.20
Nodes (5): pgx.Row, pgx.Rows, TestAuthRepository_PropagatesDBErrors(), poolProbe, poolProbeRow

### Community 149 - "dbtxProbe"
Cohesion: 0.20
Nodes (5): pgx.Row, pgx.Rows, TestRepository_WriteReadTopology(), dbtxProbe, probeRow

### Community 150 - "readImportCSV"
Cohesion: 0.36
Nodes (9): cleanCell(), cleanRecord(), expandLegacy(), headerMatches(), invalidText(), legacyPick(), readImportCSV(), spansOverlap() (+1 more)

### Community 151 - "003_master_data.sql"
Cohesion: 0.22
Nodes (9): public.master_data_change_requests, public.vendor_pics, public.set_updated_at, public.users, public.vendor_branches, public.vendors, trg_master_data_change_requests_set_updated_at, trg_vendor_pics_set_updated_at (+1 more)

### Community 152 - "mountAdminBranchATMHandler"
Cohesion: 0.42
Nodes (8): mountAdminBranchATMHandler(), TestAdminBranchATMHandler_List_BadPageSize(), TestAdminBranchATMHandler_List_BadPathID(), TestAdminBranchATMHandler_List_BranchNotFound(), TestAdminBranchATMHandler_List_EmptyResult(), TestAdminBranchATMHandler_List_NoToken_Unauthorized(), TestAdminBranchATMHandler_List_WithRows(), TestAdminBranchATMHandler_List_WrongRole_Forbidden()

### Community 153 - "mountAdminVendorPackageHandler"
Cohesion: 0.42
Nodes (8): mountAdminVendorPackageHandler(), TestAdminVendorPackageHandler_Create_ErrorMapping(), TestAdminVendorPackageHandler_Create_Returns202(), TestAdminVendorPackageHandler_Disable_Returns202(), TestAdminVendorPackageHandler_Get_NotFound(), TestAdminVendorPackageHandler_List(), TestAdminVendorPackageHandler_Update_Returns202(), TestAdminVendorPackageHandler_WrongRole_Forbidden()

### Community 154 - "acmAreaHarness"
Cohesion: 0.53
Nodes (8): acmAreaHarness(), countAudit(), pgx.Tx, seedAcmUser(), seedRow(), TestAcmAreaAdmin_AuditFailureRollsBack_Integration(), TestAcmAreaAdmin_Lifecycle_Integration(), TestAcmAreaAdmin_Regions_Integration()

### Community 155 - "IsVendor"
Cohesion: 0.33
Nodes (7): ForParty(), IsVendor(), PartyRoles(), TestForParty(), TestIsVendor(), TestPartyRoles(), TestReturnedSlicesDoNotAliasMapping()

### Community 156 - "public.vendor_package_prices"
Cohesion: 0.22
Nodes (8): public.cimb_branches, public.package_frequencies, public.vendor_package_prices, public, public.atms, public.regions, public.vendor_branches, public.vendors

### Community 157 - "019_import_jobs.sql"
Cohesion: 0.31
Nodes (8): public.import_jobs, public.late_detections, public.retry_audit_logs, public.scan_runs, public.set_updated_at, public.users, trg_import_jobs_set_updated_at, trg_late_detections_set_updated_at

### Community 158 - "021_atm_visit_quota.sql"
Cohesion: 0.31
Nodes (8): public.atm_visit_quotas, public.atm_visits, public.vendor_request_atm_results, public.atms, public.set_updated_at, public.users, public.vendor_requests, trg_atm_visit_quotas_set_updated_at

### Community 159 - "mountAdminVendorVaultHandler"
Cohesion: 0.46
Nodes (7): mountAdminVendorVaultHandler(), TestAdminVendorVaultHandler_Create_ErrorMapping(), TestAdminVendorVaultHandler_Create_Returns202(), TestAdminVendorVaultHandler_Get_NotFound(), TestAdminVendorVaultHandler_List_DecimalStringsInResponse(), TestAdminVendorVaultHandler_Toggle_Returns202(), TestAdminVendorVaultHandler_WrongRole_Forbidden()

### Community 160 - ".SetRoles"
Cohesion: 0.43
Nodes (6): TestNormalizePicRoles(), AllPicRoles(), NewVendorPicRoleService(), normalizePicRoles(), VendorPicRolePool, VendorPicRoleService

### Community 161 - "public.export_jobs"
Cohesion: 0.29
Nodes (7): public.export_job_chunks, public.export_jobs, public.set_updated_at, public.users, public.vendor_branches, public.vendors, trg_export_jobs_set_updated_at

### Community 162 - "mountAdminVendorPackagePriceHandler"
Cohesion: 0.60
Nodes (5): mountAdminVendorPackagePriceHandler(), TestAdminVendorPackagePriceHandler_ErrorMapping(), TestAdminVendorPackagePriceHandler_ListAndGet(), TestAdminVendorPackagePriceHandler_MutationsReturn202(), TestAdminVendorPackagePriceHandler_RBAC()

### Community 163 - "SMTPMailer"
Cohesion: 0.47
Nodes (3): SMTPMailer, orDefault(), TestSMTPMailer_DefaultTimeouts()

### Community 164 - "NewMasterDataChangeRepository"
Cohesion: 0.40
Nodes (4): NewMasterDataChangeRepository(), pgx.Tx, integrationTx(), TestMasterDataImportBatchRepository()

### Community 165 - ".Apply"
Cohesion: 0.47
Nodes (3): ensureBranchHasNoActiveChildren(), pgx.Tx, VendorBranchApplier

### Community 166 - "masterdata_import_notify_test.go"
Cohesion: 0.33
Nodes (3): TestImportEntityLabelsCoverEveryImportEntity(), TestImportNotifier_QuerierFailureIsSwallowed(), failingNotifyQuerier

### Community 167 - "SubmitRequest"
Cohesion: 0.47
Nodes (4): collectedChange, fakeVendorBranchSubmitter, importCollector, SubmitRequest

### Community 171 - "public.notifications"
Cohesion: 0.50
Nodes (4): public.notification_emails, public.notifications, public, public.users

### Community 172 - "public.dsr_receipts"
Cohesion: 0.40
Nodes (4): public.dsr_receipts, public, public.users, public.vendors

### Community 174 - "TestMasterDataExportRepository_KeysetPagingIsCompleteAndExact"
Cohesion: 0.67
Nodes (3): R, idsOf(), TestMasterDataExportRepository_KeysetPagingIsCompleteAndExact()

### Community 175 - "no_hard_delete_test.go"
Cohesion: 0.83
Nodes (3): stripSQLLineComments(), TestNoHardDeletePattern_ActuallyFires(), TestQueries_NoHardDelete()

### Community 176 - "public.master_data_import_batches"
Cohesion: 0.50
Nodes (3): public.master_data_import_batches, public.users, public.master_data_change_requests

### Community 177 - "public.branch_coverage_areas"
Cohesion: 0.50
Nodes (3): public.branch_coverage_areas, public, public.vendor_branches

## Knowledge Gaps
- **51 isolated node(s):** `github.com/cimb-niaga/cms/backend`, `AdminStore`, `AuditWriter`, `AtmDenom`, `ExportJobChunk` (+46 more)
  These have ≤1 connection - possible missing edges or undocumented components.
- **29 thin communities (<3 nodes) omitted from report** — run `graphify query` to explore isolated nodes.

## Suggested Questions
_Questions this graph is uniquely positioned to answer:_

- **Why does `main()` connect `main` to `context.Context`, `AuthHandler`, `Queries`, `harness`, `NewLocalProvider`, `testing.T`, `.List`, `DBTX`, `net/http.Handler`, `fakeUserAdminRepo`, `NewMasterDataExportRepository`, `NewATMAssignmentAdminService`, `parsePathID`, `writeError`, `adminCtx`, `NewRbacReadRepository`, `time.Time`, `atm_assignments_admin.sql.go`, `Region`, `Queries`, `AuditLog`, `tokenFor`, `Role`, `DsrLocationMapAdminService`, `DsrLateFilter`, `testTokenService`, `fakeVendorPicAdminServicer`, `net/http.ResponseWriter`, `.SetRoles`, `branch_atm_test.go`, `NewDmaaForecastService`, `NewUserAdminService`, `NewMasterDataChangeRepository`, `net/http.Request`, `fakeATMAdminRepo`, `fakeVendorBranchAdminServicer`, `jakartaCalendarDate`, `RbacListHandler`, `harness`, `fakeVendorAdminRepo`, `vendor_request_response.go`, `vendor_package_admin.go`, `vendor_package_price_admin.go`, `GetVendorPackageAdminByIDRow`, `vendor_package_prices_admin.sql.go`, `formatTimestamptz`, `NewChangePasswordService`, `writeUnauthorized`, `holiday_admin.go`, `VendorVaultAdminService`, `.withFilter`, `atm_admin_test.go`, `writeJSON`, `NewVendorAdminService`, `NewAtmPortalService`, `newAuditWriter`, `github.com/cimb-niaga/cms/pkg/auth.TokenService`, `NewOrchestrator`, `ApprovalPolicy`, `admin_atm_assignment_handler_test.go`, `VendorPicAdminService`, `.Get`, `mountCashposHandler`, `NewService`, `.actorAndID`, `.toggle`, `serveNotification`, `New`, `AdminMasterDataImportHandler`, `NewDsrService`, `worker_test.go`, `MasterDataImportBatchRepository`, `NewDeactivateUserService`?**
  _High betweenness centrality (0.039) - this node is a cross-community bridge._
- **Why does `assertDsrLateSummary()` connect `DsrLateReportRepository` to `context.Context`, `github.com/jackc/pgx/v5/pgtype.Timestamptz`, `testing.T`?**
  _High betweenness centrality (0.023) - this node is a cross-community bridge._
- **Why does `assertDsrLateWarnings()` connect `DsrLateReportRepository` to `context.Context`, `github.com/jackc/pgx/v5/pgtype.Timestamptz`, `testing.T`?**
  _High betweenness centrality (0.023) - this node is a cross-community bridge._
- **Are the 269 inferred relationships involving `doRequest()` (e.g. with `TestAdminAcmAreaHandler()` and `TestAdminApprovalHandler_AdminRole_SetHierarchy_HappyPath()`) actually correct?**
  _`doRequest()` has 269 INFERRED edges - model-reasoned connections that need verification._
- **Are the 224 inferred relationships involving `tokenForRole()` (e.g. with `TestAdminAcmAreaHandler()` and `TestAdminATMAssignmentHandler_CoMountedWithATMCrudRouter()`) actually correct?**
  _`tokenForRole()` has 224 INFERRED edges - model-reasoned connections that need verification._
- **What connects `github.com/cimb-niaga/cms/backend`, `AdminStore`, `AuditWriter` to the rest of the system?**
  _51 weakly-connected nodes found - possible documentation gaps or missing edges._
- **Should `context.Context` be split into smaller, more focused modules?**
  _Cohesion score 0.02729292288162488 - nodes in this community are weakly interconnected._