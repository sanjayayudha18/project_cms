"""Load vendor DSR workbooks (sheets 'Daily' + 'Rencana Isi') into the DSR tables.

Filename contract (written by the Go upload handler, backend/internal/service/dsr_upload.go):
    <vendor_code>__<uploaded_by_user_id>__<original_filename>
`uploaded_by_user_id` is optional: a 2-part `<vendor_code>__<original_filename>` name is
also accepted, with uploaded_by_user_id left NULL (e.g. a manual backfill drop).

Real-sample quirks this parser is built against (see DSR_DATA/*.xls, *.xlsx):
  - 'Daily' denom-grid column positions are NOT fixed -- they shift per vendor and even
    per vault block within the same workbook (merged-cell artifacts in the legacy .xls).
    Column -> denom mapping is therefore read from each block's own header row, never
    hardcoded.
  - Header metadata ("Kepada", "Bank", "Tanggal", ...) is sometimes one cell
    ("Bank   :   BANK CIMB NIAGA - NCC") and sometimes two adjacent cells
    (label cell "Bank", value cell "  :  BANK CIMB NIAGA - NCC" or a bare value with no
    colon at all). Both forms are handled.
  - A multi-location workbook (e.g. TAG JAKARTA) repeats the whole SALDO HARIAN ATM grid
    once per vault (LENTENG AGUNG, BINTARO, BEKASI, CIMONE, ...), ending in a derived
    SALDO GABUNGAN row. Only the first (unlabeled) block carries the file-level
    STATUS SALDO SEMENTARA (section d1) / STATUS UANG CADANGAN ATM content; per-vault
    blocks are section d0 only. This matches 013_dsr.sql / 017_...sql's documented shape.
  - Broken formula cells read as literal strings containing '#REF' (or otherwise fail to
    parse as a number) -> NULL + error_count++, per 013_dsr.sql's documented tolerance.
    A genuinely blank/NaN denom cell is NOT an error -- it is stored as 0, matching the
    surrounding cells' own convention of writing explicit 0 rather than leaving blanks.

Units: denom_* / *_total_idr columns are FULL IDR, no x1000 scale, despite the "(x 1.000)"
header label -- verified against both sample workbooks (013_dsr.sql's own note).
"""
from __future__ import annotations

import argparse
import hashlib
import json
import logging
import os
import re
import shutil
from datetime import date, datetime
from decimal import Decimal, InvalidOperation
from pathlib import Path
from typing import Any

import pandas as pd
import psycopg
from dotenv import load_dotenv


ROOT_DIR = Path(__file__).resolve().parents[2]
INPUT_DIR = ROOT_DIR / "FTP_DATA" / "DSR"
BACKUP_DIR = INPUT_DIR / "backups"
# Files that have been dry-run parsed but not yet confirmed by the vendor sit here --
# deliberately NOT inside INPUT_DIR's own glob pattern, so the batch-drain safety net
# (main()'s no-args mode, run by retry_scheduler's cron-scan) never auto-commits a file
# that is intentionally still awaiting explicit confirmation.
PENDING_DIR = INPUT_DIR / "pending_confirmation"
FILE_PATTERNS = ("*.xls", "*.xlsx")

DAILY_SHEET = "Daily"
RENCANA_SHEET = "Rencana Isi"

HEADER_FIELD_KEYS = {
    "kepada": "recipient",
    "dari": "sender",
    "bank": "bank",
    "company": "company",
    "tanggal": "report_date",
    "perihal": "subject",
}
DENOM_VALUES = {100000: "denom_100k", 50000: "denom_50k", 20000: "denom_20k",
                10000: "denom_10k", 5000: "denom_5k", 2000: "denom_2k", 1000: "denom_1k"}

load_dotenv(Path(__file__).resolve().parents[1] / ".env")


def setup_logging() -> None:
    logging.basicConfig(level=logging.INFO, format="%(asctime)s | %(levelname)s | %(message)s")


def file_checksum(path: Path) -> str:
    digest = hashlib.sha256()
    with path.open("rb") as source:
        for chunk in iter(lambda: source.read(1024 * 1024), b""):
            digest.update(chunk)
    return digest.hexdigest()


def connect():
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


