# pg_embedding inspection

Status: review\_required

Source profile: pg\_embedding-0.3.6@5d48508aeaeb86b9e9c630468ada7a124b905795

Target reference: pgvector-0.8.7@f37c13f68b57d2c3472b2214fbcff699d6d34876

Catalog SHA-256: bdb6e93d8e95112231c892ad5a6653ca72ccd66297efa99446ac0f2610b8f804

## Columns
- "public"."documents"."embedding": legacy\_vector; selected=true; dimensions=3; safe; validation=exhaustive
- "public"."documents"."ordinary\_samples": unrelated\_real\_array\_candidate; selected=false; dimensions=0; safe; validation=not\_run

## Indexes
- "public"."custom nearest": safe; identity verified=true; old m=16, efconstruction=64, efsearch=64
- "public"."documents\_cosine": safe; identity verified=true; old m=16, efconstruction=64, efsearch=64
- "public"."documents\_manhattan": safe; identity verified=true; old m=16, efconstruction=64, efsearch=64

## Findings
- warning / efsearch\_scope\_change / "public"."custom nearest": Legacy per-index efsearch becomes an explicit PostgreSQL session/transaction setting or local VectorSearchOptions.EFSearch, never a CREATE INDEX option.
- warning / efsearch\_scope\_change / "public"."documents\_cosine": Legacy per-index efsearch becomes an explicit PostgreSQL session/transaction setting or local VectorSearchOptions.EFSearch, never a CREATE INDEX option.
- warning / efsearch\_scope\_change / "public"."documents\_manhattan": Legacy per-index efsearch becomes an explicit PostgreSQL session/transaction setting or local VectorSearchOptions.EFSearch, never a CREATE INDEX option.
- warning / hnsw\_access\_method\_collision / access\_method.hnsw: Both pinned extensions own database-wide hnsw. A side-by-side unmodified extension installation is not a valid in-place migration plan.
- warning / application\_sql\_inventory\_incomplete / scope: Catalog inspection cannot discover arbitrary application or dynamic SQL; supplied SQL corpus is not yet converted by this inspector.
- warning / current\_neon\_chapter\_absent / source: Current Neon target page was retrieved on 2026-10-10; its requested migration chapter is absent. The official historical 2024-01-23 chapter is a separate reference, not current-page conformance.
- warning / numeric\_semantics\_difference / source: Legacy distances return architecture-sensitive float32; local metrics use compensated float64, and cosine rejects zero norms. Exact result-bit equality is not claimed.

## Approval points
- Retain source arrays and immutable export; retuning index parameters requires explicit review and new recall measurements.
- Review maintenance-window or separate-database strategy for the database-wide hnsw access-method collision.
- Review target installation/version, privileges, dependency inventory and application query changes before PostgreSQL execution.

## Transition gates

Plan: fe8daaa375ab4e5a2fe42619d7d9c96a92c857a454e47c35ba0af554b37a0a3b

Status: review_required; PostgreSQL execution has not occurred.
- application\_cutover / unresolved: Approve query/parameter/operator/column/settings/NULL/zero/tie/filter changes together, then cut over explicitly. Maintain retained arrays on every later write or reconcile before rollback.
- cleanup\_separate / unresolved: Keep source arrays, export and migration state through an agreed rollback window. Cleanup and post-cutover old-service restoration require new operational plans; no cleanup statements are generated.
- data\_and\_training / unresolved: Revalidate all source rows and PK identity, then full NULL/value/norm alignment and exact distance/business fixtures. Review IVF training/list/probe choices or HNSW retuning; evaluate ANN recall separately.
- extension\_transition / unresolved: Approve each inventoried index drop and DROP EXTENSION embedding RESTRICT individually. Both extensions own database-wide hnsw. Unexpected dependencies mean stop, never CASCADE or catalog/AM renaming hacks.
- fresh\_catalog / unresolved: Recollect the complete catalog and dependencies under write quiescence; compare canonical\_json\_v1 catalog and dependency hashes. Any drift requires regeneration. Preflight diagnostics do not compute the full Go catalog hash.
- maintenance\_window / unresolved: Approve a maintenance window with all application writes, DDL and primary-key changes paused from validation through cutover. Row locks between batches do not enforce this contract. Separate-database/branch transition is a different plan.
- privileges\_capacity\_locks / unresolved: Verify table/extension ownership and required CREATE/USAGE/SELECT/UPDATE rights, RLS visibility, disk/WAL headroom, lock and downtime budgets; extension installation may require a provider administrator.
- snapshot\_and\_old\_service / unresolved: Verify immutable export/backup, restoration procedure, original definitions, and actual availability of embedding 0.3.6 binaries before promising old-ANN rollback.
- target\_capabilities / unresolved: Verify exact available and installed vector 0.8.7, extension schema, extension-owned hnsw/ivfflat, real\[\] casts, vector type, operators/functions/opclasses and option bounds before copy/index/application use.
