# Dense vectors and exact KNN (M6 ordinary implementation)

The engine implements finite float32 VECTOR(n), native/SQL exact search and text/vector hybrid examples. It does not yet implement HNSW or IVFFlat. Ordinary successful persistence is tested; physical recovery/crash qualification remains blocked. See vector-primitives.md for the pinned pgvector reference, deliberate arithmetic differences and binary codec.

## Real SQL surface

```sql
CREATE TABLE items(id INTEGER PRIMARY KEY, embedding VECTOR(3), active INTEGER);
INSERT INTO items VALUES(1,'[1,2,3]',1),(2,'[2,3,4]',1),(3,NULL,1);
SELECT id,embedding <-> '[1,2,3]' AS distance
FROM items WHERE active=1 AND embedding IS NOT NULL
ORDER BY distance LIMIT 10;
```

VECTOR(n) requires1..16,000 dimensions. Declarations and literal defaults remain legal ordinary SQLite schema SQL. Native INSERT/UPDATE validates all non-NULL typed values and canonicalizes them to the versioned GSV BLOB. Wrong dimensions/nonfinite values/unknown or malformed encodings fail without partial statement changes; earlier successful statements in an explicit transaction remain. NULL and NOT NULL follow normal supported SQL constraints. String defaults are validated at CREATE and converted when stored; function defaults remain unsupported.

The functions are vector(value) (canonical BLOB), vector_to_text(value), vector_dims(value), vector_norm(value), l2_distance(a,b), l1_distance(a,b), cosine_distance(a,b), inner_product(a,b) and negative_inner_product(a,b). All propagate NULL. Text is bracketed decimal syntax; BLOB is strictly the versioned binary codec, never guessed textual bytes. Numeric scalars are not vectors. Context is checked around bounded conversion/arithmetic.

| Operator | Meaning | Ordering |
|---|---|---|
| `<->` | L2 Euclidean distance | ascending |
| `<+>` | L1/Manhattan distance | ascending |
| `<#>` | negative inner product | ascending ranks largest positive dot product first |
| `<=>` | cosine distance | ascending; zero norm errors |

inner_product returns the positive dot product. `<~>` is not dense Manhattan and remains unsupported until a separate bit type is implemented. Operators are project SQL extensions at SQLite's left-associative bitwise tier, above comparisons and below arithmetic. Existing SQLite precedence is unchanged; this is not a general PostgreSQL grammar. Parenthesize mixed expressions for clarity.

Ordinary SQL equality and non-distance ordering retain SQLite storage-class/BLOB semantics. No pgvector vector equality, aggregate, arithmetic, cast/operator-class or general PostgreSQL compatibility is claimed.

## External SQLite values and typed boundaries

Official SQLite accepts VECTOR(n) as a legal affinity declaration but does not enforce this project's vector semantics. It may store ordinary TEXT or BLOB. Native typed interpretation validates the declared dimension while preserving the actual stored SQL class; a valid external TEXT value stays TEXT on read and becomes canonical BLOB on a native rewrite. Referenced invalid typed values error explicitly. Predicates that exclude a row without referencing that vector can run before vector interpretation. Native SQL does not silently repair malformed values.

The ordinary interop suite uses512/4096/65536-byte pages in rollback/shared-WAL modes, official-created declarations, native canonical writes, ordinary official text/BLOB writes, exact signed-zero/subnormal bit preservation, rollback, normal reopen/checkpoint and official integrity_check. A wrong-dimension textual value inserted by ordinary official SQL is a type-compatibility negative test in a well-formed SQLite file, not a damaged-file/recovery test.

## Go values and database/sql

```go
v, err := vector.New([]float32{1, 2, 3})
if err != nil { return err }
_, err = db.ExecContext(ctx, "INSERT INTO items(id,embedding) VALUES(?,?)", 1, v)
var nullable vector.NullVector
err = db.QueryRowContext(ctx, "SELECT embedding FROM items WHERE id=?", 1).Scan(&nullable)
```

Vector implements driver.Valuer; NullVector implements Valuer/Scanner. New/Values copy slices; a constructed Vector is immutable. Bind nil or an invalid NullVector for SQL NULL; zero Vector is invalid. Bare []float32 must be wrapped with vector.New rather than relying on reflection. Scanner accepts nil, text or strict binary BLOB and preserves its previous value on error. Driver-owned buffers are not retained.

## Native exact query

```go
result, err := db.VectorSearch(ctx, "items", "embedding", v,
    sqlite.VectorSearchOptions{
        Metric: vector.Cosine, Limit: 10,
        Where: "active=?", Args: []any{1},
    })
```