def parse_upload_filename(filename: str) -> tuple[str, int | None, str]:
    """<vendor_code>__<user_id>__<original>  or  <vendor_code>__<original> (legacy/manual)."""
    parts = filename.split("__", 2)
    if len(parts) == 3 and parts[1].isdigit():
        return parts[0], int(parts[1]), parts[2]
    if len(parts) >= 2:
        return parts[0], None, "__".join(parts[1:])
    raise ValueError(f"Filename does not follow <vendor_code>__...: {filename}")


def _cell_text(value: Any) -> str:
    if value is None or (isinstance(value, float) and pd.isna(value)):
        return ""
    return str(value).strip()


def cell_number(value: Any) -> tuple[Decimal | None, bool]:
    """Returns (amount, is_error). Blank/NaN -> (0, False). '#REF!'/unparseable -> (None, True)."""
    if value is None or (isinstance(value, float) and pd.isna(value)):
        return Decimal(0), False
    if isinstance(value, str):
        text = value.strip()
        if not text:
            return Decimal(0), False
        if "#" in text or "ref" in text.lower():
            return None, True
        text = text.replace(",", "")
        try:
            return Decimal(text), False
        except InvalidOperation:
            return None, True
    try:
        return Decimal(str(value)), False
    except InvalidOperation:
        return None, True


# ============================================================
# 'Daily' sheet
# ============================================================
def _extract_header_fields(grid: pd.DataFrame, max_row: int) -> dict[str, Any]:
    fields: dict[str, Any] = {}
    for r in range(max_row):
        row = grid.iloc[r]
        for c in range(len(row)):
            raw = row[c]
            text = _cell_text(raw)
            if not text:
                continue
            if ":" in text:
                label, _, value = text.partition(":")
                label = label.strip().lower()
                value = value.strip()
            else:
                label = text.strip().lower()
                value = ""
            key = HEADER_FIELD_KEYS.get(label)
            if not key:
                continue
            if not value:
                # Label and value live in separate cells (label cell may or may not
                # carry a trailing ':'); scan ahead for the first non-empty cell,
                # which may already be a real datetime rather than text.
                for c2 in range(c + 1, min(c + 4, len(row))):
                    candidate = row[c2]
                    if isinstance(candidate, (datetime, date)):
                        value = candidate
                        break
                    candidate_text = _cell_text(candidate).lstrip(":").strip()
                    if candidate_text:
                        value = candidate_text
                        break
            if key and value != "":
                fields[key] = value
    if "report_date" in fields:
        parsed = pd.to_datetime(fields["report_date"], errors="coerce")
        fields["report_date"] = parsed.date() if not pd.isna(parsed) else None
    return fields


def _find_deno_header_rows(grid: pd.DataFrame) -> list[int]:
    rows = []
    for r in range(len(grid)):
        row = grid.iloc[r]
        if any("deno" in _cell_text(v).lower() for v in row):
            rows.append(r)
    return rows


def _block_location(grid: pd.DataFrame, header_row: int) -> str | None:
    for r in range(header_row - 1, -1, -1):
        text = _cell_text(grid.iloc[r, 0])
        if text:
            return None if text.upper() == "SALDO HARIAN ATM" else text
    return None


def _column_maps(grid: pd.DataFrame, header_row: int) -> tuple[dict[int, str], int | None]:
    denom_col: dict[int, str] = {}
    total_col: int | None = None
    header = grid.iloc[header_row]
    for c, v in enumerate(header):
        if "total rupiah" in _cell_text(v).lower():
            total_col = c
    denom_row = header_row + 1
    if denom_row < len(grid):
        for c, v in enumerate(grid.iloc[denom_row]):
            amount, is_error = cell_number(v)
            if not is_error and amount is not None and int(amount) in DENOM_VALUES:
                denom_col[c] = DENOM_VALUES[int(amount)]
    return denom_col, total_col


