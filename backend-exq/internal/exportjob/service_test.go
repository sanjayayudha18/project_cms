package exportjob

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/cimb-niaga/cms/backend-exq/internal/audit"
	"github.com/cimb-niaga/cms/backend-exq/internal/repository"
)

// fakeDB is an in-memory primary: jobs written inside a tx become visible only on commit.
type fakeDB struct {
	vendors  map[int64]string // vendor id -> code
	branches map[int64][2]any // branch id -> {vendor id, code}
	jobs     []repository.ExportJob
	pending  []repository.ExportJob
	calls    int // every DB call, including tx begin
	locked   []int64
	commits  int
	rollback int
	failFind error
}

func (f *fakeDB) GetVendorCodeByID(_ context.Context, id int64) (string, error) {
	f.calls++
	if c, ok := f.vendors[id]; ok {
		return c, nil
	}
	return "", pgx.ErrNoRows
}

func (f *fakeDB) GetVendorBranchForVendor(_ context.Context, a repository.GetVendorBranchForVendorParams) (string, error) {
	f.calls++
	if b, ok := f.branches[a.BranchID]; ok && b[0].(int64) == a.VendorID {
		return b[1].(string), nil
	}
	return "", pgx.ErrNoRows
}

func (f *fakeDB) LockExportJobsForUser(_ context.Context, id int64) error {
	f.calls++
	f.locked = append(f.locked, id)
	return nil
}

func (f *fakeDB) FindActiveExportJobByHash(_ context.Context, a repository.FindActiveExportJobByHashParams) (repository.ExportJob, error) {
	f.calls++
	if f.failFind != nil {
		return repository.ExportJob{}, f.failFind
	}
	for _, j := range f.jobs {
		if j.RequestedBy == a.RequestedBy && j.ParamsHash == a.ParamsHash && (j.Status == "queued" || j.Status == "running") {
			return j, nil
		}
	}
	return repository.ExportJob{}, pgx.ErrNoRows
}

func (f *fakeDB) CountActiveExportJobs(_ context.Context, uid int64) (int64, error) {
	f.calls++
	var n int64
	for _, j := range f.jobs {
		if j.RequestedBy == uid && (j.Status == "queued" || j.Status == "running") {
			n++
		}
	}
	return n, nil
}

func (f *fakeDB) InsertExportJob(_ context.Context, a repository.InsertExportJobParams) (repository.ExportJob, error) {
	f.calls++
	j := repository.ExportJob{
		ID: int64(len(f.jobs) + len(f.pending) + 100), RequestedBy: a.RequestedBy, Kind: a.Kind, Entity: a.Entity,
		VendorID: a.VendorID, VendorCode: a.VendorCode, VendorBranchID: a.VendorBranchID, BranchCode: a.BranchCode,
		StatusFilter: a.StatusFilter, PriceLevel: a.PriceLevel, ParamsHash: a.ParamsHash, FileName: a.FileName,
		Status: "queued",
	}
	f.pending = append(f.pending, j)
	return j, nil
}

func (f *fakeDB) inTx(ctx context.Context, fn func(repository.DBTX, createQueries) error) error {
	f.calls++
	if err := fn(nil, f); err != nil {
		f.pending = nil
		f.rollback++
		return err
	}
	f.jobs = append(f.jobs, f.pending...)
	f.pending = nil
	f.commits++
	return nil
}

type fakeAudit struct {
	entries []audit.Entry
	err     error
}

func (a *fakeAudit) Write(_ context.Context, _ repository.DBTX, e audit.Entry) error {
	if a.err != nil {
		return a.err
	}
	a.entries = append(a.entries, e)
	return nil
}

var fixedNow = time.Date(2026, 10, 28, 20, 0, 0, 0, time.UTC) // 29 Oct 03:00 WIB

type harness struct {
	db    *fakeDB
	audit *fakeAudit
	wakes int
	svc   *Service
}

