"""Unit tests for dsr_late_notify.message (dsr-late-report FR5.5). No DB.

Run from backend_python/:  python -m unittest discover -s dsr_late_notify -t .
"""
from __future__ import annotations

import unittest
from datetime import date

from dsr_late_notify.message import (
    MAX_BODY,
    MAX_TITLE,
    fit_list,
    internal_message,
    normalize_email,
    vendor_message,
)

D = date(2026, 10, 27)


class MessageTest(unittest.TestCase):
    def test_vendor_message_lists_branches(self):
        title, body = vendor_message(D, [("B1", "Cabang Satu"), ("B2", "Cabang Dua")])
        self.assertEqual(title, "DSR 27/10/2026 belum diterima (2 cabang)")
        self.assertIn("B1 - Cabang Satu, B2 - Cabang Dua.", body)
        self.assertIn("09:00 WIB", body)

    def test_long_list_is_cut_with_dan_n_lainnya(self):
        branches = [(f"BR{i:04d}", "Cabang dengan nama yang cukup panjang") for i in range(200)]
        title, body = vendor_message(D, branches)
        self.assertLessEqual(len(body), MAX_BODY)
        self.assertLessEqual(len(title), MAX_TITLE)
        shown = body.count("BR")
        self.assertIn(f" dan {200 - shown} lainnya.", body)
        self.assertGreater(shown, 0)

    def test_fit_list_falls_back_to_count(self):
        self.assertEqual(fit_list("H: ", ["x" * 50], ".", limit=20), "H: 1 cabang.")
        self.assertEqual(fit_list("H: ", ["a", "b"], ".", limit=100), "H: a, b.")

    def test_internal_message_counts(self):
        title, body = internal_message(D, [("Vendor A", 3), ("Vendor B", 1)])
        self.assertEqual(title, "DSR 27/10/2026: 4 cabang belum kirim")
        self.assertIn("4 cabang dari 2 vendor", body)
        self.assertIn("Vendor A (3 cabang), Vendor B (1 cabang)", body)

    def test_normalize_email(self):
        cases = {
            " A.B@Vendor.TEST ": "a.b@vendor.test",
            "": None, None: None, "no-at": None, "a@b@c": None,
            "x@y.test\r\nBcc: z@evil.test": None, "Name <x@y.test>": None, "@y.test": None,
        }
        for raw, want in cases.items():
            with self.subTest(raw=raw):
                self.assertEqual(normalize_email(raw), want)


if __name__ == "__main__":
    unittest.main()
