//go:build integration

package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/cimb-niaga/cms/backend/internal/db"
	"github.com/cimb-niaga/cms/backend/internal/service"
)

// setupAtmPortalHarness returns a router with only the ATM Portal handler
// mounted (no RequireAuth wrapper — auth enforcement over this route is
// already covered by the manual verification in Task 4.2 and by
// integration_test.go's login/token tests, so this file focuses purely on
// Handler -> Service -> Repository -> Postgres round-trip correctness) and
// the real Postgres transaction backing it, always rolled back via
// t.Cleanup.
func setupAtmPortalHarness(t *testing.T) (chi.Router, pgx.Tx) {
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
		t.Fatalf("begin tx: %v", err)
	}
	t.Cleanup(func() { tx.Rollback(ctx) })

	svc := service.NewAtmPortalService(db.New(tx))
	h := NewAtmPortalHandler(svc)

	r := chi.NewRouter()
	r.Mount("/api/v1/atm-portal", h.Routes())

	return r, tx
}

func seedRegionID(t *testing.T, tx pgx.Tx) int64 {
	t.Helper()
	var id int64
	err := tx.QueryRow(context.Background(), "SELECT id FROM regions ORDER BY id LIMIT 1").Scan(&id)
	if err != nil {
		t.Fatalf("reading a seed region id: %v", err)
	}
	return id
}

func insertLocation(t *testing.T, tx pgx.Tx, regionID int64) int64 {
	t.Helper()
	var id int64
	err := tx.QueryRow(context.Background(), `
		INSERT INTO locations (region_id, type, name, address_line1, city_or_regency, province, country_code)
		VALUES ($1, 'branch', 'Integration Test Location', 'Jl. Test No. 1', 'Jakarta', 'DKI Jakarta', 'ID')
		RETURNING id
	`, regionID).Scan(&id)
	if err != nil {
		t.Fatalf("insert location: %v", err)
	}
	return id
}

// testATMSpec describes one ATM to seed for these integration tests.
type testATMSpec struct {
	terminalID                      string
	machineType, brand, deployType  string
	lowThreshold, criticalThreshold *float64
	hasCashpos                      bool
	refundTotal, replenishTotal     float64
}

func insertATM(t *testing.T, tx pgx.Tx, locationID int64, spec testATMSpec) {
	t.Helper()
	ctx := context.Background()
	_, err := tx.Exec(ctx, `
		INSERT INTO atms (terminal_id, location_id, machine_type, brand, model, operation_hours, deployment_type, low_threshold_amount, critical_threshold_amount, is_active)
		VALUES ($1, $2, $3, $4, 'TestModel', '24H', $5, $6, $7, true)
	`, spec.terminalID, locationID, spec.machineType, spec.brand, spec.deployType, spec.lowThreshold, spec.criticalThreshold)
	if err != nil {
		t.Fatalf("insert atm %s: %v", spec.terminalID, err)
	}

	if !spec.hasCashpos {
		return
	}
	var fileID int64
	err = tx.QueryRow(ctx, `
		INSERT INTO itm_replenish_files (filename, file_date, status)
		VALUES ($1, CURRENT_DATE, 'completed')
		RETURNING id
	`, "itest-"+uuid.NewString()+".txt").Scan(&fileID)
	if err != nil {
		t.Fatalf("insert cashpos file: %v", err)
	}
	_, err = tx.Exec(ctx, `
		INSERT INTO itm_replenish (file_id, replenish_date, replenish_time, terminal_id, machine_type, teller_id, branch_code, refund_total, replenish_total)
		VALUES ($1, CURRENT_DATE, '10:00:00'::time, $2, $3, 'T001', 'BR001', $4, $5)
	`, fileID, spec.terminalID, spec.machineType, spec.refundTotal, spec.replenishTotal)
	if err != nil {
		t.Fatalf("insert cashpos for %s: %v", spec.terminalID, err)
	}
}

func doGetATMs(h http.Handler, query string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodGet, "/api/v1/atm-portal/atms"+query, nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	return w
}

func decodeListATMsResponse(t *testing.T, w *httptest.ResponseRecorder) listATMsResponse {
	t.Helper()
	var resp listATMsResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decoding response: %v (body: %s)", err, w.Body.String())
	}
	return resp
}

func findRow(rows []atmPortalRow, terminalID string) *atmPortalRow {
	for i := range rows {
		if rows[i].TerminalID == terminalID {
			return &rows[i]
		}
	}
	return nil
}

