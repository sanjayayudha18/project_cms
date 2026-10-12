package middleware

import (
	"net/http"
	"net/netip"
	"strings"
)

// TrustedRealIP replaces chi's deprecated RealIP. X-Forwarded-For is only read
// when the connection comes from a trusted proxy; it is then walked right to
// left, skipping trusted hops, and the first untrusted address becomes
// r.RemoteAddr (bare IP, no port). Otherwise r.RemoteAddr is left untouched,
// so a client can never choose its own IP. X-Real-IP / True-Client-IP are
// ignored. An empty trusted list makes this a no-op.
func TrustedRealIP(trusted []netip.Prefix) func(http.Handler) http.Handler {
	isTrusted := func(a netip.Addr) bool {
		for _, p := range trusted {
			if p.Contains(a) {
				return true
			}
		}
		return false
	}
	return func(next http.Handler) http.Handler {
		if len(trusted) == 0 {
			return next
		}
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if remote, ok := parseRemoteAddr(r.RemoteAddr); ok && isTrusted(remote) {
				if ip, ok := clientFromXFF(r.Header.Values("X-Forwarded-For"), isTrusted); ok {
					r.RemoteAddr = ip.String()
				}
			}
			next.ServeHTTP(w, r)
		})
	}
}

func parseRemoteAddr(s string) (netip.Addr, bool) {
	if ap, err := netip.ParseAddrPort(s); err == nil {
		return ap.Addr().Unmap(), true
	}
	a, err := netip.ParseAddr(s)
	return a.Unmap(), err == nil
}

// clientFromXFF returns the rightmost untrusted address. A malformed entry
// stops the walk with no result: anything left of it cannot be trusted either.
func clientFromXFF(values []string, isTrusted func(netip.Addr) bool) (netip.Addr, bool) {
	hops := strings.Split(strings.Join(values, ","), ",")
	for i := len(hops) - 1; i >= 0; i-- {
		a, err := netip.ParseAddr(strings.TrimSpace(hops[i]))
		if err != nil {
			return netip.Addr{}, false
		}
		a = a.Unmap()
		if !isTrusted(a) {
			return a, true
		}
	}
	return netip.Addr{}, false
}
