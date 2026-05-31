# Level 6 Runbook

Level 6 keeps the Level 5 demo and adds a local CRDT-based AgentFacts update protocol.

## Paper Mapping

`2507.14263v1.pdf` calls out a lean index, signed AgentFacts, private facts resolution, fast revocation, and CRDT-based updates. Level 6 maps those pieces this way:

- `EnterpriseRegistryAddrCredential` is the AgentAddr-like record returned by NANDA.
- `EnterpriseMCPCatalogCredential` is the base AgentFacts-like signed metadata.
- `catalogURL` is the direct source path.
- `privateFactsURL` is the neutral-hosted facts path.
- `credentialStatus` points each VC at a signed local status list.
- `StatusList2021Credential` is the W3C-style revocation list served by `revocation-authority`.
- `crdtUpdateURL` points from the signed catalog to signed dynamic AgentFacts updates.
- `AgentFactsCRDTUpdateCredential` is the signed CRDT update document served by `crdt-update-bus`.

This is a local demo implementation. The important behavior is separation: the index remains stable while AgentFacts updates can change through a signed update stream.

## Public, Private, And CRDT Paths

Enterprise A is public:

```text
consumer -> nanda-index -> enterprise-a-registry -> signed catalog
```

Enterprise B is private:

```text
consumer -> nanda-index -> private-facts-gateway -> signed catalog
```

Both verified catalogs contain `crdtUpdateURL`:

```text
consumer -> crdt-update-bus -> signed AgentFactsCRDTUpdateCredential
```

The consumer verifies the base catalog first, then fetches and verifies the CRDT update credential, then deterministically merges the CRDT operations.

## CRDT Update Protocol

The demo uses a small local CRDT shape:

- `lww-register` for a `routingProfile` value
- `or-set-add` for `telemetryEndpoints`
- `or-set-add` for `capabilityTags`

The `crdt-update-bus` publishes two replica streams for each enterprise. Both replicas write `routingProfile` values in the same update epoch. The consumer resolves that conflict deterministically with last-writer-wins ordering by logical time and replica ID.

Events to look for:

- `publish_crdt_update`
- `serve_crdt_update`
- `fetch_crdt_updates`
- `verified_crdt_updates`
- `merge_crdt_ops`
- `crdt_conflict_resolved`
- `crdt_state_applied`

These logs show the update protocol is separate from NANDA index writes. NANDA still only resolves the enterprise registry/catalog path.

## Verification Order

For registry address and catalog credentials, the consumer verifies:

1. VC signature
2. VC expiration
3. status-list credential signature
4. status-list credential expiration
5. status bit at `statusListIndex`

For CRDT updates, the consumer verifies the same signature, expiration, and status-list checks on `AgentFactsCRDTUpdateCredential` before merging operations.

Only after the base catalog and CRDT updates are accepted does the consumer compare live MCP tool lists and call tools.

## Revocation Behavior

Level 6 keeps Level 5 revocation behavior:

- active Enterprise B catalog credentials are revoked by status-list update
- revoked credentials fail even if their `expirationDate` is still in the future
- the next rotated credential recovers because it uses a fresh `statusListIndex`

The CRDT update credentials are also status-list checked, but the demo does not intentionally revoke them.

## Network Shape

The consumer is connected to:

- `nanda_net`
- `privacy_net`
- `revocation_net`
- `crdt_net`
- `enterprise_a_net`
- `enterprise_b_net`

Enterprise B registry is not on the consumer's networks. It is still published on `localhost:18082` for manual inspection, but the in-compose consumer cannot resolve or reach `enterprise-b-registry`. The consumer reaches Enterprise B facts through `private-facts-gateway` and reaches dynamic updates through `crdt-update-bus`.

## Run

```sh
./scripts/test-e2e.sh
```

The script asserts:

- credential rotation happened
- explicit status-list revocation happened
- revoked credential verification failed and later recovered
- registry-address verification failed and recovered through TTL behavior
- catalog verification failed and recovered
- Enterprise A used the public catalog path
- Enterprise B used `privateFactsURL`
- Enterprise B direct catalog was not used by the consumer
- CRDT updates were published, served, verified, merged, and applied
- a CRDT conflict was resolved deterministically
- tools still executed after verified facts resolution

## Manual Curl Checks

With the stack running:

```sh
# Lists enterprise registries known to NANDA.
curl http://localhost:18080/registries

# Returns Enterprise A's direct public catalog URL and credentialStatus.
curl http://localhost:18080/resolve/enterprise-a.registry.nanda.local

# Returns Enterprise B's direct catalog URL, PrivateFactsURL, and credentialStatus.
curl http://localhost:18080/resolve/enterprise-b.registry.nanda.local

# Public facts path for Enterprise A, including crdtUpdateURL.
curl http://localhost:18081/catalog

# Source path for Enterprise B, available to host for inspection only.
curl http://localhost:18082/catalog

# Private facts path used by the consumer for Enterprise B.
curl http://localhost:18083/private-facts/enterprise-b/catalog

# Signed status list used by the consumer for revocation checks.
curl http://localhost:18084/status-lists/level6-revocation

# Signed CRDT update stream for Enterprise A.
curl http://localhost:18085/crdt/enterprise-a/updates

# Signed CRDT update stream for Enterprise B.
curl http://localhost:18085/crdt/enterprise-b/updates
```
