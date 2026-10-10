# Persistent typed ANN (M9 checkpoint H)

The typed value storage described in [half-vector](half-vector.md),
[bit-vector](bit-vector.md), and [sparse-vector](sparse-vector.md) now participates
in genuine persisted HNSW and supported IVFFlat searches. The implementation is
native Go. It does not execute PostgreSQL or delegate SQL to another engine.

## Supported matrix and explicit source identity

The profile remains pinned to pgvector v0.8.7 commit
`f37c13f68b57d2c3472b2214fbcff699d6d34876`. These are local algorithms and codecs,
not binary index, network, PostgreSQL planner, or full SQL compatibility.

| Source | Indexed bounds | HNSW operator classes | IVFFlat operator classes |
|---|---|---|---|
| VECTOR | 1..2000 dimensions | vector_l2/ip/cosine/l1_ops | vector_l2/ip/cosine_ops |
| HALFVEC | 1..4000 dimensions | halfvec_l2/ip/cosine/l1_ops | halfvec_l2/ip/cosine_ops |
| BIT | 1..64000 bits | bit_hamming_ops, bit_jaccard_ops | bit_hamming_ops |
| SPARSEVEC | 1..1e9 dimensions, at most 1000 nonzeros per indexed/query value | sparsevec_l2/ip/cosine/l1_ops | unsupported |

The compact numeric names above expand independently, e.g. `halfvec_l2_ops`,
`halfvec_ip_ops`, `halfvec_cosine_ops`, `halfvec_l1_ops`. IVFFlat L1, bit Jaccard,
and every sparse IVFFlat combination reject. Exact type limits remain separate:
HALFVEC may store up to 16000 components, SPARSEVEC up to 16000 nonzeros. An ANN
index imposes its narrower limit on existing values at build and future writes.
Sparse dimensions are metadata. No graph, query, training, codec or memory path
allocates by sparse dimension or visits implicit zeros.

A representation/opclass/dimension/metric/source mismatch never reuses an
index. Source values preserve their exact GSV/GSH/GSB/GSS identities. HNSW stores
original typed values and verifies same-snapshot candidate bytes against source
rows before exact rescoring. IVF persists centroids and source-row memberships;
search opens only selected list ranges and verifies current membership and
source identities. Exact rescoring does not make approximate recall exact.

## SQL and native access

```sql
CREATE TABLE items(id INTEGER PRIMARY KEY,h HALFVEC(3) NOT NULL);
INSERT INTO items VALUES(1,'[1,2,3]'),(2,'[2,3,4]');
CREATE INDEX half_hnsw ON items(h) USING hnsw
  WITH(opclass='halfvec_l2_ops',m=16,ef_construction=64);
SELECT id,h <-> '[1,2,3]' AS distance
FROM items ORDER BY distance LIMIT 10;
SELECT id FROM items NOT INDEXED ORDER BY h <-> '[1,2,3]' LIMIT 10;
```

This is the project's CREATE INDEX grammar. The existing eligible one-distance
ASC/LIMIT SQL path automatically selects a matching typed index. Bound queries,
both operand orders, aliases, ordinal ORDER BY, and bounded OFFSET work. Nullable
columns retain exact/general SQL ordering to preserve NULL-first semantics.
`NOT INDEXED`, unsupported sort shapes, and no matching index retain exact paths.
The first matching usable index wins; there is no new cost model or named SQL
ANN hint. An earlier untrained index can mask a later ready index, as in dense
ANN. Native `Index` selects the intended index explicitly.

`CreateHNSWIndex` and `CreateIVFFlatIndex` derive representation and dimensions
from the physical column. Existing numeric `Metric` options remain supported.
A matching bit operator class selects Hamming/Jaccard for HNSW and Hamming for
IVF; an explicitly converted MetricKind must agree. These constructors never silently convert
an existing column to another vector kind.

```go
err := db.CreateHNSWIndex(ctx, "bit_knn", "items", "bits",
    sqlite.HNSWIndexOptions{Opclass: "bit_hamming_ops"})
result, err := db.BitSearch(ctx, "items", "bits", query,
    sqlite.BitSearchOptions{
        Mode: sqlite.VectorModeApproximate, Index: "bit_knn",
        Metric: vector.Hamming, Limit: 10, EFSearch: 40, MaxEFSearch: 1000,
        Where: "active=1",
    })
```

`HalfSearch`, `BitSearch`, and `SparseSearch`, on DB and Tx, still default to
exact. Their options add `Mode`, `Index`, `EFSearch`, `MaxEFSearch`, `Probes`,
`MaxProbes`, and `ANNLimits`. Dense `VectorSearch` remains dense-only. Explicit
probes on a selected HNSW index reject. Unknown/dirty/mismatched named indexes
use an explained exact fallback. Query/index dimension and sparse-nnz validation
still applies. Limit zero and NULL queries retain EMPTY behavior and do not
select an index; global option checks still run.

Native NULL candidates are excluded. Numeric cosine indexes reject non-NULL
zero-norm values, including writes, and zero-norm queries error; they do not
silently omit them. This differs from pgvector's zero-vector exclusion. IP zero
values remain indexable; IVF skips them only as spherical training directions.
Bit zeros remain indexed. Jaccard all-zero/all-zero retains the existing local
primitive result of 1. Sparse zero support remains valid for L2/L1/IP.

