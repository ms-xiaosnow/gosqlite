# Native persistent HNSW measurement

```sh
CGO_ENABLED=0 GOTOOLCHAIN=local go run ./benchmarks/ann -rows 2048 -dims 12 -queries 48 -seed 20261010
```

The command owns a new temporary directory, creates a real SQLite database,
builds the persistent index, checks it, closes and reopens, then compares exact
and approximate top1/10/100 in the same read transaction per query. The command
prints JSON and removes only its own temporary directory on normal completion.
There are no subprocesses, physical failure tests, or external reference scores.

Dataset: pinned Go math/rand standard-normal draws narrowed to float32; held-out
queries follow source rows on the same seeded stream. SHA256 of all canonical
GSV vector/query bytes is recorded. HNSW uses fixed default graph parameters.
Warm OS cache, no forced flushing, an unmeasured warmup for each K, alternating
exact-first/ANN-first timed order. Percentiles use sorted sample index
floor((N-1)*p). Wall-clock samples on a shared cloud VM are descriptive only;
no confidence interval, cold-cache or isolated machine claim.

First run2026-10-10T11:47:23Z: Go1.27.2, Linux/amd64, AMD EPYC9V74 80-Core
Processor host model,9 visible logical CPUs, GOMAXPROCS9, approximately9.75GiB
visible memory; page4096, ordinary DELETEjournal. Output retained in
`docs/evidence/m7-hnsw-persistent-benchmark.json`. Recall@10=1.0; source fetches
and metric calls reduce, but p50 latency is substantially slower than exact at
this scale. Future optimization must retain this baseline and rerun the same
command; do not convert distance-count reduction into a latency claim.
