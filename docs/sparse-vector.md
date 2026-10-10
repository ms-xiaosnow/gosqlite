# SPARSEVEC SQL and support-bounded exact search (M9 checkpoint G)

This slice integrates immutable `vector.SparseVector` into typed SQL, ordinary
SQLite-compatible storage, `database/sql`, and exact DB/Tx/SQL search. The
representation is **GSS v1**, independent of dense GSV, half GSH and bit GSB.
Sparse ANN, PostgreSQL wire compatibility and PostgreSQL vector equality are
not implemented or implied.

## Values and ordinary storage

```sql
CREATE TABLE items(id INTEGER PRIMARY KEY,s SPARSEVEC(1000000000),active INTEGER);
INSERT INTO items VALUES(1,'{1:3,1000000000:-2}/1000000000',1),
                        (2,sparsevec('{3:4}/1000000000'),1),(3,NULL,1);
SELECT id,s <-> '{1:3}/1000000000' AS distance
FROM items WHERE active=1 ORDER BY distance LIMIT 2;
```

`SPARSEVEC(n)` requires an explicit decimal dimension in 1..1,000,000,000.
Each value contains at most 16,000 entries. Dimension is metadata: no sparse
parse, storage, scalar metric or search operation allocates dimension-sized
arrays, visits implicit zeros or charges the dimension as work. An empty
support `{}/n` is a valid zero vector, distinct from NULL or an invalid zero Go
object. Invalid/unknown declarations fail closed; dimensions are never inferred.

Text uses one-based indices; Go entries and stored indices are zero-based.
The existing parser rounds decimal values directly to binary32, preserves
representable subnormals, rejects nonzero decimal underflow to zero and all
NaN/infinity. It validates every index, including explicit zero entries, removes
signed zeros, sorts retained entries, and rejects duplicate nonzero indices.
The input cap counts explicit zeros too. This deliberately retains the existing
primitive policy: text/native construction can canonicalize order; binary
input must already have strictly increasing unique indices and finite nonzero
values. No binary repair, sorting or zero removal is performed on decode.

GSS encoding is 16 + 8*nnz bytes: `GSS\0`, version 1, format 4, endian 1,
flags 0, uint32 dimension, uint32 nnz, then interleaved uint32 index / binary32
bits. All integers and value bits are big-endian. Headers, version, count,
length, bounds, order, uniqueness and values are strictly checked. It is not a
PostgreSQL network format and adds no checksum/authentication of its own.

Native INSERT/UPDATE/default values validate dimensions and canonicalize to
GSS BLOB. Numbers, textual `[]byte` and the other vector codecs reject.
Ordinary statement undo, savepoints, rollback, rowid changes, deletes, reopen,
overflow pages and shared-WAL snapshots use the existing transactional path.
`SparseVector` implements `driver.Valuer`; `NullSparseVector` implements nullable
Valuer/Scanner, owns its data and preserves both fields after failed scans.

Official SQLite assigns NUMERIC affinity to `SPARSEVEC(n)`. Its legal external
writers may store sparse text or canonical GSS; reads validate but retain the
actual TEXT/BLOB storage class. ALTER-added defaults on old short rows retain
SQLite affinity. Unsupported stored values fail on interpretation and can be
replaced by an explicit ordinary UPDATE. Predicates that don't reference the
sparse column can filter them first. Ordinary equality/order remain SQLite
storage-class semantics. Generic index planning excludes typed vector keys and
predicates that could conceal interpretation errors.

## Functions, metrics and representation identity

All functions propagate SQL NULL, with cancellation checked first.

- `sparsevec(value)` returns canonical GSS
- `sparsevec_dims`, `sparsevec_nnz`, `sparsevec_norm`, `sparsevec_to_text`
- `l2_distance(a,b)` / `<->`: Euclidean square-root L2
- `l1_distance(a,b)` / `<+>`
- `negative_inner_product(a,b)` / `<#>`; `inner_product` returns positive IP
- `cosine_distance(a,b)` / `<=>`

Numeric functions/operators select sparse math when GSS or a known SPARSEVEC
expression supplies representation identity. Untyped text on the other side is
then sparse text; two untyped strings retain the existing dense signature.
Use `sparsevec(...)` to make sparse identity explicit. Known representation
identity follows unary +, coalesce/ifnull/nullif and result aliases, including
valid external TEXT. Mixed dense/half/bit/sparse signatures reject; dense-only
constructors and accessors do not infer a sparse conversion. Hamming/Jaccard
remain bit-only. Operator precedence stays the engine's existing
left-associative bitwise tier, not PostgreSQL's general operator grammar.

Metrics merge sorted supports in O(nnz(a)+nnz(b)) time and O(1) arithmetic
space, using compensated float64 accumulation. This intentionally differs
from upstream binary32/architecture-sensitive accumulation. Sparse norm also
visits only support. Cosine clamps to [0,2] and a zero norm returns
`vector.ErrZeroNorm` rather than NaN. Published upstream overflow/NaN fixtures
are checked against explicitly separate finite64 policies, not claimed equal.

Explicit `sparsevec_from_vector` / Go `SparseFromVector` preserves nonzero
binary32 bits and drops both signs of dense zero. `vector_from_sparsevec` /
Go `SparseVector.Dense` requires dimensions <=16,000 before allocating output;
a billion-dimensional value returns `vector.ErrLimit`. `subvector(sparse,...)`
and `binary_quantize(sparse)` are unsupported and reject; they never silently
densify or borrow half/dense semantics. Materialized transformed columns must
be maintained explicitly by applications.

