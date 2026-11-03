# Graph Report - backend  (2026-11-02)

## Corpus Check
- 461 files · ~701,455 words
- Verdict: corpus is large enough that graph structure adds value.

## Summary
- 5493 nodes · 16836 edges · 284 communities (255 shown, 29 thin omitted)
- Extraction: 85% EXTRACTED · 15% INFERRED · 0% AMBIGUOUS · INFERRED: 2482 edges (avg confidence: 0.85)
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
- handler/integration_test.go
- mountAuditLogHandler
- fakeUserAdminRepo
- vault_plan.go
- Queries
- Queries
- master_data_export.sql.go
- NewATMAssignmentAdminService
- writeJSON
- net/http.ResponseWriter
- masterdata_import_test.go
- ReplenishmentFilter
- github.com/jackc/pgx/v5/pgtype.Timestamptz
- atm_assignments_admin.sql.go
- fakeRegionAdminServicer
- Queries
- audit_log_read.go
- vendor_parties.sql.go
- tokenFor
- Role
- dsr_location_maps_admin.sql.go
- DsrLateFilter
- net/http.Handler
- vendor_pics_admin.sql.go
- net/http.Request
- 001_baseline_schema.sql
- profileFakeRepo
- .Create
- branch_atm_test.go
- NewDmaaForecastService
- vendor_party.go
- NewUserAdminService
- dsr_late_report.sql.go
- writeUnexpectedError
- doRequest
- fakeATMAdminRepo
- fakeVendorBranchAdminServicer
- jakartaCalendarDate
- setupForecastQueryHarness
- mountVendorRequestHandler
- ApprovalRequest
- main
- RbacListHandler
- vault_allocation.go
- harness
- fakeVendorAdminRepo
- vendor_request_response.go
- atm_admin.go
- vendor_package_admin.go
- vendor_package_price_admin.go
- github.com/jackc/pgx/v5/pgtype.Numeric
- vendor_package_prices_admin.sql.go
- formatTimestamptz
- NewChangePasswordService
- Queries
- writeValidationError
- fakeDsrRepository
- NewVendorBranchAdminService
- openapi_contract_test.go
- holiday_admin.go
- VendorVaultAdminService
- masterdata_export_test.go
- .withFilter
- testing.T
- time.Time
- Queries
- atm_visit_quota.sql.go
- atm_admin_test.go
- MasterDataChangeRequest
- Queries
- AdminATMHandler
- NewVendorAdminService
- admin_vendor_handler_test.go
- newServiceUnderTest
- .Get
- vault_allocations.sql.go
- .Confirm
- NewAtmPortalService
- vendor_request.go
- setupQuotaFixture
- masterdata_import_history.go
- github.com/cimb-niaga/cms/pkg/auth.TokenService
- NewOrchestrator
- masterdata_import.go
- propertyTestPool
- adminCtx
- Queries
- masterdata_import_confirm_test.go
- .Apply
- NewApprovalPolicyStore
- admin_atm_assignment_handler_test.go
- dsr_upload.go
- masterdata_import_confirm.go
- VendorPicAdminService
- .Submit
- extractClientIP
- masterdata_approval_notify_test.go
- writeUnauthorized
- notifyRig
- NewService
- newImportFixture
- .actorAndID
- .toggle
- serveNotification
- New
- 027_acm_vault_assignment.sql
- github.com/jackc/pgx/v5/pgtype.Date
- AdminMasterDataImportHandler
- mountRbacListHandler
- newHistoryFixture
- importEnv
- regionHarness
- 031_replenish_realization.sql
- .ListUploads
- DsrService
- mountAdminVendorBranchHandler
- Worker
- MasterDataImportBatchRepository
- atm_portal_profile.go
- BuildChain
- Orchestrator
- Entry
- NewDeactivateUserService
- Holiday
- DsrLocationMapAdminService
- service_property_test.go
- dsr_upload_handler.go
- RegionAdminService
- NewLocalProvider
- stubAtmPortalService
- mountAdminVendorPicHandler
- numericFromDecimalPtr
- DsrLateReportRepository
- atm_portal_filter_property_test.go
- reasonProbePool
- public.vendor_request_vault_allocations
- MasterDataApprovalService
- dbtxProbe
- mountAdminMasterDataChangeHandler
- NewRbacReadRepository
- ListVendorRealizationsForRequestRow
- ListVaultTicketsForRequestRow
- mountRoleMgmtHandler
- admin_repository_topology_test.go
- github.com/jackc/pgx/v5/pgconn.CommandTag
- atm_portal.go
- 003_master_data.sql
- AdminVendorPicHandler
- mountAdminVendorPackageHandler
- acmAreaHarness
- IsVendor
- public.vendor_package_prices
- 019_import_jobs.sql
- 021_atm_visit_quota.sql
- mountAdminVendorVaultHandler
- .Apply
- public.export_jobs
- mountAdminVendorPackagePriceHandler
- VendorPackagePrice
- MasterDataChangeRepository
- .Apply
- RoleMgmtHandler
- atm_portal_integration_test.go
- AuditLog
- Region
- VendorPackage
- public.notifications
- public.dsr_receipts
- stubProvider
- TestMasterDataImport_PicRoles_Integration
- VaultPlanDetail
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
- NewAuditLogReadService
- TestVendorPicRoleService_SetRoles_Integration
- ListForecastForDateRow
- audit_log_handler.go
- GetVendorPackagePriceAdminByIDRow
- AuditLogFilter
- RegionAdminRepository
- .Apply
- .ChangePassword
- syncVaultTickets
- .ListAuditLogs
- auth_handler_refresh_test.go
- NewRepository
- NewRepository
- TestIntegration_VendorPackagePriceApplier
- vendorSide
- NewWriter
- TestEndToEnd_ThreeLevelHierarchyWithDelegationOnLeave

## God Nodes (most connected - your core abstractions)
1. `doRequest()` - 295 edges
2. `tokenForRole()` - 242 edges
3. `writeError()` - 186 edges
4. `writeJSON()` - 169 edges
5. `MasterDataChangeRequest` - 130 edges
6. `Actor` - 112 edges
7. `New()` - 111 edges
8. `main()` - 96 edges
9. `writeUnauthorized()` - 92 edges
10. `Queries` - 91 edges

## Surprising Connections (you probably didn't know these)
- `TestIntegration_VendorRequestTransition_Guards()` --calls--> `status()`  [INFERRED]
  internal/service/vendor_request_rules_integration_test.go → cmd/migrate/runner_integration_test.go
- `main()` --calls--> `NewOrchestrator()`  [EXTRACTED]
  cmd/api/main.go → internal/approval/orchestrator.go
- `main()` --calls--> `NewRepository()`  [EXTRACTED]
  cmd/api/main.go → internal/approval/repository.go
- `main()` --calls--> `NewWriter()`  [EXTRACTED]
  cmd/api/main.go → internal/audit/writer.go
- `main()` --calls--> `NewChangePasswordService()`  [EXTRACTED]
  cmd/api/main.go → internal/auth/change_password.go

## Import Cycles
- None detected.

## Communities (284 total, 29 thin omitted)

### Community 0 - "context.Context"
Cohesion: 0.03
Nodes (24): mockUserRepository, stubRateLimiter, stubUserRepo, context.Context, github.com/cimb-niaga/cms/pkg/auth.UserRecord, fakeDeactivateService, fakeDsrLocationMapServicer, fakeMeRepo (+16 more)

### Community 1 - "Actor"
Cohesion: 0.08
Nodes (30): fakeVendorRequestServicer, extractMetadata(), extractState(), VendorRequestService, checkActor(), checkCompletionActor(), CompletionResultInput, VendorRequestService (+22 more)

### Community 2 - "Queries"
Cohesion: 0.04
Nodes (35): AddAcmAreaBranchParams, AddAcmAreaMemberParams, AddAcmAreaRegionParams, CreateAcmAreaParams, ListBranchesInOtherAcmAreasParams, ListRegionsInOtherAcmAreasParams, RenameAcmAreaParams, SetAcmAreaActiveParams (+27 more)

### Community 3 - "atm_portal_handler.go"
Cohesion: 0.20
Nodes (23): net/url.Values, atmPortalRow, atmProfileResponse, atmSummaryResponse, cashposRow, listATMCashposResponse, listATMReplenishResponse, listATMsResponse (+15 more)

### Community 4 - "harness"
Cohesion: 0.16
Nodes (34): approveAssignment(), approveATM(), assignmentFixture(), dbtx(), testHelpers, harness(), jsonOrNil(), mustJSON() (+26 more)

### Community 5 - "handler/integration_test.go"
Cohesion: 0.18
Nodes (24): net/http/httptest.ResponseRecorder, errorResponseBody, loginResponseBody, testHarness, doLogin(), doLogout(), doRefresh(), extractRefreshCookie() (+16 more)

### Community 6 - "mountAuditLogHandler"
Cohesion: 0.27
Nodes (13): NewAuditLogHandler(), mountAuditLogHandler(), sampleListItem(), TestAuditLogGetByID_NonIntegerID(), TestAuditLogGetByID_NotFound(), TestAuditLogGetByID_SuccessIncludesBeforeAfter(), TestAuditLogList_BadRequestCases(), TestAuditLogList_DateFromAfterDateTo_ValidationErrorMapsTo400() (+5 more)

