# EOD pipeline image (be-tech-debt F3). Build context = backend_python/.
# Default: eod_retry_scheduler (:8091). DSR service: override the command with
#   python -m service_dsr_etl.run   and set HEALTHCHECK_PORT=8090.
# FTP_DATA and the service .env are mounted/injected at runtime, never baked in.
FROM python:3.14-slim

ENV PYTHONDONTWRITEBYTECODE=1 \
    PYTHONUNBUFFERED=1 \
    TZ=Asia/Jakarta \
    HEALTHCHECK_PORT=8091 \
    RETRY_ETL_DMAA_SCRIPT=dmaa/dmaa_etl.py \
    RETRY_ETL_ITM_CASHPOS_SCRIPT=itm/cashpos/itm_cashpos_etl.py \
    RETRY_ETL_ITM_REPLENISH_SCRIPT=itm/replenish/itm_replenish_etl.py \
    DSR_ETL_ETL_DSR_SCRIPT=dsr/dsr_etl.py
# ETL script defaults are relative to the repo root; here the cwd is backend_python/.
# FTP_DATA: mount at /app/FTP_DATA (default *_FTP_DATA_ROOT) or set *_FTP_DATA_ROOT.

WORKDIR /app

# All pinned requirements in one resolve, so a version conflict fails the build.
RUN --mount=type=bind,target=/src \
    pip install --no-cache-dir \
      -r /src/eod_retry_scheduler/requirements.txt \
      -r /src/service_dsr_etl/requirements.txt \
      -r /src/dsr/requirements.txt \
      -r /src/dmaa/requirements.txt \
      -r /src/itm/cashpos/requirements.txt \
      -r /src/itm/replenish/requirements.txt

COPY . .

RUN useradd --system --uid 10001 --no-create-home app
USER app

EXPOSE 8090 8091

# slim has no curl/wget; /health needs no auth.
HEALTHCHECK --interval=30s --timeout=5s --start-period=20s --retries=3 \
  CMD python -c "import os, urllib.request; urllib.request.urlopen('http://127.0.0.1:' + os.environ['HEALTHCHECK_PORT'] + '/health', timeout=4)"

CMD ["python", "-m", "eod_retry_scheduler.run"]
