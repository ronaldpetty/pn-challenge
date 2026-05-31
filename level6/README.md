# NANDA Level 6 CRDT Update Demo

This demo leaves `level1/` through `level5/` unchanged. It keeps Level 5's privacy, credential freshness, and explicit revocation behavior, then adds a local CRDT-based AgentFacts update protocol.

- Enterprise A is public. The consumer fetches its signed catalog directly from `enterprise-a-registry`.
- Enterprise B is private. The consumer uses `privateFactsURL` through `private-facts-gateway`.
- Every registry address, catalog, CRDT update, and status-list document is signed as a W3C-style VC.
- The local `revocation-authority` serves a signed status list and revokes active Enterprise B catalog credentials before expiry.
- The local `crdt-update-bus` serves signed CRDT update credentials outside the NANDA index path.
- The consumer verifies and merges CRDT operations after the base catalog is verified.

The CRDT update path is:

```text
consumer -> verified catalog -> crdt-update-bus -> signed CRDT update VC -> deterministic merge
```

The index does not change when CRDT updates are published. The catalog contains a stable `crdtUpdateURL`, and the update bus publishes new signed operation sets that the consumer can merge.

## Run A Bounded Test

```sh
./scripts/test-e2e.sh
```

The test starts the stack, observes credential rotation, verifies public and private facts paths, observes explicit status-list revocation and recovery, verifies CRDT update credentials, confirms CRDT conflict resolution, confirms Enterprise B's direct catalog endpoint was not used by the consumer, then stops Docker Compose.

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
- `crdt-update-bus`: publishes signed `AgentFactsCRDTUpdateCredential` documents.
- `enterprise-a-registry`: public direct catalog source.
- `enterprise-b-registry`: private source registry; kept off the consumer's networks.
- `private-facts-gateway`: neutral host for Enterprise B's signed catalog.
- `enterprise-a-reverse`: MCP server with `reverse`.
- `enterprise-a-uppercase`: MCP server with `uppercase`.
- `enterprise-b-truncate`: MCP server with `truncate`.
- `enterprise-b-count`: MCP server with `count`.
- `consumer`: resolves both enterprises, verifies revocation status, merges CRDT updates, and calls tools only after verification.
- `swimlane`: highlights rotations, revocations, CRDT publishing, CRDT conflict resolution, and failed verification with `!!!`.

## Manual Inspection

```sh
# Lists enterprise registry names known to NANDA.
curl http://localhost:18080/registries

# Returns Enterprise A's public registry address with credentialStatus.
curl http://localhost:18080/resolve/enterprise-a.registry.nanda.local

# Returns Enterprise B's private registry address with credentialStatus and privateFactsURL.
curl http://localhost:18080/resolve/enterprise-b.registry.nanda.local

# Direct public catalog for Enterprise A. The catalog includes crdtUpdateURL.
curl http://localhost:18081/catalog

# Direct source catalog for Enterprise B, exposed to host only for inspection.
curl http://localhost:18082/catalog

# Neutral private facts path used by the consumer for Enterprise B.
curl http://localhost:18083/private-facts/enterprise-b/catalog

# Signed W3C-style status list used for revocation checks.
curl http://localhost:18084/status-lists/level6-revocation

# Signed CRDT AgentFacts update credential for Enterprise A.
curl http://localhost:18085/crdt/enterprise-a/updates

# Signed CRDT AgentFacts update credential for Enterprise B.
curl http://localhost:18085/crdt/enterprise-b/updates
```

Generated runtime files are ignored:

- `bin/`
- `artifacts/`
- `logs/`
