# NANDA Index — Technical Challenge Submission

**Challenge:** Beyond DNS: Unlocking the Internet of AI Agents via the NANDA Index and Verified AgentFacts  
**Paper:** arxiv.org/pdf/2507.14263

---

## Quick Start

```sh
cd level8
./scripts/test-e2e.sh
```

For an interactive run with live logs:

```sh
cd level8
docker compose up --build
```

See `level8/LEVEL8_WALKTHROUGH.md` for a full walkthrough including registering a host Python agent, creating a dynamic runtime registry, and doing the discovery-to-tool-call flow manually with `curl`. See `level8/CURL_CLIENT_GUIDE.md` for a client-only `curl` flow.

---

## What Was Built

The implementation is in `level8/`. It demonstrates the core NANDA resolution flow end-to-end:

```
AgentName → Index → AgentAddr → FactsURL/PrivateFactsURL → AgentFacts → Endpoint
```

On each consumer cycle (`internal/consumer/consumer.go`):

1. Searches NANDA A, discovers NANDA B through `/peers`, searches both indexes.
2. Resolves a signed `EnterpriseRegistryAddrCredential` (AgentAddr) from the advertising index.
3. Verifies: Ed25519 signature → expiration → StatusList2021 revocation bit.
4. Fetches the signed `EnterpriseMCPCatalogCredential` (AgentFacts) from either the public catalog URL or `privateFactsURL` through a neutral gateway.
5. Verifies the same way, then fetches and verifies a signed CRDT update credential for dynamic metadata.
6. Cross-checks live MCP `tools/list` against what the catalog declared.
7. Calls a tool only after all checks pass.

Tampering at any layer fails signature verification and the consumer skips that registry.

### Level 1 — Required

- Two federated NANDA indexes (`nanda-a`, `nanda-b`) with peer discovery.
- Four built-in MCP tool agents plus dynamic runtime registration.
- Full `index → AgentAddr → AgentFacts` path in code.
- Ed25519-signed W3C-style VCs with canonical JSON; client detects tampering.

### Level 2 — Bonus

- **Different registration types:** static enterprise registries that join an index on startup; runtime registries created from flags with no code change; ad-hoc agent registration via `POST /agents/register`.
- **Visualization:** `swimlane` service tails all JSONL audit logs and prints a live activity stream; rotation events, revocations, key lifecycle, and verification failures are highlighted.
- **Test harness:** `scripts/test-e2e.sh` asserts federation, revocation/recovery, privacy path, CRDT merge, key rotation lifecycle, dynamic registration, and tool execution.
- **CLI client guide:** `CURL_CLIENT_GUIDE.md` walks the full discovery flow with plain `curl`.

### Additional Features

- **PrivateFactsURL:** Enterprise B's facts are served through a neutral `private-facts-gateway`; the consumer never contacts the source registry directly.
- **StatusList2021 revocation:** a `revocation-authority` revokes an active catalog credential mid-lifetime; the consumer rejects it and recovers after rotation.
- **CRDT-based AgentFacts updates:** a `crdt-update-bus` publishes signed update credentials with LWW-register and OR-set operations outside the index path.
- **Issuer key rotation:** a `issuer-key-rotator` prepublishes a new Ed25519 key, promotes it, keeps the previous key trusted during overlap, then retires it; the consumer reloads the trust bundle each cycle and verifies by `proof.verificationMethod`.
- **Tool and agent search:** NANDA `/search` filters by `registrationType`, `tool`, or `agent` and returns matching registry and agent endpoint metadata.

---

## What Was Set Aside

- **AdaptiveResolverURL** — the paper's programmable routing layer (geo-dispatch, load balancing, signed ephemeral tokens). The index and facts layers are fully modeled; this routing tier is not.
- **Full JSON-LD and W3C VC v2 compliance** — VCs are structurally correct but not validated against a JSON-LD processor; the v1 context URL is used.
- **DID resolution** — issuer IDs are `did:web:` strings used as identifiers; no DID document resolution over HTTP.
- **SBOM hashes, certifications, third-party audit claims** — capabilities and tools are modeled but not the full AgentFacts audit/certification schema.
- **Real decentralized hosting** — the privacy path uses a local Docker gateway; no IPFS, CDN, or Tor relay.
- **Internet-scale sharding and edge caching** — two local indexes demonstrate the quilt concept; no replication or consensus protocol.

Full feature coverage against the paper is in `level8/features.md`.

---

## How The Work Progressed

The build went through eight iterations, each closing one paper concept at a time. See `level_overview.md` for the full level-by-level summary.

---

## AI Tools

Claude Code (Anthropic) was used throughout — for code generation, architecture decisions, and iterative review. Each level was reviewed against the paper, gaps were identified, and the next level was built to close one gap. The reviews drove the feature roadmap rather than up-front planning. This README was also written with Claude Code.
