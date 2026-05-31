# cURL Dynamic Client Guide

This guide fakes a dynamic client agent with plain `curl`. Start the stack first:

```sh
docker compose up --build
```

## Discover Indexes

```sh
# Ask the only bootstrap index you know about for federated peers.
curl --resolve nanda-a.local:18080:127.0.0.1 http://nanda-a.local:18080/peers

# Ask the discovered peer for its peers too.
curl --resolve nanda-b.local:18086:127.0.0.1 http://nanda-b.local:18086/peers
```

## Search The Quilt

```sh
# Search NANDA A. Enterprise A should appear after its registry joins.
curl --resolve nanda-a.local:18080:127.0.0.1 'http://nanda-a.local:18080/search?registrationType=enterprise-mcp-registry'

# Search NANDA B. Enterprise B should appear after its registry joins.
curl --resolve nanda-b.local:18086:127.0.0.1 'http://nanda-b.local:18086/search?registrationType=enterprise-mcp-registry'
```

## Resolve AgentAddr-Like Records

```sh
# Resolve Enterprise A from the index that advertised it.
curl --resolve nanda-a.local:18080:127.0.0.1 http://nanda-a.local:18080/resolve/enterprise-a.registry.nanda.local

# Resolve Enterprise B from the index that advertised it.
curl --resolve nanda-b.local:18086:127.0.0.1 http://nanda-b.local:18086/resolve/enterprise-b.registry.nanda.local
```

## Fetch Facts

```sh
# Fetch Enterprise A public facts from its source registry.
curl --resolve enterprise-a.registry.nanda.local:18081:127.0.0.1 http://enterprise-a.registry.nanda.local:18081/catalog

# Fetch Enterprise B private facts from the neutral private facts gateway.
curl --resolve private-facts.enterprise-b.registry.nanda.local:18083:127.0.0.1 http://private-facts.enterprise-b.registry.nanda.local:18083/private-facts/enterprise-b/catalog
```

## Inspect Revocation And Updates

```sh
# Fetch the signed revocation status list.
curl --resolve revocation.nanda.local:18084:127.0.0.1 http://revocation.nanda.local:18084/status-lists/level8-revocation

# Fetch signed dynamic AgentFacts updates for Enterprise A.
curl --resolve crdt.nanda.local:18085:127.0.0.1 http://crdt.nanda.local:18085/crdt/enterprise-a/updates

# Fetch signed dynamic AgentFacts updates for Enterprise B.
curl --resolve crdt.nanda.local:18085:127.0.0.1 http://crdt.nanda.local:18085/crdt/enterprise-b/updates
```

## Call Tools After Verification

These calls skip verification because `curl` is only faking the client loop. The real `consumer` service verifies first, then calls the same MCP endpoints.

```sh
# List Enterprise A reverse tool.
curl --resolve enterprise-a.reverse.mcp.local:18087:127.0.0.1 http://enterprise-a.reverse.mcp.local:18087/mcp/tools/list
```

The compose file does not publish every MCP tool to the host by default. For host-side manual tool calls, add a temporary port mapping to the specific MCP service you want to inspect.
