# Persistent approximate nearest neighbors

## Implemented scope and recall contract

The first M7 slice implements **HNSW** over legal SQLite tables and B-trees.
It performs real multilayer graph traversal, not an exact scan labeled as ANN.
Opening a database or explaining a query does not load the graph. Search fetches
individual nodes lazily from the current transaction's B-trees. Each returned
candidate is checked against the live identity mapping and the current source
row, then its distance is recalculated exactly. Exact distances do **not** imply
exact recall. `Plan.Exact`/SQL `VectorExact` remain false for a graph result.

Native `VectorSearch` remains exact by default. Explicit approximate mode opts in:

```go
err := db.CreateHNSWIndex(ctx, "items_hnsw", "items", "embedding")
result, err := db.VectorSearch(ctx, "items", "embedding", "[1,2,3]",
    sqlite.VectorSearchOptions{
        Mode: sqlite.VectorModeApproximate, Index: "items_hnsw",
        Limit: 10, EFSearch: 40, MaxEFSearch: 1000,
        Where: "active=1",
    })
```

A matching SQL `ORDER BY` distance ASC + finite LIMIT can automatically choose
HNSW. This SQL path is approximate after the index is installed. Use `NOT INDEXED`
for the exact reference or native exact mode. Only a declared `VECTOR(n) NOT NULL`
column is eligible for automatic ANN SQL in this slice, preserving the existing
NULL-first SQL order for nullable columns. Native search supports nullable columns
and excludes NULL. Filters are rechecked before source-vector decoding/scoring.

```sql
CREATE INDEX items_hnsw ON items(embedding) USING hnsw
  WITH(opclass='vector_l2_ops',m=16,ef_construction=64);
SELECT id,embedding <-> '[1,2,3]' AS distance
  FROM items WHERE active=1 ORDER BY distance LIMIT 10;
SELECT id,embedding <-> '[1,2,3]' AS distance
  FROM items NOT INDEXED WHERE active=1 ORDER BY distance LIMIT 10;
REINDEX items_hnsw;
```

This is project SQL grammar, not PostgreSQL's CREATE INDEX grammar. Aliases,
distance functions and bounded OFFSET use the same eligible-shape analysis as
exact ranking. Secondary sort keys or unsupported expressions retain the existing
exact/general path. Native query controls are not SQL SET/PRAGMA variables.

## Metrics, construction and bounds

One typed vector field, dimensions1..2000, four metric/operator-class pairs:

| Metric | SQL distance | Operator class |
|---|---|---|
| L2 | `<->` | vector_l2_ops |
| L1 | `<+>` | vector_l1_ops |
| negative inner product | `<#>` | vector_ip_ops |
| cosine distance | `<=>` | vector_cosine_ops |

A metric/opclass/dimension/source mismatch never reuses the index. A missing or
dirty index gives an explained exact fallback. Unknown version/configuration or
inconsistent owned storage errors rather than being silently trusted.

`HNSWIndexOptions` configures Metric, Opclass, M, EFConstruction, Seed, MaxNodes,
MaxTombstones, MaxVisited, MaxDistanceEvaluations and MaxMemoryBytes. Native zero
fields inherit defaults. SQL explicitly supplied zero limits reject (seed0 selects
the fixed default). Defaults: m16, ef_construction64, deterministic seed
0x243f6a8885a308d3, max100000 nodes including tombstones,25000 tombstones,
4096 visited nodes/operation,1000000 distance evaluations/operation and32MiB
logical retained algorithm memory. m2..100, ef_construction4..1000 and >=2m.
Cosine indexes reject every non-NULL zero-norm source value, including later
writes; they do not silently omit it. This differs from pgvector's zero-vector
index exclusion and preserves the native exact error contract.

