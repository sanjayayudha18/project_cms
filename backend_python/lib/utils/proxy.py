"""Trusted-proxy config for the EOD FastAPI services (same rule as Go QC F13).

uvicorn's ProxyHeadersMiddleware reads X-Forwarded-For right to left, skipping
trusted hops, but only when the connecting peer is in ``forwarded_allow_ips``.
Two of its defaults are unsafe for us, so they are overridden here:
- with nothing configured uvicorn still trusts 127.0.0.1 -> we turn proxy
  headers off entirely (client IP = connection address);
- a malformed entry is silently kept as a literal -> we fail at start instead.
"""
from __future__ import annotations

import ipaddress


def parse_trusted_proxies(raw: str) -> list[str]:
    """Comma-separated CIDRs or IPs -> normalized networks; blank entries skipped.

    Raises ValueError naming the bad entry.
    """
    out: list[str] = []
    for entry in (e.strip() for e in raw.split(",")):
        if not entry:
            continue
        try:
            out.append(str(ipaddress.ip_network(entry, strict=False)))
        except ValueError as exc:
            raise ValueError(f"TRUSTED_PROXIES: invalid entry {entry!r} (want CIDR or IP)") from exc
    return out


def uvicorn_proxy_kwargs(raw: str) -> dict:
    """kwargs for uvicorn.run: proxy headers only from the configured proxies."""
    trusted = parse_trusted_proxies(raw)
    if not trusted:
        return {"proxy_headers": False}
    return {"proxy_headers": True, "forwarded_allow_ips": trusted}
