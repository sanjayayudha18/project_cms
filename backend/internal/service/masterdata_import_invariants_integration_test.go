//go:build integration

// External test package: these tests drive the real handlers, and handler
// imports service. Living in this directory also runs them in the same test
// binary as the export/import round-trip tests, which read whole tables and
// must not see fixture rows vanish between their export and their dry-run.
package service_test

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/csv"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/cimb-niaga/cms/backend/internal/approval"
	"github.com/cimb-niaga/cms/backend/internal/audit"
	"github.com/cimb-niaga/cms/backend/internal/db"
	"github.com/cimb-niaga/cms/backend/internal/handler"
	"github.com/cimb-niaga/cms/backend/internal/notification"
	"github.com/cimb-niaga/cms/backend/internal/repository"
	"github.com/cimb-niaga/cms/backend/internal/service"
	"github.com/cimb-niaga/cms/pkg/csvexport"
	"github.com/cimb-niaga/cms/pkg/middleware"
)

const invariantFileName = "invariant.csv"

// spyRequestStore is the real approval repository with two test hooks.
// requiredLevel > 0 replaces the policy lookup, so a two-level chain needs no
// change to approval_policies: the orchestrator runs on the pool, so a policy in
// a test tx would be invisible to it, and editing the shared row would affect
// every other session using the dev DB. createAttempts proves the
// check-then-create race in SubmitForApproval really happened (Property 3).
type spyRequestStore struct {
	*approval.Repository
	requiredLevel int32

	mu             sync.Mutex
	createAttempts map[int64]int
}

func (s *spyRequestStore) FindPolicy(ctx context.Context, documentType string, amount pgtype.Numeric) (int32, error) {
	if s.requiredLevel > 0 {
		return s.requiredLevel, nil
	}
	return s.Repository.FindPolicy(ctx, documentType, amount)
}

// RunInTx keeps the spy in front of the transaction's store, so createAttempts
// still counts the inserts.
func (s *spyRequestStore) RunInTx(ctx context.Context, fn func(store approval.RequestStore, tx db.DBTX) error) error {
	return s.Repository.RunInTx(ctx, func(store approval.RequestStore, tx db.DBTX) error {
		return fn(&spyTxStore{RequestStore: store, spy: s}, tx)
	})
}

type spyTxStore struct {
	approval.RequestStore
	spy *spyRequestStore
}

func (s *spyTxStore) CreateRequest(ctx context.Context, makerID int64, documentType string, documentID int64, amount pgtype.Numeric, requiredLevel int32) (db.ApprovalRequest, error) {
	s.spy.mu.Lock()
	s.spy.createAttempts[documentID]++
	s.spy.mu.Unlock()
	return s.RequestStore.CreateRequest(ctx, makerID, documentType, documentID, amount, requiredLevel)
}

// invariantFixture is the production wiring of cmd/api/main.go (real handlers,
// orchestrator, appliers and pool) over rows owned by this fixture alone: its own
// users, so every bookkeeping count can be scoped by user, and its own vendor and
// ATM tree, whose rows all carry tag, so a snapshot is exact and unaffected by
// other packages running against the same DB. Nothing here rolls back, because
// the code under test commits across several connections; cleanup deletes by owner.
type invariantFixture struct {
	t      *testing.T
	ctx    context.Context
	pool   *pgxpool.Pool
	router http.Handler
	store  *spyRequestStore
	tag    string

	maker, approver1, approver2 int64
	location                    int64
	vendorID, branchID          int64
	vaultID, picID              int64
	atmID, atm2ID, assignmentID int64
	vendorCode, branchCode      string
	vaultCode, terminal         string
	terminal2, packageCode      string
	assignmentStart             string
}

// newInvariantFixture wires the app. requiredLevel 0 uses the real policy row.
// notifyConn builds the connection the real ImportNotifier uses; nil = the pool.
func newInvariantFixture(t *testing.T, requiredLevel int32, notifyConn func(*pgxpool.Pool) db.DBTX) *invariantFixture {
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

	f := &invariantFixture{t: t, ctx: ctx, pool: pool, tag: "IV" + strconv.FormatInt(time.Now().UnixNano(), 10)}
	t.Cleanup(f.cleanup)
	f.seed()

	repo := approval.NewRepository(pool)
	f.store = &spyRequestStore{Repository: repo, requiredLevel: requiredLevel, createAttempts: map[int64]int{}}
	auditWriter := audit.NewWriter(pool)
	orchestrator := approval.NewOrchestrator(f.store, repo, repo, auditWriter, nil)

	var conn db.DBTX = pool
	if notifyConn != nil {
		conn = notifyConn(pool)
	}
	// smtpEnabled=false: outbox rows are stored as "skipped", so a dev API running
	// next to these tests can never mail a fixture address.
	notifier := service.NewImportNotifier(notification.NewService(false), conn)

	appliers := service.ApplierRegistry{
		"vendor":         service.VendorApplier{},
		"vendor_branch":  service.VendorBranchApplier{},
		"vendor_vault":   service.VendorVaultApplier{},
		"vendor_pic":     service.VendorPicApplier{},
		"atm":            service.ATMApplier{},
		"atm_assignment": service.ATMAssignmentApplier{},
	}
	changeRepo := repository.NewMasterDataChangeRepository(pool)
	approvals := service.NewMasterDataApprovalService(orchestrator, changeRepo, pool, appliers, auditWriter).WithImportNotifier(notifier)
	batchRepo := repository.NewMasterDataImportBatchRepository(pool, pool)
	importer := service.NewMasterDataImporter(repository.NewMasterDataExportRepository(pool)).WithConfirm(service.ImportConfirmDeps{
		Batches: batchRepo, Orchestrator: orchestrator, Audit: auditWriter, Notifier: notifier,
		Services: func(sub service.ImportSubmitter) service.ImportServices {
			return service.ImportServices{
				Vendors: service.NewVendorAdminService(repository.NewVendorAdminRepository(pool, pool), sub),
				// The child counter is only consulted on disable, which no file here does.
				Branches:    service.NewVendorBranchAdminService(repository.NewVendorBranchAdminRepository(pool, pool), sub, nil),
				Vaults:      service.NewVendorVaultAdminService(repository.NewVendorVaultAdminRepository(pool, pool), sub),
				PICs:        service.NewVendorPicAdminService(repository.NewVendorPicAdminRepository(pool, pool), sub),
				ATMs:        service.NewATMAdminService(repository.NewATMAdminRepository(pool, pool), sub),
				Assignments: service.NewATMAssignmentAdminService(repository.NewATMAssignmentAdminRepository(pool, pool), sub),
			}
		},
	})

	r := chi.NewRouter()
	// Stands in for RequireAuth: the token path is not under test, the role is ADMIN.
	r.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			id, err := strconv.ParseInt(req.Header.Get("X-Test-User"), 10, 64)
			if err != nil {
				http.Error(w, "missing test user", http.StatusUnauthorized)
				return
			}
			next.ServeHTTP(w, req.WithContext(middleware.WithAuthContext(req.Context(), &middleware.AuthContext{UserID: id, Username: "invariant", Role: "ADMIN"})))
		})
	})
	r.With(middleware.RequireRoles("ADMIN", "ADMIN_PARAM")).Mount("/import", handler.NewAdminMasterDataImportHandler(importer).Routes())
	r.Mount("/approvals", handler.NewApprovalHandler(approvals, repo).WithApplyRetry(approvals).Routes())
	f.router = r
	return f
}