### Community 7 - "fakeUserAdminRepo"
Cohesion: 0.04
Nodes (22): fakeUserAdminRepo, fakeVendorOptionsReader, FindUserAdminByEmailExcludingIDParams, FindUserAdminByEmployeeIDExcludingIDParams, VendorBranchBelongsToVendorParams, fakeUserAdminServicer, CreateUserRequest, VendorOptions (+14 more)

### Community 8 - "vault_plan.go"
Cohesion: 0.15
Nodes (26): allocatedByVault(), TestVaultCapacity(), vaultCapacity(), vaultSaldoFromRow(), amountsJSON(), capacityWarning(), coverage(), snapshotBytes() (+18 more)

### Community 9 - "Queries"
Cohesion: 0.04
Nodes (25): GetActiveVendorForTerminalParams, GetActiveVendorForTerminalRow, GetVendorForRequestNumberRow, IssueVendorRequestTicketParams, LockTicketSeqParams, SetVendorRequestTicketActiveParams, SetVendorRequestTicketResultParams, UpdateVendorRequestCompletionParams (+17 more)

### Community 10 - "Queries"
Cohesion: 0.07
Nodes (17): CountNotificationsForUserParams, ListNotificationsForUserParams, MarkNotificationReadParams, ClaimPendingNotificationEmailsRow, InsertNotificationEmailParams, InsertNotificationParams, ListActiveUserRecipientsByIDsRow, ListActiveUserRecipientsByRolesRow (+9 more)

### Community 11 - "master_data_export.sql.go"
Cohesion: 0.06
Nodes (28): ExportATMAssignmentsBatchParams, ExportATMsBatchParams, ExportVendorBranchesBatchParams, ExportVendorPicsBatchParams, ExportVendorsBatchParams, ExportVendorVaultsBatchParams, ExportATMAssignmentsBatchRow, ExportATMsBatchRow (+20 more)

### Community 12 - "NewATMAssignmentAdminService"
Cohesion: 0.07
Nodes (43): dateToStringPtr(), ATMAssignment, ATMAssignmentUpdatePayload, newATMAssignment(), NewATMAssignmentAdminService(), date(), ATMAssignmentUpdatePayload, TestATMAssignmentAdminService_Create() (+35 more)

### Community 13 - "writeJSON"
Cohesion: 0.07
Nodes (27): AdminATMAssignmentHandler, AdminVendorBranchHandler, AdminVendorHandler, AdminVendorPackageHandler, AdminVendorPackagePriceHandler, createVendorBranchRequestBody, updateVendorBranchRequestBody, VendorBranchAdminServicer (+19 more)

### Community 14 - "net/http.ResponseWriter"
Cohesion: 0.08
Nodes (26): net/http.ResponseWriter, AdminDsrLocationMapHandler, AtmPortalHandler, AuditLogHandler, AuthHandler, changePasswordRequest, ChangePasswordService, DsrLocationMapAdminServicer (+18 more)

### Community 15 - "masterdata_import_test.go"
Cohesion: 0.13
Nodes (31): NewMasterDataImporter(), TestImport_VendorPICLegacyHeaderKeepsRoles(), TestImport_VendorPICRoles(), TestImport_VendorPICRolesChangeIsAnUpdate(), TestImport_VendorPICUnsortedStoredRolesRoundTripUnchanged(), dryRun(), hasErr(), TestAssignmentFields_BySource() (+23 more)

### Community 16 - "ReplenishmentFilter"
Cohesion: 0.09
Nodes (30): fakeReplenishmentReader, goldenReplenishmentReader, T, goldenDailyRows(), goldenMismatchRows(), ptr(), wibAt(), classificationViews() (+22 more)

### Community 17 - "github.com/jackc/pgx/v5/pgtype.Timestamptz"
Cohesion: 0.07
Nodes (53): AcmAreaBranch, AcmAreaMember, AcmAreaRegion, Atm, AtmDenom, AtmVendorPackage, AtmVisitQuota, BranchCoverageArea (+45 more)

### Community 18 - "atm_assignments_admin.sql.go"
Cohesion: 0.05
Nodes (22): CloseATMAssignmentsBeforeStartParams, CreateATMAssignmentAdminParams, CreateATMAssignmentAdminRow, DisableATMAssignmentsStartingOnParams, FindOverlappingATMAssignmentParams, UpdateATMAssignmentAdminParams, UpdateATMAssignmentAdminRow, fakeATMAssignmentAdminServicer (+14 more)

### Community 19 - "fakeRegionAdminServicer"
Cohesion: 0.15
Nodes (5): fakeRegionAdminServicer, CountRegionsAdminParams, GetRegionAdminByIDRow, ListRegionsAdminParams, ListRegionsAdminRow

### Community 20 - "Queries"
Cohesion: 0.10
Nodes (30): contractOrderSvc, fakeVendorOrderServicer, Writer, Queries, pgx.Tx, VendorRequest, contractOrderFull(), RealizationRowInput (+22 more)

### Community 21 - "audit_log_read.go"
Cohesion: 0.28
Nodes (11): stubAuditLogService, clampAuditLogPage(), clampAuditLogPageSize(), AuditLogDetail, AuditLogListItem, ListAuditLogsParams, ListAuditLogsResult, toAuditLogDetail() (+3 more)

### Community 22 - "vendor_parties.sql.go"
Cohesion: 0.10
Nodes (14): CountVendorOrdersParams, DecideVendorPartyParams, GetUserVendorScopeRow, GetVendorOrderParams, InsertVendorPartyEventParams, InsertVendorPartyParams, ListVendorOrdersParams, ListVendorPartiesForRequestRow (+6 more)

### Community 23 - "tokenFor"
Cohesion: 0.08
Nodes (56): ChangePasswordService, ApprovalOrchestrator, ApprovalReader, approvalRequestResponse, detailBody, fakeApplyRetrier, inboxItemResponse, submitRequest (+48 more)

### Community 24 - "Role"
Cohesion: 0.05
Nodes (15): HasRolePermissionParams, InsertRolePermissionParams, fakeRoleMgmtServicer, MenuFeature, Role, CreateRoleParams, ListRolesWithPermissionsRow, Queries (+7 more)

### Community 25 - "dsr_location_maps_admin.sql.go"
Cohesion: 0.09
Nodes (11): CreateDsrLocationMapParams, CreateDsrLocationMapRow, FindActiveDsrLocationMapParams, UpdateDsrLocationMapParams, UpdateDsrLocationMapRow, GetDsrLocationMapAdminByIDRow, ListDsrLocationMapsAdminParams, ListDsrLocationMapsAdminRow (+3 more)

### Community 26 - "DsrLateFilter"
Cohesion: 0.07
Nodes (36): github.com/cimb-niaga/cms/pkg/reports.AmountMismatchRow, github.com/cimb-niaga/cms/pkg/reports.DsrLateDailyRow, github.com/cimb-niaga/cms/pkg/reports.DsrLateSummaryRow, github.com/cimb-niaga/cms/pkg/reports.TripRow, contractDsrLate, fakeDsrLateReader, goldenDsrLateReader, pagedGoldenDsrLateReader (+28 more)

### Community 27 - "net/http.Handler"
Cohesion: 0.07
Nodes (50): math/rand.Rand, net/http.Handler, cutoverTarget, fakeReturner, TestAdminHolidayHandler(), TestAdminHolidayHandler_Import(), importRouter(), TestAdminMasterDataImportHandler_Anonymous_401() (+42 more)

### Community 28 - "vendor_pics_admin.sql.go"
Cohesion: 0.09
Nodes (12): CreateVendorPicAdminParams, CreateVendorPicAdminRow, GetVendorPicRolesForUpdateParams, SetVendorPicRolesParams, SetVendorPicRolesRow, UpdateVendorPicAdminParams, UpdateVendorPicAdminRow, GetVendorPicAdminByIDRow (+4 more)

### Community 29 - "net/http.Request"
Cohesion: 0.07
Nodes (30): net/http.Request, AtmVisitQuotaHandler, cancelBody, completionBody, createVendorRequestBody, itemInputPayload, NotificationHandler, notificationListResponse (+22 more)

### Community 30 - "001_baseline_schema.sql"
Cohesion: 0.08
Nodes (50): public.approval_delegations, public.approval_policies, public.approval_requests, public.approval_steps, public.atm_denoms, public.atm_vendor_packages, public.atms, public.audit_logs (+42 more)

### Community 31 - "profileFakeRepo"
Cohesion: 0.07
Nodes (18): github.com/jackc/pgx/v5/pgtype.Timestamp, CountATMsWithCashPosParams, CountCashposByTerminalParams, CountItmCashposParams, CountReplenishByTerminalParams, GetATMByTerminalIDRow, GetATMSummaryRow, ListATMsWithCashPosParams (+10 more)

### Community 32 - ".Create"
Cohesion: 0.08
Nodes (41): resolveItems(), validateItemsPayload(), acceptItems(), attemptCreateWithNumber(), categoryOrNil(), createWithRetryingNumber(), fallbackVendorPrefix(), pgx.Tx (+33 more)

### Community 33 - "branch_atm_test.go"
Cohesion: 0.09
Nodes (27): AdminBranchATMHandler, BranchATMServicer, fakeBranchATMServicer, ListBranchATMsParams, ListBranchATMsRow, Queries, chi.Router, managedATMToResponse() (+19 more)

