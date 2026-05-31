# NANDA Level 8 Federated Index Demo

Level 8 keeps Level 7's privacy path, short-lived credentials, explicit status-list revocation, CRDT update stream, and issuer key rotation. It adds a simple federated quilt:

- `nanda-a` and `nanda-b` are separate indexes.
- `nanda-a` advertises `nanda-b` through `/peers`; `nanda-b` advertises `nanda-a`.
- Enterprise A joins `nanda-a`; Enterprise B joins `nanda-b`.
- The consumer boots with only `nanda-a`, discovers `nanda-b`, searches both, resolves from the index that advertised each registry, verifies facts, then calls MCP tools.

## Run

```sh
./scripts/test-e2e.sh
```

For an interactive run:

```sh
mkdir -p bin artifacts logs
docker compose up --build
```

Stop it:

```sh
docker compose down --remove-orphans
```

## Main Files

- `LEVEL8_WALKTHROUGH.md`: single interactive guide for compose startup, Python agents, dynamic registries, registration, discovery, and tool calls.
- `LEVEL8_RUNBOOK.md`: operational runbook.
- `CURL_CLIENT_GUIDE.md`: manual cURL flow that mimics a dynamic client agent.
- `features.md`: feature coverage against `2507.14263v1.pdf`.

## Services

- `nanda-index-a`, `nanda-index-b`: federated indexes.
- `enterprise-a-registry`, `enterprise-b-registry`: registries that join the quilt.
- `private-facts-gateway`: neutral host for Enterprise B facts.
- `revocation-authority`: signed status-list revocation.
- `crdt-update-bus`: signed AgentFacts update stream.
- `credential-rotator`: short-lived VC rotation.
- `issuer-key-rotator`: issuer signing key rotation.
- `enterprise-*`: MCP tool agents.
- `consumer`: dynamic client/verifier.
- `swimlane`: audit log display.
