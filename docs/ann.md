# Persistent approximate nearest neighbors

## Implemented scope and recall contract

M7 implements **HNSW and IVFFlat** over legal SQLite tables and B-trees, using
independently written Go algorithms. HNSW performs real multilayer graph
traversal. IVFFlat trains centroids, persists source-row membership in lists,
then reads only selected list ranges. Neither path substitutes an exact scan
while claiming ANN access. Opening a database does not preload a graph, centroid
set or membership corpus.

Every returned candidate is checked against its current snapshot-bound identity
and source row, then scored with the exact metric on the original source vector.
Exact candidate distances do **not** imply exact recall. `Plan.Exact` and SQL
`VectorExact` remain false for a successful ANN result. A fallback discards the
approximate ranking, scans the source and reports `SCAN`, `exact_vector`, an
exact flag and the fallback reason. Normal lifecycle tests and benchmark reopen
checks do not qualify physical crash/recovery durability or production readiness.

## Choosing exact or approximate search

Native `VectorSearch` remains exact by default, including after an ANN index is
created. To opt in, set `Mode: sqlite.VectorModeApproximate`. A named `Index`
selects the intended matching index; an absent, dirty or mismatched index produces
an explained exact fallback rather than using an incompatible metric/source.

```go
err := db.CreateIVFFlatIndex(ctx, "items_ivf", "items", "embedding",
    sqlite.IVFFlatIndexOptions{Lists: 100})
result, err := db.VectorSearch(ctx, "items", "embedding", "[1,2,3]",
    sqlite.VectorSearchOptions{
        Mode: sqlite.VectorModeApproximate, Index: "items_ivf",
        Limit: 10, Probes: 10, MaxProbes: 100,
        Where: "active=1",
    })
```

Create IVF after loading representative data, or explicitly `REINDEX` after
loading. An index created with too few trainable samples remains untrained even
when later INSERTs provide enough data. The example's 100-list configuration
requires at least 100 trainable samples and enough resource budget to build.

HNSW uses the same native opt-in with method-specific query controls:

```go
err := db.CreateHNSWIndex(ctx, "items_hnsw", "items", "embedding")
result, err := db.VectorSearch(ctx, "items", "embedding", "[1,2,3]",
    sqlite.VectorSearchOptions{
        Mode: sqlite.VectorModeApproximate, Index: "items_hnsw",
        Limit: 10, EFSearch: 40, MaxEFSearch: 1000,
        Where: "active=1",
    })
```

### Automatic SQL selection and frozen selection limitations

A matching SQL `ORDER BY` distance ASC with a finite, bounded LIMIT can
**automatically choose ANN** after the index is installed. Use `NOT INDEXED` for
the exact SQL reference or use native exact mode. Eligible shapes have one
built-in distance key over a declared `VECTOR(n) NOT NULL` column and a literal
or bound query value. Aliases, ordinal ORDER BY references, built-in distance
functions and bounded OFFSET work; LIMIT plus OFFSET is the retained candidate
count. DESC, secondary sort keys, unlimited sorts, arbitrary query expressions,
NULL queries and nullable vector columns retain the existing exact/general path.
The NOT NULL requirement preserves SQL's NULL-first ascending ordering; native
search supports nullable vector columns and excludes NULL candidates.

```sql
CREATE INDEX items_ivf ON items(embedding) USING ivfflat
  WITH(opclass='vector_l2_ops',lists=100,training_samples=10000,
       training_iterations=20);
SELECT id,embedding <-> '[1,2,3]' AS distance
  FROM items WHERE active=1 ORDER BY distance LIMIT 10;
SELECT id,embedding <-> '[1,2,3]' AS distance
  FROM items NOT INDEXED WHERE active=1 ORDER BY distance LIMIT 10;
REINDEX items_ivf;
```

This is project SQL grammar, not PostgreSQL's CREATE INDEX grammar. Native probe
and ef controls are not SQL SET/PRAGMA variables. SQL uses IVF's default one
initial probe and a compensation ceiling equal to the list count.

Selection is first matching usable catalog index in creation order, with no
recall/cost comparison or retry of later matches after selection. An earlier
untrained IVF index, or a one-list IVF index whose default probe requests all
lists, can therefore cause an exact fallback and mask a later trained index.
Native `Index: "later_index"` avoids this ambiguity. SQL currently has no named
ANN-index hint; `NOT INDEXED` explicitly chooses the exact reference.

Native `Limit: 0` and NULL-query searches return `EMPTY` before selecting an
index. They still validate global options, including probe bounds 0..32768 and
nonzero `MaxProbes >= Probes`; zero means default. A non-NULL zero-limit query
also validates its vector and metric. These EMPTY paths skip index-dependent
validation: they do not establish that the named index exists, uses IVF, or has
enough lists for the supplied probe count. Nonempty approximate searches validate
probe compatibility once an index is selected. Native exact mode does not select
an ANN index either. Explicit probes on a selected HNSW index reject.

