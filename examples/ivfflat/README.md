# Persistent IVFFlat example

From the repository root, using Go 1.27.2:

```sh
CGO_ENABLED=0 GOTOOLCHAIN=local go run ./examples/ivfflat
CGO_ENABLED=0 GOTOOLCHAIN=local go test ./examples/ivfflat
```

The command creates a new temporary directory, performs ordinary native writes,
closes/reopens the database, emits JSON and removes only its own directory. It
does not invoke another SQL engine, launch subprocesses, or exercise physical
failure/recovery paths.

The example deliberately creates a four-list index on an empty table. It then
inserts 128 vectors in four separated clusters and demonstrates:

1. An untrained index still uses an explained exact scan after inserts.
2. Explicit `Reindex` trains the index; native default search remains exact.
3. A named approximate search reads four centroids and one real membership
   range, rescoring the 32 original source vectors in that range.
4. A selective filter widens the nearest-centroid prefix without rereading lists.
5. Limiting compensation to one probe causes an exact fallback, with the initial
   32 source reads still counted alongside the 128-row scan.
6. Requesting all four probes directly uses an exact scan, with no centroid or
   membership reads.
7. Eligible SQL automatically chooses IVFFlat; `NOT INDEXED` supplies the exact
   reference.
8. A committed INSERT/UPDATE/DELETE transaction maintains fixed-centroid
   memberships, followed by an explicit SQL `REINDEX` and ordinary reopen.

All output counters come from actual `Plan`/`Stats` values. `exact: false` stays
false even where this easy clustered example happens to match the exact top-K,
including filter compensation that opens all lists. Counters are logical
records/keys and metric calls, not physical I/O. This is a usage/regression
example, not a recall or latency benchmark. See [ANN documentation](../../docs/ann.md)
for the held-out benchmark and its limits.
