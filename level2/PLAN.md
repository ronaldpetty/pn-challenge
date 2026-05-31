# Level 2 Plan

Level 2 demonstrates a different registration model from Level 1.

Instead of directly registering every agent in NANDA, the NANDA index registers enterprise MCP registry proxies. A consumer searches the NANDA index for enterprise MCP registries, resolves each registry, verifies the signed registry address, fetches that registry's signed MCP catalog, verifies the catalog, then calls the MCP servers listed in it.

## Shape

- One shared `nanda-index`.
- Two fake enterprises: `enterprise-a` and `enterprise-b`.
- One registry proxy per enterprise.
- Two fake MCP servers per enterprise.
- One consumer that uses both enterprises.
- One swimlane container that reads shared audit logs and prints the timeline.

## Flow

```text
consumer -> nanda-index search -> enterprise registry address
consumer -> enterprise registry -> signed MCP catalog
consumer -> MCP server -> tool list
consumer -> MCP server -> tool call
```

## Verification

- NANDA signs `EnterpriseRegistryAddrCredential` records.
- Each enterprise signs an `EnterpriseMCPCatalogCredential`.
- The consumer verifies both signed JSON credentials before using their contents.
- The consumer compares each MCP server's live `tools/list` response to the signed catalog.
- The consumer intentionally calls a wrong tool on one MCP server and expects a skill mismatch rejection.

## Audit

Every component writes simple JSONL events with:

- `time`
- `actor`
- `peer`
- `action`
- `result`

The event schema is intentionally small. It is enough to show that the client searched NANDA, resolved enterprise registries, fetched catalogs, confirmed skills, called tools, and saw the invalid skill rejected.
