package service

import (
	"context"
	"errors"
	"testing"
	"time"
)

// RBAC, month validation and the start-date switch never reach the repository, so a service
// without a repo is enough (queries are covered by the repository integration test).
func newDsrLateTestService(startDate *time.Time) *DsrLateReportService {
	now := time.Date(2026, 10, 27, 3, 0, 0, 0, time.UTC) // 10:00 WIB
	return &DsrLateReportService{startDate: startDate, now: func() time.Time { return now }}
}

func TestDsrLateReport_Authorize(t *testing.T) {
	month := time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC)
	cases := []struct {
		role    string
		allowed bool
	}{
		{"ATM-USER", true}, {"ATM-SPV", true}, {"BRANCH-ATM-SPV", true}, {"ADMIN", true}, {"atm-spv", true},
		{"BRANCH-ATM-USER", false}, {"ACM-USER", false}, {"VENDOR-USER", false}, {"", false},
	}
	for _, tc := range cases {
		t.Run(tc.role, func(t *testing.T) {
			s := newDsrLateTestService(nil)
			a := Actor{UserID: 1, Role: tc.role}
			f := DsrLateFilter{Month: month}
			_, e1 := s.Daily(context.Background(), a, f)
			_, e2 := s.Summary(context.Background(), a, f)
			_, e3 := s.Warnings(context.Background(), a, f)
			for i, err := range []error{e1, e2, e3} {
				if tc.allowed && err != nil {
					t.Errorf("call %d: unexpected error %v", i, err)
				}
				if !tc.allowed && !errors.Is(err, ErrNotAuthorized) {
					t.Errorf("call %d: err = %v, want ErrNotAuthorized", i, err)
				}
			}
		})
	}
}

func TestDsrLateReport_MonthAndStatusValidation(t *testing.T) {
	bad := "x"
	good := "late"
	cases := []struct {
		name   string
		month  time.Time
		status *string
		field  string // "" = valid
	}{
		{"current month", time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC), nil, ""},
		{"past month", time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC), &good, ""},
		{"next month", time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC), nil, "month"},
		{"zero month", time.Time{}, nil, "month"},
		{"unknown status", time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC), &bad, "status"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			s := newDsrLateTestService(nil)
			_, err := s.Daily(context.Background(), Actor{Role: "ADMIN"}, DsrLateFilter{Month: tc.month, Status: tc.status})
			var ve *ValidationError
			switch {
			case tc.field == "" && err != nil:
				t.Fatalf("err = %v, want nil", err)
			case tc.field != "" && (!errors.As(err, &ve) || ve.Field != tc.field):
				t.Fatalf("err = %v, want validation error on %s", err, tc.field)
			}
		})
	}
}

func TestDsrLateReport_NoStartDateIsEmpty(t *testing.T) {
	s := newDsrLateTestService(nil)
	a := Actor{Role: "ADMIN"}
	f := DsrLateFilter{Month: time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC)}
	d, err := s.Daily(context.Background(), a, f)
	if err != nil || d.Total != 0 || len(d.Items) != 0 || d.StartDate != nil || d.Page != 1 || d.PageSize != dsrLateDefaultPage {
		t.Fatalf("daily = %+v, %v", d, err)
	}
	if sum, err := s.Summary(context.Background(), a, f); err != nil || sum.Items == nil || len(sum.Items) != 0 || sum.StartDate != nil {
		t.Fatalf("summary = %+v, %v", sum, err)
	}
	if w, err := s.Warnings(context.Background(), a, f); err != nil || w.UnmappedVaults == nil || w.UnmappedLabels == nil || w.UnresolvedVendors == nil {
		t.Fatalf("warnings = %+v, %v", w, err)
	}
}

func TestParseDsrLateMonth(t *testing.T) {
	for _, in := range []string{"", "2026-1", "2026-13", "2026-10-01", "abc"} {
		if _, err := ParseDsrLateMonth(in); err == nil {
			t.Errorf("%q: want error", in)
		}
	}
	if m, err := ParseDsrLateMonth("2026-10"); err != nil || m.Day() != 1 || m.Month() != time.October {
		t.Errorf("valid month: %v, %v", m, err)
	}
}