func (f *invariantFixture) scanID(sql string, args ...any) int64 {
	f.t.Helper()
	var id int64
	if err := f.pool.QueryRow(f.ctx, sql, args...).Scan(&id); err != nil {
		f.t.Fatalf("fixture %q: %v", sql, err)
	}
	return id
}

func (f *invariantFixture) count(sql string, args ...any) int {
	f.t.Helper()
	return int(f.scanID(sql, args...))
}

func (f *invariantFixture) exec(sql string, args ...any) {
	f.t.Helper()
	if _, err := f.pool.Exec(f.ctx, sql, args...); err != nil {
		f.t.Fatalf("exec %q: %v", sql, err)
	}
}

func (f *invariantFixture) seed() {
	f.t.Helper()
	user := func(suffix string, supervisor *int64, level *int32) int64 {
		name := strings.ToLower(f.tag) + "-" + suffix
		return f.scanID(`INSERT INTO users (role_id, username, full_name, email, is_karyawan, auth_source, supervisor_id, approval_level)
			SELECT id, $1, $1, $1 || '@invariant.example.com', true, 'ldap', $2::bigint, $3::int FROM roles WHERE role = 'ADMIN' RETURNING id`, name, supervisor, level)
	}
	one, two := int32(1), int32(2)
	f.approver2 = user("a2", nil, &two)
	f.approver1 = user("a1", &f.approver2, &one)
	f.maker = user("m", &f.approver1, nil)

	f.location = f.scanID(`SELECT min(id) FROM locations`)
	f.vendorCode, f.branchCode = f.tag+"V", f.tag+"B"
	f.vaultCode, f.terminal, f.terminal2 = f.tag+"VT", f.tag+"T", f.tag+"T2"
	f.packageCode = f.tag + "P"
	f.assignmentStart = time.Now().AddDate(0, 0, -30).Format("2006-01-02")

	f.vendorID = f.seedVendor(f.vendorCode, "Vendor")
	f.branchID = f.scanID(`INSERT INTO vendor_branches (vendor_id, branch_code, branch_name, category) VALUES ($1, $2, 'Cabang', 'ATM_CASH') RETURNING id`, f.vendorID, f.branchCode)
	f.vaultID = f.scanID(`INSERT INTO vendor_vaults (vendor_branch_id, vault_code, type, category, currency_code) VALUES ($1, $2, 'CASH', 'CASH', 'IDR') RETURNING id`, f.branchID, f.vaultCode)
	f.picID = f.scanID(`INSERT INTO vendor_pics (vendor_id, name, roles) VALUES ($1, 'PIC', '{vault}') RETURNING id`, f.vendorID)
	atm := func(terminal string) int64 {
		return f.scanID(`INSERT INTO atms (terminal_id, location_id, machine_type, brand, model, operation_hours, deployment_type, priority_class)
			VALUES ($1, $2, 'CRM', 'Hitachi', 'SR7500', '24_HOURS', 'OFFSITE', 'Non VIP') RETURNING id`, terminal, f.location)
	}
	f.atmID, f.atm2ID = atm(f.terminal), atm(f.terminal2)
	pkg := f.scanID(`INSERT INTO vendor_packages_branch (vendor_branch_id, package_code, machine_group, price_class, tier_min, base_price, currency, effective_start_date)
		VALUES ($1, $2, 'CDM_CRM', 'REGULAR', 1, 1, 'IDR', CURRENT_DATE - 365) RETURNING id`, f.branchID, f.packageCode)
	f.assignmentID = f.scanID(`INSERT INTO atm_vendor_packages (atm_id, vendor_package_id, effective_start_date) VALUES ($1, $2, $3::date) RETURNING id`, f.atmID, pkg, f.assignmentStart)
}

func (f *invariantFixture) seedVendor(code, name string) int64 {
	f.t.Helper()
	return f.scanID(`INSERT INTO vendors (code, name) VALUES ($1, $2) RETURNING id`, code, name)
}

// cleanup deletes by owner, children first. A failure is an error, not a log
// line: fixtures that quietly pile up broke the dev DB once (bugfixes 2026-10-29).
func (f *invariantFixture) cleanup() {
	ctx := context.Background()
	like := f.tag + "%"
	users := []int64{f.maker, f.approver1, f.approver2}
	vendors := `(SELECT id FROM vendors WHERE code LIKE $1)`
	atms := `(SELECT id FROM atms WHERE terminal_id LIKE $1)`
	steps := []struct {
		sql string
		arg any
	}{
		{`DELETE FROM notification_emails WHERE notification_id IN (SELECT id FROM notifications WHERE recipient_user_id = ANY($1))`, users},
		{`DELETE FROM notifications WHERE recipient_user_id = ANY($1)`, users},
		{`DELETE FROM audit_logs WHERE actor_id = ANY($1)`, users},
		{`DELETE FROM master_data_change_requests WHERE maker_id = ANY($1)`, users},
		{`DELETE FROM approval_steps WHERE request_id IN (SELECT id FROM approval_requests WHERE maker_id = ANY($1))`, users},
		{`DELETE FROM approval_requests WHERE maker_id = ANY($1)`, users},
		{`DELETE FROM master_data_import_batches WHERE maker_id = ANY($1)`, users},
		{`DELETE FROM atm_vendor_packages WHERE atm_id IN ` + atms, like},
		{`DELETE FROM atms WHERE terminal_id LIKE $1`, like},
		{`DELETE FROM vendor_packages_branch WHERE vendor_branch_id IN (SELECT id FROM vendor_branches WHERE vendor_id IN ` + vendors + `)`, like},
		{`DELETE FROM vendor_pics WHERE vendor_id IN ` + vendors, like},
		{`DELETE FROM vendor_vaults WHERE vendor_branch_id IN (SELECT id FROM vendor_branches WHERE vendor_id IN ` + vendors + `)`, like},
		{`DELETE FROM vendor_branches WHERE vendor_id IN ` + vendors, like},
		{`DELETE FROM vendors WHERE code LIKE $1`, like},
		{`DELETE FROM users WHERE id = ANY($1)`, users},
	}
	for _, s := range steps {
		if _, err := f.pool.Exec(ctx, s.sql, s.arg); err != nil {
			f.t.Errorf("cleanup %q: %v", s.sql, err)
		}
	}
}

