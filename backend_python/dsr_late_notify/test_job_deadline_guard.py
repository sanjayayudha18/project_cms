"""run_once refuses to run before the 09:00 WIB deadline of the report date (review R2). No DB needed."""
from __future__ import annotations

import unittest
from contextlib import asynccontextmanager
from datetime import date, datetime
from unittest import mock

from dsr_late_notify import job

DAY = date(2026, 10, 27)


class _FakePool:
    @asynccontextmanager
    async def acquire(self):
        conn = mock.MagicMock()
        conn.transaction = mock.MagicMock()
        conn.transaction.return_value.__aenter__ = mock.AsyncMock()
        conn.transaction.return_value.__aexit__ = mock.AsyncMock(return_value=False)
        yield conn


class DeadlineGuardTest(unittest.IsolatedAsyncioTestCase):
    async def test_before_deadline_is_refused_and_writes_nothing(self):
        with mock.patch.object(job, "notify", new=mock.AsyncMock()) as notify, \
                mock.patch.object(job, "_audit", new=mock.AsyncMock()) as audit:
            with self.assertRaises(ValueError):
                await job.run_once(_FakePool(), DAY, date(2026, 10, 1), trigger="manual",
                                   as_of=datetime.fromisoformat("2026-10-27T08:59:59+07:00"))
        notify.assert_not_called()
        audit.assert_not_called()

    async def test_at_deadline_runs(self):
        summary = job.Summary(status="sent")
        with mock.patch.object(job, "notify", new=mock.AsyncMock(return_value=summary)) as notify:
            got = await job.run_once(_FakePool(), DAY, date(2026, 10, 1), trigger="auto",
                                     as_of=datetime.fromisoformat("2026-10-27T09:00:00+07:00"))
        self.assertIs(summary, got)
        notify.assert_awaited_once()

    async def test_deadline_is_judged_in_wib_not_the_callers_offset(self):
        with self.assertRaises(ValueError):  # 01:59:59 UTC == 08:59:59 WIB
            await job.run_once(_FakePool(), DAY, date(2026, 10, 1), trigger="manual",
                               as_of=datetime.fromisoformat("2026-10-27T01:59:59+00:00"))


if __name__ == "__main__":
    unittest.main()
