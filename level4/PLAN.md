# Level 4 Plan

Level 4 demonstrates mixed public and private facts resolution.

## Goal

Show `PrivateFactsURL` locally without adding external services. The demo keeps signed JSON verification and credential freshness from Level 3, but adds a neutral facts gateway for one enterprise.

## Shape

- Enterprise A: public facts through direct `catalogURL`.
- Enterprise B: private facts through `privateFactsURL`.
- Consumer uses both paths in the same run.
- Enterprise B direct registry stays alive but is not connected to the consumer's Docker networks.

## Privacy Property

The private gateway does not prove anonymity like Tor would. It proves the local version of the paper's decoupling point: the consumer can retrieve and verify signed facts without directly contacting the source registry for Enterprise B.

## Verification

- NANDA signs registry address credentials.
- Enterprise catalogs are signed and short-lived.
- Consumer verifies signature and expiration on both public and private paths.
- Consumer calls tools only after catalog verification.
- Test logs prove Enterprise A direct facts were used, Enterprise B private facts were used, and Enterprise B direct facts were not used by the consumer.
