from __future__ import annotations

import hashlib
import os
import shutil
from dataclasses import dataclass
from datetime import datetime
from pathlib import Path

import pandas as pd
import psycopg
from psycopg.rows import dict_row


# Use a single file path OR a wildcard pattern.
INPUT_PATTERN = (
    r"C:\Users\RB Yudha Rangga\OneDrive\Documents\Development\CMS2\FTP_DATA\DMAA\output\cleaned_Order_All_*.csv"
)

# "latest" = process only the newest matching file (recommended)
# "all" = process every matching file
PROCESS_MODE = "latest"

REQUIRED_COLUMNS = {
    "atm_id",
    "periode_pred",
    "denomination",
    "amount_replenish",
    "location",
}

WILDCARD_CHARS = {"*", "?", "["}


@dataclass
class LoadResult:
    dmaa_file_id: int
    file_name: str
    row_count: int
    success_count: int
    archived_to: str | None = None


class ValidationError(Exception):
    pass


def has_wildcard(path_value: str) -> bool:
    return any(char in path_value for char in WILDCARD_CHARS)


def resolve_input_files(input_pattern: str | Path, process_mode: str = "latest") -> list[Path]:
    raw_pattern = str(input_pattern)
    pattern_path = Path(raw_pattern)

    if not has_wildcard(raw_pattern):
        resolved = pattern_path.resolve()
        if not resolved.exists() or not resolved.is_file():
            raise FileNotFoundError(f"File not found: {resolved}")
        return [resolved]

    parent_dir = pattern_path.parent
    file_pattern = pattern_path.name

    if not parent_dir.exists() or not parent_dir.is_dir():
        raise FileNotFoundError(f"Base folder not found: {parent_dir}")

    matched_files = [path.resolve() for path in parent_dir.glob(file_pattern) if path.is_file()]
    if not matched_files:
        raise FileNotFoundError(f"No files matched pattern: {raw_pattern}")

    if process_mode not in {"latest", "all"}:
        raise ValidationError("PROCESS_MODE must be either 'latest' or 'all'.")

    if process_mode == "latest":
        latest_file = max(matched_files, key=lambda path: path.stat().st_mtime)
        return [latest_file]

    return sorted(matched_files, key=lambda path: (path.stat().st_mtime, path.name))


def sha256_file(file_path: Path) -> str:
    digest = hashlib.sha256()
    with file_path.open("rb") as handle:
        for chunk in iter(lambda: handle.read(1024 * 1024), b""):
            digest.update(chunk)
    return digest.hexdigest()


def get_file_date_from_modified_time(file_path: Path):
    return datetime.fromtimestamp(file_path.stat().st_mtime).date()


def get_source_system(file_path: Path) -> str:
    parent_name = file_path.parent.name.strip()
    if not parent_name:
        raise ValidationError("Cannot derive source_system from folder name.")
    return parent_name


# If you want source_system = DMAA instead of output, replace the function above with:
# def get_source_system(file_path: Path) -> str:
#     return file_path.parents[1].name.strip()


def validate_and_transform(df: pd.DataFrame) -> pd.DataFrame:
    missing = REQUIRED_COLUMNS - set(df.columns)
    if missing:
        raise ValidationError(f"Missing required columns: {sorted(missing)}")

    data = df.copy()

    for column in ["atm_id", "periode_pred", "denomination", "amount_replenish"]:
        if data[column].isna().any():
            raise ValidationError(f"Column '{column}' contains null values.")

    data["atm_id"] = data["atm_id"].astype(str).str.strip()
    if (data["atm_id"] == "").any():
        raise ValidationError("Column 'atm_id' contains blank values.")

    data["periode_pred"] = pd.to_datetime(data["periode_pred"], errors="raise").dt.date

    denom_numeric = pd.to_numeric(data["denomination"], errors="raise")
    if (denom_numeric <= 0).any():
        raise ValidationError("Column 'denomination' must be > 0.")
    if ((denom_numeric % 1) != 0).any():
        raise ValidationError("Column 'denomination' must contain integer values only.")
    data["denom"] = denom_numeric.astype("int64")

    amount_numeric = pd.to_numeric(data["amount_replenish"], errors="raise")
    if (amount_numeric < 0).any():
        raise ValidationError("Column 'amount_replenish' must be >= 0.")
    if ((amount_numeric % 1) != 0).any():
        raise ValidationError("Column 'amount_replenish' must contain integer values only.")
    data["amount_replenish"] = amount_numeric.astype("int64")

    duplicated = data.duplicated(subset=["atm_id", "periode_pred", "denom"], keep=False)
    if duplicated.any():
        sample = data.loc[duplicated, ["atm_id", "periode_pred", "denom"]].head(10).to_dict("records")
        raise ValidationError(
            "Duplicate business keys found in CSV for (atm_id, periode_pred, denom). "
            f"Sample: {sample}"
        )

    data["amount_refund"] = 0

    return data[["atm_id", "periode_pred", "denom", "amount_replenish", "amount_refund"]]


def get_connection():
    return psycopg.connect(
        host=os.getenv("PGHOST", "localhost"),
        port=int(os.getenv("PGPORT", "5432")),
        dbname=os.getenv("PGDATABASE", "cms"),
        user=os.getenv("PGUSER", "postgres"),
        password=os.getenv("PGPASSWORD","1818"),
        row_factory=dict_row,
    )


