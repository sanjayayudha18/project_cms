package handler

import (
	"net/http"
	"net/http/httptest"
	"net/netip"
	"testing"

	"github.com/cimb-niaga/cms/pkg/middleware"
)

// T6 (client-ip-trust FR3): host from RemoteAddr only; XFF is never read here.
func TestExtractClientIP(t *testing.T) {
	cases := []struct{ remote, want string }{
		{"203.0.113.9:5555", "203.0.113.9"},
		{"[2001:db8::7]:5555", "2001:db8::7"},
		{"203.0.113.9", "203.0.113.9"}, // already bare (set by TrustedRealIP)
		{"2001:db8::7", "2001:db8::7"},
	}
	for _, tc := range cases {
		req := httptest.NewRequest(http.MethodPost, "/login", nil)
		req.RemoteAddr = tc.remote
		req.Header.Set("X-Forwarded-For", "1.2.3.4")
		if got := extractClientIP(req); got != tc.want {
			t.Errorf("extractClientIP(%q) = %q, want %q", tc.remote, got, tc.want)
		}
	}
}

// T7 (FR3, NFR3): through the production middleware, a spoofed XFF from an
// untrusted client never reaches the IP login uses for its rate limit
// (Login passes extractClientIP(r) as LoginRequest.IP); from trusted Nginx the
// real client IP does.
func TestExtractClientIP_BehindTrustedRealIP(t *testing.T) {
	trusted := []netip.Prefix{netip.MustParsePrefix("172.17.0.1/32")}
	cases := []struct{ name, remote, xff, want string }{
		{"direct client spoofs XFF", "203.0.113.9:5555", "1.2.3.4", "203.0.113.9"},
		{"via nginx", "172.17.0.1:5555", "1.2.3.4, 198.51.100.7", "198.51.100.7"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var got string
			h := middleware.TrustedRealIP(trusted)(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
				got = extractClientIP(r)
			}))
			req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", nil)
			req.RemoteAddr = tc.remote
			req.Header.Set("X-Forwarded-For", tc.xff)
			h.ServeHTTP(httptest.NewRecorder(), req)
			if got != tc.want {
				t.Errorf("IP = %q, want %q", got, tc.want)
			}
		})
	}
}
