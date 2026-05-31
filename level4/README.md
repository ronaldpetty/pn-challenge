# NANDA Level 4 Private Facts Demo

This demo leaves `level1/`, `level2/`, and `level3/` unchanged. It starts from Level 3's rotating signed credentials and adds a mixed facts resolution model:

- Enterprise A is public. The consumer fetches its signed catalog directly from `enterprise-a-registry`.
- Enterprise B is private. The consumer receives both a direct `catalogURL` and a neutral `privateFactsURL`, then intentionally uses `privateFactsURL`.

The private path is local and simple:

```text
consumer -> nanda-index -> private-facts-gateway -> signed Enterprise B catalog
```

The gateway is not trusted for truth. It only serves signed JSON. The consumer still verifies the signature and expiration before using the catalog.

## Run A Bounded Test

```sh
./scripts/test-e2e.sh
```

The test starts the stack, observes credential rotation, verifies both public and private facts paths, checks that Enterprise B's direct catalog endpoint was not used by the consumer, then stops Docker Compose.

## Run The Live Demo

```sh
mkdir -p bin artifacts logs
docker compose up --build
```

Stop it:

```sh
docker compose down --remove-orphans
```

## Services

- `nanda-index`: serves current signed registry address credentials.
- `credential-rotator`: rewrites signed JSON credentials with random short TTLs.
- `enterprise-a-registry`: public direct catalog source.
- `enterprise-b-registry`: private source registry; kept off the consumer's networks.
- `private-facts-gateway`: neutral host for Enterprise B's signed catalog.
- `enterprise-a-reverse`: MCP server with `reverse`.
- `enterprise-a-uppercase`: MCP server with `uppercase`.
- `enterprise-b-truncate`: MCP server with `truncate`.
- `enterprise-b-count`: MCP server with `count`.
- `consumer`: loops forever, resolves both enterprises, uses public facts for A and private facts for B.
- `swimlane`: loops forever and highlights rotations and failed verification with `!!!`.

## Manual Inspection

```sh
# Lists enterprise registry names known to NANDA.
curl http://localhost:18080/registries

# Returns Enterprise A's public registry address. The consumer uses catalogURL.
curl http://localhost:18080/resolve/enterprise-a.registry.nanda.local

# Returns Enterprise B's private registry address. The consumer uses privateFactsURL.
curl http://localhost:18080/resolve/enterprise-b.registry.nanda.local

# Direct public catalog for Enterprise A.
curl http://localhost:18081/catalog

# Direct source catalog for Enterprise B, exposed to host only for inspection.
curl http://localhost:18082/catalog

# Neutral private facts path used by the consumer for Enterprise B.
curl http://localhost:18083/private-facts/enterprise-b/catalog
```

Generated runtime files are ignored:

- `bin/`
- `artifacts/`
- `logs/`
