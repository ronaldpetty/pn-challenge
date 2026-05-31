# NANDA Level 5 Revocation Demo

This demo leaves `level1/`, `level2/`, `level3/`, and `level4/` unchanged. It keeps Level 4's mixed facts model and adds explicit W3C-style VC status-list revocation.

- Enterprise A is public. The consumer fetches its signed catalog directly from `enterprise-a-registry`.
- Enterprise B is private. The consumer receives both a direct `catalogURL` and a neutral `privateFactsURL`, then intentionally uses `privateFactsURL`.
- Every registry address and enterprise catalog VC includes `credentialStatus`.
- The local `revocation-authority` serves a signed `StatusList2021Credential` and pushes revocation updates for active credentials.
- The consumer rejects a revoked credential even when its TTL has not expired.

The private path is still:

```text
consumer -> nanda-index -> private-facts-gateway -> signed Enterprise B catalog
```

The revocation path is:

```text
consumer -> revocation-authority -> signed status list
```

Neither gateway is trusted for truth. The consumer verifies VC signatures, expirations, and status-list bits before using a catalog or calling an MCP tool.

## Run A Bounded Test

```sh
./scripts/test-e2e.sh
```

The test starts the stack, observes credential rotation, verifies public and private facts paths, observes an explicit status-list revocation, confirms recovery after rotation, checks that Enterprise B's direct catalog endpoint was not used by the consumer, then stops Docker Compose.

## Run The Live Demo

```sh
mkdir -p bin artifacts logs
docker compose up --build
```

Stop it:

```sh
docker compose down --remove-orphans
```

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

## Manual Inspection

```sh
# Lists enterprise registry names known to NANDA.
curl http://localhost:18080/registries

# Returns Enterprise A's public registry address with credentialStatus.
curl http://localhost:18080/resolve/enterprise-a.registry.nanda.local

# Returns Enterprise B's private registry address with credentialStatus and privateFactsURL.
curl http://localhost:18080/resolve/enterprise-b.registry.nanda.local

# Direct public catalog for Enterprise A.
curl http://localhost:18081/catalog

# Direct source catalog for Enterprise B, exposed to host only for inspection.
curl http://localhost:18082/catalog

# Neutral private facts path used by the consumer for Enterprise B.
curl http://localhost:18083/private-facts/enterprise-b/catalog

# Signed W3C-style status list used for revocation checks.
curl http://localhost:18084/status-lists/level5-revocation
```

Generated runtime files are ignored:

- `bin/`
- `artifacts/`
- `logs/`