def move_file_to_bak(file_path: Path) -> Path:
    bak_dir = file_path.parent / "BAK"
    bak_dir.mkdir(parents=True, exist_ok=True)

    target = bak_dir / file_path.name
    if target.exists():
        timestamp = datetime.now().strftime("%Y%m%d_%H%M%S")
        target = bak_dir / f"{file_path.stem}_{timestamp}{file_path.suffix}"

    shutil.move(str(file_path), str(target))
    return target


def ensure_expected_schema(conn) -> None:
    expected_files_columns = {
        "id",
        "name",
        "status",
        "is_valid",
        "file_date",
        "source_system",
        "checksum",
        "row_count",
        "success_count",
        "error_count",
        "error_message",
        "processed_at",
        "created_at",
        "updated_at",
    }
    expected_forecast_columns = {
        "atm_id",
        "dmaa_file_id",
        "periode_pred",
        "denom",
        "amount_replenish",
        "amount_refund",
        "created_at",
    }

    with conn.cursor() as cur:
        cur.execute(
            """
            SELECT table_name, column_name
            FROM information_schema.columns
            WHERE table_schema = 'public'
              AND table_name IN ('dmaa_files', 'dmaa_atm_forecast')
            """
        )
        rows = cur.fetchall()

    actual = {"dmaa_files": set(), "dmaa_atm_forecast": set()}
    for row in rows:
        actual[row["table_name"]].add(row["column_name"])

    missing_files = expected_files_columns - actual["dmaa_files"]
    missing_forecast = expected_forecast_columns - actual["dmaa_atm_forecast"]

    if missing_files or missing_forecast:
        problems = []
        if missing_files:
            problems.append(f"dmaa_files missing columns: {sorted(missing_files)}")
        if missing_forecast:
            problems.append(f"dmaa_atm_forecast missing columns: {sorted(missing_forecast)}")
        raise ValidationError("; ".join(problems))


def load_csv_to_postgres(file_path: str | Path) -> LoadResult:
    path = Path(file_path).resolve()
    if not path.exists() or not path.is_file():
        raise FileNotFoundError(f"File not found: {path}")

    checksum = sha256_file(path)
    file_name = path.name
    file_date = get_file_date_from_modified_time(path)
    source_system = get_source_system(path)

    raw_df = pd.read_csv(path)
    data_df = validate_and_transform(raw_df)

    row_count = len(raw_df)
    if row_count == 0:
        raise ValidationError("CSV is empty.")

    with get_connection() as conn:
        ensure_expected_schema(conn)

        with conn.transaction():
            with conn.cursor() as cur:
                cur.execute(
                    """
                    SELECT id
                    FROM public.dmaa_files
                    WHERE checksum = %s
                    """,
                    (checksum,),
                )
                existing = cur.fetchone()
                if existing:
                    raise ValidationError(
                        f"Duplicate file rejected. checksum already exists in dmaa_files.id={existing['id']}"
                    )

                cur.execute(
                    """
                    INSERT INTO public.dmaa_files (
                        name,
                        status,
                        is_valid,
                        file_date,
                        source_system,
                        checksum,
                        row_count,
                        success_count,
                        error_count,
                        error_message,
                        processed_at,
                        updated_at
                    )
                    VALUES (%s, %s, %s, %s, %s, %s, %s, %s, %s, %s, NOW(), NOW())
                    RETURNING id
                    """,
                    (
                        file_name,
                        "processing",
                        True,
                        file_date,
                        source_system,
                        checksum,
                        row_count,
                        0,
                        0,
                        None,
                    ),
                )
                dmaa_file_id = cur.fetchone()["id"]

                detail_rows = [
                    (
                        row.atm_id,
                        dmaa_file_id,
                        row.periode_pred,
                        int(row.denom),
                        int(row.amount_replenish),
                        int(row.amount_refund),
                    )
                    for row in data_df.itertuples(index=False)
                ]

                cur.executemany(
                    """
                    INSERT INTO public.dmaa_atm_forecast (
                        atm_id,
                        dmaa_file_id,
                        periode_pred,
                        denom,
                        amount_replenish,
                        amount_refund
                    )
                    VALUES (%s, %s, %s, %s, %s, %s)
                    """,
                    detail_rows,
                )

                success_count = len(detail_rows)

                cur.execute(
                    """
                    UPDATE public.dmaa_files
                    SET
                        status = %s,
                        is_valid = %s,
                        success_count = %s,
                        error_count = %s,
                        error_message = %s,
                        processed_at = NOW(),
                        updated_at = NOW()
                    WHERE id = %s
                    """,
                    (
                        "success",
                        True,
                        success_count,
                        0,
                        None,
                        dmaa_file_id,
                    ),
                )

    archived_to = move_file_to_bak(path)

    return LoadResult(
        dmaa_file_id=dmaa_file_id,
        file_name=file_name,
        row_count=row_count,
        success_count=success_count,
        archived_to=str(archived_to),
    )


def process_inputs(input_pattern: str | Path, process_mode: str = "latest") -> list[LoadResult]:
    input_files = resolve_input_files(input_pattern, process_mode=process_mode)
    results = []

    for file_path in input_files:
        result = load_csv_to_postgres(file_path)
        results.append(result)

    return results


if __name__ == "__main__":
    results = process_inputs(INPUT_PATTERN, process_mode=PROCESS_MODE)
    for result in results:
        print(
            {
                "dmaa_file_id": result.dmaa_file_id,
                "file_name": result.file_name,
                "row_count": result.row_count,
                "success_count": result.success_count,
                "archived_to": result.archived_to,
            }
        )
