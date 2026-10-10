# Exact bit candidates and dense reranking

Run with a new disposable file:

```sh
CGO_ENABLED=0 go run ./examples/bit-rerank /new/disposable/bit-rerank.db
```

The example populates real `VECTOR(4)` and `BIT(4)` columns together using SQL
`binary_quantize`, then obtains Hamming candidates and reranks original dense
vectors by L2 in a single read transaction. Both stages use `active=1` and the
same row identities. It also executes SQL `subvector`.

The JSON reports candidate Hamming distances and final L2 distances separately,
with actual plans/counters and an exact dense reference. With one candidate it
returns row 10 and has observed recall@1 = 0; with two it returns row 20 and has
recall@1 = 1 for this fixture. Exact reranking cannot restore an excluded true
neighbor. Bit candidate search is an exact full scan; no bit ANN is claimed.

The bit column is a static materialized fixture. It is not a generated column,
expression index or automatically maintained transform. Applications using this
pattern must explicitly maintain source and transformed values together.

Tests use ordinary SQL, a committed setup, a read transaction and normal reopen.
They do not exercise crash, fault injection or recovery workflows.
