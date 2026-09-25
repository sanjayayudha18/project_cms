//go:build integration

package service

import (
	"context"
	"errors"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/cimb-niaga/cms/backend/internal/db"
	"github.com/cimb-niaga/cms/backend/internal/repository"
)

// regionHarness opens a real DB connection wrapped in an outer transaction
// rolled back on cleanup, and passes that same tx as both the primary/
// replica pools and RegionAdminPool -- same convention as
// rolemgmt.harness/TestCreateRole_* (internal/rolemgmt/service_integration_test.go):
// pgx.Tx.Begin opens a savepoint-based pseudo nested transaction, which is
// exactly what RegionAdminService.pool.Begin needs to exercise real
// commit/rollback behavior with no DB/tx mocking.
func regionHarness(t *testing.T) (*RegionAdminService, int64, string) {
	t.Helper()
	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		t.Skip("DATABASE_URL not set — skipping integration test")
	}

	ctx := context.Background()
	pool, err := pgxpool.New(ctx, dbURL)
	if err != nil {
		t.Fatalf("connecting to database: %v", err)
	}
	t.Cleanup(pool.Close)

	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatalf("begin outer tx: %v", err)
	}
	t.Cleanup(func() { _ = tx.Rollback(ctx) })

	var adminUserID int64
	if err := tx.QueryRow(ctx, `SELECT id FROM users WHERE username = 'Yudha'`).Scan(&adminUserID); err != nil {
		t.Fatalf("loading seeded admin user: %v", err)
	}

	repo := repository.NewRegionAdminRepository(tx, tx)
	svc := NewRegionAdminService(repo, tx)
	// Region codes are restricted to [A-Za-z0-9], max 20 chars (regionCodeRe)
	// -- base36 keeps the tag short and alphanumeric, unlike rolemgmt's
	// hyphenated TROLE- tags. Uppercased because Create normalizes codes to
	// uppercase (Req 2.7): callers below compare created.Code directly
	// against "<PREFIX>"+tag, so tag must already be in its normalized form.
	tag := strings.ToUpper(strconv.FormatInt(time.Now().UnixNano()%1e12, 36))
	return svc, adminUserID, tag
}

func TestRegionAdminService_ListCountGet(t *testing.T) {
	svc, adminUserID, tag := regionHarness(t)
	ctx := context.Background()
	code := "RGLIST" + tag

	created, err := svc.Create(ctx, adminUserID, "ADMIN", CreateRegionRequest{Code: code, Region: "List Test"}, "")
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	q := code
	got, err := svc.List(ctx, db.ListRegionsAdminParams{Q: &q, Status: "all", PageLimit: 100})
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(got) != 1 || got[0].ID != created.ID {
		t.Fatalf("List(q=%q) = %+v, want exactly the created region", q, got)
	}

	count, err := svc.Count(ctx, db.CountRegionsAdminParams{Q: &q, Status: "all"})
	if err != nil {
		t.Fatalf("Count: %v", err)
	}
	if count != 1 {
		t.Fatalf("Count = %d, want 1", count)
	}

	fetched, err := svc.Get(ctx, created.ID)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if fetched.Code != code {
		t.Fatalf("Get.Code = %q, want %q", fetched.Code, code)
	}

	if _, err := svc.Get(ctx, -1); !errors.Is(err, ErrRegionNotFound) {
		t.Fatalf("Get(-1) error = %v, want ErrRegionNotFound", err)
	}
}

// TestCreateRegion_NormalizesCodeAndWritesAudit is design property 1 (create
// side) + property 4 (success branch).
func TestCreateRegion_NormalizesCodeAndWritesAudit(t *testing.T) {
	svc, adminUserID, tag := regionHarness(t)
	ctx := context.Background()
	rawCode := "  rgnorm" + tag + "  "
	wantCode := "RGNORM" + tag

	created, err := svc.Create(ctx, adminUserID, "ADMIN", CreateRegionRequest{Code: rawCode, Region: "  Norm Test  "}, "127.0.0.1")
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if created.Code != wantCode {
		t.Fatalf("Code = %q, want %q (trim+uppercase)", created.Code, wantCode)
	}
	if created.Region == nil || *created.Region != "Norm Test" {
		t.Fatalf("Region = %v, want trimmed %q", created.Region, "Norm Test")
	}
	if !created.IsActive {
		t.Error("IsActive = false, want true on create")
	}

	var count int
	if err := queryRowScanRegion(ctx, svc, `SELECT COUNT(*) FROM audit_logs WHERE entity_type = 'region' AND entity_id = $1 AND action = 'region_created'`, created.ID, &count); err != nil {
		t.Fatalf("counting audit_logs: %v", err)
	}
	if count != 1 {
		t.Fatalf("audit_logs rows for region_created = %d, want 1", count)
	}
}

