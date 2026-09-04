"""Settings for the DSR ETL service (env-driven, DSR_ETL_ prefix).

Split out of retry_scheduler -- owns only the `dsr` file type (the vendor
upload dry-run/confirm flow, see backend_python/dsr/dsr_etl.py), separate from the
batch-shaped dmaa/itm_cashpos/itm_replenish ETLs now in eod_retry_scheduler.
"""
from __future__ import annotations

from datetime import time
from pathlib import Path

from pydantic import field_validator
from pydantic_settings import BaseSettings, SettingsConfigDict


class Settings(BaseSettings):
    model_config = SettingsConfigDict(env_prefix="DSR_ETL_", env_file=".env", extra="ignore")

    # Database
    database_url: str = "postgresql://postgres:postgres@localhost:5432/cms"

    # Auth
    auth_secret: str = "change_me"
    auth_mode: str = "api_key"  # "api_key" or "jwt"

    # SLA deadline (WIB = Asia/Jakarta)
    sla_dsr: str = "09:00"

    # Retry settings
    retry_interval_minutes: int = 30
    max_auto_retries: int = 3
    db_retry_max_attempts: int = 3
    db_retry_base_delay_seconds: float = 1.0

    # Detection schedule (cron window)
    scan_cron_start_hour: int = 5
    scan_cron_end_hour: int = 9
    scan_interval_minutes: int = 15

    # Filesystem paths
    ftp_data_root: Path = Path("FTP_DATA")
    dsr_input_dir: Path | None = None
    dsr_not_processed_dir: Path | None = None

    # ETL script path
    etl_dsr_script: Path = Path("backend_python/dsr/dsr_etl.py")

    # Service
    service_version: str = "1.0.0"
    log_level: str = "INFO"

    @field_validator("sla_dsr")
    @classmethod
    def validate_time_format(cls, v: str) -> str:
        parts = v.split(":")
        if len(parts) != 2:
            raise ValueError(f"Invalid time format: {v}. Expected HH:MM")
        try:
            hour, minute = int(parts[0]), int(parts[1])
        except ValueError as exc:
            raise ValueError(f"Invalid time format: {v}. Expected HH:MM") from exc
        if not (0 <= hour <= 23 and 0 <= minute <= 59):
            raise ValueError(f"Invalid time value: {v}")
        return v

    def get_sla_time(self, file_type: str) -> time:
        mapping = {"dsr": self.sla_dsr}
        h, m = map(int, mapping[file_type].split(":"))
        return time(h, m)

    def not_processed_dir(self, file_type: str) -> Path:
        explicit = {"dsr": self.dsr_not_processed_dir}[file_type]
        if explicit:
            return explicit
        return self.input_dir(file_type) / "not_processed"

    def input_dir(self, file_type: str) -> Path:
        explicit = {"dsr": self.dsr_input_dir}[file_type]
        if explicit:
            return explicit
        default = {"dsr": self.ftp_data_root / "DSR"}
        return default[file_type]

    def file_pattern(self, file_type: str) -> str:
        return FILE_PATTERNS[file_type]

    def etl_script(self, file_type: str) -> Path:
        mapping = {"dsr": self.etl_dsr_script}
        return mapping[file_type]


FILE_TYPES = ("dsr",)

FILE_PATTERNS = {
    "dsr": "*__*.xls*",
}
