// Package handler holds the HTTP handlers of backend-extapi.
package handler

import (
	"context"
	"encoding/json"
	"log/slog"
	"math"
	"mime"
	"net/http"
	"net/netip"
	"net/url"
	"slices"
	"strconv"
	"strings"

	"github.com/cimb-niaga/cms/backend-extapi/internal/extauth"
	custommw "github.com/cimb-niaga/cms/pkg/middleware"
)

// maxTokenBody caps the form body; a client_credentials request is a few
// dozen bytes.
const maxTokenBody = 4 << 10

// basicChallenge is the WWW-Authenticate value on every invalid_client (header
// value, not a credential).
const basicChallenge = `Basic realm="crown-ext"` //nolint:gosec // header value

// tokenErrors maps each RFC 6749 §5.2 error code to one fixed description, so
// every cause of one code yields a byte-identical body (Property 8).
var tokenErrors = map[string]struct {
	status int
	desc   string
}{
	"invalid_request":        {http.StatusBadRequest, "malformed token request"},
	"unsupported_grant_type": {http.StatusBadRequest, "grant_type must be client_credentials"},
	"invalid_client":         {http.StatusUnauthorized, "client authentication failed"},
	"invalid_scope":          {http.StatusBadRequest, "requested scope is not allowed"},
	"rate_limited":           {http.StatusTooManyRequests, "too many failed token requests"},
	"service_unavailable":    {http.StatusServiceUnavailable, "service unavailable"},
	"server_error":           {http.StatusInternalServerError, "internal error"},
}

// TokenHandler serves POST /api/ext/v1/oauth/token (OAuth2 client
// credentials, Req 2).
type TokenHandler struct {
	tokens   *extauth.TokenService
	clients  extauth.ClientReader
	failures *extauth.TokenFailures
}

// NewTokenHandler wires the token endpoint. clients must read the primary pool.
func NewTokenHandler(tokens *extauth.TokenService, clients extauth.ClientReader, failures *extauth.TokenFailures) *TokenHandler {
	return &TokenHandler{tokens: tokens, clients: clients, failures: failures}
}

type tokenResponse struct {
	AccessToken string `json:"access_token"`
	TokenType   string `json:"token_type"`
	ExpiresIn   int    `json:"expires_in"`
	Scope       string `json:"scope"`
}

// tokenRequest is a parsed request whose Basic credentials are present but
// not yet verified.
type tokenRequest struct {
	clientID, secret, scope string
	ip                      netip.Addr // invalid when RemoteAddr does not parse
	ipKey                   string     // rate-limit key and log value: the IP, never ip:port
}

// Token runs the design's 8 steps. r.RemoteAddr must already be resolved by
// TrustedRealIP. The secret, the Authorization header and the token are never
// logged (Req 2.10).
func (h *TokenHandler) Token(w http.ResponseWriter, r *http.Request) {
	// 1. Every response, success or error (Req 2.7).
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Pragma", "no-cache")

	// 2–4.
	req, code := parseTokenRequest(w, r)
	if code != "" {
		writeTokenError(w, code)
		return
	}
	ctx := r.Context()

	// 5. Failed-request limit, fail closed (Req 2.9).
	retryAfter, err := h.failures.Check(ctx, req.clientID, req.ipKey)
	if err != nil {
		slog.ErrorContext(ctx, "ext_token_rate_unavailable", slog.String("error", err.Error()))
		writeTokenError(w, "service_unavailable")
		return
	}
	if retryAfter > 0 {
		logDenied(ctx, req, "rate_limited")
		w.Header().Set("Retry-After", strconv.Itoa(max(1, int(math.Ceil(retryAfter.Seconds())))))
		writeTokenError(w, "rate_limited")
		return
	}

	// 6.
	client, ok := h.authenticate(ctx, w, req)
	if !ok {
		return
	}

	// 7–8.
	h.issue(ctx, w, req, client)
}