func newHarness() *harness {
	h := &harness{
		db: &fakeDB{
			vendors:  map[int64]string{7: "ABA", 8: "OLD"}, // 8 = inactive vendor, still resolves
			branches: map[int64][2]any{31: {int64(7), "MDN"}, 41: {int64(8), "SBY"}},
		},
		audit: &fakeAudit{},
	}
	h.svc = &Service{q: h.db, inTx: h.db.inTx, audit: h.audit, wake: func() { h.wakes++ }, now: func() time.Time { return fixedNow }}
	return h
}

func ptr64(v int64) *int64 { return &v }

var admin = Actor{UserID: 5, Username: "admin.param", Role: "ADMIN_PARAM", IP: "10.0.0.9"}

func mustParams(t *testing.T, k Kind, entity string, v, b *int64, status, pl string) Params {
	t.Helper()
	p, err := ParseParams(k, entity, v, b, status, pl)
	if err != nil {
		t.Fatalf("ParseParams: %v", err)
	}
	return p
}

func TestCreateUnauthenticatedBeforeAnyDBCall(t *testing.T) {
	h := newHarness()
	p := mustParams(t, KindVendor, "pics", ptr64(7), nil, "", "")
	_, _, err := h.svc.Create(context.Background(), Actor{Role: "ADMIN"}, p)
	if !errors.Is(err, ErrUnauthenticated) {
		t.Fatalf("err = %v, want ErrUnauthenticated", err)
	}
	if h.db.calls != 0 || len(h.audit.entries) != 0 || h.wakes != 0 {
		t.Fatalf("calls=%d audit=%d wakes=%d, want 0", h.db.calls, len(h.audit.entries), h.wakes)
	}
}

func TestCreateRoles(t *testing.T) {
	kinds := []Params{
		mustParams(t, KindVendor, "vendor", ptr64(7), nil, "", ""),
		mustParams(t, KindCabang, "pics", ptr64(7), ptr64(31), "", ""),
		mustParams(t, KindHargaPaket, "package-prices", ptr64(7), nil, "", ""),
		mustParams(t, KindMasterDataGlobal, "vendors", nil, nil, "", ""),
	}
	roles := map[string]bool{"ADMIN": true, "ADMIN_PARAM": true, "APPACCESS": false, "ATM-SPV": false, "VENDOR-USER": false, "": false, "admin": false}
	for _, p := range kinds {
		for role, ok := range roles {
			t.Run(string(p.Kind)+"/"+role, func(t *testing.T) {
				h := newHarness()
				a := admin
				a.Role = role
				_, created, err := h.svc.Create(context.Background(), a, p)
				if ok {
					if err != nil || !created {
						t.Fatalf("created=%v err=%v, want created", created, err)
					}
					return
				}
				if !errors.Is(err, ErrForbidden) {
					t.Fatalf("err = %v, want ErrForbidden", err)
				}
				if h.db.calls != 0 || len(h.audit.entries) != 0 {
					t.Fatalf("calls=%d audit=%d, want 0", h.db.calls, len(h.audit.entries))
				}
			})
		}
	}
}

func TestCreateUnknownKindForbidden(t *testing.T) {
	h := newHarness()
	_, _, err := h.svc.Create(context.Background(), admin, Params{Kind: "report", Entity: "x", StatusFilter: "active"})
	if !errors.Is(err, ErrForbidden) {
		t.Fatalf("err = %v, want ErrForbidden", err)
	}
}

