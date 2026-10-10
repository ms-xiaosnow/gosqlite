# Bounded whole-table copy and swap (review-only)

`GenerateFullTableTransition` and the explicit `--full-table-evidence` CLI route
implement a separate typed table, copy every supported column, compare all values,
rename the old table to a retained backup, and recreate supported contracts on the
new table under the original name. This is actual whole-table generation, not a
shadow-column alias. No database is opened and no PostgreSQL SQL is executed.
All mutation and post-install SQL remains fully line-commented; all human gates
remain unresolved. `blocked` bundles contain no proposed mutation statements.

This profile is PostgreSQL **16 only**, with the existing pinned embedding 0.3.6
and vector 0.8.7 profiles. It has synthetic generator/parser/JSON/CLI evidence,
not live PostgreSQL or Neon qualification, server availability or authorization.

## Run the fixed synthetic example

From the repository root, with the pinned offline toolchain:

```sh
export GOTOOLCHAIN=local GOPROXY=off GOSUMDB=off CGO_ENABLED=0
export GOCACHE="$PWD/../gocache" GOPATH="$PWD/../go-path"
GO=../toolchains/go1.27.2/bin/go
"$GO" run ./examples/migration-full-table

"$GO" run ./cmd/gosqlite migrate pgembedding transition \
  --catalog examples/migration-full-table/catalog.json \
  --options examples/migration-full-table/options.json \
  --full-table-evidence examples/migration-full-table/evidence.json
```

The inputs and every golden artifact are checked in. The fixture includes integer
sequence allocation, ordinary text and array columns, defaults, PK/UNIQUE/CHECK,
an ordinary descending btree, three mapped ANN indexes, comments, owner/table and
column ACLs, default privileges, internal row types and TOAST dependencies.
Its completeness declarations are synthetic, not measurements of a server.

`--report new-review.json` retains the existing exclusive-create output policy;
existing files are refused. No apply/resume/credentials/server flags are accepted.
There is no automatic artifact publication or interrupted-write qualification.

## APIs and unchanged earlier contracts

- `DecodeFullTableOptions(io.Reader) (FullTableOptions, error)`
- `DecodeFullTableEvidence(io.Reader) (FullTableEvidence, error)`
- `GenerateFullTableTransition(context.Context, CatalogSnapshot,
  FullTableOptions, FullTableEvidence) (TransitionBundle, error)`
- Existing `RenderTransitionJSON` renders the same artifact-envelope shape.

Catalog v1, `TransitionOptions`, the retained-array/shadow APIs, old CLI dispatch
without the new flag, and previous golden artifacts remain unchanged. The new
options wrap an explicit `transition` with approach `full_table_copy_swap`,
`single_transaction`, no shadow/batch/concurrent settings, copy/backup table names
and one explicit archive name per ordinary source index. Legacy ANN indexes are
removed by their already-reviewed mappings, not archived as usable old service.

The supplement uses format 1 / `gosqlite-pg16-full-table-v1`. It binds the exact
base catalog's `canonical_json_v1` SHA-256, extraction timestamp, server number,
and source/target profiles. It requires `deparse_search_path: "pg_catalog"` and
`standard_conforming_strings: true` for captured expression interpretation.
Every exported field and array is required. Duplicate/unknown/missing keys,
invalid UTF-8/NUL/surrogates, excessive depth/tokens/bytes and bad identities fail
closed. Options cap is 64 KiB, evidence 2 MiB, generated artifact content 8 MiB
(or a lower `max_bytes`). Programmatic evidence is budgeted before serialization.
Supplement ordering remains part of captured evidence identity. Operator index
mapping and archive-name order are canonicalized without changing caller input.

## Collect facts, not approval checkboxes

The original catalog does not contain complete owner/default/constraint/ACL
facts. The supplement supplies typed, OID-bound records for those facts, not raw
SQL to execute. All base dependency records must appear unchanged in its complete
union; it may add missing whole-table dependencies. Both catalog and supplement
hashes identify the plan. Hashes cannot establish authenticity or live completeness.

Supply complete coverage for table/column properties, defaults, constraints,
indexes, sequences, roles, privileges, creation privileges, dependencies, triggers,
rules, views, functions, policies, publications, security labels, inheritance,
statistics, comments, internal storage, namespaces, database-wide event triggers and legacy operator-family/support rows. Empty inventories require explicit
complete coverage; omitted/incomplete coverage is a gate, never inferred absence.
Unsupported objects must be inventoried with class/OID/sub-ID, kind, identity and
original definition. Unmapped dependencies generate object-address-specific gates.

Read-only `00-preflight.sql` is a collection/reference aid, not an automatic JSON
collector or exhaustive server-side fingerprint. Collect all records with owner
visibility in one quiescent snapshot; include source composite/array types and
internal TOAST objects in the base catalog. Include dependencies of owned sequences
and row types, not just direct extension dependents. Incoming views/FKs/defaults,
arbitrary application SQL and cached/prepared OIDs cannot be made safe by names.
Use the complete original catalog export plus the supplement, rather than editing
coverage flags to make an incomplete export look complete.