// parseTokenRequest covers steps 2–4: form body only, grant_type, Basic
// credentials. It returns the RFC 6749 error code, or "" when the request may
// go on to client authentication.
func parseTokenRequest(w http.ResponseWriter, r *http.Request) (tokenRequest, string) {
	mt, _, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if err != nil || mt != "application/x-www-form-urlencoded" {
		return tokenRequest{}, "invalid_request"
	}
	r.Body = http.MaxBytesReader(w, r.Body, maxTokenBody)
	if err := r.ParseForm(); err != nil {
		return tokenRequest{}, "invalid_request"
	}
	// The secret goes only in the Basic header (D2), never in the body or the
	// URL; a parameter sent twice is malformed (RFC 6749 §3.2).
	if r.Form.Has("client_secret") || len(r.PostForm["grant_type"]) > 1 || len(r.PostForm["scope"]) > 1 {
		return tokenRequest{}, "invalid_request"
	}
	if r.PostForm.Get("grant_type") != "client_credentials" {
		return tokenRequest{}, "unsupported_grant_type"
	}

	user, pass, ok := r.BasicAuth()
	if !ok {
		return tokenRequest{}, "invalid_client"
	}
	// RFC 6749 §2.3.1: both parts are form-urlencoded before Basic encoding.
	clientID, err1 := url.QueryUnescape(user)
	secret, err2 := url.QueryUnescape(pass)
	if err1 != nil || err2 != nil || clientID == "" || secret == "" {
		return tokenRequest{}, "invalid_client"
	}

	req := tokenRequest{clientID: clientID, secret: secret, scope: r.PostForm.Get("scope"), ipKey: r.RemoteAddr}
	if ip, ok := parseRemoteIP(r.RemoteAddr); ok {
		req.ip, req.ipKey = ip, ip.String()
	}
	return req, ""
}

// authenticate is step 6. Every failure cause answers the same 401
// invalid_client (Req 2.4); only the log carries the internal reason. The
// secret always goes through bcrypt (the dummy hash for an unknown client), so
// timing does not tell whether a client_id exists.
func (h *TokenHandler) authenticate(ctx context.Context, w http.ResponseWriter, req tokenRequest) (*extauth.Client, bool) {
	var client *extauth.Client
	if extauth.ValidClientIDFormat(req.clientID) {
		c, err := h.clients.GetByClientID(ctx, req.clientID)
		if err != nil {
			slog.ErrorContext(ctx, "ext_token_client_lookup_failed", slog.String("error", err.Error()))
			writeTokenError(w, "server_error")
			return nil, false
		}
		client = c
	}

	hash := ""
	if client != nil {
		hash = client.SecretHash
	}
	secretOK := extauth.VerifySecret(hash, req.secret)

	reason := ""
	switch {
	case client == nil:
		reason = "unknown_client"
	case !secretOK:
		reason = "bad_secret"
	case !client.IsActive:
		reason = "inactive"
	case !req.ip.IsValid() || !client.AllowsIP(req.ip):
		reason = "ip_not_allowed"
	}
	if reason == "" {
		return client, true
	}

	if err := h.failures.RecordFailure(ctx, req.clientID, req.ipKey); err != nil {
		slog.ErrorContext(ctx, "ext_token_rate_unavailable", slog.String("error", err.Error()))
		writeTokenError(w, "service_unavailable")
		return nil, false
	}
	logDenied(ctx, req, reason)
	writeTokenError(w, "invalid_client")
	return nil, false
}

