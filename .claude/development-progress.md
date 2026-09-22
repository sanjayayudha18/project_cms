# Development Progress — CMS2 Cash Management System

**Last Updated:** 2026-09-21

## Completed Features

### User Session Lifetime (Absolute 1 Hour, Parameterized)
- **Status:** ✅ COMPLETE
- **Branch:** dev1
- **PR:** Pending
- **Completed Tasks:**
  - [x] T1: Configuration layer (SESSION_MAX_LIFETIME env var, 1h default, min 5m guard)
  - [x] T2: Token service (session deadline inheritance, deadline clamping, access token cap)
  - [x] T3: Handler integration (rotation → deadline inheritance, cookie MaxAge dynamic, session-end clear)
  - [x] T4: Frontend (CompanyPortal session notice via query param, VendorPortal error display)
  - [x] T5: Tests and closure (83.9% coverage, all suites green)

**What Changed:**
- Sessions are now capped at 1 hour after login, regardless of activity.
- Refresh token rotation inherits the original deadline instead of extending it.
- A shortened `SESSION_MAX_LIFETIME` (e.g., 30m) takes effect immediately on next refresh.
- On session expiry, both portals redirect to login with a specific notice.
- Test: `.claude/sdlc/user-session/tests.md` (manual 2m real-time validation).

**Files Modified:**
- Backend: `pkg/config` (SessionMaxLifetime), `pkg/auth/token_service` (RotateTokenPair + deadline clamping), `backend/internal/handler/auth_handler` (cookie MaxAge + session-end clear)
- Frontend: CompanyPortal/VendorPortal auth stores/contexts (redirect + notice)
- Tests: All test fixtures updated, +83.9% coverage on pkg/auth
- Config: `.env.example`, `.claude/CLAUDE.md` (env list)

**Not Changed:**
- Authentication mechanisms (LDAP/local) — transparent to this feature.
- Invoice, reconciliation, DSR, or other business logic modules.
- Database schema or migrations.

---

## Next Phase (Post-Session Cleanup)

1. **Idle Timeout** (separate feature): Warn at T-5 minutes, auto-logout if no interaction.
2. **Session Tracking** (optional): Audit event per session start/end for compliance.
3. **Force Logout Admin** (optional): In-app button to log out all sessions for a user.
