"""Load DMAA Order_All XLSX files into the DMAA forecast tables."""
from __future__ import annotations

import hashlib
import logging
import os
import shutil
from datetime import datetime
from decimal import Decimal, InvalidOperation
from pathlib import Path
from typing import Any

import pandas as pd
import psycopg
from dotenv import load_dotenv


ROOT_DIR = Path(__file__).resolve().parents[2]
INPUT_DIR = ROOT_DIR / "FTP_DATA" / "DMAA"
BACKUP_DIR = INPUT_DIR / "backup"
MISSING_TERMINAL_LOG = INPUT_DIR / "missing_terminal.log"
FILE_PATTERN = "Order_All_*.xlsx"
REQUIRED_COLUMNS = {
    "atm_id",
    "periode_pred",
    "denomination",
    "amount_replenish",
    "location",
}

load_dotenv(Path(__file__).resolve().parents[1] / ".env")


def setup_logging() -> None:
    logging.basicConfig(level=logging.INFO, format="%(asctime)s | %(levelname)s | %(message)s")


def file_checksum(path: Path) -> str:
    digest = hashlib.sha256()
    with path.open("rb") as source:
        for chunk in iter(lambda: source.read(1024 * 1024), b""):
            digest.update(chunk)
    return digest.hexdigest()


def integer_value(value: Any, column: str, row_number: int, minimum: int = 0) -> int:
    try:
        number = Decimal(str(value).replace(",", "").strip())
    except (InvalidOperation, AttributeError):
        raise ValueError(f"Row {row_number}: {column} is not numeric") from None
    if not number.is_finite() or number != number.to_integral_value() or number < minimum:
        raise ValueError(f"Row {row_number}: {column} must be a whole number >= {minimum}")
    return int(number)


def read_rows(path: Path) -> list[dict[str, Any]]:
    with pd.ExcelFile(path) as workbook:
        data = pd.read_excel(workbook)
    missing = REQUIRED_COLUMNS - set(data.columns)
    if missing:
        raise ValueError(f"Missing required columns: {sorted(missing)}")
    if data.empty:
        raise ValueError("XLSX contains no data rows")

    rows: list[dict[str, Any]] = []
    for row_number, row in enumerate(data.to_dict("records"), start=2):
        terminal_id = str(row["atm_id"]).strip()
        if not terminal_id or terminal_id.lower() == "nan":
            raise ValueError(f"Row {row_number}: atm_id is empty")

        periode_pred = pd.to_datetime(row["periode_pred"], errors="coerce")
        if pd.isna(periode_pred):
            raise ValueError(f"Row {row_number}: periode_pred is invalid")

        rows.append(
            {
                "terminal_id": terminal_id,
                "periode_pred": periode_pred.date(),
                "denom": integer_value(row["denomination"], "denomination", row_number, 1),
                "amount_replenish": integer_value(
                    row["amount_replenish"], "amount_replenish", row_number
                ),
                "amount_refund": 0,
            }
        )

    keys = [(r["terminal_id"], r["periode_pred"], r["denom"]) for r in rows]
    if len(keys) != len(set(keys)):
        raise ValueError("Duplicate rows found for terminal_id, periode_pred, and denom")
    return rows


def connect():
    # Prevent libpq from reading malformed per-user config paths on Windows.
    os.environ.setdefault("PGSERVICEFILE", "NUL")
    os.environ.setdefault("PGPASSFILE", "NUL")
    os.environ.setdefault("PGSYSCONFDIR", ".")
    return psycopg.connect(
        host=os.getenv("DB_HOST", "localhost"),
        port=int(os.getenv("DB_PORT", "5432")),
        dbname=os.getenv("DB_NAME", "cms"),
        user=os.getenv("DB_USER", "postgres"),
        password=os.getenv("DB_PASS", "postgres"),
    )


