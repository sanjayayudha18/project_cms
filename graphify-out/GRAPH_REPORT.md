# Graph Report - CMS2  (2026-08-27)

## Corpus Check
- Large corpus: 519 files · ~412,563 words. Semantic extraction will be expensive (many Claude tokens). Consider running on a subfolder.

## Summary
- 2624 nodes · 4869 edges · 208 communities (169 shown, 39 thin omitted)
- Extraction: 96% EXTRACTED · 4% INFERRED · 0% AMBIGUOUS · INFERRED: 219 edges (avg confidence: 0.85)
- Token cost: 0 input · 0 output

## Community Hubs (Navigation)
- Eod Monitoring Hooks
- Response Atm Portal
- Auth Token Property
- Dashboard Property Replenishment
- Api Stubs Auth
- Cash Flow Atmleveltable
- Forecasting Dsr Dsruploadform
- Atm Portal Sql
- Dmaa Forecast Usedmaaforecasturlstate
- Auth Property Middleware
- Atm Portal Useatmportalurlstate
- Biome Formatter Rules
- Cit Filter Property
- Tsconfig Compileroptions Ref
- Atm Portal Profile
- Atm Portal Filterbar
- Itm Replenish Scheduler
- Atm Portal Property
- Feedback Errorboundary Skeleton
- Card Loginpage Auth
- Formatters Date Derivestatus
- Tsconfig Compileroptions Json
- Tsconfig Compileroptions Ref
- Dmaa Forecast Testlistdmaaforecast
- Config Atm Portal
- Dsr Dsrtable Dsrdashboard
- Middleware Rbac Property
- Auth Authcontext Routeguard
- Itm Cashpos Scheduler
- Scheduler Retry Routers
- Forecast Schedulelist Forecasttable
- Routes Atm Portal
- Opencode Plugins Changed
- Replenishment Filter Property
- Scheduler Retry Services
- Package Dependencies React
- Reconciliation Reconciliationscreen Filter
- Integration Testintegration Miniredis
- Atm Portal Cashpos
- Package Devdependencies Json
- Atm Portal Formatters
- Github Com Jackc
- Scheduler Retry Services
- Invoice Invoiceflow Invoicedetail
- Node Tsconfig Compileroptions
- Node Tsconfig Compileroptions
- Atm Portal Time
- Atm Portal Useatmprofiledata
- Atm Portal Useatmprofileurlstate
- Dmaa Scheduler Bak
- Package React Dependencies
- Package Devdependencies Json
- Atm Portal Replenishtable
- Config Navigation Property
- Scheduler Retry Services
- Scheduler Retry Services
- Dmaa Bak Scheduler
- Scheduler Retry Services
- Auth Local Provider
- Auth Mockuserrepository Stubuserrepo
- Scheduler Retry Schemas
- Atm Portal Cashposprofiletable
- Routes Auth Login
- Routes Protected Forecasting
- Styles Contrast Property
- Authstate Authuser Balancestatus
- Atm Portal Integration
- Atm Portal Atmcashpostable
- Scheduler Retry Config
- Layout Header Appshell
- Middleware Rate Limiter
- Dmaa Scheduler Etl
- Schedule Grouping Property
- Api Client Apiclient
- Middleware Ratelimiter Rate
- Scheduler Retry Services
- Appshell Errorboundary Handlekeydown
- Scheduler Retry Database
- Schedulepage Schedule Dategroup
- Datafilters Property Alldates
- Atm Portal Arialiveregion
- Usetoast Hooks Toast
- Routes Root Auth
- Strings Language Property
- Oxlintrc Rules Ref
- Notifications Notificationspage Usenotifications
- Validation Property Accepted
- Dmaa Forecast Testlistdmaaforecast
- Package Scripts Build
- Sidebar Layout Group
- Integration Conditionalthrower Mockfetch
- Queries Auth Sql
- Import Alias Compliance
- Hardcoded Colors Property
- Package Scripts Build
- Button Buttonprops Buttonsize
- Orderspage Orders Columnhelper
- Auth Ratelimiterror Error
- Package Name Packagemanager
- Layout Accessibility Property
- Toast Config Toastcontainer
- Invoicespage Invoices Invoicerow
- Notifications Routing Property
- Constants Threshold Critical
- Sorting Property Recordarb
- Opencode Ref Instructions
- Queries Dbtx Pgx
- Rename Itm Cashpos
- Retry Migrations Scheduler
- Badge Badgeprops Badgevariant
- Noticebanner Noticebannerprops Noticebannervariant
- Package Name Private
- Badge Badgeprops Defaulticons
- Datatable Columnmeta Datatableprops
- Dsrpage Dsr Balancestatusbadgemap
- Evidenceform Evidence Evidenceformdata
- Evidencepage Evidence Existingevidence
- Modulecard Modulecardprops Property
- Progressbar Fillcolorbystatus Progressbarprops
- Summarycard Formatvalue Summarycardprops
- Data Json Integrity
- Cash Count Routes
- Forecasting Routes Cards
- Invoice Routes Cards
- Filtertabs Filtertaboption Filtertabsprops
- Datatable Testcolumns Testdata
- Invoicedetail Invoices Invoicedetailprops
- Ordersummarybar Orders Ordersummarybarprops
- Scheduler Audit Retry
- Scheduler Late Retry
- Scheduler Summary Retry
- Atm Portal Summary
- Itm Cashpos Migrations
- Emptystate Emptystateprops
- Tsconfig Files References
- Notificationbadge Layout Notificationbadgeprops
- Datepicker Datepickerprops
- Emptystate Emptystateprops
- Fileupload Createfile Defaultprops
- Tsconfig Files References
- Github Com Cimb
- Package Lucide React
- Package React Dom
- Package Fast Check
- Package Jsdom Devdependencies
- Playwright Package Devdependencies
- Package Devdependencies Json
- Package Vitejs Plugin
- Package Jsdom Devdependencies
- Package Tailwindcss Devdependencies
- Package React Devdependencies
- Package Typescript Devdependencies
- Package Vitejs Plugin
- Package Vitest Devdependencies
- Routes Router
- Queryclient
- Itm Cashpos
- Itm Cashpos Files
- Itm Replenish
- Itm Replenish Files

