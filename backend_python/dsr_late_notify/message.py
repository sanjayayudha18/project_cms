"""Notification text for the 09:00 DSR-late job (dsr-late-report spec FR5.3-5.5). Pure, no DB."""
from __future__ import annotations

from datetime import date
from email.utils import parseaddr

# notifications CHECKs (migration 025) = Go notification.maxTitleLen / maxBodyLen.
MAX_TITLE = 120
MAX_BODY = 1000
# Go notification.emailFooter: keep identical.
EMAIL_FOOTER = "\n\n--\nPesan otomatis CROWN Cash Management System. Mohon tidak membalas email ini."
INTERNAL_LINK = "/dsr-late-report"


def fit_list(head: str, items: list[str], tail: str, limit: int = MAX_BODY) -> str:
    """head + comma-joined items + tail within limit; items that do not fit become "dan N lainnya"."""
    for shown in range(len(items), -1, -1):
        rest = len(items) - shown
        listed = ", ".join(items[:shown])
        if rest:
            listed = f"{listed} dan {rest} lainnya" if listed else f"{rest} cabang"
        text = f"{head}{listed}{tail}"
        if len(text) <= limit:
            return text
    return f"{head}{len(items)} cabang{tail}"[:limit]


def vendor_message(report_date: date, branches: list[tuple[str, str]]) -> tuple[str, str]:
    """One notification per vendor recipient listing their branches still without a DSR."""
    d = report_date.strftime("%d/%m/%Y")
    title = f"DSR {d} belum diterima ({len(branches)} cabang)"[:MAX_TITLE]
    body = fit_list(f"DSR tanggal {d} belum diterima sampai batas 09:00 WIB untuk cabang: ",
                    [f"{code} - {name}" for code, name in branches],
                    ". Mohon segera unggah DSR. Keterlambatan dicatat sebagai dasar penalti FLM.")
    return title, body


def internal_message(report_date: date, per_vendor: list[tuple[str, int]]) -> tuple[str, str]:
    """ATM-USER summary (FR5.4): branch + vendor count, link to the report screen."""
    d = report_date.strftime("%d/%m/%Y")
    total = sum(n for _, n in per_vendor)
    title = f"DSR {d}: {total} cabang belum kirim"[:MAX_TITLE]
    body = fit_list(f"Sampai 09:00 WIB, {total} cabang dari {len(per_vendor)} vendor belum mengirim DSR "
                    f"tanggal {d}: ", [f"{name} ({n} cabang)" for name, n in per_vendor],
                    ". Detail di Laporan DSR Telat.")
    return title, body


def normalize_email(raw: str | None) -> str | None:
    """Lowercased bare address or None, like Go notification.normalizeEmail (blocks CR/LF too)."""
    addr = (raw or "").strip().lower()
    if not addr or any(c in addr for c in "\r\n<>, ") or addr.count("@") != 1:
        return None
    local, domain = addr.split("@")
    if not local or not domain or parseaddr(addr)[1] != addr:
        return None
    return addr