func TestCreateScopeNotFoundSameErrorNoAudit(t *testing.T) {
	cases := map[string]Params{
		"missing vendor":           mustParams(t, KindVendor, "pics", ptr64(99), nil, "", ""),
		"missing branch":           mustParams(t, KindCabang, "pics", ptr64(7), ptr64(999), "", ""),
		"branch of another vendor": mustParams(t, KindCabang, "pics", ptr64(7), ptr64(41), "", ""),
		"price branch of other":    mustParams(t, KindHargaPaket, "package-prices", ptr64(7), ptr64(41), "", ""),
	}
	for name, p := range cases {
		t.Run(name, func(t *testing.T) {
			h := newHarness()
			_, _, err := h.svc.Create(context.Background(), admin, p)
			if !errors.Is(err, ErrNotFound) || err.Error() != ErrNotFound.Error() { // same error and message for every case
				t.Fatalf("err = %v, want ErrNotFound", err)
			}
			if len(h.db.jobs) != 0 || len(h.audit.entries) != 0 || h.wakes != 0 || len(h.db.locked) != 0 {
				t.Fatalf("side effects: jobs=%d audit=%d wakes=%d locks=%d", len(h.db.jobs), len(h.audit.entries), h.wakes, len(h.db.locked))
			}
		})
	}
}

func TestCreateInactiveVendorResolves(t *testing.T) {
	h := newHarness()
	j, created, err := h.svc.Create(context.Background(), admin, mustParams(t, KindCabang, "vaults", ptr64(8), ptr64(41), "all", ""))
	if err != nil || !created || j.VendorCode != "OLD" || j.BranchCode != "SBY" {
		t.Fatalf("job=%+v created=%v err=%v", j, created, err)
	}
}

func TestCreateDedupeReturnsExistingWithoutInsertOrAudit(t *testing.T) {
	h := newHarness()
	p := mustParams(t, KindVendor, "pics", ptr64(7), nil, "", "")
	first, created, err := h.svc.Create(context.Background(), admin, p)
	if err != nil || !created {
		t.Fatalf("first: created=%v err=%v", created, err)
	}
	again, created, err := h.svc.Create(context.Background(), admin, p)
	if err != nil || created || again.ID != first.ID {
		t.Fatalf("second: id=%d (want %d) created=%v err=%v", again.ID, first.ID, created, err)
	}
	if len(h.db.jobs) != 1 || len(h.audit.entries) != 1 || h.wakes != 1 {
		t.Fatalf("jobs=%d audit=%d wakes=%d, want 1/1/1", len(h.db.jobs), len(h.audit.entries), h.wakes)
	}

	// Same params, other user = separate job.
	other := admin
	other.UserID = 6
	if _, created, err := h.svc.Create(context.Background(), other, p); err != nil || !created {
		t.Fatalf("other user: created=%v err=%v", created, err)
	}

	// Finished job no longer dedupes.
	h.db.jobs[0].Status = "succeeded"
	if _, created, err := h.svc.Create(context.Background(), admin, p); err != nil || !created {
		t.Fatalf("after finish: created=%v err=%v", created, err)
	}
}

func TestCreateActiveLimit(t *testing.T) {
	h := newHarness()
	ents := []string{"vendor", "branches", "vaults"}
	for _, e := range ents {
		if _, created, err := h.svc.Create(context.Background(), admin, mustParams(t, KindVendor, e, ptr64(7), nil, "", "")); err != nil || !created {
			t.Fatalf("%s: created=%v err=%v", e, created, err)
		}
	}
	h.db.jobs[0].Status = "running" // running still counts
	_, _, err := h.svc.Create(context.Background(), admin, mustParams(t, KindVendor, "pics", ptr64(7), nil, "", ""))
	if !errors.Is(err, ErrTooManyActive) {
		t.Fatalf("4th: err = %v, want ErrTooManyActive", err)
	}
	if len(h.db.jobs) != 3 || len(h.audit.entries) != 3 || h.wakes != 3 {
		t.Fatalf("jobs=%d audit=%d wakes=%d, want 3/3/3", len(h.db.jobs), len(h.audit.entries), h.wakes)
	}

	// Dedupe wins over the limit: an identical request at 3 active returns the existing job.
	j, created, err := h.svc.Create(context.Background(), admin, mustParams(t, KindVendor, "vendor", ptr64(7), nil, "", ""))
	if err != nil || created || j.ID != h.db.jobs[0].ID {
		t.Fatalf("dedupe at limit: id=%d created=%v err=%v", j.ID, created, err)
	}

	// One finishes -> room for one more.
	h.db.jobs[1].Status = "failed"
	if _, created, err := h.svc.Create(context.Background(), admin, mustParams(t, KindVendor, "pics", ptr64(7), nil, "", "")); err != nil || !created {
		t.Fatalf("after one failed: created=%v err=%v", created, err)
	}
}