// response is an HTTP answer with its JSON body decoded.
type response struct {
	status int
	body   map[string]any
	raw    string
}

func (f *invariantFixture) call(user int64, method, path string, body *bytes.Buffer, contentType string) response {
	f.t.Helper()
	if body == nil {
		body = &bytes.Buffer{}
	}
	req := httptest.NewRequest(method, path, body)
	req.Header.Set("X-Test-User", strconv.FormatInt(user, 10))
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}
	w := httptest.NewRecorder()
	f.router.ServeHTTP(w, req)
	out := response{status: w.Code, raw: w.Body.String()}
	_ = json.Unmarshal(w.Body.Bytes(), &out.body)
	return out
}

func (f *invariantFixture) upload(entity, action, file string) response {
	f.t.Helper()
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	part, err := mw.CreateFormFile("file", invariantFileName)
	if err != nil {
		f.t.Fatal(err)
	}
	if _, err := part.Write([]byte(file)); err != nil {
		f.t.Fatal(err)
	}
	if err := mw.Close(); err != nil {
		f.t.Fatal(err)
	}
	return f.call(f.maker, http.MethodPost, "/import/"+entity+"/"+action, &buf, mw.FormDataContentType())
}

func (f *invariantFixture) approve(user, requestID int64) response {
	f.t.Helper()
	return f.call(user, http.MethodPost, fmt.Sprintf("/approvals/%d/approve", requestID), nil, "")
}

func (f *invariantFixture) reject(user, requestID int64) response {
	f.t.Helper()
	return f.call(user, http.MethodPost, fmt.Sprintf("/approvals/%d/reject", requestID), nil, "")
}

func (f *invariantFixture) retry(requestID int64) response {
	f.t.Helper()
	return f.call(f.maker, http.MethodPost, fmt.Sprintf("/approvals/%d/retry-apply", requestID), nil, "")
}

func (f *invariantFixture) wantStatus(step string, got response, want int) {
	f.t.Helper()
	if got.status != want {
		f.t.Fatalf("%s: HTTP %d, want %d; body %s", step, got.status, want, got.raw)
	}
}

// confirm stages a file that must be new and returns its batch and approval.
func (f *invariantFixture) confirm(entity, file string) (batchID, requestID int64) {
	f.t.Helper()
	res := f.upload(entity, "confirm", file)
	f.wantStatus("confirm", res, http.StatusAccepted)
	return confirmIDs(f.t, res)
}

func confirmIDs(t *testing.T, res response) (batchID, requestID int64) {
	t.Helper()
	batch, okB := res.body["batch_id"].(float64)
	req, okR := res.body["approval_request_id"].(float64)
	if !okB || !okR {
		t.Fatalf("confirm response without batch or approval id: %s", res.raw)
	}
	return int64(batch), int64(req)
}

// csvFile renders rows under the entity's real export header.
func csvFile(t *testing.T, entity string, rows ...map[string]string) string {
	t.Helper()
	spec, ok := csvexport.Lookup(entity)
	if !ok {
		t.Fatalf("unknown entity %q", entity)
	}
	var buf bytes.Buffer
	w := csv.NewWriter(&buf)
	if err := w.Write(spec.Header); err != nil {
		t.Fatal(err)
	}
	for _, row := range rows {
		rec := make([]string, len(spec.Header))
		for i, col := range spec.Header {
			rec[i] = row[col]
		}
		if err := w.Write(rec); err != nil {
			t.Fatal(err)
		}
	}
	w.Flush()
	return buf.String()
}

func (f *invariantFixture) statuses(batchID int64) []string {
	f.t.Helper()
	rows, err := f.pool.Query(f.ctx, `SELECT status FROM master_data_change_requests WHERE batch_id = $1 ORDER BY id`, batchID)
	if err != nil {
		f.t.Fatal(err)
	}
	out, err := pgx.CollectRows(rows, pgx.RowTo[string])
	if err != nil {
		f.t.Fatal(err)
	}
	return out
}

func (f *invariantFixture) wantAllStatus(step string, batchID int64, want string, n int) {
	f.t.Helper()
	got := f.statuses(batchID)
	if len(got) != n {
		f.t.Errorf("%s: batch has %d rows, want %d", step, len(got), n)
	}
	for _, s := range got {
		if s != want {
			f.t.Errorf("%s: row statuses %v, want all %q", step, got, want)
			return
		}
	}
}

// snapshot captures every column of every row the fixture owns in each entity
// table. Comparing full rows (updated_at included) catches a touched row that
// count(*) would miss, and a staged create would show up under the tag.
func (f *invariantFixture) snapshot() map[string]string {
	f.t.Helper()
	vendors := `(SELECT id FROM vendors WHERE code LIKE $1)`
	tables := map[string]string{
		"vendors":             `t.code LIKE $1`,
		"vendor_branches":     `t.vendor_id IN ` + vendors,
		"vendor_vaults":       `t.vendor_branch_id IN (SELECT id FROM vendor_branches WHERE vendor_id IN ` + vendors + `)`,
		"vendor_pics":         `t.vendor_id IN ` + vendors,
		"atms":                `t.terminal_id LIKE $1`,
		"atm_vendor_packages": `t.atm_id IN (SELECT id FROM atms WHERE terminal_id LIKE $1)`,
	}
	out := make(map[string]string, len(tables))
	for table, where := range tables {
		var rows string
		q := `SELECT coalesce(json_agg(t ORDER BY t.id)::text, '') FROM ` + table + ` t WHERE ` + where
		if err := f.pool.QueryRow(f.ctx, q, f.tag+"%").Scan(&rows); err != nil {
			f.t.Fatalf("snapshot %s: %v", table, err)
		}
		out[table] = rows
	}
	return out
}