### Community 34 - "NewDmaaForecastService"
Cohesion: 0.08
Nodes (35): DmaaForecastHandler, dmaaForecastPagination, dmaaForecastResponse, dmaaForecastResponseRow, stubDmaaForecastService, CountDmaaForecastParams, ListDmaaForecastParams, Queries (+27 more)

### Community 35 - "vendor_party.go"
Cohesion: 0.06
Nodes (61): main(), run(), checkChecksums(), pgx.ConnConfig, pgx.ConnConfig, migrate(), queryInt(), status() (+53 more)

### Community 36 - "NewUserAdminService"
Cohesion: 0.07
Nodes (59): branchCall, ConflictError, ReferenceError, UserAdminRepo, UserAdminService, VendorOptionsReader, changesImmutable(), conflictFieldFromPgError() (+51 more)

### Community 37 - "dsr_late_report.sql.go"
Cohesion: 0.13
Nodes (12): CountDsrLateDailyParams, ListDsrLateDailyParams, ListDsrLateDailyRow, ListDsrLateSummaryParams, ListDsrLateSummaryRow, ListDsrLateUnmappedLabelsParams, ListDsrLateUnmappedLabelsRow, ListDsrLateUnmappedVaultsParams (+4 more)

### Community 38 - "writeUnexpectedError"
Cohesion: 0.14
Nodes (10): github.com/cimb-niaga/cms/pkg/middleware.AuthContext, AcmAreaAdminServicer, acmAreaNameBody, AdminAcmAreaHandler, acmAreaDetailJSON(), acmAreaJSON(), chi.Router, NewAdminAcmAreaHandler() (+2 more)

### Community 39 - "doRequest"
Cohesion: 0.07
Nodes (96): AdminStore, TestAdminAcmAreaHandler(), mountAdminApprovalHandler(), TestAdminApprovalHandler_AdminRole_SetHierarchy_HappyPath(), TestAdminApprovalHandler_CreateDelegation_BadRequest(), TestAdminApprovalHandler_CreateDelegation_HappyPath(), TestAdminApprovalHandler_CreateDelegation_OverlapConflict(), TestAdminApprovalHandler_CreateLeave_BadRequest() (+88 more)

### Community 40 - "fakeATMAdminRepo"
Cohesion: 0.08
Nodes (11): CreateATMAdminParams, CreateATMAdminRow, UpdateATMAdminParams, UpdateATMAdminRow, CountATMsAdminParams, ListATMsAdminParams, ListATMsAdminRow, ListLocationsForSelectRow (+3 more)

### Community 41 - "fakeVendorBranchAdminServicer"
Cohesion: 0.09
Nodes (12): CreateVendorBranchAdminParams, CreateVendorBranchAdminRow, UpdateVendorBranchAdminParams, UpdateVendorBranchAdminRow, fakeVendorBranchAdminServicer, CountVendorBranchesAdminParams, GetVendorBranchAdminByIDRow, ListVendorBranchesAdminParams (+4 more)

### Community 42 - "jakartaCalendarDate"
Cohesion: 0.06
Nodes (70): github.com/jackc/pgx/v5/pgxpool.Pool, VendorRequestService, newVaultFlowFixture(), TestIntegration_VaultFlow_ApproveCreatesPlanAndCancelCascades(), TestIntegration_VaultFlow_CompletionFromReadyReturnsToReady(), TestIntegration_VaultFlow_RegionWithoutAreaGetsPlanOnMapping(), TestIntegration_VaultPlan_Guards(), vaultPlanFixture (+62 more)

### Community 43 - "setupForecastQueryHarness"
Cohesion: 0.22
Nodes (19): TestIntegration_VendorWideAssignment(), pgx.Tx, seedVendorWide(), TestIntegration_VendorWideAssignment_Applier(), TestIntegration_VendorWideAssignment_ForecastCountSummaryAndVendorResolver(), pgx.Tx, linkAtmVendorPackage(), seedForecastATM() (+11 more)

### Community 44 - "mountVendorRequestHandler"
Cohesion: 0.14
Nodes (43): TestVendorRequestHandler_UpdateItemsAndRevise(), mountAtmVisitQuota(), TestAtmVisitQuotaHandler_ErrorMapping(), TestAtmVisitQuotaHandler_RoleGates(), TestVendorRequestHandler_CompletionRoutes(), NewVendorRequestHandler(), mountVendorRequestHandler(), TestVendorRequestHandler_Approve_SelfApprovalForbidden() (+35 more)

### Community 45 - "ApprovalRequest"
Cohesion: 0.11
Nodes (9): fakeStore, fakeApprovalOrchestrator, fakeApprovalReader, docKey(), InboxItem, ApprovalRequest, runReject(), canApproveOnce (+1 more)

### Community 46 - "main"
Cohesion: 0.05
Nodes (31): branchChildCounterAdapter, getenvDefault(), loadNotificationConfig(), main(), DBTX, TestAdminGuardQueries(), TestAdminRepositories_ReadTopology(), NewATMAdminRepository() (+23 more)

### Community 47 - "RbacListHandler"
Cohesion: 0.10
Nodes (23): AdminApprovalHandler, AdminAuditWriter, AdminStore, ApprovalPolicyWriter, createDelegationRequest, createLeaveRequest, delegationResponse, leaveResponse (+15 more)

### Community 48 - "vault_allocation.go"
Cohesion: 0.12
Nodes (29): ListVaultCandidateVaultsRow, allocationTotals(), allocErr(), checkCoverage(), denomLabel(), VaultBranchOption, groupVaultOptions(), isAllocationDenom() (+21 more)

### Community 49 - "harness"
Cohesion: 0.12
Nodes (28): CreateRoleRequest, assertInt64Set(), assertPersistedPermissions(), harness(), queryRowScan(), TestCreateRole_AuditFailureRollsBack(), TestCreateRole_AuthorizationMatrix(), TestCreateRole_ConcurrentDuplicateCaughtByUniqueConstraint() (+20 more)

### Community 50 - "fakeVendorAdminRepo"
Cohesion: 0.09
Nodes (10): CreateVendorAdminParams, CreateVendorAdminRow, UpdateVendorAdminParams, UpdateVendorAdminRow, CountVendorsAdminParams, ListVendorsAdminParams, ListVendorsAdminRow, Queries (+2 more)

### Community 51 - "vendor_request_response.go"
Cohesion: 0.12
Nodes (32): atmVisitQuotaResponse, AtmVisitQuotaServicer, atmVisitResponse, auditLogEntryResponse, auditLogResponse, forecastResponse, forecastRowResponse, forecastSummaryGroupResponse (+24 more)

### Community 52 - "atm_admin.go"
Cohesion: 0.11
Nodes (18): fakeATMAdminServicer, atmFromGetRow(), atmFromListRow(), decimalStrings(), ATM, CreateATMRequest, LocationOption, UpdateATMRequest (+10 more)

### Community 53 - "vendor_package_admin.go"
Cohesion: 0.16
Nodes (15): VendorPackageCreatePayload, NewVendorPackageAdminService(), baseVendorPackageCreatePayload(), int64ptr(), TestVendorPackageAdminService_Create(), TestVendorPackageAdminService_Disable_NotYetEffective(), TestVendorPackageAdminService_Disable_Scoping(), TestVendorPackageAdminService_Update() (+7 more)

### Community 54 - "vendor_package_price_admin.go"
Cohesion: 0.20
Nodes (11): TestTierMaxInt32(), TestValidatePriceContent_BasePrice(), TestValidatePriceContent_OptionalFields(), VendorPackagePriceContentPayload, VendorPackagePriceCreatePayload, NewVendorPackagePriceAdminService(), validatePriceContent(), validatePriceGrain() (+3 more)

### Community 55 - "github.com/jackc/pgx/v5/pgtype.Numeric"
Cohesion: 0.07
Nodes (20): CreateVendorPackageAdminParams, CreateVendorPackageAdminRow, CreateVendorPackagePriceAdminParams, CreateVendorPackagePriceAdminRow, CreateVendorVaultAdminParams, CreateVendorVaultAdminRow, UpdateVendorPackageAdminParams, UpdateVendorPackageAdminRow (+12 more)

### Community 56 - "vendor_package_prices_admin.sql.go"
Cohesion: 0.15
Nodes (5): UpdateVendorPackagePriceAdminParams, UpdateVendorPackagePriceAdminRow, CountVendorPackagePricesAdminParams, GetVendorKindForPackagePriceRow, Queries

### Community 57 - "formatTimestamptz"
Cohesion: 0.10
Nodes (22): AdminUserHandler, createUserRequestBody, createVendorAdminRequestBody, DeactivateService, optionalInt64, setInitialPasswordRequest, SetInitialPasswordService, updateUserRequestBody (+14 more)

### Community 58 - "NewChangePasswordService"
Cohesion: 0.35
Nodes (11): AuditWriter, ChangePasswordService, NewChangePasswordService(), localUserWithPassword(), TestChangePasswordService_AuditWriteFails_PropagatesError(), TestChangePasswordService_HappyPath(), TestChangePasswordService_LDAPAccount_Rejected(), TestChangePasswordService_NewEqualsOld_Rejected() (+3 more)