// TestCreateRegion_CodeConflictRejectsAndCreatesNoRow is design property 1
// (uniqueness on the normalized form -- differing case/whitespace collide).
func TestCreateRegion_CodeConflictRejectsAndCreatesNoRow(t *testing.T) {
	svc, adminUserID, tag := regionHarness(t)
	ctx := context.Background()
	code := "RGDUP" + tag

	if _, err := svc.Create(ctx, adminUserID, "ADMIN", CreateRegionRequest{Code: code, Region: "First"}, ""); err != nil {
		t.Fatalf("first Create: %v", err)
	}

	lowerVariant := "  " + code + "  " // same normalized code, different casing/whitespace
	_, err := svc.Create(ctx, adminUserID, "ADMIN", CreateRegionRequest{Code: lowerVariant, Region: "Second"}, "")
	if !errors.Is(err, ErrRegionCodeConflict) {
		t.Fatalf("second Create error = %v, want ErrRegionCodeConflict", err)
	}

	var count int
	if err := queryRowScanRegion(ctx, svc, `SELECT COUNT(*) FROM regions WHERE code = $1`, code, &count); err != nil {
		t.Fatalf("counting regions: %v", err)
	}
	if count != 1 {
		t.Fatalf("regions with code %q = %d rows, want exactly 1 (no duplicate persisted)", code, count)
	}
}

// TestCreateRegion_ValidationRejectsBadCodeAndName is design property 2.
func TestCreateRegion_ValidationRejectsBadCodeAndName(t *testing.T) {
	svc, adminUserID, tag := regionHarness(t)
	ctx := context.Background()

	cases := []struct {
		name   string
		code   string
		region string
	}{
		{"empty code", "", "Valid Name"},
		{"whitespace code", "   ", "Valid Name"},
		{"code too long", "THISCODEISWAYTOOLONG" + tag, "Valid Name"},
		{"code non-alphanumeric", "RG-" + tag, "Valid Name"},
		{"empty name", "RGVAL" + tag, ""},
		{"whitespace name", "RGVAL" + tag, "   "},
		{"name too long", "RGVAL" + tag, longString(101)},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := svc.Create(ctx, adminUserID, "ADMIN", CreateRegionRequest{Code: tc.code, Region: tc.region}, "")
			var ve *ValidationError
			if !errors.As(err, &ve) {
				t.Fatalf("Create(code=%q, region=%q) error = %v, want *ValidationError", tc.code, tc.region, err)
			}
		})
	}
}

// TestCreateRegion_AuthorizationMatrix is design property 8 (create side).
func TestCreateRegion_AuthorizationMatrix(t *testing.T) {
	svc, adminUserID, tag := regionHarness(t)
	ctx := context.Background()

	cases := []struct {
		role    string
		allowed bool
	}{
		{"ADMIN", true},
		{"ADMIN_PARAM", true},
		{"admin", true}, // case-insensitive, matches RequireRoles convention
		{"APPACCESS", false},
		{"ATM-USER", false},
		{"VENDOR-USER", false},
		{"", false},
	}
	for i, tc := range cases {
		t.Run(tc.role, func(t *testing.T) {
			code := "RGAUTH" + tag + strconv.Itoa(i)
			_, err := svc.Create(ctx, adminUserID, tc.role, CreateRegionRequest{Code: code, Region: "Auth Test"}, "")
			if tc.allowed {
				if err != nil {
					t.Fatalf("actor role %q: Create error = %v, want nil", tc.role, err)
				}
				return
			}
			if !errors.Is(err, ErrNotAuthorized) {
				t.Fatalf("actor role %q: Create error = %v, want ErrNotAuthorized", tc.role, err)
			}
			var count int
			if err := queryRowScanRegion(ctx, svc, `SELECT COUNT(*) FROM regions WHERE code = $1`, code, &count); err != nil {
				t.Fatalf("counting regions: %v", err)
			}
			if count != 0 {
				t.Fatalf("unauthorized actor %q still created a region row", tc.role)
			}
		})
	}
}

