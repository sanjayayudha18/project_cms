package handler

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/cimb-niaga/cms/backend/internal/service"
)

func TestGetATMProfile_EmptyTerminalID(t *testing.T) {
	svc := &stubAtmPortalService{}
	r := mountCashposHandler(svc)
	// chi collapses "//" so hit the handler directly via an empty-after-trim id.
	req := httptest.NewRequest(http.MethodGet, "/api/v1/atm-portal/atms/%20", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("status %d body %s", w.Code, w.Body.String())
	}
}

func TestGetATMProfile_NotFound(t *testing.T) {
	svc := &stubAtmPortalService{profileErr: service.ErrATMNotFound}
	r := mountCashposHandler(svc)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/atm-portal/atms/MISSING", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusNotFound {
		t.Fatalf("status %d body %s", w.Code, w.Body.String())
	}
}

func TestGetATMProfile_Success(t *testing.T) {
	low := "50000000.00"
	svc := &stubAtmPortalService{
		profileResult: &service.ATMProfileResult{
			TerminalID: "ATM001", LocationName: "KCP Sudirman", Address: "Jl. Sudirman",
			MachineType: "CRM", Brand: "NCR", Model: "SelfServ 84", DeploymentType: "ONSITE",
			OperationHours: "24_HOURS", LowThresholdAmount: &low, IsActive: true,
			ReplenishmentStatus: "normal",
		},
	}
	r := mountCashposHandler(svc)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/atm-portal/atms/ATM001", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("status %d body %s", w.Code, w.Body.String())
	}
	if svc.lastProfileTerminalID != "ATM001" {
		t.Errorf("terminalID = %q", svc.lastProfileTerminalID)
	}
}

func TestListATMReplenish_BadPage(t *testing.T) {
	svc := &stubAtmPortalService{}
	r := mountCashposHandler(svc)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/atm-portal/atms/ATM001/replenish?page=abc", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("status %d", w.Code)
	}
}

func TestListATMReplenish_Defaults(t *testing.T) {
	svc := &stubAtmPortalService{
		listReplenishResult: &service.ListATMReplenishResult{Page: 1, PageSize: 25},
	}
	r := mountCashposHandler(svc)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/atm-portal/atms/ATM001/replenish", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("status %d body %s", w.Code, w.Body.String())
	}
	p := svc.lastReplenishParams
	if p.TerminalID != "ATM001" || p.Page != 1 || p.PageSize != 25 {
		t.Errorf("params: %+v", p)
	}
}

func TestListATMCashpos_ValidationError(t *testing.T) {
	svc := &stubAtmPortalService{
		listATMCashposErr: &service.ValidationError{Field: "date_from", Message: "tidak boleh lebih besar dari date_to"},
	}
	r := mountCashposHandler(svc)
	req := httptest.NewRequest(http.MethodGet,
		"/api/v1/atm-portal/atms/ATM001/cashpos?date_from=2026-08-21&date_to=2026-08-01", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("status %d body %s", w.Code, w.Body.String())
	}
}

func TestListATMCashpos_Success(t *testing.T) {
	svc := &stubAtmPortalService{
		listATMCashposResult: &service.ListATMCashposResult{Page: 1, PageSize: 25},
	}
	r := mountCashposHandler(svc)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/atm-portal/atms/ATM001/cashpos", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("status %d body %s", w.Code, w.Body.String())
	}
	if svc.lastATMCashposParams.TerminalID != "ATM001" {
		t.Errorf("terminalID = %q", svc.lastATMCashposParams.TerminalID)
	}
}
