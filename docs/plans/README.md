# Remaining implementation work

This document is an engineering backlog, not acceptance evidence. Current actual
capabilities are recorded in progress.md and compatibility.md. Missing features
remain code work; unsupported is a safe interim behavior, not removal from goal.md.

## Storage, recovery and native platforms

- Broaden rollback-journal recovery segments/modes and integrate safe automatic
  Open recovery. Extend current-generation ambiguous WAL-tail handling. Source-only
  physical branches and manual harnesses do not establish runtime durability.
- Plain Darwin process-fcntl aggregation/deferred close and Windows/amd64 native
  checked handles/locks/mapping/publication code now exist. Complete remaining
  native policy and target-runtime evidence; default Windows cross-vet has one
  disclosed unsafe-pointer mapping warning. No physical durability claim follows.
- Linux encrypted immutable shared selections and one writer now exist, with
  pinned copy/checkpoint and quiescent rekey. Native encrypted Windows/Darwin
  ownership/coordination now exists; Windows shared rekey and Darwin Unicode
  inventory remain explicit code gaps, with native runtime qualification open.
- Retained encrypted history has no garbage collection or external freshness
  guarantee. Keep these limitations distinct from independent security review.

## Required SQL batches

- Lazy deferred snapshots, BEGIN EXCLUSIVE, nested savepoints, rollback-to/release,
  bounded aggregate undo and authoritative driver state now exist. Complete cursor
  relaxation and remaining connection-state/statement-outcome parity without
  regressing these implemented paths.
- Ordinary rowid and leading BINARY ASC equality seeks now exist. Extend to
  inequalities, covering access, reverse/order use and statistics only with
  proven residual/error semantics, reduced work and truthful diagnostics.
- CASE/CAST/BETWEEN/IN-list scalar nodes, SELECT ALL and explicit NULL ordering
  now exist with immutable boolean binding. Scoped named-table INNER/CROSS/comma/LEFT
  joins now use bounded composable pull operators and one statement snapshot.
  Correct inherited predicate AND/OR error suppression, then extend remaining
  JOIN forms, grouping/aggregates, subqueries, compound SELECT, CTE and windows.
  Preserve binding, lifetime and shared work/memory budgets across operators.
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

- Bounded strict catalog/real-array inspection and typed PostgreSQL-subset
  AST/source-span conversion now exist, with explicit local and review-only
  PostgreSQL targets. Resumable bounded CSV/NDJSON import and exhaustive typed
  readback verification now exist for an explicit single-table dense-vector
  mapping. Extend supported migration shapes without weakening source underflow,
  identity or bind contracts; no
  global operator replacement or automatic source-error coercion.
- Review-only retained-array/shadow-column transition templates and CLI now exist.
  A bounded PG16 whole-table copy/swap generator now recreates supported simple
  columns/defaults/constraints/indexes/ACLs/sequences in fully commented review
  artifacts. Implement remaining dependency families and supported online workflows,
  then qualify actual PostgreSQL/Neon operation separately from local import.
  Both old embedding and modern pgvector own a database-wide hnsw access
  method, so naïve side-by-side installation is invalid. Old m100/efconstruction16
  defaults and the old3/5 example violate modern ef_construction>=2*m; report
  retuning rather than silently changing values. Inventory dependencies and avoid
  automatic CASCADE/destructive cleanup.
- Preserve real[] finite binary32 values and NULL distinctions, explicit dimensions,
  primary keys and committed batch cursors. Verify exact distances and ANN recall
  separately. Legacy array Manhattan<~> becomes dense<+> only with typed legacy
  evidence; modern bit<~> remains Hamming.
- Strict distinct half/bit/sparse memory values/codecs and metrics now exist;
  BIT, HALFVEC and SPARSEVEC typed storage/SQL/driver/exact ranking, explicit half
  loss reports and bounded transforms are integrated. Typed HNSW half/bit/sparse
  and supported half/bit IVF now use version2 identities while preserving dense
  v1 bytes and public struct layouts. Sparse IVF, IVF L1 and bit Jaccard remain
  unsupported combinations. Retain half rounding and O(nonzeros) sparse contracts;
  quantized/half approximate search requires explicit lossy/rerank semantics.
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

## Exceptional ownership cleanup

Shared-session failed-open cleanup now uses 64 bounded strongly retained groups
and a deliberate RetryOpenCleanup route. Nested owners retire before index/DMS,
WAL and main protection. A separate native WALShared successful-close increment
now covers public DB.Close retry and driver disposal with1024 live+pending
reservations and explicit discovery/retry. Encrypted constructor/selection/copy/session ownership now has its own bounded
managed scope and dedicated discovery/retry API. Plain rollback Reader/Tx and
exclusive-WAL ownership still require their own corrections; these scopes are
not silently treated as covered. Terminal retention may last for process lifetime. Ordinary
and memory checks do not qualify real unmap/close failures.

A later source review found plain Windows metadata probes outside this ownership
model: generic os.Stat/Lstat/SameFile can internally create handles with discarded
close outcomes, and native VFS probes did not retain unresolved closes. The
whole live metadata path was subsequently corrected with managed/bounded probes,
including public Open and poison propagation. Native runtime remains unverified;
standard earlier cross-build/memory-ledger tests did not cover this correction.
No data-loss or ordinary-lock-failure claim has been established.

Windows encrypted modes0/1/shared2 now have native managed coordination code.
Darwin encrypted coordination now shares plain/encrypted inode ownership. Keep
Windows shared rekey, Darwin Unicode inventory and platform runtime/durability
acceptance as separate open requirements. A cross-build does not
close those gates. Preserve current encrypted bytes and exact lifecycle rules.
