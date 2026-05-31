#!/usr/bin/env sh
set -eu

SCRIPT_DIR=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)
REPO_ROOT=$(CDPATH= cd -- "$SCRIPT_DIR/.." && pwd)

cd "$REPO_ROOT"
rm -rf artifacts logs
mkdir -p bin artifacts logs

cleanup() {
  docker rm -f level8-e2e-existing-agent level8-e2e-lab-agent level8-e2e-lab-registry >/dev/null 2>&1 || true
  docker compose down --remove-orphans >/dev/null 2>&1 || true
}
trap cleanup EXIT
cleanup

docker compose up --build -d \
  nanda-index-a \
  nanda-index-b \
  credential-rotator \
  issuer-key-rotator \
  private-facts-gateway \
  revocation-authority \
  crdt-update-bus \
  enterprise-a-registry \
  enterprise-b-registry \
  enterprise-a-reverse \
  enterprise-a-uppercase \
  enterprise-b-truncate \
  enterprise-b-count \
  consumer \
  swimlane

sleep 55

grep -R "registry_joined_quilt" logs/nanda-a.jsonl >/dev/null
grep -R "registry_joined_quilt" logs/nanda-b.jsonl >/dev/null
grep -R "registry_refreshed_quilt" logs/nanda-a.jsonl >/dev/null
grep -R "registry_refreshed_quilt" logs/nanda-b.jsonl >/dev/null
grep -R "discover_federated_index" logs/consumer.jsonl >/dev/null
grep -R "search_registries_result.*enterprise-a.registry.nanda.local" logs/consumer.jsonl >/dev/null
grep -R "search_registries_result.*enterprise-b.registry.nanda.local" logs/consumer.jsonl >/dev/null
grep -R "verified_registry_addr" logs/consumer.jsonl >/dev/null
grep -R "credential_rotated_registry_addr" logs >/dev/null
grep -R "credential_rotated_catalog" logs >/dev/null
grep -R "issuer_key_prepublished" logs/issuer-key-rotator.jsonl >/dev/null
grep -R "issuer_key_rotated" logs/issuer-key-rotator.jsonl >/dev/null
grep -R "old_issuer_key_retired" logs/issuer-key-rotator.jsonl >/dev/null
grep -R "trust_bundle_reloaded" logs/consumer.jsonl >/dev/null
grep -R "verified_with_issuer_key" logs/consumer.jsonl >/dev/null
grep -R "push_revocation" logs/revocation-authority.jsonl >/dev/null
grep -R "serve_status_list" logs/revocation-authority.jsonl >/dev/null
grep -R "verified_status_list" logs/consumer.jsonl >/dev/null
grep -R "verification_failed_revoked_status_list" logs/consumer.jsonl >/dev/null
grep -R "verification_recovered_revoked_status_list" logs/consumer.jsonl >/dev/null
grep -R "publish_crdt_update" logs/crdt-update-bus.jsonl >/dev/null
grep -R "serve_crdt_update" logs/crdt-update-bus.jsonl >/dev/null
grep -R "fetch_crdt_updates" logs/consumer.jsonl >/dev/null
grep -R "verified_crdt_updates" logs/consumer.jsonl >/dev/null
grep -R "merge_crdt_ops" logs/consumer.jsonl >/dev/null
grep -R "crdt_conflict_resolved" logs/consumer.jsonl >/dev/null
grep -R "crdt_state_applied" logs/consumer.jsonl >/dev/null
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

echo "ok: base quilt, federation, VC verification, revocation, CRDT, key rotation, and privacy checks passed"
echo "proving runtime agent registration with an existing enterprise registry"

docker compose run -d --no-deps \
  --name level8-e2e-existing-agent \
  -p 19190:8080 \
  --entrypoint /shared-bin/mcp-agent \
  enterprise-a-reverse \
  --logs /logs \
  --agent e2e-existing-agent \
  --tool echo \
  --addr :8080 >/dev/null

sleep 2
curl -fsS --resolve host.docker.internal:19190:127.0.0.1 \
  http://host.docker.internal:19190/healthz >/dev/null

curl -fsS --resolve enterprise-a.registry.nanda.local:18081:127.0.0.1 \
  -X POST http://enterprise-a.registry.nanda.local:18081/agents/register \
  -H 'Content-Type: application/json' \
  -d '{
    "id": "e2e-existing-agent",
    "name": "e2e.existing.agent.mcp.local",
    "endpoint": "http://host.docker.internal:19190",
    "tools": ["echo"]
  }' >/tmp/level8-e2e-existing-register.json

sleep 3
curl -fsS --resolve nanda-a.local:18080:127.0.0.1 \
  'http://nanda-a.local:18080/search?tool=echo' >/tmp/level8-e2e-existing-search.json
grep "enterprise-a.registry.nanda.local" /tmp/level8-e2e-existing-search.json >/dev/null
grep "e2e-existing-agent" /tmp/level8-e2e-existing-search.json >/dev/null

curl -fsS --resolve enterprise-a.registry.nanda.local:18081:127.0.0.1 \
  http://enterprise-a.registry.nanda.local:18081/catalog >/tmp/level8-e2e-existing-catalog.json
