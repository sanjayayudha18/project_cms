from __future__ import annotations

import json
import logging
import re
from dataclasses import dataclass
from decimal import Decimal, InvalidOperation
from pathlib import Path
from typing import Any

import pandas as pd

import shutil
from datetime import datetime

# =========================
# CONFIG
# =========================
from pathlib import Path

INPUT_DIR = Path(r"C:\Users\RB Yudha Rangga\OneDrive\Documents\Development\CMS2\FTP_DATA\DMAA")
OUTPUT_DIR = INPUT_DIR / "output"
LOG_FILE = OUTPUT_DIR / "etl.log"
SUMMARY_FILE = OUTPUT_DIR / "summary.json"
ERROR_FILE = OUTPUT_DIR / "validation_errors.csv"
BACKUP_DIR = INPUT_DIR / "backup"


@dataclass(frozen=True)
class FieldRule:
    name: str
    kind: str  # string | integer | decimal | date | boolean
    required: bool = False


SCHEMA: list[FieldRule] = [
    FieldRule("atm_id", "string", required=True),
    FieldRule("periode_pred", "date", required=True),
    FieldRule("denomination", "integer", required=True),
    FieldRule("amount_replenish", "decimal", required=True),
    FieldRule("location", "string", required=True),
]

EXPECTED_COLUMNS = {field.name for field in SCHEMA}


# =========================
# LOGGING
# =========================
def setup_logging() -> None:
    OUTPUT_DIR.mkdir(parents=True, exist_ok=True)
    logging.basicConfig(
        level=logging.INFO,
        format="%(asctime)s | %(levelname)s | %(message)s",
        handlers=[
            logging.FileHandler(LOG_FILE, encoding="utf-8"),
            logging.StreamHandler(),
        ],
    )


# =========================
# HELPERS
# =========================
def to_snake_case(value: str) -> str:
    value = value.strip().lower()
    value = re.sub(r"[^a-z0-9]+", "_", value)
    value = re.sub(r"_+", "_", value)
    return value.strip("_")


def normalize_headers(df: pd.DataFrame) -> pd.DataFrame:
    df = df.copy()
    df.columns = [to_snake_case(str(col)) for col in df.columns]
    return df


def trim_string_cells(df: pd.DataFrame) -> pd.DataFrame:
    df = df.copy()
    for col in df.columns:
        if pd.api.types.is_object_dtype(df[col]):
            df[col] = df[col].apply(
                lambda x: x.strip() if isinstance(x, str) else x
            )
    return df


def normalize_empty_values(df: pd.DataFrame) -> pd.DataFrame:
    df = df.copy()
    empty_like = {"", "null", "none", "nan", "na", "n/a", "-"}

    def normalize_value(x: Any) -> Any:
        if pd.isna(x):
            return pd.NA
        if isinstance(x, str) and x.strip().lower() in empty_like:
            return pd.NA
        return x

    for col in df.columns:
        df[col] = df[col].map(normalize_value)
    return df


def parse_integer(value: Any) -> tuple[Any, str | None]:
    if pd.isna(value):
        return pd.NA, None
    try:
        if isinstance(value, str):
            value = value.replace(",", "").strip()
        number = Decimal(str(value))
        if number != number.to_integral_value():
            return value, "not a whole number"
        return int(number), None
    except (InvalidOperation, ValueError):
        return value, "invalid integer"


def parse_decimal(value: Any) -> tuple[Any, str | None]:
    if pd.isna(value):
        return pd.NA, None
    try:
        if isinstance(value, str):
            value = value.replace(",", "").strip()
        return Decimal(str(value)), None
    except (InvalidOperation, ValueError):
        return value, "invalid decimal"


def parse_date(value: Any) -> tuple[Any, str | None]:
    if pd.isna(value):
        return pd.NaT, None
    parsed = pd.to_datetime(value, errors="coerce")
    if pd.isna(parsed):
        return value, "invalid date"
    return parsed.normalize(), None


def parse_boolean(value: Any) -> tuple[Any, str | None]:
    if pd.isna(value):
        return pd.NA, None

    if isinstance(value, bool):
        return value, None

    if isinstance(value, str):
        normalized = value.strip().lower()
        mapping = {
            "true": True,
            "false": False,
            "yes": True,
            "no": False,
            "y": True,
            "n": False,
            "1": True,
            "0": False,
        }
        if normalized in mapping:
            return mapping[normalized], None

    if value in (1, 0):
        return bool(value), None

    return value, "invalid boolean"


def parse_string(value: Any) -> tuple[Any, str | None]:
    if pd.isna(value):
        return pd.NA, None
    return str(value).strip(), None


PARSERS = {
    "string": parse_string,
    "integer": parse_integer,
    "decimal": parse_decimal,
    "date": parse_date,
    "boolean": parse_boolean,
}


def validate_and_cast(df: pd.DataFrame, source_file: str) -> tuple[pd.DataFrame, list[dict[str, Any]]]:
    df = df.copy()
    errors: list[dict[str, Any]] = []

    for field in SCHEMA:
        if field.name not in df.columns:
            errors.append(
                {
                    "source_file": source_file,
                    "row_number": None,
                    "column": field.name,
                    "value": None,
                    "error": "missing required column",
                }
            )
            continue

        cleaned_values = []
        parser = PARSERS[field.kind]

        for idx, raw_value in df[field.name].items():
            parsed_value, err = parser(raw_value)

            if field.required and (pd.isna(parsed_value) or parsed_value is pd.NA):
                errors.append(
                    {
                        "source_file": source_file,
                        "row_number": int(idx) + 2,
                        "column": field.name,
                        "value": raw_value,
                        "error": "required value is empty",
                    }
                )
            elif err:
                errors.append(
                    {
                        "source_file": source_file,
                        "row_number": int(idx) + 2,
                        "column": field.name,
                        "value": raw_value,
                        "error": err,
                    }
                )

            cleaned_values.append(parsed_value)

        df[field.name] = cleaned_values

    return df, errors


