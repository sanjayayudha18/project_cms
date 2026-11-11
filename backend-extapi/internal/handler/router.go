package handler

import (
	"context"
	"log/slog"
	"net/http"
	"net/netip"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"

	"github.com/cimb-niaga/cms/backend-extapi/internal/extauth"
	"github.com/cimb-niaga/cms/pkg/health"
	custommw "github.com/cimb-niaga/cms/pkg/middleware"
)

const (
	requestTimeout = 30 * time.Second
	readyTimeout   = 2 * time.Second
	maxRequestBody = 1 << 20
)

// Deps is everything NewRouter needs; main builds it, tests can fake it.
type Deps struct {
	TrustedProxies []netip.Prefix
	Tokens         *extauth.TokenService
	Clients        extauth.ClientReader
	Failures       *extauth.TokenFailures
	Rate           *extauth.APIRate
	Users          UserService
	ReadyChecks    map[string]func(context.Context) error
}

// NewRouter builds the whole backend-extapi route table. Only /health, /ready
// and /api/ext/v1/* may exist here (Property 13).
func NewRouter(d Deps) http.Handler {
	r := chi.NewRouter()
	r.Use(custommw.TrustedRealIP(d.TrustedProxies))
	r.Use(middleware.RequestID)
	r.Use(custommw.RequestLogger(slog.Default()))
	r.Use(middleware.Recoverer)
	r.Use(middleware.Timeout(requestTimeout))
	r.Use(custommw.BodyLimit(maxRequestBody, nil))

	r.NotFound(func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusNotFound, apiError{Error: "not_found", Message: "route not found"})
	})
	r.MethodNotAllowed(func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusMethodNotAllowed, apiError{Error: "method_not_allowed", Message: "method not allowed"})
	})

	r.Get("/health", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})
	r.Get("/ready", health.Ready(readyTimeout, d.ReadyChecks))

	tokenH := NewTokenHandler(d.Tokens, d.Clients, d.Failures)
	userH := NewUserHandler(d.Users)
	authMW := extauth.RequireClient(d.Tokens, d.Clients, d.Rate)
	read := extauth.RequireScope(extauth.ScopeUsersRead)
	write := extauth.RequireScope(extauth.ScopeUsersWrite)

	r.Route("/api/ext/v1", func(r chi.Router) {
		r.Post("/oauth/token", tokenH.Token)
		r.Group(func(r chi.Router) {
			r.Use(authMW)
			r.With(write).Post("/users", userH.Create)
			r.With(read).Get("/users", userH.FindByUsername)
			r.With(read).Get("/users/{id}", userH.Get)
			r.With(write).Put("/users/{id}", userH.Update)
			r.With(write).Delete("/users/{id}", userH.Disable)
			r.With(write).Post("/users/{id}/enable", userH.Enable)
		})
	})
	return r
}