// issue covers steps 7–8 for an authenticated client.
func (h *TokenHandler) issue(ctx context.Context, w http.ResponseWriter, req tokenRequest, client *extauth.Client) {
	// The client is verified: client_id is now trusted for the request log.
	if lf := custommw.LogFieldsFrom(ctx); lf != nil {
		lf.ClientID = client.ClientID
	}

	scopes, ok := grantScopes(req.scope, *client)
	if !ok {
		logDenied(ctx, req, "invalid_scope")
		writeTokenError(w, "invalid_scope")
		return
	}

	token, expiresIn, err := h.tokens.Issue(*client, scopes)
	if err != nil {
		logIssueFailed(ctx, client.ClientID, err)
		writeTokenError(w, "server_error")
		return
	}
	// Only to read the jti for the log; the token was just signed here.
	claims, err := h.tokens.Validate(token)
	if err != nil {
		logIssueFailed(ctx, client.ClientID, err)
		writeTokenError(w, "server_error")
		return
	}
	// A stale per-client counter only locks the client out sooner, so a reset
	// failure is logged and the valid token is still returned.
	if err := h.failures.ResetClient(ctx, client.ClientID); err != nil {
		slog.WarnContext(ctx, "ext_token_rate_reset_failed", slog.String("client_id", client.ClientID), slog.String("error", err.Error()))
	}

	scope := strings.Join(scopes, " ")
	slog.InfoContext(ctx, "ext_token_issued",
		slog.String("client_id", client.ClientID),
		slog.String("ip", req.ipKey),
		slog.String("scope", scope),
		slog.String("jti", claims.ID))

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	//nolint:gosec // G117: returning the access token is this endpoint's purpose (RFC 6749 §5.1)
	_ = json.NewEncoder(w).Encode(tokenResponse{ // headers already sent
		AccessToken: token,
		TokenType:   "Bearer",
		ExpiresIn:   expiresIn,
		Scope:       scope,
	})
}

// grantScopes is step 7: no scope parameter grants every scope the client
// holds; otherwise every requested scope must be known and held (Req 2.2–2.3).
func grantScopes(param string, c extauth.Client) ([]string, bool) {
	requested, err := extauth.ParseScopes(param)
	if err != nil {
		return nil, false
	}
	if requested == nil {
		granted := slices.DeleteFunc(slices.Clone(c.Scopes), func(s string) bool { return !extauth.ValidScope(s) })
		return granted, len(granted) > 0
	}
	for _, s := range requested {
		if !c.HasScope(s) {
			return nil, false
		}
	}
	return requested, true
}

// logDenied logs a refused token request. client_id is logged only when it has
// the client_id format: a caller who swapped the Basic fields would otherwise
// put the secret in the log.
func logDenied(ctx context.Context, req tokenRequest, reason string) {
	attrs := []any{slog.String("ip", req.ipKey), slog.String("reason", reason)}
	if extauth.ValidClientIDFormat(req.clientID) {
		attrs = append(attrs, slog.String("client_id", req.clientID))
	}
	slog.WarnContext(ctx, "ext_token_denied", attrs...)
}

func logIssueFailed(ctx context.Context, clientID string, err error) {
	slog.ErrorContext(ctx, "ext_token_issue_failed", slog.String("client_id", clientID), slog.String("error", err.Error()))
}

// parseRemoteIP parses r.RemoteAddr: a bare IP (set by TrustedRealIP) or
// ip:port.
func parseRemoteIP(addr string) (netip.Addr, bool) {
	if ap, err := netip.ParseAddrPort(addr); err == nil {
		return ap.Addr().Unmap(), true
	}
	ip, err := netip.ParseAddr(addr)
	if err != nil {
		return netip.Addr{}, false
	}
	return ip.Unmap(), true
}

// writeTokenError writes an RFC 6749 §5.2 body. invalid_client also carries
// the Basic challenge (Req 2.4).
func writeTokenError(w http.ResponseWriter, code string) {
	e, ok := tokenErrors[code]
	if !ok {
		code, e = "server_error", tokenErrors["server_error"]
	}
	if code == "invalid_client" {
		w.Header().Set("WWW-Authenticate", basicChallenge)
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(e.status)
	_ = json.NewEncoder(w).Encode(map[string]string{"error": code, "error_description": e.desc}) // headers already sent
}
