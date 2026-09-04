import tempfile
import unittest
from datetime import date
from decimal import Decimal
from pathlib import Path

import openpyxl

from dsr_etl import (
    _rencana_isi_subtotal,
    cell_number,
    parse_upload_filename,
    read_daily_rows,
    read_rencana_isi_rows,
)


def _build_synthetic_workbook(path: Path) -> None:
    """A minimal single-location Daily + Rencana Isi workbook, shaped like the real
    DSR_DATA samples (label/value in separate cells, dynamic denom-column header)."""
    wb = openpyxl.Workbook()

    daily = wb.active
    daily.title = "Daily"
    daily["D1"] = "Kepada   :"
    daily["E1"] = "Ibu. Test"
    daily["D2"] = "Bank   :"
    daily["E2"] = "BANK CIMB NIAGA - NCC"
    daily["D5"] = "Tanggal   :"
    daily["E5"] = date(2026, 7, 15)
    daily["A8"] = "SALDO HARIAN ATM"
    daily["A10"] = "Tanggal"
    daily["C10"] = "Uraian"
    daily["F10"] = "Deno (lembar)"
    daily["K10"] = "Total Rupiah x1.000"
    daily["F11"] = 100000
    daily["G11"] = 50000
    daily["A12"] = date(2026, 7, 15)
    daily["C12"] = "SALDO AWAL"
    daily["F12"] = 1000000
    daily["G12"] = 500000
    daily["K12"] = 1500000
    daily["C13"] = "Penerimaan"
    daily["D13"] = "Dari CIMB Niaga CIT"
    daily["F13"] = 0
    daily["G13"] = 100000
    daily["K13"] = 100000
    daily["C14"] = "Subtotal Penerimaan"
    daily["F14"] = 0
    daily["G14"] = 100000
    daily["K14"] = 100000
    # openpyxl auto-types a literal "#REF!" string as an Excel error cell (reads back
    # blank via pandas, not as text), and "N/A" is one of pandas' default NaN sentinels
    # -- both workbook-authoring quirks of this test harness, not something xlrd does for
    # the real legacy .xls files. "GARBLED" exercises the same unparseable-cell -> NULL +
    # error_count code path without tripping either quirk.
    daily["C15"] = "Pengeluaran"
    daily["D15"] = "GARBLED"
    daily["F15"] = "GARBLED"
    daily["G15"] = 0
    daily["K15"] = 0
    daily["C16"] = "Subtotal Pengeluaran"
    daily["F16"] = 0
    daily["G16"] = 0
    daily["K16"] = 0
    daily["C17"] = "SALDO AKHIR SAMPAI PUKUL 00:00"
    daily["F17"] = 1000000
    daily["G17"] = 600000
    daily["K17"] = 1600000

    rencana = wb.create_sheet("Rencana Isi")
    rencana["A1"] = "RENCANA PENGISIAN"
    rencana["A2"] = "Tanggal :"
    rencana["B2"] = date(2026, 7, 16)
    rencana["A3"] = "ATM ID"
    rencana["B3"] = "Lokasi ATM"
    rencana["C3"] = "50/100"
    rencana["D3"] = "Jumlah Isi Denom 100rb (x 1.000)"
    rencana["E3"] = "Jumlah Isi Denom 50rb (x 1.000)"
    rencana["F3"] = "Saldo Splank Pukul 08:00"
    rencana["G3"] = "Keterangan"
    rencana["A4"] = "1234"
    rencana["B4"] = "TEST LOCATION"
    rencana["C4"] = "100"
    rencana["D4"] = 400000
    rencana["F4"] = 350000
    rencana["A5"] = "Sub Total"
    rencana["D5"] = 400000

    wb.save(path)


class DsrEtlParserTests(unittest.TestCase):
    def setUp(self) -> None:
        self._tmpdir = tempfile.TemporaryDirectory()
        self.path = Path(self._tmpdir.name) / "VENDOR__1__Laporan Test.xlsx"
        _build_synthetic_workbook(self.path)

    def tearDown(self) -> None:
        self._tmpdir.cleanup()

    def test_parses_upload_filename_with_user_id(self) -> None:
        vendor_code, user_id, original = parse_upload_filename("VENDOR__1__Laporan Test.xlsx")
        self.assertEqual("VENDOR", vendor_code)
        self.assertEqual(1, user_id)
        self.assertEqual("Laporan Test.xlsx", original)

    def test_parses_upload_filename_without_user_id(self) -> None:
        vendor_code, user_id, original = parse_upload_filename("VENDOR__Laporan Test.xlsx")
        self.assertEqual("VENDOR", vendor_code)
        self.assertIsNone(user_id)
        self.assertEqual("Laporan Test.xlsx", original)

    def test_cell_number_blank_is_zero_not_error(self) -> None:
        amount, is_error = cell_number(None)
        self.assertEqual(Decimal(0), amount)
        self.assertFalse(is_error)

    def test_cell_number_ref_error_is_null_and_flagged(self) -> None:
        amount, is_error = cell_number("#REF!")
        self.assertIsNone(amount)
        self.assertTrue(is_error)

    def test_read_daily_rows_header_and_leaf_lines(self) -> None:
        fields, rows, error_count = read_daily_rows(self.path)

        self.assertEqual(date(2026, 7, 15), fields["report_date"])
        self.assertEqual("BANK CIMB NIAGA - NCC", fields["bank"])

        # Subtotal / SALDO AKHIR rows must not be stored (they're derived).
        labels = [r["line_label"] for r in rows]
        self.assertNotIn("Subtotal Penerimaan", labels)
        self.assertNotIn("SALDO AKHIR SAMPAI PUKUL 00:00", labels)

        saldo_awal = next(r for r in rows if r["flow"] == "saldo_awal")
        self.assertEqual(Decimal("1000000"), saldo_awal["denom_100k"])
        self.assertEqual(Decimal("500000"), saldo_awal["denom_50k"])

        pengeluaran = next(r for r in rows if r["flow"] == "pengeluaran")
        self.assertIsNone(pengeluaran["denom_100k"])  # unparseable cell -> NULL
        self.assertEqual(1, error_count)

    def test_read_rencana_isi_rows_excludes_sub_total(self) -> None:
        plan_date, rows = read_rencana_isi_rows(self.path)

        self.assertEqual(date(2026, 7, 16), plan_date)
        self.assertEqual(1, len(rows))
        self.assertEqual("1234", rows[0]["atm_terminal_id"])
        self.assertEqual(Decimal("400000"), rows[0]["fill_100k_idr"])
        self.assertEqual(Decimal("350000"), rows[0]["splank_balance_0800_idr"])

    def test_rencana_isi_subtotal_sums_both_fill_columns(self) -> None:
        # Recomputed, never read from the sheet's own derived Sub Total row --
        # this is the value cross-checked against Daily d1 Subtotal Pengeluaran.
        rows = [
            {"fill_100k_idr": Decimal("150000"), "fill_50k_idr": Decimal("100000")},
            {"fill_100k_idr": Decimal(0), "fill_50k_idr": Decimal("250000")},
            {"fill_100k_idr": None, "fill_50k_idr": None},
        ]
        self.assertEqual(Decimal("500000"), _rencana_isi_subtotal(rows))
        self.assertIsNone(_rencana_isi_subtotal([]))


if __name__ == "__main__":
    unittest.main()