def read_daily_rows(path: Path) -> tuple[dict[str, Any], list[dict[str, Any]], int]:
    grid = pd.read_excel(path, sheet_name=DAILY_SHEET, header=None,
                          engine="xlrd" if path.suffix.lower() == ".xls" else None)
    deno_rows = _find_deno_header_rows(grid)
    if not deno_rows:
        raise ValueError(f"No denom grid found on sheet '{DAILY_SHEET}'")

    file_fields = _extract_header_fields(grid, deno_rows[0])
    rows: list[dict[str, Any]] = []
    error_count = 0
    row_no = 0

    for block_idx, header_row in enumerate(deno_rows):
        location = _block_location(grid, header_row)
        denom_col, total_col = _column_maps(grid, header_row)
        data_start = header_row + 2
        data_end = deno_rows[block_idx + 1] if block_idx + 1 < len(deno_rows) else len(grid)

        section = "d0"
        flow: str | None = None

        for r in range(data_start, data_end):
            row = grid.iloc[r]
            col2 = _cell_text(row[2]) if len(row) > 2 else ""
            col3 = _cell_text(row[3]) if len(row) > 3 else ""
            label = col2.upper()

            def totals() -> tuple[dict[str, Decimal | None], Decimal | None, bool]:
                denoms: dict[str, Decimal | None] = {}
                has_error = False
                for c, key in denom_col.items():
                    amount, is_err = cell_number(row[c] if c < len(row) else None)
                    denoms[key] = amount
                    has_error = has_error or is_err
                total_amt = None
                if total_col is not None and total_col < len(row):
                    total_amt, t_err = cell_number(row[total_col])
                    has_error = has_error or t_err
                return denoms, total_amt, has_error

            if not col2 and not col3 and all(
                pd.isna(row[c]) if c < len(row) else True for c in denom_col
            ):
                continue  # spacer row

            if "SALDO AWAL" in label:
                denoms, total_amt, err = totals()
                error_count += err
                flow = "saldo_awal"
                row_no += 1
                rows.append({"row_no": row_no, "section": section, "flow": flow,
                              "line_label": "SALDO AWAL", "memo_no": None,
                              **denoms, "line_total_idr": total_amt, "remarks": None})
            elif "STATUS SALDO SEMENTARA" in label:
                section = "d1"
                flow = None
                status_date = pd.to_datetime(row[0], errors="coerce")
                if not pd.isna(status_date):
                    file_fields["status_date"] = status_date.date()
            elif label.startswith("SUBTOTAL"):
                flow = None
            elif label.startswith("SALDO AKHIR"):
                if location is None and section == "d0":
                    _, total_amt, _ = totals()
                    file_fields["saldo_akhir_0000_total_idr"] = total_amt
                flow = None
            elif label.startswith("SALDO SEMENTARA"):
                if location is None:
                    _, total_amt, _ = totals()
                    file_fields["saldo_sementara_0900_total_idr"] = total_amt
                flow = None
            elif "PENERIMAAN" in label and not label.startswith("SUBTOTAL"):
                flow = "penerimaan"
                denoms, total_amt, err = totals()
                error_count += err
                row_no += 1
                memo = _extract_memo(col3)
                rows.append({"row_no": row_no, "section": section, "flow": flow,
                              "line_label": col3 or col2, "memo_no": memo,
                              **denoms, "line_total_idr": total_amt, "remarks": None})
            elif "PENGELUARAN" in label and not label.startswith("SUBTOTAL"):
                flow = "pengeluaran"
                denoms, total_amt, err = totals()
                error_count += err
                row_no += 1
                memo = _extract_memo(col3)
                rows.append({"row_no": row_no, "section": section, "flow": flow,
                              "line_label": col3 or col2, "memo_no": memo,
                              **denoms, "line_total_idr": total_amt, "remarks": None})
            elif label.startswith("STATUS") and "UANG CADANGAN" in label:
                flow = "status_cadangan"
            elif label.startswith("KONDISI"):
                flow = "status_cadangan"
                denoms, total_amt, err = totals()
                error_count += err
                row_no += 1
                rows.append({"row_no": row_no, "section": section, "flow": flow,
                              "line_label": col2, "memo_no": None,
                              **denoms, "line_total_idr": total_amt, "remarks": None})
            elif label.startswith("TOTAL STATUS UANG CADANGAN"):
                if location is None:
                    _, total_amt, _ = totals()
                    file_fields["status_cadangan_total_idr"] = total_amt
                flow = None
            elif label.startswith("SALDO GABUNGAN"):
                _, total_amt, _ = totals()
                file_fields["saldo_gabungan_total_idr"] = total_amt
            elif not col2:
                if flow in ("penerimaan", "pengeluaran") and col3:
                    denoms, total_amt, err = totals()
                    error_count += err
                    row_no += 1
                    memo = _extract_memo(col3)
                    rows.append({"row_no": row_no, "section": section, "flow": flow,
                                  "line_label": col3, "memo_no": memo,
                                  **denoms, "line_total_idr": total_amt, "remarks": None})
                # else: unlabeled derived/spacer row -- skip
            elif flow:
                denoms, total_amt, err = totals()
                error_count += err
                row_no += 1
                rows.append({"row_no": row_no, "section": section, "flow": flow,
                              "line_label": col3 or col2, "memo_no": _extract_memo(col2),
                              **denoms, "line_total_idr": total_amt, "remarks": None})

    for row in rows:
        for key in DENOM_VALUES.values():
            row.setdefault(key, Decimal(0))

    return file_fields, rows, error_count


