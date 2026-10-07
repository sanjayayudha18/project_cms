"""Ingest ITM ATM cash-position snapshot CSV files into CMS."""
from __future__ import annotations

import csv
import hashlib
import logging
import os
import shutil
import sys
from datetime import date, datetime
from decimal import Decimal, InvalidOperation
from pathlib import Path
from typing import Any

import psycopg
from dotenv import load_dotenv


PROJECT_ROOT = Path(__file__).resolve().parents[3]
load_dotenv(PROJECT_ROOT / "scheduler" / ".env")

INPUT_DIR = PROJECT_ROOT / "FTP_DATA" / "ITM" / "atm_caspos"
BACKUP_DIR = INPUT_DIR / "backups"
LOG_DIR = INPUT_DIR / "logs"
FILE_PATTERN = "ATM_Cashpos_*.csv"

_MONTH_MAP: dict[str, int] = {
    "january": 1, "januari": 1, "jan": 1,
    "february": 2, "februari": 2, "feb": 2,
    "march": 3, "maret": 3, "mar": 3,
    "april": 4, "apr": 4,
    "may": 5, "mei": 5,
    "june": 6, "juni": 6, "jun": 6,
    "july": 7, "juli": 7, "jul": 7,
    "august": 8, "agustus": 8, "agu": 8, "aug": 8,
    "september": 9, "sep": 9, "sept": 9,
    "october": 10, "oktober": 10, "oct": 10, "okt": 10,
    "november": 11, "nov": 11,
    "december": 12, "desember": 12, "dec": 12, "des": 12,
}

EXPECTED_HEADERS = [
    "ATMID",
    "TYPEATMCRM",
    "TELLERID",
    "CABANG",
    "STARTINGCASH50K",
    "CASHIN50K",
    "CASHOUT50K",
    "CASPOSDENOM50K",
    "STARTINGCASH100K",
    "CASHIN100K",
    "CASHOUT100K",
    "CASPOSDENOM100K",
    "POSITION_SOURCE",
]

_DENOMINATIONS = (10, 20, 50, 100)


def setup_logging() -> None:
    LOG_DIR.mkdir(parents=True, exist_ok=True)
    log_file = LOG_DIR / f"itm_cashpos_{datetime.now():%Y%m%d_%H%M%S}.log"
    logging.basicConfig(
        level=logging.INFO,
        format="%(asctime)s | %(levelname)s | %(message)s",
        handlers=[logging.FileHandler(log_file, encoding="utf-8"), logging.StreamHandler()],
    )


def compute_checksum(path: Path) -> str:
    """Return the SHA-256 checksum of a file."""
    checksum = hashlib.sha256()
    with path.open("rb") as source:
        for chunk in iter(lambda: source.read(8192), b""):
            checksum.update(chunk)
    return checksum.hexdigest()


def extract_file_date(filename: str) -> date | None:
    """Parse ATM_Cashpos_<day>_<month>_<year>.csv filenames."""
    parts = Path(filename).stem.split("_")
    if len(parts) < 5 or parts[0].upper() != "ATM" or parts[1].lower() != "cashpos":
        return None

    try:
        day = int(parts[2])
        month = _MONTH_MAP.get(parts[3].lower())
        year = int(parts[4])
        return date(year, month, day) if month is not None else None
    except (ValueError, IndexError):
        return None


def parse_decimal(raw: str | None) -> Decimal:
    """Parse an amount, treating blank CSV values as zero."""
    value = (raw or "").strip()
    if not value:
        return Decimal("0")
    try:
        return Decimal(value)
    except InvalidOperation as error:
        raise ValueError(f"Invalid decimal value: '{value}'") from error


def move_to_backup(path: Path) -> Path:
    """Move a successfully processed file into the input backup directory."""
    BACKUP_DIR.mkdir(parents=True, exist_ok=True)
    target = BACKUP_DIR / path.name
    if target.exists():
        target = BACKUP_DIR / f"{path.stem}_{datetime.now():%Y%m%d_%H%M%S}{path.suffix}"
    shutil.move(str(path), str(target))
    return target