### Community 59 - "Queries"
Cohesion: 0.04
Nodes (28): Repository, CreateApprovalDelegationParams, CreateApprovalRequestParams, CreateApprovalStepParams, CreateUserLeaveParams, FindActiveDelegateParams, FindActiveLeaveParams, FindApprovalPolicyParams (+20 more)

### Community 60 - "writeValidationError"
Cohesion: 0.13
Nodes (13): net/http.HandlerFunc, AdminHolidayHandler, HolidayAdminer, realizationBody, VendorOrderServicer, chi.Router, holidayID(), NewAdminHolidayHandler() (+5 more)

### Community 61 - "fakeDsrRepository"
Cohesion: 0.06
Nodes (16): detailFakeDsrRepo, scopeFakeDsrRepo, CountDsrUploadsByVendorParams, GetDsrUploadByIDForVendorParams, GetDsrUploadByIDForVendorRow, GetVendorByIDRow, ListDsrDailyRowErrorsRow, ListDsrDailyRowsRow (+8 more)

### Community 62 - "NewVendorBranchAdminService"
Cohesion: 0.13
Nodes (19): VendorBranchPayload, VendorBranchUpdatePayload, NewVendorBranchAdminService(), normalizeBranchRegionCode(), TestVendorBranchAdminService_Create_DuplicateCode_Conflict(), TestVendorBranchAdminService_Create_HappyPath_SubmitsCorrectRequest(), TestVendorBranchAdminService_Create_MissingFields_ValidationError(), TestVendorBranchAdminService_Disable_ActivePicsOrPackages_Refused() (+11 more)

### Community 63 - "openapi_contract_test.go"
Cohesion: 0.18
Nodes (23): github.com/getkin/kin-openapi/openapi3.T, github.com/getkin/kin-openapi/routers.Router, net/http.Header, contractCase, contractPartyReader, fakeVaultReviewer, contractCases(), contractDetailFull() (+15 more)

### Community 64 - "holiday_admin.go"
Cohesion: 0.14
Nodes (18): io.Reader, fakeHolidays, authorizeHolidayAdmin(), HolidayImportResult, HolidayList, HolidayView, holidayView(), ptrInt32() (+10 more)

### Community 65 - "VendorVaultAdminService"
Cohesion: 0.14
Nodes (16): fakeVendorVaultAdminServicer, VendorVault, VendorVaultPayload, VendorVaultUpdatePayload, newVendorVault(), NewVendorVaultAdminService(), parseDecimalField(), TestValidateVaultFields() (+8 more)

### Community 66 - "masterdata_export_test.go"
Cohesion: 0.23
Nodes (17): CSVSafe(), NewMasterDataExporter(), spec(), export(), newTestExporter(), TestCSVSafe(), TestExport_CellsAlignWithHeader(), TestExport_FailureModes() (+9 more)

### Community 67 - ".withFilter"
Cohesion: 0.12
Nodes (14): DsrLateHandler, DsrLateReader, ReplenishmentHandler, ReplenishmentReader, dsrLateError(), chi.Router, parseDsrLateFilter(), rejectCSVFormat() (+6 more)

### Community 68 - "testing.T"
Cohesion: 0.06
Nodes (67): testing.T, mountAdminRegionHandler(), regionName(), TestAdminRegionHandler_Create_CodeConflict_409(), TestAdminRegionHandler_Create_Created201(), TestAdminRegionHandler_Create_Forbidden_403(), TestAdminRegionHandler_Create_MalformedBody_400(), TestAdminRegionHandler_Create_UnexpectedError_500() (+59 more)

### Community 69 - "time.Time"
Cohesion: 0.12
Nodes (16): failingAvailability, fakeAvailabilityRepository, time.Time, fakePartyReader, TestResolveEffectiveApprover(), civilDate(), day(), dayPtr() (+8 more)

### Community 70 - "Queries"
Cohesion: 0.10
Nodes (11): IsAcmAreaMemberParams, ListAcmAreaMemberIDsByRoleParams, ListVaultPlanAtmsRow, ListVaultPlansParams, LockVaultPlanRow, SetVaultPlanStatusParams, SetVendorRequestStatusOnlyParams, SetVendorRequestVaultReviewParams (+3 more)

### Community 71 - "atm_visit_quota.sql.go"
Cohesion: 0.11
Nodes (14): AdjustAtmVisitQuotaParams, AtmVisit, CancelAtmVisitParams, EnsureAtmVisitQuotaParams, GetAtmVisitQuotaByAtmRow, InsertAtmVisitParams, ListAtmsForVendorQuotaResetParams, ListAtmsForVendorQuotaResetRow (+6 more)

### Community 72 - "atm_admin_test.go"
Cohesion: 0.21
Nodes (28): GetATMAdminByIDRow, NewATMAdminService(), activeATM(), atmRepoWith(), disabledATM(), TestATMAdminService_Count_PassesThroughToRepo(), TestATMAdminService_Create_InvalidLocationReference(), TestATMAdminService_Create_MoneyStaysExactAndBlanksBecomeNil() (+20 more)

### Community 73 - "MasterDataChangeRequest"
Cohesion: 0.14
Nodes (8): capturingReader, countingDetail, fakeMasterDataChangeReader, fakeMasterDataDetail, CreateMasterDataChangeRequestParams, ListMasterDataChangeRequestsParams, MasterDataChangeRequest, fakeMasterDataChangeRepo

### Community 74 - "Queries"
Cohesion: 0.06
Nodes (15): CountMasterDataImportBatchesParams, CreateMasterDataImportBatchParams, FindOpenMasterDataImportBatchParams, FindPendingMasterDataChangeRequestParams, FindPendingMasterDataChangeRequestRow, ListMasterDataImportBatchesParams, MarkMasterDataChangeRequestApplyFailedParams, MarkMasterDataChangeRequestStaleParams (+7 more)

### Community 75 - "AdminATMHandler"
Cohesion: 0.21
Nodes (7): AdminATMHandler, ATMAdminServicer, createATMAdminRequestBody, updateATMAdminRequestBody, atmToResponse(), chi.Router, NewAdminATMHandler()

### Community 76 - "NewVendorAdminService"
Cohesion: 0.10
Nodes (37): fakeVendorAdminServicer, GetVendorAdminByIDRow, CreateVendorRequest, DisableVendorResult, UpdateVendorRequest, isValidEmail(), NewVendorAdminService(), normalizeNPWP() (+29 more)

### Community 77 - "admin_vendor_handler_test.go"
Cohesion: 0.15
Nodes (26): deref(), equalStrPtr(), mountAdminVendorHandler(), strPtr(), TestAdminVendorHandler_Create_Accepted202(), TestAdminVendorHandler_Create_CodeConflict_409(), TestAdminVendorHandler_Create_InvalidNPWP_422(), TestAdminVendorHandler_Create_ValidationError_422() (+18 more)

### Community 78 - "newServiceUnderTest"
Cohesion: 0.20
Nodes (23): noopBlacklist, github.com/cimb-niaga/cms/pkg/auth.TokenConfig, activeKaryawanUser(), newServiceUnderTest(), TestService_Login_DeletedUser_GenericError(), TestService_Login_InactiveUser(), TestService_Login_InvalidCredentials(), TestService_Login_LDAPAccount_NeverLocked() (+15 more)

### Community 79 - ".Get"
Cohesion: 0.42
Nodes (5): AdminMasterDataChangeHandler, MasterDataChangeReader, chi.Router, masterDataChangeToResponse(), NewAdminMasterDataChangeHandler()

### Community 80 - "vault_allocations.sql.go"
Cohesion: 0.11
Nodes (14): CreateMissingVaultPlansByRegionParams, CreateMissingVaultPlansByRegionRow, InsertVaultAllocationAmountParams, InsertVaultAllocationParams, ListRequestOrderTotalsRow, ListVaultAllocatedTotalsParams, ListVaultSaldoParams, SetVaultPlanBufferReasonParams (+6 more)

### Community 81 - ".Confirm"
Cohesion: 0.15
Nodes (13): authorizeMasterDataMaker(), marshalOrNilJSON(), marshalPayloadJSON(), validateMasterDataOp(), MasterDataImporter, collectedChange, fakeVendorBranchSubmitter, importCollector (+5 more)

### Community 82 - "NewAtmPortalService"
Cohesion: 0.18
Nodes (17): mustNumeric(), sampleCashposRow(), TestListCashpos_CountError(), TestListCashpos_Empty(), TestListCashpos_ForwardsFilters(), TestListCashpos_MapsAllFieldsAndDecimalStrings(), TestListCashpos_RepoError(), TestListCashpos_Validation() (+9 more)

### Community 83 - "vendor_request.go"
Cohesion: 0.07
Nodes (36): fakeAtmVisitQuotaServicer, validateDateBound(), pgx.Tx, AtmVisitQuotaView, VendorQuotaResetResult, NewAtmVisitQuotaService(), packageLabel(), recordVisit() (+28 more)

### Community 84 - "setupQuotaFixture"
Cohesion: 0.20
Nodes (18): pgx.Rows, pgx.Tx, pgx.TxOptions, VendorRequestService, seedTicket(), setupQuotaFixture(), TestIntegration_VisitQuota_ApproveDecrements(), TestIntegration_VisitQuota_FailedApproveRollsBack() (+10 more)

