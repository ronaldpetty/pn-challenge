# Level 8 Plan

Level 8 demonstrates a small federated NANDA quilt while preserving all Level 7 behavior.

## Goal

Show discovery across more than one index. The client starts from one known index, follows peer metadata to another index, searches both, resolves registry address credentials from the proper index, verifies facts, and calls tools only after verification.

## Shape

- `nanda-a` and `nanda-b` are separate local index services.
- Each index exposes `/peers` with the other index.
- Enterprise A joins `nanda-a`.
- Enterprise B joins `nanda-b`.
- Enterprise B remains private from the consumer's point of view; the consumer uses `privateFactsURL`.
- Each component has its own command binary under `cmd/`.
- Shared VC, AgentAddr-like, AgentFacts-like, status-list, trust-bundle, and CRDT logic lives under `internal/shared`.

## Verification

- Both indexes log `registry_joined_quilt`.
- Enterprise registries periodically refresh their join after credential rotation.
- Consumer logs `discover_federated_index`.
- Consumer logs search results from both indexes.
- All previous Level 7 checks still pass.
