# Remaining implementation work

This document is an engineering backlog, not acceptance evidence. Current actual
capabilities are recorded in progress.md and compatibility.md. Missing features
remain code work; unsupported is a safe interim behavior, not removal from goal.md.

## Storage, recovery and native platforms

- Broaden rollback-journal recovery segments/modes and integrate safe automatic
  Open recovery. Extend current-generation ambiguous WAL-tail handling. Source-only
  physical branches and manual harnesses do not establish runtime durability.
- Add native Darwin descriptor/inode registration and deferred-close discipline:
  process-scoped fcntl locks may be lost by closing another descriptor for the same
  inode. Every open/early-error path must participate, including sidecars.
- Add native Windows LockFileEx acquisition/conversion semantics, stable handle
  identities and supported publication/directory durability policy. Map SHM with
  allocation-granularity-aligned base addresses while retaining logical offsets;
  a32KiB SQLite region can lie inside a64KiB-aligned Windows mapping.
- Add encrypted immutable authenticated WAL transaction segments with a versioned
  root manifest selecting exact committed descriptors, bounded overlay replay,
  checkpoint/rekey/backup and normal reopen. Preserve existing snapshot families.
  Concurrent encrypted writer/readers need a separate stable lock/read-lease design.

## Required SQL batches

- Complete transaction semantics: lazy deferred snapshots, BEGIN EXCLUSIVE,
  savepoints, rollback-to/release and driver state. BEGIN/COMMIT/ROLLBACK already
  exist and must not be replaced by stubs.
- Wire actual ordinary BINARY ASC index seeks/ranges into SELECT, with proven
  residual predicates, reduced source visits and accurate plans.
- Introduce scoped immutable binding and composable bounded pull operators before
  JOIN, grouping/aggregates, subqueries, compound SELECT, CTE and windows. All
  operators share one statement snapshot and resource budget.
- Unify mutation outcomes before general constraints, conflict policies, UPSERT,
  foreign keys and triggers. SQLite FAIL can preserve earlier row changes in the
  statement; whole-statement rollback on every error is not compatible.
- Implement ALTER/DROP/views/triggers, JSON, ANALYZE/statistics and real planner use,
  native VACUUM/compaction, FTS5, ATTACH/DETACH, general backup and typed PRAGMAs.
  Official FTS5 shadow structures require genuine WITHOUT ROWID support; grams13
  is not a substitute. Multi-database commit needs explicit coordination.
- Keep current grams/vector/ANN maintenance inside the same statement/transaction;
  preserve existing disk identities and strict unknown-schema rejection. No
  unprotected temporary spill may be introduced.

## Migration and extended vector types

- Build bounded strict catalog/array inspection, a typed PostgreSQL-subset SQL
  converter using AST/source spans, deterministic review reports, local resumable
  CSV/NDJSON import and exhaustive value verification. No global operator replace.
- Generate review-only PostgreSQL/Neon transition templates separately from local
  import. Both old embedding and modern pgvector own a database-wide hnsw access
  method, so naïve side-by-side installation is invalid. Old m100/efconstruction16
  defaults and the old3/5 example violate modern ef_construction>=2*m; report
  retuning rather than silently changing values. Inventory dependencies and avoid
  automatic CASCADE/destructive cleanup.
- Preserve real[] finite binary32 values and NULL distinctions, explicit dimensions,
  primary keys and committed batch cursors. Verify exact distances and ANN recall
  separately. Legacy array Manhattan<~> becomes dense<+> only with typed legacy
  evidence; modern bit<~> remains Hamming.
- Add strict distinct half/bit/sparse representations, typed storage/SQL/driver and
  exact metrics, then versioned supported ANN identities. Preserve dense GSVv1.
  Half needs exhaustive binary16 rounding checks; sparse operations must remain
  O(nonzeros) even at dimension1e9; bit Jaccard zero/zero follows the pinned target's
  result1. Subvector and quantization require explicit lossy/rerank semantics.
- The requested current Neon page was retrieved on2026-10-10; its migration chapter
  is absent. The historical official chapter is in
  [the2024 snapshot](https://github.com/neondatabase/website/blob/d4c1cf37f79acbf4c02880eddb97dd9a36cb2d20/content/docs/extensions/pg_embedding.md#migrate-from-pg_embedding-to-pgvector).
  Do not claim the current missing chapter was followed. Pin source profiles and
  retained licenses when these implementation checkpoints land.

## Separate acceptance work

Large fixed-workload benchmarks, real-duration concurrent soak, native target-OS
runtime evidence and independent security/cryptography/license/API reviews remain
required. Restricted physical crash/fault/forced-exit/hot-recovery/tampered-disk
execution is BLOCKED/NOT_RUN and cannot be moved elsewhere as a workaround. No
source plan, cross-build, ordinary rollback or benchmark closes those gates.