_MEMO_RE = re.compile(r"(\d+/\S+/\S+/[A-Z0-9]+/\d{4})")


def _extract_memo(text: str) -> str | None:
    match = _MEMO_RE.search(text)
    return match.group(1) if match else None


# ============================================================
# 'Rencana Isi' sheet
# ============================================================
def _rencana_column_roles(headers: list[str]) -> dict[int, str]:
    roles: dict[int, str] = {}
    for i, h in enumerate(headers):
        text = h.lower()
        if "atm id" in text:
            roles[i] = "atm_terminal_id"
        elif "lokasi" in text:
            roles[i] = "atm_location"
        elif "100rb" in text or "100 rb" in text:
            roles[i] = "fill_100k_idr"
        elif "50rb" in text or "50 rb" in text:
            roles[i] = "fill_50k_idr"
        elif "splank" in text:
            roles[i] = "splank_balance_0800_idr"
        elif "keterangan" in text:
            roles[i] = "remarks"
        elif text.strip() == "50/100" or "denom" in text:
            roles[i] = "denom_config"
    return roles


def read_rencana_isi_rows(path: Path) -> tuple[date | None, list[dict[str, Any]]]:
    grid = pd.read_excel(path, sheet_name=RENCANA_SHEET, header=None,
                          engine="xlrd" if path.suffix.lower() == ".xls" else None)

    plan_date = None
    for r in range(min(3, len(grid))):
        row = grid.iloc[r]
        for c in range(len(row)):
            text = _cell_text(row[c])
            if not text.lower().startswith("tanggal"):
                continue
            for c2 in range(c + 1, min(c + 4, len(row))):
                candidate = row[c2]
                if isinstance(candidate, (datetime, date)):
                    plan_date = candidate.date() if isinstance(candidate, datetime) else candidate
                    break
                candidate_text = _cell_text(candidate).lstrip(":").strip()
                if candidate_text:
                    parsed = pd.to_datetime(candidate_text, errors="coerce")
                    if not pd.isna(parsed):
                        plan_date = parsed.date()
                        break
            if plan_date:
                break
        if plan_date:
            break

    header_row_idx = None
    for r in range(len(grid)):
        if any("atm id" in _cell_text(v).lower() for v in grid.iloc[r]):
            header_row_idx = r
            break
    if header_row_idx is None:
        raise ValueError(f"No 'ATM ID' header found on sheet '{RENCANA_SHEET}'")

    headers = [_cell_text(v) for v in grid.iloc[header_row_idx]]
    roles = _rencana_column_roles(headers)
    id_col = next((c for c, role in roles.items() if role == "atm_terminal_id"), None)
    if id_col is None:
        raise ValueError("Could not locate ATM ID column")

    rows: list[dict[str, Any]] = []
    row_no = 0
    for r in range(header_row_idx + 1, len(grid)):
        row = grid.iloc[r]
        terminal_id = _cell_text(row[id_col])
        if not terminal_id or terminal_id.lower() in ("sub total", "subtotal", "total"):
            continue  # derived/summary row -- recomputed on read, not stored
        row_no += 1
        record: dict[str, Any] = {
            "row_no": row_no, "atm_terminal_id": terminal_id, "atm_location": None,
            "denom_config": None, "fill_100k_idr": Decimal(0), "fill_50k_idr": Decimal(0),
            "splank_balance_0800_idr": Decimal(0), "remarks": None,
        }
        for c, role in roles.items():
            if c == id_col or c >= len(row):
                continue
            value = row[c]
            if role in ("fill_100k_idr", "fill_50k_idr", "splank_balance_0800_idr"):
                amount, _ = cell_number(value)
                record[role] = amount if amount is not None else Decimal(0)
            elif role in ("atm_location", "denom_config", "remarks"):
                text = _cell_text(value)
                record[role] = text or None
        rows.append(record)

    return plan_date, rows