### Community 85 - "masterdata_import_history.go"
Cohesion: 0.14
Nodes (24): batchCauseMessage(), TestBatchCauseMessage_AlwaysReadsAsCause(), authorizeMasterDataAdmin(), changeFrom(), countOps(), T, isFailedRow(), NewMasterDataImportHistoryService() (+16 more)

### Community 86 - "github.com/cimb-niaga/cms/pkg/auth.TokenService"
Cohesion: 0.08
Nodes (36): LoginRequest, LoginResponse, SetInitialPasswordService, github.com/cimb-niaga/cms/pkg/auth.Provider, github.com/cimb-niaga/cms/pkg/auth.RateLimiter, github.com/cimb-niaga/cms/pkg/auth.TokenService, github.com/cimb-niaga/cms/pkg/auth.UserRepository, github.com/cimb-niaga/cms/pkg/middleware.RateLimiter (+28 more)

### Community 87 - "NewOrchestrator"
Cohesion: 0.25
Nodes (22): fakeStore, NewOrchestrator(), stepStatuses(), submitInvoice(), TestOrchestrator_ApproveRejectErrorBranches(), TestOrchestrator_ApproverOnLeaveWithoutDelegateBlocksStep(), TestOrchestrator_DelegateActsWhileApproverOnLeave(), TestOrchestrator_FinalApproveAuditFailureIsReturned() (+14 more)

### Community 88 - "masterdata_import.go"
Cohesion: 0.10
Nodes (23): fakeImportServicer, assignmentSourceCell(), ImportConfirmResult, cleanCell(), cleanRecord(), expandLegacy(), headerMatches(), invalidText() (+15 more)

### Community 89 - "propertyTestPool"
Cohesion: 0.26
Nodes (20): keys(), TestListATMs_DateValidation(), TestProperty_DateRangeFilterCorrectness(), TestProperty4_StatusFilterCorrectness(), TestProperty5_ActiveOnlyInvariant(), TestPreservation_CascadeDeletesReplenishRowsWithFile(), TestPreservation_GetLastUpdated_NoRecordsReturnsNoRows(), TestPreservation_GetLastUpdated_ReturnsGlobalMostRecentRecord() (+12 more)

### Community 90 - "adminCtx"
Cohesion: 0.15
Nodes (23): NewMasterDataChangeService(), adminCtx(), ctxWithRole(), TestMasterDataChangeService_Submit_ServiceLayerRBAC(), TestMasterDataChangeService_Submit_ApprovalFailureReleasesPendingSlot(), TestMasterDataChangeService_Submit_Create(), TestMasterDataChangeService_Submit_Create_RejectsEntityID(), TestMasterDataChangeService_Submit_Create_SkipsPendingCheck() (+15 more)

### Community 91 - "Queries"
Cohesion: 0.12
Nodes (8): FindUserByEmailRow, FindUserByIDRow, FindUserByUsernameRow, GetUserProfileRow, LockAccountParams, SetInitialPasswordParams, SetPasswordParams, Queries

### Community 92 - "masterdata_import_confirm_test.go"
Cohesion: 0.14
Nodes (15): FindOpenMasterDataImportBatchRow, sanitizeImportFileName(), MasterDataImporter, newBatchRow(), newFinishRig(), TestImportFinish_AuditFailureSkipsNotifier(), TestImportFinish_NewBatchAuditsAndNotifiesOnce(), TestImportFinish_NilNotifierIsSafe() (+7 more)

### Community 93 - ".Apply"
Cohesion: 0.17
Nodes (14): TestDigitsOrFallback(), TestMapPriceAndPackageDBError(), TestVendorPackagePriceApplier_RejectsBadChanges(), pgx.Tx, mapPackageDBError(), digitsOrFallback(), pgx.Tx, mapPriceDBError() (+6 more)

### Community 94 - "NewApprovalPolicyStore"
Cohesion: 0.36
Nodes (5): NewApprovalPolicyStore(), numericParam(), TestApprovalPolicyStore_CreateGetUpdate(), TestApprovalPolicyStore_Get_NotFound(), ApprovalPolicyStore

### Community 95 - "admin_atm_assignment_handler_test.go"
Cohesion: 0.19
Nodes (19): ATMAssignmentAdminServicer, NewAdminATMAssignmentHandler(), assignmentTokenSvc(), mountAdminATMAssignmentHandler(), mountPackageOptions(), TestAdminATMAssignmentHandler_CoMountedWithATMCrudRouter(), TestAdminATMAssignmentHandler_Create_ErrorMapping(), TestAdminATMAssignmentHandler_Create_Returns202() (+11 more)

### Community 96 - "dsr_upload.go"
Cohesion: 0.20
Nodes (18): dailyRowErrorsToDto(), dailyRowsToDto(), dateOrNil(), DsrSheetDetail, DsrSheetResult, DsrUploadListResult, DsrUploadResult, int32OrZero() (+10 more)

### Community 97 - "masterdata_import_confirm.go"
Cohesion: 0.23
Nodes (21): assignmentFields(), atmFields(), ATMAssignmentUpdatePayload, importEnv, optInt64(), picFields(), rowRejection(), stageCreate() (+13 more)

### Community 98 - "VendorPicAdminService"
Cohesion: 0.18
Nodes (12): nilIfEmpty(), VendorPicUpdatePayload, NewVendorPicAdminService(), TestVendorPicAdminService_Create(), TestVendorPicAdminService_UpdateAndToggle_Scoping(), TestVendorPicAdminService_Validate(), TestVendorPicAdminService_Warnings(), trimOptional() (+4 more)

### Community 99 - ".Submit"
Cohesion: 0.21
Nodes (13): GetVaultPlanRow, ListVaultPlansForRequestRow, AllocationLineInput, checkAllocationLines(), checkReason(), VaultPlanService, insertAllocation(), optionalReason() (+5 more)

### Community 100 - "extractClientIP"
Cohesion: 0.17
Nodes (14): AdminRegionHandler, createRegionRequestBody, RegionAdminServicer, updateRegionRequestBody, getRegionRowToResponse(), chi.Router, listRegionRowToResponse(), NewAdminRegionHandler() (+6 more)

### Community 101 - "masterdata_approval_notify_test.go"
Cohesion: 0.22
Nodes (18): checkScenarioErr(), newNotifyRig(), notifyScenarios(), runApprove(), runForOutcome(), runRetry(), TestApprovalNotify_CallPerScenario(), TestApprovalNotify_CancelledContextStillNotifies() (+10 more)

### Community 102 - "writeUnauthorized"
Cohesion: 0.16
Nodes (10): MasterDataApplyRetrier, MasterDataApprovalBatchReader, MasterDataApprovalDetailReader, chi.Router, ApprovalHandler, parseApprovalRequestID(), toApprovalRequestResponse(), ApprovalHandler (+2 more)

### Community 103 - "notifyRig"
Cohesion: 0.13
Nodes (6): context.CancelFunc, failHeadApply(), pgx.Row, pgx.Tx, notifyRig, notifyTx

### Community 104 - "NewService"
Cohesion: 0.08
Nodes (36): Message, Querier, Recipients, Service, NewService(), normalizeEmail(), optional(), resolveUsers() (+28 more)

### Community 105 - "newImportFixture"
Cohesion: 0.23
Nodes (12): MasterDataImporter, testHelpers, newImportFixture(), TestMasterDataImportConfirm_InvalidFileStagesNothing_NoChangesIsAnError_Integration(), TestMasterDataImportConfirm_MiddleRowStale_ReturnsPromptlyAndRetiresWholeBatch_Integration(), TestMasterDataImportConfirm_OneBadRowRollsBackTheWholeBatch_Integration(), TestMasterDataImportConfirm_OneBatchOneApproval_IdempotentPerFile_AppliedAtomically_Integration(), TestMasterDataImportConfirm_RejectRejectsWholeBatch_AndFileMayBeReuploaded_Integration() (+4 more)

### Community 106 - ".actorAndID"
Cohesion: 0.29
Nodes (7): VaultPlanHandler, VaultPlanServicer, formatPgDate(), chi.Router, NewVaultPlanHandler(), parseDateParam(), vaultPlanJSON()

### Community 107 - ".toggle"
Cohesion: 0.19
Nodes (8): AdminVendorVaultHandler, createVendorVaultRequestBody, VendorVaultAdminServicer, chi.Router, NewAdminVendorVaultHandler(), vaultToResponse(), formatTimePtr(), toListATMsResponse()

### Community 108 - "serveNotification"
Cohesion: 0.23
Nodes (12): fakeNotificationStore, ListNotificationsForUserRow, notificationRequest(), serveNotification(), TestNotificationHandler_BadRequests(), TestNotificationHandler_ListParsesQuery(), TestNotificationHandler_ListUsesCallerAndDefaults(), TestNotificationHandler_MarkReadOwnOnly() (+4 more)

### Community 109 - "New"
Cohesion: 0.28
Nodes (13): New(), activeUserIDs(), countFor(), pgx.Tx, integrationTx(), TestIntegration_PurgeDeletesOnlyOldRows(), TestIntegration_RepositoryScopesToOwner(), TestIntegration_SendExpandsVendorUsersAndPICs() (+5 more)

