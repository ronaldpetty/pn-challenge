# NANDA Level 3 Credential Freshness Demo

Level 3 keeps the Level 2 enterprise registry flow, but makes signed identity metadata expire and rotate while the services stay alive.

The rotated credentials are:

- `EnterpriseRegistryAddrCredential`: the NANDA index record that maps an enterprise registry name to its catalog URL.
- `EnterpriseMCPCatalogCredential`: the enterprise registry catalog that lists MCP servers, endpoints, and tools.

Both expire after random 5-10 second TTLs. The rotator deliberately waits an additional random 2-4 seconds after the first layer expires before writing the next version, so the always-running consumer sees successful verification, failed catalog verification, failed registry-address verification, and recovery.

## Paper Mapping

The closest concern in `2507.14263v1.pdf` is signed metadata freshness around `AgentAddr` and `AgentFacts`.

- The paper's `AgentAddr` maps to this demo's `EnterpriseRegistryAddrCredential`.
- The paper's `AgentFacts` maps more closely to this demo's `EnterpriseMCPCatalogCredential`.

So "identity expires" here means the VC-shaped signed JSON expires. It does not mean the Docker container stops, the HTTP service identity changes, or a TLS certificate rotates.

## What Runs

Long-running services:

- `nanda-index`
- `credential-rotator`
- `enterprise-a-registry`
- `enterprise-b-registry`
- `enterprise-a-reverse`
- `enterprise-a-uppercase`
- `enterprise-b-truncate`
- `enterprise-b-count`
- `consumer`
- `swimlane`

The `go-build` and `artifact-init` services are setup jobs and exit after they finish.

Generated runtime files are ignored:

- `bin/`
- `artifacts/`
- `logs/`

## Run A Bounded Test

```sh
./scripts/test-e2e.sh
```

The script starts the stack, lets it run long enough to observe credential rotation, expiration failures, verification recovery, and tool calls, then stops Docker Compose.

## Run The Live Demo

```sh
mkdir -p bin artifacts logs
docker compose up --build
```

This keeps the services alive until you stop Docker Compose:

```sh
docker compose down --remove-orphans
```

## Rotation Behavior

The rotator writes fresh credentials with random TTLs between 5 and 10 seconds. It staggers the two layers so one generation shows catalog expiration first and the next generation shows registry-address expiration first. After the first layer expires, the rotator intentionally waits another random 2 to 4 seconds before writing the next credential version.

That deliberate expired gap makes failed verification visible:

```text
fresh credential -> verification succeeds
catalog or registry-address credential expires -> verification fails
rotator writes next version -> verification recovers
```

## Consumer Behavior

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

Important events:

- `credential_rotated_registry_addr`
- `credential_rotated_catalog`
- `verification_failed_registry_addr`
- `verification_failed_enterprise_catalog`
- `verification_recovered_registry_addr`
- `verification_recovered_enterprise_catalog`
- `tool_result`

The `swimlane` service continuously prints new events and marks credential rotations and failed verifications with `!!!`.

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
