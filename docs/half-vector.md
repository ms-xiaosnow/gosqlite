# HALFVEC SQL, exact search and explicit precision loss (M9 checkpoint F)

This slice integrates the existing immutable `vector.HalfVector` into typed SQL,
ordinary SQLite-compatible storage, `database/sql`, and exact native/SQL ranking.
It stores actual IEEE-754 binary16 bits with the distinct **GSH v1** codec;
existing dense GSV v1, packed-bit GSB v1 and dense search APIs are unchanged.
Half ANN remains required work; SPARSEVEC SQL/native exact search is covered in [sparse-vector.md](sparse-vector.md).

## SQL type and representation

```sql
CREATE TABLE items(id INTEGER PRIMARY KEY,h HALFVEC(3),active INTEGER);
INSERT INTO items VALUES(1,'[1.0001,-0,5.9604645e-08]',1),
                       (2,halfvec('[2,3,4]'),1),(3,NULL,1);
SELECT id,h <-> '[1,0,0]' AS distance
FROM items WHERE active=1 ORDER BY distance LIMIT 2;
```

`HALFVEC(n)` requires 1..16000 dimensions. Non-NULL INSERT/UPDATE values and
literal defaults are checked for exact dimension and finite binary16 values,
then canonicalized to GSH BLOBs. NULL remains distinct from an invalid zero Go
value or empty vector. Native writes reject numbers, textual `[]byte`, GSV,
GSB, sparse codecs, missing dimensions and oversized values. A failed statement
preserves earlier statements in an explicit transaction; ordinary savepoints,
rollback, close/reopen and shared-WAL snapshots use the existing transaction
machinery.

Text follows the pinned pgvector decimal -> binary32 -> binary16 route. The
second rounding is nearest, ties to even. Signed zero and half subnormals are
preserved. Values such as 65519 round to finite 65504; 65520 rounds to infinity
and is rejected. NaN and infinity are always rejected. Binary16 rounding is
lossy: retaining the original dense vector is an application decision.

Ordinary SQLite accepts `HALFVEC(n)` with NUMERIC affinity. Bracketed text
remains TEXT, so external writers can store valid half text or canonical GSH.
Native reads validate either while preserving its actual storage class. Bound
HALFVEC columns carry representation identity separately from SQLite affinity,
so their numeric distance and transform expressions interpret valid external
TEXT as half values. An external ALTER-added literal default retains ordinary
SQLite storage affinity on old short records. Invalid numeric values reject
when interpreted rather than being guessed or silently repaired. Predicates
that do not reference the half column can filter a row before interpretation.
Ordinary equality/order retain SQLite storage-class/BLOB semantics, not
pgvector vector equality; ordinary index planning excludes typed vector keys
and predicates that could otherwise conceal interpretation failures.

## Functions, operators and conversions

All functions propagate NULL. Half-specific functions accept bracketed TEXT or
strict GSH BLOBs; the codecs are project-specific, not PostgreSQL wire formats.

- `halfvec(value)`: canonical GSH
- `halfvec_dims(value)`, `halfvec_norm(value)`, `halfvec_to_text(value)`
- `l2_distance(a,b)` / `<->`
- `l1_distance(a,b)` / `<+>`
- `negative_inner_product(a,b)` / `<#>`; `inner_product(a,b)` returns positive IP
- `cosine_distance(a,b)` / `<=>`
- `subvector(half,start,count)`: GSH, preserving exact half bits
- `binary_quantize(half)`: GSB, one bit per half component strictly greater than 0

Numeric operators/functions dispatch to matching half operands when a GSH
value or bound HALFVEC expression provides the identity. Text on the other
side is parsed as half. Mixed GSH/GSV and mixed typed VECTOR/HALFVEC columns
reject, even at equal dimensions. Identity follows value-preserving unary +, subvector, coalesce/ifnull/nullif and
result aliases. Mixing known kinds through these forms still rejects. Two
otherwise untyped strings retain the
existing dense behavior; use `halfvec(...)` for explicit half identity. Dense
`vector`, `vector_dims`, `vector_norm` and `vector_to_text` remain dense-only.
No implicit sparse or bit numeric conversion is performed. Hamming/Jaccard
remain bit-only. Operators retain the existing left-associative bitwise
precedence tier, not PostgreSQL's general operator grammar.

`subvector` uses the existing 1-based PostgreSQL substring-like rules: start
and count must be signed int32 SQL INTEGERs, count positive; end is based on
original start+count before clamping. Empty results reject. Both signed zeros
become 0 in binary quantization; tiny positive dense coordinates that rounded
to half zero also become 0. Neither transform promises preserved ranking.

Dense-to-half is explicit:

```go
half, loss, err := vector.HalfFromVector(original)
// loss.Changed, loss.MaxAbsoluteError, loss.MaxRelativeError
```

```sql
SELECT halfvec_from_vector(vector('[1.0001,2]')),
       halfvec_conversion_changed('[1.0001,2]'),
       halfvec_conversion_max_abs('[1.0001,2]'),
       halfvec_conversion_max_rel('[1.0001,2]');
```

`halfvec_from_vector` explicitly authorizes rounding, returns GSH, and errors
on overflow. The three report functions independently compute the same
conversion; `Changed` compares re-widened binary32 bits, absolute error is the
maximum absolute coordinate difference, and relative error divides by nonzero
source magnitude. Exact signed zeros have zero error. SQL does not silently
attach a report to the BLOB; call these functions or use the Go API when a loss
report is needed. `vector_from_halfvec` performs exact widening to GSV.
Legacy real-array migration remains exact dense conversion and never silently
invokes these functions.

