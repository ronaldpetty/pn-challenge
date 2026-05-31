# Level 7 Feature Coverage

This list compares `level7/` against the feature set described in `2507.14263v1.pdf`.

## Implemented

- Lean index flow: `nanda-index` resolves registry names to signed address credentials.
- Enterprise registry model: NANDA points to enterprise registries instead of directly owning every agent.
- Public and private facts paths: Enterprise A uses a direct catalog; Enterprise B uses `privateFactsURL`.
- Signed metadata: registry address, catalog, CRDT updates, and status list are W3C-style verifiable credentials.
- VC verification: signature, expiration, issuer verification key, and status-list checks.
- Explicit revocation: signed `StatusList2021Credential` with pushed revocation.
- Credential freshness: short random TTLs plus verification failure and recovery logs.
- Issuer key rotation: prepublish, promote, overlap, retire, and trust-bundle reload.
- CRDT-based AgentFacts updates: signed update stream outside the index with deterministic merge and conflict resolution.
- Agent use after discovery: consumer discovers agents through NANDA, verifies facts, then calls MCP tools.

## Partially Implemented

- Lightweight index: the separation of index and facts exists, but records are not constrained to the paper's 120-byte target.
- Diverse registration models: enterprise registry indirection exists, but not government, Web3, DID marketplace, or native public registry variants.
- Privacy preservation: `privateFactsURL` and a neutral gateway exist, but there is no true anonymity, IPFS, Tor, mix-net, or traffic-correlation protection.
- Endpoint agility: short TTLs and dynamic metadata exist, but there is no rotating endpoint pool, geo failover, or DDoS shuffle.
- AgentFacts schema: the catalog/facts shape is minimal and signed, but not the full paper schema.

## Not Implemented

- Adaptive routing with `AdaptiveResolverURL`, geo-dispatch, load balancing, ephemeral endpoint tokens, or fallback routing rules.
- Full AgentFacts JSON-LD richness such as SBOM/code hashes, multi-tier endpoint arrays, security/auth fields, provider/jurisdiction, evaluations, certifications, or complete telemetry metadata.
- Distributed or federated index behavior such as sharding, index-to-index federation, cross-registry trust exchange, or replication.
- Full W3C VC v2 compliance and JSON-LD schema validation.
- DID resolution beyond string-form `did:web` issuer identifiers.
- Cross-signing, federated trust zones, threshold verification, or TRS reputation scoring.
- Hash-linked credential chains.
- Real CDN, IPFS, decentralized storage, or Tor-style relays.
- OpenTelemetry integration beyond simple demo logs.
- ZKP or private credential assertions.
- Internet-scale sharding, edge caching, or performance architecture.
