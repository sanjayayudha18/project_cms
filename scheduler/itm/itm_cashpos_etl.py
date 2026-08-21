"""
ITM ATM Cash Position ETL Scheduler
====================================
Reads CSV files from FTP_DATA/ITM/, parses them, and pushes rows
into the `itm_cashpos` table in the CMS PostgreSQL database.

Idempotent: uses SHA-256 file checksum to skip already-processed files.
"""
from __future__ import annotations

import csv
import hashlib
import logging
import os
import shutil
import sys
from datetime import date, datetime, time
from decimal import Decimal, InvalidOperation
from pathlib import Path
from typing import Any

import psycopg

from dotenv import load_dotenv

# Load .env file from the same directory as this script
load_dotenv(Path(__file__).parent / ".env")

# =============================================================================
# CONFIG
# =============================================================================
INPUT_DIR = Path(
    os.getenv(
        "ITM_INPUT_DIR",
        r"C:\Users\RB Yudha Rangga\OneDrive\Documents\Development\CMS2\FTP_DATA\ITM\atm_replenish",
    )
)
BACKUP_DIR = INPUT_DIR / "backups"
LOG_DIR = INPUT_DIR / "logs"

FILE_PATTERN = "ATM_Replenish_*.csv"

# Full + abbreviated month names (English + Indonesian)
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

# =============================================================================
# LOGGING
# =============================================================================
def setup_logging() -> None:
    LOG_DIR.mkdir(parents=True, exist_ok=True)
    log_file = LOG_DIR / f"itm_cashpos_{datetime.now().strftime('%Y%m%d_%H%M%S')}.log"
    logging.basicConfig(
        level=logging.INFO,
        format="%(asctime)s | %(levelname)s | %(message)s",
        handlers=[
            logging.FileHandler(log_file, encoding="utf-8"),
            logging.StreamHandler(),
        ],
    )


# =============================================================================
# HELPERS
# =============================================================================
def compute_checksum(path: Path) -> str:
    """SHA-256 hex digest of the file content."""
    h = hashlib.sha256()
    with open(path, "rb") as f:
        for chunk in iter(lambda: f.read(8192), b""):
            h.update(chunk)
    return h.hexdigest()


def parse_itm_date(raw: str) -> date:
    """
    Parse ITM date format: 1YYMMDD
    '1' prefix is fixed, YY = 2-digit year, MM = month, DD = day.
    Example: 1260818 -> 2026-08-18
    """
    raw = raw.strip()
    if len(raw) != 7 or not raw.startswith("1"):
        raise ValueError(f"Invalid ITM date format: '{raw}' (expected 1YYMMDD)")
    yy = int(raw[1:3])
    mm = int(raw[3:5])
    dd = int(raw[5:7])
    year = 2000 + yy
    return date(year, mm, dd)


def parse_itm_time(raw: str) -> time:
    """
    Parse ITM time format: HHMMSS (no separators, no leading zero guarantee).
    Example: 111507 -> 11:15:07, 85842 -> 08:58:42, 3431 -> 00:34:31
    """
    raw = raw.strip()
    # Pad to 6 digits with leading zeros
    raw = raw.zfill(6)
    hh = int(raw[0:2])
    mm = int(raw[2:4])
    ss = int(raw[4:6])
    return time(hh, mm, ss)


def parse_decimal(raw: str) -> Decimal:
    """Parse a numeric string to Decimal, default 0 if empty."""
    raw = raw.strip()
    if not raw:
        return Decimal("0")
    try:
        return Decimal(raw)
    except InvalidOperation:
        raise ValueError(f"Invalid decimal value: '{raw}'")


def extract_file_date(filename: str) -> date | None:
    """
    Extract the business date from filename like:
      'ATM_Replenish_21_Agu_2026.csv' or 'ATM_Cashpos_19_April_2026.csv'
    Returns None if parsing fails.
    """
    stem = Path(filename).stem  # ATM_Replenish_21_Agu_2026
    parts = stem.split("_")
    # Expected: ['ATM', 'Replenish'|'Cashpos', day, month, year]
    if len(parts) >= 5:
        try:
            day = int(parts[2])
            month = _MONTH_MAP.get(parts[3].lower())
            year = int(parts[4])
            if month is None:
                return None
            return date(year, month, day)
        except (ValueError, IndexError):
            pass
    return None


