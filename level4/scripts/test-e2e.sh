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
  private-facts-gateway \
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
grep -R "selected_public_catalog_url" logs/consumer.jsonl >/dev/null
grep -R "selected_private_facts_url" logs/consumer.jsonl >/dev/null
grep -R "direct_catalog_url_not_used" logs/consumer.jsonl >/dev/null
grep -R "serve_private_facts" logs/private-facts-gateway.jsonl >/dev/null
grep -R "serve_signed_catalog" logs/enterprise-a-registry.jsonl >/dev/null

if [ -f logs/enterprise-b-registry.jsonl ] && grep "serve_signed_catalog" logs/enterprise-b-registry.jsonl >/dev/null; then
  echo "enterprise-b direct catalog was used; expected PrivateFactsURL only" >&2
  exit 1
fi

docker compose logs --tail=80 swimlane