DB and Tx expose VectorSearch. A physical VECTOR(n) column is required; arbitrary BLOB columns do not acquire a guessed dimension. Empty Metric means L2. Only built-in MetricKind identities are accepted, not arbitrary names from callbacks. Limit is explicit and nonnegative; zero validates the query/options but reads no source rows. NULL query yields EMPTY. Normal SQL value conversion supports Vector/NullVector and checks cancellation after Valuer callbacks before reporting success.

The operation owns one snapshot; a transaction sees its own writes. It scans each source row, evaluates one supported bound WHERE expression, then decodes/scores eligible vectors. NULL candidates are excluded. Any eligible zero-norm cosine candidate errors, as does a zero-norm cosine query; no undefined value is silently skipped. Hits are distance ASC,signed rowid ASC. Errors/cancellation return a zero result, never partial hits. The whole source is searched within quotas; this is not ANN.

VectorPlan embeds QueryPlan and adds Field, Metric, Dimensions, Limit and Exact. Its inherited Vector* fields describe the separate SQL ranking stage and remain unset for native calls; use its own Metric/Dimensions/Exact fields for native semantics. Access is SCAN or EMPTY and Detail states exact mode, NULL policy and reason. VectorStats embeds QueryStats and adds FilteredRows, ScoredRows, NullRows and ScoredComponents. BaseRowFetches counts every visited row, including excluded/NULL rows. ScoredComponents charges the query once plus every scored vector dimension; it does not claim to count arbitrary WHERE/scalar work or B-tree pages.

Zero SearchLimits selects defaults:100,000 visited candidates,10,000 result rows,2^27 charged components. Partially specified limits reject. MaxSortBytes additionally bounds retained identities/results. Every visited row counts, even when filtered or NULL; limits return errors rather than incomplete exact results. These are logical budgets, not exact process-memory/time guarantees. Trusted callbacks' allocations and side effects remain outside the engine's control.

## SQL bounded ranking and truthful plans

A narrow optimization recognizes one ASC distance key over a declared vector column and a literal/bind, with bounded LIMIT. Both operand orders, output aliases and ORDER BY result ordinals are supported. It scans the selected source and retains only LIMIT+OFFSET rows in a bounded worst-first heap, with exact distances and signed-rowid tie-breaking.

Business WHERE executes first. Source access can be SCAN or an independently sound grams13 SEARCH; a ranking heap is never called an index. SQL keeps ordinary NULL-first ascending order, unlike native NULL exclusion. EXPLAIN describes ranking but does not score vectors. Query parsing remains lazy about bad query-vector values until a qualifying non-NULL row needs a score; a zero LIMIT never scores.

Secondary ORDER BY keys, DESC, computed query-vector operands and unbounded LIMIT keep the existing general MaxSortBytes-bounded SQL sorter, preserving explicit ordering semantics. The recognized heap uses default vector candidate/result/component quotas; LIMIT+OFFSET must fit10,000 retained rows. None of these limits may silently truncate an exact answer.

Rows.Plan adds VectorMetric, VectorColumn, VectorDimensions, VectorTopK, VectorExact and VectorRanking="bounded_heap". Rows.Stats adds VectorRankedRows, VectorFilteredRows, VectorNullRows and VectorRankComponents. These count the recognized ranking stage only, not all repeated scalar projections/filters. Existing source posting/base-row counters remain distinct. Rows.Close releases the ranking cache while retaining value-only plan/stats diagnostics.

## Text/vector hybrid search

```sh
CGO_ENABLED=0 go run ./examples/vector /new/disposable/hybrid.db
```

The executable example obtains text SimilaritySearch and cosine VectorSearch lists within one read transaction and the same business filter, then computes weighted RRF. Output preserves raw text scores/cardinalities, vector distances, fields/metric and one-based rank/weight contributions. Missing source measures are explicit NULL. Fusion is exact for the supplied Top-K lists, not a claim of global recall beyond those lists.

The same snapshot also executes a SQL query combining grams13 LIKE candidates, a business predicate, bigram text scoring and exact vector ORDER BY/LIMIT. Its actual source and ranking plans/counters are returned separately. This supplies the ordinary M5 vector/text integration; restricted durability and later ANN/security/release gates remain open.

Evidence: evidence/m6-review.md, m6-driver-oracle-example.txt and m6-aggregate.txt. No PostgreSQL/pgvector executable, ANN index or independent release audit is implied.
