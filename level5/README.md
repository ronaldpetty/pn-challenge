# NANDA Level 5 Revocation Demo

Level 5 keeps Level 4's public/private facts paths and adds explicit W3C-style VC status-list revocation.

- Enterprise A is public. The consumer fetches its signed catalog directly from `enterprise-a-registry`.
- Enterprise B is private. The consumer receives both a direct `catalogURL` and a neutral `privateFactsURL`, then intentionally uses `privateFactsURL`.
- Every registry address and enterprise catalog VC includes `credentialStatus`.
- The local `revocation-authority` serves a signed `StatusList2021Credential` and pushes revocation updates for active credentials.
- The consumer rejects a revoked credential even when its TTL has not expired.

The private path is:

```text
consumer -> nanda-index -> private-facts-gateway -> signed Enterprise B catalog
```

The revocation path is:

```text
consumer -> revocation-authority -> signed status list
```

Neither gateway is trusted for truth. The consumer verifies VC signatures, expirations, and status-list bits before using a catalog or calling an MCP tool.

## Paper Mapping

`2507.14263v1.pdf` discusses short-lived agent identity, verifiable facts, `PrivateFactsURL`, and revocation-style checks as separate concerns. Level 5 keeps these separate:

- `EnterpriseRegistryAddrCredential` is the AgentAddr-like record returned by NANDA.
- `EnterpriseMCPCatalogCredential` is the AgentFacts-like signed metadata.
- `catalogURL` is the direct source path.
- `privateFactsURL` is the neutral-hosted facts path.
- `credentialStatus` points each VC at a signed local status list.
- `StatusList2021Credential` is the W3C-style revocation list served by `revocation-authority`.

This is a local demo implementation, not a production revocation service. The important behavior is that revocation is distinct from TTL expiry.

## Services

- `nanda-index`: serves current signed registry address credentials.
- `credential-rotator`: rewrites signed JSON credentials with random short TTLs.
- `revocation-authority`: serves a signed status list and revokes active Enterprise B catalog credentials before expiry.
- `enterprise-a-registry`: public direct catalog source.
- `enterprise-b-registry`: private source registry; kept off the consumer's networks.
- `private-facts-gateway`: neutral host for Enterprise B's signed catalog.
- `enterprise-a-reverse`: MCP server with `reverse`.
- `enterprise-a-uppercase`: MCP server with `uppercase`.
- `enterprise-b-truncate`: MCP server with `truncate`.
- `enterprise-b-count`: MCP server with `count`.
- `consumer`: loops forever, resolves both enterprises, verifies revocation status, and calls tools only after verification.
- `swimlane`: loops forever and highlights rotations, revocations, and failed verification with `!!!`.

Generated runtime files are ignored:

- `bin/`
- `artifacts/`
- `logs/`

## Run A Bounded Test

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
- tools still executed after verified facts resolution

## Run The Live Demo

```sh
mkdir -p bin artifacts logs
docker compose up --build
```

Stop it:

```sh
docker compose down --remove-orphans
```

## Public And Private Enterprises

Enterprise A is public:

```text
consumer -> nanda-index -> enterprise-a-registry -> signed catalog
```

Enterprise B is private:

```text
consumer -> nanda-index -> private-facts-gateway -> signed catalog
```

The Enterprise B registry address still includes the original direct `catalogURL`, but it also includes `privateFactsURL` and `factsMode=private`. The consumer logs that the direct catalog URL was not used and fetches through the gateway instead.

## Revocation Flow

Every signed registry address and catalog VC includes:

- `credentialStatus.type=StatusList2021Entry`
- `credentialStatus.statusPurpose=revocation`
- `credentialStatus.statusListIndex`
- `credentialStatus.statusListCredential=http://revocation-authority:8080/status-lists/level5-revocation`

The consumer verifies in this order:

1. VC signature
2. VC expiration
3. status-list credential signature
4. status-list credential expiration
5. status bit at `statusListIndex`
6. MCP live tool list against the verified catalog

The `revocation-authority` rewrites the signed status list and marks the active Enterprise B catalog credential revoked while it is still inside its TTL. The consumer logs `verification_failed_revoked_status_list` and rejects that catalog. When `credential-rotator` issues the next generation with a fresh status index, verification recovers and the consumer logs `verification_recovered_revoked_status_list`.

## Why The Gateways Are Not Trusted

The `private-facts-gateway` only serves signed JSON. The `revocation-authority` only serves a signed status list. Neither becomes a blind trust anchor.

If either service serves tampered, expired, or revoked data, the consumer rejects the VC before making a tool call.

## Network Shape

The consumer is connected to:

- `nanda_net`
- `privacy_net`
- `revocation_net`
- `enterprise_a_net`
- `enterprise_b_net`

Enterprise A registry is reachable by the consumer for direct public facts.

Enterprise B registry is not on the consumer's networks. It is still published on `localhost:18082` for manual inspection, but the in-compose consumer cannot resolve or reach `enterprise-b-registry`. The consumer reaches Enterprise B facts through `private-facts-gateway` on `privacy_net`.

The status list is served on `revocation_net`, so the consumer can verify revocation status without contacting either enterprise registry for that check.

## Rotation Behavior

Level 5 keeps Level 3 credential freshness behavior:

- registry-address and catalog credentials have random 5-10 second TTLs
- the two layers are staggered so both failure types appear
- rotation waits an additional random 2-4 seconds after the first layer expires
- the consumer logs verification failures and recoveries

Level 5 adds revocation before TTL expiry:

- active Enterprise B catalog credentials are revoked by status-list update
- revoked credentials fail even if their `expirationDate` is still in the future
- the next rotated credential recovers because it uses a fresh `statusListIndex`

## Logs To Look For

Important events:

- `push_revocation`
- `fetch_status_list`
- `serve_status_list`
- `verified_status_list`
- `verification_failed_revoked_status_list`
- `verification_recovered_revoked_status_list`
- `selected_public_catalog_url`
- `selected_private_facts_url`
- `direct_catalog_url_not_used`
- `serve_private_facts`
- `serve_signed_catalog` from `enterprise-a-registry`
- no `serve_signed_catalog` from `enterprise-b-registry` during the e2e consumer run
- `verification_failed_registry_addr`
- `verification_failed_enterprise_catalog`
- `verification_recovered_registry_addr`
- `verification_recovered_enterprise_catalog`

The swimlane marks credential rotations, revocations, and verification failures with `!!!`.

## Manual Curl Checks

With the stack running:

```sh
# Lists enterprise registries known to NANDA.
curl http://localhost:18080/registries

# Returns Enterprise A's direct public catalog URL and credentialStatus.
curl http://localhost:18080/resolve/enterprise-a.registry.nanda.local

# Returns Enterprise B's direct catalog URL, PrivateFactsURL, and credentialStatus.
curl http://localhost:18080/resolve/enterprise-b.registry.nanda.local

# Public facts path for Enterprise A.
curl http://localhost:18081/catalog

# Source path for Enterprise B, available to host for inspection only.
curl http://localhost:18082/catalog

# Private facts path used by the consumer for Enterprise B.
curl http://localhost:18083/private-facts/enterprise-b/catalog

# Signed status list used by the consumer for revocation checks.
curl http://localhost:18084/status-lists/level5-revocation
```