def move_to_backup(path: Path) -> Path:
    """Move processed file to backup directory."""
    BACKUP_DIR.mkdir(parents=True, exist_ok=True)
    target = BACKUP_DIR / path.name
    if target.exists():
        stamp = datetime.now().strftime("%Y%m%d_%H%M%S")
        target = BACKUP_DIR / f"{path.stem}_{stamp}{path.suffix}"
    shutil.move(str(path), str(target))
    return target


# =============================================================================
# CSV PARSING
# =============================================================================
EXPECTED_HEADERS = [
    "TGLREPLENISH",
    "JAMREPLENISH",
    "ATMID",
    "TYPEATMCRM",
    "TELLERID",
    "CABANG",
    "ESCROW",
    "REFUNDDENOM50K",
    "REFUNDDENOM100K",
    "REFUNDTOTAL",
    "REPLENISHDENOM50K",
    "REPLENISHDENOM100K",
    "REPLENISHTOTAL",
]


def parse_csv_row(row: dict[str, str], row_num: int) -> dict[str, Any]:
    """Parse a single CSV row into DB-ready values. Raises ValueError on bad data."""
    errors: list[str] = []

    # Date & time
    try:
        replenish_date = parse_itm_date(row["TGLREPLENISH"])
    except (ValueError, KeyError) as e:
        errors.append(f"TGLREPLENISH: {e}")
        replenish_date = None

    try:
        replenish_time = parse_itm_time(row["JAMREPLENISH"])
    except (ValueError, KeyError) as e:
        errors.append(f"JAMREPLENISH: {e}")
        replenish_time = None

    # String fields (strip whitespace)
    terminal_id = row.get("ATMID", "").strip()
    machine_type = row.get("TYPEATMCRM", "").strip()
    teller_id = row.get("TELLERID", "").strip()
    branch_code = row.get("CABANG", "").strip()

    if not terminal_id:
        errors.append("ATMID is empty")
    if not machine_type:
        errors.append("TYPEATMCRM is empty")

    # Numeric fields
    try:
        escrow = parse_decimal(row.get("ESCROW", "0"))
    except ValueError as e:
        errors.append(str(e))
        escrow = Decimal("0")

    try:
        refund_denom_50k = parse_decimal(row.get("REFUNDDENOM50K", "0"))
    except ValueError as e:
        errors.append(str(e))
        refund_denom_50k = Decimal("0")

    try:
        refund_denom_100k = parse_decimal(row.get("REFUNDDENOM100K", "0"))
    except ValueError as e:
        errors.append(str(e))
        refund_denom_100k = Decimal("0")

    try:
        refund_total = parse_decimal(row.get("REFUNDTOTAL", "0"))
    except ValueError as e:
        errors.append(str(e))
        refund_total = Decimal("0")

    try:
        replenish_denom_50k = parse_decimal(row.get("REPLENISHDENOM50K", "0"))
    except ValueError as e:
        errors.append(str(e))
        replenish_denom_50k = Decimal("0")

    try:
        replenish_denom_100k = parse_decimal(row.get("REPLENISHDENOM100K", "0"))
    except ValueError as e:
        errors.append(str(e))
        replenish_denom_100k = Decimal("0")

    try:
        replenish_total = parse_decimal(row.get("REPLENISHTOTAL", "0"))
    except ValueError as e:
        errors.append(str(e))
        replenish_total = Decimal("0")

    if errors:
        raise ValueError(f"Row {row_num}: {'; '.join(errors)}")

    return {
        "replenish_date": replenish_date,
        "replenish_time": replenish_time,
        "terminal_id": terminal_id,
        "machine_type": machine_type,
        "teller_id": teller_id,
        "branch_code": branch_code,
        "escrow": escrow,
        "refund_denom_50k": refund_denom_50k,
        "refund_denom_100k": refund_denom_100k,
        "refund_total": refund_total,
        "replenish_denom_50k": replenish_denom_50k,
        "replenish_denom_100k": replenish_denom_100k,
        "replenish_total": replenish_total,
    }


def read_csv_file(path: Path) -> tuple[list[dict[str, str]], list[str]]:
    """Read CSV file, return (rows, header_errors)."""
    header_errors: list[str] = []
    with open(path, "r", encoding="utf-8-sig") as f:
        reader = csv.DictReader(f)
        if reader.fieldnames is None:
            return [], ["CSV file has no header row"]

        # Validate headers
        actual = [h.strip() for h in reader.fieldnames]
        missing = set(EXPECTED_HEADERS) - set(actual)
        if missing:
            header_errors.append(f"Missing columns: {sorted(missing)}")

        rows = list(reader)
    return rows, header_errors


