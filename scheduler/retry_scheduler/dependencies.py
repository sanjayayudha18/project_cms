"""Auth dependency for protected endpoints (task 4.1)."""
from __future__ import annotations

import logging

from fastapi import HTTPException, Request
from jose import JWTError, jwt

logger = logging.getLogger(__name__)


async def require_auth(request: Request) -> str:
    """Validate API key or JWT on protected endpoints. Returns the user identity."""
    auth_header = request.headers.get("Authorization")
    if not auth_header:
        raise HTTPException(status_code=401, detail="Missing authentication credentials")

    scheme, _, token = auth_header.partition(" ")
    if not token:
        raise HTTPException(status_code=401, detail="Invalid authentication format")

    settings = request.app.state.settings
    if settings.auth_mode == "api_key":
        if token != settings.auth_secret:
            raise HTTPException(status_code=401, detail="Invalid API key")
        return "api_key_user"

    try:
        payload = jwt.decode(
            token, settings.auth_secret, algorithms=["HS256"],
            options={"verify_aud": False},
        )
    except JWTError:
        raise HTTPException(status_code=401, detail="Invalid or expired token")
    return payload.get("sub", "unknown")


def extract_user_id(auth_result: str) -> str:
    """Get user identity for audit log. Never logs the token itself."""
    return auth_result


if __name__ == "__main__":
    # Smoke check: JWT round-trip and identity extraction (no HTTP involved).
    secret = "test_secret"
    token = jwt.encode({"sub": "user@company.co.id"}, secret, algorithm="HS256")
    payload = jwt.decode(token, secret, algorithms=["HS256"], options={"verify_aud": False})
    assert extract_user_id(payload.get("sub", "unknown")) == "user@company.co.id"
    print("dependencies.py demo OK")