// ─── Task 5.4: Full round-trip ─────────────────────────────────────────────

// TestIntegration_AtmPortal_FullRoundTrip validates the complete
// Handler -> Service -> Repository -> Postgres path: seeded ATMs across
// three distinct statuses come back with the exact field values expected,
// through the real HTTP handler and JSON encoding.
func TestIntegration_AtmPortal_FullRoundTrip(t *testing.T) {
	router, tx := setupAtmPortalHarness(t)
	regionID := seedRegionID(t, tx)
	locationID := insertLocation(t, tx, regionID)

	marker := "RT" + uuid.NewString()[:8]
	low, critical := 100.0, 50.0

	specs := []testATMSpec{
		{
			terminalID: marker + "-normal", machineType: "ATM", brand: "Hyosung", deployType: "ONSITE",
			lowThreshold: &low, criticalThreshold: &critical,
			hasCashpos: true, refundTotal: 200, replenishTotal: 500,
		},
		{
			terminalID: marker + "-critical", machineType: "CRM", brand: "Wincor", deployType: "OFFSITE",
			lowThreshold: &low, criticalThreshold: &critical,
			hasCashpos: true, refundTotal: 25, replenishTotal: 300,
		},
		{
			terminalID: marker + "-unconfigured", machineType: "ATM", brand: "Hyosung", deployType: "ONSITE",
			hasCashpos: false,
		},
	}
	for _, spec := range specs {
		insertATM(t, tx, locationID, spec)
	}

	w := doGetATMs(router, "?search="+marker+"&status=all&page=1&page_size=10&sort_by=terminal_id&sort_order=asc")
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	resp := decodeListATMsResponse(t, w)
	if len(resp.Data) != 3 {
		t.Fatalf("expected 3 rows, got %d: %+v", len(resp.Data), resp.Data)
	}
	if resp.Total != 3 {
		t.Errorf("total = %d, want 3", resp.Total)
	}
	if resp.Page != 1 || resp.PageSize != 10 {
		t.Errorf("page/page_size = %d/%d, want 1/10", resp.Page, resp.PageSize)
	}

	normal := findRow(resp.Data, marker+"-normal")
	if normal == nil {
		t.Fatal("normal-status atm not found in response")
	}
	if normal.Status != "normal" {
		t.Errorf("normal atm status = %q, want %q", normal.Status, "normal")
	}
	if normal.MachineType != "ATM" || normal.Brand != "Hyosung" || normal.DeploymentType != "ONSITE" {
		t.Errorf("normal atm fields = %+v, want machine_type=ATM brand=Hyosung deployment_type=ONSITE", normal)
	}
	if normal.RefundTotal == nil || *normal.RefundTotal != 200 {
		t.Errorf("normal atm refund_total = %v, want 200", normal.RefundTotal)
	}
	if normal.LowThreshold == nil || *normal.LowThreshold != 100 {
		t.Errorf("normal atm low_threshold = %v, want 100", normal.LowThreshold)
	}

	critRow := findRow(resp.Data, marker+"-critical")
	if critRow == nil {
		t.Fatal("critical-status atm not found in response")
	}
	if critRow.Status != "critical" {
		t.Errorf("critical atm status = %q, want %q", critRow.Status, "critical")
	}
}

// ─── Task 5.4: Brand / deployment_type filtering ───────────────────────────

// TestIntegration_AtmPortal_BrandDeploymentTypeFiltering validates that
// combining brand and deployment_type filters narrows results to exactly
// the matching row, through the real HTTP query-param parsing.
func TestIntegration_AtmPortal_BrandDeploymentTypeFiltering(t *testing.T) {
	router, tx := setupAtmPortalHarness(t)
	regionID := seedRegionID(t, tx)
	locationID := insertLocation(t, tx, regionID)

	marker := "BD" + uuid.NewString()[:8]
	combos := []struct{ brand, deployType string }{
		{"Hyosung", "ONSITE"},
		{"Hyosung", "OFFSITE"},
		{"Wincor", "ONSITE"},
		{"Wincor", "OFFSITE"},
	}
	for i, c := range combos {
		insertATM(t, tx, locationID, testATMSpec{
			terminalID: marker + "-" + string(rune('a'+i)), machineType: "ATM",
			brand: c.brand, deployType: c.deployType,
		})
	}

	w := doGetATMs(router, "?search="+marker+"&brand=Wincor&deployment_type=OFFSITE&page_size=10")
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	resp := decodeListATMsResponse(t, w)
	if len(resp.Data) != 1 {
		t.Fatalf("expected 1 row for brand=Wincor&deployment_type=OFFSITE, got %d: %+v", len(resp.Data), resp.Data)
	}
	if resp.Data[0].Brand != "Wincor" || resp.Data[0].DeploymentType != "OFFSITE" {
		t.Errorf("row = %+v, want brand=Wincor deployment_type=OFFSITE", resp.Data[0])
	}

	// Comma-separated brand should also work (multi-select on the frontend).
	w2 := doGetATMs(router, "?search="+marker+"&brand=Wincor,Hyosung&page_size=10")
	resp2 := decodeListATMsResponse(t, w2)
	if len(resp2.Data) != 4 {
		t.Fatalf("expected 4 rows for brand=Wincor,Hyosung, got %d", len(resp2.Data))
	}
}