### Community 110 - "027_acm_vault_assignment.sql"
Cohesion: 0.20
Nodes (17): public.acm_area_branches, public.acm_area_members, public.acm_areas, public.dsr_location_vault_maps, public.vendor_request_vault_assignments, public.vendor_request_vault_plans, public, public.set_updated_at (+9 more)

### Community 111 - "github.com/jackc/pgx/v5/pgtype.Date"
Cohesion: 0.15
Nodes (16): CountReplenishClassificationsParams, ListReplenishClassificationsParams, ListReplenishClassificationsRow, ListVendorOrdersRow, ReplenishClassificationAmount, ReportReplenishTripsParams, ReportReplenishTripsRow, SummarizeReplenishClassificationsParams (+8 more)

### Community 112 - "AdminMasterDataImportHandler"
Cohesion: 0.12
Nodes (16): mime/multipart.File, mime/multipart.FileHeader, importRateLimiter, MasterDataImportHistoryServicer, MasterDataImportServicer, chi.Router, AdminMasterDataImportHandler, NewAdminMasterDataImportHandler() (+8 more)

### Community 113 - "mountRbacListHandler"
Cohesion: 0.24
Nodes (16): mountRbacListHandler(), numeric(), TestRbacListHandler_APPACCESS_Allowed(), TestRbacListHandler_CreatePolicy_BadBody(), TestRbacListHandler_CreatePolicy_HappyPath(), TestRbacListHandler_CreatePolicy_Validation(), TestRbacListHandler_ListDelegations_HappyPath(), TestRbacListHandler_ListEndpoint_StoreError() (+8 more)

### Community 114 - "newHistoryFixture"
Cohesion: 0.30
Nodes (12): ImportBatch, pgx.Tx, T, historyPtr(), newHistoryFixture(), TestImportHistory_DecidedFromLatestActedStep(), TestImportHistory_FileNameCheckConstraint(), TestImportHistory_FiltersAndPaging() (+4 more)

### Community 115 - "importEnv"
Cohesion: 0.28
Nodes (7): canonicalPicRolesCell(), TestParsePicRoles(), boolCol(), importEnv, parsePicRoles(), requiredCols(), validateVendor()

### Community 116 - "regionHarness"
Cohesion: 0.27
Nodes (16): longString(), queryRowScanRegion(), regionHarness(), TestCreateRegion_AuditFailureRollsBack(), TestCreateRegion_AuthorizationMatrix(), TestCreateRegion_CodeConflictRejectsAndCreatesNoRow(), TestCreateRegion_NormalizesCodeAndWritesAudit(), TestCreateRegion_ValidationRejectsBadCodeAndName() (+8 more)

### Community 117 - "031_replenish_realization.sql"
Cohesion: 0.15
Nodes (16): public.holidays, public.replenish_classification_amounts, public.replenish_classifications, public.vendor_request_realization_amounts, public.vendor_request_realizations, public, public.set_updated_at, public.users (+8 more)

### Community 119 - "DsrService"
Cohesion: 0.12
Nodes (17): encoding/json.RawMessage, net/http.Client, redis.Client, DsrService, NewDsrService(), TestDsrService_CheckUploadRate(), TestDsrService_ListUploads(), TestDsrService_ResolveVendor() (+9 more)

### Community 120 - "mountAdminVendorBranchHandler"
Cohesion: 0.24
Nodes (13): TestAdminMasterDataUpdate_Returns202And400(), TestAdminVendorBranchHandler_UpdateAndEnableErrors(), mountAdminVendorBranchHandler(), TestAdminVendorBranchHandler_Create_HappyPath_Returns202WithChangeRequest(), TestAdminVendorBranchHandler_Create_ServiceConflict_MapsTo409(), TestAdminVendorBranchHandler_Create_ValidationError_MapsTo400(), TestAdminVendorBranchHandler_Disable_HappyPath_Returns202(), TestAdminVendorBranchHandler_Disable_PendingConflict_MapsTo409() (+5 more)

### Community 121 - "Worker"
Cohesion: 0.06
Nodes (35): time.Duration, noopBlacklist, vendorRequestNoopBlacklist, SMTPMailer, orDefault(), NewWorker(), newTestWorker(), TestNewWorker_Defaults() (+27 more)

### Community 122 - "MasterDataImportBatchRepository"
Cohesion: 0.14
Nodes (9): ListMasterDataImportBatchesRow, findOpenBatch(), ImportBatch, ImportBatchFilter, NewMasterDataImportBatchRepository(), BatchRowError, ImportBatchPool, MasterDataImportBatchRepository (+1 more)

### Community 123 - "atm_portal_profile.go"
Cohesion: 0.17
Nodes (16): math/big.Rat, computeReplenishmentStatus(), AtmPortalService, ListATMReplenishParams, ListATMReplenishResult, latestRefundTotal(), numericToDecimalStringPtr(), numericToRat() (+8 more)

### Community 124 - "BuildChain"
Cohesion: 0.22
Nodes (8): ApproverInfo, ChainRepository, errNotFound, fakeChainRepository, BuildChain(), int32p(), int64p(), TestBuildChain()

### Community 125 - "Orchestrator"
Cohesion: 0.24
Nodes (6): AuditWriter, AvailabilityRepository, Orchestrator, RequestStore, ResolveEffectiveApprover(), AuditWriter

### Community 126 - "Entry"
Cohesion: 0.17
Nodes (8): countingFailAudit, fakeAuditWriter, fakeAuditWriter, fakeAdminAuditWriter, Entry, marshalOrNil(), fakeMasterDataAuditWriter, finishAudit

### Community 127 - "NewDeactivateUserService"
Cohesion: 0.23
Nodes (10): DeactivateUserService, AuditWriter, NewDeactivateUserService(), TestDeactivateUserService_Deactivate_AuditWriteFails_PropagatesError(), TestDeactivateUserService_Deactivate_TargetNotFound(), TestDeactivateUserService_Deactivate_UserWithAuditHistory_IsSoftDeleted(), TestDeactivateUserService_Deactivate_UserWithoutAuditHistory_StillSoftDeleted(), TestDeactivateUserService_Deactivate_WritesAuditWithoutSecrets() (+2 more)

### Community 128 - "Holiday"
Cohesion: 0.21
Nodes (7): CountHolidaysAdminParams, CreateHolidayParams, ListHolidaysAdminParams, SetHolidayActiveParams, UpdateHolidayNameParams, Queries, Holiday

### Community 129 - "DsrLocationMapAdminService"
Cohesion: 0.19
Nodes (8): DsrLocationMapPayload, DsrLocationMapUpdatePayload, NewDsrLocationMapAdminService(), TestDsrLocationMapAdminService_Create(), TestDsrLocationMapAdminService_OtherVendorIDIsNotFound(), TestDsrLocationMapAdminService_UpdateDisabledAndEnableConflict(), DsrLocationMapAdminRepo, DsrLocationMapAdminService

### Community 130 - "service_property_test.go"
Cohesion: 0.35
Nodes (13): pgregory.net/rapid.Generator, genNonWhitespaceString(), genValidPortalType(), genWhitespaceString(), int64Ptr(), nonWSRune(), printableRune(), TestProperty_Service_AuthErrorUniformity() (+5 more)

### Community 131 - "dsr_upload_handler.go"
Cohesion: 0.26
Nodes (13): confirmRequest, dsrDailyRowResponse, dsrRencanaIsiRowResponse, dsrRowErrorResponse, dsrSheetDetailResponse, dsrSheetResponse, dsrSheetSummaryResponse, dsrUploadListItemResponse (+5 more)

### Community 132 - "RegionAdminService"
Cohesion: 0.20
Nodes (13): CreateRegionRequest, UpdateRegionNameRequest, isRegionAdminActor(), NewRegionAdminService(), normalizeRegionCode(), TestCreateRegion_InvalidInputRejectedWithoutMutation(), TestRegionCode_NormalizedAndAcceptedIffAlnum(), TestRegionMutations_NonAdminRefusedAtService() (+5 more)

### Community 133 - "NewLocalProvider"
Cohesion: 0.26
Nodes (9): LocalProvider, NewLocalProvider(), hashPassword(), TestLocalProvider_Authenticate_NilPasswordHash(), TestLocalProvider_Authenticate_RepoError(), TestLocalProvider_Authenticate_UserNotFound(), TestLocalProvider_Authenticate_ValidPassword(), TestLocalProvider_Authenticate_WrongPassword() (+1 more)

### Community 134 - "stubAtmPortalService"
Cohesion: 0.20
Nodes (11): stubAtmPortalService, AtmPortalService, ListCashposParams, ListCashposResult, numericToDecimalString(), rowToCashposRow(), ATMProfileResult, ListATMCashposParams (+3 more)

### Community 135 - "mountAdminVendorPicHandler"
Cohesion: 0.29
Nodes (12): mountAdminVendorPicHandler(), TestAdminVendorPicHandler_Create_ErrorMapping(), TestAdminVendorPicHandler_Create_Returns202(), TestAdminVendorPicHandler_Get_IncludesRoles(), TestAdminVendorPicHandler_Get_NotFound(), TestAdminVendorPicHandler_List_IncludesWarnings(), TestAdminVendorPicHandler_SetRoles_ErrorMapping(), TestAdminVendorPicHandler_SetRoles_NonAdminForbidden() (+4 more)

