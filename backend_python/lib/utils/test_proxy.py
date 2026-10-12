"""Trusted-proxy config for the EOD services (QC F13 rule, python follow-up).

Run from backend_python/:  python -m unittest lib.utils.test_proxy -v
"""
from __future__ import annotations

import asyncio
import unittest

from uvicorn.middleware.proxy_headers import ProxyHeadersMiddleware

from eod_retry_scheduler.config import Settings as RetrySettings
from lib.utils.proxy import parse_trusted_proxies, uvicorn_proxy_kwargs
from service_dsr_etl.config import Settings as DsrSettings


def client_seen(trusted_raw: str, peer: str, xff: str | None) -> str:
    """Client host the app sees behind uvicorn's middleware, configured like run.py."""
    seen: dict[str, str] = {}

    async def app(scope, receive, send):
        seen["host"] = scope["client"][0]

    kwargs = uvicorn_proxy_kwargs(trusted_raw)
    handler = ProxyHeadersMiddleware(app, trusted_hosts=kwargs["forwarded_allow_ips"]) if kwargs["proxy_headers"] else app
    headers = [(b"x-forwarded-for", xff.encode())] if xff is not None else []
    scope = {"type": "http", "client": (peer, 5555), "headers": headers}
    asyncio.run(handler(scope, None, None))
    return seen["host"]


class ParseTrustedProxiesTest(unittest.TestCase):
    def test_valid(self):
        self.assertEqual(parse_trusted_proxies(""), [])
        self.assertEqual(parse_trusted_proxies(" , ,"), [])
        self.assertEqual(
            parse_trusted_proxies(" 172.17.0.1 , 10.1.2.3/8,::1 "),
            ["172.17.0.1/32", "10.0.0.0/8", "::1/128"],
        )

    def test_invalid_entry_raises(self):
        for raw in ("172.17.0.1,nginx", "10.0.0.0/33"):
            with self.assertRaises(ValueError):
                parse_trusted_proxies(raw)

    def test_settings_fail_fast(self):
        for settings in (RetrySettings, DsrSettings):
            with self.assertRaises(ValueError):
                settings(trusted_proxies="not-an-ip")
            self.assertEqual(settings().trusted_proxies, "")

    def test_empty_disables_proxy_headers(self):
        # uvicorn would otherwise trust 127.0.0.1 by default.
        self.assertEqual(uvicorn_proxy_kwargs(""), {"proxy_headers": False})


class ClientIpTest(unittest.TestCase):
    def test_untrusted_peer_cannot_spoof(self):
        self.assertEqual(client_seen("172.17.0.1", "203.0.113.9", "1.2.3.4"), "203.0.113.9")

    def test_nothing_configured_ignores_xff_even_from_localhost(self):
        self.assertEqual(client_seen("", "127.0.0.1", "1.2.3.4"), "127.0.0.1")

    def test_trusted_nginx_uses_rightmost_untrusted(self):
        self.assertEqual(client_seen("172.17.0.1", "172.17.0.1", "1.2.3.4, 198.51.100.7"), "198.51.100.7")

    def test_lb_hop_skipped(self):
        self.assertEqual(
            client_seen("172.17.0.1,10.0.0.0/8", "172.17.0.1", "198.51.100.7, 10.1.2.3"), "198.51.100.7"
        )


if __name__ == "__main__":
    unittest.main()
