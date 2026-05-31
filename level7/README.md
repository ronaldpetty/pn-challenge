# NANDA Level 7 Key Rotation Demo

Level 7 keeps Level 6's public/private facts paths, short-lived credentials, explicit status-list revocation, and signed CRDT update stream, then adds issuer signing-key rotation.

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

## Paper Mapping

`2507.14263v1.pdf` calls out a lean index, signed AgentFacts, private facts resolution, fast revocation, CRDT-based updates, and key lifecycle concerns. Level 7 maps those pieces this way:

- `EnterpriseRegistryAddrCredential` is the AgentAddr-like record returned by NANDA.
- `EnterpriseMCPCatalogCredential` is the base AgentFacts-like signed metadata.
- `catalogURL` is the direct source path.
- `privateFactsURL` is the neutral-hosted facts path.
- `credentialStatus` points each VC at a signed local status list.
- `StatusList2021Credential` is the W3C-style revocation list served by `revocation-authority`.
- `crdtUpdateURL` points from the signed catalog to signed dynamic AgentFacts updates.
- `AgentFactsCRDTUpdateCredential` is the signed CRDT update document served by `crdt-update-bus`.
- `proof.verificationMethod` names the issuer key that signed each credential.
- `issuer-key-rotator` prepublishes, activates, overlaps, and retires issuer verification keys.

This is a local demo implementation. The important behavior is separation: the index remains lean, AgentFacts can change through a signed CRDT update stream, revocation can invalidate a still-fresh credential, and verifier trust can move from one issuer key to the next.

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
- issuer key prepublish, active promotion, trust reload, and old-key retirement happened
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

## Run The Live Demo

```sh
mkdir -p bin artifacts logs
docker compose up --build
```

Stop it:

```sh
docker compose down --remove-orphans
```

## Index, Agents, And Verifiers

The index is a discovery layer. It tells the consumer where an enterprise registry can be found and returns a signed registry address credential. It does not make a tool call and it does not directly vouch for runtime tool output.

The enterprise registries and private facts gateway serve signed catalog credentials. Those catalogs describe the MCP agents and tool endpoints. The consumer verifies the index credential first, verifies the catalog next, verifies CRDT updates if present, checks revocation status, then calls the listed tools.

The trust bundle is the verifier's local view of issuer keys. Level 7 keeps multiple issuer keys in that bundle during rotation so credentials signed by the previous key and credentials signed by the new key can both verify during the overlap window.

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

## Issuer Key Rotation

Level 7 uses a local issuer keyring instead of one static issuer key:

- `active`: the key currently used to sign new credentials.
- `prepublished`: the next verification key, already present in the trust bundle before signers use it.
- `previous`: an older key kept in the trust bundle for a short overlap window.
- `retired`: an older key removed from verifier trust after overlap expires.

The rotation flow is:

```text
create new key -> publish to trust bundle -> wait -> promote active key -> keep previous key -> retire old key
```

Services keep running while this happens. The credential rotator, revocation authority, and CRDT update bus load the active signing key when they write signed JSON. The consumer reloads the trust bundle on each loop and verifies each credential against the key named in `proof.verificationMethod`.

Events to look for:

- `issuer_key_prepublished`
- `issuer_key_rotated`
- `old_issuer_key_retired`
- `trust_bundle_reloaded`
- `verified_with_issuer_key`

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

For registry address, catalog, status-list, and CRDT update credentials, the consumer verifies:

1. `proof.verificationMethod` exists in the current trust bundle
2. VC signature
3. VC expiration
4. status-list credential signature
5. status-list credential expiration
6. status bit at `statusListIndex`

Only after the base catalog and CRDT updates are accepted does the consumer compare live MCP tool lists and call tools.

## Revocation Behavior

Level 7 keeps Level 5 revocation behavior:

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

## Manual DNS-Style Curl Checks

With the stack running:

```sh
# Proves the NANDA index can be reached through a DNS-like host mapping.
curl --resolve nanda.local:18080:127.0.0.1 http://nanda.local:18080/registries

# Proves Enterprise A's index record exposes the public catalog URL and credentialStatus.
curl --resolve nanda.local:18080:127.0.0.1 http://nanda.local:18080/resolve/enterprise-a.registry.nanda.local

# Proves Enterprise B's index record exposes privateFactsURL and credentialStatus.
curl --resolve nanda.local:18080:127.0.0.1 http://nanda.local:18080/resolve/enterprise-b.registry.nanda.local

# Proves Enterprise A's public catalog includes crdtUpdateURL and the signing verificationMethod.
curl --resolve enterprise-a.registry.nanda.local:18081:127.0.0.1 http://enterprise-a.registry.nanda.local:18081/catalog

# Proves Enterprise B's source catalog exists but is only exposed to the host for inspection.
curl --resolve enterprise-b.registry.nanda.local:18082:127.0.0.1 http://enterprise-b.registry.nanda.local:18082/catalog

# Proves the neutral private facts path serves Enterprise B's signed catalog.
curl --resolve private-facts.enterprise-b.registry.nanda.local:18083:127.0.0.1 http://private-facts.enterprise-b.registry.nanda.local:18083/private-facts/enterprise-b/catalog

# Proves the signed W3C-style status list used for revocation checks is available.
curl --resolve revocation.nanda.local:18084:127.0.0.1 http://revocation.nanda.local:18084/status-lists/level7-revocation

# Proves the signed CRDT update stream for Enterprise A is available.
curl --resolve crdt.nanda.local:18085:127.0.0.1 http://crdt.nanda.local:18085/crdt/enterprise-a/updates

# Proves the signed CRDT update stream for Enterprise B is available.
curl --resolve crdt.nanda.local:18085:127.0.0.1 http://crdt.nanda.local:18085/crdt/enterprise-b/updates
```