## Storage versions, training, and lifecycle

Dense indexes keep their exact canonical version-1 JSON, GSV payloads, GHA v1
adjacency, method version, SQL object names, and ordinary table/B-tree schemas.
Typed indexes use method version 2 and a strict canonical envelope recording
`value_kind`, `value_codec`, transform policy, sparse support limit, and centroid
policy. Dense v2 and typed v1 are rejected rather than interpreted ambiguously.
Typed node/centroid values use their existing distinct versioned GSH/GSB/GSS
codecs. Unknown/duplicate/noncanonical configurations fail closed. No old index
is automatically rewritten or migrated. `REINDEX` preserves construction policy
and advances generation.

Typed HNSW uses the same deterministic multilayer insertion, greedy traversal,
neighbor pruning, and filter compensation as dense HNSW. Original typed node
values remain traversable in tombstones until rebuild. Actual graph and edge
reads are observable. It never labels a source scan as graph traversal.

Half IVF widens source halves exactly for compensated float64 Lloyd/spherical
training. Derived centroids follow an explicit float64 → float32 → binary16
round-to-nearest-even policy and persist as GSH. Spherical centroid norm tolerance
is 1e-3 after half rounding; dense's 1e-6 tolerance is unchanged. Original source
halves are untouched. This is not a claim of identical upstream centroids.

Bit IVF performs real farthest-first Hamming seeding, list assignments, and
Lloyd updates with per-bit majority centers. Tied bits retain the previous
center's bit; empty clusters retain their previous center. It uses packed bit
samples/centroids and bounded temporary bit counters, not a dense numeric metric
substitute. List IDs break assignment/probe ties. Existing bounded seeded
reservoir, training work, iteration, and untrained-state policies remain active.
An empty/undersampled index needs explicit REINDEX after sufficient input arrives.

Source writes, graph/list changes, counts, and metadata share statement undo,
savepoints and transaction commits. Tests cover normal update, delete, rowid
changes, rollback, rebuild, shared-WAL read snapshots, writer BUSY, and ordinary
close/reopen. They do not qualify physical crash recovery or interrupted
publication. Existing exact source guards invalidate external ordinary writes.

## Work and memory accounting

`Rows.ANNWork()` and native typed `Stats.ANNWork` count actual graph/centroid metric support: paired numeric
components, paired packed-byte positions, or the sum of both sparse nonzero
supports. It supplements the existing real node/edge/list/member/distance
counters. `HalfStats.ScoredComponents`, `BitStats.ScoredPackedBytes`, and
`SparseStats.ScoredNonzeros` remain exact-source scoring work, including the
query's initial validation charge. The corresponding SQL counters remain
representation-specific. Widening, abandoned candidates and exact fallback
retain cumulative work already spent.

The existing typed work limits bound both ANN and exact rescoring. ANN limits
also bound reads, distance calls, iterations, and retained algorithm memory.
Query/candidate backing, graph/list bookkeeping, source decoding/identity
scratch, and retained results share conservative logical memory admission.
Sparse budgets use support size, never declared dimensions. Budget exhaustion
and cancellation return an error, never successful partial native results;
SQL callers must check Rows.Err. These are logical quotas, not total allocation,
RSS, arbitrary WHERE/projection CPU, pager I/O, or wall-clock guarantees.

Typed algorithm structures are separate from the preserved legacy dense
Config/Node/Sample/Centroid/Limits/Stats. Their larger fixed overhead is included
in conservative admission. Dense public struct layouts, unkeyed literals and
function signatures remain unchanged. SQLite uses typed adapters for both
version families, so conservative build/query admission may reject a formerly
borderline memory budget; on-disk and source-value identities remain unchanged.

## Verification and honest quality scope

The fixed persistent 96-row matrix exercises all ten typed HNSW combinations
with one independent-math query per combination, ordinary close/reopen, source
byte checks, real plans and graph counters. Recall@10 was 1 for those small
fixtures, with 40 source fetches. A separate 128-row persistent IVF fixture at
one initial probe measured half L2 .90, IP .80, cosine .60, and bit Hamming .30;
IP/cosine compensated to two of four lists. These results do not establish a
universal 95% target or an ANN latency advantage.

Separate seeded pure-memory held-out algorithm tests cover 384 rows/16 queries
for HNSW and 512 rows/16 queries for IVF. Their larger ef/probe settings visit
much of the corpus; results and scope are recorded in the algorithm evidence.
See [component evidence](evidence/m9-typed-ann-components.txt) and
[source identities](evidence/m9-typed-ann-source-sha256.txt). Full combined-tree
checks, restricted physical durability, native-platform qualification,
independent security/legal review, and M10 remain separate gates.

The [runnable typed ANN rerank example](../examples/typed-ann-rerank/README.md)
uses real half/bit HNSW candidates and original dense L2 reranking in one read
snapshot. It shows one-candidate loss and the effect of widening to two on a
small fixture. Materialized transforms require explicit application maintenance;
full generated/expression ANN indexes are not implemented.