Expand effective table/column ACLs by grantor/grantee OIDs. A NULL catalog ACL
means PostgreSQL's `acldefault`, not zero grants. Capture the table owner's effective
new-table default ACL for that schema (global and per-schema default privileges
combined), separately from the source ACL. `creation_privileges` is that expanded
result. OID 0 denotes PUBLIC; other roles require exact OID/name records. Resolve
zero `reltablespace` to its effective existing tablespace before supplying the
bounded `pg_default` identity (1663). Do not substitute a current session's defaults.

The `namespaces` list binds captured schema OIDs to names, including distinct source
and legacy-extension schemas. Mandatory non-pinned dependencies cannot be omitted
from both inputs to manufacture absence. `kind` records use these exact categories:

- `namespace`: NORMAL dependency from the source heap, each owned sequence, old
  extension, and old extension-owned function/operator/opclass/family to its
  captured namespace. Dependent row types, row arrays and TOAST inherit namespace
  dependence indirectly and require their explicit internal ownership links.
- `legacy_implementation`: NORMAL old access-method-to-handler or
  operator-to-implementation-function link, matched to the exact base object OIDs.
- `operator_family`: AUTO opclass-to-family and family-to-access-method links;
  INTERNAL support-row-to-opclass links; NORMAL support-row-to-implementation
  operator/function links. The PG16 `float_ops` sort family is pinned to OID 1970.
- `extension_member`, `row_type`, `internal_storage`, `default`, `constraint`,
  `sequence`, and `index` retain their typed object-specific meanings. Exact
  mandatory links are checked in addition to classifying every supplied edge.

These categories describe catalog facts, not permission to drop arbitrary objects.
Do not synthesize ordinary `pg_depend` rows to built-in pinned objects; PostgreSQL
normally omits them. Unknown endpoints, schema mismatches, incomplete namespace
identity or missing mandatory links block. Database-wide event triggers are
inventoried separately from table `pg_trigger`; any presence is unsupported even
when disabled. Missing event-trigger coverage blocks DDL proposals.

## Preservation matrix

| Contract | Generated behavior | Concrete gates |
|---|---|---|
| Heap identity | New permanent heap under original owner; same schema; explicit copy/backup names; retains old table and its data | Other server versions, RLS/forced RLS, inheritance/partitions, reloptions, nondefault replica identity/tablespace, absent owner/row-type facts |
| Every ordinary column | Explicit ordered INSERT SELECT for all columns; preserves names, built-in types, NOT NULL and ordinary array dimensions; no SELECT * | Dropped/gapped attributes, generated/identity columns, typmods, unknown/domain/composite types |
| Types | bool, bytea, int2/int4/int8, text, float4/float8, unchanged real[]; only selected array becomes vector(n) | Selected default/constraint/index uses needing an unimplemented type-semantic rewrite |
| Column properties | Exact default text collation, storage, compression setting, statistics target, comments | Custom collation, attribute options, invalid/nonstandard fixed-width storage |
| Defaults | Strict typed scalar NULL/literal or exact owned integer-sequence nextval(regclass), rebound with qualified names | Functions/arithmetic/casts outside whitelist, altered expressions, shared/external/identity sequences; raw SQL is never interpolated |
| Constraints | Matching immediate PK/UNIQUE and backing indexes with standalone PG16 `connoinherit=true`; UNIQUE NULLS NOT DISTINCT; bounded scalar comparison or IS [NOT] NULL CHECK, validation/NO INHERIT/comment preserved | FK/exclusion/deferrable/internal-trigger semantics; complex checks; converted-vector references; mismatched ownership/keys, PK NULLS NOT DISTINCT |
| Ordinary indexes | Explicitly archives old names, recreates simple built-in default btree with uniqueness, ordering, NULL position, tablespace/comment | Expression/partial/INCLUDE/storage options; custom method/opclass or mismatched schema; invalid/not-ready/not-live/non-immediate; clustered/replica-identity indexes |
| Legacy family internals | Explicit pinned opclass/family/pg_amop/pg_amproc OID/strategy/function/sort-family tuple and dependency edges, removed with reviewed extension | Missing or mismatched tuple/ownership edges; unknown family members |
| ANN | Existing identity-verified source metric and explicit target parameter mapping; simple typed target column | Existing source/target profile, shape, parameter, dimension, unmapped-index and method gates still apply |
| Sequence | Retains the original sequence OID, state/options/owner/ACL; no nextval during copy, setval, restart or recreation; rebinds OWNED BY | Missing same-schema/same-owner serial ownership/default edges, other consumers, unsupported sequence contracts |
| Ownership and ACL | Create as original owner; clear captured default/source recipients and PUBLIC/owner privileges; reconstruct exact owner-origin table/column grants and grant options | Unknown role, non-owner grantor chain, unsupported privilege, missing effective default ACL, PUBLIC grant option |
| Internal row types / TOAST | Original internal identities retained with backup; PostgreSQL creates new internal storage/type identities with new heap; exact storage dependency facts recognized | Missing/mismatched internal identities, nondefault TOAST options, extra TOAST indexes or external consumers |
| Other dependents | Report exact class/OID/sub-ID/reference and withhold all mutation SQL | Views/materialized views, functions, user triggers, rules, policies, publications (including schema/all-table), security labels, extended statistics, all unmapped edges |

