# Billion-dimensional sparse exact search

Run `go run ./examples/sparse-exact /new/disposable/path/sparse.db`.
The path must be new. The example leaves an ordinary SQLite-compatible file.

The 1,000,000,000-dimensional query contains two nonzeros and encodes into 32
GSS bytes. It runs native and SQL L2 searches on one transaction's own writes,
filters an inactive row, excludes NULL explicitly in SQL, and reports support
work (13 charged entries) separately from dimension. Native NULL candidates
are excluded automatically. All three selected distances agree.

The output explicitly checks that converting this value to dense fails before
allocation. This example is exact full-scan search, with no sparse ANN or
PostgreSQL execution. The ranking heap is bounded result retention, not an index.
