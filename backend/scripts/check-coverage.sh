#!/usr/bin/env bash
# Fail when a package in coverage-thresholds.txt is missing from the `go test -cover`
# output or its coverage is below the threshold.
# Usage (from backend/): scripts/check-coverage.sh <go-test-output> [thresholds-file]
set -euo pipefail

log=$1
thresholds=${2:-coverage-thresholds.txt}

awk '
  NR == FNR {
    # go test lines: "ok  <pkg>  1.2s  coverage: 81.6% of statements"
    if ($1 == "ok" && match($0, /coverage: [0-9.]+%/)) {
      got[$2] = substr($0, RSTART + 10, RLENGTH - 11)
    }
    next
  }
  /^[[:space:]]*(#|$)/ { next }
  {
    pkg = $1; min = $2
    if (!(pkg in got)) { printf "MISSING %s (no coverage line)\n", pkg; bad = 1; next }
    if (got[pkg] + 0 < min + 0) { printf "FAIL    %s %s%% < %s%%\n", pkg, got[pkg], min; bad = 1; next }
    printf "ok      %s %s%% >= %s%%\n", pkg, got[pkg], min
  }
  END { exit bad }
' "$log" "$thresholds"
