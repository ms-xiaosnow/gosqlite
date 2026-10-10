# Offline PostgreSQL transition review

Status: review_required. Plan `fe8daaa375ab4e5a2fe42619d7d9c96a92c857a454e47c35ba0af554b37a0a3b`. No PostgreSQL command has been run.

Approach: `retain_real_array_cast`; transaction mode: `single_transaction`.

Only 00-preflight.sql contains active read-only SQL. It is an operator-reviewed diagnostic reference, not an automatic step. Every later SQL file is fully line-commented, including its SELECT/BEGIN/COMMIT statements. No flag or supplied approval changes that. The JSON renderer does not run or write artifacts.

Read report.json, report.md and inventory.json first. Any blocked gate suppresses all proposed migration statements. Unresolved gates are mandatory human approvals and external evidence, never automatically satisfied by this generator. A synthetic catalog is not proof of a server. Manifest hashes identify bytes, not authenticity. The manifest lists all other artifacts, not itself.

## Required order

1. Verify source backups and old-extension reinstall/restore availability. Pause application writes and DDL, including primary-key changes, and capture legacy exact-query/business baselines before removing embedding. Audit every dependency and arbitrary application/dynamic SQL.
2. Review and run the bounded-time read-only preflight. Array profiling is an exhaustive potentially expensive read. Recollect a complete catalog; compare its canonical_json_v1 hash and dependency hash with this plan. Diagnostics alone are not the full fingerprint. Schema drift, incomplete visibility, bad rows or unknown target availability means stop and regenerate.
3. Obtain every report approval. Review privileges, extension installation policy, disk/WAL capacity, lock cost and maintenance duration. Tables and extensions need appropriate ownership as well as ordinary DML privileges.
4. Review phase 10: exact legacy index drops, extension RESTRICT removal, target creation and exact post-install capability inspection. Both extensions own database-wide hnsw; schema isolation does not solve this collision. Do not use CASCADE, rename access methods or modify catalogs.
5. Review copy, batch and index phases. Keep source real[] arrays intact. Normal CREATE INDEX may share a transaction; concurrent creation must be outside every explicit transaction and is incompatible with the single-transaction mode.
6. Inspect valid/ready index flags and exact definitions, counts, PKs, NULL alignment, float32 values, norms, exact-distance/business fixtures, ANN recall and real staging query plans before cutover. This package has not performed these PostgreSQL checks.
7. Approve coordinated application SQL, vector-text binding, settings, column choice, NULL/zero policy, tie order and filter rollout. An old Manhattan <~> must become dense <+>; modern bit <~> is Hamming. See the separate AST converter for supported source-span conversions, never global replacement.
8. Preserve original arrays and export for the rollback window. Future writes must keep arrays current. No source rename, source drop, table drop, cleanup or generic dual-write trigger is generated.

## One offline transaction

Phases 10, 20, 40, 50 and the commit part of 60 share one connection and one still-open transaction. Phase 10 takes ACCESS EXCLUSIVE on the selected table. Review statements, resolve post-install/data checks and prepare application deployment before committing. Do not execute a COMMIT merely because the file exists. Phase 30 is unused. Normal ROLLBACK before COMMIT reverses that transaction's DDL/data changes; it does not undo external application deployment. A large transaction retains locks and WAL/disk costs throughout; sub-batching inside it would not release them.

## Retained-array approach

No shadow or backfill is created. New indexes use an explicit vector(n) cast expression and queries must use that same expression and verified operator schema. Removing embedding still removes old distance functions/operators and ANN service; retained data alone does not preserve that service. Every new array write must remain castable.

## Rollback boundaries

Before the single transaction commits, database ROLLBACK is transactional. After any migration commit, stopping shadow reads only abandons the data-copy workflow; it cannot restore the dropped old extension or old indexes. After application cutover, stop writers and reconcile any newer data with retained arrays before switching readers. Restoring old ANN may require a separate database/branch restore or reviewed reverse migration, available old binaries and removal/reconciliation of vector-owned dependencies. hnsw collides in the reverse direction too. No automatic post-cutover rollback is promised. Cleanup requires a separate approval and plan.

## Pinned evidence

Source: pg_embedding-0.3.6@5d48508aeaeb86b9e9c630468ada7a124b905795. Target: pgvector-0.8.7@f37c13f68b57d2c3472b2214fbcff699d6d34876. Current Neon page retrieved 2026-10-10; requested migration chapter is absent. Historical official chapter is a separate source: https://github.com/neondatabase/website/blob/d4c1cf37f79acbf4c02880eddb97dd9a36cb2d20/content/docs/extensions/pg_embedding.md#migrate-from-pg_embedding-to-pgvector. Neither current Neon availability nor a live PostgreSQL execution is claimed.
