# Level 3 Plan

Level 3 demonstrates credential freshness and recovery.

## Goal

Keep the Level 2 enterprise registry topology, but make signed JSON credentials short-lived so the consumer must continuously verify freshness instead of trusting cached metadata.

## Scope

- New isolated `level3/` directory.
- Existing `level1/` and `level2/` directories stay unchanged.
- No host Go build required.
- Services stay alive until Docker Compose is stopped.

## Credential Choice

The rotated credentials are VC-shaped signed JSON documents:

- `EnterpriseRegistryAddrCredential`, analogous to AgentAddr.
- `EnterpriseMCPCatalogCredential`, analogous to AgentFacts.

The catalog credential is the closer match for the paper's metadata freshness concern because it carries capabilities and endpoints. Rotating both shows the full resolution path.

## Expected Behavior

- Registry-address and catalog credentials each expire after random 5-10 second TTLs.
- Their TTLs are staggered so both failure types are visible.
- Rotation waits an additional random 2-4 seconds after the first layer expires.
- Consumer verification succeeds during fresh windows.
- Consumer verification fails during expired windows.
- Consumer verification recovers after rotation.
- Swimlane marks rotations and failed verifications with `!!!`.
