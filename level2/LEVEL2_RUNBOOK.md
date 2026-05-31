# Level 2 Runbook

This demo extends the Level 1 idea with a different registration type: **enterprise-routed MCP registry registration**.

## What Is Different From Level 1

Level 1 directly registers agents in the NANDA index. Level 2 registers enterprise MCP registry proxies in the NANDA index. Each enterprise registry is intentionally fake and small: it is just a simple catalog for that enterprise's MCP servers.

The visible flow is:

```text
consumer -> NANDA index -> enterprise registry -> MCP server
```

## What Runs

The demo creates a shared index plus three demo groups:

- `nanda-index`: the shared NANDA index.
- `enterprise-a`: one registry and two MCP servers.
- `enterprise-b`: one registry and two MCP servers.
- `consumer`: a client agent that uses both enterprises.

Enterprise A MCP servers:

- `enterprise-a-reverse`: supports `reverse`.
- `enterprise-a-uppercase`: supports `uppercase`.

Enterprise B MCP servers:

- `enterprise-b-truncate`: supports `truncate`.
- `enterprise-b-count`: supports `count`.

## How To Run

```sh
./scripts/test-e2e.sh
```

The script:

1. Builds the Go binary inside an Ubuntu container.
2. Generates Level 2 signed JSON artifacts.
3. Starts the NANDA index.
4. Starts both enterprise registries.
5. Starts four fake MCP servers.
6. Runs the consumer, which searches NANDA for enterprise MCP registries.
7. Runs the swimlane printer against shared audit logs.

## What Verification Means Here

NANDA signs `EnterpriseRegistryAddr` records. These tell the consumer which enterprise registry proxies exist and where their signed catalogs are.

Each enterprise registry serves a signed `EnterpriseMCPCatalog`. This tells the consumer which MCP servers exist, where they are, and which skills each one claims to support.

The consumer verifies both layers:

1. Verify the registry address from NANDA.
2. Use the verified `catalogURL`.
3. Fetch the enterprise catalog.
4. Verify the enterprise catalog.
5. Ask each MCP server for its live tool list.
6. Compare the live tool list to the verified catalog.
7. Use only the skills listed in the verified catalog.
8. Call the MCP server.

This demonstrates that NANDA helps the consumer find enterprise registry proxies, while each enterprise registry is responsible for cataloging its MCP servers.

## Skill Mismatch Check

After successful tool calls, the consumer intentionally asks one MCP server to run a skill that was not listed in the verified catalog for that server. The server rejects the call with a skill mismatch error, and the consumer logs that rejection.

This proves the client is not treating arbitrary tool calls as valid just because a server exists. It checks the verified catalog first.

## Audit Logs And Swimlane

Every major component writes simple JSONL audit events to the shared `logs/` directory:

```json
{"time":"...","actor":"consumer","peer":"nanda-index","action":"search_registries_result","result":"enterprise-a,enterprise-b"}
```

The `swimlane` container reads those files, sorts events by timestamp, and prints a text timeline to stdout. The output is intentionally simple; it is meant to make the agent discovery and call chain visible in Docker logs.

## Manual Inspection

Start the services:

```sh
mkdir -p bin artifacts logs
docker compose up --build nanda-index enterprise-a-registry enterprise-b-registry enterprise-a-reverse enterprise-a-uppercase enterprise-b-truncate enterprise-b-count
```

Inspect NANDA and registry responses:

```sh
# Lists enterprise registries known to NANDA.
curl http://localhost:18080/registries

# Searches NANDA for enterprise MCP registry registrations.
curl 'http://localhost:18080/search?registrationType=enterprise-mcp-registry'

# Returns the signed address for Enterprise A's MCP registry.
curl http://localhost:18080/resolve/enterprise-a.registry.nanda.local

# Returns Enterprise A's signed MCP catalog.
curl http://localhost:18081/catalog
```

Stop services:

```sh
docker compose down --remove-orphans
```
