package service

import (
	"errors"
	"testing"
)

// SetRoles checks the role and the roles payload before it opens a transaction
// (a nil pool would panic if it got further): non-admin and invalid roles are
// refused with no DB access. The director rules live in
// vendor_pic_director_integration_test.go (they need the row).
func TestVendorPicRoleService_SetRoles_RefusedBeforeTx(t *testing.T) {
	svc := NewVendorPicRoleService(nil)

	if _, err := svc.SetRoles(ctxWithRole(7, "ATM-SPV"), 7, 3, 1, []string{}, ""); !errors.Is(err, ErrMasterDataForbidden) {
		t.Errorf("non-admin: want ErrMasterDataForbidden, got %v", err)
	}
	for _, roles := range [][]string{{"courier"}, {"vault", "vault"}} {
		_, err := svc.SetRoles(adminCtx(7), 7, 3, 1, roles, "")
		var ve *ValidationError
		if !errors.As(err, &ve) || ve.Field != "roles" {
			t.Errorf("roles %q: want roles ValidationError, got %v", roles, err)
		}
	}
	if !errors.Is(ErrVendorDirectorNoRoles, ErrVendorDirectorNoRoles) || ErrVendorDirectorNoRoles.Error() == "" {
		t.Error("ErrVendorDirectorNoRoles must carry a message")
	}
}