grep "EnterpriseMCPCatalogCredential" /tmp/level8-e2e-existing-catalog.json >/dev/null
grep "e2e-existing-agent" /tmp/level8-e2e-existing-catalog.json >/dev/null

curl -fsS --resolve host.docker.internal:19190:127.0.0.1 \
  -X POST http://host.docker.internal:19190/mcp/tools/call \
  -H 'Content-Type: application/json' \
  -d '{"tool":"echo","input":"existing registry dynamic agent"}' >/tmp/level8-e2e-existing-call.json
grep "existing registry dynamic agent" /tmp/level8-e2e-existing-call.json >/dev/null

grep -R "agent_registered.*e2e-existing-agent" logs/enterprise-a-registry.jsonl >/dev/null
grep -R "agent_registration_join_refreshed.*e2e-existing-agent" logs/enterprise-a-registry.jsonl >/dev/null

echo "ok: Enterprise A dynamically registered agent was found through NANDA and called"
echo "proving a dynamic registry can join NANDA, register an agent, and serve signed facts"

docker compose run -d --no-deps \
  --name level8-e2e-lab-agent \
  -p 19191:8080 \
  --entrypoint /shared-bin/mcp-agent \
  enterprise-a-reverse \
  --logs /logs \
  --agent e2e-lab-agent \
  --tool title \
  --addr :8080 >/dev/null

docker compose run -d --no-deps \
  --name level8-e2e-lab-registry \
  -p 18111:8080 \
  --entrypoint /shared-bin/enterprise-registry \
  enterprise-a-registry \
  --artifacts /artifacts \
  --logs /logs \
  --enterprise e2e-lab \
  --registry-name e2e-lab.registry.nanda.local \
  --registry-id e2e-lab-registry \
  --catalog-url http://host.docker.internal:18111/catalog \
  --facts-mode public \
  --description "Runtime e2e lab registry for ad-hoc agent registration." \
  --join-index http://nanda-index-a:8080 \
  --addr :8080 >/dev/null

sleep 4
curl -fsS --resolve host.docker.internal:19191:127.0.0.1 \
  http://host.docker.internal:19191/healthz >/dev/null
curl -fsS --resolve e2e-lab.registry.nanda.local:18111:127.0.0.1 \
  http://e2e-lab.registry.nanda.local:18111/healthz >/dev/null

curl -fsS --resolve e2e-lab.registry.nanda.local:18111:127.0.0.1 \
  -X POST http://e2e-lab.registry.nanda.local:18111/agents/register \
  -H 'Content-Type: application/json' \
  -d '{
    "id": "e2e-lab-agent",
    "name": "e2e.lab.agent.mcp.local",
    "endpoint": "http://host.docker.internal:19191",
    "tools": ["title"]
  }' >/tmp/level8-e2e-lab-register.json

sleep 3
curl -fsS --resolve nanda-a.local:18080:127.0.0.1 \
  'http://nanda-a.local:18080/search?tool=title' >/tmp/level8-e2e-lab-search.json
grep "e2e-lab.registry.nanda.local" /tmp/level8-e2e-lab-search.json >/dev/null
grep "e2e-lab-agent" /tmp/level8-e2e-lab-search.json >/dev/null

curl -fsS --resolve nanda-a.local:18080:127.0.0.1 \
  http://nanda-a.local:18080/resolve/e2e-lab.registry.nanda.local >/tmp/level8-e2e-lab-addr.json
grep "EnterpriseRegistryAddrCredential" /tmp/level8-e2e-lab-addr.json >/dev/null

curl -fsS --resolve e2e-lab.registry.nanda.local:18111:127.0.0.1 \
  http://e2e-lab.registry.nanda.local:18111/catalog >/tmp/level8-e2e-lab-catalog.json
grep "EnterpriseMCPCatalogCredential" /tmp/level8-e2e-lab-catalog.json >/dev/null
grep "e2e-lab-agent" /tmp/level8-e2e-lab-catalog.json >/dev/null

curl -fsS --resolve host.docker.internal:19191:127.0.0.1 \
  -X POST http://host.docker.internal:19191/mcp/tools/call \
  -H 'Content-Type: application/json' \
  -d '{"tool":"title","input":"dynamic registry e2e"}' >/tmp/level8-e2e-lab-call.json
grep "dynamic registry e2e" /tmp/level8-e2e-lab-call.json >/dev/null

grep -R "managed_registry_credentials_rotated" logs/e2e-lab-registry.jsonl >/dev/null
grep -R "agent_registered.*e2e-lab-agent" logs/e2e-lab-registry.jsonl >/dev/null
grep -R "agent_registration_join_refreshed.*e2e-lab-agent" logs/e2e-lab-registry.jsonl >/dev/null
grep -R "registry_joined_quilt.*e2e-lab.registry.nanda.local" logs/nanda-a.jsonl >/dev/null
grep -R "registry_refreshed_quilt.*e2e-lab.registry.nanda.local" logs/nanda-a.jsonl >/dev/null

echo "ok: dynamic registry was discovered through NANDA and its registered tool was called"

docker compose logs --tail=80 swimlane
