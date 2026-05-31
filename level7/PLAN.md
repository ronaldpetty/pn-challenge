# Level 7 Plan

Level 7 demonstrates issuer signing-key rotation while preserving all Level 6 behavior.

## Goal

Show that the issuer key is not static. New credentials should move to a new signing key while verifiers keep enough overlap to accept still-fresh credentials signed by the previous key.

## Shape

- Enterprise A: public facts through direct `catalogURL`.
- Enterprise B: private facts through `privateFactsURL`.
- Consumer uses both facts paths in the same run.
- Catalogs include `crdtUpdateURL`.
- `crdt-update-bus` publishes signed `AgentFactsCRDTUpdateCredential` documents.
- `revocation-authority` publishes signed status-list credentials and pushes explicit revocation.
- `issuer-key-rotator` prepublishes, promotes, overlaps, and retires issuer keys.
- Consumer reloads the trust bundle and verifies each VC against its `proof.verificationMethod`.
- Every address, catalog, status-list, and CRDT update VC remains signed.

## Key Rotation Details

- `active` key signs new credentials.
- `prepublished` key is added to the trust bundle before signers use it.
- `previous` key stays trusted during overlap.
- `retired` key is removed from verifier trust after overlap expires.
- Logs highlight `issuer_key_prepublished`, `issuer_key_rotated`, `old_issuer_key_retired`, `trust_bundle_reloaded`, and `verified_with_issuer_key`.

## Verification

- NANDA signs registry address credentials.
- Enterprise catalogs are signed and short-lived.
- CRDT update credentials are signed and short-lived.
- Status lists are signed and checked by index.
- Consumer verifies signing key, signature, expiration, and revocation status before using a VC.
- Test logs prove public facts, private facts, explicit revocation, CRDT publishing, CRDT merge, conflict resolution, key rotation, trust reload, and tool calls all work in one demo.
