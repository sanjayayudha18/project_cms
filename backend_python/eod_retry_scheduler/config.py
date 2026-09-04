"""Settings for the EOD Retry Scheduler service (env-driven, RETRY_ prefix).

Handles dmaa, itm_cashpos, itm_replenish only -- DSR was split out into the
sibling service_dsr_etl service (separate FastAPI app, separate port), since
DSR has its own vendor-upload dry-run/commit flow that doesn't belong next to
the batch-shaped ETLs here.
"""
from __future__ import annotations

from datetime import time
from pathlib import Path

from pydantic import field_validator
from pydantic_settings import BaseSettings, SettingsConfigDict


class Settings(BaseSettings):
    model_config = SettingsConfigDict(env_prefix="RETRY_", env_file=".env", extra="ignore")

    # Database
    database_url: str = "postgresql://postgres:postgres@localhost:5432/cms"

    # Auth
    auth_secret: str = "change_me"
    auth_mode: str = "api_key"  # "api_key" or "jwt"

    # SLA deadlines (WIB = Asia/Jakarta)
    sla_dmaa: str = "06:00"
    sla_itm_cashpos: str = "07:00"
    sla_itm_replenish: str = "07:00"

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
    dmaa_input_dir: Path | None = None
    dmaa_not_processed_dir: Path | None = None
    itm_cashpos_input_dir: Path | None = None
    itm_cashpos_not_processed_dir: Path | None = None
    itm_replenish_input_dir: Path | None = None
    itm_replenish_not_processed_dir: Path | None = None

    # ETL script paths
    etl_dmaa_script: Path = Path("scheduler/dmaa/dmaa_etl.py")
    etl_itm_cashpos_script: Path = Path("scheduler/itm/cashpos/itm_cashpos_etl.py")
    etl_itm_replenish_script: Path = Path("scheduler/itm/replenish/itm_replenish_etl.py")

    # Service
    service_version: str = "1.0.0"
    log_level: str = "INFO"

    @field_validator("sla_dmaa", "sla_itm_cashpos", "sla_itm_replenish")
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
        mapping = {
            "dmaa": self.sla_dmaa,
            "itm_cashpos": self.sla_itm_cashpos,
            "itm_replenish": self.sla_itm_replenish,
        }
        h, m = map(int, mapping[file_type].split(":"))
        return time(h, m)

    def not_processed_dir(self, file_type: str) -> Path:
        explicit = {
            "dmaa": self.dmaa_not_processed_dir,
            "itm_cashpos": self.itm_cashpos_not_processed_dir,
            "itm_replenish": self.itm_replenish_not_processed_dir,
        }[file_type]
        if explicit:
            return explicit
        return self.input_dir(file_type) / "not_processed"

    def input_dir(self, file_type: str) -> Path:
        explicit = {
            "dmaa": self.dmaa_input_dir,
            "itm_cashpos": self.itm_cashpos_input_dir,
            "itm_replenish": self.itm_replenish_input_dir,
        }[file_type]
        if explicit:
            return explicit
        default = {
            "dmaa": self.ftp_data_root / "DMAA",
            "itm_cashpos": self.ftp_data_root / "ITM" / "atm_caspos",
            "itm_replenish": self.ftp_data_root / "ITM" / "atm_replenish",
        }
        return default[file_type]

    def file_pattern(self, file_type: str) -> str:
        return FILE_PATTERNS[file_type]

    def etl_script(self, file_type: str) -> Path:
        mapping = {
            "dmaa": self.etl_dmaa_script,
            "itm_cashpos": self.etl_itm_cashpos_script,
            "itm_replenish": self.etl_itm_replenish_script,
        }
        return mapping[file_type]


FILE_TYPES = ("dmaa", "itm_cashpos", "itm_replenish")

FILE_PATTERNS = {
    "dmaa": "Order_All_*.xlsx",
    "itm_cashpos": "ATM_Cashpos_*.csv",
    "itm_replenish": "ATM_Replenish_*.csv",
}
