# Level 8 Security Review

This demo is safe to share as source for a local prototype. It should not be exposed as a network service or treated as production security without the hardening listed below.

## Key Findings

- High: `POST /agents/register` is unauthenticated. A reachable registry can accept an arbitrary agent endpoint, rewrite the enterprise catalog, and sign that endpoint into the facts credential. The consumer later verifies the signed catalog and calls advertised MCP tool URLs. This is intentional for the local dynamic-registration demo, but in an exposed environment it can become SSRF or tool-call redirection.
  - Relevant code: `internal/shared/shared.go` `RunRegistryWithOptions`, `registerDynamicAgent`, `writeEnterpriseCatalogCredential`, `validateAgentSpec`.
  - Hardening: require auth or mTLS for registration, apply endpoint allowlists, restrict private-network and host-gateway egress, and separate demo signing authority from untrusted registration input.

- Medium: `POST /join` on the NANDA index accepts registry records without verifying the credential at ingest. The consumer does verify records after `/resolve`, but `/search` can return compact `agentMatches` from joined metadata. Clients must treat search results as hints and perform resolve plus credential verification before calling anything.
  - Relevant code: `internal/shared/shared.go` `RunIndex`.
  - Hardening: verify joined credentials before accepting them, reject expired or revoked joins, and consider omitting endpoint metadata from search responses unless already verified.

- Medium: consumer URL fetching has no response-size cap, redirect policy, or egress allowlist. `getBytes` and `callTool` use the default HTTP client behavior and read whole response bodies.
  - Relevant code: `internal/shared/shared.go` `getBytes`, `getJSON`, `callTool`.
  - Hardening: cap response sizes, restrict redirects, enforce allowed schemes and hosts, and block loopback, metadata, host-gateway, and private ranges where inappropriate.

- Low/operational: Docker Compose publishes demo ports on all interfaces by default, and the consumer maps `host.docker.internal` to the host gateway. This is convenient for the walkthrough but should be run only on a trusted local machine.
  - Relevant code: `compose.yaml`.
  - Hardening: bind host ports to `127.0.0.1`, avoid host-gateway mapping outside the walkthrough, or run behind an authenticated local-only development network.

- Low: services run as root in an Ubuntu image that includes the Go toolchain. This is acceptable for the build-and-run demo image, but not ideal for a hardened runtime.
  - Relevant code: `Dockerfile`, `compose.yaml`.
  - Hardening: use a non-root runtime user, split build and runtime images, and mount artifacts with the minimum necessary access.

## Secret And Artifact Handling

- No generated keys, credentials, binaries, logs, or Go caches should be committed.
- `level8/.gitignore` excludes `artifacts/`, `bin/`, `logs/`, `.gocache/`, and `.gomodcache/`.
- `level8/.dockerignore` only sends `go.mod`, `cmd/`, and `internal/` to the Docker build context.
- If sharing a zip or tarball instead of the git repository, clean ignored local directories first so generated key material from `artifacts/keys/issuer.json` is not included.

## Positive Security Properties Demonstrated

- Signed registry address, catalog, CRDT update, and status-list credentials.
- Consumer verification of issuer trust, signature, expiration, and status-list revocation before acting on facts.
- Short-lived credentials with visible verification failures and recovery.
- Issuer key rotation with trust-bundle reload and old-key retirement.
- Public and private facts paths, with Enterprise B resolved through `privateFactsURL`.
- Two-index federation and dynamic registry/agent discovery flow.

## Sharing Verdict

Share as a local, educational prototype. Do not present it as production-secure or internet-safe without authentication, egress controls, verified index ingestion, local-only port binding, and runtime hardening.