def archive(path: Path) -> Path:
    BACKUP_DIR.mkdir(parents=True, exist_ok=True)
    target = BACKUP_DIR / path.name
    if target.exists():
        target = BACKUP_DIR / f"{path.stem}_{datetime.now():%Y%m%d_%H%M%S}{path.suffix}"
    shutil.move(str(path), str(target))
    return target


def log_missing_terminals(filename: str, terminal_ids: set[str]) -> None:
    with MISSING_TERMINAL_LOG.open("a", encoding="utf-8") as log:
        for terminal_id in sorted(terminal_ids):
            log.write(f"{datetime.now():%Y-%m-%d %H:%M:%S} | {filename} | {terminal_id}\n")


def process_file(conn, path: Path) -> tuple[str, int]:
    checksum = file_checksum(path)
    rows = read_rows(path)
    with conn.transaction():
        with conn.cursor() as cur:
            cur.execute("SELECT id FROM public.dmaa_files WHERE checksum = %s", (checksum,))
            if cur.fetchone():
                return "skipped", len(rows)

            cur.execute(
                """
                INSERT INTO public.dmaa_files
                    (name, status, is_valid, file_date, source_system, checksum,
                     row_count, success_count, error_count, processed_at, updated_at)
                VALUES (%s, 'processing', true, %s, 'DMAA', %s, %s, 0, 0, now(), now())
                RETURNING id
                """,
                (path.name, datetime.fromtimestamp(path.stat().st_mtime).date(), checksum, len(rows)),
            )
            file_id = cur.fetchone()[0]
            terminal_ids = list({row["terminal_id"] for row in rows})
            cur.execute(
                "SELECT terminal_id FROM public.atms WHERE terminal_id = ANY(%s)",
                (terminal_ids,),
            )
            valid_terminal_ids = {row[0] for row in cur.fetchall()}
            missing_terminal_ids = set(terminal_ids) - valid_terminal_ids
            if missing_terminal_ids:
                log_missing_terminals(path.name, missing_terminal_ids)
                logging.warning(
                    "Skipping %d missing terminal ID(s) from %s: %s",
                    len(missing_terminal_ids),
                    path.name,
                    sorted(missing_terminal_ids),
                )
            valid_rows = [row for row in rows if row["terminal_id"] in valid_terminal_ids]
            cur.executemany(
                """
                INSERT INTO public.dmaa_atm_forecast
                    (terminal_id, dmaa_file_id, periode_pred, denom, amount_replenish, amount_refund)
                VALUES (%(terminal_id)s, %(file_id)s, %(periode_pred)s, %(denom)s,
                        %(amount_replenish)s, %(amount_refund)s)
                """,
                [{**row, "file_id": file_id} for row in valid_rows],
            )
            cur.execute(
                """
                UPDATE public.dmaa_files
                SET status = 'success', is_valid = %s, success_count = %s,
                    error_count = %s, error_message = %s,
                    processed_at = now(), updated_at = now()
                WHERE id = %s
                """,
                (
                    not missing_terminal_ids,
                    len(valid_rows),
                    len(rows) - len(valid_rows),
                    f"Missing terminal IDs: {', '.join(sorted(missing_terminal_ids))}"
                    if missing_terminal_ids
                    else None,
                    file_id,
                ),
            )
    return "completed", len(rows)


def main() -> None:
    setup_logging()
    files = sorted(INPUT_DIR.glob(FILE_PATTERN))
    if not files:
        logging.info("No files matching %s in %s", FILE_PATTERN, INPUT_DIR)
        return

    with connect() as conn:
        for path in files:
            try:
                status, row_count = process_file(conn, path)
                if status == "completed":
                    logging.info("Loaded %s (%s rows), archived to %s", path.name, row_count, archive(path))
                else:
                    try:
                        archived_path = archive(path)
                    except PermissionError:
                        logging.warning("File is locked; could not archive %s", path.name)
                    else:
                        logging.info("Skipped already loaded file: %s; archived to %s", path.name, archived_path)
            except Exception:
                conn.rollback()
                logging.exception("Failed to process %s", path.name)
                raise


if __name__ == "__main__":
    main()
