# Level 8 Docker Step By Step

This file starts the Level 8 demo one group at a time. It is intentionally more verbose than `./scripts/test-e2e.sh` so you can see what each container represents.

Run all commands from this directory:

```sh
cd level8
```

## Clean Start

```sh
# Stop any old Level 8 containers and remove the old compose network attachments.
docker compose down --remove-orphans

# Build the shared Ubuntu-based image used by every modeled component.
docker compose build
```

## Build And Static Artifacts

```sh
# Compile every Go component into ./bin using the shared Ubuntu-based image.
# This represents the local build environment, not a host Go install.
docker compose up go-build

# Generate static demo artifacts in ./artifacts:
# self-signed CA/certs, issuer keys, trust bundle, W3C-style VCs,
# AgentAddr-like registry records, AgentFacts-like catalogs, and status-list data.
docker compose up artifact-init
```

## Federated NANDA Indexes

```sh
# Start NANDA index A.
# This represents one discovery index that can accept enterprise registry joins.
docker compose up -d nanda-index-a

# Start NANDA index B.
# This represents a second discovery index federated with index A through /peers.
docker compose up -d nanda-index-b

# Confirm both indexes are running and healthy.
docker compose ps nanda-index-a nanda-index-b
```

## Security And Update Support Services

```sh
# Start credential rotation.
# This continuously rewrites short-lived signed registry credentials.
docker compose up -d credential-rotator

# Start issuer key rotation.
# This prepublishes, promotes, and later retires issuer keys in the trust bundle.
docker compose up -d issuer-key-rotator

# Start the revocation authority.
# This hosts the signed VC status list used to show explicit revocation.
docker compose up -d revocation-authority

# Start the CRDT update bus.
# This hosts signed catalog update events that the consumer verifies and merges.
docker compose up -d crdt-update-bus

# Start the private facts gateway.
# This represents the privacy-preserving path for Enterprise B facts.
docker compose up -d private-facts-gateway
```

## Enterprise Registries

```sh
# Start Enterprise A registry.
# It joins NANDA index A and exposes a public catalog path.
docker compose up -d enterprise-a-registry

# Start Enterprise B registry.
# It joins NANDA index B and points clients to a PrivateFactsURL instead of direct facts.
docker compose up -d enterprise-b-registry

# Confirm both enterprise registries are healthy.
docker compose ps enterprise-a-registry enterprise-b-registry
```

## MCP Agents

```sh
# Start Enterprise A reverse-text MCP agent.
# This is one callable agent advertised by Enterprise A facts.
docker compose up -d enterprise-a-reverse

# Start Enterprise A uppercase MCP agent.
# This is another callable agent advertised by Enterprise A facts.
docker compose up -d enterprise-a-uppercase

# Start Enterprise B truncate MCP agent.
# This is one callable agent advertised through Enterprise B private facts.
docker compose up -d enterprise-b-truncate

# Start Enterprise B count MCP agent.
# This is another callable agent advertised through Enterprise B private facts.
docker compose up -d enterprise-b-count

# Confirm all modeled MCP agents are healthy.
docker compose ps enterprise-a-reverse enterprise-a-uppercase enterprise-b-truncate enterprise-b-count
```

## Consumer And Swimlane

```sh
# Start the dynamic client/consumer.
# It boots knowing only NANDA index A, discovers index B through /peers,
# searches both indexes, verifies signed credentials/facts, follows the public
# and private facts paths, applies CRDT updates, checks revocation, and calls MCP tools.
docker compose up -d consumer

# Print the audit swimlane.
# This reads logs and gives a compact view of joins, discovery, verification,
# credential rotation, revocation, CRDT updates, and tool calls.
docker compose up swimlane
```

## Useful Checks

```sh
# Show the current container state.
docker compose ps

# Watch the consumer's agentic loop.
docker compose logs -f consumer

# Watch registry join events at both indexes.
docker compose logs -f nanda-index-a nanda-index-b

# Watch credential and issuer key rotation.
docker compose logs -f credential-rotator issuer-key-rotator

# Stop the demo when done.
docker compose down --remove-orphans
```

## Host Ports

- `18080`: NANDA index A
- `18086`: NANDA index B
- `18081`: Enterprise A registry
- `18082`: Enterprise B registry
- `18083`: private facts gateway
- `18084`: revocation authority
- `18085`: CRDT update bus
- `18087`: Enterprise A reverse MCP agent exposed for manual `curl`