# ============================================================
# DB writes
# ============================================================
def _upsert_upload(conn, path: Path, vendor_name: str, uploaded_by: int | None) -> str:
    """Writes one workbook into dsr_uploads + both row tables, in one transaction.

    Both sheets share a single dsr_uploads row (020_dsr_daily_rencana_isi.sql), so
    they are parsed first -- tolerantly, either may fail -- and then written
    together. A sheet that failed to parse lands as status 'failed' with its
    error in error_message; the other sheet still ingests.

    Amounts are stored verbatim as printed on the sheet (no x1.000 rescaling).
    """
    checksum = file_checksum(path)

    daily_fields: dict[str, Any] = {}
    daily_rows: list[dict[str, Any]] = []
    daily_error_count = 0
    daily_status = "completed"
    errors: list[str] = []
    try:
        daily_fields, daily_rows, daily_error_count = read_daily_rows(path)
    except Exception as exc:
        daily_status = "failed"
        errors.append(f"Daily: {exc}")
        logging.exception("Failed to parse 'Daily' sheet from %s", path.name)

    plan_date = None
    rencana_rows: list[dict[str, Any]] = []
    rencana_status = "completed"
    try:
        plan_date, rencana_rows = read_rencana_isi_rows(path)
    except Exception as exc:
        rencana_status = "failed"
        errors.append(f"Rencana Isi: {exc}")
        logging.exception("Failed to parse 'Rencana Isi' sheet from %s", path.name)

    # report_date comes from the Daily sheet; fall back to Rencana Isi's plan
    # date minus a day when only that sheet parsed.
    report_date = daily_fields.get("report_date")
    if report_date is None and plan_date is not None:
        fallback = plan_date - pd.Timedelta(days=1)
        report_date = fallback.date() if hasattr(fallback, "date") else fallback
    if report_date is None:
        raise ValueError(f"Could not determine report_date from {path.name}")

    with conn.transaction(), conn.cursor() as cur:
        cur.execute(
            "SELECT checksum FROM public.dsr_uploads WHERE report_date = %s AND vendor = %s",
            (report_date, vendor_name),
        )
        existing = cur.fetchone()
        if existing and existing[0] == checksum:
            return "skipped"
        if existing:
            # Re-upload for the same date: replace it wholesale, child rows cascade.
            cur.execute(
                "DELETE FROM public.dsr_uploads WHERE report_date = %s AND vendor = %s",
                (report_date, vendor_name),
            )

        # Resolve vendor-sent terminal ids against master data. Unresolved ones
        # now INGEST with atm_id NULL (no hard FK any more) and count as errors,
        # instead of being silently dropped.
        atm_ids: dict[str, int] = {}
        if rencana_rows:
            terminal_ids = list({row["atm_terminal_id"] for row in rencana_rows})
            cur.execute(
                "SELECT terminal_id, id FROM public.atms WHERE terminal_id = ANY(%s)",
                (terminal_ids,),
            )
            atm_ids = {row[0]: row[1] for row in cur.fetchall()}
            missing = set(terminal_ids) - set(atm_ids)
            if missing:
                logging.warning("%d unresolved ATM terminal id(s) in %s: %s",
                                len(missing), path.name, sorted(missing))
        rencana_error_count = sum(1 for row in rencana_rows if row["atm_terminal_id"] not in atm_ids)

        cur.execute(
            """
            INSERT INTO public.dsr_uploads
                (filename, checksum, vendor, report_date, uploaded_by_user_id,
                 bank, company, recipient, sender, subject, currency,
                 daily_status, daily_row_count, daily_error_count,
                 rencana_isi_status, rencana_isi_row_count, rencana_isi_error_count,
                 error_message, daily_status_date, rencana_isi_plan_date,
                 saldo_akhir_0000_idr, saldo_sementara_0900_idr, status_cadangan_idr,
                 saldo_gabungan_idr, rencana_isi_subtotal_idr, processed_at)
            VALUES (%s, %s, %s, %s, %s,
                    %s, %s, %s, %s, %s, 'IDR',
                    %s, %s, %s,
                    %s, %s, %s,
                    %s, %s, %s,
                    %s, %s, %s, %s, %s, now())
            RETURNING id
            """,
            (path.name, checksum, vendor_name, report_date, uploaded_by,
             daily_fields.get("bank"), daily_fields.get("company"),
             daily_fields.get("recipient"), daily_fields.get("sender"),
             daily_fields.get("subject"),
             daily_status, len(daily_rows), daily_error_count,
             rencana_status, len(rencana_rows), rencana_error_count,
             "; ".join(errors) or None,
             daily_fields.get("status_date"), plan_date,
             daily_fields.get("saldo_akhir_0000_total_idr"),
             daily_fields.get("saldo_sementara_0900_total_idr"),
             daily_fields.get("status_cadangan_total_idr"),
             daily_fields.get("saldo_gabungan_total_idr"),
             _rencana_isi_subtotal(rencana_rows)),
        )
        upload_id = cur.fetchone()[0]

        if daily_rows:
            cur.executemany(
                """
                INSERT INTO public.dsr_daily_rows
                    (upload_id, row_no, location, section, flow, line_label, memo_no,
                     denom_100k_idr, denom_50k_idr, denom_20k_idr, denom_10k_idr,
                     denom_5k_idr, denom_2k_idr, denom_1k_idr, line_total_idr, remarks)
                VALUES (%(upload_id)s, %(row_no)s, %(location)s, %(section)s, %(flow)s,
                        %(line_label)s, %(memo_no)s,
                        %(denom_100k)s, %(denom_50k)s, %(denom_20k)s, %(denom_10k)s,
                        %(denom_5k)s, %(denom_2k)s, %(denom_1k)s, %(line_total_idr)s,
                        %(remarks)s)
                """,
                [{"upload_id": upload_id, "location": row.get("location"), **row} for row in daily_rows],
            )

        if rencana_rows:
            cur.executemany(
                """
                INSERT INTO public.dsr_rencana_isi_rows
                    (upload_id, row_no, atm_terminal_id, atm_id, atm_location, denom_config,
                     fill_100k_idr, fill_50k_idr, splank_balance_0800_idr, remarks)
                VALUES (%(upload_id)s, %(row_no)s, %(atm_terminal_id)s, %(atm_id)s,
                        %(atm_location)s, %(denom_config)s, %(fill_100k_idr)s,
                        %(fill_50k_idr)s, %(splank_balance_0800_idr)s, %(remarks)s)
                """,
                [{"upload_id": upload_id, "atm_id": atm_ids.get(row["atm_terminal_id"]), **row}
                 for row in rencana_rows],
            )

    return f"daily={daily_status} rencana_isi={rencana_status}"


