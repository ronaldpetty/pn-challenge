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
- `level1/LEVEL1_RUNBOOK.md`

## Level 2

Level 2 lives in `level2/` and demonstrates enterprise-routed registration. NANDA registers two fake enterprise MCP registry proxies, and a consumer searches for those registries through NANDA, verifies their signed catalogs, compares live MCP tool lists to those catalogs, executes skills, and logs the flow.

Run it:

```sh
cd level2
./scripts/test-e2e.sh
```

Read:

- `level2/README.md`
- `level2/LEVEL2_RUNBOOK.md`

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
- `level3/LEVEL3_RUNBOOK.md`

## Level 4

Level 4 lives in `level4/` and adds a local `PrivateFactsURL` demo. Enterprise A uses the original public direct catalog path, while Enterprise B uses a neutral `private-facts-gateway`; the consumer exercises both paths in the same run while still verifying signed, rotating credentials.

Run it:

```sh
cd level4
./scripts/test-e2e.sh
```

Read:

- `level4/README.md`
- `level4/LEVEL4_RUNBOOK.md`

## Level 5

Level 5 lives in `level5/` and keeps Level 4's public/private facts paths while adding explicit W3C-style VC status-list revocation. A local `revocation-authority` serves a signed status list, revokes an active Enterprise B catalog credential before TTL expiry, and the consumer rejects it until the next rotated credential recovers.

Run it:

```sh
cd level5
./scripts/test-e2e.sh
```

Read:

- `level5/README.md`
- `level5/LEVEL5_RUNBOOK.md`
