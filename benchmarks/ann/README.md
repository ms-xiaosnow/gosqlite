# Native persistent ANN measurement

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
GSV vector/query bytes is recorded. HNSW uses fixed default graph parameters;
IVFFlat uses100 lists,20 training iterations and a10000 sample cap unless the
listed flags change its list/probe selection.
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

## IVFFlat probe curve

```sh
CGO_ENABLED=0 GOTOOLCHAIN=local go run ./benchmarks/ann -method ivfflat -rows 2048 -dims 12 -queries 48 -seed 20261010 -lists 100 -probes 1
CGO_ENABLED=0 GOTOOLCHAIN=local go run ./benchmarks/ann -method ivfflat -rows 2048 -dims 12 -queries 48 -seed 20261010 -lists 100 -probes 10
CGO_ENABLED=0 GOTOOLCHAIN=local go run ./benchmarks/ann -method ivfflat -rows 2048 -dims 12 -queries 48 -seed 20261010 -lists 100 -probes 50
```

The default method remains hnsw. `-max-probes 0` uses the selected list count as
the maximum compensation bound. IVFFlat performs real persistent centroid/list
access after ordinary reopen. Output separates centroid scores, lists opened,
membership keys, original source scores and combined distance work, and records
effective probe ranges. Graph-only fields are zero for IVF.

These three actual runs share the HNSW dataset/query SHA256. Recall@10 is
0.2541667/0.7708333/0.9958333, with no exact fallbacks, but all three settings are
slower than exact on this small dataset. These are separate shared-cloud timing
observations, not isolated controlled comparisons or a default95% recall claim.
Full JSON is in docs/evidence/m7-ivf-benchmark-probes{1,10,50}.json. The original
probes1 JSON has a legacy descriptive graph note; its actual IVF method/counters
and measurements are preserved unchanged and explained in docs/ann.md.