func TestCreateAuditFailureRollsBack(t *testing.T) {
	h := newHarness()
	h.audit.err = errors.New("audit_logs down")
	_, created, err := h.svc.Create(context.Background(), admin, mustParams(t, KindVendor, "pics", ptr64(7), nil, "", ""))
	if !errors.Is(err, ErrInternal) || created {
		t.Fatalf("created=%v err=%v, want ErrInternal", created, err)
	}
	if len(h.db.jobs) != 0 || h.db.rollback != 1 || h.db.commits != 0 || h.wakes != 0 {
		t.Fatalf("jobs=%d rollback=%d commits=%d wakes=%d", len(h.db.jobs), h.db.rollback, h.db.commits, h.wakes)
	}
}

func TestCreateDBErrorIsInternal(t *testing.T) {
	h := newHarness()
	h.db.failFind = errors.New("connection reset")
	_, _, err := h.svc.Create(context.Background(), admin, mustParams(t, KindVendor, "pics", ptr64(7), nil, "", ""))
	if !errors.Is(err, ErrInternal) || errors.Is(err, ErrNotFound) {
		t.Fatalf("err = %v, want ErrInternal only", err)
	}
	if h.wakes != 0 || len(h.audit.entries) != 0 {
		t.Fatalf("wakes=%d audit=%d", h.wakes, len(h.audit.entries))
	}
}

func TestCreateSuccessAuditEntry(t *testing.T) {
	cases := []struct {
		name       string
		p          Params
		entityType string
		entityID   int64
		after      string
		file       string
	}{
		{
			name: "cabang", p: mustParams(t, KindCabang, "pics", ptr64(7), ptr64(31), "", ""),
			entityType: "vendor_branch", entityID: 31,
			after: `{"branch_code":"MDN","branch_id":31,"export_entity":"pics","export_job_id":100,"export_kind":"cabang","price_level":null,"role":"ADMIN_PARAM","status_filter":"active","username":"admin.param","vendor_code":"ABA","vendor_id":7}`,
			file:  "vendor-ABA-cabang-MDN-pics-20261029.csv",
		},
		{
			name: "vendor", p: mustParams(t, KindVendor, "dsr-location-maps", ptr64(7), nil, "all", ""),
			entityType: "vendor", entityID: 7,
			after: `{"branch_code":null,"branch_id":null,"export_entity":"dsr-location-maps","export_job_id":100,"export_kind":"vendor","price_level":null,"role":"ADMIN_PARAM","status_filter":"all","username":"admin.param","vendor_code":"ABA","vendor_id":7}`,
			file:  "vendor-ABA-dsr-location-maps-20261029.csv",
		},
		{
			name: "harga paket vendor", p: mustParams(t, KindHargaPaket, "package-prices", ptr64(7), nil, "", ""),
			entityType: "vendor", entityID: 7,
			after: `{"branch_code":null,"branch_id":null,"export_entity":"package-prices","export_job_id":100,"export_kind":"harga_paket","price_level":"all","role":"ADMIN_PARAM","status_filter":"active","username":"admin.param","vendor_code":"ABA","vendor_id":7}`,
			file:  "harga-paket-ABA-20261029.csv",
		},
		{
			name: "harga paket cabang", p: mustParams(t, KindHargaPaket, "package-prices", ptr64(7), ptr64(31), "disabled", "atm"),
			entityType: "vendor_branch", entityID: 31,
			after: `{"branch_code":"MDN","branch_id":31,"export_entity":"package-prices","export_job_id":100,"export_kind":"harga_paket","price_level":"atm","role":"ADMIN_PARAM","status_filter":"disabled","username":"admin.param","vendor_code":"ABA","vendor_id":7}`,
			file:  "harga-paket-ABA-cabang-MDN-20261029.csv",
		},
		{
			name: "global", p: mustParams(t, KindMasterDataGlobal, "vendors", nil, nil, "", ""),
			entityType: "master_data", entityID: 0,
			after: `{"branch_code":null,"branch_id":null,"export_entity":"vendors","export_job_id":100,"export_kind":"master_data_global","price_level":null,"role":"ADMIN_PARAM","status_filter":"active","username":"admin.param","vendor_code":null,"vendor_id":null}`,
			file:  "master-data-vendors-20261029.csv",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			h := newHarness()
			j, created, err := h.svc.Create(context.Background(), admin, tc.p)
			if err != nil || !created {
				t.Fatalf("created=%v err=%v", created, err)
			}
			if h.wakes != 1 || h.db.commits != 1 || len(h.db.locked) != 1 || h.db.locked[0] != admin.UserID {
				t.Fatalf("wakes=%d commits=%d locked=%v", h.wakes, h.db.commits, h.db.locked)
			}
			if j.Status != StatusQueued || j.FileName != tc.file || j.RequestedBy != admin.UserID || j.Params.Kind != tc.p.Kind {
				t.Fatalf("job = %+v", j)
			}
			if len(h.audit.entries) != 1 {
				t.Fatalf("audit entries = %d", len(h.audit.entries))
			}
			e := h.audit.entries[0]
			if e.ActorID != admin.UserID || e.Action != "export_requested" || e.EntityType != tc.entityType || e.EntityID != tc.entityID || e.IP != admin.IP {
				t.Fatalf("entry = %+v", e)
			}
			got, err := json.Marshal(e.After)
			if err != nil {
				t.Fatal(err)
			}
			if string(got) != tc.after {
				t.Fatalf("after\n got %s\nwant %s", got, tc.after)
			}
		})
	}
}

