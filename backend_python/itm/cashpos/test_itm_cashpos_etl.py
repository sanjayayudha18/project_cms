import unittest
from datetime import date
from decimal import Decimal

from itm_cashpos_etl import extract_file_date, parse_csv_row


class CashposParserTests(unittest.TestCase):
    def test_extracts_indonesian_filename_date(self) -> None:
        self.assertEqual(
            date(2026, 8, 21),
            extract_file_date("ATM_Cashpos_21_Agu_2026.csv"),
        )

    def test_rejects_invalid_filename(self) -> None:
        self.assertIsNone(extract_file_date("ATM_Replenish_21_Agu_2026.csv"))

    def test_maps_csv_columns_and_defaults_missing_denominations(self) -> None:
        row = {
            "ATMID": "A002      ",
            "TYPEATMCRM": "ATM100K",
            "TELLERID": "Z4CB      ",
            "CABANG": "47007",
            "STARTINGCASH50K": "",
            "CASHIN50K": "0",
            "CASHOUT50K": "0",
            "CASPOSDENOM50K": "0",
            "STARTINGCASH100K": "30000000000",
            "CASHIN100K": "0",
            "CASHOUT100K": "11380000000",
            "CASPOSDENOM100K": "18620000000",
            "POSITION_SOURCE": "REPLENISH",
        }

        parsed = parse_csv_row(row, 2, date(2026, 8, 21))

        self.assertEqual("A002", parsed["terminal_id"])
        self.assertEqual(Decimal("0"), parsed["starting_cash_10k"])
        self.assertEqual(Decimal("30000000000"), parsed["starting_cash_100k"])
        self.assertEqual("REPLENISH", parsed["position_source"])


if __name__ == "__main__":
    unittest.main()
