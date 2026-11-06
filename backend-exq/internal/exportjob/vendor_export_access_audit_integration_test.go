//go:build integration

package exportjob

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/cimb-niaga/cms/backend-exq/internal/repository"
)

// Task 3.5 (vendor-export-pks, Req 3.2, 4.1-4.4) on real Postgres: access, audit and
// replica wiring of the vendor master-data export are unchanged by the PKS columns.
// Both kinds that export vendor rows are covered: master_data_global "vendors" and
// the per-vendor kind "vendor".

func TestVendorExportAccessAndAudit(t *testing.T) {
	e := s11NewEnv(t)
	ctx := context.Background()
	prev := &Previewer{
		scope: repository.New(e.pool), reader: pgReader{pool: e.pool},
		producers: DefaultProducers(), now: time.Now,
	}
	kinds := map[string]Params{
		"global-vendors": e.params(KindMasterDataGlobal, "vendors", nil, nil, "", ""),
		"per-vendor":     e.params(KindVendor, "vendor", s11Ptr(e.vendorID), nil, "", ""),
	}

	// Roles other than ADMIN / ADMIN_PARAM: 403 on create and preview, no job, no audit.
	denied := e.user("denied", "ATM-SPV")
	for name, p := range kinds {
		for _, role := range []string{"ATM-SPV", "BRANCH-ATM-SPV", "VENDOR-USER", "ATM-USER"} {
			a := Actor{UserID: denied.UserID, Username: denied.Username, Role: role, IP: s11IP}
			if _, _, err := e.svc.Create(ctx, a, p); !errors.Is(err, ErrForbidden) {
				t.Errorf("%s: Create as %s err = %v, want ErrForbidden", name, role, err)
			}
			if _, err := prev.Preview(ctx, a, p, 0, 5); !errors.Is(err, ErrForbidden) {
				t.Errorf("%s: Preview as %s err = %v, want ErrForbidden", name, role, err)
			}
		}
	}
	if e.jobCount(denied.UserID) != 0 || e.n(`SELECT count(*) FROM audit_logs WHERE actor_id = $1`, denied.UserID) != 0 {
		t.Error("refused requests left a job or an audit row")
	}

	// Allowed roles: export_requested at create, master_data_exported (+row_count) at download, in that order.
	for _, role := range []string{"ADMIN", "ADMIN_PARAM"} {
		for name, p := range kinds {
			a := e.user(role+"-"+name, role)

			if _, err := prev.Preview(ctx, a, p, 0, 5); err != nil {
				t.Fatalf("%s/%s: Preview: %v", role, name, err)
			}
			if n := e.n(`SELECT count(*) FROM audit_logs WHERE actor_id = $1`, a.UserID); n != 0 {
				t.Fatalf("%s/%s: preview wrote %d audit rows, want 0", role, name, n)
			}

			job, created, err := e.svc.Create(ctx, a, p)
			if err != nil || !created {
				t.Fatalf("%s/%s: Create created=%v err=%v", role, name, created, err)
			}
			e.setStatus(job.ID, "succeeded")
			if _, err := e.svc.StartDownload(ctx, a, job.ID); err != nil {
				t.Fatalf("%s/%s: StartDownload: %v", role, name, err)
			}

			req := e.viewerRows(a.UserID, ActionExportRequested)
			exp := e.viewerRows(a.UserID, ActionMasterDataExported)
			if len(req) != 1 || len(exp) != 1 {
				t.Fatalf("%s/%s: export_requested=%d master_data_exported=%d, want 1/1", role, name, len(req), len(exp))
			}
			if _, has := req[0].After["row_count"]; has {
				t.Errorf("%s/%s: export_requested carries row_count", role, name)
			}
			if rc, _ := exp[0].After["row_count"].(float64); rc != 3 {
				t.Errorf("%s/%s: master_data_exported row_count = %v, want 3", role, name, exp[0].After["row_count"])
			}
			var inOrder bool
			if err := e.pool.QueryRow(ctx,
				`SELECT (SELECT min(id) FROM audit_logs WHERE actor_id = $1 AND action = $2)
				      < (SELECT min(id) FROM audit_logs WHERE actor_id = $1 AND action = $3)`,
				a.UserID, ActionExportRequested, ActionMasterDataExported).Scan(&inOrder); err != nil || !inOrder {
				t.Errorf("%s/%s: export_requested not before master_data_exported (err=%v)", role, name, err)
			}
		}
	}
}
