# NANDA Level 1 Local Demo

This repo implements the Level 1 challenge only: a client resolves an agent name through a NANDA-style index, receives a signed `AgentAddr`, fetches signed `AgentFacts`, verifies both, and then calls the agent endpoint.

The demo is intentionally local and small:

- one HTTPS index service
- two HTTPS agents: `alpha.nanda.local` and `beta.nanda.local`
- two separate agent Docker networks: `agent_alpha_net` and `agent_beta_net`
- one Compose file
- one artifact init container that uses OpenSSL
- W3C Verifiable Credential shaped JSON for `AgentAddr` and `AgentFacts`
- Ed25519 signatures over deterministic JSON so the client detects tampering
- local CA signed TLS certificates for index and agent HTTPS endpoints

## Quick Test

Run the full containerized test:

```sh
./scripts/test-e2e.sh
```

The script builds the Ubuntu-based image, starts the Compose stack, compiles the Go app inside the `go-build` container, writes the Linux binary to `./bin/pn-demo`, initializes certificates and credentials in `./artifacts`, and runs the `e2e-test` container. The test resolves both agents, verifies both credentials, proves tampered `AgentFacts` fail verification, and invokes each agent.

## Interactive Docker Compose Run

This still uses Docker. It is "interactive" because you start the services yourself and test with curl instead of running the full e2e script.

Start the services:

```sh
mkdir -p bin artifacts
docker compose up --build index agent-alpha agent-beta
```

If you already have older artifacts and curl reports a certificate name mismatch, regenerate them:

```sh
rm -rf artifacts
mkdir -p bin artifacts
docker compose up --build index agent-alpha agent-beta
```

In another terminal, test with curl:

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

Stop the stack:

```sh
docker compose down --remove-orphans
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

## Artifacts

`artifact-init` writes generated files to `./artifacts`:

- `tls/ca/ca.crt`: local CA certificate for curl and the Go client
- `tls/index`, `tls/agent-alpha`, `tls/agent-beta`: CA-signed service certs and keys
- `keys/nanda-issuer.pem`: Ed25519 VC issuer private key
- `trust/issuers.json`: trusted issuer public key
- `index/agents.json`: signed AgentAddr credentials
- `agents/alpha/facts.vc.json` and `agents/beta/facts.vc.json`: signed AgentFacts credentials

`./artifacts` and `./bin` are ignored by git.

## Scope Notes

This is not a full Level 2 implementation. It does not include enterprise routing, DID resolution, revocation, adaptive routing, private facts gateways, or JSON-LD remote context processing. The VC documents use W3C VC fields and Ed25519 proof metadata, with local deterministic JSON signing for a simple, testable Level 1 tamper check.