def _rencana_isi_subtotal(rows: list[dict[str, Any]]) -> Decimal | None:
    """Sum of the fill columns -- the sheet's own Sub Total row is derived and
    never stored, so this recomputes it for the cross-check against the Daily
    d1 'Subtotal Pengeluaran' total."""
    if not rows:
        return None
    total = Decimal(0)
    for row in rows:
        total += row.get("fill_100k_idr") or Decimal(0)
        total += row.get("fill_50k_idr") or Decimal(0)
    return total


def _json_default(value: Any) -> Any:
    if isinstance(value, Decimal):
        return str(value)
    if isinstance(value, (datetime, date)):
        return value.isoformat()
    raise TypeError(f"Not JSON serializable: {type(value)!r}")


def dry_run_file(conn, path: Path) -> dict[str, Any]:
    """Parses both sheets WITHOUT writing to the DB, then stages the file under
    PENDING_DIR so a later --mode commit can re-parse the identical bytes (never
    trusting data echoed back from the browser for what actually gets persisted).
    """
    vendor_code, uploaded_by, original_name = parse_upload_filename(path.name)
    vendor_name = resolve_vendor_name(conn, vendor_code)
    checksum = file_checksum(path)

    result: dict[str, Any] = {
        "mode": "dry_run",
        "filename": path.name,
        "original_filename": original_name,
        "checksum": checksum,
        "vendor_code": vendor_code,
        "vendor_name": vendor_name,
        "uploaded_by_user_id": uploaded_by,
    }

    try:
        fields, rows, error_count = read_daily_rows(path)
        result["daily"] = {"fields": fields, "rows": rows, "error_count": error_count}
    except Exception as exc:  # tolerate one sheet failing; the other may still be usable
        result["daily"] = {"error": str(exc)}

    try:
        plan_date, rencana_rows = read_rencana_isi_rows(path)
        result["rencana_isi"] = {"plan_date": plan_date, "rows": rencana_rows}
    except Exception as exc:
        result["rencana_isi"] = {"error": str(exc)}

    PENDING_DIR.mkdir(parents=True, exist_ok=True)
    staged = PENDING_DIR / path.name
    if staged.exists():
        staged = PENDING_DIR / f"{path.stem}_{datetime.now():%Y%m%d_%H%M%S}{path.suffix}"
    shutil.move(str(path), str(staged))
    result["staged_filename"] = staged.name

    return result