Native query defaults: ef_search40, max_ef_search1000, cumulative100000 node
reads,1000000 graph-distance evaluations,16 widening iterations and32MiB graph
cache. `ANNWorkLimits` permits bounded overrides; persisted construction limits
still constrain each graph operation. Existing `vector.SearchLimits` also charge
source fetches and all graph/base vector components. DB.MaxSortBytes bounds
logical result/bookkeeping memory. These are conservative accounting bounds,
not exact process RSS guarantees; one structurally bounded fetched node and
transient decoding overhead can exist outside the graph cache.

## Lifecycle, filtering and fallback

HNSW has deterministic geometric levels up to32, greedy upper-layer descent,
best-first layer search, diversity selection and bounded reciprocal pruning.
Layer0 has at most2m neighbors; upper layers at mostm. Signed rowid breaks equal
score ties. The local topology and compensated float64 arithmetic need not match
pgvector bit-for-bit. Production code is independently written Go.

INSERT creates a new immutable node identity. DELETE tombstones the node;
UPDATE or rowid replacement tombstones the old identity and creates another.
Tombstones retain their original vector and edges for traversal. Byte-identical
vector and rowid updates preserve identity. Live mapping, source rows, graph and
catalog state share the same transaction and statement rollback. Reindex builds
and atomically replaces the graph, compacts tombstones and advances generation.
Construction configuration is immutable in this slice: Reindex uses the existing
config; changing parameters requires another named index. DROP is not implemented.

Filtering never removes graph routing nodes. The query widens ef, deduplicates
immutable candidate identities and rechecks the original WHERE. Insufficient
eligible results at max_ef_search cause an explained exact scan with already
spent work still charged. A short frontier is not treated as proof of exact
coverage. Explicit resource limits can return an error; no partial native result
is returned on error. Streaming SQL users must check Rows.Err.

Exact official-SQLite guard triggers mark indices dirty on external source DML;
schema-cookie/source-hash checks cover schema epochs. Dirty indices scan and
native mutations require REINDEX. No arbitrary trigger execution is implemented.
Normal rollback, close/reopen, shared snapshots and checkpoint are tested;
physical crash/recovery durability remains unqualified.

## Storage and diagnostics

Original seven grams extension objects are unchanged. An optional complete
ANN-v1 family adds five ordinary tables (state, nodes, live mapping, centroids,
members) and six lookup indices. The centroid/member tables are empty and checked
as such for HNSW; their presence is not IVFFlat support. Old grams-only databases
remain usable and add the family on first ANN creation. Actual pre-ANN fixture
upgrade is tested. Node adjacency has a strict versioned bounded binary encoding;
original vectors use the GSV codec. All metadata uses canonical versioned config.

Plan/Stats expose method, metric/opclass, generation, requested/effective ef,
stop/fallback reason, state/node/edge reads, visited nodes, graph metric calls,
identity lookups, exact source rescoring, iterations and logical peak bytes.
Counters are actual logical work, not physical I/O. Widening and abandoned graph
attempts remain counted. EXPLAIN reads only schema/config, not graph state/nodes.
`CheckExtensionIndex` explicitly scans and reconciles full graph/source state;
normal search/open does not. The checker is not general SQLite integrity_check.

## Evidence and remaining work

Run `go run ./examples/ann NEW_DISPOSABLE_DB` and the reproducible command in
[benchmarks/ann](../benchmarks/ann/README.md). First persisted/reopened measurement:
2048×12 dimensions,48 held-out queries, recall@1=1, @10=1, @100=0.999375;
no exact fallbacks. @10 source fetches40 vs2048 and graph distances575.08 vs2048.
However p50 HNSW40.62ms was **slower** than exact2.12ms on this small dataset;
index build49.38s, total file1,462,272bytes vs286,720 before indexing. These are
shared-cloud observations, not a latency speedup or universal recall guarantee.

IVFFlat is still required implementation work, not an acceptance-only gap.
Probes/MaxProbes currently reject. Native non-Linux runtime, broader recovery,
restricted physical durability, independent security/legal and long-soak evidence
also remain open. This HNSW slice is experimental and not production acceptance.
