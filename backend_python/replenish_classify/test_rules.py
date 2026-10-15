"""Unit tests for replenish_classify.rules (replenishment-realisasi FR3.3-3.6), no DB.

Run from backend_python/:  python -m unittest replenish_classify.test_rules -v
"""
from __future__ import annotations

import unittest
from datetime import date, time

from replenish_classify.rules import (
    Fill,
    Ticket,
    add_workdays,
    amount_rows,
    classify_offset,
    is_vendor_mismatch,
    match_fills,
    window_closed,
    workday_offset,
)

# 2026-10-12 is a Monday; 2026-10-16 Friday; 17/18 weekend.
MON, TUE, WED, THU, FRI, SAT, SUN = (date(2026, 10, d) for d in range(12, 19))
NEXT_MON, NEXT_TUE = date(2026, 10, 19), date(2026, 10, 20)


class WorkdayTest(unittest.TestCase):
    def test_add_workdays_skips_weekend_and_holiday(self) -> None:
        self.assertEqual(add_workdays(THU, 2, set()), NEXT_MON)
        self.assertEqual(add_workdays(THU, 2, {FRI}), NEXT_TUE)
        self.assertEqual(add_workdays(NEXT_MON, -2, set()), THU)

    def test_offset(self) -> None:
        self.assertEqual(workday_offset(WED, WED, set()), 0)
        self.assertEqual(workday_offset(WED, FRI, set()), 2)
        self.assertEqual(workday_offset(FRI, NEXT_MON, set()), 1)  # weekend not counted
        self.assertEqual(workday_offset(FRI, NEXT_MON, {NEXT_MON}), 0)  # holiday not counted
        self.assertEqual(workday_offset(WED, MON, set()), -2)
        self.assertEqual(workday_offset(NEXT_MON, SAT, set()), 0)  # weekend fill before Monday schedule

    def test_window_closed(self) -> None:
        self.assertFalse(window_closed(THU, NEXT_MON, set()))  # last day of the window
        self.assertTrue(window_closed(THU, NEXT_TUE, set()))
        self.assertFalse(window_closed(THU, NEXT_TUE, {FRI}))  # holiday stretches the window

    def test_classify_offset(self) -> None:
        self.assertEqual(classify_offset(0), "on_schedule")
        self.assertEqual(classify_offset(-1), "early")
        self.assertEqual(classify_offset(2), "late")


class MatchTest(unittest.TestCase):
    def test_nearest_fill_wins_and_rest_unused(self) -> None:
        t = Ticket(id=1, terminal_id="A", ticket_number="A_100K_20261014_001", scheduled=WED)
        fills = [Fill(10, "A", MON, time(9)), Fill(11, "A", THU, time(9)), Fill(12, "A", WED, time(15))]
        self.assertEqual(match_fills([t], fills, set()), {1: fills[2]})

    def test_tie_prefers_earlier_date_then_time(self) -> None:
        t = Ticket(1, "A", "A_001", WED)
        early, late = Fill(10, "A", TUE, time(9)), Fill(11, "A", THU, time(9))
        self.assertEqual(match_fills([t], [late, early], set())[1], early)
        same_day = [Fill(20, "A", WED, time(14)), Fill(21, "A", WED, time(8))]
        self.assertEqual(match_fills([t], same_day, set())[1].id, 21)

    def test_one_fill_per_ticket_nearest_schedule_wins(self) -> None:
        t1, t2 = Ticket(1, "A", "A_001", TUE), Ticket(2, "A", "A_002", THU)
        fill = Fill(10, "A", WED, time(9))  # 1 workday from both: tie -> smaller ticket number
        self.assertEqual(match_fills([t2, t1], [fill], set()), {1: fill})
        closer = Fill(11, "A", THU, time(9))
        self.assertEqual(match_fills([t1, t2], [fill, closer], set()), {1: fill, 2: closer})

    def test_outside_window_or_other_terminal_ignored(self) -> None:
        t = Ticket(1, "A", "A_001", MON)
        self.assertEqual(match_fills([t], [Fill(10, "A", THU, time(9)), Fill(11, "B", MON, time(9))], set()), {})


class AmountTest(unittest.TestCase):
    def test_rows_and_diff(self) -> None:
        rows, diff = amount_rows({100000: 200_000_000}, {100000: 200_000_000, 50000: 0}, None)
        self.assertEqual(rows, [(100000, 200_000_000, 200_000_000, None)])
        self.assertFalse(diff)
        rows, diff = amount_rows({100000: 200_000_000}, {100000: 150_000_000, 50000: 10_000_000}, {100000: 200_000_000})
        self.assertEqual(rows, [(100000, 200_000_000, 150_000_000, 200_000_000), (50000, 0, 10_000_000, None)])
        self.assertTrue(diff)

    def test_vendor_mismatch(self) -> None:
        d, itm = date(2026, 10, 14), {100000: 600_000_000, 50000: 0}
        self.assertFalse(is_vendor_mismatch(d, itm, d, {100000: 600_000_000}))
        self.assertTrue(is_vendor_mismatch(d, itm, d, {100000: 610_000_000}))       # amount
        self.assertTrue(is_vendor_mismatch(d, itm, date(2026, 10, 15), {100000: 600_000_000}))  # date
        self.assertTrue(is_vendor_mismatch(None, {}, d, {100000: 1}))                # vendor filled, ITM did not
        self.assertTrue(is_vendor_mismatch(d, itm, None, {}))                        # vendor failed, ITM filled
        self.assertFalse(is_vendor_mismatch(None, {}, None, {}))


if __name__ == "__main__":
    unittest.main()