func TestCreateNilWake(t *testing.T) {
	h := newHarness()
	h.svc.wake = nil
	if _, created, err := h.svc.Create(context.Background(), admin, mustParams(t, KindVendor, "pics", ptr64(7), nil, "", "")); err != nil || !created {
		t.Fatalf("created=%v err=%v", created, err)
	}
}

func TestKindSpecs(t *testing.T) {
	for _, k := range []Kind{KindVendor, KindCabang, KindHargaPaket, KindMasterDataGlobal} {
		s, ok := KindSpecs[k]
		if !ok || s.DownloadAction != "master_data_exported" || s.EntityRef == nil {
			t.Fatalf("%s: spec %+v", k, s)
		}
		if len(KindRoles[k]) != 2 || !allowed(k, "ADMIN") || !allowed(k, "ADMIN_PARAM") || allowed(k, "APPACCESS") {
			t.Fatalf("%s: roles %v", k, KindRoles[k])
		}
	}
	// KindRoles is a copy: mutating it must not widen KindSpecs.
	KindRoles[KindVendor][0] = "X"
	defer func() { KindRoles = rolesOf(KindSpecs) }()
	if KindSpecs[KindVendor].Roles[0] != "ADMIN" {
		t.Fatal("KindRoles aliases KindSpecs.Roles")
	}
}

func TestJobFromRowTimestampsAndCounts(t *testing.T) {
	r := repository.ExportJob{ID: 1, Kind: "vendor", Status: "succeeded", ChunkCount: func() *int32 { c := int32(3); return &c }()}
	r.CreatedAt.Time, r.CreatedAt.Valid = fixedNow, true
	r.ExpiresAt.Time, r.ExpiresAt.Valid = fixedNow.Add(Retention), true
	j := jobFromRow(r)
	if j.ChunkCount == nil || *j.ChunkCount != 3 || j.StartedAt != nil || j.ExpiresAt == nil || !j.ExpiresAt.Equal(fixedNow.Add(Retention)) || !j.CreatedAt.Equal(fixedNow) {
		t.Fatalf("job = %+v", j)
	}
}
