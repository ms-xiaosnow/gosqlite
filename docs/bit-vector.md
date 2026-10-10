# BIT SQL, exact search and dense transforms (M9 checkpoint E)

This checkpoint integrates the already-tested packed-bit value into real SQL,
ordinary SQLite-compatible storage, `database/sql`, and exact ranking. It also
adds dense `subvector` and `binary_quantize` SQL functions. Existing dense
`VECTOR(n)`, GSV v1 bytes, numeric metrics and `VectorSearch` APIs are unchanged.

HALFVEC SQL and native typed search are now documented in half-vector.md.
SPARSEVEC SQL remains a subsequent checkpoint; its memory value/codec alone
does not imply database integration. Bit, half and sparse ANN remain unsupported. No PostgreSQL runtime,
wire format, cast, operator-class or general SQL compatibility is claimed.

## Typed SQL and strict representation identity

```sql
CREATE TABLE items(id INTEGER PRIMARY KEY, bits BIT(9), active INTEGER);
INSERT INTO items VALUES(1,'001010001',1),(2,bit_vector('111000111'),1),(3,NULL,1);
SELECT id,bits <~> '001010001' AS distance
FROM items WHERE active=1
ORDER BY distance LIMIT 2;
```

`BIT(n)` requires 1..64000 bits. INSERT/UPDATE and literal defaults validate exact
width and canonicalize non-NULL values to GSB v1 BLOBs. No implicit padding,
truncation, scalar-number interpretation, dense conversion or padding repair is
performed. Statement failure preserves earlier statements in an explicit
transaction. NULL/NOT NULL follow the ordinary supported SQL rules.

- `bit_vector(value)` returns canonical GSB v1 BLOB
- `bit_dims(value)` returns bit width
- `bit_to_text(value)` returns exactly ASCII 0/1
- `hamming_distance(a,b)` and `a <~> b` count XOR bits
- `jaccard_distance(a,b)` and `a <%> b` return 1 - intersection/union; zero
  intersection returns 1, including all-zero/all-zero and empty/empty

All functions propagate NULL. Inputs are exact bit text strings or strict GSB
BLOBs, never guessed textual bytes. Scalar functions accept native zero-width
values (`bit_vector('')`); declared `BIT(0)` and storing a zero-width value in a
positive-width column reject. `B'...'`, varbit, width-coercing casts and bit
string arithmetic are not implemented.

Representation-specific signatures are closed. Dense `<+>` remains Manhattan;
`<~>` is bit-only Hamming. Dense or half/sparse BLOBs fail bit decoding, and bit
BLOBs fail dense decoding even when dimensions coincide. Existing integer
`~`, `&`, `|`, `<<`, `>>` remain unchanged. Distance operators use the project's
existing left-associative bitwise precedence tier. This is not PostgreSQL's
operator grammar; parenthesize mixed expressions.

## Ordinary SQLite affinity and interoperation

Official SQLite accepts `BIT(n)` as legal schema SQL with NUMERIC affinity. It
can turn `'001'` into INTEGER 1, irreversibly losing width. Cross-engine writers
must bind a canonical GSB BLOB to preserve the value. Native reads reject
external numeric values rather than guess leading zeros or silently repair them.
The read validator accepts genuine bit TEXT when present, while preserving its
actual storage class; native rewrites canonicalize it. Missing-column defaults
after an external ALTER retain ordinary SQLite affinity too: a virtual default
'001' becomes INTEGER 1 and fails typed interpretation, while a canonical GSB
BLOB default remains usable.

Ordinary equality and non-distance ordering retain SQLite storage-class/BLOB
semantics, not pgvector equality. Ordinary index planning excludes typed bit
keys/predicates where interpretation errors could otherwise be hidden. Predicates
that do not reference an invalid bit value can exclude its row before decoding.

The oracle tests create intact databases with native and official SQLite SQL,
compare canonical BLOBs, exercise ordinary updates/rollback/reopen/checkpoint,
and run official `integrity_check`, at page sizes 512/4096/65536 with rollback
and shared WAL. The numeric-affinity rejection is a normal SQL compatibility
case in an intact database, not damaged-file or recovery evidence.

## Native exact search

```go
bits, err := vector.ParseBit("001010001")
if err != nil { return err }
result, err := db.BitSearch(ctx, "items", "bits", bits, sqlite.BitSearchOptions{
    Metric: vector.Hamming, Limit: 10,
    Where: "active=?", Args: []any{1},
})
```

Both DB and Tx expose `BitSearch`. The query accepts normal driver values,
including `BitVector`/`NullBitVector`, bit text, canonical BLOB and NULL. A
physical `BIT(n)` column is required. `VectorSearch` remains dense-only. Empty
metric selects Hamming; only closed Hamming/Jaccard identities are valid.
There is no bit approximate mode or arbitrary metric callback.

