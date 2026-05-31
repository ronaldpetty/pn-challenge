# Level 1 Runbook

This document explains how to run the local demo scripts and how the repo satisfies **Level 1 - Required** from `Technical Challenge - VP of Engineering.pdf`.

## Prerequisites

Install these locally:

- Docker and Docker Compose

Go is installed inside the Ubuntu-based Docker image for the demo build. You only need local Go if you want to run unit tests directly on the host.

The first Docker build may download the `ubuntu:24.04` image and Ubuntu packages if they are not already cached. After the image is built, the demo runs locally with Docker Compose, Docker networking, local certificates, and locally generated signed credentials.

## Scripts

### Full End-to-End Test

Run:

```sh
./scripts/test-e2e.sh
```

What it does:

1. Builds the Ubuntu-based Docker image.
2. Starts the `go-build` container.
3. Installs/runs Go inside that Ubuntu container and writes the binary to `bin/pn-demo` through the shared `./bin:/shared-bin` volume.
4. Runs `artifact-init`, which creates local certificates and signed credentials under `artifacts/`.
5. Starts the index and both agents, each using the shared binary from `/shared-bin/pn-demo`.
6. Runs the `e2e-test` container.
7. Resolves both agents, verifies signatures, checks tamper detection, and invokes both agent endpoints.
8. Stops the Compose stack after the test container exits.

Expected successful output includes lines like:

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

### Artifact Init Script

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

## Interactive Docker Compose Run

This is still a Docker-based run. The difference from `./scripts/test-e2e.sh` is that you start the services yourself and use curl interactively instead of letting the e2e test container drive the whole flow.

There is no separate host-only run path documented here because the demo intentionally uses Docker Compose networking to put the two agents on different Docker networks, which is part of the requested setup.

Start the local services:

```sh
mkdir -p bin artifacts
docker compose up --build index agent-alpha agent-beta
```

Use curl from another terminal:

```sh
curl --noproxy '*' --cacert artifacts/tls/ca/ca.crt \
  --resolve index.nanda.local:8443:127.0.0.1 \
  https://index.nanda.local:8443/agents

curl --noproxy '*' --cacert artifacts/tls/ca/ca.crt \
  --resolve index.nanda.local:8443:127.0.0.1 \
  https://index.nanda.local:8443/resolve/alpha.nanda.local

curl --noproxy '*' --cacert artifacts/tls/ca/ca.crt \
  --resolve alpha.nanda.local:9443:127.0.0.1 \
  https://alpha.nanda.local:9443/facts

curl --noproxy '*' --cacert artifacts/tls/ca/ca.crt \
  --resolve alpha.nanda.local:9443:127.0.0.1 \
  -H 'content-type: application/json' \
  -d '{"message":"hello alpha"}' \
  https://alpha.nanda.local:9443/invoke

curl --noproxy '*' --cacert artifacts/tls/ca/ca.crt \
  --resolve beta.nanda.local:10443:127.0.0.1 \
  https://beta.nanda.local:10443/facts
```

These `--resolve` flags make curl use near-real names while still keeping everything local. They map the demo DNS names to `127.0.0.1` for the request, and the generated TLS certificates include those DNS names as certificate SANs.

Stop the services:

```sh
docker compose down --remove-orphans
```

## How Level 1 Is Accomplished

The challenge asks for a working prototype, not just architecture. Level 1 specifically requires a client to resolve an agent name and receive something it can verify and act on. The core path should be visible:

```text
index -> AgentAddr -> AgentFacts
```

This repo implements that path as follows.

### 1. Register at Least Two Agents

The artifact generator creates two local agents:

- `alpha.nanda.local`
- `beta.nanda.local`

Their signed index records are stored in:

```text
artifacts/index/agents.json
```

### 2. Resolve Agent Name Through an Index

The index service exposes:

```text
GET https://index.nanda.local:8443/resolve/alpha.nanda.local
GET https://index.nanda.local:8443/resolve/beta.nanda.local
```

Each response is a signed `AgentAddr` credential. In simple terms, `AgentAddr` tells the client where to fetch the agent's facts and where the runtime endpoint is.

### 3. Fetch AgentFacts

Each `AgentAddr` points to an agent facts endpoint:

```text
https://agent-alpha:8443/facts
https://agent-beta:8443/facts
```

Those endpoints return signed `AgentFacts` credentials. In simple terms, `AgentFacts` describes the agent: its name, capabilities, network, facts endpoint, and invoke endpoint.

### 4. Verify Before Acting

The client verifies both credentials before using them:

1. Verify the `AgentAddr` signature.
2. Read the `factsURL` from the verified `AgentAddr`.
3. Fetch `AgentFacts`.
4. Verify the `AgentFacts` signature.
5. Only then call the agent's `invoke` endpoint.

The credentials are W3C Verifiable Credential shaped JSON with Ed25519 proof metadata. The client uses the local trust bundle in:

```text
artifacts/trust/issuers.json
```

### 5. Detect Tampering

The e2e client intentionally mutates a fetched `AgentFacts` document in memory and verifies that the signature check fails. This demonstrates the Level 1 requirement that the client can detect tampering.

### 6. Act On Verified Data

After verification, the client calls each verified agent runtime endpoint:

```text
POST https://agent-alpha:8443/invoke
POST https://agent-beta:8443/invoke
```

This proves the client can not only resolve and verify metadata, but also use the result to interact with the agent.

## Local Security Model

The demo keeps security basic and local:

- HTTPS is used for the index and agents.
- TLS certificates are signed by a local CA generated with OpenSSL.
- `AgentAddr` and `AgentFacts` are signed with an Ed25519 issuer key.
- The client trusts only the generated local issuer public key.
- The two agents run on separate Docker networks.

This is intentionally Level 1 scope. It does not implement Level 2 features such as enterprise routing, DID resolution, revocation, adaptive routing, or private facts gateways.