def parse_csv_row(row: dict[str, str], row_num: int, cashpos_date: date) -> dict[str, Any]:
    """Convert one CSV row into values accepted by itm_cashpos."""
    errors: list[str] = []
    text_fields = {
        "terminal_id": row.get("ATMID", "").strip(),
        "machine_type": row.get("TYPEATMCRM", "").strip(),
        "teller_id": row.get("TELLERID", "").strip(),
        "branch_code": row.get("CABANG", "").strip(),
        "position_source": row.get("POSITION_SOURCE", "").strip(),
    }
    for name, value in text_fields.items():
        if not value:
            errors.append(f"{name} is empty")

    values: dict[str, Any] = {"cashpos_date": cashpos_date, **text_fields}
    for denomination in _DENOMINATIONS:
        suffix = f"{denomination}K"
        source_names = {
            "starting_cash": f"STARTINGCASH{suffix}",
            "cash_in": f"CASHIN{suffix}",
            "cash_out": f"CASHOUT{suffix}",
            "cash_position": f"CASPOSDENOM{suffix}",
        }
        for field, source_name in source_names.items():
            target_name = f"{field}_{denomination}k"
            try:
                values[target_name] = parse_decimal(row.get(source_name))
            except ValueError as error:
                errors.append(f"{source_name}: {error}")
                values[target_name] = Decimal("0")

    if errors:
        raise ValueError(f"Row {row_num}: {'; '.join(errors)}")
    return values


def read_csv_file(path: Path) -> tuple[list[dict[str, str]], list[str]]:
    """Read a cash-position CSV and validate its required headers."""
    with path.open("r", encoding="utf-8-sig", newline="") as source:
        reader = csv.DictReader(source)
        if reader.fieldnames is None:
            return [], ["CSV file has no header row"]

        actual_headers = {header.strip() for header in reader.fieldnames if header}
        missing = sorted(set(EXPECTED_HEADERS) - actual_headers)
        return list(reader), [f"Missing columns: {missing}"] if missing else []


INSERT_ROW_SQL = """
INSERT INTO itm_cashpos (
    file_id, cashpos_date, terminal_id, machine_type, teller_id, branch_code,
    starting_cash_10k, cash_in_10k, cash_out_10k, cash_position_10k,
    starting_cash_20k, cash_in_20k, cash_out_20k, cash_position_20k,
    starting_cash_50k, cash_in_50k, cash_out_50k, cash_position_50k,
    starting_cash_100k, cash_in_100k, cash_out_100k, cash_position_100k,
    position_source
) VALUES (
    %(file_id)s, %(cashpos_date)s, %(terminal_id)s, %(machine_type)s, %(teller_id)s, %(branch_code)s,
    %(starting_cash_10k)s, %(cash_in_10k)s, %(cash_out_10k)s, %(cash_position_10k)s,
    %(starting_cash_20k)s, %(cash_in_20k)s, %(cash_out_20k)s, %(cash_position_20k)s,
    %(starting_cash_50k)s, %(cash_in_50k)s, %(cash_out_50k)s, %(cash_position_50k)s,
    %(starting_cash_100k)s, %(cash_in_100k)s, %(cash_out_100k)s, %(cash_position_100k)s,
    %(position_source)s
)
"""


def file_already_processed(conn: Any, checksum: str) -> bool:
    with conn.cursor() as cursor:
        cursor.execute(
            "SELECT id, status FROM itm_cashpos_files WHERE checksum = %s",
            (checksum,),
        )
        row = cursor.fetchone()
    return row is not None and row[1] == "completed"


def create_file_record(conn: Any, filename: str, file_date: date, checksum: str) -> int:
    with conn.cursor() as cursor:
        cursor.execute(
            "SELECT id FROM itm_cashpos_files WHERE checksum = %s", (checksum,)
        )
        existing = cursor.fetchone()
        if existing:
            file_id = existing[0]
            cursor.execute(
                """
                UPDATE itm_cashpos_files
                SET status = 'processing', error_message = NULL,
                    row_count = NULL, success_count = NULL, error_count = NULL,
                    processed_at = NULL, updated_at = now()
                WHERE id = %s
                """,
                (file_id,),
            )
            cursor.execute("DELETE FROM itm_cashpos WHERE file_id = %s", (file_id,))
            conn.commit()
            return file_id

        cursor.execute(
            """
            INSERT INTO itm_cashpos_files (filename, file_date, checksum, status)
            VALUES (%s, %s, %s, 'processing')
            RETURNING id
            """,
            (filename, file_date, checksum),
        )
        file_id = cursor.fetchone()[0]
    conn.commit()
    return file_id