### Community 136 - "numericFromDecimalPtr"
Cohesion: 0.22
Nodes (7): atmMoney(), pgx.Tx, pgx.Tx, numericFromDecimalPtr(), vaultNumerics(), ATMApplier, VendorVaultApplier

### Community 137 - "DsrLateReportRepository"
Cohesion: 0.25
Nodes (15): DsrLateReportRepository, assertDsrLateSummary(), assertDsrLateWarnings(), pgx.Tx, loadDsrLateCases(), mustDate(), mustTime(), seedDsrLateCases() (+7 more)

### Community 138 - "atm_portal_filter_property_test.go"
Cohesion: 0.26
Nodes (11): containsFold(), pgx.Tx, insertTestATMWithLifecycle(), nullableDecimalOrderOK(), nullableTimeOrderOK(), sortedPairOK(), stringOrderOK(), strOrNil() (+3 more)

### Community 139 - "reasonProbePool"
Cohesion: 0.18
Nodes (7): pgx.Row, pgx.Rows, pgx.Tx, pgx.TxOptions, reasonGen(), TestProperty5_ReasonValidation(), reasonProbePool

### Community 140 - "public.vendor_request_vault_allocations"
Cohesion: 0.18
Nodes (11): public.acm_area_regions, public.vendor_request_vault_allocation_amounts, public.vendor_request_vault_allocations, public, public.set_updated_at, public.users, public.vendor_requests, public.vendor_vaults (+3 more)

### Community 141 - "MasterDataApprovalService"
Cohesion: 0.22
Nodes (10): pgx.Tx, NewMasterDataApprovalService(), statesEqual(), TestStatesEqual(), ApplierRegistry, MasterDataApprovalAuditWriter, MasterDataApprovalOrchestrator, MasterDataApprovalPool (+2 more)

### Community 142 - "dbtxProbe"
Cohesion: 0.20
Nodes (5): pgx.Row, pgx.Rows, TestRepository_WriteReadTopology(), dbtxProbe, probeRow

### Community 143 - "mountAdminMasterDataChangeHandler"
Cohesion: 0.35
Nodes (10): mountAdminMasterDataChangeHandler(), TestAdminMasterDataChangeHandler_Get_HappyPath_IncludesPayloadAndBefore(), TestAdminMasterDataChangeHandler_Get_InternalError(), TestAdminMasterDataChangeHandler_Get_InvalidID_BadRequest(), TestAdminMasterDataChangeHandler_Get_NotFound(), TestAdminMasterDataChangeHandler_List_EntityTypeAndStatusFilter_PassedThrough(), TestAdminMasterDataChangeHandler_List_HappyPath(), TestAdminMasterDataChangeHandler_List_InvalidStatus_BadRequest() (+2 more)

### Community 144 - "NewRbacReadRepository"
Cohesion: 0.53
Nodes (5): NewRbacReadRepository(), TestRbacReadRepository_ListApprovalPolicies_ExactMoney(), TestRbacReadRepository_ListDelegations_OnlyActive(), TestRbacReadRepository_ListLeaves(), TestRbacReadRepository_ListUserHierarchy()

### Community 145 - "ListVendorRealizationsForRequestRow"
Cohesion: 0.24
Nodes (5): InsertVendorRealizationAmountParams, ListRequestTicketOrdersRow, UpsertVendorRealizationParams, ListVendorRealizationsForRequestRow, Queries

### Community 146 - "ListVaultTicketsForRequestRow"
Cohesion: 0.27
Nodes (5): IssueVaultTicketParams, LockVaultTicketSeqParams, SetVaultTicketActiveParams, ListVaultTicketsForRequestRow, Queries

### Community 147 - "mountRoleMgmtHandler"
Cohesion: 0.38
Nodes (9): mountRoleMgmtHandler(), TestRoleMgmtHandler_CreateRole_ErrorMapping(), TestRoleMgmtHandler_CreateRole_HappyPath(), TestRoleMgmtHandler_ListCatalog_HappyPath(), TestRoleMgmtHandler_ListRoles_GroupsPermissionsByRole(), TestRoleMgmtHandler_NoToken_Unauthorized(), TestRoleMgmtHandler_RBAC(), TestRoleMgmtHandler_UpdateRolePermissions_ErrorMapping() (+1 more)

### Community 148 - "admin_repository_topology_test.go"
Cohesion: 0.20
Nodes (5): pgx.Row, pgx.Rows, TestAuthRepository_PropagatesDBErrors(), poolProbe, poolProbeRow

### Community 149 - "github.com/jackc/pgx/v5/pgconn.CommandTag"
Cohesion: 0.25
Nodes (4): github.com/jackc/pgx/v5/pgconn.CommandTag, pgx.Row, pgx.Rows, dbtxProbe

### Community 150 - "atm_portal.go"
Cohesion: 0.20
Nodes (13): pgregory.net/rapid.T, dateToTimePtr(), AtmPortalService, ListATMsParams, ListATMsResult, pgTimeToStringPtr(), rowToAtmWithCashPos(), drawFilterParams() (+5 more)

### Community 151 - "003_master_data.sql"
Cohesion: 0.22
Nodes (9): public.master_data_change_requests, public.vendor_pics, public.set_updated_at, public.users, public.vendor_branches, public.vendors, trg_master_data_change_requests_set_updated_at, trg_vendor_pics_set_updated_at (+1 more)

### Community 152 - "AdminVendorPicHandler"
Cohesion: 0.23
Nodes (6): AdminVendorPicHandler, VendorPicAdminServicer, VendorPicRoleSetter, chi.Router, NewAdminVendorPicHandler(), picToResponse()

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

### Community 160 - ".Apply"
Cohesion: 0.23
Nodes (8): pgx.Tx, TestNormalizePicRoles(), AllPicRoles(), NewVendorPicRoleService(), normalizePicRoles(), VendorPicApplier, VendorPicRolePool, VendorPicRoleService

### Community 161 - "public.export_jobs"
Cohesion: 0.29
Nodes (7): public.export_job_chunks, public.export_jobs, public.set_updated_at, public.users, public.vendor_branches, public.vendors, trg_export_jobs_set_updated_at

### Community 162 - "mountAdminVendorPackagePriceHandler"
Cohesion: 0.60
Nodes (5): mountAdminVendorPackagePriceHandler(), TestAdminVendorPackagePriceHandler_ErrorMapping(), TestAdminVendorPackagePriceHandler_ListAndGet(), TestAdminVendorPackagePriceHandler_MutationsReturn202(), TestAdminVendorPackagePriceHandler_RBAC()

### Community 163 - "VendorPackagePrice"
Cohesion: 0.22
Nodes (7): fakeVendorPackagePriceServicer, ListVendorPackagePricesAdminParams, ListVendorPackagePricesAdminRow, fromGetRow(), fromListRow(), VendorPackagePrice, newVendorPackagePrice()

### Community 164 - "MasterDataChangeRepository"
Cohesion: 0.15
Nodes (5): NewMasterDataChangeRepository(), pgx.Tx, integrationTx(), TestMasterDataImportBatchRepository(), MasterDataChangeRepository

### Community 165 - ".Apply"
Cohesion: 0.47
Nodes (3): ensureBranchHasNoActiveChildren(), pgx.Tx, VendorBranchApplier

### Community 166 - "RoleMgmtHandler"
Cohesion: 0.19
Nodes (9): createRoleRequestBody, RoleMgmtHandler, RoleMgmtServicer, updateRolePermissionsRequestBody, catalogEntryToResponse(), chi.Router, NewRoleMgmtHandler(), rolesWithPermissionsToResponse() (+1 more)

### Community 167 - "atm_portal_integration_test.go"
Cohesion: 0.33
Nodes (14): testATMSpec, decodeListATMsResponse(), doGetATMs(), findRow(), chi.Router, pgx.Tx, insertATM(), insertLocation() (+6 more)

### Community 168 - "AuditLog"
Cohesion: 0.19
Nodes (7): CreateAuditLogParams, ListAuditLogsByEntityParams, Queries, AuditLog, TestAuditLog(), TestListVendorOptions_MapsVendorsAndSkipsNullRegions(), optionsAuditRepo

### Community 169 - "Region"
Cohesion: 0.20
Nodes (4): UpdateRegionNameParams, Region, CreateRegionAdminParams, Queries

### Community 170 - "VendorPackage"
Cohesion: 0.21
Nodes (8): github.com/cimb-niaga/cms/backend/internal/service.VendorPackageContentPayload, fakeVendorPackageAdminServicer, fromGetPackageRow(), fromListPackageRow(), VendorPackage, newVendorPackageFromRow(), tierMax64(), priceDateToStringPtr()

### Community 171 - "public.notifications"
Cohesion: 0.50
Nodes (4): public.notification_emails, public.notifications, public, public.users

### Community 172 - "public.dsr_receipts"
Cohesion: 0.40
Nodes (4): public.dsr_receipts, public, public.users, public.vendors

### Community 174 - "TestMasterDataImport_PicRoles_Integration"
Cohesion: 0.15
Nodes (13): NewMasterDataExportRepository(), R, idsOf(), TestMasterDataExportRepository_KeysetPagingIsCompleteAndExact(), testHelpers, scopedRows(), seedScopedFixture(), TestExportScopedRoundTrip_DryRunIsZeroChanges() (+5 more)