def resolve_vendor_name(conn, vendor_code: str) -> str:
    with conn.cursor() as cur:
        cur.execute("SELECT name FROM public.vendors WHERE code = %s", (vendor_code,))
        row = cur.fetchone()
    if not row:
        raise ValueError(f"Unknown vendor code '{vendor_code}' -- Go should have rejected this upload")
    return row[0]


def process_file(conn, path: Path) -> str:
    vendor_code, uploaded_by, _original_name = parse_upload_filename(path.name)
    vendor_name = resolve_vendor_name(conn, vendor_code)
    return _upsert_upload(conn, path, vendor_name, uploaded_by)


def parse_args(argv: list[str] | None = None) -> argparse.Namespace:
    parser = argparse.ArgumentParser(description="DSR ETL: batch-drain mode by default.")
    parser.add_argument(
        "--mode", choices=("dry_run", "commit"),
        help="Single-file mode, used by Go's synchronous upload/confirm endpoints. "
             "Omit both --mode and --file for the default batch-drain mode.",
    )
    parser.add_argument(
        "--file",
        help="Filename to process in single-file mode: looked up in FTP_DATA/DSR/ for "
             "--mode dry_run, or FTP_DATA/DSR/pending_confirmation/ for --mode commit.",
    )
    return parser.parse_args(argv)


def main() -> None:
    setup_logging()
    args = parse_args()

    if args.mode == "dry_run":
        if not args.file:
            raise SystemExit("--file is required with --mode dry_run")
        path = INPUT_DIR / args.file
        if not path.exists():
            raise SystemExit(f"File not found: {path}")
        with connect() as conn:
            result = dry_run_file(conn, path)
        print(json.dumps(result, default=_json_default))
        return

    if args.mode == "commit":
        if not args.file:
            raise SystemExit("--file is required with --mode commit")
        path = PENDING_DIR / args.file
        if not path.exists():
            raise SystemExit(f"Staged file not found: {path}")
        with connect() as conn:
            result = process_file(conn, path)
        archived = archive(path)
        print(json.dumps(
            {"mode": "commit", "filename": path.name, "result": result, "archived_to": str(archived)},
            default=_json_default,
        ))
        return

    # Default: batch-drain mode -- retry_scheduler's cron-scan / late-check safety net.
    # Unchanged from before. Never touches PENDING_DIR (glob() here is non-recursive
    # over INPUT_DIR itself), so a file awaiting vendor confirmation is never
    # auto-committed by this path.
    files: list[Path] = []
    for pattern in FILE_PATTERNS:
        files.extend(INPUT_DIR.glob(pattern))
    files.sort()
    if not files:
        logging.info("No files matching %s in %s", FILE_PATTERNS, INPUT_DIR)
        return

    with connect() as conn:
        for path in files:
            try:
                result = process_file(conn, path)
                logging.info("Processed %s (%s), archived to %s", path.name, result, archive(path))
            except Exception:
                logging.exception("Failed to process %s", path.name)
                raise


if __name__ == "__main__":
    main()