def update_file_status(
    conn: Any,
    file_id: int,
    status: str,
    row_count: int,
    success_count: int,
    error_count: int,
    error_message: str | None = None,
) -> None:
    with conn.cursor() as cursor:
        cursor.execute(
            """
            UPDATE itm_cashpos_files
            SET status = %s, row_count = %s, success_count = %s,
                error_count = %s, error_message = %s,
                processed_at = now(), updated_at = now()
            WHERE id = %s
            """,
            (status, row_count, success_count, error_count, error_message, file_id),
        )
    conn.commit()


def insert_rows_batch(conn: Any, file_id: int, rows: list[dict[str, Any]]) -> int:
    if not rows:
        return 0
    with conn.cursor() as cursor:
        for row in rows:
            cursor.execute(INSERT_ROW_SQL, {**row, "file_id": file_id})
    conn.commit()
    return len(rows)


def process_file(conn: Any, path: Path) -> dict[str, Any]:
    filename = path.name
    logging.info("Processing: %s", filename)
    cashpos_date = extract_file_date(filename)
    if cashpos_date is None:
        raise ValueError(f"Invalid cashpos filename date: {filename}")

    checksum = compute_checksum(path)
    if file_already_processed(conn, checksum):
        logging.info("SKIP (already processed): %s [checksum=%s]", filename, checksum[:12])
        return {"file": filename, "status": "skipped", "reason": "already processed"}

    file_id = create_file_record(conn, filename, cashpos_date, checksum)
    raw_rows, header_errors = read_csv_file(path)
    if header_errors:
        error_message = "; ".join(header_errors)
        update_file_status(conn, file_id, "failed", 0, 0, 0, error_message)
        return {"file": filename, "status": "failed", "error": error_message}

    parsed_rows: list[dict[str, Any]] = []
    row_errors: list[str] = []
    for row_num, raw_row in enumerate(raw_rows, start=2):
        try:
            parsed_rows.append(parse_csv_row(raw_row, row_num, cashpos_date))
        except ValueError as error:
            row_errors.append(str(error))
            logging.warning("Parse error: %s", error)

    inserted = insert_rows_batch(conn, file_id, parsed_rows)
    status = "completed" if inserted > 0 or not row_errors else "failed"
    error_message = "\n".join(row_errors[:20]) if row_errors else None
    if len(row_errors) > 20:
        error_message += f"\n... and {len(row_errors) - 20} more errors"
    update_file_status(conn, file_id, status, len(raw_rows), inserted, len(row_errors), error_message)

    if status == "completed":
        move_to_backup(path)
    return {
        "file": filename,
        "status": status,
        "total_rows": len(raw_rows),
        "inserted": inserted,
        "errors": len(row_errors),
    }


def main() -> None:
    setup_logging()
    csv_files = sorted(INPUT_DIR.glob(FILE_PATTERN))
    if not csv_files:
        logging.info("No files matching '%s' in %s", FILE_PATTERN, INPUT_DIR)
        return

    os.environ.setdefault("PGSERVICEFILE", "NUL")
    os.environ.setdefault("PGPASSFILE", "NUL")
    os.environ.setdefault("PGSYSCONFDIR", ".")
    try:
        conn = psycopg.connect(
            host=os.getenv("DB_HOST", "localhost"),
            port=int(os.getenv("DB_PORT", "5432")),
            dbname=os.getenv("DB_NAME", "cms"),
            user=os.getenv("DB_USER", "postgres"),
            password=os.getenv("DB_PASS", ""),
            autocommit=False,
        )
    except Exception as error:
        logging.error("Database connection failed: %s", error)
        sys.exit(1)

    results: list[dict[str, Any]] = []
    try:
        for csv_path in csv_files:
            try:
                results.append(process_file(conn, csv_path))
            except Exception as error:
                logging.exception("Unexpected error processing %s", csv_path.name)
                conn.rollback()
                results.append({"file": csv_path.name, "status": "error", "error": str(error)})
    finally:
        conn.close()

    failed = sum(1 for result in results if result.get("status") in ("failed", "error"))
    logging.info("ETL finished | files=%d | failed=%d", len(results), failed)
    if failed:
        sys.exit(1)


if __name__ == "__main__":
    main()