func (f *invariantFixture) wantEntities(step string, want map[string]string) {
	f.t.Helper()
	got := f.snapshot()
	for table := range want {
		if got[table] != want[table] {
			f.t.Errorf("%s: %s changed before final approval\nbefore: %s\nafter:  %s", step, table, want[table], got[table])
		}
	}
}

// bookkeeping counts the workflow rows scoped to the fixture's own users.
func (f *invariantFixture) bookkeeping() map[string]int {
	f.t.Helper()
	users := []int64{f.maker, f.approver1, f.approver2}
	return map[string]int{
		"batches":   f.count(`SELECT count(*) FROM master_data_import_batches WHERE maker_id = ANY($1)`, users),
		"changes":   f.count(`SELECT count(*) FROM master_data_change_requests WHERE maker_id = ANY($1)`, users),
		"approvals": f.count(`SELECT count(*) FROM approval_requests WHERE maker_id = ANY($1)`, users),
		"steps":     f.count(`SELECT count(*) FROM approval_steps WHERE assigned_approver_id = ANY($1)`, users),
		"audit":     f.count(`SELECT count(*) FROM audit_logs WHERE actor_id = ANY($1)`, users),
		"inbox":     f.count(`SELECT count(*) FROM notifications WHERE recipient_user_id = ANY($1)`, users),
	}
}

// invariantEntity is one importable entity: a file with at least one create and
// one update, and the SQL proving both landed after the final approval.
type invariantEntity struct {
	name                  string
	rows                  func(f *invariantFixture) []map[string]string
	wantCreate, wantOther int
	applied               func(f *invariantFixture) int // rows of the file visible in the table
}

func id(n int64) string { return strconv.FormatInt(n, 10) }

var invariantEntities = []invariantEntity{
	{
		name: service.ExportVendors, wantCreate: 1, wantOther: 1,
		rows: func(f *invariantFixture) []map[string]string {
			return []map[string]string{
				{"code": f.tag + "N", "name": "Baru"},
				{"id": id(f.vendorID), "code": f.vendorCode, "name": "Diubah", "is_active": "true"},
			}
		},
		applied: func(f *invariantFixture) int {
			return f.count(`SELECT count(*) FROM vendors WHERE (code = $1 AND name = 'Baru') OR (id = $2 AND name = 'Diubah')`, f.tag+"N", f.vendorID)
		},
	},
	{
		name: service.ExportVendorBranches, wantCreate: 1, wantOther: 1,
		rows: func(f *invariantFixture) []map[string]string {
			return []map[string]string{
				{"vendor_code": f.vendorCode, "branch_code": f.tag + "BN", "branch_name": "Baru", "category": "CASH"},
				{"id": id(f.branchID), "vendor_code": f.vendorCode, "branch_code": f.branchCode, "branch_name": "Diubah", "category": "ATM_CASH", "is_active": "true"},
			}
		},
		applied: func(f *invariantFixture) int {
			return f.count(`SELECT count(*) FROM vendor_branches WHERE vendor_id = $1 AND ((branch_code = $2 AND branch_name = 'Baru') OR (id = $3 AND branch_name = 'Diubah'))`, f.vendorID, f.tag+"BN", f.branchID)
		},
	},
	{
		name: service.ExportVendorVaults, wantCreate: 1, wantOther: 1,
		rows: func(f *invariantFixture) []map[string]string {
			return []map[string]string{
				{"vendor_code": f.vendorCode, "branch_code": f.branchCode, "vault_code": f.tag + "VN", "category": "ATM", "currency_code": "IDR"},
				{"id": id(f.vaultID), "vendor_code": f.vendorCode, "branch_code": f.branchCode, "vault_code": f.vaultCode, "category": "CASH", "currency_code": "IDR", "operating_hours": "08-17", "is_active": "true"},
			}
		},
		applied: func(f *invariantFixture) int {
			return f.count(`SELECT count(*) FROM vendor_vaults WHERE vendor_branch_id = $1 AND (vault_code = $2 OR (id = $3 AND operating_hours = '08-17'))`, f.branchID, f.tag+"VN", f.vaultID)
		},
	},
	{
		name: service.ExportVendorPICs, wantCreate: 1, wantOther: 1,
		rows: func(f *invariantFixture) []map[string]string {
			return []map[string]string{
				{"vendor_code": f.vendorCode, "name": "PIC Baru", "roles": "replenish"},
				{"id": id(f.picID), "vendor_code": f.vendorCode, "name": "PIC Diubah", "is_notification_recipient": "false", "is_active": "true", "roles": "cashcount"},
			}
		},
		applied: func(f *invariantFixture) int {
			return f.count(`SELECT count(*) FROM vendor_pics WHERE vendor_id = $1 AND ((name = 'PIC Baru' AND roles = '{replenish}') OR (id = $2 AND name = 'PIC Diubah' AND roles = '{cashcount}'))`, f.vendorID, f.picID)
		},
	},
	{
		name: service.ExportATMs, wantCreate: 1, wantOther: 1,
		rows: func(f *invariantFixture) []map[string]string {
			row := func(terminal, brand string) map[string]string {
				return map[string]string{"terminal_id": terminal, "location_id": id(f.location), "machine_type": "CRM", "brand": brand, "model": "SR7500",
					"operation_hours": "24_HOURS", "deployment_type": "OFFSITE", "blacklisted": "false", "priority_class": "Non VIP"}
			}
			updated := row(f.terminal, "Diubah")
			updated["id"], updated["is_active"] = id(f.atmID), "true"
			return []map[string]string{row(f.tag+"TN", "Baru"), updated}
		},
		applied: func(f *invariantFixture) int {
			return f.count(`SELECT count(*) FROM atms WHERE (terminal_id = $1 AND brand = 'Baru') OR (id = $2 AND brand = 'Diubah')`, f.tag+"TN", f.atmID)
		},
	},
	{
		name: service.ExportATMAssignments, wantCreate: 1, wantOther: 1,
		rows: func(f *invariantFixture) []map[string]string {
			row := func(terminal string) map[string]string {
				return map[string]string{"terminal_id": terminal, "vendor_code": f.vendorCode, "branch_code": f.branchCode, "package_source": "branch",
					"package_code": f.packageCode, "effective_start_date": f.assignmentStart}
			}
			updated := row(f.terminal)
			updated["id"], updated["is_active"], updated["effective_end_date"] = id(f.assignmentID), "true", "2099-12-31"
			return []map[string]string{row(f.terminal2), updated}
		},
		applied: func(f *invariantFixture) int {
			return f.count(`SELECT count(*) FROM atm_vendor_packages WHERE atm_id = $1 OR (id = $2 AND effective_end_date = '2099-12-31')`, f.atm2ID, f.assignmentID)
		},
	},
}