The coverage promise is captured evidence, not a claim that all PostgreSQL object
kinds are supported. Unsupported presence blocks instead of silently omitting it.

## Phase order and rollback

There are 14 artifacts: report, base inventory, supplement, explicit full options,
README, manifest, read-only preflight, and seven commented SQL phases.

1. Pause readers/writers/DDL and all sequence consumers. Verify backup/export,
   exact live identities, target capabilities, privileges, capacity, locks and
   old-extension restoration. Recollect and compare hashes under quiescence.
2. Phase 10 starts one transaction, locks the source, drops only planned old ANN
   indexes, removes embedding with RESTRICT and proposes pinned target installation.
3. Phase 20 creates a genuinely separate table as the source owner and copies all
   explicit values. It never evaluates a default or advances a sequence.
4. Phase 30 compares source/copy row/key coverage, every column's binary-send
   representation, NULLs and vector dimensions. Ordinary real[] binary comparison
   retains dimensions/lower bounds/NULL elements; selected arrays satisfy the
   existing exhaustive finite-vector profile. Stop on every mismatch.
5. Phase 40 renames ordinary source indexes to explicit archive names; renaming a
   constraint-backed index also renames its constraint. It renames source to backup
   and copy to source, rebuilds constraints/indexes and restores defaults/ACLs/comments.
   It removes owned-sequence defaults from the backup and moves ownership to the
   new table. The backup is deliberately an archive, not an interchangeable writer.
6. Phase 50 inspects the new table by qualified name/new OID, never the old OID.
   Compare all supplied contracts and actual exact-distance/business/ANN results.
   Retire all prepared statements and cached OID bindings; reconnect applications.
7. Only after every gate, phase 60 proposes COMMIT and separate typed query examples.

All phases share one potentially large offline transaction. Before its COMMIT,
ROLLBACK on that same connection reverses transactional DDL/data changes. Sequence
use by independent clients is not rolled back; quiescence is an external gate.
After COMMIT, the retained backup is stale on subsequent writes. Reverse migration,
new-write reconciliation, old-ANN restoration and cleanup need separate plans.
There is no table drop, CASCADE, generic dual-write, sequence reset or instant
post-cutover rollback. No operational claim is proven by generated SQL alone.

## Primary references (PostgreSQL 16)

- [CREATE TABLE](https://www.postgresql.org/docs/16/sql-createtable.html)
  and [ALTER TABLE](https://www.postgresql.org/docs/16/sql-altertable.html)
- [CREATE INDEX](https://www.postgresql.org/docs/16/sql-createindex.html)
  and [ALTER INDEX](https://www.postgresql.org/docs/16/sql-alterindex.html)
- [ALTER SEQUENCE](https://www.postgresql.org/docs/16/sql-altersequence.html)
  and [GRANT](https://www.postgresql.org/docs/16/sql-grant.html)
- [Dependency catalog](https://www.postgresql.org/docs/16/catalog-pg-depend.html)
- [PG16 access-method source](https://github.com/postgres/postgres/blob/REL_16_STABLE/src/include/catalog/pg_am.dat),
  [opclass source](https://github.com/postgres/postgres/blob/REL_16_STABLE/src/include/catalog/pg_opclass.dat)
  and [TOAST creation source](https://github.com/postgres/postgres/blob/REL_16_STABLE/src/backend/catalog/toasting.c)

- [PG16 constraint-index source](https://github.com/postgres/postgres/blob/REL_16_STABLE/src/backend/catalog/index.c),
  [operator-class/family dependencies](https://github.com/postgres/postgres/blob/REL_16_STABLE/src/backend/commands/opclasscmds.c),
  [row-type dependencies](https://github.com/postgres/postgres/blob/REL_16_STABLE/src/backend/catalog/pg_type.c)
- [Event-trigger behavior](https://www.postgresql.org/docs/16/event-trigger-definition.html)
  and [operator precedence](https://www.postgresql.org/docs/16/sql-syntax-lexical.html#SQL-PRECEDENCE)

References were read on 2026-10-10; stable-branch source links identify the profile,
not an independently pinned PostgreSQL executable or live execution oracle. No
third-party implementation was copied into the Go generator. See the existing
migration-transition guide for pinned extension commits and the absent current
Neon migration chapter. Restricted durability/security/release gates remain open.
