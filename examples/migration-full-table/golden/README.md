# Whole-table transition review

Status: review_required. Plan `38ffcc99a8d56321f69fdf307b4609d8692177d5d6dbe2a2d9d5915f541b3b75`. Catalog `9a28597fd6836c6f790a17e33b851d921be04ea78f36da145850255e9c982cf7`. Supplement `2b96912def81f3b2ee0c1c61a2d43840c5ecf2b70b901586bfab89da870c8f35`.

This is an offline PostgreSQL 16 / embedding 0.3.6 to vector 0.8.7 text generator. No SQL has been executed, no server contacted and no gate approved. Only 00-preflight.sql is active read-only SQL; every other SQL line is commented. Do not remove comments without a separately approved operational plan. Unsupported/missing evidence withholds every proposed mutation.

The full-table supplement is a captured-fact contract, not operator approval. Hashes bind supplied bytes/identity, not authenticity or completeness on a live server. Catalog v1 and the retained-array/shadow-column APIs are unchanged.

## Order and transaction

1. Pause all application reads/writes, DDL and sequence consumers. Verify immutable export/backup, restoration and availability of old extension binaries. Recollect all complete inventory categories with owner visibility in the same snapshot and compare both source hashes. Arbitrary application/dynamic SQL still needs an external inventory.
2. Review every report gate, ownership/role membership/default ACL, provider extension-install policy, target capability, disk/WAL capacity and lock/downtime budget. Full-table mode has one potentially large offline transaction; no online or multi-transaction copy/swap is supported.
3. Phase 10 starts the shared transaction, locks the source, removes only the explicitly mapped legacy indexes and uses DROP EXTENSION RESTRICT before installing/inspecting the pinned target. Unexpected dependents mean stop, never CASCADE.
4. Phase 20 creates an actual separate typed table as the original owner, preserves every supported ordinary column, and INSERT SELECT copies all columns explicitly. No default or sequence is evaluated. Phase 30 checks every column's binary send representation, NULLs, row/key coverage and vector dimensions against the retained source. Stop on any mismatch.
5. Phase 40 archives original ordinary index names, renames source to the explicit backup and copy to the original name, recreates typed constraints/indexes, restores defaults/comments/ACLs, and rebinds the SAME existing sequence OIDs to the new table. Backup sequence defaults are removed so the archive is not a supported writer. Sequence options/state/owner/ACLs are retained without nextval/setval/reset.
6. Compare phase-50 new-OID catalog facts with every supplied contract, plus staging exact-distance/business results and ANN recall/plans. Do not use the old OID to check the new target. Retire prepared statements, cached OIDs and all old application sessions; renaming alone does not rebind dependencies.
7. Only after all checks and separate application approval, phase 60 proposes COMMIT and typed query examples. The backup becomes stale immediately on later writes. Retain it and export; any post-commit reverse migration, sequence reconciliation or old-ANN service restoration needs a new plan. No source table drop or cleanup is generated.

## Supported versus blocked

Supported: complete PG16 ordinary permanent heap; exact built-in scalar/real-array identities; all explicit column values; preserved NOT NULL, storage/compression/statistics/comments; bounded typed-literal and owned integer-sequence defaults; immediate primary/unique constraints, bounded scalar CHECK expressions, simple built-in btree and explicitly mapped ANN indexes; original owner, owner-origin table/column ACLs with grant options and explicit default-ACL clearing; unchanged sequence object with ownership rebound; explicitly inventoried internal row types and default TOAST storage recreated by PostgreSQL with new identities.

Blocked: unsupported/missing type/default/constraint/index/ACL facts; foreign/exclusion/deferrable constraints and associated triggers; identity/generated/dropped/typmod columns; custom collations/storage options; views/materialized views/functions/user triggers/rules/RLS/policies; inheritance/partitions/publications/security labels/extended statistics; shared/external sequences; non-owner grant chains; any unmapped incoming/outgoing dependency. No raw catalog definition is interpolated into proposed SQL. All absence claims require complete inventory evidence.

- application\_cutover / unresolved: Approve new relation OID, original-name typed columns, vector parameter/operator/settings changes and retire all cached/prepared OID bindings. Backup arrays become stale after later writes; reconcile before any reverse migration.
- cleanup\_separate / unresolved: Retain the backup table, its ordinary indexes and immutable export through an approved rollback window. Its owned sequence defaults are removed and sequence ownership is rebound to the new table. No cleanup/reverse DDL is generated.
- data\_and\_training / unresolved: Revalidate all source rows and PK identity, then full NULL/value/norm alignment and exact distance/business fixtures. Review IVF training/list/probe choices or HNSW retuning; evaluate ANN recall separately.
- extension\_transition / unresolved: Approve each inventoried index drop and DROP EXTENSION embedding RESTRICT individually. Both extensions own database-wide hnsw. Unexpected dependencies mean stop, never CASCADE or catalog/AM renaming hacks.
- fresh\_catalog / unresolved: Recollect the complete catalog and dependencies under write quiescence; compare canonical\_json\_v1 catalog and dependency hashes. Any drift requires regeneration. Preflight diagnostics do not compute the full Go catalog hash.
- full\_table\_acl\_and\_roles / unresolved: Verify role identity/membership, same-owner creation privileges, complete current default ACLs, storage headroom and backup access. Review table/column ACL reconstruction and retained sequence access. No permissions have been changed.
- full\_table\_rebinding / unresolved: Approve the new relation OID, explicit copy/backup/archive names, all recreated constraints/indexes/ACLs, owned-sequence rebinding and prepared-statement/application reconnect. Names alone do not rebind old OIDs; prove no uncaptured external dependencies.
- maintenance\_window / unresolved: Approve a maintenance window with all application writes, DDL and primary-key changes paused from validation through cutover. Row locks between batches do not enforce this contract. Separate-database/branch transition is a different plan.
- privileges\_capacity\_locks / unresolved: Verify table/extension ownership and required CREATE/USAGE/SELECT/UPDATE rights, RLS visibility, disk/WAL headroom, lock and downtime budgets; extension installation may require a provider administrator.
- snapshot\_and\_old\_service / unresolved: Verify immutable export/backup, restoration procedure, original definitions, and actual availability of embedding 0.3.6 binaries before promising old-ANN rollback.
- target\_capabilities / unresolved: Verify exact available and installed vector 0.8.7, extension schema, extension-owned hnsw/ivfflat, real\[\] casts, vector type, operators/functions/opclasses and option bounds before copy/index/application use.

## Sources and qualification

PostgreSQL 16 CREATE/ALTER TABLE, CREATE INDEX, ALTER SEQUENCE and GRANT reference semantics; pinned extension profiles from the existing migration guide. This bundle has only synthetic Go generator/JSON/CLI qualification, not live PostgreSQL or Neon execution. No current Neon support/availability is inferred from the historical migration chapter. Restricted durability/security/release qualification remains separate.