// Property 1: nothing a maker or a non-final approver does changes an entity
// table. Runs on all six entities through the real HTTP handlers and a real
// two-level chain. Ends with the control that gives the rest its meaning: the
// same file, fully approved, DOES change the snapshot.
func TestImportInvariant_P1_NoEntityChangeBeforeFinalApproval_Integration(t *testing.T) {
	for _, e := range invariantEntities {
		t.Run(e.name, func(t *testing.T) {
			f := newInvariantFixture(t, 2, nil)
			file := csvFile(t, e.name, e.rows(f)...)
			before := f.snapshot()
			books := f.bookkeeping()

			dry := f.upload(e.name, "dry-run", file)
			f.wantStatus("dry-run", dry, http.StatusOK)
			if creates, updates := dry.body["creates"], dry.body["updates"]; creates != float64(e.wantCreate) || updates != float64(e.wantOther) {
				t.Fatalf("dry-run preview = %s, want %d create(s) and %d update(s)", dry.raw, e.wantCreate, e.wantOther)
			}
			f.wantEntities("dry-run", before)
			if got := f.bookkeeping(); !reflect.DeepEqual(got, books) {
				t.Errorf("dry-run wrote workflow rows: %v -> %v", books, got)
			}

			batchID, requestID := f.confirm(e.name, file)
			n := e.wantCreate + e.wantOther
			if c := f.count(`SELECT count(*) FROM master_data_change_requests WHERE batch_id = $1 AND op = 'create'`, batchID); c != e.wantCreate {
				t.Errorf("staged creates = %d, want %d", c, e.wantCreate)
			}
			if u := f.count(`SELECT count(*) FROM master_data_change_requests WHERE batch_id = $1 AND op = 'update'`, batchID); u != e.wantOther {
				t.Errorf("staged updates = %d, want %d", u, e.wantOther)
			}
			f.wantEntities("confirm", before)
			f.wantAllStatus("confirm", batchID, "pending", n)
			books = f.bookkeeping()

			again := f.upload(e.name, "confirm", file)
			f.wantStatus("second confirm", again, http.StatusOK)
			if existing, _ := again.body["existing"].(bool); !existing {
				t.Errorf("second confirm = %s, want existing=true", again.raw)
			}
			if b, r := confirmIDs(t, again); b != batchID || r != requestID {
				t.Errorf("second confirm = batch %d approval %d, want %d and %d", b, r, batchID, requestID)
			}
			f.wantEntities("second confirm", before)
			if got := f.bookkeeping(); !reflect.DeepEqual(got, books) {
				t.Errorf("second confirm wrote workflow rows: %v -> %v", books, got)
			}

			// The maker, and the approver of a later step, may do nothing yet.
			f.wantStatus("approve by maker", f.approve(f.maker, requestID), http.StatusForbidden)
			f.wantStatus("reject by maker", f.reject(f.maker, requestID), http.StatusForbidden)
			f.wantStatus("approve out of turn", f.approve(f.approver2, requestID), http.StatusForbidden)
			f.wantStatus("retry-apply before approval", f.retry(requestID), http.StatusConflict)
			f.wantEntities("refused actions", before)
			f.wantAllStatus("refused actions", batchID, "pending", n)
			if got := f.bookkeeping(); !reflect.DeepEqual(got, books) {
				t.Errorf("refused actions wrote workflow rows: %v -> %v", books, got)
			}

			step := f.approve(f.approver1, requestID)
			f.wantStatus("non-final approve", step, http.StatusOK)
			if step.body["status"] != "pending" {
				t.Errorf("non-final approve = %s, want status pending", step.raw)
			}
			f.wantStatus("retry-apply after non-final step", f.retry(requestID), http.StatusConflict)
			f.wantEntities("non-final approve", before)
			f.wantAllStatus("non-final approve", batchID, "pending", n)

			rej := f.reject(f.approver2, requestID)
			f.wantStatus("reject", rej, http.StatusOK)
			if rej.body["status"] != "rejected" {
				t.Errorf("reject = %s, want status rejected", rej.raw)
			}
			f.wantStatus("retry-apply after reject", f.retry(requestID), http.StatusConflict)
			f.wantEntities("reject", before)
			f.wantAllStatus("reject", batchID, "rejected", n)

			// Control: approved through its last step, the same file does land.
			batch2, request2 := f.confirm(e.name, file)
			if batch2 == batchID {
				t.Fatalf("file re-uploaded after rejection reused batch %d", batchID)
			}
			f.wantStatus("control step 1", f.approve(f.approver1, request2), http.StatusOK)
			f.wantEntities("control step 1", before)
			f.wantStatus("control final", f.approve(f.approver2, request2), http.StatusOK)
			f.wantAllStatus("control final", batch2, "applied", n)
			if got := e.applied(f); got != n {
				t.Errorf("after final approval %d of %d rows are in the table", got, n)
			}
			if reflect.DeepEqual(f.snapshot(), before) {
				t.Errorf("snapshot did not change after final approval: it cannot detect a change")
			}
		})
	}
}