## God Nodes (most connected - your core abstractions)
1. `react` - 55 edges
2. `NewAtmPortalService()` - 24 edges
3. `compilerOptions` - 23 edges
4. `Settings` - 22 edges
5. `newServiceUnderTest()` - 21 edges
6. `compilerOptions` - 20 edges
7. `compilerOptions` - 20 edges
8. `setupHarness()` - 19 edges
9. `TokenService` - 19 edges
10. `NewTokenService()` - 19 edges

## Surprising Connections (you probably didn't know these)
- `main()` --calls--> `Load()`  [EXTRACTED]
  backend-cit/cmd/api/main.go → pkg/config/config.go
- `main()` --calls--> `RequireAuth()`  [EXTRACTED]
  backend-cit/cmd/api/main.go → pkg/middleware/rbac.go
- `main()` --calls--> `NewRedisTokenBlacklist()`  [EXTRACTED]
  backend/cmd/api/main.go → pkg/auth/token_blacklist.go
- `main()` --calls--> `NewTokenService()`  [EXTRACTED]
  backend/cmd/api/main.go → pkg/auth/token_service.go
- `main()` --calls--> `Load()`  [EXTRACTED]
  backend/cmd/api/main.go → pkg/config/config.go

## Import Cycles
- None detected.

## Communities (208 total, 39 thin omitted)

### Community 0 - "Eod Monitoring Hooks"
Cohesion: 0.06
Nodes (68): AuditLogSection(), AuditLogSectionProps, FILE_TYPE_OPTIONS, OUTCOME_LABELS, TRIGGER_OPTIONS, EodMonitoringPage(), FileStatusSection(), FileStatusSectionProps (+60 more)

### Community 1 - "Response Atm Portal"
Cohesion: 0.06
Nodes (62): formatDatePtr(), formatTimePtr(), chi.Router, NewAtmPortalHandler(), parseIntParam(), parseListATMCashposParams(), parseListATMReplenishParams(), parseListATMsParams() (+54 more)

### Community 2 - "Auth Token Property"
Cohesion: 0.06
Nodes (50): AccessTokenClaims, LoginRequest, LoginResponse, mockBlacklist, mockTokenBlacklist, RedisTokenBlacklist, RefreshTokenClaims, TokenBlacklist (+42 more)

### Community 3 - "Dashboard Property Replenishment"
Cohesion: 0.05
Nodes (45): AttentionPanel(), categoryStyles, iconMap, ActivityFeed(), ActivityFeedProps, formatRelativeTime(), TYPE_ICONS, ACCENT_STYLES (+37 more)

### Community 4 - "Api Stubs Auth"
Cohesion: 0.05
Nodes (46): api, apiClient(), ApiError, ApiResponse, createApiError(), executeRequest(), handleUnauthorized(), injectAuthHeader() (+38 more)

### Community 5 - "Cash Flow Atmleveltable"
Cohesion: 0.06
Nodes (42): AtmLevelRow(), AtmLevelTable(), AtmLevelTableProps, getCashLevelTier(), tierStyles, CASH_FLOW_SKELETON_CARD_IDS, CashFlowScreen(), CASH_FLOW_QUERY_KEY (+34 more)

### Community 6 - "Forecasting Dsr Dsruploadform"
Cohesion: 0.08
Nodes (36): DSRPreviewTable(), DSRPreviewTableProps, DSRUploadForm(), handleCancel(), handleConfirm(), handleFileSelect(), handleReset(), resetFileInput() (+28 more)

### Community 7 - "Atm Portal Sql"
Cohesion: 0.08
Nodes (19): stubRateLimiter, CountATMsWithCashPosParams, CountCashposByTerminalParams, CountItmCashposParams, CountReplenishByTerminalParams, GetATMByTerminalIDRow, GetATMSummaryRow, ListATMsWithCashPosParams (+11 more)

### Community 8 - "Dmaa Forecast Usedmaaforecasturlstate"
Cohesion: 0.08
Nodes (34): RFC-3339, DmaaForecastFilters(), DmaaForecastFiltersProps, cellValue(), Column, COLUMNS, DmaaForecastTable(), DmaaForecastTableProps (+26 more)

