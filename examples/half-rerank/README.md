# Half candidates, original dense reranking

Run against a **new disposable path** (existing paths are refused):

```sh
go run ./examples/half-rerank /tmp/my-new-half-rerank.db
```

The executable writes a small ordinary database with `VECTOR(4)` originals and
explicitly rounded `HALFVEC(4)` copies, then begins one read-only transaction.
It compares exact dense L2 top-1 to exact half L2 candidate sets of size 1 and 2,
reranks their original dense values within the same snapshot and business
filter, and emits JSON containing metrics, plans, counters and observed recall.
It also prints the query's binary32-to-binary16 conversion report, a SQL half
subvector, and binary quantization of that half value.

Both candidates round to the same binary16 vector. Rowid tie-breaking selects
the less-close dense item when only one candidate is retained: measured recall
at 1 is **0** for one candidate and **1** for two on this fixture. Neither value
is a general recall guarantee. Reranking cannot recover excluded candidates.
There is no half ANN in this checkpoint; both candidate and reference searches
are truthful full scans. The materialized half column is populated explicitly
alongside the original and is not automatically maintained for future writes.

No PostgreSQL runtime or PostgreSQL-only SQL is used. The example performs
ordinary create/insert/commit/read/close only; it does not clear existing files
or exercise crash, tamper, fault injection or recovery workflows.
