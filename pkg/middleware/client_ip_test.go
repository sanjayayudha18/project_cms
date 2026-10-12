package middleware

import (
	"net/http"
	"net/http/httptest"
	"net/netip"
	"testing"
)

// T2–T5 (client-ip-trust FR2, NFR3).
func TestTrustedRealIP(t *testing.T) {
	nginx := []netip.Prefix{netip.MustParsePrefix("172.17.0.1/32")}
	withLB := []netip.Prefix{netip.MustParsePrefix("172.17.0.1/32"), netip.MustParsePrefix("10.0.0.0/8")}
	loop6 := []netip.Prefix{netip.MustParsePrefix("::1/128")}

	cases := []struct {
		name    string
		trusted []netip.Prefix
		remote  string
		xff     []string
		realIP  string
		want    string
	}{
		// T2: untrusted peer — every header ignored.
		{"untrusted peer spoofs XFF", nginx, "203.0.113.9:5555", []string{"1.2.3.4"}, "5.6.7.8", "203.0.113.9:5555"},
		{"empty trusted list is no-op", nil, "172.17.0.1:5555", []string{"1.2.3.4"}, "", "172.17.0.1:5555"},
		// T3: one trusted hop, leftmost spoofed entry ignored.
		{"nginx hop", nginx, "172.17.0.1:5555", []string{"1.2.3.4, 203.0.113.9"}, "", "203.0.113.9"},
		{"X-Real-IP ignored", nginx, "172.17.0.1:5555", nil, "1.2.3.4", "172.17.0.1:5555"},
		// T4: two trusted hops (future LB).
		{"LB + nginx hops", withLB, "172.17.0.1:5555", []string{"203.0.113.9, 10.1.2.3"}, "", "203.0.113.9"},
		{"XFF split across headers", withLB, "172.17.0.1:5555", []string{"1.2.3.4, 203.0.113.9", "10.1.2.3"}, "", "203.0.113.9"},
		// T5: nothing usable → RemoteAddr kept.
		{"no XFF", nginx, "172.17.0.1:5555", nil, "", "172.17.0.1:5555"},
		{"all hops trusted", withLB, "172.17.0.1:5555", []string{"10.9.9.9"}, "", "172.17.0.1:5555"},
		{"malformed rightmost", nginx, "172.17.0.1:5555", []string{"1.2.3.4, nope"}, "", "172.17.0.1:5555"},
		{"IPv6 peer with port", loop6, "[::1]:5555", []string{"2001:db8::7"}, "", "2001:db8::7"},
		{"IPv4-mapped client unmapped", nginx, "172.17.0.1:5555", []string{"::ffff:203.0.113.9"}, "", "203.0.113.9"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var got string
			h := TrustedRealIP(tc.trusted)(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
				got = r.RemoteAddr
			}))
			req := httptest.NewRequest(http.MethodGet, "/", nil)
			req.RemoteAddr = tc.remote
			for _, v := range tc.xff {
				req.Header.Add("X-Forwarded-For", v)
			}
			if tc.realIP != "" {
				req.Header.Set("X-Real-IP", tc.realIP)
			}
			h.ServeHTTP(httptest.NewRecorder(), req)
			if got != tc.want {
				t.Errorf("RemoteAddr = %q, want %q", got, tc.want)
			}
		})
	}
}