def enforce_column_set(df: pd.DataFrame, source_file: str) -> list[dict[str, Any]]:
    errors: list[dict[str, Any]] = []
    extra_columns = sorted(set(df.columns) - EXPECTED_COLUMNS)
    missing_columns = sorted(EXPECTED_COLUMNS - set(df.columns))

    for col in extra_columns:
        errors.append(
            {
                "source_file": source_file,
                "row_number": None,
                "column": col,
                "value": None,
                "error": "unexpected column",
            }
        )

    for col in missing_columns:
        errors.append(
            {
                "source_file": source_file,
                "row_number": None,
                "column": col,
                "value": None,
                "error": "missing expected column",
            }
        )

    return errors

def move_to_backup(path: Path) -> Path:
    BACKUP_DIR.mkdir(parents=True, exist_ok=True)

    target = BACKUP_DIR / path.name
    if target.exists():
        stamp = datetime.now().strftime("%Y%m%d_%H%M%S")
        target = BACKUP_DIR / f"{path.stem}_{stamp}{path.suffix}"

    shutil.move(str(path), str(target))
    return target

# =========================
# MAIN PROCESS
# =========================
def process_file(path: Path) -> dict[str, Any]:
    logging.info("Processing file: %s", path.name)

    df = pd.read_excel(path)
    original_rows = len(df)

    df = normalize_headers(df)
    df = trim_string_cells(df)
    df = normalize_empty_values(df)

    structure_errors = enforce_column_set(df, path.name)
    df, validation_errors = validate_and_cast(df, path.name)
    all_errors = structure_errors + validation_errors

    # standard output formatting
    if "periode_pred" in df.columns:
        df["periode_pred"] = pd.to_datetime(df["periode_pred"], errors="coerce").dt.strftime("%Y-%m-%d")

    if "denomination" in df.columns:
        df["denomination"] = pd.Series(df["denomination"], dtype="Int64")

    if "amount_replenish" in df.columns:
        df["amount_replenish"] = df["amount_replenish"].apply(
            lambda x: float(x) if isinstance(x, Decimal) else x
        )

    cleaned_path = OUTPUT_DIR / f"cleaned_{path.stem}.csv"
    df.to_csv(cleaned_path, index=False)

    status = "success" if not all_errors else "partial_success"
    logging.info(
        "Finished file: %s | rows=%s | errors=%s | output=%s",
        path.name,
        original_rows,
        len(all_errors),
        cleaned_path.name,
    )

    return {
        "file": path.name,
        "rows": original_rows,
        "status": status,
        "error_count": len(all_errors),
        "output_file": cleaned_path.name,
        "errors": all_errors,
    }


def main() -> None:
    setup_logging()
    logging.info("ETL started")

    excel_files = sorted(INPUT_DIR.glob("*.xlsx"))
    if not excel_files:
        logging.error("No .xlsx files found in %s", INPUT_DIR.resolve())
        raise FileNotFoundError(f"No .xlsx files found in {INPUT_DIR}")

    summary: list[dict[str, Any]] = []
    all_errors: list[dict[str, Any]] = []

    for file_path in excel_files:
        try:
            result = process_file(file_path)

            backup_file = None
            if result["status"] == "success":
                moved_path = move_to_backup(file_path)
                backup_file = str(moved_path)
                logging.info("Moved source file to backup: %s", moved_path)
            else:
                logging.info(
                    "Source file kept in input folder because status=%s: %s",
                    result["status"],
                    file_path.name,
                )

            summary.append(
                {
                    "file": result["file"],
                    "rows": result["rows"],
                    "status": result["status"],
                    "error_count": result["error_count"],
                    "output_file": result["output_file"],
                    "backup_file": backup_file,
                }
            )
            all_errors.extend(result["errors"])

        except Exception as exc:
            logging.exception("Fatal error while processing %s", file_path.name)
            summary.append(
                {
                    "file": file_path.name,
                    "rows": None,
                    "status": "error",
                    "error_count": 1,
                    "output_file": None,
                    "backup_file": None,
                    "message": str(exc),
                }
            )
            all_errors.append(
                {
                    "source_file": file_path.name,
                    "row_number": None,
                    "column": None,
                    "value": None,
                    "error": str(exc),
                }
            )

    with SUMMARY_FILE.open("w", encoding="utf-8") as f:
        json.dump(summary, f, indent=2, ensure_ascii=False, default=str)

    pd.DataFrame(all_errors).to_csv(ERROR_FILE, index=False)

    success_count = sum(1 for item in summary if item["status"] == "success")
    partial_count = sum(1 for item in summary if item["status"] == "partial_success")
    error_count = sum(1 for item in summary if item["status"] == "error")

    logging.info(
        "ETL finished | success=%s | partial_success=%s | error=%s | validation_errors=%s",
        success_count,
        partial_count,
        error_count,
        len(all_errors),
    )


if __name__ == "__main__":
    main()