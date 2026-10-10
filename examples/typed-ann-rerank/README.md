# Typed ANN candidates and original dense reranking

Run with Go 1.27.2 and a new disposable path:

```sh
CGO_ENABLED=0 GOTOOLCHAIN=local go run ./examples/typed-ann-rerank /tmp/new-ann-rerank.db
```

This builds and normally reopens genuine halfvec L2 and bit Hamming HNSW
indexes, then obtains approximate candidates and exact original dense L2 scores
inside the same read transaction, using the same business filter and row IDs.
Actual plans, graph/source counters and recall are emitted as JSON.

The one-candidate runs miss the nearest dense neighbor. Two candidates recover
it on this deliberately small fixture. Half rounding and binary quantization
change rankings; an exact reranker cannot restore a missing candidate. The
materialized transformed columns must be maintained explicitly by applications.
There is no general expression-index, speedup, or universal recall claim.
