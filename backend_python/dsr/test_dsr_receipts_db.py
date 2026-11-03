"""Integration tests for the dsr_receipts write in dsr_etl._upsert_upload
(dsr-late-report P2, migration 033). Skipped when DATABASE_URL is empty.

Run against a throwaway DB, never dev (build it with `go run ./cmd/migrate up`):
  cd backend_python && DATABASE_URL=postgres://...@localhost:5432/<tmp> \
      py -3 -m unittest dsr.test_dsr_receipts_db -v   (or discover -s dsr)
"""
from __future__ import annotations

import os
import tempfile
import unittest
import uuid
from datetime import date
from pathlib import Path
from unittest import mock

import openpyxl
import psycopg

import dsr_etl
from test_dsr_etl import _build_multi_vault_workbook

DATABASE_URL = os.environ.get("DATABASE_URL", "")
REPORT_DATE = date(2099, 1, 1)


@unittest.skipUnless(DATABASE_URL, "DATABASE_URL not set")
class DsrReceiptsDbTest(unittest.TestCase):
    def setUp(self) -> None:
        self.conn = psycopg.connect(DATABASE_URL)
        self.tmp = tempfile.TemporaryDirectory()
        self.label = f"test_rcpt_{uuid.uuid4().hex[:10]}"
        self.vendor_ids: list[int] = []

    def tearDown(self) -> None:
        self.conn.rollback()
        with self.conn.cursor() as cur:
            cur.execute("DELETE FROM public.dsr_uploads WHERE report_date = %s", (REPORT_DATE,))
            if self.vendor_ids:  # receipts are append-only by design; test-only cleanup, temp DB is dropped anyway
                cur.execute("DELETE FROM public.dsr_receipts WHERE vendor_id = ANY(%s)", (self.vendor_ids,))
                cur.execute("DELETE FROM public.vendors WHERE id = ANY(%s)", (self.vendor_ids,))
        self.conn.commit()  # dsr_receipts is append-only by design; the temp DB is dropped afterwards
        self.conn.close()
        self.tmp.cleanup()

    def workbook(self, name: str, tweak: int = 0) -> Path:
        path = Path(self.tmp.name) / name
        _build_multi_vault_workbook(path)
        wb = openpyxl.load_workbook(path)
        wb["Daily"]["E5"] = REPORT_DATE
        wb["Daily"]["F12"] = tweak  # different bytes -> different checksum
        wb.save(path)
        return path

    def vendor(self, code: str, name: str) -> int:
        with self.conn.cursor() as cur:
            cur.execute("INSERT INTO public.vendors (code, name) VALUES (%s, %s) RETURNING id", (code, name))
            vid = cur.fetchone()[0]
        self.conn.commit()
        self.vendor_ids.append(vid)
        return vid

    def ingest(self, path: Path, label: str | None = None, vendor_id: int | None = None) -> str:
        if vendor_id is None:
            vendor_id = self.vendor(f"RC{uuid.uuid4().hex[:8]}", label or self.label)
        return dsr_etl._upsert_upload(self.conn, path, label or self.label, vendor_id, None)

    def receipts(self, label: str | None = None) -> list[tuple]:
        with self.conn.cursor() as cur:
            cur.execute(
                "SELECT vendor_id, locations, received_at, checksum FROM public.dsr_receipts"
                " WHERE vendor_label = %s ORDER BY received_at, id",
                (label or self.label,),
            )
            return cur.fetchall()

    def uploads(self) -> int:
        with self.conn.cursor() as cur:
            cur.execute("SELECT count(*) FROM public.dsr_uploads WHERE vendor = %s", (self.label,))
            return cur.fetchone()[0]

    def test_commit_writes_one_receipt_with_locations_and_received_at(self) -> None:
        self.ingest(self.workbook("a.xlsx"))
        rows = self.receipts()
        self.assertEqual(1, len(rows))
        vendor_id, locations, received_at, checksum = rows[0]
        self.assertIsNotNone(vendor_id)
        self.assertEqual(["BINTARO", "LENTENG AGUNG"], sorted(locations))
        self.assertIsNotNone(received_at)
        self.assertEqual(64, len(checksum))

    def test_two_vendors_with_same_name_receipt_gets_id_of_the_code(self) -> None:
        """Review R1: vendors.name is not unique; the id must follow the filename's code."""
        first = self.vendor(f"RA{uuid.uuid4().hex[:8]}", self.label)
        second = self.vendor(f"RB{uuid.uuid4().hex[:8]}", self.label)
        for vid in (second, first):
            self.assertEqual(vid, dsr_etl.resolve_vendor(self.conn, self._code(vid))[0])
        self.ingest(self.workbook("a.xlsx"), vendor_id=second)
        self.assertEqual(second, self.receipts()[-1][0])

    def _code(self, vid: int) -> str:
        with self.conn.cursor() as cur:
            cur.execute("SELECT code FROM public.vendors WHERE id = %s", (vid,))
            return cur.fetchone()[0]

    def test_failure_after_receipt_rolls_back_receipt_and_upload(self) -> None:
        real = dsr_etl._insert_receipt

        def insert_then_fail(*args, **kwargs):
            real(*args, **kwargs)
            raise RuntimeError("boom")

        with mock.patch.object(dsr_etl, "_insert_receipt", insert_then_fail):
            with self.assertRaises(RuntimeError):
                self.ingest(self.workbook("a.xlsx"))
        self.conn.rollback()
        self.assertEqual([], self.receipts())
        self.assertEqual(0, self.uploads())

    def test_recommit_same_file_keeps_one_receipt(self) -> None:
        path = self.workbook("a.xlsx")
        self.ingest(path)
        self.assertEqual("skipped", self.ingest(path))
        self.assertEqual(1, len(self.receipts()))

    def test_revision_adds_row_and_first_received_at_is_kept(self) -> None:
        self.ingest(self.workbook("a.xlsx", tweak=1))
        first = self.receipts()[0][2]
        self.ingest(self.workbook("b.xlsx", tweak=2))
        rows = self.receipts()
        self.assertEqual(2, len(rows))
        self.assertNotEqual(rows[0][3], rows[1][3])
        self.assertEqual(first, min(r[2] for r in rows))
        self.assertEqual(1, self.uploads())  # upload row itself is replaced


if __name__ == "__main__":
    unittest.main()
