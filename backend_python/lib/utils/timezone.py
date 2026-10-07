"""WIB (Asia/Jakarta) timezone helpers (task 5.2)."""
from __future__ import annotations

from datetime import date, datetime, time
from zoneinfo import ZoneInfo

WIB = ZoneInfo("Asia/Jakarta")


def current_processing_date() -> date:
    """Today's calendar date in WIB."""
    return datetime.now(WIB).date()


def is_past_sla(sla_time: time, processing_date: date) -> bool:
    """True if the current WIB time is past sla_time on processing_date."""
    now_wib = datetime.now(WIB)
    deadline = datetime.combine(processing_date, sla_time, tzinfo=WIB)
    return now_wib > deadline


def demo() -> None:
    from datetime import timedelta

    today = current_processing_date()
    assert isinstance(today, date)

    past_time = (datetime.now(WIB) - timedelta(hours=1)).time()
    future_time = (datetime.now(WIB) + timedelta(hours=1)).time()
    assert is_past_sla(past_time, today) is True
    assert is_past_sla(future_time, today) is False
    print("timezone.py demo OK")


if __name__ == "__main__":
    demo()