### Community 9 - "Auth Property Middleware"
Cohesion: 0.10
Nodes (37): noopBlacklist, stubProvider, printableASCIIRune(), TestProperty_BcryptRoundTrip(), genNonWhitespaceString(), genValidPortalType(), genWhitespaceString(), int64Ptr() (+29 more)

### Community 10 - "Atm Portal Useatmportalurlstate"
Cohesion: 0.11
Nodes (34): AtmPortalScreen(), handleClearAll(), handleSortChange(), ATM_CASHPOS_QUERY_KEY, ATM_PORTAL_MODE_CASHPOS, ATM_PORTAL_MODE_REPLENISH, ATM_PORTAL_QUERY_KEY, ATM_PORTAL_STALE_TIME (+26 more)

### Community 11 - "Biome Formatter Rules"
Cohesion: 0.05
Nodes (37): useKeyWithClickEvents, useSemanticElements, noUselessFragments, noUnusedImports, noUnusedVariables, useExhaustiveDependencies, files, ignore (+29 more)

### Community 12 - "Cit Filter Property"
Cohesion: 0.11
Nodes (27): CitSummary(), CitSummaryProps, statuses, statusMeta, CitTable(), CitTableProps, columns, statusConfig (+19 more)

### Community 13 - "Tsconfig Compileroptions Ref"
Cohesion: 0.06
Nodes (35): compilerOptions, allowArbitraryExtensions, allowImportingTsExtensions, erasableSyntaxOnly, forceConsistentCasingInFileNames, jsx, lib, module (+27 more)

### Community 14 - "Atm Portal Profile"
Cohesion: 0.12
Nodes (22): toListATMCashposResponse(), AtmPortalService, ListCashposParams, ListCashposResult, numericToDecimalString(), rowToCashposRow(), numericToFloat64Ptr(), AtmPortalService (+14 more)

### Community 15 - "Atm Portal Filterbar"
Cohesion: 0.07
Nodes (29): AtmTable(), BRAND_OPTIONS, ChipGroup(), ChipGroupProps, DEPLOYMENT_TYPE_OPTIONS, FilterBar(), FilterBarProps, MACHINE_TYPE_OPTIONS (+21 more)

### Community 16 - "Itm Replenish Scheduler"
Cohesion: 0.10
Nodes (34): compute_checksum(), create_file_record(), extract_file_date(), file_already_processed(), insert_rows_batch(), main(), move_to_backup(), parse_csv_row() (+26 more)

### Community 17 - "Atm Portal Property"
Cohesion: 0.18
Nodes (31): New(), TestProperty_DateRangeFilterCorrectness(), containsFold(), pgx.Tx, insertTestATMWithLifecycle(), nullableFloatOrderOK(), nullableTimeOrderOK(), sortedPairOK() (+23 more)

### Community 18 - "Feedback Errorboundary Skeleton"
Cohesion: 0.11
Nodes (19): ErrorBoundary, ErrorBoundaryProps, ErrorBoundaryState, ErrorFallbackProps, NetworkError(), NetworkErrorProps, containsSensitiveDetails(), sanitizeServerError() (+11 more)

### Community 19 - "Card Loginpage Auth"
Cohesion: 0.06
Nodes (12): ButtonProps, CardProps, DataTableProps, FilterSelectProps, PageHeaderProps, CardProps, FileUploadProps, isNonWhitespace() (+4 more)

### Community 20 - "Formatters Date Derivestatus"
Cohesion: 0.11
Nodes (20): cn(), formatDate(), formatDateShort(), formatDateTime(), BalanceStatus, deriveStatus(), compoundFilter(), filterByField() (+12 more)

### Community 21 - "Tsconfig Compileroptions Json"
Cohesion: 0.06
Nodes (31): compilerOptions, allowImportingTsExtensions, baseUrl, forceConsistentCasingInFileNames, jsx, lib, module, moduleDetection (+23 more)

### Community 22 - "Tsconfig Compileroptions Ref"
Cohesion: 0.06
Nodes (31): compilerOptions, allowImportingTsExtensions, baseUrl, jsx, lib, module, moduleDetection, moduleResolution (+23 more)

### Community 23 - "Dmaa Forecast Testlistdmaaforecast"
Cohesion: 0.14
Nodes (21): CountDmaaForecastParams, ListDmaaForecastParams, Queries, DmaaAtmForecast, ListDmaaForecastParams, ListDmaaForecastResult, NewDmaaForecastService(), sampleDmaaRow() (+13 more)

### Community 24 - "Config Atm Portal"
Cohesion: 0.17
Nodes (28): mountCashposHandler(), TestListCashpos_BadPage(), TestListCashpos_Defaults(), TestListCashpos_Empty(), TestListCashpos_InternalError(), TestListCashpos_QueryForwarding(), TestListCashpos_SuccessAllFields(), TestListCashpos_ValidationError() (+20 more)

### Community 25 - "Dsr Dsrtable Dsrdashboard"
Cohesion: 0.14
Nodes (23): DsrDashboard(), computeDsrTotals(), DsrSummary(), DsrSummaryProps, columnHelper, columns, DsrTable(), DsrTableProps (+15 more)

### Community 26 - "Middleware Rbac Property"
Cohesion: 0.17
Nodes (26): chi.Router, net/http.Handler, AuthContext, contextKey, GetAuthContext(), containsRole(), identityGen(), newTestTokenService() (+18 more)