One snapshot is used for the whole operation; transactions see their own writes.
The source is a full scan with WHERE evaluated before bit decoding/scoring.
Native NULL candidates are excluded, and NULL query returns EMPTY. Limit zero
validates options, filter and query but reads no source rows. Non-NULL invalid
queries still fail with zero limit. Results are distance ASC, signed rowid ASC.
Errors and cancellation return zero results, never partially exact hits.

`BitPlan` reports representation/metric/dimensions, exactness and SCAN/EMPTY.
`BitStats` separates fetched, filtered, NULL and scored rows. ScoredPackedBytes
charges the query's ceil(n/8) bytes once and that width once per scored candidate,
representing paired packed-byte positions visited. It does not count every
machine memory read, conversion pass, WHERE expression, scalar projection or
B-tree page. Native plan fields embedded from the generic SQL rank diagnostics
remain unset; use `BitPlan.Metric`, `Dimensions` and `Exact`.

Zero `BitSearchLimits` uses 100000 source candidates, 10000 results, and 2^27
packed bytes. Every visited row consumes a candidate, even if filtered or NULL.
Partial/invalid limits reject. `MaxSortBytes` separately bounds conservative
retained query/candidate/result/identity storage. Limits return errors rather
than truncating an exact answer. Input/callback memory and general SQL work are
outside these ranking budgets; these are logical limits, not RSS/time promises.

## SQL bounded ranking and ANN boundaries

The existing narrow heap optimization now also recognizes bit distances:
one ASC Hamming/Jaccard key over a declared BIT column and a literal/bind query,
plus bounded LIMIT. Both operand orders, aliases, ordinals and OFFSET work.
It retains LIMIT+OFFSET rows and uses signed rowid ties. WHERE runs first.
SQL retains ordinary NULL-first ascending ordering, unlike native exclusion.

Plan `VectorRepresentation="bit"` and `VectorBitMetric` identify the type and
metric; dense `VectorMetric` stays empty. `VectorRankPackedBytes` counts packed
work, while `VectorRankComponents` and ANN distance counters remain zero.
`VectorRanking="bounded_heap"` is an exact ranking stage, never an index claim.
Source access can independently use a sound ordinary/grams index.

Query decoding stays lazy until an eligible non-NULL row needs it. EXPLAIN does
not score, and LIMIT 0 does not decode query values. Secondary ORDER BY keys,
DESC, transformed query expressions and unlimited sorts retain the existing
MaxSortBytes-bounded general sorter. Default candidate/result/packed-byte quotas
apply to the recognized heap, not all scalar expressions in a query.

SQL reports an explicit `BIT(n) ANN is unsupported; exact packed-bit ranking`
fallback. Dense HNSW/IVFFlat creation and dense native search reject BIT columns;
they never treat GSB bytes as GSV. Typed bit ANN is a later implementation.

## Driver bindings and dense SQL transforms

`BitVector` is immutable and implements driver.Valuer. `NullBitVector` implements
Valuer/Scanner with explicit NULL state. Failed scans preserve both fields;
borrowed buffers are not retained. The invalid zero BitVector is not NULL.
Driver tests cover prepared bindings, named-type values, nullable destinations,
transactions, statement rollback and normal close/reopen.

```sql
SELECT vector_to_text(subvector('[1,2,3,4,5]',-1,3)); -- [1]
SELECT bit_to_text(binary_quantize('[-1,-0,0,1e-45,3]')); -- 00011
```

The original examples below use dense values. These transforms now also accept
HALFVEC with explicit representation-preserving semantics described in half-vector.md. `subvector`
preserves exact binary32 bits and returns GSV. Start/count must have SQL INTEGER
storage and fit signed int32; numeric text and REAL values are not coerced.
Count must be positive. End is computed from original start+count, then start
is clamped to 1 and end to dimension+1. Empty results error. For example (-1,2)
errors; (3,2147483647) truncates; (-2147483644,2147483647) yields [1,2].

`binary_quantize` emits a bit iff a dense component is >0. Both signed zeros
and negative values become 0. It returns GSB of exactly the source dimension.
The lossy result does not preserve dense distance ordering or recall.

See [the runnable rerank example](../examples/bit-rerank/README.md). It uses
exact Hamming candidates then original dense L2 distances in one read snapshot,
with the same business filter and row identities. A deliberately small candidate
set misses the true dense neighbor; exact reranking cannot recover it. Candidate
and final metrics, plans and measured recall are separate. The fixture's
materialized bit column is static and not automatically maintained.

This checkpoint makes no physical crash, fault injection, tamper, interrupted
publication, wrong-key or recovery qualification claim.

## Verification records

The [component gate](evidence/m9-bit-sql-components.txt) records the ordinary
source guard, repeated focused CGO0 tests, preserved dense regressions, focused
CGO1 race, vet and production build. The [oracle record](evidence/m9-bit-sql-oracle.txt)
records three runs of the selected ordinary interop cases. The
[review](evidence/m9-bit-sql-review.txt) records findings and fixes, with
[example output](evidence/m9-bit-sql-example.txt) and
[source hashes](evidence/m9-bit-sql-source-sha256.txt). Full combined-tree
aggregate/oracle and release gates remain separately owned and reported.