# =============================================================================
# DATABASE OPERATIONS
# =============================================================================
INSERT_ROW_SQL = """
INSERT INTO itm_cashpos (
    file_id, replenish_date, replenish_time, terminal_id, machine_type,
    teller_id, branch_code, escrow,
    refund_denom_10k, refund_denom_20k, refund_denom_50k, refund_denom_100k, refund_total,
    replenish_denom_10k, replenish_denom_20k, replenish_denom_50k, replenish_denom_100k, replenish_total
) VALUES (
    %(file_id)s, %(replenish_date)s, %(replenish_time)s, %(terminal_id)s, %(machine_type)s,
    %(teller_id)s, %(branch_code)s, %(escrow)s,
    0, 0, %(refund_denom_50k)s, %(refund_denom_100k)s, %(refund_total)s,
    0, 0, %(replenish_denom_50k)s, %(replenish_denom_100k)s, %(replenish_total)s
)
"""


def file_already_processed(conn, checksum: str) -> bool:
    """Check if file with this checksum was already processed successfully."""
    with conn.cursor() as cur:
        cur.execute(
            "SELECT id, status FROM itm_cashpos_files WHERE checksum = %s",
            (checksum,),
        )
        row = cur.fetchone()
        if row is None:
            return False
        # If previously failed, allow re-processing
        return row[1] == "completed"


def create_file_record(
    conn, filename: str, file_date: date | None, checksum: str
) -> int:
    """Insert or get existing file tracking record. Returns file_id."""
    with conn.cursor() as cur:
        # Try to find existing record (for retries of failed files)
        cur.execute(
            "SELECT id FROM itm_cashpos_files WHERE checksum = %s",
            (checksum,),
        )
        existing = cur.fetchone()
        if existing:
            file_id = existing[0]
            # Reset for re-processing
            cur.execute(
                """
                UPDATE itm_cashpos_files
                SET status = 'processing', error_message = NULL,
                    row_count = NULL, success_count = NULL, error_count = NULL,
                    processed_at = NULL, updated_at = now()
                WHERE id = %s
                """,
                (file_id,),
            )
            # Delete old rows from failed attempt
            cur.execute("DELETE FROM itm_cashpos WHERE file_id = %s", (file_id,))
            conn.commit()
            return file_id

        cur.execute(
            """
            INSERT INTO itm_cashpos_files (filename, file_date, checksum, status)
            VALUES (%s, %s, %s, 'processing')
            RETURNING id
            """,
            (filename, file_date or date.today(), checksum),
        )
        file_id = cur.fetchone()[0]
        conn.commit()
        return file_id