### Community 27 - "Auth Authcontext Routeguard"
Cohesion: 0.11
Nodes (18): ApiErrorResponse, AuthContext, AuthContextValue, AuthProvider(), initialize(), AuthProviderProps, LoginSuccessResponse, mapUserResponse() (+10 more)

### Community 28 - "Itm Cashpos Scheduler"
Cohesion: 0.13
Nodes (25): compute_checksum(), create_file_record(), extract_file_date(), file_already_processed(), insert_rows_batch(), main(), move_to_backup(), parse_csv_row() (+17 more)

### Community 29 - "Scheduler Retry Routers"
Cohesion: 0.13
Nodes (20): datetime, FastAPI, Request, Auth dependency for protected endpoints (task 4.1)., Validate API key or JWT on protected endpoints. Returns the user identity., require_auth(), create_app(), lifespan() (+12 more)

### Community 30 - "Forecast Schedulelist Forecasttable"
Cohesion: 0.17
Nodes (20): columns, ForecastTable(), ForecastTableProps, priorityOrder, ForecastView(), priorityOptions, formatDate(), GroupedSchedule (+12 more)

### Community 31 - "Routes Atm Portal"
Cohesion: 0.13
Nodes (16): Register, rootElement, router, routeTree, @tanstack/react-router, atmPortalRoute, atmProfileRoute, cashFlowRoute (+8 more)

### Community 32 - "Opencode Plugins Changed"
Cohesion: 0.09
Nodes (17): ECCHooksPlugin(), hasProjectFile(), resolvePath(), ECCHooksPluginFn, FileEvent, getECCVersion(), PermissionEvent, TodoEvent (+9 more)

### Community 33 - "Replenishment Filter Property"
Cohesion: 0.15
Nodes (20): filterSchedules(), sortByStatusPriority(), STATUS_PRIORITY, columnHelper, columns, ReplenishmentScreen(), schedules, STATUS_CONFIG (+12 more)

### Community 34 - "Scheduler Retry Services"
Cohesion: 0.10
Nodes (16): Settings for the EOD Retry Scheduler service (env-driven, RETRY_ prefix)., _demo(), Async asyncpg connection pool + health check + DB retry helper. No ORM: the…, Execute a DB operation with exponential backoff retry (1s, 2s, 4s...)., with_db_retry(), _demo(), Path, ETL subprocess invocation for retry attempts (task 9.1). (+8 more)

### Community 35 - "Package Dependencies React"
Cohesion: 0.08
Nodes (25): clsx, dependencies, clsx, @hookform/resolvers, react, react-hook-form, recharts, tailwind-merge (+17 more)

### Community 36 - "Reconciliation Reconciliationscreen Filter"
Cohesion: 0.15
Nodes (17): filterExceptions(), matchesExceptionType(), matchesSeverity(), columnHelper, columns, data, exceptionTypeOptions, ReconciliationScreen() (+9 more)

### Community 37 - "Integration Testintegration Miniredis"
Cohesion: 0.21
Nodes (22): doLogin(), doLogout(), doRefresh(), extractRefreshCookie(), miniredis.Miniredis, redis.Client, runMigrations(), setupHarness() (+14 more)

### Community 38 - "Atm Portal Cashpos"
Cohesion: 0.14
Nodes (20): mustNumeric(), sampleCashposRow(), TestListCashpos_CountError(), TestListCashpos_Empty(), TestListCashpos_ForwardsFilters(), TestListCashpos_MapsAllFieldsAndDecimalStrings(), TestListCashpos_RepoError(), TestListCashpos_Validation() (+12 more)

### Community 39 - "Package Devdependencies Json"
Cohesion: 0.09
Nodes (23): @biomejs/biome, devDependencies, @biomejs/biome, tailwindcss, @tailwindcss/vite, @testing-library/jest-dom, @testing-library/react, @testing-library/user-event (+15 more)

### Community 40 - "Atm Portal Formatters"
Cohesion: 0.14
Nodes (16): AtmRow(), AtmTableProps, Column, COLUMNS, DataFreshnessIndicator(), DataFreshnessIndicatorProps, formatAtmDate(), formatAtmDateTime() (+8 more)

### Community 41 - "Github Com Jackc"
Cohesion: 0.20
Nodes (21): ItmCashpo, Atm, AtmDenom, AtmVendorPackage, Currency, Denom, DmaaFile, ItmCashposFile (+13 more)

### Community 42 - "Scheduler Retry Services"
Cohesion: 0.11
Nodes (15): Process automatic retries for eligible failed files, isolating failures per…, Check if the SLA deadline passed without completion (Requirement 2)., Run one automatic retry attempt for a failed file (state machine transitions)., Manages periodic detection scans, retry cycles, and retry orchestration., Register jobs and start the APScheduler., Execute failure detection with mutual exclusion (Property 17)., SchedulerService, current_processing_date() (+7 more)

### Community 43 - "Invoice Invoiceflow Invoicedetail"
Cohesion: 0.23
Nodes (15): InvoiceDetail(), InvoiceDetailProps, matchStatusConfig, columnHelper, columns, InvoiceFlow(), loadInvoices(), statusConfig (+7 more)