## Metrics and creation controls

Both methods index one typed vector field with dimensions 1..2000. A
metric/opclass/dimension/source mismatch never reuses an index. Unknown versions,
noncanonical configuration or inconsistent owned storage fail closed.

| Metric | SQL distance | Operator class | HNSW | IVFFlat |
|---|---|---|---|---|
| L2 | `<->` | vector_l2_ops | yes | yes |
| L1 | `<+>` | vector_l1_ops | yes | rejected |
| negative inner product | `<#>` | vector_ip_ops | yes | yes |
| cosine distance | `<=>` | vector_cosine_ops | yes | yes |

Cosine indexes reject every non-NULL zero-norm source value, including subsequent
writes. They do not silently omit zeros. This preserves the exact layer's error
contract, and differs from pgvector's zero-vector exclusion. Original vector bits
and magnitudes remain authoritative for final scoring, especially inner product.

Native zero-valued creation fields inherit fixed defaults. SQL explicitly
supplied zero limits reject; seed 0 selects the fixed default. Construction
configuration is immutable: `REINDEX` uses the stored configuration and advances
generation; changing parameters requires another named index. DROP is not
implemented in this slice.

### IVFFlat deterministic bounded training

`IVFFlatIndexOptions` exposes `Metric`, `Opclass`, `Lists`, `TrainingSamples`,
`TrainingIterations`, `Seed`, `MaxNodes`, `MaxTrainingWork`, `MaxVisited`,
`MaxDistanceEvaluations` and `MaxMemoryBytes`.

Defaults are 100 lists, at most 10000 retained training samples, 20 Lloyd
iterations, seed 0x243f6a8885a308d3, 100000 eligible source rows, 2^28 training
component-work units, 32768 centroid visits, 1000000 assignment/probe centroid
scores per operation and 32 MiB logical retained algorithm memory. If more than
10000 lists are requested with no sample cap specified, the default sample cap
rises to the list count. Lists permit 1..32768, training samples must be at least
lists and at most 2^20, iterations permit 1..1000, and training work 1..2^40.
Independent quotas can reject otherwise valid large configurations. These are
local resource policies, not pgvector session settings or unlimited allowances.

Creation and rebuild use a bounded streaming bottom-hash (minhash-style)
reservoir: SplitMix64-derived seed/rowid priorities choose the smallest
(priority, signed rowid) pairs. Every indexed value is validated, but only the
bounded sample is retained; its capacity is not allocated up front. Selected
samples are processed in signed-rowid order. The first seed has the smallest
hash/rowid, then farthest-first initialization repeatedly selects the remaining
sample farthest from its nearest seed, with signed-rowid ties. This is not
k-means++ and does not copy upstream C training code.

L2 training uses squared Euclidean assignment and ordinary arithmetic means.
Negative IP and cosine train with nonzero sample directions normalized in
float64, then normalize their arithmetic mean directions: spherical k-means.
Reductions use compensated float64 sums in fixed rowid order. Genuine Lloyd
iterations reassign samples to their nearest centers and update means, stopping
when assignments stabilize after a mean update or the persisted iteration cap
is reached. Output centroids are float32 GSV vectors with list IDs 1..lists;
centroid ties choose the lowest list ID. Spherical norm validation allows 1e-6
serialization error.

Empty clusters retain their previous centroid. Exactly canceling spherical sums
also retain their previous unit centroid. Duplicate/empty lists from repeated
input directions are valid; no random or zero replacement is manufactured.
IP zero vectors remain valid indexed values, are excluded only from training,
and join list 1 once trained because their centroid scores tie. Cosine zeros
reject. Fewer trainable samples than lists, including empty input, produce a
persisted `untrained` index with no centroids or memberships. Later writes update
counts but do not implicitly train it. Search explains the exact fallback and
`REINDEX` requirement; a rebuild with insufficient samples stays untrained.

Training bounds include retained original samples, float64 training copies,
seeding/assignment arrays, compensated sums, centroid/output arrays and
conversion allowances. Component work charges dimensional normalization,
distance, reduction and conversion stages, plus final source-to-list assignment
at the storage layer. These are conservative logical memory/work accounts,
not Go allocator/RSS measurements; pager/storage costs are additional. See the
[algorithm contract](../extension/vector/ann/README.md) for precise scope.

### HNSW construction

`HNSWIndexOptions` configures `Metric`, `Opclass`, `M`, `EFConstruction`, `Seed`,
`MaxNodes`, `MaxTombstones`, `MaxVisited`, `MaxDistanceEvaluations` and
`MaxMemoryBytes`. Defaults are m=16, ef_construction=64, the same fixed seed,
100000 nodes including tombstones, 25000 tombstones, 4096 visited nodes per
operation, 1000000 metric evaluations per operation and 32 MiB logical memory.
M permits 2..100; ef_construction permits 4..1000 and must be at least 2M.