// TestCreateRegion_AuditFailureRollsBack is design property 4 (create side):
// a non-existent actor_id violates audit_logs_actor_fk, and the region
// insert must roll back with it.
func TestCreateRegion_AuditFailureRollsBack(t *testing.T) {
	svc, _, tag := regionHarness(t)
	ctx := context.Background()
	code := "RGNOAUDIT" + tag
	const nonExistentActorID = int64(999999999)

	_, err := svc.Create(ctx, nonExistentActorID, "ADMIN", CreateRegionRequest{Code: code, Region: "No Audit"}, "")
	if err == nil {
		t.Fatal("Create with a non-existent actor_id succeeded, want an error (audit FK violation)")
	}

	var count int
	if err := queryRowScanRegion(ctx, svc, `SELECT COUNT(*) FROM regions WHERE code = $1`, code, &count); err != nil {
		t.Fatalf("counting regions: %v", err)
	}
	if count != 0 {
		t.Fatalf("region %q persisted despite the audit write failing — rollback did not happen", code)
	}
}

// TestUpdateRegionName_SuccessAndCodeImmutable is design property 3.
func TestUpdateRegionName_SuccessAndCodeImmutable(t *testing.T) {
	svc, adminUserID, tag := regionHarness(t)
	ctx := context.Background()
	code := "RGUPD" + tag

	created, err := svc.Create(ctx, adminUserID, "ADMIN", CreateRegionRequest{Code: code, Region: "Before"}, "")
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	updated, err := svc.UpdateName(ctx, adminUserID, "ADMIN", created.ID, UpdateRegionNameRequest{Region: "After"}, "")
	if err != nil {
		t.Fatalf("UpdateName: %v", err)
	}
	if updated.Code != code {
		t.Fatalf("Code changed to %q, want unchanged %q", updated.Code, code)
	}
	if updated.Region == nil || *updated.Region != "After" {
		t.Fatalf("Region = %v, want %q", updated.Region, "After")
	}

	var count int
	if err := queryRowScanRegion(ctx, svc, `SELECT COUNT(*) FROM audit_logs WHERE entity_type = 'region' AND entity_id = $1 AND action = 'region_updated'`, created.ID, &count); err != nil {
		t.Fatalf("counting audit_logs: %v", err)
	}
	if count != 1 {
		t.Fatalf("audit_logs rows for region_updated = %d, want 1", count)
	}

	differentCode := code + "X"
	_, err = svc.UpdateName(ctx, adminUserID, "ADMIN", created.ID, UpdateRegionNameRequest{Code: &differentCode, Region: "Ignored"}, "")
	if !errors.Is(err, ErrRegionCodeImmutable) {
		t.Fatalf("UpdateName with a different code: err = %v, want ErrRegionCodeImmutable", err)
	}

	unchanged, err := svc.Get(ctx, created.ID)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if unchanged.Code != code || unchanged.Region == nil || *unchanged.Region != "After" {
		t.Fatalf("region mutated by the rejected code-change request: %+v", unchanged)
	}
}

func TestUpdateRegionName_NotFound(t *testing.T) {
	svc, adminUserID, _ := regionHarness(t)
	ctx := context.Background()

	_, err := svc.UpdateName(ctx, adminUserID, "ADMIN", -1, UpdateRegionNameRequest{Region: "X"}, "")
	if !errors.Is(err, ErrRegionNotFound) {
		t.Fatalf("UpdateName(nonexistent): err = %v, want ErrRegionNotFound", err)
	}
}