Half metric arithmetic widens the rounded coordinates exactly and accumulates
with compensated float64 math. This intentionally differs from upstream
float32/architecture-dependent accumulation. L2 is square-root Euclidean
distance, negative IP retains its sign, cosine is clamped to [0,2], and zero
cosine norm returns `vector.ErrZeroNorm` instead of a NaN result.

## Native exact search and budgets

```go
result, err := db.HalfSearch(ctx, "items", "h", half, sqlite.HalfSearchOptions{
    Metric: vector.L2, Limit: 10,
    Where: "active=?", Args: []any{1},
})
```

DB and Tx expose `HalfSearch`. It requires a physical `HALFVEC(n)` column;
`VectorSearch` remains dense-only. Query values may be half text, GSH,
`HalfVector`, `NullHalfVector`, normal `driver.Valuer` values or NULL. The
metric is a closed `vector.MetricKind`: L2 (default), L1, NegativeIP or Cosine.
Arbitrary callbacks and approximate modes are not supported.

Each operation uses one snapshot; explicit transactions see their own writes.
WHERE runs before candidate decoding and scoring. Native NULL candidates are
excluded, NULL query returns EMPTY, and limit zero validates the non-NULL query
(including cosine norm) without fetching source rows. Results are distance ASC,
signed rowid ASC. Error/cancellation returns the zero result, never partial
exact hits. `HalfPlan` reports `exact_halfvec`, SCAN/EMPTY, dimensions, metric
and exactness; embedded SQL-ranking fields remain unset. `HalfStats` separates
fetched, filtered, NULL and scored rows.

Zero `HalfSearchLimits` selects 100000 source candidates, 10000 results and
2^27 components; partial/invalid settings reject. Every fetched source row
consumes a candidate, including filtered/NULL rows. Component work charges the
query dimension once and every scored candidate dimension once. It does not
count arbitrary WHERE/projection work or every machine memory read. Quotas fail
rather than truncate exact results. `MaxSortBytes` conservatively reserves
query/candidate decoding storage, result heap and retained row identities.
Known typed query values are checked before encoding copies; input size and
known dimensions are checked before component allocation. Caller-owned input,
custom Valuer callback allocations and general SQL work are outside these
logical search budgets; they are not total RSS or wall-clock guarantees.

## Bounded SQL ranking and ANN boundaries

The narrow heap optimizer recognizes one ASC numeric metric over a declared
HALFVEC column and a literal/bind query with bounded LIMIT. Both operand
orders, aliases/ordinals, LIMIT and OFFSET use exact binary16 scoring; retain
count is LIMIT+OFFSET. WHERE runs first. SQL preserves ordinary NULL-first
ascending order, unlike native NULL exclusion. Unsupported shapes (DESC,
secondary keys, transformed query expressions or unlimited sorting) use the
existing MaxSortBytes-bounded general sorter.

`QueryPlan.VectorRepresentation="halfvec"`, `VectorMetric`, dimensions and
`VectorExact=true` identify the stage; `VectorRanking="bounded_heap"` is never
an index claim. `VectorRankComponents` charges query and scored candidate
dimensions; packed-bit and ANN counters stay zero. SQL decoding remains lazy:
EXPLAIN and LIMIT 0 do not decode the query, and an all-NULL/filtered eligible
set need not decode it. Default exact-ranking work limits apply to the
recognized heap, not every scalar expression.

Plans explicitly report `HALFVEC(n) ANN is unsupported; exact binary16 ranking`.
Dense HNSW/IVFFlat creation rejects half columns. No half ANN index, opclass,
4000-dimension indexing capability or approximate recall claim is made here;
type-aware ANN metadata and indexing remain a separate checkpoint.

## Driver, example and verification scope

`HalfVector` is immutable and implements `driver.Valuer`. `NullHalfVector`
implements nullable Valuer/Scanner, preserves both fields on a failed scan,
and does not retain borrowed buffers. The driver uses these existing standard
interfaces; no half bytes are reinterpreted as dense values.

The [runnable half-rerank example](../examples/half-rerank/README.md) obtains
exact half L2 candidates, reranks original dense values in the same transaction,
and reports conversion loss and observed recall. Its deliberately truncated
candidate set loses the true dense neighbor; expanding it recovers that one
fixture, never a general recall guarantee. Materialized transformed columns
must be explicitly maintained by applications.

Retained public pgvector v0.8.7 fixtures are pinned at
`f37c13f68b57d2c3472b2214fbcff699d6d34876`, with hashes and license under
[testdata/pgvector-v0.8.7/extended](../testdata/pgvector-v0.8.7/extended/manifest.json).
Tests compare published expected outputs and independently computed arithmetic;
no live PostgreSQL execution or general PostgreSQL SQL compatibility is claimed.
The mathematical primitive tests also exhaustively round-trip all finite half
patterns. SQL tests use intact ordinary files and memory-only malformed values.
This slice makes no crash, fault, namespace replacement, tamper, wrong-key,
interrupted publication/rekey or recovery qualification claim.

## Verification records

The [component gate](evidence/m9-half-sql-components.txt) records the mandatory
source guard; primitive/published fixture tests; three focused CGO0 runs;
preserved dense/BIT/driver regressions; focused CGO1 race; vet; and production
build. The [independent review](evidence/m9-half-sql-review.txt) records
additional driver, exact-ranking and ordinary official-SQLite interop checks.
The [example output](evidence/m9-half-sql-example.json) exposes measured recall
and conversion loss, with [source hashes](evidence/m9-half-sql-source-sha256.txt)
identifying this slice. Whole-tree aggregates and release gates remain
separately owned; these records do not qualify restricted physical workflows.
