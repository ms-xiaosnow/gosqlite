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
  pinned copy/checkpoint and quiescent rekey. Implement non-Linux encrypted
  ownership/coordination; the plain native ports do not supply that code.
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

- Bounded strict catalog/real-array inspection and typed PostgreSQL-subset
  AST/source-span conversion now exist, with explicit local and review-only
  PostgreSQL targets. Add resumable CSV/NDJSON import and exhaustive value
  verification. Preserve strict source underflow/identity/bind contracts; no
  global operator replacement or automatic source-error coercion.
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
- Strict distinct half/bit/sparse memory values/codecs and metrics now exist;
  BIT and HALFVEC typed storage/SQL/driver/exact ranking, explicit half loss
  reports and dense/half transforms are integrated. Add sparse typed
  storage/SQL/driver and exact search, then
  versioned supported ANN identities while preserving dense
  GSVv1. Retain exhaustive half rounding and O(nonzeros) sparse contracts;
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

Add a bounded synchronous pending-retirement group for shared-session failed-open
cleanup when mapping retirement fails. Keep DMS/main locks until every view is
retired, never evict live ownership, and expose a deliberate retry. Current
fail-closed resource retention can last for the process lifetime; ordinary tests
and memory state models do not qualify real unmap/close failures.