// TestDisableRegion_BlockedByActiveLocations is design property 5.
func TestDisableRegion_BlockedByActiveLocations(t *testing.T) {
	svc, adminUserID, tag := regionHarness(t)
	ctx := context.Background()
	code := "RGDEPLOC" + tag

	created, err := svc.Create(ctx, adminUserID, "ADMIN", CreateRegionRequest{Code: code, Region: "Has Locations"}, "")
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	dbtx := svc.pool.(db.DBTX)
	if _, err := dbtx.Exec(ctx,
		`INSERT INTO locations (region_id, type, name, address_line1, city_or_regency, province, country_code)
		 VALUES ($1, 'branch', 'Dep Loc Test', 'Jl. Test No. 1', 'Jakarta', 'DKI Jakarta', 'ID')`,
		created.ID,
	); err != nil {
		t.Fatalf("seeding dependent location: %v", err)
	}

	_, err = svc.Disable(ctx, adminUserID, "ADMIN", created.ID, "")
	if !errors.Is(err, ErrRegionHasActiveLocations) {
		t.Fatalf("Disable with a dependent location: err = %v, want ErrRegionHasActiveLocations", err)
	}

	unchanged, err := svc.Get(ctx, created.ID)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if !unchanged.IsActive || unchanged.DeletedAt.Valid {
		t.Fatalf("region status changed despite the block: %+v", unchanged)
	}
}

// TestDisableEnableRegion_RoundTrip is design property 6 (success branch).
func TestDisableEnableRegion_RoundTrip(t *testing.T) {
	svc, adminUserID, tag := regionHarness(t)
	ctx := context.Background()
	code := "RGTOGGLE" + tag

	created, err := svc.Create(ctx, adminUserID, "ADMIN", CreateRegionRequest{Code: code, Region: "Toggle"}, "")
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	disabled, err := svc.Disable(ctx, adminUserID, "ADMIN", created.ID, "")
	if err != nil {
		t.Fatalf("Disable: %v", err)
	}
	if disabled.IsActive || !disabled.DeletedAt.Valid {
		t.Fatalf("after Disable: IsActive=%v DeletedAt.Valid=%v, want false/true", disabled.IsActive, disabled.DeletedAt.Valid)
	}

	enabled, err := svc.Enable(ctx, adminUserID, "ADMIN", created.ID, "")
	if err != nil {
		t.Fatalf("Enable: %v", err)
	}
	if !enabled.IsActive || enabled.DeletedAt.Valid {
		t.Fatalf("after Enable: IsActive=%v DeletedAt.Valid=%v, want true/false", enabled.IsActive, enabled.DeletedAt.Valid)
	}

	var count int
	if err := queryRowScanRegion(ctx, svc,
		`SELECT COUNT(*) FROM audit_logs WHERE entity_type = 'region' AND entity_id = $1 AND action IN ('region_deactivated', 'region_reactivated')`,
		created.ID, &count); err != nil {
		t.Fatalf("counting audit_logs: %v", err)
	}
	if count != 2 {
		t.Fatalf("audit_logs rows for disable+enable = %d, want 2", count)
	}
}

// TestDisableEnableRegion_NoOpConflictWritesNoAudit is design property 6
// (idempotency-guard branch): disabling an already-inactive region (or
// enabling an already-active one) is rejected without a state or audit change.
func TestDisableEnableRegion_NoOpConflictWritesNoAudit(t *testing.T) {
	svc, adminUserID, tag := regionHarness(t)
	ctx := context.Background()
	code := "RGNOOP" + tag

	created, err := svc.Create(ctx, adminUserID, "ADMIN", CreateRegionRequest{Code: code, Region: "NoOp"}, "")
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	if _, err := svc.Enable(ctx, adminUserID, "ADMIN", created.ID, ""); !errors.Is(err, ErrRegionStatusUnchanged) {
		t.Fatalf("Enable an already-active region: err = %v, want ErrRegionStatusUnchanged", err)
	}

	if _, err := svc.Disable(ctx, adminUserID, "ADMIN", created.ID, ""); err != nil {
		t.Fatalf("Disable: %v", err)
	}
	if _, err := svc.Disable(ctx, adminUserID, "ADMIN", created.ID, ""); !errors.Is(err, ErrRegionStatusUnchanged) {
		t.Fatalf("Disable an already-inactive region: err = %v, want ErrRegionStatusUnchanged", err)
	}

	var count int
	if err := queryRowScanRegion(ctx, svc,
		`SELECT COUNT(*) FROM audit_logs WHERE entity_type = 'region' AND entity_id = $1 AND action IN ('region_deactivated', 'region_reactivated')`,
		created.ID, &count); err != nil {
		t.Fatalf("counting audit_logs: %v", err)
	}
	if count != 1 {
		t.Fatalf("audit_logs rows = %d, want 1 (only the single real Disable, no-op attempts wrote none)", count)
	}
}

