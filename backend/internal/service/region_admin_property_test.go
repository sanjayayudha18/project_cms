package service

import (
	"context"
	"errors"
	"regexp"
	"strings"
	"testing"

	"pgregory.net/rapid"
)

// Feature: region-management, Properties 1, 2, 8 (tasks 4.3, 4.4, 4.9).
//
// Pure, no DB: the service is built with nil repo/pool, so any input that got
// past RBAC + validation would panic on the first repo call. Every case here
// must therefore be decided before the service touches storage. The
// DB-dependent properties (3-6, 9) are covered by region_admin_integration_test.go.

var alnumRe = regexp.MustCompile(`^[A-Z0-9]{1,20}$`)

// Property 1: the stored code is the trimmed+uppercased input, normalization is
// idempotent, and a code is accepted iff that normalized form is 1-20 alphanumerics.
func TestRegionCode_NormalizedAndAcceptedIffAlnum(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		raw := rapid.OneOf(
			rapid.StringMatching(`\s{0,3}[A-Za-z0-9]{1,25}\s{0,3}`),
			rapid.String(),
		).Draw(t, "raw")

		want := normalizeRegionCode(raw)
		if normalizeRegionCode(want) != want {
			t.Fatalf("normalize not idempotent for %q", raw)
		}

		got, err := validateRegionCode(raw)
		if alnumRe.MatchString(want) {
			if err != nil || got != want {
				t.Fatalf("valid code %q: got (%q, %v), want (%q, nil)", raw, got, err, want)
			}
			return
		}
		var ve *ValidationError
		if !errors.As(err, &ve) || ve.Field != "code" {
			t.Fatalf("invalid code %q: want ValidationError on code, got %v", raw, err)
		}
	})
}

// Property 2: an invalid code or name is rejected with ValidationError before
// any storage call (nil repo would panic otherwise).
func TestCreateRegion_InvalidInputRejectedWithoutMutation(t *testing.T) {
	svc := NewRegionAdminService(nil, nil)
	rapid.Check(t, func(t *rapid.T) {
		code := rapid.OneOf(
			rapid.Just(""), rapid.Just("   "),
			rapid.StringMatching(`[A-Z0-9]{21,30}`),
			rapid.StringMatching(`[A-Z0-9]{0,5}[-_.!@][A-Z0-9]{0,5}`),
		).Draw(t, "badCode")
		name := rapid.OneOf(
			rapid.Just(""), rapid.Just("  "),
			rapid.StringMatching(`[a-z]{101,120}`),
			rapid.StringMatching(`[a-z]{1,100}`),
		).Draw(t, "name")

		_, err := svc.Create(context.Background(), 1, "ADMIN", CreateRegionRequest{Code: code, Region: name}, "127.0.0.1")
		var ve *ValidationError
		if !errors.As(err, &ve) {
			t.Fatalf("code=%q name=%q: want ValidationError, got %v", code, name, err)
		}
	})
}

// Property 8: any role other than ADMIN/ADMIN_PARAM is refused at the service
// layer, for every mutation, regardless of payload.
func TestRegionMutations_NonAdminRefusedAtService(t *testing.T) {
	svc := NewRegionAdminService(nil, nil)
	ctx := context.Background()
	rapid.Check(t, func(t *rapid.T) {
		role := rapid.OneOf(
			rapid.SampledFrom([]string{"", "APPACCESS", "ATM-USER", "VENDOR", "OPERATOR", "MANAGER", "ADMINX"}),
			rapid.StringMatching(`[A-Z_-]{0,12}`),
		).Filter(func(r string) bool {
			n := strings.ToUpper(strings.TrimSpace(r))
			return n != "ADMIN" && n != "ADMIN_PARAM"
		}).Draw(t, "role")
		id := rapid.Int64Range(1, 1_000_000).Draw(t, "id")

		errs := []error{}
		_, err := svc.Create(ctx, 1, role, CreateRegionRequest{Code: "JKT", Region: "Jakarta"}, "")
		errs = append(errs, err)
		_, err = svc.UpdateName(ctx, 1, role, id, UpdateRegionNameRequest{Region: "Jakarta"}, "")
		errs = append(errs, err)
		_, err = svc.Disable(ctx, 1, role, id, "")
		errs = append(errs, err)
		_, err = svc.Enable(ctx, 1, role, id, "")
		errs = append(errs, err)

		for i, e := range errs {
			if !errors.Is(e, ErrNotAuthorized) {
				t.Fatalf("role %q mutation #%d: want ErrNotAuthorized, got %v", role, i, e)
			}
		}
	})
}
