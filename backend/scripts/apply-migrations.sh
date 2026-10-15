#!/usr/bin/env bash
# Apply every top-level migration in numeric order to an empty database (CI only).
# Connection comes from the PG* env vars (PGHOST, PGUSER, PGPASSWORD, PGDATABASE, ...),
# never from argv. Run from backend/.
# Temporary: replaced by the cmd/migrate runner in be-tech-debt slice D3.
set -euo pipefail

for f in $(ls migrations/[0-9][0-9][0-9]_*.sql | sort); do
  echo "applying $f"
  psql -v ON_ERROR_STOP=1 -X -q -f "$f"
done
