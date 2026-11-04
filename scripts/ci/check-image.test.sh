#!/usr/bin/env bash
# Fixture test for scripts/ci/check-image.sh (Property 1-3).
# Spec: .kiro/specs/backend-exq-deploy-readiness/. Usage: bash scripts/ci/check-image.test.sh
set -euo pipefail

here="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
tag_prefix="cms-check-image-fixture"
HC="HEALTHCHECK CMD true"

# name|dockerfile lines (\n separated)|expected exit|user value that must appear ("" = none)
fixtures=(
  "no-user|$HC|1|"
  "user-root|USER root\n$HC|1|root"
  "user-0|USER 0\n$HC|1|0"
  "user-root-group|USER root:nogroup\n$HC|1|root:nogroup"
  "user-0-0|USER 0:0\n$HC|1|0:0"
  "no-healthcheck|USER 65534|1|"
  "healthcheck-none|USER 65534\nHEALTHCHECK NONE|1|"
  "ok-uid|USER 65534\n$HC|0|"
  "ok-uid-gid|USER 65534:65534\n$HC|0|"
  "ok-name|USER nobody\n$HC|0|"
)

cleanup() {
  for f in "${fixtures[@]}"; do
    docker image rm -f "${tag_prefix}-${f%%|*}" >/dev/null 2>&1 || true
  done
}
trap cleanup EXIT

failed=()
for f in "${fixtures[@]}"; do
  IFS='|' read -r name body want user <<<"$f"
  image="${tag_prefix}-${name}"
  printf 'FROM alpine:3.24\n%b\n' "$body" | docker build -q -t "$image" - >/dev/null

  set +e
  out="$(bash "$here/check-image.sh" "$image" "$name" 2>&1)"
  got=$?
  set -e

  problem=""
  if [ "$got" -ne "$want" ]; then
    problem="exit $got, want $want"
  elif [ "$want" -ne 0 ]; then
    case "$out" in *"$name"*) ;; *) problem="output lacks module name" ;; esac
    if [ -n "$user" ]; then
      case "$out" in *"Config.User='${user}'"*) ;; *) problem="output lacks user '${user}'" ;; esac
    fi
  fi

  if [ -n "$problem" ]; then
    echo "FAIL  $name: $problem"
    echo "      output: $out"
    failed+=("$name")
  else
    echo "ok    $name (exit $got)"
  fi
done

if [ "${#failed[@]}" -ne 0 ]; then
  echo "mismatched fixtures: ${failed[*]}" >&2
  exit 1
fi
echo "all ${#fixtures[@]} fixtures match"
