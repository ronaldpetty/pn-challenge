# Level 6 Plan

Level 6 demonstrates a local CRDT-based AgentFacts update protocol while preserving all Level 5 behavior.

## Goal

Show dynamic AgentFacts updates without rewriting the lean NANDA index.

## Shape

- Enterprise A: public facts through direct `catalogURL`.
- Enterprise B: private facts through `privateFactsURL`.
- Consumer uses both facts paths in the same run.
- Catalogs include `crdtUpdateURL`.
- `crdt-update-bus` publishes signed `AgentFactsCRDTUpdateCredential` documents.
- Consumer verifies and merges CRDT updates after catalog verification.
- Every address, catalog, status-list, and CRDT update VC remains signed.
- Status-list revocation from Level 5 remains active.

## CRDT Details

- `lww-register` handles conflicting `routingProfile` updates.
- `or-set-add` handles additive telemetry endpoints and capability tags.
- Two simulated replicas publish concurrent updates.
- The consumer logs deterministic conflict resolution and the merged state hash.

## Verification

- NANDA signs registry address credentials.
- Enterprise catalogs are signed and short-lived.
- CRDT update credentials are signed and short-lived.
- Status lists are signed and checked by index.
- Consumer verifies signature, expiration, and revocation status before using a VC.
- Test logs prove public facts, private facts, explicit revocation, CRDT publishing, CRDT merge, conflict resolution, and tool calls all work in one demo.