// ─── Task 5.4: NULL threshold / refund_total edge cases ────────────────────

// TestIntegration_AtmPortal_NullEdgeCases validates that NULL DB values
// serialize as JSON `null` (not zero values) for both an ATM with no
// thresholds configured at all (status "unconfigured") and an ATM with
// thresholds configured but no itm_replenish record yet (status "no_data").
func TestIntegration_AtmPortal_NullEdgeCases(t *testing.T) {
	router, tx := setupAtmPortalHarness(t)
	regionID := seedRegionID(t, tx)
	locationID := insertLocation(t, tx, regionID)

	marker := "NE" + uuid.NewString()[:8]
	low := 100.0

	insertATM(t, tx, locationID, testATMSpec{
		terminalID: marker + "-unconfigured", machineType: "ATM", brand: "Hyosung", deployType: "ONSITE",
		hasCashpos: false,
	})
	insertATM(t, tx, locationID, testATMSpec{
		terminalID: marker + "-nodata", machineType: "ATM", brand: "Hyosung", deployType: "ONSITE",
		lowThreshold: &low, hasCashpos: false,
	})

	w := doGetATMs(router, "?search="+marker+"&status=all&page_size=10")
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	// Decode as raw JSON too, to distinguish JSON `null` from an absent
	// field or a zero value — json.Decoder into *float64/*string already
	// gives nil for `null`, but re-checking against the raw body closes
	// off any doubt about what actually went over the wire.
	rawBody := w.Body.String()
	for _, field := range []string{`"low_threshold":null`, `"refund_total":null`, `"last_replenish_date":null`} {
		if !strings.Contains(rawBody, field) {
			t.Errorf("expected raw response to contain %s, body: %s", field, rawBody)
		}
	}

	resp := decodeListATMsResponse(t, doGetATMs(router, "?search="+marker+"&status=all&page_size=10"))

	unconfigured := findRow(resp.Data, marker+"-unconfigured")
	if unconfigured == nil {
		t.Fatal("unconfigured atm not found")
	}
	if unconfigured.Status != "unconfigured" {
		t.Errorf("status = %q, want unconfigured", unconfigured.Status)
	}
	if unconfigured.LowThreshold != nil || unconfigured.CriticalThreshold != nil {
		t.Errorf("expected nil thresholds, got low=%v critical=%v", unconfigured.LowThreshold, unconfigured.CriticalThreshold)
	}
	if unconfigured.RefundTotal != nil || unconfigured.ReplenishTotal != nil || unconfigured.Escrow != nil {
		t.Errorf("expected nil cashpos fields, got refund=%v replenish=%v escrow=%v",
			unconfigured.RefundTotal, unconfigured.ReplenishTotal, unconfigured.Escrow)
	}
	if unconfigured.LastReplenishDate != nil || unconfigured.LastReplenishTime != nil {
		t.Errorf("expected nil replenish date/time, got date=%v time=%v", unconfigured.LastReplenishDate, unconfigured.LastReplenishTime)
	}

	noData := findRow(resp.Data, marker+"-nodata")
	if noData == nil {
		t.Fatal("no_data atm not found")
	}
	if noData.Status != "no_data" {
		t.Errorf("status = %q, want no_data", noData.Status)
	}
	if noData.LowThreshold == nil || *noData.LowThreshold != low {
		t.Errorf("low_threshold = %v, want %v (threshold is configured)", noData.LowThreshold, low)
	}
	if noData.RefundTotal != nil {
		t.Errorf("expected nil refund_total for no_data atm, got %v", *noData.RefundTotal)
	}
}