## Native exact search

```go
result, err := db.SparseSearch(ctx, "items", "s", query,
    sqlite.SparseSearchOptions{
        Metric: vector.L2, Limit: 10,
        Where: "active=?", Args: []any{1},
    })
```

DB and Tx require a physical SPARSEVEC column. Query input may be sparse TEXT,
GSS, `SparseVector`, `NullSparseVector`, an ordinary Valuer or NULL. Metrics are
closed to L2 (default), L1, NegativeIP and Cosine; no arbitrary callbacks or
approximate mode. Each operation uses one snapshot and explicit transactions
see their own writes. WHERE precedes candidate decoding/scoring. Native NULL
candidates are excluded; NULL query yields EMPTY. Limit zero validates the
non-NULL query, including cosine norm, without source reads. Results order by
distance ASC, signed rowid ASC. Error/cancellation returns a zero result and
never partial exact hits. `SparsePlan` reports `exact_sparsevec`, dimensions,
metric, SCAN/EMPTY and exactness; embedded SQL-ranking fields stay unset.

Zero `SparseSearchLimits` selects MaxCandidates=100000, MaxResults=10000 and
MaxNonzeros=2^27. Partial settings reject. Hard caps are 2^20 candidates/results
and 2^34 nonzero work; limits fail instead of truncating exact search.
Every fetched row consumes a candidate, including NULL/filtered rows.
`ScoredNonzeros` charges query input entries once plus both input supports per
scored merge. GSS counts exactly nnz; TEXT conservatively counts explicit zero
entries as well. Implicit zeros never count. A zero-support metric consumes no
support work but still consumes its source candidate.

`MaxSortBytes` conservatively reserves query/candidate support scratch, result
heap and retained row identities. Checks precede vector decoding, including
known typed Valuer encoding, and use bounded counts and checked subtraction.
Memory usage and limits never multiply the dimension. Caller-owned inputs,
custom Valuer callback allocations, general SQL/filter/projection work, pager
I/O and allocator/RSS overhead are outside these logical search budgets.

## Bounded SQL ranking

The narrow optimizer handles one ASC numeric metric over a physical SPARSEVEC
column and a literal/bind query with bounded LIMIT. Both operand orders,
aliases/ordinals and LIMIT/OFFSET retain exact sparse scoring; retention is
LIMIT+OFFSET. SQL keeps ordinary NULL-first ASC behavior. Unsupported shapes
such as DESC, additional sort keys, transformed query expressions and unlimited
sorting use the existing MaxSortBytes-bounded general sorter.

`QueryPlan.VectorRepresentation="sparsevec"`, numeric metric, dimensions and
`VectorExact=true` identify the stage; `VectorRanking="bounded_heap"` does not
claim an index. `VectorRankNonzeros` uses the same entry-work contract as native
search. Dense/half component and packed-bit counters remain zero. The eligible
heap uses default exact-ranking work limits. Scalar/general expression work is
bounded by codec limits rather than the heap's cumulative quota.

EXPLAIN and LIMIT 0 do not decode the query, and all-NULL/filtered candidate sets
need not decode it. Typed candidates still validate if actually selected with
a NULL query, matching scalar interpretation. Scratch and retained-row memory
are charged together. Cancellation checks surround parsing/encoding and occur
at most every 256 support positions during metric/norm arithmetic; parsing is
bounded by 1 MiB text and 16,000 entries.

Plans say `SPARSEVEC(n) ANN is unsupported; exact sparse ranking`. Dense HNSW
and IVFFlat constructors reject sparse columns. There is no sparse ANN index,
opclass, 1000-nnz HNSW claim or approximate recall claim in this checkpoint.

## Evidence and runnable example

The [sparse exact example](../examples/sparse-exact/README.md) stores a
billion-dimensional/two-nonzero query in 32 bytes, reads its own writes, filters
ordinary rows and demonstrates exact native/SQL ranking with 13 charged support
entries. It explicitly verifies refusal of expensive dense conversion.

Public pgvector v0.8.7 fixtures retain the existing pin
`f37c13f68b57d2c3472b2214fbcff699d6d34876` with source hashes/license in the
[fixture manifest](../testdata/pgvector-v0.8.7/extended/manifest.json). SQL tests
cover 56 published parse and 40 metric cases. Independent dictionary-union math
checks all four metrics without a production metric or dimension-sized oracle.
No live PostgreSQL or general PostgreSQL SQL compatibility is claimed.

The [component evidence](evidence/m9-sparse-sql-components.txt) records focused
CGO0, separate CGO1 race, preserved dense/half/bit tests, official SQLite valid
file interop, vet/build and the source guard. Independent review is recorded
[separately](evidence/m9-sparse-review.txt). [Example output](evidence/m9-sparse-sql-example.json)
and [source hashes](evidence/m9-sparse-sql-source-sha256.txt) identify this slice.
Only intact ordinary files and memory-only malformed representations are used.
Crash/fault/tamper/wrong-key/namespace replacement/interrupted publication/rekey
and recovery qualification are not executed or claimed; global release gates
remain separate.
