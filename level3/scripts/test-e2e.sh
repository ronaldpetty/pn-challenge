#!/usr/bin/env sh
set -eu

SCRIPT_DIR=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)
REPO_ROOT=$(CDPATH= cd -- "$SCRIPT_DIR/.." && pwd)

cd "$REPO_ROOT"
rm -rf artifacts logs
mkdir -p bin artifacts logs

cleanup() {
  docker compose down --remove-orphans >/dev/null 2>&1 || true
}
trap cleanup EXIT

docker compose up --build -d \
  nanda-index \
  credential-rotator \
  enterprise-a-registry \
  enterprise-b-registry \
  enterprise-a-reverse \
  enterprise-a-uppercase \
  enterprise-b-truncate \
  enterprise-b-count \
  consumer \
  swimlane

sleep 35

grep -R "credential_rotated_registry_addr" logs >/dev/null
grep -R "credential_rotated_catalog" logs >/dev/null
grep -R "verification_failed_registry_addr" logs >/dev/null
grep -R "verification_failed_enterprise_catalog" logs >/dev/null
grep -R "verification_recovered_registry_addr" logs >/dev/null
grep -R "verification_recovered_enterprise_catalog" logs >/dev/null
grep -R "tool_result" logs >/dev/null

docker compose logs --tail=80 swimlane
