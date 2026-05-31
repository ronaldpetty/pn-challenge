# NANDA Level 7 Key Rotation Demo

This demo leaves `level1/` through `level6/` unchanged. It keeps Level 6's public/private facts paths, short-lived credentials, explicit status-list revocation, and signed CRDT update stream, then adds issuer signing-key rotation.

- Enterprise A is public. The consumer fetches its signed catalog directly from `enterprise-a-registry`.
- Enterprise B is private. The consumer uses `privateFactsURL` through `private-facts-gateway`.
- Every registry address, catalog, CRDT update, and status-list document is signed as a W3C-style VC.
- The local `revocation-authority` serves a signed status list and revokes active Enterprise B catalog credentials before expiry.
- The local `crdt-update-bus` serves signed CRDT update credentials outside the NANDA index path.
- The local `issuer-key-rotator` prepublishes a new issuer verification key, promotes it to active, keeps the previous key trusted for overlap, then retires it.
- The consumer reloads the trust bundle and verifies each credential against the `proof.verificationMethod` key that signed it.

The key rotation path is:

```text
issuer-key-rotator -> keyring -> trust bundle -> new signed VCs -> consumer verification by proof key
```

The overlap window matters: old credentials can still verify while they are fresh, new credentials can verify as soon as signers switch keys, and retired keys are logged once their overlap expires.

## Run A Bounded Test

```sh
./scripts/test-e2e.sh
```

The test starts the stack, observes credential rotation, observes issuer key prepublish/promote/retire events, verifies public and private facts paths, observes explicit status-list revocation and recovery, verifies CRDT update credentials, confirms CRDT conflict resolution, confirms Enterprise B's direct catalog endpoint was not used by the consumer, then stops Docker Compose.

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
- `issuer-key-rotator`: rotates the issuer signing key with prepublish and overlap windows.
- `revocation-authority`: serves a signed status list and revokes active Enterprise B catalog credentials before expiry.
- `crdt-update-bus`: publishes signed `AgentFactsCRDTUpdateCredential` documents.
- `enterprise-a-registry`: public direct catalog source.
- `enterprise-b-registry`: private source registry; kept off the consumer's networks.
- `private-facts-gateway`: neutral host for Enterprise B's signed catalog.
- `enterprise-a-reverse`: MCP server with `reverse`.
- `enterprise-a-uppercase`: MCP server with `uppercase`.
- `enterprise-b-truncate`: MCP server with `truncate`.
- `enterprise-b-count`: MCP server with `count`.
- `consumer`: resolves both enterprises, reloads trust, verifies revocation status, verifies issuer keys, merges CRDT updates, and calls tools only after verification.
- `swimlane`: highlights rotations, key updates, revocations, CRDT publishing, CRDT conflict resolution, and failed verification with `!!!`.

## Manual DNS-Style Inspection

With the stack running:

```sh
# Proves the NANDA index can be reached through a DNS-like host mapping.
curl --resolve nanda.local:18080:127.0.0.1 http://nanda.local:18080/registries

# Proves Enterprise A's index record exposes the public catalog URL and credentialStatus.
curl --resolve nanda.local:18080:127.0.0.1 http://nanda.local:18080/resolve/enterprise-a.registry.nanda.local

# Proves Enterprise B's index record exposes privateFactsURL and credentialStatus.
curl --resolve nanda.local:18080:127.0.0.1 http://nanda.local:18080/resolve/enterprise-b.registry.nanda.local

# Proves Enterprise A's public catalog includes crdtUpdateURL and a signed proof key.
curl --resolve enterprise-a.registry.nanda.local:18081:127.0.0.1 http://enterprise-a.registry.nanda.local:18081/catalog

# Proves Enterprise B's source catalog exists but is only exposed to the host for inspection.
curl --resolve enterprise-b.registry.nanda.local:18082:127.0.0.1 http://enterprise-b.registry.nanda.local:18082/catalog

# Proves the neutral private facts path serves Enterprise B's signed catalog.
curl --resolve private-facts.enterprise-b.registry.nanda.local:18083:127.0.0.1 http://private-facts.enterprise-b.registry.nanda.local:18083/private-facts/enterprise-b/catalog

# Proves the signed W3C-style status list used for revocation checks is available.
curl --resolve revocation.nanda.local:18084:127.0.0.1 http://revocation.nanda.local:18084/status-lists/level7-revocation

# Proves the signed CRDT AgentFacts update credential for Enterprise A is available.
curl --resolve crdt.nanda.local:18085:127.0.0.1 http://crdt.nanda.local:18085/crdt/enterprise-a/updates

# Proves the signed CRDT AgentFacts update credential for Enterprise B is available.
curl --resolve crdt.nanda.local:18085:127.0.0.1 http://crdt.nanda.local:18085/crdt/enterprise-b/updates
```

Generated runtime files are ignored:

- `bin/`
- `artifacts/`
- `logs/`