HNSW has deterministic geometric levels up to 32, greedy upper-layer descent,
best-first layer search, diversity selection and bounded reciprocal pruning.
Layer 0 has at most 2M neighbors; upper layers at most M. Signed rowid breaks
score ties. Local topology and compensated float64 arithmetic need not match
pgvector bit-for-bit. No PostgreSQL binary index compatibility is claimed.

## Transactional lifecycle

For trained IVF, INSERT assigns a row to the nearest fixed stored centroid;
DELETE removes its membership; vector or rowid UPDATE removes the old membership
and adds the new one. NULL values have no membership. Byte-identical vector and
rowid updates preserve membership. Per-list counts and live/source counts change
in the same transaction as source rows. Ordinary DML never moves centroids.
Explicit `REINDEX` retrains and atomically replaces centroids/memberships.

For HNSW, INSERT creates a new immutable node identity; DELETE tombstones it.
Vector/rowid replacement tombstones the old identity and creates another, while
byte-identical vector and rowid updates preserve identity. Tombstones retain
original vectors and edges for traversal. Reindex rebuilds and compacts them.

Source rows, owned shadow tables/indices, counts and catalog metadata share
statement rollback and transaction boundaries for both methods. Exact generated
source guards mark indexes dirty when ordinary official SQLite writes source
rows; source SQL hashes/schema cookies check schema epochs. Dirty indexes scan,
and native source mutations require REINDEX. Arbitrary trigger execution is not
implemented. Ordinary rollback, close/reopen, snapshot and checkpoint evidence
is distinct from unqualified interrupted-publication/physical recovery behavior.

## Probing, filters and cumulative limits

IVF defaults to `Probes: 1` and `MaxProbes: lists`. Nonzero values must satisfy
`1 <= Probes <= MaxProbes <= lists`; they are never silently clamped. Search reads
this index's bounded centroid range from the actual B-trees, validates it and
scores every centroid once. It then opens only the selected nearest-centroid
membership ranges, validates each member table/reverse identity and seeks the
current source row. Original WHERE is applied before source-vector decoding and
exact rescoring. Final ties use signed source rowid.

If too few eligible results survive, compensation doubles the probed prefix up
to `MaxProbes`, opening each additional list at most once. Centroids are not
rescored and old lists are not reread. Compensation satisfies the requested
eligible count; it is not a recall target or a promise of 95% recall. If the cap
or all-list exhaustion still leaves too few eligible results, search discards
the approximate heap and performs an explained exact scan. If widening reaches
all lists and satisfies the count, it can finish as ANN and remains marked
approximate. A direct `Probes == lists` request instead selects an exact `SCAN`
with reason `probes equals lists`, before centroid/list reads. A one-list index
therefore uses exact scanning with the default probe count.

HNSW instead widens ef, retaining graph routing nodes regardless of the filter,
deduplicating immutable candidate identities and rechecking WHERE. Defaults are
`EFSearch: 40`, `MaxEFSearch: 1000`. Insufficient eligible results at the maximum
cause an explained exact scan. A short graph frontier is not proof of exact
coverage.

`ANNWorkLimits` bounds cumulative logical query work: defaults are 100000 node
reads, 32768 centroid records, 2^20 member keys, 1000000 combined graph/centroid
metric evaluations, 16 widening iterations and 32 MiB algorithm memory.
Persisted per-operation limits remain additional bounds. `vector.SearchLimits`
charges all fetched source rows, including filtered/NULL rows and repeated reads
in an exact fallback, plus query validation, graph/centroid distance components
and exact source rescoring. `DB.MaxSortBytes` also bounds retained source/result
bookkeeping. Widening and abandoned attempts retain their spent work; fallback
is not a fresh quota. Budget exhaustion or cancellation returns an error, never
a successful partial native result. Streaming SQL callers must check `Rows.Err`.

## Storage and diagnostics

The original seven grams extension objects are unchanged. An optional complete
ANN-v1 family adds five ordinary tables (state, nodes, live mapping, centroids,
members) and six lookup indices. HNSW owns nodes/live records and keeps its
centroid/member rows empty; IVF owns centroids/members and has no graph nodes,
live mappings or tombstones. Old grams-only databases add the family on first
ANN creation. IVF's separate canonical JSON envelope preserves all previously
persisted HNSW-v1 canonical bytes/hashes. See [file formats](file-formats.md).

IVF plans expose list count, requested/maximum/effective probes, training state,
generation, stop reason and exact-fallback reason. HNSW exposes requested and
effective ef. Query statistics distinguish:

