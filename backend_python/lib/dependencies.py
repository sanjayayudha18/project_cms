"""Auth dependency for protected endpoints (task 4.1). Fully generic -- reads
settings off request.app.state, no per-service import needed."""
from __future__ import annotations

import hmac
import logging

from fastapi import HTTPException, Request
from jose import JWTError, jwt

logger = logging.getLogger(__name__)


# EOD monitoring/retry is admin/app-support only (CLAUDE.md Sec 14). Role names as in Go's
# RequireRoles for admin routes; there is no separate app-support role today.
EOD_ROLES = frozenset({"ADMIN", "APPACCESS"})


def assert_auth_configured(settings) -> None:
    """Refuse to start with an empty or default secret (checked at startup, not at import)."""
    if settings.auth_secret in ("", "change_me"):
        raise RuntimeError("auth_secret is empty or 'change_me'; set the service's *_AUTH_SECRET")


def _authenticate(request: Request) -> tuple[str, str | None]:
    """Validate API key or JWT. Returns (identity, role); role is None in api_key mode
    (a service credential has no role)."""
    auth_header = request.headers.get("Authorization")
    if not auth_header:
        raise HTTPException(status_code=401, detail="Missing authentication credentials")

    scheme, _, token = auth_header.partition(" ")
    if not token:
        raise HTTPException(status_code=401, detail="Invalid authentication format")

    settings = request.app.state.settings
    if settings.auth_mode == "api_key":
        if not hmac.compare_digest(token.encode(), settings.auth_secret.encode()):
            raise HTTPException(status_code=401, detail="Invalid API key")
        return "api_key_user", None

    if scheme.lower() != "bearer":  # api_key mode stays lenient: Go sends DSR_RETRY_SCHEDULER_AUTH verbatim
        raise HTTPException(status_code=401, detail="Invalid authentication format")
    try:
        payload = jwt.decode(
            token, settings.auth_secret, algorithms=["HS256"],
            options={"verify_aud": False, "require_exp": True},
        )
    except JWTError:
        raise HTTPException(status_code=401, detail="Invalid or expired token")
    # Go refresh tokens carry `id` but no `role`; only access tokens may call this API.
    if not payload.get("role") or token_identity(payload) == "unknown":
        raise HTTPException(status_code=401, detail="Access token required")
    return token_identity(payload), str(payload["role"])


async def require_auth(request: Request) -> str:
    """Validate API key or JWT on protected endpoints. Returns the user identity."""
    return _authenticate(request)[0]


async def require_eod_role(request: Request) -> str:
    """require_auth + role gate (Sec 5: RBAC per route). Returns the user identity."""
    identity, role = _authenticate(request)
    if role is not None and role not in EOD_ROLES:
        raise HTTPException(status_code=403, detail="Insufficient role")
    return identity


def token_identity(payload: dict) -> str:
    """`sub` when present; otherwise the numeric `id` claim that Go's
    pkg/auth AccessTokenClaims issues (it never sets `sub`)."""
    if payload.get("sub"):
        return str(payload["sub"])
    if payload.get("id") is not None:
        return str(payload["id"])
    return "unknown"


def extract_user_id(auth_result: str) -> str:
    """Get user identity for audit log. Never logs the token itself."""
    return auth_result


if __name__ == "__main__":
    # Smoke check: JWT round-trip and identity extraction (no HTTP involved).
    secret = "test_secret"
    token = jwt.encode({"sub": "user@company.co.id"}, secret, algorithm="HS256")
    payload = jwt.decode(token, secret, algorithms=["HS256"], options={"verify_aud": False})
    assert extract_user_id(token_identity(payload)) == "user@company.co.id"
    # Go-issued access token shape: numeric `id`, no `sub`
    go_token = jwt.encode({"id": 42, "username": "john.admin", "role": "ADMIN"}, secret, algorithm="HS256")
    go_payload = jwt.decode(go_token, secret, algorithms=["HS256"], options={"verify_aud": False})
    assert token_identity(go_payload) == "42"
    assert token_identity({}) == "unknown"
    try:
        assert_auth_configured(type("S", (), {"auth_secret": "change_me"})())
    except RuntimeError:
        pass
    else:
        raise AssertionError("default secret must be refused")
    print("dependencies.py demo OK")
