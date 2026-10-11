# Dense vectors and exact KNN (M6 ordinary implementation)

The engine implements finite float32 VECTOR(n), native/SQL exact search and text/vector hybrid examples. The first M7 slice now also implements persistent HNSW; IVFFlat remains open. This page describes the exact path; see ann.md for explicit native approximate mode and automatic eligible SQL ANN planning. Ordinary successful persistence is tested; physical recovery/crash qualification remains blocked. See vector-primitives.md for the pinned pgvector reference, deliberate arithmetic differences and binary codec.

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

In the default exact mode, the operation owns one snapshot; a transaction sees its own writes. It scans each source row, evaluates one supported bound WHERE expression, then decodes/scores eligible vectors. NULL candidates are excluded. Any eligible zero-norm cosine candidate errors, as does a zero-norm cosine query; no undefined value is silently skipped. Hits are distance ASC,signed rowid ASC. Errors/cancellation return a zero result, never partial hits. The whole source is searched within quotas; this is not ANN.

VectorPlan embeds QueryPlan and adds Field, Metric, Dimensions, Limit and Exact. Its inherited Vector* fields describe the separate SQL ranking stage and remain unset for native calls; use its own Metric/Dimensions/Exact fields for native semantics. Access is SCAN or EMPTY and Detail states exact mode, NULL policy and reason. VectorStats embeds QueryStats and adds FilteredRows, ScoredRows, NullRows and ScoredComponents. BaseRowFetches counts every visited row, including excluded/NULL rows. ScoredComponents charges the query once plus every scored vector dimension; it does not claim to count arbitrary WHERE/scalar work or B-tree pages.

Zero SearchLimits selects defaults:100,000 visited candidates,10,000 result rows,2^27 charged components. Partially specified limits reject. MaxSortBytes additionally bounds retained identities/results. Every visited row counts, even when filtered or NULL; limits return errors rather than incomplete exact results. These are logical budgets, not exact process-memory/time guarantees. Trusted callbacks' allocations and side effects remain outside the engine's control.

## SQL bounded ranking and truthful plans

A narrow optimization recognizes one ASC distance key over a declared vector column and a literal/bind, with bounded LIMIT. Both operand orders, output aliases and ORDER BY result ordinals are supported. It scans the selected source and retains only LIMIT+OFFSET rows in a bounded worst-first heap, with exact distances and signed-rowid tie-breaking.

Business WHERE executes first. Source access can be SCAN or an independently sound grams13 SEARCH; a ranking heap is never called an index. SQL keeps ordinary NULL-first ascending order, unlike native NULL exclusion. EXPLAIN describes ranking but does not score vectors. Query parsing remains lazy about bad query-vector values until a qualifying non-NULL row needs a score; a zero LIMIT never scores.

2026-10-11 未发布窄修合同：EXPLAIN 的 LIMIT/OFFSET bound hint 只读取字面量或
已通过普通绑定准入的参数中的 int64，以及有限、可精确表示为整数且严格位于
(-2^63, +2^63) 的 REAL；REAL 两端均排除。允许安全 unary `+`/`-`，但对
MinInt64 取负为 unknown。解析器已表示为 int64 的 TRUE/FALSE 也遵循这条规则。
NULL、TEXT（包括数字文本）、BLOB、function、CAST、binary expression、其他形态
或超界值均为 unknown；hint 不执行它们，也不新增 bound runtime 错误。普通参数
准入和名字/函数绑定校验仍先发生，并可按原合同报错。

已知 LIMIT 0 不读取 OFFSET 提示；正 LIMIT 要求 OFFSET 缺省或已知（缺省为 0，
已知负数按 0），不把 unknown OFFSET 猜成 0。LIMIT 与规范化 OFFSET 的安全
合计必须不超过现有 `vector.DefaultSearchLimits().MaxResults`（10,000）；否则
保守放弃有界排名提示。未知、负数/无限 LIMIT 同样保守处理。实际 SELECT 仍用原有
求值/转换和错误阶段；例如数字文本 LIMIT 的 SELECT 可以使用 ANN，EXPLAIN 却
保守显示不采用该 ANN/有界堆提示，不能把 EXPLAIN 当成该 SELECT 的已执行计划。

只有这项 bound hint 不消耗 SQL 求值预算，也不调用求值器或转换回调；这不是整个
EXPLAIN 零预算或零元数据 I/O 的承诺。已有 vector operand 规划和 IVF 状态元数据
读取仍有原边界。该生产窄修在 209ba 的旧规划回归测试已 PASS，但该树因新增
shape 断言错误总体 ordinary FAIL。只修测试常量的 52b 已取得本地两 quick / 全
15 门 PASS，含实际 oracle；未发布，也不证明 native/物理或 M10 资格。
两轮原始 FAIL 与源码身份见[分层证据](evidence/startup-wal-limit-20261011.md)。

Secondary ORDER BY keys, DESC, computed query-vector operands and unbounded LIMIT keep the existing general MaxSortBytes-bounded SQL sorter, preserving explicit ordering semantics. The recognized heap uses default vector candidate/result/component quotas; LIMIT+OFFSET must fit10,000 retained rows. None of these limits may silently truncate an exact answer.

Rows.Plan adds VectorMetric, VectorColumn, VectorDimensions, VectorTopK, VectorExact and VectorRanking="bounded_heap". Rows.Stats adds VectorRankedRows, VectorFilteredRows, VectorNullRows and VectorRankComponents. These count the recognized ranking stage only, not all repeated scalar projections/filters. Existing source posting/base-row counters remain distinct. Rows.Close releases the ranking cache while retaining value-only plan/stats diagnostics.

## Text/vector hybrid search

```sh
CGO_ENABLED=0 go run ./examples/vector /new/disposable/hybrid.db
```

The executable example obtains text SimilaritySearch and cosine VectorSearch lists within one read transaction and the same business filter, then computes weighted RRF. Output preserves raw text scores/cardinalities, vector distances, fields/metric and one-based rank/weight contributions. Missing source measures are explicit NULL. Fusion is exact for the supplied Top-K lists, not a claim of global recall beyond those lists.

The same snapshot also executes a SQL query combining grams13 LIKE candidates, a business predicate, bigram text scoring and exact vector ORDER BY/LIMIT. Its actual source and ranking plans/counters are returned separately. This supplies the ordinary M5 vector/text integration; restricted durability, remaining ANN implementation and security/release gates remain open.

Evidence: evidence/m6-review.md, m6-driver-oracle-example.txt and m6-aggregate.txt. These M6 evidence files do not imply a PostgreSQL/pgvector execution or independent release audit. The later HNSW evidence is documented separately in ann.md.