// Property 2: a batch applies all its rows or none. Each position of the stale
// row is a separate case because the rows before it are the ones at risk of
// staying behind. Rows that fail on an SQL error rather than staleness are
// covered by TestMasterDataImportConfirm_OneBadRowRollsBackTheWholeBatch_Integration;
// the staleness here is real (the target row moved), not a doctored "before".
func TestImportInvariant_P2_AllOrNothing_Integration(t *testing.T) {
	batchFile := func(f *invariantFixture) (string, []int64) {
		ids := []int64{f.seedVendor(f.tag+"S0", "Satu"), f.seedVendor(f.tag+"S1", "Dua"), f.seedVendor(f.tag+"S2", "Tiga")}
		rows := []map[string]string{{"code": f.tag + "NEW", "name": "Baru"}}
		for i, vid := range ids {
			rows = append(rows, map[string]string{"id": id(vid), "code": fmt.Sprintf("%sS%d", f.tag, i), "name": fmt.Sprintf("Diubah %d", i), "is_active": "true"})
		}
		return csvFile(f.t, service.ExportVendors, rows...), ids
	}

	for _, staleAt := range []int{0, 1, 2} {
		t.Run(fmt.Sprintf("stale_update_%d", staleAt), func(t *testing.T) {
			f := newInvariantFixture(t, 0, nil)
			file, ids := batchFile(f)
			batchID, requestID := f.confirm(service.ExportVendors, file)

			f.exec(`UPDATE vendors SET name = 'Diubah di luar' WHERE id = $1`, ids[staleAt])
			before := f.snapshot()

			res := f.approve(f.approver1, requestID)

			f.wantStatus("approve stale batch", res, http.StatusConflict)
			f.wantEntities("stale batch", before)
			f.wantAllStatus("stale batch", batchID, "stale", 4)
			if n := f.count(`SELECT count(*) FROM audit_logs WHERE actor_id = $1 AND action = 'master_data_applied'`, f.approver1); n != 0 {
				t.Errorf("%d master_data_applied audit rows after a rolled-back batch", n)
			}
			if n := f.count(`SELECT count(*) FROM approval_requests WHERE id = $1 AND status = 'approved'`, requestID); n != 1 {
				t.Errorf("the approval decision itself must stay recorded as approved")
			}

			// Row 0 of the batch is the create, so update k is change row k+1.
			rows, err := f.pool.Query(f.ctx, `SELECT id, coalesce(error, '') FROM master_data_change_requests WHERE batch_id = $1 ORDER BY id`, batchID)
			if err != nil {
				t.Fatal(err)
			}
			type row struct {
				ID  int64
				Err string
			}
			got, err := pgx.CollectRows(rows, pgx.RowToStructByPos[row])
			if err != nil {
				t.Fatal(err)
			}
			cause := got[staleAt+1]
			if cause.Err != service.ErrMasterDataChangeStale.Error() {
				t.Errorf("cause row error = %q, want the stale message without the rollback prefix", cause.Err)
			}
			rollback := fmt.Sprintf("%schange request %d failed", service.BatchRollbackPrefix, cause.ID)
			for _, r := range got {
				if r.ID != cause.ID && r.Err != rollback {
					t.Errorf("row %d error = %q, want %q", r.ID, r.Err, rollback)
				}
			}

			// The rows are free again: a corrected file for the moved vendor stages as a new batch.
			again, _ := f.confirm(service.ExportVendors, csvFile(t, service.ExportVendors, map[string]string{"id": id(ids[staleAt]), "code": fmt.Sprintf("%sS%d", f.tag, staleAt), "name": "Diperbaiki", "is_active": "true"}))
			if again == batchID {
				t.Errorf("a new file reused the stale batch")
			}
		})
	}

	t.Run("clean_batch", func(t *testing.T) {
		f := newInvariantFixture(t, 0, nil)
		file, ids := batchFile(f)
		batchID, requestID := f.confirm(service.ExportVendors, file)

		f.wantStatus("approve clean batch", f.approve(f.approver1, requestID), http.StatusOK)

		f.wantAllStatus("clean batch", batchID, "applied", 4)
		if n := f.count(`SELECT count(*) FROM master_data_change_requests WHERE batch_id = $1 AND error IS NOT NULL`, batchID); n != 0 {
			t.Errorf("%d rows of a clean batch carry an error", n)
		}
		if n := f.count(`SELECT count(*) FROM vendors WHERE code = $1 AND name = 'Baru'`, f.tag+"NEW"); n != 1 {
			t.Errorf("the created vendor is missing")
		}
		for i, vid := range ids {
			if n := f.count(`SELECT count(*) FROM vendors WHERE id = $1 AND name = $2`, vid, fmt.Sprintf("Diubah %d", i)); n != 1 {
				t.Errorf("vendor %d was not updated", i)
			}
		}
		if n := f.count(`SELECT count(*) FROM audit_logs WHERE actor_id = $1 AND action = 'master_data_applied'`, f.approver1); n != 4 {
			t.Errorf("%d master_data_applied audit rows, want 4", n)
		}
	})
}

// Property 3: concurrent confirms of one file end in exactly one batch, one
// approval, one submit audit and one set of notifications, and every caller gets
// a usable answer. CGO is off here, so -race is unavailable; the store spy
// proves the race window was really hit (two callers attempted the INSERT).
func TestImportInvariant_P3_ConcurrentConfirm_Integration(t *testing.T) {
	const iterations, callers = 15, 8
	f := newInvariantFixture(t, 0, nil)

	raced, failedCalls := 0, 0
	for i := range iterations {
		file := csvFile(t, service.ExportVendors, map[string]string{"code": fmt.Sprintf("%sC%d", f.tag, i), "name": "Bersamaan"})
		sum := sha256.Sum256([]byte(file))
		hash := hex.EncodeToString(sum[:])

		results := make([]response, callers)
		start := make(chan struct{})
		var wg sync.WaitGroup
		for c := range callers {
			wg.Add(1)
			go func() {
				defer wg.Done()
				<-start
				results[c] = f.upload(service.ExportVendors, "confirm", file)
			}()
		}
		close(start)
		wg.Wait()

		if n := f.count(`SELECT count(*) FROM master_data_import_batches WHERE file_hash = $1`, hash); n != 1 {
			t.Fatalf("iteration %d: %d batches for one file", i, n)
		}
		batchID := f.scanID(`SELECT id FROM master_data_import_batches WHERE file_hash = $1`, hash)
		headID := f.scanID(`SELECT min(id) FROM master_data_change_requests WHERE batch_id = $1`, batchID)
		if n := f.count(`SELECT count(*) FROM approval_requests WHERE document_type = 'master_data' AND document_id = $1`, headID); n != 1 {
			t.Fatalf("iteration %d: %d approval requests", i, n)
		}
		requestID := f.scanID(`SELECT id FROM approval_requests WHERE document_type = 'master_data' AND document_id = $1`, headID)
		if n := f.count(`SELECT count(*) FROM approval_steps WHERE request_id = $1`, requestID); n != 1 {
			t.Errorf("iteration %d: %d approval steps, want 1", i, n)
		}
		if n := f.count(`SELECT count(*) FROM master_data_change_requests WHERE batch_id = $1 AND approval_request_id = $2`, batchID, requestID); n != 1 {
			t.Errorf("iteration %d: the batch row is not linked to its approval", i)
		}
		if n := f.count(`SELECT count(*) FROM audit_logs WHERE action = 'submit_batch' AND entity_type = 'master_data_import_batch' AND entity_id = $1`, batchID); n != 1 {
			t.Errorf("iteration %d: %d submit_batch audit rows", i, n)
		}
		if n := f.count(`SELECT count(*) FROM audit_logs WHERE action = 'submit' AND entity_type = 'approval_request' AND entity_id = $1`, requestID); n != 1 {
			t.Errorf("iteration %d: %d orchestrator submit audit rows", i, n)
		}
		if n := f.count(`SELECT count(*) FROM notifications WHERE type = 'master_data_import.submitted' AND entity_id = $1 AND recipient_user_id = $2`, batchID, f.approver1); n != 1 {
			t.Errorf("iteration %d: %d submitted notifications", i, n)
		}
		if n := f.count(`SELECT count(*) FROM notification_emails e JOIN notifications n ON n.id = e.notification_id WHERE n.type = 'master_data_import.submitted' AND n.entity_id = $1`, batchID); n != 1 {
			t.Errorf("iteration %d: %d submitted emails", i, n)
		}

		f.store.mu.Lock()
		attempts := f.store.createAttempts[headID]
		f.store.mu.Unlock()
		if attempts > 1 {
			raced++
		}

		for _, res := range results {
			if res.status != http.StatusAccepted && res.status != http.StatusOK {
				failedCalls++
				t.Logf("iteration %d: caller answered HTTP %d: %s", i, res.status, res.raw)
				continue
			}
			if b, r := confirmIDs(t, res); b != batchID || r != requestID {
				t.Errorf("iteration %d: caller told batch %d approval %d, want %d and %d", i, b, r, batchID, requestID)
			}
		}
	}

	t.Logf("%d of %d iterations hit the SubmitForApproval race; %d of %d calls failed", raced, iterations, failedCalls, iterations*callers)
	if raced == 0 {
		t.Fatalf("no iteration raced: this run proved nothing about concurrency")
	}
	if failedCalls > 0 {
		t.Errorf("%d of %d concurrent confirms got an error although their file was staged and submitted", failedCalls, iterations*callers)
	}
}

