# Level 8 Runbook

Level 8 keeps the Level 7 security features and makes discovery more realistic:

- two NANDA indexes, `nanda-a` and `nanda-b`
- simple federation through signed peer records exposed at `/peers`
- enterprise registries join their home index through `/join`
- a registry can be created at runtime, sign its own registry address and catalog credentials, rotate those credentials, and join an index through the same `/join` path
- registries accept runtime agent registration through `/agents/register`, refresh their NANDA join, and make those agents searchable by tool or agent id
- the consumer starts with only `nanda-a`, discovers `nanda-b`, searches both, resolves AgentAddr-like records from the right index, verifies facts, then calls MCP tools

## Architecture

```text
consumer -> nanda-a -> /peers -> nanda-b
consumer -> nanda-a -> enterprise-a AgentAddr -> public facts -> MCP tools
consumer -> nanda-b -> enterprise-b AgentAddr -> PrivateFactsURL -> MCP tools
developer client -> enterprise registry -> POST /agents/register -> refreshed NANDA join
runtime registry -> POST /join -> nanda-a -> searchable registry and agent metadata
```

Enterprise registries join the quilt this way:

```text
enterprise-a-registry -> POST /join -> nanda-a
enterprise-b-registry -> POST /join -> nanda-b
runtime-registry -> POST /join -> selected NANDA index
```

The join payload is a registry record containing the current signed `EnterpriseRegistryAddrCredential` plus a compact list of advertised agents. The credential rotator keeps rewriting fresh signed records for static registries, and each enterprise registry periodically refreshes its join. Runtime registries manage their own short-lived registry address and catalog credentials with a 15 second rotation loop and 45 second TTLs.

## Code Layout

- `cmd/*`: one binary per modeled component.
- `internal/index`: NANDA index wrapper.
- `internal/registry`: enterprise registry and private facts gateway wrappers.
- `internal/agents`: MCP agent wrapper.
- `internal/consumer`: dynamic client/verifier wrapper.
- `internal/revocation`: revocation authority wrapper.
- `internal/crdt`: CRDT update bus wrapper.
- `internal/rotator`: credential and issuer key rotators.
- `internal/swimlane`: audit swimlane wrapper.
- `internal/shared`: shared AgentAddr-like, AgentFacts-like, dynamic registry, runtime agent registration, VC, status-list, trust-bundle, and helper logic.

## Verification Order

For registry address, catalog, status-list, and CRDT update credentials, the consumer verifies:

1. `proof.verificationMethod` exists in the current trust bundle
2. VC signature
3. VC expiration
4. status-list credential signature
5. status-list credential expiration
6. status bit at `statusListIndex`

Only after the base catalog and CRDT updates are accepted does the consumer compare live MCP tool lists and call tools.

## Run

```sh
./scripts/test-e2e.sh
```

The script asserts:

- both indexes accepted enterprise registry joins
- enterprise registries refreshed their joins after credential rotation
- the consumer discovered `nanda-b` from `nanda-a`
- the consumer searched both indexes
- issuer key prepublish, active promotion, trust reload, and old-key retirement happened
- explicit status-list revocation happened
- revoked credential verification failed and later recovered
- Enterprise A used the public catalog path
- Enterprise B used `privateFactsURL`
- Enterprise B direct catalog was not used by the consumer
- CRDT updates were published, served, verified, merged, and applied
- tools still executed after verified facts resolution
- an e2e-only agent registered with Enterprise A, appeared in NANDA tool search, appeared in the signed catalog, and was callable
- an e2e-only runtime registry joined NANDA, rotated signed registry/catalog credentials, registered an agent, appeared in NANDA tool search, and exposed a callable tool

## Manual Checks

With the stack running:

```sh
# NANDA A knows NANDA B as a federated peer.
curl --resolve nanda-a.local:18080:127.0.0.1 http://nanda-a.local:18080/peers

# NANDA B knows NANDA A as a federated peer.
curl --resolve nanda-b.local:18086:127.0.0.1 http://nanda-b.local:18086/peers

# NANDA A search returns Enterprise A after the registry joins.
curl --resolve nanda-a.local:18080:127.0.0.1 'http://nanda-a.local:18080/search?registrationType=enterprise-mcp-registry'

# NANDA B search returns Enterprise B after the registry joins.
curl --resolve nanda-b.local:18086:127.0.0.1 'http://nanda-b.local:18086/search?registrationType=enterprise-mcp-registry'

# NANDA A can filter joined registry metadata by advertised tool.
curl --resolve nanda-a.local:18080:127.0.0.1 'http://nanda-a.local:18080/search?tool=reverse'

# NANDA A can filter joined registry metadata by agent id.
curl --resolve nanda-a.local:18080:127.0.0.1 'http://nanda-a.local:18080/search?agent=enterprise-a-reverse'

# Enterprise A AgentAddr-like record from NANDA A.
curl --resolve nanda-a.local:18080:127.0.0.1 http://nanda-a.local:18080/resolve/enterprise-a.registry.nanda.local

# Enterprise B AgentAddr-like record from NANDA B.
curl --resolve nanda-b.local:18086:127.0.0.1 http://nanda-b.local:18086/resolve/enterprise-b.registry.nanda.local

# Enterprise A public facts.
curl --resolve enterprise-a.registry.nanda.local:18081:127.0.0.1 http://enterprise-a.registry.nanda.local:18081/catalog

# Enterprise A's static plus runtime-registered agents.
curl --resolve enterprise-a.registry.nanda.local:18081:127.0.0.1 http://enterprise-a.registry.nanda.local:18081/agents

# Enterprise B private facts.
curl --resolve private-facts.enterprise-b.registry.nanda.local:18083:127.0.0.1 http://private-facts.enterprise-b.registry.nanda.local:18083/private-facts/enterprise-b/catalog

# Signed W3C-style status list.
curl --resolve revocation.nanda.local:18084:127.0.0.1 http://revocation.nanda.local:18084/status-lists/level8-revocation
```

For the complete manual flow that starts host-side agents, registers them, creates a runtime registry, searches NANDA, and calls discovered tools, use `LEVEL8_WALKTHROUGH.md`. For a client-only discovery flow, use `CURL_CLIENT_GUIDE.md`.