// TestDisableRegion_AuditFailureRollsBack is design property 4 (status-change side).
func TestDisableRegion_AuditFailureRollsBack(t *testing.T) {
	svc, adminUserID, tag := regionHarness(t)
	ctx := context.Background()
	code := "RGSTATNA" + tag
	const nonExistentActorID = int64(999999999)

	created, err := svc.Create(ctx, adminUserID, "ADMIN", CreateRegionRequest{Code: code, Region: "Status No Audit"}, "")
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	_, err = svc.Disable(ctx, nonExistentActorID, "ADMIN", created.ID, "")
	if err == nil {
		t.Fatal("Disable with a non-existent actor_id succeeded, want an error (audit FK violation)")
	}

	unchanged, err := svc.Get(ctx, created.ID)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if !unchanged.IsActive || unchanged.DeletedAt.Valid {
		t.Fatalf("region status changed despite the audit write failing — rollback did not happen: %+v", unchanged)
	}
}

// TestListRegionsAdmin_SearchCaseInsensitive is design property 9.
func TestListRegionsAdmin_SearchCaseInsensitive(t *testing.T) {
	svc, adminUserID, tag := regionHarness(t)
	ctx := context.Background()
	code := "RGSEARCH" + tag

	if _, err := svc.Create(ctx, adminUserID, "ADMIN", CreateRegionRequest{Code: code, Region: "Findable " + tag}, ""); err != nil {
		t.Fatalf("Create: %v", err)
	}

	lowerQ := "findable " + tag
	got, err := svc.List(ctx, db.ListRegionsAdminParams{Q: &lowerQ, Status: "all", PageLimit: 100})
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(got) != 1 || got[0].Code != code {
		t.Fatalf("List(q=%q) = %+v, want exactly the created region (case-insensitive match)", lowerQ, got)
	}

	noMatch := "nonexistent-keyword-" + tag
	gotEmpty, err := svc.List(ctx, db.ListRegionsAdminParams{Q: &noMatch, Status: "all", PageLimit: 100})
	if err != nil {
		t.Fatalf("List (no match): %v", err)
	}
	if len(gotEmpty) != 0 {
		t.Fatalf("List(q=%q) = %+v, want empty", noMatch, gotEmpty)
	}
	countEmpty, err := svc.Count(ctx, db.CountRegionsAdminParams{Q: &noMatch, Status: "all"})
	if err != nil {
		t.Fatalf("Count (no match): %v", err)
	}
	if countEmpty != 0 {
		t.Fatalf("Count(q=%q) = %d, want 0", noMatch, countEmpty)
	}
}

func longString(n int) string {
	b := make([]byte, n)
	for i := range b {
		b[i] = 'a'
	}
	return string(b)
}

// queryRowScanRegion runs a one-off scalar assertion query directly through
// the service's underlying pool (a pgx.Tx in these tests, which also
// satisfies db.DBTX), avoiding a second DB connection per assertion. Same
// convention as rolemgmt's queryRowScan. The last element of args is the scan
// destination; the rest are query parameters.
func queryRowScanRegion(ctx context.Context, svc *RegionAdminService, sql string, args ...any) error {
	dest := args[len(args)-1]
	queryArgs := args[:len(args)-1]
	dbtx := svc.pool.(db.DBTX)
	return dbtx.QueryRow(ctx, sql, queryArgs...).Scan(dest)
}