// brokenConn makes the real notifier's queries fail. Reads pass through when
// failReads is false, so a run also covers a failure after the batch was read.
type brokenConn struct {
	inner     db.DBTX
	failReads bool
}

var errBrokenConn = errors.New("notification database is down")

type errorRow struct{}

func (errorRow) Scan(...any) error { return errBrokenConn }

func (c brokenConn) Exec(context.Context, string, ...any) (pgconn.CommandTag, error) {
	return pgconn.CommandTag{}, errBrokenConn
}

func (c brokenConn) Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
	if c.failReads {
		return nil, errBrokenConn
	}
	return c.inner.Query(ctx, sql, args...)
}

func (c brokenConn) QueryRow(ctx context.Context, sql string, args ...any) pgx.Row {
	if c.failReads || strings.Contains(sql, "INSERT INTO") {
		return errorRow{}
	}
	return c.inner.QueryRow(ctx, sql, args...)
}

// notifyOutcome is everything Property 9 says must not depend on notifications,
// free of ids and of the fixture tag so that two runs can be compared.
type notifyOutcome struct {
	HTTP     []string
	Batches  []string
	Requests []string
	Audit    []string
	Vendors  []string
}

// notifyBatches names the four batches of runNotifyScenario.
type notifyBatches struct{ applied, rejected, stale, retried int64 }

// runNotifyScenario walks every call site of ImportNotifier: submit, next step,
// apply success and failure, reject and retry-apply.
func runNotifyScenario(f *invariantFixture) (notifyOutcome, notifyBatches) {
	f.t.Helper()
	var out notifyOutcome
	var b notifyBatches
	record := func(step string, res response) {
		out.HTTP = append(out.HTTP, fmt.Sprintf("%s=%d", step, res.status))
	}
	confirm := func(step string, row map[string]string) (int64, int64) {
		res := f.upload(service.ExportVendors, "confirm", csvFile(f.t, service.ExportVendors, row))
		record(step, res)
		f.wantStatus(step, res, http.StatusAccepted)
		return confirmIDs(f.t, res)
	}
	var requests []int64

	var reqA int64
	b.applied, reqA = confirm("confirm applied", map[string]string{"id": id(f.vendorID), "code": f.vendorCode, "name": "Diterapkan", "is_active": "true"})
	record("step 1", f.approve(f.approver1, reqA))
	record("step 2", f.approve(f.approver2, reqA))
	requests = append(requests, reqA)

	var reqB int64
	b.rejected, reqB = confirm("confirm rejected", map[string]string{"code": f.tag + "RJ", "name": "Ditolak"})
	record("reject", f.reject(f.approver1, reqB))
	requests = append(requests, reqB)

	staleID := f.seedVendor(f.tag+"ST", "Stale")
	var reqC int64
	b.stale, reqC = confirm("confirm stale", map[string]string{"id": id(staleID), "code": f.tag + "ST", "name": "Tidak jadi", "is_active": "true"})
	f.exec(`UPDATE vendors SET name = 'Diubah di luar' WHERE id = $1`, staleID)
	record("stale step 1", f.approve(f.approver1, reqC))
	record("stale step 2", f.approve(f.approver2, reqC))
	requests = append(requests, reqC)

	// The state a failed commit leaves behind (it cannot be forced from outside
	// a real tx): approved everywhere, error recorded, nothing applied.
	var reqD int64
	b.retried, reqD = confirm("confirm retried", map[string]string{"code": f.tag + "RT", "name": "Dicoba lagi"})
	f.exec(`UPDATE approval_requests SET status = 'approved' WHERE id = $1`, reqD)
	f.exec(`UPDATE approval_steps SET status = 'approved', acted_by_id = assigned_approver_id, acted_at = now() WHERE request_id = $1`, reqD)
	f.exec(`UPDATE master_data_change_requests SET status = 'approved', error = 'commit failed' WHERE batch_id = $1`, b.retried)
	record("retry", f.retry(reqD))
	requests = append(requests, reqD)

	for _, batch := range []int64{b.applied, b.rejected, b.stale, b.retried} {
		out.Batches = append(out.Batches, strings.Join(f.statuses(batch), ","))
	}
	for _, r := range requests {
		var status string
		if err := f.pool.QueryRow(f.ctx, `SELECT status FROM approval_requests WHERE id = $1`, r).Scan(&status); err != nil {
			f.t.Fatal(err)
		}
		out.Requests = append(out.Requests, status)
	}

	alias := map[int64]string{f.maker: "maker", f.approver1: "a1", f.approver2: "a2"}
	rows, err := f.pool.Query(f.ctx, `SELECT actor_id, action, entity_type FROM audit_logs WHERE actor_id = ANY($1)`, []int64{f.maker, f.approver1, f.approver2})
	if err != nil {
		f.t.Fatal(err)
	}
	type auditRow struct {
		Actor        int64
		Action, Type string
	}
	audits, err := pgx.CollectRows(rows, pgx.RowToStructByPos[auditRow])
	if err != nil {
		f.t.Fatal(err)
	}
	for _, a := range audits {
		out.Audit = append(out.Audit, alias[a.Actor]+"/"+a.Action+"/"+a.Type)
	}
	sort.Strings(out.Audit)

	vrows, err := f.pool.Query(f.ctx, `SELECT replace(code, $1, '') || '=' || name || '/' || is_active::text FROM vendors WHERE code LIKE $1 || '%' ORDER BY code`, f.tag)
	if err != nil {
		f.t.Fatal(err)
	}
	if out.Vendors, err = pgx.CollectRows(vrows, pgx.RowTo[string]); err != nil {
		f.t.Fatal(err)
	}
	return out, b
}

