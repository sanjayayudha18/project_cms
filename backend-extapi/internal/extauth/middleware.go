package extauth

import (
	"context"
	"encoding/json"
	"log/slog"
	"math"
	"net/http"
	"net/netip"
	"slices"
	"strconv"
	"strings"
	"time"

	custommw "github.com/cimb-niaga/cms/pkg/middleware"
)

// ClientContext is the only identity RequireClient puts in the request
// context. There is never a pkg/middleware.AuthContext here (Req 3.6).
type ClientContext struct {
	Client        Client
	GrantedScopes []string
	IP            string
}

// ClientReader loads a client by client_id from the primary pool; nil, nil
// when it does not exist.
type ClientReader interface {
	GetByClientID(ctx context.Context, clientID string) (*Client, error)
}

type clientContextKey struct{}

// GetClientContext returns the ClientContext stored by RequireClient.
func GetClientContext(ctx context.Context) (*ClientContext, bool) {
	cc, ok := ctx.Value(clientContextKey{}).(*ClientContext)
	return cc, ok && cc != nil
}

// WWW-Authenticate header values (RFC 6750), not credentials.
const (
	bearerInvalidToken      = `Bearer error="invalid_token"`      //nolint:gosec // header value
	bearerInsufficientScope = `Bearer error="insufficient_scope"` //nolint:gosec // header value
)

// RequireClient authenticates a Client_Token in this order (Req 4.1–4.5):
// Bearer header → Validate → client from primary → active and ver ==
// credential_version → source IP in allowed_cidrs → per-client rate → store
// ClientContext. r.RemoteAddr must already be resolved by TrustedRealIP.
func RequireClient(tokens *TokenService, clients ClientReader, rate *APIRate) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx := r.Context()

			raw, ok := bearerToken(r)
			if !ok {
				writeInvalidToken(w)
				return
			}
			claims, err := tokens.Validate(raw)
			if err != nil {
				writeInvalidToken(w)
				return
			}

			client, err := clients.GetByClientID(ctx, claims.Subject)
			if err != nil {
				slog.ErrorContext(ctx, "ext_auth_client_lookup_failed", slog.String("error", err.Error()))
				writeError(w, http.StatusInternalServerError, "internal_error", "internal error")
				return
			}
			// Missing, disabled, or secret rotated since the token was issued
			// (Req 1.4, 1.5, 4.2).
			if client == nil || !client.IsActive || client.ClientID != claims.Subject ||
				claims.Ver != client.CredentialVersion {
				writeInvalidToken(w)
				return
			}

			// Token and client are verified: only now is client_id trusted
			// enough for the request log (Req 4.6).
			if lf := custommw.LogFieldsFrom(ctx); lf != nil {
				lf.ClientID = client.ClientID
			}

			ip, ok := remoteIP(r.RemoteAddr)
			if !ok || !client.AllowsIP(ip) { // empty CIDR list allows nothing (Req 1.7)
				writeError(w, http.StatusForbidden, "ip_not_allowed", "source IP not allowed")
				return
			}

			retryAfter, err := rate.Allow(ctx, client.ClientID)
			if err != nil {
				slog.ErrorContext(ctx, "ext_auth_rate_unavailable", slog.String("error", err.Error()))
				writeError(w, http.StatusServiceUnavailable, "service_unavailable", "service unavailable")
				return
			}
			if retryAfter > 0 {
				w.Header().Set("Retry-After", retryAfterSeconds(retryAfter))
				writeError(w, http.StatusTooManyRequests, "rate_limited", "too many requests")
				return
			}

			cc := &ClientContext{
				Client:        *client,
				GrantedScopes: grantedScopes(claims.Scopes(), *client),
				IP:            ip.String(),
			}
			next.ServeHTTP(w, r.WithContext(context.WithValue(ctx, clientContextKey{}, cc)))
		})
	}
}

// RequireScope answers 403 insufficient_scope unless the verified token
// carries scope (Req 4.4). It must run after RequireClient.
func RequireScope(scope string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			cc, ok := GetClientContext(r.Context())
			if !ok {
				writeInvalidToken(w)
				return
			}
			if !slices.Contains(cc.GrantedScopes, scope) {
				w.Header().Set("WWW-Authenticate", bearerInsufficientScope)
				writeError(w, http.StatusForbidden, "insufficient_scope", "insufficient scope")
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// grantedScopes keeps the token scopes the client still holds, so narrowing a
// client's scopes takes effect before its tokens expire.
func grantedScopes(tokenScopes []string, c Client) []string {
	out := make([]string, 0, len(tokenScopes))
	for _, s := range tokenScopes {
		if c.HasScope(s) {
			out = append(out, s)
		}
	}
	return out
}

// bearerToken extracts the token from "Authorization: Bearer <token>" (scheme
// case-insensitive, RFC 6750).
func bearerToken(r *http.Request) (string, bool) {
	scheme, tok, ok := strings.Cut(r.Header.Get("Authorization"), " ")
	tok = strings.TrimSpace(tok)
	if !ok || !strings.EqualFold(scheme, "Bearer") || tok == "" {
		return "", false
	}
	return tok, true
}

// remoteIP parses r.RemoteAddr: a bare IP (set by TrustedRealIP) or ip:port.
func remoteIP(addr string) (netip.Addr, bool) {
	if ap, err := netip.ParseAddrPort(addr); err == nil {
		return ap.Addr().Unmap(), true
	}
	ip, err := netip.ParseAddr(addr)
	if err != nil {
		return netip.Addr{}, false
	}
	return ip.Unmap(), true
}

func retryAfterSeconds(d time.Duration) string {
	return strconv.Itoa(max(1, int(math.Ceil(d.Seconds()))))
}

func writeInvalidToken(w http.ResponseWriter) {
	w.Header().Set("WWW-Authenticate", bearerInvalidToken)
	writeError(w, http.StatusUnauthorized, "invalid_token", "invalid or expired token")
}

func writeError(w http.ResponseWriter, status int, code, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]string{"error": code, "message": message}) // headers already sent
}