### Community 44 - "Node Tsconfig Compileroptions"
Cohesion: 0.10
Nodes (20): compilerOptions, allowImportingTsExtensions, lib, module, moduleDetection, moduleResolution, noEmit, noFallthroughCasesInSwitch (+12 more)

### Community 45 - "Node Tsconfig Compileroptions"
Cohesion: 0.10
Nodes (20): compilerOptions, allowImportingTsExtensions, erasableSyntaxOnly, lib, module, moduleDetection, noEmit, noFallthroughCasesInSwitch (+12 more)

### Community 46 - "Atm Portal Time"
Cohesion: 0.19
Nodes (15): dateToTimePtr(), AtmPortalService, ListATMsParams, ListATMsResult, pgTimeToStringPtr(), pgTimeToHHMMSS(), rowToAtmWithCashPos(), timestampToTimePtr() (+7 more)

### Community 47 - "Atm Portal Useatmprofiledata"
Cohesion: 0.19
Nodes (14): announcementFor(), AtmProfileScreen(), TabNavigation(), focusAndActivate(), handleKeyDown(), buildHistoryQueryString(), fetchAtmCashposHistory(), fetchAtmMasterData() (+6 more)

### Community 48 - "Atm Portal Useatmprofileurlstate"
Cohesion: 0.17
Nodes (16): Tab, TabNavigationProps, TABS, AtmCashposParams, AtmCashposResponse, AtmPortalResponse, AtmProfileHistoryParams, AtmProfileTab (+8 more)

### Community 49 - "Dmaa Scheduler Bak"
Cohesion: 0.21
Nodes (19): enforce_column_set(), FieldRule, main(), move_to_backup(), normalize_empty_values(), normalize_headers(), parse_boolean(), parse_date() (+11 more)

### Community 50 - "Package React Dependencies"
Cohesion: 0.11
Nodes (19): dependencies, @hookform/resolvers, lucide-react, react, react-dom, react-hook-form, react-router, @tanstack/react-query (+11 more)

### Community 51 - "Package Devdependencies Json"
Cohesion: 0.11
Nodes (19): devDependencies, fast-check, oxlint, @tailwindcss/vite, @testing-library/jest-dom, @testing-library/react, @testing-library/user-event, @types/node (+11 more)

### Community 52 - "Atm Portal Replenishtable"
Cohesion: 0.17
Nodes (13): ariaLabelForAmount(), AtmHeader(), AtmHeaderProps, Field, fields(), cellValue(), Column, COLUMNS (+5 more)

### Community 53 - "Config Navigation Property"
Cohesion: 0.17
Nodes (13): filterNavByRoles(), GROUP_LABELS, NAV_CONFIG, NavGroup, NavItem, ADMIN_ROLES, ALL_DB_ROLES, arbAdminRole (+5 more)

### Community 54 - "Scheduler Retry Services"
Cohesion: 0.16
Nodes (12): NamedTuple, DetectedFile, FileDetector, File failure detection + late (SLA) detection services (tasks 7.1, 7.2, 8.1)., Scans filesystem directories to find failed/unprocessed files., Find files in the not_processed directory for a given file type., Find files remaining in the input directory after the ETL window closed., compute_checksum() (+4 more)

### Community 55 - "Scheduler Retry Services"
Cohesion: 0.15
Nodes (15): post, extract_user_id(), Get user identity for audit log. Never logs the token itself., manual_retry(), Request, Response, UUID, POST /retry/{file_id} -- manual retry (task 14.3). (+7 more)

### Community 56 - "Dmaa Bak Scheduler"
Cohesion: 0.29
Nodes (16): ensure_expected_schema(), get_connection(), get_file_date_from_modified_time(), get_source_system(), has_wildcard(), load_csv_to_postgres(), LoadResult, move_file_to_bak() (+8 more)

### Community 57 - "Scheduler Retry Services"
Cohesion: 0.15
Nodes (11): LateDetector, date, Pool, time, UUID, Create a scan_runs entry recording this scan's outcome., Checks whether SLA deadlines have passed without completed processing., True if file_type is late for the given processing_date. (+3 more)

### Community 58 - "Auth Local Provider"
Cohesion: 0.21
Nodes (10): LocalProvider, main(), NewLocalProvider(), hashPassword(), TestLocalProvider_Authenticate_NilPasswordHash(), TestLocalProvider_Authenticate_RepoError(), TestLocalProvider_Authenticate_UserNotFound(), TestLocalProvider_Authenticate_ValidPassword() (+2 more)

### Community 59 - "Auth Mockuserrepository Stubuserrepo"
Cohesion: 0.19
Nodes (5): mockUserRepository, stubUserRepo, timestamptzToPtr(), UserRecord, AuthRepository

### Community 60 - "Scheduler Retry Schemas"
Cohesion: 0.24
Nodes (14): BaseModel, Enum, APIResponse, AuditLogItem, FileStatusItem, FileType, LateDetectionItem, ManualRetryResponse (+6 more)

### Community 61 - "Atm Portal Cashposprofiletable"
Cohesion: 0.16
Nodes (9): CashposProfileTable(), DENOM_FIELDS, DENOMINATIONS, denomKey(), PAGE_SIZE_OPTIONS, PaginationControls(), PaginationControlsProps, formatWholeNumber() (+1 more)

