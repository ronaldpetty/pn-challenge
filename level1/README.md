# NANDA Level 1 Local Demo

Level 1 implements the required local `index -> AgentAddr -> AgentFacts` flow. A client resolves an agent name through a NANDA-style index, receives a signed `AgentAddr`, fetches signed `AgentFacts`, verifies both, and only then calls the agent endpoint.

The demo is intentionally small and local:

- one HTTPS index service
- two HTTPS agents: `alpha.nanda.local` and `beta.nanda.local`
- two separate agent Docker networks: `agent_alpha_net` and `agent_beta_net`
- one Compose file
- one artifact init container that uses OpenSSL
- local CA-signed TLS certificates for the index and agents
- VC-shaped signed JSON for `AgentAddr` and `AgentFacts`
- Ed25519 signatures over deterministic JSON so the client detects tampering

## Prerequisites

- Docker and Docker Compose

Go is installed inside the Ubuntu-based Docker image for the demo build. Local Go is only needed if you want to run host-side unit tests.

The first Docker build may download `ubuntu:24.04` and Ubuntu packages. After that, the demo runs locally with Docker Compose, Docker networking, local certificates, and locally generated signed credentials.

## Run

Run the full containerized test:

```sh
./scripts/test-e2e.sh
```

The script:

1. Builds the Ubuntu-based Docker image.
2. Starts the `go-build` container.
3. Installs/runs Go inside that Ubuntu container and writes the binary to `bin/pn-demo` through the shared `./bin:/shared-bin` volume.
4. Runs `artifact-init`, which creates local certificates and signed credentials under `artifacts/`.
5. Starts the index and both agents, each using the shared binary from `/shared-bin/pn-demo`.
6. Runs the `e2e-test` container.
7. Resolves both agents, verifies signatures, checks tamper detection, and invokes both agent endpoints.
8. Stops the Compose stack after the test exits.

Expected successful output includes:

```text
resolve alpha.nanda.local
  AgentAddr verified
  AgentFacts verified
  Tamper check passed
  Invoke response
resolve beta.nanda.local
  AgentAddr verified
  AgentFacts verified
  Tamper check passed
  Invoke response
```

## Interactive Docker Compose Run

This is still Docker-based. It is interactive because you start the services yourself and test with `curl` instead of letting the e2e script drive the whole flow.

Start the services:

```sh
mkdir -p bin artifacts
docker compose up --build index agent-alpha agent-beta
```

If older artifacts cause a certificate name mismatch, regenerate them:

```sh
rm -rf artifacts
mkdir -p bin artifacts
docker compose up --build index agent-alpha agent-beta
```

Use `curl` from another terminal:

```sh
# Proves the index is reachable by a DNS-like name and lists registered agents.
curl --noproxy '*' --cacert artifacts/tls/ca/ca.crt \
  --resolve index.nanda.local:8443:127.0.0.1 \
  https://index.nanda.local:8443/agents

# Proves name resolution returns a signed AgentAddr for alpha.nanda.local.
curl --noproxy '*' --cacert artifacts/tls/ca/ca.crt \
  --resolve index.nanda.local:8443:127.0.0.1 \
  https://index.nanda.local:8443/resolve/alpha.nanda.local

# Proves the alpha AgentAddr points to retrievable signed AgentFacts.
curl --noproxy '*' --cacert artifacts/tls/ca/ca.crt \
  --resolve alpha.nanda.local:9443:127.0.0.1 \
  https://alpha.nanda.local:9443/facts

# Proves the verified alpha runtime endpoint can be invoked over HTTPS.
curl --noproxy '*' --cacert artifacts/tls/ca/ca.crt \
  --resolve alpha.nanda.local:9443:127.0.0.1 \
  -H 'content-type: application/json' \
  -d '{"message":"hello alpha"}' \
  https://alpha.nanda.local:9443/invoke

# Proves the second registered agent has its own DNS-like facts endpoint.
curl --noproxy '*' --cacert artifacts/tls/ca/ca.crt \
  --resolve beta.nanda.local:10443:127.0.0.1 \
  https://beta.nanda.local:10443/facts
```

The `--resolve` flags make curl use near-real names while keeping everything local. They map demo DNS names to `127.0.0.1`, and the generated TLS certificates include those DNS names as certificate SANs.

Stop the stack:

```sh
docker compose down --remove-orphans
```

## Verification Choice

The challenge says: **"On verification: signed JSON, W3C Verifiable Credentials, or another approach of your choice"**.

This demo uses **signed JSON with W3C Verifiable Credential fields**:

