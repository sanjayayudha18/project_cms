//go:build integration

package handler

import (
	"context"
	"net/http"
	"strings"
	"testing"
	"time"
)

// ext-user-api task 3.5 (Property 4, login side, Req 9.4): a service account
// can never log in -- not while is_active=false, and not even if someone
// flips is_active=true (no provider supports auth_source='service').
// Rows are committed (the harness uses the shared pool), so they are deleted
// in cleanup.
func TestIntegration_ServiceAccountCannotLogin(t *testing.T) {
	h := setupHarness(t)
	ctx := context.Background()
	tag := time.Now().Format("150405.000000000")

	if _, err := h.pool.Exec(ctx, `INSERT INTO roles (role) VALUES ('EXT-CLIENT') ON CONFLICT DO NOTHING`); err != nil {
		t.Fatalf("seed EXT-CLIENT role: %v", err)
	}

	// Reuse the seeded user's password hash so the password itself is valid:
	// only auth_source/is_active may be what stops the login.
	for _, active := range []bool{false, true} {
		name := "inactive"
		if active {
			name = "active"
		}
		t.Run(name, func(t *testing.T) {
			username := "tsa_" + name + "_" + tag
			email := username + "@example.com"
			if _, err := h.pool.Exec(ctx,
				`INSERT INTO users (role_id, username, full_name, email, is_karyawan, auth_source, is_active, password_hash)
				 SELECT (SELECT id FROM roles WHERE role = 'EXT-CLIENT'), $1, $1, $2, true, 'service', $3,
				        (SELECT password_hash FROM users WHERE email = $4)`,
				username, email, active, testCompanyEmail,
			); err != nil {
				t.Fatalf("seed service account: %v", err)
			}
			t.Cleanup(func() {
				if _, err := h.pool.Exec(ctx, `DELETE FROM users WHERE username = $1`, username); err != nil {
					t.Errorf("cleanup %s: %v", username, err)
				}
			})

			w := doLogin(h.router, email, testDevPassword, companyPortal)

			if w.Code == http.StatusOK {
				t.Fatalf("service account logged in: %s", w.Body.String())
			}
			if strings.Contains(w.Body.String(), "access_token") || extractRefreshCookie(w) != "" {
				t.Errorf("tokens issued to a service account: %s", w.Body.String())
			}
		})
	}
}