### Community 62 - "Routes Auth Login"
Cohesion: 0.19
Nodes (10): formatRetryAfter(), LoginFormData, LoginPage(), loginSchema, arbNonEmptyCredential, arbRetryAfterSeconds, arbWhitespaceOnly, remainingRateLimitSeconds() (+2 more)

### Community 63 - "Routes Protected Forecasting"
Cohesion: 0.16
Nodes (7): eodMonitoringRoute, dmaaForecastRoute, dsrUploadRoute, ALL_DB_ROLES, NON_ADMIN_ROLES, ROLE_RESTRICTED_ROUTES, requireRoles()

### Community 64 - "Styles Contrast Property"
Cohesion: 0.21
Nodes (14): ALL_PAIRS, ColorPair, computeContrastFromOklch(), contrastRatio(), LARGE_TEXT_PAIRS, linearToSrgb(), NORMAL_TEXT_PAIRS, oklabToLinearSrgb() (+6 more)

### Community 65 - "Authstate Authuser Balancestatus"
Cohesion: 0.13
Nodes (14): AuthState, AuthUser, BalanceStatus, CITOrder, DbRole, DsrRecord, EvidenceFile, HandoverEvidence (+6 more)

### Community 66 - "Atm Portal Integration"
Cohesion: 0.41
Nodes (13): decodeListATMsResponse(), doGetATMs(), findRow(), chi.Router, pgx.Tx, insertATM(), insertLocation(), seedRegionID() (+5 more)

### Community 67 - "Atm Portal Atmcashpostable"
Cohesion: 0.16
Nodes (9): AtmCashposTable(), AtmCashposTableProps, CashposRow(), cellValue(), Column, COLUMNS, MONEY_KEYS, CashposProfileTableProps (+1 more)

### Community 68 - "Scheduler Retry Config"
Cohesion: 0.21
Nodes (7): BaseSettings, field_validator, Path, time, Settings, main(), Uvicorn entrypoint (task 17.1). Run: python -m retry_scheduler.run

### Community 69 - "Layout Header Appshell"
Cohesion: 0.23
Nodes (5): AppShell(), AppShellProps, Header(), HeaderProps, mockUser

### Community 70 - "Middleware Rate Limiter"
Cohesion: 0.32
Nodes (12): miniredis.Miniredis, RateLimiter, isRateLimitError(), newTestRateLimiter(), TestRateLimiter_AllowsBeforeUsernameLimit(), TestRateLimiter_AllowsUpToIPLimit(), TestRateLimiter_AllowsUpToUsernameLimit(), TestRateLimiter_BlocksAt21stIPAttempt() (+4 more)

### Community 71 - "Dmaa Scheduler Etl"
Cohesion: 0.31
Nodes (12): archive(), connect(), file_checksum(), integer_value(), log_missing_terminals(), main(), process_file(), Any (+4 more)

### Community 72 - "Schedule Grouping Property"
Cohesion: 0.17
Nodes (10): dateArb, DateGroup, Priority, priorityArb, priorityOrder, Schedule, scheduleArb, schedulesArb (+2 more)

### Community 73 - "Api Client Apiclient"
Cohesion: 0.26
Nodes (10): api, apiClient(), ApiError, ApiResponse, createApiError(), executeRequest(), handleUnauthorized(), injectAuthHeader() (+2 more)

### Community 74 - "Middleware Ratelimiter Rate"
Cohesion: 0.33
Nodes (4): RateLimitConfig, RateLimiter, redis.Client, NewRateLimiter()

### Community 75 - "Scheduler Retry Services"
Cohesion: 0.21
Nodes (7): AuditService, date, Pool, UUID, Append-only audit log writer (task 10.1). No update/delete methods are exposed…, Writes retry_audit_logs entries. Append-only: no update/delete methods., Pool

### Community 76 - "Appshell Errorboundary Handlekeydown"
Cohesion: 0.18
Nodes (4): AppShell(), ErrorBoundary, ErrorBoundaryProps, ErrorBoundaryState

### Community 77 - "Scheduler Retry Database"
Cohesion: 0.20
Nodes (11): check_db_health(), create_db_pool(), get_db_pool(), Pool, Create the asyncpg connection pool., Lifespan helper: yields a pool, closes it on exit., True if a trivial query succeeds against the pool., health() (+3 more)

### Community 78 - "Schedulepage Schedule Dategroup"
Cohesion: 0.24
Nodes (9): DateGroup, formatDateHeader(), groupAndSort(), Priority, priorityBadgeMap, priorityOrder, SchedulePage(), ScheduleStatus (+1 more)

### Community 79 - "Datafilters Property Alldates"
Cohesion: 0.22
Nodes (5): allDates, dateInRange(), dateRangeArb(), orderStatusArb, vendorIdArb

### Community 80 - "Atm Portal Arialiveregion"
Cohesion: 0.33
Nodes (7): announcementFor(), AriaLiveRegion(), AriaLiveRegionProps, datasetLabel(), TableModeSelect(), TableModeSelectProps, AtmPortalMode

### Community 81 - "Usetoast Hooks Toast"
Cohesion: 0.25
Nodes (8): Toast, ToastActions, ToastOptions, ToastState, ToastStore, ToastType, useToast(), useToastStore