// Property 9: a notifier that always fails changes nothing but the notifications.
// The healthy run doubles as the check of what the real notifier writes.
func TestImportInvariant_P9_NotificationFailureChangesNothing_Integration(t *testing.T) {
	healthy := newInvariantFixture(t, 2, nil)
	want, batches := runNotifyScenario(healthy)

	wantHTTP := []string{"confirm applied=202", "step 1=200", "step 2=200", "confirm rejected=202", "reject=200",
		"confirm stale=202", "stale step 1=200", "stale step 2=409", "confirm retried=202", "retry=200"}
	if !reflect.DeepEqual(want.HTTP, wantHTTP) {
		t.Errorf("responses = %v, want %v", want.HTTP, wantHTTP)
	}
	if wb := []string{"applied", "rejected", "stale", "applied"}; !reflect.DeepEqual(want.Batches, wb) {
		t.Errorf("batch statuses = %v, want %v", want.Batches, wb)
	}
	assertImportNotifications(t, healthy, batches)

	for name, conn := range map[string]func(*pgxpool.Pool) db.DBTX{
		"every_query_fails": func(p *pgxpool.Pool) db.DBTX { return brokenConn{inner: p, failReads: true} },
		"only_writes_fail":  func(p *pgxpool.Pool) db.DBTX { return brokenConn{inner: p} },
	} {
		t.Run(name, func(t *testing.T) {
			f := newInvariantFixture(t, 2, conn)

			got, _ := runNotifyScenario(f)

			if !reflect.DeepEqual(got, want) {
				t.Errorf("outcome differs from the healthy run\n got: %+v\nwant: %+v", got, want)
			}
			users := []int64{f.maker, f.approver1, f.approver2}
			if n := f.count(`SELECT count(*) FROM notifications WHERE recipient_user_id = ANY($1)`, users); n != 0 {
				t.Errorf("%d notifications written through a broken connection: the failure was not exercised", n)
			}
		})
	}
}

// assertImportNotifications compares the rows of the healthy run with the design
// table: approvers get in-app + email linking to /approvals, the maker in-app only
// linking to the batch.
func assertImportNotifications(t *testing.T, f *invariantFixture, b notifyBatches) {
	t.Helper()
	type want struct {
		recipient int64
		typ       string
		batch     int64
		link      string
		email     bool
	}
	history := func(batch int64) string {
		return fmt.Sprintf("/settings/admin/master-data-io?tab=history&batch=%d", batch)
	}
	const submitted, applied, rejected, failed = "master_data_import.submitted", "master_data_import.applied", "master_data_import.rejected", "master_data_import.apply_failed"
	wants := []want{
		{f.approver1, submitted, b.applied, "/approvals", true},
		{f.approver2, submitted, b.applied, "/approvals", true},
		{f.maker, applied, b.applied, history(b.applied), false},
		{f.approver1, submitted, b.rejected, "/approvals", true},
		{f.maker, rejected, b.rejected, history(b.rejected), false},
		{f.approver1, submitted, b.stale, "/approvals", true},
		{f.approver2, submitted, b.stale, "/approvals", true},
		{f.maker, failed, b.stale, history(b.stale), false},
		{f.approver1, submitted, b.retried, "/approvals", true},
		{f.maker, applied, b.retried, history(b.retried), false},
	}

	rows, err := f.pool.Query(f.ctx, `SELECT n.recipient_user_id, n.type, n.entity_type, n.entity_id, n.link, n.body, n.title,
			coalesce(e.to_address, ''), coalesce(e.subject, ''), coalesce(e.status, ''), u.email
		FROM notifications n JOIN users u ON u.id = n.recipient_user_id
		LEFT JOIN notification_emails e ON e.notification_id = n.id
		WHERE n.recipient_user_id = ANY($1) ORDER BY n.id`, []int64{f.maker, f.approver1, f.approver2})
	if err != nil {
		t.Fatal(err)
	}
	type got struct {
		Recipient                          int64
		Type, EntityType                   string
		EntityID                           int64
		Link, Body, Title, To, Subject, St string
		UserEmail                          string
	}
	gots, err := pgx.CollectRows(rows, pgx.RowToStructByPos[got])
	if err != nil {
		t.Fatal(err)
	}
	if len(gots) != len(wants) {
		t.Fatalf("%d notifications, want %d: %+v", len(gots), len(wants), gots)
	}
	for i, w := range wants {
		g := gots[i]
		if g.Recipient != w.recipient || g.Type != w.typ || g.EntityID != w.batch || g.EntityType != "master_data_import_batch" || g.Link != w.link {
			t.Errorf("notification %d = %+v, want %+v", i, g, w)
		}
		if body := fmt.Sprintf("batch #%d (file %s), 1 baris", w.batch, invariantFileName); !strings.Contains(g.Body, body) || !strings.Contains(g.Body, "Vendor") {
			t.Errorf("notification %d body = %q, want the entity and %q", i, g.Body, body)
		}
		if w.email && (g.To != strings.ToLower(g.UserEmail) || g.Subject != g.Title || g.St != "skipped") {
			t.Errorf("notification %d email = to %q subject %q status %q, want the approver's address, the title and status skipped", i, g.To, g.Subject, g.St)
		}
		if !w.email && g.To != "" {
			t.Errorf("notification %d went by email to %q; the maker gets in-app only", i, g.To)
		}
	}
	// A submit audit must name the file, also when the notifier is healthy.
	if n := f.count(`SELECT count(*) FROM audit_logs WHERE action = 'submit_batch' AND entity_id = $1 AND after->>'file_name' = $2`, b.applied, invariantFileName); n != 1 {
		t.Errorf("submit_batch audit does not name the file")
	}
}
