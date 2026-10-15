"""Pure classification rules (.claude/sdlc/replenishment-realisasi/spec.md FR3.3-3.6). No DB.

Workday = Monday-Friday not in the active `holidays` set. A fill is "in the window" of a ticket
when it is at most 2 workdays from the schedule; the window closes the day after the 2nd workday
past the schedule.
"""
from __future__ import annotations

from dataclasses import dataclass
from datetime import date, time, timedelta

WINDOW_WORKDAYS = 2


@dataclass(frozen=True)
class Ticket:
    id: int
    terminal_id: str
    ticket_number: str
    scheduled: date


@dataclass(frozen=True)
class Fill:
    """One itm_replenish row (a physical fill reported by the ATM)."""

    id: int
    terminal_id: str
    date: date
    time: time


def is_workday(d: date, holidays: set[date]) -> bool:
    return d.weekday() < 5 and d not in holidays


def add_workdays(d: date, n: int, holidays: set[date]) -> date:
    """The n-th workday after d (before d when n < 0)."""
    step = timedelta(days=1 if n >= 0 else -1)
    left = abs(n)
    while left:
        d += step
        if is_workday(d, holidays):
            left -= 1
    return d


def workday_offset(scheduled: date, actual: date, holidays: set[date]) -> int:
    """Workdays from the schedule to the fill; negative = early. Non-workdays don't count."""
    if actual > scheduled:
        days = (scheduled + timedelta(days=i) for i in range(1, (actual - scheduled).days + 1))
        return sum(is_workday(d, holidays) for d in days)
    days = (actual + timedelta(days=i) for i in range((scheduled - actual).days))
    return -sum(is_workday(d, holidays) for d in days)


def window_closed(scheduled: date, today: date, holidays: set[date]) -> bool:
    return today > add_workdays(scheduled, WINDOW_WORKDAYS, holidays)


def classify_offset(offset: int) -> str:
    if offset == 0:
        return "on_schedule"
    return "early" if offset < 0 else "late"


def match_fills(tickets: list[Ticket], fills: list[Fill], holidays: set[date]) -> dict[int, Fill]:
    """ticket id -> its fill. Nearest to the schedule first (tie: earlier date, then earlier time);
    each fill serves one ticket, the ticket scheduled nearest wins (tie: smaller ticket number)."""
    by_terminal: dict[str, list[Fill]] = {}
    for f in fills:
        by_terminal.setdefault(f.terminal_id, []).append(f)
    pairs = []
    for t in tickets:
        for f in by_terminal.get(t.terminal_id, []):
            off = workday_offset(t.scheduled, f.date, holidays)
            if abs(off) <= WINDOW_WORKDAYS:
                pairs.append((abs(off), f.date, f.time, t.ticket_number, t.id, f))
    pairs.sort(key=lambda p: p[:5])
    matched: dict[int, Fill] = {}
    used: set[int] = set()
    for *_, ticket_id, f in pairs:
        if ticket_id not in matched and f.id not in used:
            matched[ticket_id] = f
            used.add(f.id)
    return matched


def amount_rows(
    order: dict[int, int], itm: dict[int, int], vendor: dict[int, int] | None,
) -> tuple[list[tuple[int, int, int, int | None]], bool]:
    """(denom, order, itm, vendor) per denom with a non-zero order or ITM amount, high to low,
    plus whether any ITM amount differs from the order (no tolerance, Q11)."""
    denoms = sorted({d for d, v in order.items() if v} | {d for d, v in itm.items() if v}, reverse=True)
    rows = [(d, order.get(d, 0), itm.get(d, 0), (vendor or {}).get(d)) for d in denoms]
    return rows, any(o != i for _, o, i, _ in rows)


def is_vendor_mismatch(fill_date: date | None, itm: dict[int, int], vendor_date: date | None,
                       vendor: dict[int, int]) -> bool:
    """A submitted vendor report disagrees with ITM: filled vs not, another date, or another amount (FR3.6)."""
    if (vendor_date is None) != (fill_date is None):
        return True
    if fill_date is None:
        return False
    return vendor_date != fill_date or any(vendor.get(d, 0) != itm.get(d, 0) for d in set(itm) | set(vendor))


def demo() -> None:
    mon = date(2026, 10, 12)
    assert workday_offset(mon, mon + timedelta(days=7), set()) == 5
    assert classify_offset(workday_offset(mon, mon + timedelta(days=1), set())) == "late"


if __name__ == "__main__":
    demo()