### Community 175 - "VaultPlanDetail"
Cohesion: 0.29
Nodes (3): fakeVaultPlanServicer, VaultPlanDetail, VaultPlanService

### Community 176 - "public.master_data_import_batches"
Cohesion: 0.50
Nodes (3): public.master_data_import_batches, public.users, public.master_data_change_requests

### Community 177 - "public.branch_coverage_areas"
Cohesion: 0.50
Nodes (3): public.branch_coverage_areas, public, public.vendor_branches

### Community 266 - "NewAuditLogReadService"
Cohesion: 0.29
Nodes (11): NewAuditLogReadService(), sampleAuditLogRow(), TestAuditLogReadService_GetByID_BeforeAfterPreservedAsJSON(), TestAuditLogReadService_GetByID_NotFound(), TestAuditLogReadService_List_ClampsAndAssembles(), TestAuditLogReadService_List_DateRangeValidation(), TestAuditLogReadService_List_EqualDatesAllowed(), TestAuditLogReadService_List_RepoError() (+3 more)

### Community 267 - "TestVendorPicRoleService_SetRoles_Integration"
Cohesion: 0.28
Nodes (10): approveChange(), testHelpers, TestDsrLocationMapApplier_Lifecycle_Integration(), createPicViaApproval(), testHelpers, picRoles(), picRolesAuditCount(), TestVendorPicApplier_CreateRoles_Integration() (+2 more)

### Community 268 - "ListForecastForDateRow"
Cohesion: 0.24
Nodes (9): ListForecastForDateParams, ListForecastForDateRow, distinct(), pgx.Tx, listForecastParams(), seedBranchPackage(), seedRequest(), terminalIDs() (+1 more)

### Community 269 - "audit_log_handler.go"
Cohesion: 0.36
Nodes (10): auditLogDetailResponse, auditLogListResponse, auditLogListRow, optionalStringParam(), parseAuditLogDate(), parseListAuditLogsParams(), parseOptionalInt64Param(), toAuditLogDetailResponse() (+2 more)

### Community 270 - "GetVendorPackagePriceAdminByIDRow"
Cohesion: 0.24
Nodes (6): GetVendorPackagePriceAdminByIDRow, TestVendorPackagePriceAdminService_Create(), TestVendorPackagePriceAdminService_UpdateAndDisable(), validPriceCreate(), fakePriceAdminRepo, futurePriceRepo

### Community 271 - "AuditLogFilter"
Cohesion: 0.27
Nodes (5): AuditLogFilter, NewAuditLogRepository(), TestAuditLogRepository(), timestamptzFromPtr(), AuditLogRepository

### Community 272 - "RegionAdminRepository"
Cohesion: 0.24
Nodes (4): pgx.Tx, NewRegionAdminRepository(), TestRegionAdminRepository(), RegionAdminRepository

### Community 273 - ".Apply"
Cohesion: 0.20
Nodes (7): pgx.Tx, pgx.Tx, Applier, unmarshalableAfterApplier, VendorApplier, vendorCreatePayload, vendorUpdatePayload

### Community 274 - ".ChangePassword"
Cohesion: 0.28
Nodes (5): fakeChangePasswordService, fakeSetInitialPasswordService, ChangePasswordRequest, isLocalAuthSource(), SetInitialPasswordRequest

### Community 275 - "syncVaultTickets"
Cohesion: 0.39
Nodes (7): TestPlanVaultTickets(), syncVaultTickets(), applyVaultTickets(), TestIntegration_ApplyVaultTickets(), planVaultTickets(), vaultTicketAudit, vaultTicketPlan

### Community 276 - ".ListAuditLogs"
Cohesion: 0.33
Nodes (3): CountAuditLogsParams, ListAuditLogsParams, Queries

### Community 277 - "auth_handler_refresh_test.go"
Cohesion: 0.57
Nodes (6): net/http.Cookie, doRefreshDirect(), newRefreshHandler(), refreshCookie(), TestRefresh_InheritsDeadlineAndCookieMaxAge(), TestRefresh_SessionExpired_401AndClearsCookie()

### Community 278 - "NewRepository"
Cohesion: 0.53
Nodes (5): TestRepository_CreateDelegation_NonOverlappingSucceeds(), TestRepository_CreateDelegation_OverlapRejected(), TestRepository_ListInboxForApprover(), TestRepository_RevokeDelegation(), NewRepository()

### Community 280 - "TestIntegration_VendorPackagePriceApplier"
Cohesion: 0.47
Nodes (5): applyPriceChange(), pgx.Tx, pricePayload(), TestIntegration_VendorPackagePriceApplier(), VendorPackagePricePayload

### Community 281 - "vendorSide"
Cohesion: 0.70
Nodes (3): VendorRequestPartyReader, VendorRequestReturner, vendorSide

### Community 282 - "NewWriter"
Cohesion: 0.67
Nodes (3): NewWriter(), TestWriter_Write(), TestWriter_Write_NilBeforeAfter()

## Knowledge Gaps
- **52 isolated node(s):** `github.com/cimb-niaga/cms/backend`, `AdminStore`, `AuditWriter`, `AtmDenom`, `ExportJobChunk` (+47 more)
  These have ≤1 connection - possible missing edges or undocumented components.
- **29 thin communities (<3 nodes) omitted from report** — run `graphify query` to explore isolated nodes.

## Suggested Questions
_Questions this graph is uniquely positioned to answer:_

- **Why does `main()` connect `main` to `DsrLocationMapAdminService`, `Queries`, `RegionAdminService`, `NewLocalProvider`, `mountAuditLogHandler`, `handler/integration_test.go`, `fakeUserAdminRepo`, `NewAuditLogReadService`, `NewATMAssignmentAdminService`, `writeJSON`, `net/http.ResponseWriter`, `AuditLogFilter`, `NewRbacReadRepository`, `RegionAdminRepository`, `atm_assignments_admin.sql.go`, `MasterDataApprovalService`, `masterdata_import_test.go`, `ReplenishmentFilter`, `NewRepository`, `tokenFor`, `AdminVendorPicHandler`, `NewRepository`, `NewWriter`, `net/http.Handler`, `DsrLateFilter`, `net/http.Request`, `.Apply`, `branch_atm_test.go`, `NewDmaaForecastService`, `NewUserAdminService`, `MasterDataChangeRepository`, `writeUnexpectedError`, `RoleMgmtHandler`, `jakartaCalendarDate`, `mountVendorRequestHandler`, `TestMasterDataImport_PicRoles_Integration`, `RbacListHandler`, `harness`, `vendor_request_response.go`, `vendor_package_admin.go`, `vendor_package_price_admin.go`, `formatTimestamptz`, `NewChangePasswordService`, `writeValidationError`, `NewVendorBranchAdminService`, `holiday_admin.go`, `VendorVaultAdminService`, `.withFilter`, `testing.T`, `atm_admin_test.go`, `AdminATMHandler`, `NewVendorAdminService`, `.Get`, `NewAtmPortalService`, `vendor_request.go`, `github.com/cimb-niaga/cms/pkg/auth.TokenService`, `NewOrchestrator`, `adminCtx`, `NewApprovalPolicyStore`, `admin_atm_assignment_handler_test.go`, `VendorPicAdminService`, `extractClientIP`, `Queries`, `NewService`, `.actorAndID`, `.toggle`, `New`, `AdminMasterDataImportHandler`, `DsrService`, `Worker`, `MasterDataImportBatchRepository`, `NewDeactivateUserService`?**
  _High betweenness centrality (0.029) - this node is a cross-community bridge._
- **Why does `assertDsrLateSummary()` connect `DsrLateReportRepository` to `context.Context`, `testing.T`, `dsr_late_report.sql.go`, `github.com/jackc/pgx/v5/pgtype.Date`, `github.com/jackc/pgx/v5/pgtype.Timestamptz`?**
  _High betweenness centrality (0.024) - this node is a cross-community bridge._
- **Why does `assertDsrLateWarnings()` connect `DsrLateReportRepository` to `context.Context`, `github.com/jackc/pgx/v5/pgtype.Timestamptz`, `testing.T`, `github.com/jackc/pgx/v5/pgtype.Date`?**
  _High betweenness centrality (0.023) - this node is a cross-community bridge._
- **Are the 274 inferred relationships involving `doRequest()` (e.g. with `TestAdminAcmAreaHandler()` and `TestAdminApprovalHandler_AdminRole_SetHierarchy_HappyPath()`) actually correct?**
  _`doRequest()` has 274 INFERRED edges - model-reasoned connections that need verification._
- **Are the 224 inferred relationships involving `tokenForRole()` (e.g. with `TestAdminAcmAreaHandler()` and `TestAdminATMAssignmentHandler_CoMountedWithATMCrudRouter()`) actually correct?**
  _`tokenForRole()` has 224 INFERRED edges - model-reasoned connections that need verification._
- **What connects `github.com/cimb-niaga/cms/backend`, `AdminStore`, `AuditWriter` to the rest of the system?**
  _52 weakly-connected nodes found - possible documentation gaps or missing edges._
- **Should `context.Context` be split into smaller, more focused modules?**
  _Cohesion score 0.0318385042677144 - nodes in this community are weakly interconnected._