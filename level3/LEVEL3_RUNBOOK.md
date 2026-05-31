# Level 3 Runbook

Level 3 is a credential freshness demo. It keeps the Level 2 enterprise registry flow, but makes the signed identity metadata expire and rotate while the services stay alive.

## Paper Mapping

In `2507.14263v1.pdf`, the closest concern is signed metadata freshness around `AgentAddr` and `AgentFacts`. The paper's `AgentAddr` maps to this demo's `EnterpriseRegistryAddrCredential`: it tells the client where the registry is. The paper's `AgentFacts` maps more closely to this demo's `EnterpriseMCPCatalogCredential`: it describes capabilities, endpoints, and tool metadata.

So "identity expires" here means the VC-shaped signed JSON expires. It does not mean the Docker container stops, the HTTP service identity changes, or a TLS certificate rotates.

Level 3 rotates both signed JSON layers:

- NANDA registry address credentials, matching the AgentAddr freshness path.
- Enterprise MCP catalog credentials, matching the AgentFacts metadata freshness path more directly.

## What Runs Forever

Run:

```sh
docker compose up --build
```

These long-running services stay alive until `docker compose down`:

- `nanda-index`
- `credential-rotator`
- `enterprise-a-registry`
- `enterprise-b-registry`
- four MCP servers
- `consumer`
- `swimlane`

The `go-build` and `artifact-init` services are setup jobs and exit after they finish.

## Rotation Behavior

The rotator writes fresh credentials with random TTLs between 5 and 10 seconds. It staggers the two layers so one generation shows catalog expiration first and the next generation shows registry-address expiration first. After the first layer expires, the rotator intentionally waits another random 2 to 4 seconds before writing the next credential version.

That deliberate expired gap makes failed verification visible:

```text
fresh credential -> verification succeeds
catalog or registry-address credential expires -> verification fails
rotator writes next version -> verification recovers
```

## What The Consumer Does

The consumer loops every 2 seconds:

1. Searches NANDA for enterprise MCP registry registrations.
2. Resolves each registry address credential.
3. Verifies the registry address signature and expiration.
4. Fetches each enterprise catalog.
5. Verifies the catalog signature and expiration.
6. Compares each MCP server's live tool list to the verified catalog.
7. Calls tools only when verification succeeded.
8. Skips tool calls when verification fails.

Verification failures are logged on every failed attempt. Recovery is logged when a previously failing credential verifies again.

## Logs To Look For

The audit logs are JSONL files in `logs/`.

Important events:

- `credential_rotated_registry_addr`
- `credential_rotated_catalog`
- `verification_failed_registry_addr`
- `verification_failed_enterprise_catalog`
- `verification_recovered_registry_addr`
- `verification_recovered_enterprise_catalog`
- `tool_result`

The `swimlane` service continuously prints new events and marks credential rotations and failed verifications with `!!!`.

## Bounded Test Script

Run:

```sh
./scripts/test-e2e.sh
```

The script starts the live stack, waits long enough to observe rotation and failure windows, asserts that the expected log events exist, prints the last swimlane lines, and stops Docker Compose.

## Manual Curl Checks

With the stack running:

```sh
# Lists enterprise registries known to NANDA.
curl http://localhost:18080/registries

# Searches for enterprise MCP registry registrations.
curl 'http://localhost:18080/search?registrationType=enterprise-mcp-registry'

# Returns the current signed registry address credential.
curl http://localhost:18080/resolve/enterprise-a.registry.nanda.local

# Returns the current signed enterprise catalog credential.
curl http://localhost:18081/catalog
```

If you repeat the last two curl commands over time, the `expirationDate` and `credentialVersion` fields change as the rotator writes new signed JSON.