### Community 82 - "Routes Root Auth"
Cohesion: 0.28
Nodes (4): authRoute, loginRoute, queryClient, rootRoute

### Community 83 - "Strings Language Property"
Cohesion: 0.22
Nodes (4): FEATURES_DIR, FORBIDDEN_ENGLISH_PHRASES, FORBIDDEN_STANDALONE_LABELS, PERMITTED_ENGLISH_TERMS

### Community 84 - "Oxlintrc Rules Ref"
Cohesion: 0.22
Nodes (8): plugins, rules, react/only-export-components, react/rules-of-hooks, $schema, oxc, typescript, warn

### Community 85 - "Notifications Notificationspage Usenotifications"
Cohesion: 0.42
Nodes (5): formatTimestamp(), NotificationsPage(), useMarkAllAsRead(), useMarkAsRead(), useNotifications()

### Community 86 - "Validation Property Accepted"
Cohesion: 0.22
Nodes (7): ACCEPTED_TYPES, fileNameArb, FileValidationResult, invalidSizeArb, invalidTypeArb, validSizeArb, validTypeArb

### Community 87 - "Dmaa Forecast Testlistdmaaforecast"
Cohesion: 0.43
Nodes (7): dmaaGet(), mountDmaaHandler(), TestListDmaaForecast_BadRequestCases(), TestListDmaaForecast_DefaultsApplied(), TestListDmaaForecast_ServiceErrors(), TestListDmaaForecast_SuccessEnvelope(), TestParseDmaaForecastParams_PassesFilters()

### Community 88 - "Package Scripts Build"
Cohesion: 0.25
Nodes (8): scripts, build, dev, format, lint, preview, test, test:watch

### Community 89 - "Sidebar Layout Group"
Cohesion: 0.29
Nodes (4): GROUP_ORDER, NavItemButtonProps, Sidebar(), SidebarProps

### Community 91 - "Queries Auth Sql"
Cohesion: 0.33
Nodes (3): Queries, FindUserByUsernameRow, GetUserProfileRow

### Community 93 - "Hardcoded Colors Property"
Cohesion: 0.33
Nodes (5): collectSourceFiles(), FEATURES_DIR, findColorViolations(), sourceFiles, stripComments()

### Community 94 - "Package Scripts Build"
Cohesion: 0.29
Nodes (7): scripts, build, dev, lint, preview, test, test:watch

### Community 95 - "Button Buttonprops Buttonsize"
Cohesion: 0.29
Nodes (5): ButtonProps, ButtonSize, ButtonVariant, sizeStyles, variantStyles

### Community 96 - "Orderspage Orders Columnhelper"
Cohesion: 0.29
Nodes (5): columnHelper, columns, OrderStatus, statusBadgeMap, StatusFilter

### Community 98 - "Package Name Packagemanager"
Cohesion: 0.33
Nodes (5): name, packageManager, private, type, version

### Community 99 - "Layout Accessibility Property"
Cohesion: 0.33
Nodes (4): arbAccessibleIconOnlyConfig, arbIconButtonConfig, arbInaccessibleIconOnlyConfig, IconButtonConfig

### Community 100 - "Toast Config Toastcontainer"
Cohesion: 0.40
Nodes (3): TOAST_CONFIG, ToastContainer(), ToastItemProps

### Community 101 - "Invoicespage Invoices Invoicerow"
Cohesion: 0.33
Nodes (3): InvoiceRowProps, statusBadgeMap, ValidationStatus

### Community 102 - "Notifications Routing Property"
Cohesion: 0.33
Nodes (4): NotificationType, notificationTypeArb, typeToRoute, VALID_ROUTES

### Community 103 - "Constants Threshold Critical"
Cohesion: 0.33
Nodes (5): CRITICAL_THRESHOLD, LOW_THRESHOLD, NAV_ITEMS, NavItem, ROUTES

### Community 105 - "Sorting Property Recordarb"
Cohesion: 0.33
Nodes (3): recordArb, recordsArb, scheduledDateArb

### Community 106 - "Opencode Ref Instructions"
Cohesion: 0.33
Nodes (5): instructions, plugin, $schema, .opencode/opencode.md, .opencode/plugins

### Community 107 - "Queries Dbtx Pgx"
Cohesion: 0.50
Nodes (3): DBTX, Queries, pgx.Tx

### Community 108 - "Rename Itm Cashpos"
Cohesion: 0.60
Nodes (4): connectForBugConditionTest(), TestBugCondition_IndexesUseItmReplenishNaming(), TestBugCondition_NoOldTableNameReferencesInCode(), TestBugCondition_TablesRenamedToItmReplenish()

### Community 109 - "Retry Migrations Scheduler"
Cohesion: 0.40
Nodes (4): public.late_detections, public.retry_audit_logs, public.retry_file_tracking, public.scan_runs

### Community 110 - "Badge Badgeprops Badgevariant"
Cohesion: 0.40
Nodes (3): BadgeProps, BadgeVariant, variantStyles

### Community 111 - "Noticebanner Noticebannerprops Noticebannervariant"
Cohesion: 0.40
Nodes (3): NoticeBannerProps, NoticeBannerVariant, variantStyles

### Community 112 - "Package Name Private"
Cohesion: 0.40
Nodes (4): name, private, type, version

