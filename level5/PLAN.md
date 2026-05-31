# Level 5 Plan

Level 5 demonstrates explicit VC status-list revocation while preserving all Level 4 behavior.

## Goal

Show that a credential can be rejected because it has been revoked, not only because its TTL expired.

## Shape

- Enterprise A: public facts through direct `catalogURL`.
- Enterprise B: private facts through `privateFactsURL`.
- Consumer uses both facts paths in the same run.
- Every address and catalog VC includes `credentialStatus`.
- `revocation-authority` serves a signed `StatusList2021Credential`.
- The revocation authority marks an active Enterprise B catalog credential revoked before expiry.

## Verification

- NANDA signs registry address credentials.
- Enterprise catalogs are signed and short-lived.
- Status lists are signed and checked by index.
- Consumer verifies signature, expiration, and revocation status before using a VC.
- Consumer calls tools only after catalog verification.
- Test logs prove Enterprise A direct facts were used, Enterprise B private facts were used, Enterprise B direct facts were not used by the consumer, and a revoked credential was rejected before TTL expiry.
