#!/usr/bin/env bash
# Fails when a built image would run as root or has no HEALTHCHECK.
# Spec: .kiro/specs/backend-exq-deploy-readiness/ (Req 2). Called by ci.yml job go-image.
# Usage: bash scripts/ci/check-image.sh <image> <module>
set -euo pipefail

if [ "$#" -ne 2 ]; then
  echo "usage: $0 <image> <module>" >&2
  exit 2
fi
image="$1"
module="$2"

user="$(docker image inspect --format '{{.Config.User}}' "$image")"
healthcheck="$(docker image inspect --format '{{if .Config.Healthcheck}}{{join .Config.Healthcheck.Test " "}}{{end}}' "$image")"

# USER may be "name", "uid", "name:group" or "uid:gid"; only the user part decides root.
user_part="${user%%:*}"
case "$user_part" in
  "" | root | 0)
    echo "::error::${module} image runs as root (Config.User='${user}'); add a non-root USER to ${module}/Dockerfile"
    exit 1
    ;;
esac

# "HEALTHCHECK NONE" stores Test=["NONE"]; no HEALTHCHECK stores nothing.
case "$healthcheck" in
  "" | NONE)
    echo "::error::${module} image has no HEALTHCHECK; add one to ${module}/Dockerfile"
    exit 1
    ;;
esac

echo "${module}: user=${user} healthcheck=${healthcheck}"
