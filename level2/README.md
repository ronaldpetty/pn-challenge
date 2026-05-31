# NANDA Level 2 Enterprise Registry Demo

This Level 2 demo shows a second registration type: **enterprise-routed registration**.

Instead of registering every MCP server directly in NANDA, the NANDA index registers two enterprise MCP registry proxies. A consumer searches NANDA for those registries, verifies the signed registry addresses, asks each registry for its signed MCP catalog, verifies the catalog, compares each MCP server's live tool list to the signed catalog, executes those skills, and proves a skill mismatch is rejected.

## Run

```sh
./scripts/test-e2e.sh
```

The script builds the Go binary inside an Ubuntu-based Docker container, starts the full Docker Compose stack, runs the consumer task, then prints a text swimlane from shared audit logs.

## Services

- `nanda-index`: signed registry address lookup.
- `enterprise-a-registry`: fake enterprise A MCP registry.
- `enterprise-b-registry`: fake enterprise B MCP registry.
- `enterprise-a-reverse`: MCP server with `reverse`.
- `enterprise-a-uppercase`: MCP server with `uppercase`.
- `enterprise-b-truncate`: MCP server with `truncate`.
- `enterprise-b-count`: MCP server with `count`.
- `consumer`: searches NANDA for registries and executes tools.
- `swimlane`: reads shared logs and prints the interaction diagram.

Generated runtime files are written to ignored directories:

- `bin/`
- `artifacts/`
- `logs/`

## Manual Inspection

Start the long-running services:

```sh
mkdir -p bin artifacts logs
docker compose up --build nanda-index enterprise-a-registry enterprise-b-registry enterprise-a-reverse enterprise-a-uppercase enterprise-b-truncate enterprise-b-count
```

In another terminal:

```sh
# Lists enterprise registry names known to NANDA.
curl http://localhost:18080/registries

# Searches NANDA for enterprise MCP registry registrations.
curl 'http://localhost:18080/search?registrationType=enterprise-mcp-registry'

# Returns the signed address for Enterprise A's registry proxy.
curl http://localhost:18080/resolve/enterprise-a.registry.nanda.local

# Returns Enterprise A's signed MCP server catalog.
curl http://localhost:18081/catalog
```

Stop services:

```sh
docker compose down --remove-orphans
```
