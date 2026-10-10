# ADR0009: Transactional persistent HNSW

Status: accepted for the experimental first M7 slice; full M7 and release acceptance remain open.

## Decision

Keep the M6 exact default native API. Add explicit approximate native search and
an observable automatic SQL HNSW path for eligible NOT NULL typed-vector distance
ORDER BY/LIMIT. Build a real deterministic multilayer graph behind transaction-
owned lazy Reader/Writer ports. Persist legal ordinary SQLite state/node/live
mapping tables and B-trees in the same statement and transaction as source data.
Do not preload a full graph at Open/search. Recheck identities, WHERE and source
vector bits before exact distance recomputation. Expose approximate recall even
though the recomputed values are exact.

Preserve existing grams schemas byte-for-byte. The optional complete ANN family
is introduced on first index creation; old grams-only files keep working. Format1
uses canonical strict method configuration, GSV source vectors and versioned
bounded adjacency. Future IVF serialization must not change existing canonical
HNSW bytes. Empty centroid/member tables reserve layout, not feature support.

Deletion and replacement use immutable node identities and tombstones retaining
original vector/topology. Reindex atomically compacts and advances generation.
Explicit node/tombstone/work/memory limits bound growth and queries. Filtered
search widens ef without removing routing nodes. Insufficient hits fall back to
exact scan with prior work charged; resource errors never produce partial native
success. Existing source dirty guards and schema epoch checks remain authoritative.

## Alternatives and tradeoffs

Full graph loading would be simpler but violate bounded lazy opening/search.
Tombstones avoid unsound rowid reuse and graph disconnection from immediate edge
delete at the cost of increasing storage until rebuild. A fake ANN full scan is
rejected. Unbounded iteration/implicit quota growth is rejected. Cosine zero norms
are explicit write/build errors, differing from upstream exclusion semantics.

The first real persistent benchmark shows recall@10=1.0 and substantial source/
distance work reduction, but slower latency than exact scan on2048×12 dimensions.
This is retained as baseline. Repeated B-tree lookup/validation is expensive;
future optimization must preserve snapshot correctness and accounting. No
performance improvement or universal recall guarantee is claimed.

## Evidence and limits

Ordinary storage/query/race/model tests, pre-ANN file upgrade and six-way official
SQLite page-size/journal-mode interop pass. Review caught and fixed small-table
large-LIMIT memory overreservation by bounding the effective target to validated
live population. Whole graph consistency checks are explicit operations, not
hidden query scans. Physical interrupted publication/recovery tests remain blocked.

IVFFlat code, richer ANN registry hooks, general PostgreSQL grammar, broad native
platform runtime, independent audits and release evidence remain open. See
ann.md, progress.md and evidence for exact scope and terminal gate results.