- `ANNStateReads`: state metadata reads.
- `ANNCentroidReads`/`ANNCentroidScores`: decoded centroid rows and metric calls.
- `ANNListsOpened`/`ANNMemberKeysRead`: opened covering ranges and returned keys.
- `ANNIdentityLookups`: membership-table/reverse-key rechecks, or HNSW live lookups.
- `BaseRowFetches`/`ANNExactRescores`: actual source fetches and ANN candidate scores.
- `ANNNodeReads`/`ANNEdgeReads`/`ANNDistanceEvaluations`: graph work, zero for IVF.
- `ANNVisited`, `ANNIterations`, `ANNPeakBytes`: method-specific visits, widening
  rounds and conservative retained-memory accounting.

These are actual logical record/key/probe operations, not physical page I/O,
total shadow I/O or process RSS. IVF identity checks do additional table/reverse
lookups not included in `ANNMemberKeysRead`. EXPLAIN may read IVF training-state
metadata to identify an untrained/all-list exact plan, but reads no centroids,
members, graph nodes or source vectors and performs no metric scoring.
`CheckExtensionIndex` explicitly scans/reconciles full source/shadow state,
including IVF nearest-centroid assignments and per-list counts. Normal open and
search do not perform that full check; it is not general SQLite integrity_check.

## Runnable example and benchmark evidence

Run `CGO_ENABLED=0 GOTOOLCHAIN=local go run ./examples/ivfflat` with Go 1.27.2.
The [example](../examples/ivfflat/README.md) uses its own temporary database and
ordinary native INSERT/UPDATE/DELETE, untrained fallback, REINDEX, named search,
filter compensation, exact all-probe scan, SQL selection and close/reopen. Its
JSON counters are from actual execution; the easy clustered data is not a recall
benchmark. `examples/ann` remains the separate HNSW example.

IVF measurements use the same 2048 x 12 independent standard-normal float32
vectors and 48 held-out queries as the HNSW workload, seed 20261010. All three
runs record dataset/query GSV SHA256
`81aec74dd6099591042ec104f4453cd0bda864aa57a06e5077c2142bedd280a0`, 100 lists,
20 iterations, a 10000-sample cap, maximum 100 probes and ordinary close/reopen
before measurement. Exact and approximate timings share one read transaction
per query, with warm OS cache, unmeasured warmup and alternating timing order.

| Initial probes | Recall@10 | Mean source fetches | Mean centroid scores | IVF p50 ms | Exact p50 ms |
|---|---|---|---|---|---|
| 1 | 0.25417 | 23.92 | 100 | 6.42 | 3.06 |
| 10 | 0.77083 | 239.35 | 100 | 41.12 | 3.24 |
| 50 | 0.99583 | 1149.63 | 100 | 142.09 | 2.42 |

Exact search fetched 2048 source rows. No measured query fell back to exact.
At K=10, the initial one-probe run occasionally compensated to two probes;
mean lists opened was 1.02083. Ten- and fifty-probe runs stayed at their requested
counts. Build times were approximately 1.66, 1.53 and 1.23 seconds; all three
files grew from 286720 to 503808 bytes. The raw evidence retains full K=1/10/100,
latency, work and environment details:

- [Initial probes 1](evidence/m7-ivf-benchmark-probes1.json)
- [Initial probes 10](evidence/m7-ivf-benchmark-probes10.json)
- [Initial probes 50](evidence/m7-ivf-benchmark-probes50.json)

The probes-1 JSON's note saying "persistent B-tree graph" is a legacy descriptive
mislabel: this run measured IVFFlat centroid/list B-trees, and its graph counters
are zero. The original note and numerical measurements are retained unchanged;
it is not evidence of graph traversal in IVF.

**All three IVF settings were slower than exact on this dataset.** The default
one-probe result does not establish a 95% recall target; even ten probes had
only 77.08% recall@10. Fifty probes reached 99.58% on this workload at much higher
latency. These shared-cloud observations are not speedup claims, confidence
intervals or universal recall guarantees. Reduced source/distance work alone
must not be presented as reduced latency. Reproduce the workload with the
commands in [benchmarks/ann](../benchmarks/ann/README.md).

The preserved [HNSW baseline](evidence/m7-hnsw-persistent-benchmark.json) measured
recall@1=1, @10=1, @100=0.999375 without fallback; K=10 mean source fetches were
40 and graph distances 575.08, versus 2048 exact rows. HNSW p50 40.62 ms was also
slower than exact 2.12 ms. Build took 49.38 seconds and grew the file from 286720
to 1462272 bytes. It is a separate descriptive run, not a controlled latency
comparison between methods.

IVFFlat algorithm/storage/query support now exists; tuning, broader platforms,
restricted physical durability/recovery, independent security/legal review and
long-soak evidence remain open. The ANN slice remains experimental.
