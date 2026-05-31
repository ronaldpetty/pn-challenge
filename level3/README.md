# NANDA Level 3 Credential Freshness Demo

This demo leaves `level1/` and `level2/` unchanged. It starts from the Level 2 enterprise registry model and adds short-lived signed JSON credentials.

The rotated credentials are:

- `EnterpriseRegistryAddrCredential`: the NANDA index record that maps an enterprise registry name to its catalog URL.
- `EnterpriseMCPCatalogCredential`: the enterprise registry catalog that lists MCP servers, endpoints, and tools.

Both expire after random 5-10 second TTLs. The rotator deliberately waits an additional random 2-4 seconds after the first layer expires before writing the next version, so the always-running consumer sees successful verification, failed catalog verification, failed registry-address verification, and recovery.

## Run A Bounded Test

```sh
./scripts/test-e2e.sh
```

The test starts the stack, lets it run long enough to observe credential rotation, expiration failures, verification recovery, and tool calls, then stops Docker Compose.

## Run The Live Demo

```sh
mkdir -p bin artifacts logs
docker compose up --build
```

This keeps the services alive until you stop Docker Compose.

Stop it:

```sh
docker compose down --remove-orphans
```

## Services

- `nanda-index`: serves current signed registry address credentials.
- `credential-rotator`: rewrites signed JSON credentials with random short TTLs.
- `enterprise-a-registry`: serves Enterprise A's current signed MCP catalog.
- `enterprise-b-registry`: serves Enterprise B's current signed MCP catalog.
- `enterprise-a-reverse`: MCP server with `reverse`.
- `enterprise-a-uppercase`: MCP server with `uppercase`.
- `enterprise-b-truncate`: MCP server with `truncate`.
- `enterprise-b-count`: MCP server with `count`.
- `consumer`: loops forever, verifies credentials, calls tools only after verification, and logs failures.
- `swimlane`: loops forever and prints new audit events, highlighting rotations and failed verification with `!!!`.

## Manual Inspection

```sh
# Lists enterprise registry names known to NANDA.
curl http://localhost:18080/registries

# Searches NANDA for enterprise MCP registry registrations.
curl 'http://localhost:18080/search?registrationType=enterprise-mcp-registry'

# Returns the current signed address for Enterprise A's registry proxy.
curl http://localhost:18080/resolve/enterprise-a.registry.nanda.local

# Returns Enterprise A's current signed MCP server catalog.
curl http://localhost:18081/catalog
```

Generated runtime files are ignored:

- `bin/`
- `artifacts/`
- `logs/`
