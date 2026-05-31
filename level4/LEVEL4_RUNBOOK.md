# Level 4 Runbook

Level 4 adds a local privacy-preserving facts path to the Level 3 freshness demo.

## Paper Mapping

`2507.14263v1.pdf` discusses `PrivateFactsURL` as a way to decouple facts lookup from the source agent or organization. The goal is not that facts are unsigned or hidden from verification. The goal is that clients can retrieve signed metadata through a neutral path, so the source does not learn every resolver that asks for facts.

In this demo:

- `EnterpriseRegistryAddrCredential` is the AgentAddr-like record returned by NANDA.
- `EnterpriseMCPCatalogCredential` is the AgentFacts-like signed metadata.
- `catalogURL` is the direct source path.
- `privateFactsURL` is the neutral-hosted facts path.

## Public And Private Enterprises

Enterprise A is public:

```text
consumer -> nanda-index -> enterprise-a-registry -> signed catalog
```

Enterprise B is private:

```text
consumer -> nanda-index -> private-facts-gateway -> signed catalog
```

The Enterprise B `EnterpriseRegistryAddrCredential` still includes the original direct `catalogURL`, but it also includes `privateFactsURL` and `factsMode=private`. The consumer logs that the direct catalog URL was not used and fetches through the gateway instead.

## Why The Gateway Is Not Trusted

The `private-facts-gateway` is a neutral host for signed JSON. It does not become a trust anchor. The consumer still verifies:

1. the NANDA registry address signature
2. the registry address expiration
3. the enterprise catalog signature
4. the enterprise catalog expiration
5. the MCP server live tool list against the verified catalog

If the gateway serves an expired or tampered catalog, verification fails before any tool call is made.

## Network Shape

The consumer is connected to:

- `nanda_net`
- `privacy_net`
- `enterprise_a_net`
- `enterprise_b_net`

Enterprise A registry is reachable by the consumer for direct public facts.

Enterprise B registry is not on the consumer's networks. It is still published on `localhost:18082` for manual inspection, but the in-compose consumer cannot resolve or reach `enterprise-b-registry`. The consumer reaches Enterprise B facts through `private-facts-gateway` on `privacy_net`.

## Rotation Behavior

Level 4 keeps Level 3 credential freshness behavior:

- registry-address and catalog credentials have random 5-10 second TTLs
- the two layers are staggered so both failure types appear
- rotation waits an additional random 2-4 seconds after the first layer expires
- the consumer logs verification failures and recoveries

## Run

```sh
./scripts/test-e2e.sh
```

The script asserts:

- credential rotation happened
- registry-address verification failed and recovered
- catalog verification failed and recovered
- Enterprise A used the public catalog path
- Enterprise B used `privateFactsURL`
- Enterprise B direct catalog was not used by the consumer
- tools still executed after verified facts resolution

## Logs To Look For

Important events:

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

The swimlane marks credential rotations and verification failures with `!!!`.

## Manual Curl Checks

With the stack running:

```sh
# Lists enterprise registries known to NANDA.
curl http://localhost:18080/registries

# Returns Enterprise A's direct public catalog URL.
curl http://localhost:18080/resolve/enterprise-a.registry.nanda.local

# Returns Enterprise B's direct catalog URL and PrivateFactsURL.
curl http://localhost:18080/resolve/enterprise-b.registry.nanda.local

# Public facts path for Enterprise A.
curl http://localhost:18081/catalog

# Source path for Enterprise B, available to host for inspection only.
curl http://localhost:18082/catalog

# Private facts path used by the consumer for Enterprise B.
curl http://localhost:18083/private-facts/enterprise-b/catalog
```
