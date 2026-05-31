# Project NANDA Challenge Demos

This repository contains separate local Docker Compose demos for the NANDA technical challenge.

## Level 1

Level 1 lives in `level1/` and demonstrates the required `index -> AgentAddr -> AgentFacts` flow for two directly registered agents.

Run it:

```sh
cd level1
./scripts/test-e2e.sh
```

Read:

- `level1/README.md`

## Level 2

Level 2 lives in `level2/` and demonstrates enterprise-routed registration. NANDA registers two fake enterprise MCP registry proxies, and a consumer searches for those registries through NANDA, verifies their signed catalogs, compares live MCP tool lists to those catalogs, executes skills, and logs the flow.

Run it:

```sh
cd level2
./scripts/test-e2e.sh
```

Read:

- `level2/README.md`

## Level 3

Level 3 lives in `level3/` and keeps the Level 2 enterprise registry shape, but adds short-lived signed JSON credentials. Registry address credentials and enterprise catalog credentials expire after a random 5-10 seconds, rotate after a deliberate expired gap, and the consumer plus swimlane keep running until Docker Compose is stopped.

Run the bounded verification script:

```sh
cd level3
./scripts/test-e2e.sh
```

Run the live demo:

```sh
cd level3
docker compose up --build
```

Read:

- `level3/README.md`

## Level 4

Level 4 lives in `level4/` and adds a local `PrivateFactsURL` demo. Enterprise A uses the original public direct catalog path, while Enterprise B uses a neutral `private-facts-gateway`; the consumer exercises both paths in the same run while still verifying signed, rotating credentials.

Run it:

```sh
cd level4
./scripts/test-e2e.sh
```

Read:

- `level4/README.md`

## Level 5

Level 5 lives in `level5/` and keeps Level 4's public/private facts paths while adding explicit W3C-style VC status-list revocation. A local `revocation-authority` serves a signed status list, revokes an active Enterprise B catalog credential before TTL expiry, and the consumer rejects it until the next rotated credential recovers.

Run it:

```sh
cd level5
./scripts/test-e2e.sh
```

Read:

- `level5/README.md`

## Level 6

Level 6 lives in `level6/` and keeps Level 5's privacy, rotation, and revocation behavior while adding a local CRDT-based AgentFacts update protocol. Catalogs point to a signed `crdt-update-bus`; the consumer verifies and merges update operations without rewriting the NANDA index.

Run it:

```sh
cd level6
./scripts/test-e2e.sh
```

Read:

- `level6/README.md`

## Level 7

Level 7 lives in `level7/` and keeps Level 6's privacy, revocation, and CRDT behavior while adding issuer signing-key rotation. A local key rotator prepublishes a new verification key, promotes it to active, keeps the previous key trusted during overlap, then retires it while services continue running.

Run it:

```sh
cd level7
./scripts/test-e2e.sh
```

Read:

- `level7/README.md`

## Level 8

Level 8 lives in `level8/` and keeps Level 7's privacy, revocation, CRDT, and issuer key rotation behavior while adding a federated NANDA quilt. It runs two local indexes, lets enterprise registries dynamically join their home index, supports runtime registry creation, accepts runtime agent registration through registries, and lets clients search by registry, tool, or agent before resolving signed facts and calling MCP tools.

Run it:

```sh
cd level8
./scripts/test-e2e.sh
```

Read:

- `level8/README.md`
- `level8/LEVEL8_WALKTHROUGH.md`
- `level8/CURL_CLIENT_GUIDE.md`
- `level8/features.md`