def update_file_status(
    conn,
    file_id: int,
    status: str,
    row_count: int,
    success_count: int,
    error_count: int,
    error_message: str | None = None,
) -> None:
    """Update file tracking record with results."""
    with conn.cursor() as cur:
        cur.execute(
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


def insert_rows_batch(conn, file_id: int, parsed_rows: list[dict[str, Any]]) -> int:
    """Batch insert parsed rows. Returns count inserted."""
    if not parsed_rows:
        return 0

    with conn.cursor() as cur:
        for row in parsed_rows:
            row["file_id"] = file_id
            cur.execute(INSERT_ROW_SQL, row)

    conn.commit()
    return len(parsed_rows)


# =============================================================================
# MAIN PROCESS
# =============================================================================
def process_file(conn, path: Path) -> dict[str, Any]:
    """Process a single ITM cashpos CSV file end-to-end."""
    filename = path.name
    logging.info("Processing: %s", filename)

    # Compute checksum for idempotency
    checksum = compute_checksum(path)

    # Skip if already processed
    if file_already_processed(conn, checksum):
        logging.info("SKIP (already processed): %s [checksum=%s]", filename, checksum[:12])
        return {"file": filename, "status": "skipped", "reason": "already processed"}

    # Extract business date from filename
    file_date = extract_file_date(filename)

    # Create/reset file tracking record
    file_id = create_file_record(conn, filename, file_date, checksum)

    # Read CSV
    raw_rows, header_errors = read_csv_file(path)
    if header_errors:
        error_msg = "; ".join(header_errors)
        logging.error("Header validation failed for %s: %s", filename, error_msg)
        update_file_status(conn, file_id, "failed", 0, 0, 0, error_msg)
        return {"file": filename, "status": "failed", "error": error_msg}

    total_rows = len(raw_rows)
    logging.info("Read %d rows from %s", total_rows, filename)

    # Parse rows
    parsed_rows: list[dict[str, Any]] = []
    row_errors: list[str] = []

    for i, raw_row in enumerate(raw_rows, start=2):  # row 2 = first data row
        try:
            parsed = parse_csv_row(raw_row, i)
            parsed_rows.append(parsed)
        except ValueError as e:
            row_errors.append(str(e))
            logging.warning("Parse error: %s", e)

    # Insert valid rows
    inserted = insert_rows_batch(conn, file_id, parsed_rows)

    # Determine final status
    error_count = len(row_errors)
    if error_count == 0:
        status = "completed"
    elif inserted > 0:
        status = "completed"  # partial success still marks completed
    else:
        status = "failed"

    error_message = None
    if row_errors:
        # Store first N errors in the file record
        error_message = "\n".join(row_errors[:20])
        if len(row_errors) > 20:
            error_message += f"\n... and {len(row_errors) - 20} more errors"

    update_file_status(
        conn, file_id, status, total_rows, inserted, error_count, error_message
    )

    logging.info(
        "Done: %s | total=%d | inserted=%d | errors=%d | status=%s",
        filename,
        total_rows,
        inserted,
        error_count,
        status,
    )

    # Move to backup on success
    if status == "completed":
        backup_path = move_to_backup(path)
        logging.info("Moved to backup: %s", backup_path)

    return {
        "file": filename,
        "status": status,
        "total_rows": total_rows,
        "inserted": inserted,
        "errors": error_count,
    }


def main() -> None:
    setup_logging()
    logging.info("=" * 60)
    logging.info("ITM Cash Position ETL started")
    logging.info("Input directory: %s", INPUT_DIR)
    logging.info("=" * 60)

    # Find CSV files
    csv_files = sorted(INPUT_DIR.glob(FILE_PATTERN))
    if not csv_files:
        logging.info("No files matching '%s' in %s. Nothing to do.", FILE_PATTERN, INPUT_DIR)
        return

    logging.info("Found %d file(s) to process", len(csv_files))

    # Connect to database
    # Workaround: psycopg/libpq on Windows may read pg_service.conf or .pgpass
    # from APPDATA paths containing non-UTF-8 characters, causing decode errors.
    # Setting these env vars to empty/NUL prevents libpq from scanning those paths.
    os.environ.setdefault("PGSERVICEFILE", "NUL")
    os.environ.setdefault("PGPASSFILE", "NUL")
    os.environ.setdefault("PGSYSCONFDIR", ".")

    try:
        conn = psycopg.connect(
            host=os.getenv("DB_HOST", "localhost"),
            port=int(os.getenv("DB_PORT", "5432")),
            dbname=os.getenv("DB_NAME", "cms"),
            user=os.getenv("DB_USER", "postgres"),
            password=os.getenv("DB_PASS", "postgres"),
            autocommit=False,
        )
        logging.info("Connected to database")
    except Exception as e:
        logging.error("Database connection failed: %s", e)
        sys.exit(1)

    results: list[dict[str, Any]] = []

    try:
        for csv_path in csv_files:
            try:
                result = process_file(conn, csv_path)
                results.append(result)
            except Exception as e:
                logging.exception("Unexpected error processing %s", csv_path.name)
                results.append({"file": csv_path.name, "status": "error", "error": str(e)})
                # Rollback any partial transaction
                conn.rollback()
    finally:
        conn.close()
        logging.info("Database connection closed")

    # Summary
    completed = sum(1 for r in results if r.get("status") == "completed")
    skipped = sum(1 for r in results if r.get("status") == "skipped")
    failed = sum(1 for r in results if r.get("status") in ("failed", "error"))

    logging.info("=" * 60)
    logging.info(
        "ETL finished | completed=%d | skipped=%d | failed=%d",
        completed,
        skipped,
        failed,
    )
    logging.info("=" * 60)

    if failed > 0:
        sys.exit(1)


if __name__ == "__main__":
    main()