### Community 113 - "Badge Badgeprops Defaulticons"
Cohesion: 0.40
Nodes (3): BadgeProps, defaultIcons, variantClasses

### Community 114 - "Datatable Columnmeta Datatableprops"
Cohesion: 0.40
Nodes (3): ColumnMeta, DataTableProps, @tanstack/react-table

### Community 115 - "Dsrpage Dsr Balancestatusbadgemap"
Cohesion: 0.40
Nodes (3): balanceStatusBadgeMap, columnHelper, columns

### Community 116 - "Evidenceform Evidence Evidenceformdata"
Cohesion: 0.40
Nodes (3): EvidenceFormData, EvidenceFormProps, evidenceSchema

### Community 117 - "Evidencepage Evidence Existingevidence"
Cohesion: 0.60
Nodes (3): ExistingEvidence(), formatDateTime(), isImageFile()

### Community 120 - "Summarycard Formatvalue Summarycardprops"
Cohesion: 0.67
Nodes (3): formatValue(), SummaryCard(), SummaryCardProps

### Community 121 - "Data Json Integrity"
Cohesion: 0.50
Nodes (3): REQUIRED_JSON_FILES, SOURCE_DATA_DIR, TARGET_DATA_DIR

### Community 126 - "Datatable Testcolumns Testdata"
Cohesion: 0.50
Nodes (3): testColumns, testData, TestRow

### Community 129 - "Scheduler Audit Retry"
Cohesion: 0.50
Nodes (4): get_audit(), date, get, Request

### Community 130 - "Scheduler Late Retry"
Cohesion: 0.50
Nodes (4): get_late(), date, get, Request

### Community 131 - "Scheduler Summary Retry"
Cohesion: 0.50
Nodes (4): get_summary(), date, get, Request

### Community 142 - "Github Com Cimb"
Cohesion: 0.67
Nodes (3): github.com/cimb-niaga/cms/backend, github.com/cimb-niaga/cms/backend-cit, github.com/cimb-niaga/cms/pkg

## Knowledge Gaps
- **626 isolated node(s):** `$schema`, `.opencode/opencode.md`, `.opencode/plugins`, `ToolArgs`, `ToolInput` (+621 more)
  These have ≤1 connection - possible missing edges or undocumented components.
- **39 thin communities (<3 nodes) omitted from report** — run `graphify query` to explore isolated nodes.

## Suggested Questions
_Questions this graph is uniquely positioned to answer:_

- **Why does `react` connect `Card Loginpage Auth` to `Eod Monitoring Hooks`, `Cash Flow Atmleveltable`, `Forecasting Dsr Dsruploadform`, `Dmaa Forecast Usedmaaforecasturlstate`, `Atm Portal Useatmportalurlstate`, `Cit Filter Property`, `Atm Portal Filterbar`, `Feedback Errorboundary Skeleton`, `Dsr Dsrtable Dsrdashboard`, `Auth Authcontext Routeguard`, `Forecast Schedulelist Forecasttable`, `Routes Atm Portal`, `Replenishment Filter Property`, `Reconciliation Reconciliationscreen Filter`, `Invoice Invoiceflow Invoicedetail`, `Atm Portal Useatmprofiledata`, `Atm Portal Useatmprofileurlstate`, `Atm Portal Cashposprofiletable`, `Routes Auth Login`, `Layout Header Appshell`, `Appshell Errorboundary Handlekeydown`, `Schedulepage Schedule Dategroup`, `Routes Root Auth`, `Oxlintrc Rules Ref`, `Sidebar Layout Group`, `Integration Conditionalthrower Mockfetch`, `Button Buttonprops Buttonsize`, `Orderspage Orders Columnhelper`, `Toast Config Toastcontainer`, `Invoicespage Invoices Invoicerow`, `Datatable Columnmeta Datatableprops`, `Dsrpage Dsr Balancestatusbadgemap`, `Evidenceform Evidence Evidenceformdata`?**
  _High betweenness centrality (0.099) - this node is a cross-community bridge._
- **Why does `GetAuthContext()` connect `Middleware Rbac Property` to `Response Atm Portal`, `Atm Portal Sql`?**
  _High betweenness centrality (0.005) - this node is a cross-community bridge._
- **Why does `TokenService` connect `Auth Token Property` to `Response Atm Portal`, `Middleware Rbac Property`, `Integration Testintegration Miniredis`?**
  _High betweenness centrality (0.004) - this node is a cross-community bridge._
- **Are the 19 inferred relationships involving `NewAtmPortalService()` (e.g. with `TestListCashpos_CountError()` and `TestListCashpos_Empty()`) actually correct?**
  _`NewAtmPortalService()` has 19 INFERRED edges - model-reasoned connections that need verification._
- **What connects `$schema`, `.opencode/opencode.md`, `.opencode/plugins` to the rest of the system?**
  _626 weakly-connected nodes found - possible documentation gaps or missing edges._
- **Should `Eod Monitoring Hooks` be split into smaller, more focused modules?**
  _Cohesion score 0.060784313725490195 - nodes in this community are weakly interconnected._
- **Should `Response Atm Portal` be split into smaller, more focused modules?**
  _Cohesion score 0.06263173742848539 - nodes in this community are weakly interconnected._