- `AgentAddr` and `AgentFacts` are JSON documents.
- Each document has VC-style fields: `@context`, `type`, `issuer`, `issuanceDate`, `expirationDate`, `credentialSubject`, and `proof`.
- The `proof` contains an Ed25519 signature.
- The signature is computed over deterministic canonical JSON.
- The client verifies the signature with the trusted issuer public key in `artifacts/trust/issuers.json`.
- The e2e test mutates fetched `AgentFacts` in memory and proves the signature check fails.

In simple terms: the documents look like W3C Verifiable Credentials, but the implemented verification technique is the challenge's **signed JSON** option.

## How Level 1 Is Accomplished

Level 1 requires a working prototype where a client resolves an agent name and receives something it can verify and act on:

```text
index -> AgentAddr -> AgentFacts
```

Level 1 uses direct NANDA-native registration. The NANDA index directly stores signed `AgentAddr` records for:

- `alpha.nanda.local`
- `beta.nanda.local`

The signed index records are generated into:

```text
artifacts/index/agents.json
```

The index resolves agent names through:

```text
GET https://index.nanda.local:8443/resolve/alpha.nanda.local
GET https://index.nanda.local:8443/resolve/beta.nanda.local
```

Each response is a signed `AgentAddr` credential. In simple terms, `AgentAddr` tells the client where to fetch the agent's facts and where the runtime endpoint is.

Each `AgentAddr` points to an agent facts endpoint:

```text
https://agent-alpha:8443/facts
https://agent-beta:8443/facts
```

Those endpoints return signed `AgentFacts` credentials. `AgentFacts` describes the agent name, capabilities, network, facts endpoint, and invoke endpoint.

The client verifies before acting:

1. Verify the `AgentAddr` signature.
2. Read the `factsURL` from the verified `AgentAddr`.
3. Fetch `AgentFacts`.
4. Verify the `AgentFacts` signature.
5. Only then call the agent's `invoke` endpoint.

The local trust bundle is:

```text
artifacts/trust/issuers.json
```

The e2e client intentionally mutates a fetched `AgentFacts` document in memory and verifies that the signature check fails. After verification, the client calls:

```text
POST https://agent-alpha:8443/invoke
POST https://agent-beta:8443/invoke
```

## Index, Agents, And Verification Relationship

The index does not prove that an agent is safe to use by itself. It returns a signed `AgentAddr` that tells the client where the agent metadata and runtime endpoint are. The agent then serves signed `AgentFacts`.

The client is the verifier in both steps: it verifies the `AgentAddr` from the index, uses the verified `factsURL` to fetch `AgentFacts`, verifies those facts, and only then calls the verified `invokeURL`. In this demo, both `AgentAddr` and `AgentFacts` are signed by the local NANDA demo issuer, and the client trusts that issuer through `artifacts/trust/issuers.json`.

## Artifact Init

`scripts/init_artifacts.sh` is normally run by Docker Compose through the `artifact-init` service.

It creates:

- a local CA certificate
- TLS certificates for `index`, `agent-alpha`, and `agent-beta`
- an Ed25519 issuer key
- a trust bundle
- signed `AgentAddr` credentials
- signed `AgentFacts` credentials

The generated files are written to `artifacts/`.

To force fresh artifacts:

```sh
rm -rf artifacts
./scripts/test-e2e.sh
```

## Go Commands

These are optional host-side developer commands. The Docker demo path builds the Linux binary inside the `go-build` container.

Run unit tests:

```sh
mkdir -p .gocache .gomodcache
GOCACHE=$PWD/.gocache GOMODCACHE=$PWD/.gomodcache go test ./...
```

Build a local binary for the host:

```sh
go build -o bin/pn-demo-host ./cmd/pn-demo
```

## Generated Files

`artifact-init` writes generated files to `./artifacts`:

- `tls/ca/ca.crt`: local CA certificate for curl and the Go client
- `tls/index`, `tls/agent-alpha`, `tls/agent-beta`: CA-signed service certs and keys
- `keys/nanda-issuer.pem`: Ed25519 VC issuer private key
- `trust/issuers.json`: trusted issuer public key
- `index/agents.json`: signed AgentAddr credentials
- `agents/alpha/facts.vc.json` and `agents/beta/facts.vc.json`: signed AgentFacts credentials

`./artifacts` and `./bin` are ignored by git.

## Scope Notes

This is intentionally Level 1 scope. It does not include enterprise routing, DID resolution, revocation, adaptive routing, private facts gateways, or JSON-LD remote context processing. A standards-complete W3C VC implementation would require interoperable proof formats, canonicalization, JSON-LD context handling, key resolution, and verification rules beyond this local signed JSON demo.
