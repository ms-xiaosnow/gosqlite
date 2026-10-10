# Review-only PostgreSQL transition bundles

M9 checkpoint D implements bounded offline bundle generation. It does not
execute PostgreSQL SQL, prove that Neon offers an extension version, or complete
the full M9/release qualification. Local CSV/NDJSON import is a separate workflow.
The library, standalone example and `gosqlite migrate pgembedding transition`
CLI are implemented. See [the CLI guide](migration-transition-cli.md) for bounded
options decoding and review-artifact output. No interface executes generated SQL.

## Generate the synthetic examples

From the repository root, using the pinned toolchain:

```sh
export GOTOOLCHAIN=local GOPROXY=off GOSUMDB=off CGO_ENABLED=0
export GOCACHE="$PWD/../gocache" GOPATH="$PWD/../go-path"
GO=../toolchains/go1.27.2/bin/go

# Default output is a JSON envelope containing the named review artifacts.
"$GO" run ./examples/migration-transition -approach shadow -mode batches
"$GO" run ./examples/migration-transition -approach retain -mode single

# Optional: create a NEW review directory; existing destinations are refused.
"$GO" run ./examples/migration-transition -approach shadow -mode batches \
  -out-dir /tmp/my-new-transition-review
```

This example intentionally uses `testdata/pgembedding-0.3.6/catalog.json` and
explicit synthetic OIDs, names, dimensions and target parameters. It is not a
general live migration command. Use the library with a separately collected
real catalog and explicit reviewed options for a real plan. The fixture's
claimed exhaustive profile and complete coverage are synthetic evidence, not
measurements. Its three HNSW source indexes use the valid 16/64 pair.

The explicit output-directory option creates a directory and then writes each
artifact exclusively with private permissions. It never overwrites an existing
directory or file. A write error returns failure and can leave a partial review
directory; atomic/interrupted-publication safety is not claimed or tested.
The transition generator/renderer have no file-writing, database, network or
subprocess execution path; the package's separate local-import APIs are not
invoked by this workflow.

## Native API

- `GenerateTransition(context.Context, CatalogSnapshot, TransitionOptions)
  (TransitionBundle, error)`
- `RenderTransitionJSON(io.Writer, TransitionBundle) error`

`TransitionOptions` explicitly identifies one relation, one real[] attribute,
its dimensions, approach, transaction mode, intended vector schema/version,
and target index mappings. Each index maps a positively identified source index
OID to a distinct target name and an explicit method/configuration. Metrics
come from verified source opclass/operator identity; names alone are inadequate.
The existing catalog input bounds apply before inspection. Generated artifact
contents together are capped at 8 MiB; `MaxBytes` may lower this limit. At most
256 explicit index mappings are accepted. The JSON envelope adds bounded
metadata and JSON escaping overhead beyond the artifact-content limit.

The two original approaches (unchanged APIs and artifact layouts) are:

1. `RetainArrayCast` (`retain_real_array_cast`): retain the real[] column and
   build expression indexes over an explicit, schema-qualified vector(n) cast.
   Application queries must use the same expression. No data-copy phase occurs.
2. `TypedCopyShadow` (`typed_copy_shadow_column`): add one nullable vector(n)
   shadow column to the existing table, copy values, and explicitly switch
   application readers to that column. The source real[] column remains intact.

The second approach is deliberately narrower than the historical Neon
full-table-copy recipe. These original approaches never rename/drop source
columns/tables or invent dual-write triggers. A separate identity-bound supplement
API and explicit CLI route now implement a bounded PostgreSQL 16 whole-table
copy/swap, supported default/constraint/index/ACL/sequence recreation, and concrete
gates for other dependents. See [the full-table guide](migration-full-table.md)
for its different phases and retained-backup/sequence effects. Neither approach
treats stale retained data as a rollback guarantee.

`TransitionSingle` (`single_transaction`) holds one offline transaction across
extension transition, shadow copy, ordinary indexes and validation. Phase 60
contains the explicitly gated final commit. This requires one connection and
has substantial lock, transaction and WAL costs. It rejects concurrent indexes.

`TransitionBatches` (`multi_transaction`) commits the extension transition and
shadow/state setup separately. For typed copy it requires explicit state
schema/table and batch size 1..100000. Each backfill transaction locks a
plan-bound progress row and atomically advances it with the changed source rows.
Application writes, DDL and PK changes must remain externally paused between
transactions. A per-batch row lock alone does not enforce this contract.

The initial progress cursor is SQL NULL, so the smallest signed 64-bit key is
included. NULL source arrays still advance progress. The batch uses keyset
ordering, no OFFSET, no SKIP LOCKED and no shadow-IS-NULL predicate. It compares
the previous cursor/count/batch sequence, returns `batch_rows`, and marks complete
only after an empty batch with the expected total copied. An empty batch with
`complete=false` or a missing progress row means stop and inspect. Inspect
persistent progress before retrying after an ambiguous client response; printed
output is not proof of COMMIT. Parallel workers and interrupted execution
qualification are not implemented.

## Artifacts and human approvals

Each bundle contains 13 artifacts:

- `report.json`, `report.md`: inspection results, plan identity and unresolved or
  blocked human gates
- `inventory.json`: complete supplied catalog, including exact original index
  definitions, dependencies, profiles and application corpus as JSON evidence
- `manifest.json`: names, sizes, content hashes, kinds and transaction boundaries
  for every other artifact (no self-hash)
- `README.md`: operator order, costs, dependencies and rollback boundaries
- `00-preflight.sql`: active read-only diagnostic reference
- `10-extension-transition.sql`: exact mapped source-index drops, extension
  RESTRICT removal, exact target creation and post-install capability queries
- `20-shadow-copy.sql`, `30-batch.sql`: typed shadow/state setup and single-worker
  batch, or explicit unused-phase explanations
- `40-indexes.sql`: normal or separately marked concurrent target index creation
- `50-verify.sql`: exhaustive source/target count, NULL, dimension, binary32
  value and norm checks plus index validity and business/ANN validation gates
- `60-application-cutover.sql`: explicitly typed prepared-query examples,
  per-query search settings and coordinated deployment checklist
- `70-rollback.sql`: transaction, data-workflow and old-service rollback limits

Every SQL file except `00-preflight.sql` is fully line-commented, including
SELECT/BEGIN/COMMIT. These files also contain human instructions and are not
runnable scripts. There is no render-after-approval or execute flag. Supplying
options never counts as authorization for external changes. Any blocked gate
withholds proposed phase statements; the report, inventory and preflight remain
available for diagnosis. Approval gates are never marked satisfied offline.

Preflight reports installed and available extension versions, database-wide
access-method ownership, selected column/PK metadata and privileges, relation
size, transitive `pg_depend` addresses, index definitions/readiness, inheritance,
triggers, constraints, policies, rules, counts, shape/bounds, NULL elements,
nonfinite values and zero norms. It runs in a read-only repeatable-read reference
transaction with lock/statement timeouts and trusted `pg_catalog` search_path.
Full row profiling may still be expensive. Failed statements, timeouts, missing
rows and unexpected values are stop conditions, not automatically waived gaps.
The reference targets PostgreSQL 12 through 18 catalog syntax; it has not been
executed on those server versions.

The post-install reference queries expose exact vector version/schema, AM and
opclass ownership, input types, operator/function signatures, real[] casts and
operator-family support members. An operator must compare them to the pinned
profile before use. `CREATE EXTENSION IF NOT EXISTS` is deliberately not used:
it would not validate existing capability. Target-version input is a requested
version, never evidence of availability or actual installation.

The catalog's `canonical_json_v1` hash and the hash of its dependency array bind
the plan to all supplied facts. Definitions and dependency additions change
identity. Index mapping order is canonicalized. Recollect the complete source
catalog under quiescence and compare these hashes before migration; the SQL
diagnostics are not a server-side calculation of the Go catalog hash. Actual
catalog additions, missing privileges or stale snapshots require fresh review.
The current baseline catalog collector's declared completeness gaps must be
resolved; they cannot be treated as an eligible complete transition inventory.

## Extension collision and dependency handling

Both pinned extensions define the database-wide `hnsw` access method. Different
extension schemas do not allow unmodified side-by-side installation. A generic
zero-downtime same-database transition is unsupported. Review a maintenance
window or develop a separate-database/branch plan instead.

The one-column generator requires all inventoried old ANN indexes to be covered
by its verified source mappings. An unrelated old index blocks the bundle;
the generator will not silently drop it. Extension-owned members must be
explicitly linked to the selected embedding extension. Other dependencies
without a generated object-specific transition block proposed mutation output.
Raw index definitions, function bodies, dependency SQL and corpus text stay
JSON evidence and are never inserted into executable template positions.

After reviewed, exact mapped index drops, the reference uses
`DROP EXTENSION "embedding" RESTRICT`. Unexpected dependencies must stop that
operation. No CASCADE, catalog modification, access-method rename or generic
dependency recreation is generated. Verify original definitions, backups,
restoration procedure and actual availability of old extension binaries before
promising restoration of the old ANN service.

## Types, indexes and SQL trust boundaries

The existing strict inspector establishes source extension/type/opclass and
function/operator identity. Only its safe selected-column and safe source-index
subset can produce proposed phases. Missing exhaustive profiles, unknown source
identities, incomplete coverage, unknown target versions, existing target
extension, name collisions, unplanned indexes and unhandled dependencies block
generation of proposed mutations.

Identifiers retain exact case/Unicode and double quotes are escaped. Selected
SQL identifiers containing control/line-separator characters are explicitly
unsupported. Catalog comparisons use escape-string literals with quote and
backslash escaping, independently of standard_conforming_strings. Generated
methods/operators/type names come from closed pinned mappings; dimensions and
parameters are validated integers. Every physical mutation line is commented,
including bare-CR handling. There is no global SQL replacement, raw-definition
interpolation or source-span editing in this generator. The separate `Convert`
API performs bounded AST/type binding and byte-span conversion where supported.

HNSW requires m=2..100, ef_construction=4..1000 and ef_construction >= 2*m;
ef_search is explicitly 1..1000. IVF lists are 1..32768 and this narrower
template contract requires probes=1..lists. No parameter defaults, training
choices or EF-to-probes mappings are invented. The source metric is retained;
L1/Manhattan maps to HNSW `vector_l1_ops`, never IVFFlat or bit Hamming. Dense
index dimensions are capped at 2000 independently of vector's 16000 value cap.
The inspector's invalid legacy 100/16 and 3/5 configurations remain blockers;
target options cannot waive that inspection result. A separately reviewed
source-retuning workflow is outside this generator.

Normal indexes can be inside the offline transaction. Concurrent index creation
is a separate artifact marked outside all explicit transactions and is rejected
for single-transaction mode. Failed concurrent builds can leave invalid indexes;
inspect exact identity, definition, `indisvalid` and `indisready` before a new
reviewed retry. No same-name IF NOT EXISTS shortcut is used.

Verification compares array coordinates via trusted `float4send` after
full-dimensional checks, preserving the distinction between numerical equality
and exact binary32 bits. Norm tolerance is explicitly 1e-6 absolute plus 1e-6
relative. NULL alignment, profile counts, PK identity and all mismatch counts
must pass together. Legacy distance values are float32/architecture-sensitive;
capture old exact-query baselines before removing embedding, declare suitable
distance tolerances and validate business filters/tie behavior. ANN requires
separate recall measurement against exact target scans and real query plans;
the generator has not performed these PostgreSQL checks.

## Rollback is bounded

- Before the single transaction commits, ROLLBACK on that same connection undoes
  its database DDL/data changes. It does not undo an application deployment.
- After any migration transaction commits, a later ROLLBACK cannot undo its
  extension changes, indexes or prior backfill batches.
- Abandoning shadow reads before cutover preserves the data workflow's source
  arrays. It does not restore removed old operators or old ANN service.
- After cutover, pause writers and reconcile newer values with retained arrays
  before reverting readers. Retained source data must have been maintained.
- Old-service restoration can require a database/branch restore or a separately
  reviewed reverse migration, old binaries and dependency reconciliation. Modern
  vector owns hnsw and may be referenced by typed shadows/indexes. No instant
  old-ANN rollback or generic cleanup is claimed.

## Pinned sources and evidence

Source profile: pg_embedding 0.3.6, commit
`5d48508aeaeb86b9e9c630468ada7a124b905795`.
Target profile: pgvector 0.8.7, commit
`f37c13f68b57d2c3472b2214fbcff699d6d34876`.

- [Pinned legacy creation SQL](https://github.com/neondatabase/pg_embedding/blob/5d48508aeaeb86b9e9c630468ada7a124b905795/embedding--0.3.6.sql)
- [Pinned target SQL/type/cast/operator classes](https://github.com/pgvector/pgvector/blob/f37c13f68b57d2c3472b2214fbcff699d6d34876/sql/vector.sql)
- [Pinned IVF options](https://github.com/pgvector/pgvector/blob/f37c13f68b57d2c3472b2214fbcff699d6d34876/src/ivfflat.c)
  and [bounds](https://github.com/pgvector/pgvector/blob/f37c13f68b57d2c3472b2214fbcff699d6d34876/src/ivfflat.h)
- [Historical official Neon migration chapter](https://github.com/neondatabase/website/blob/d4c1cf37f79acbf4c02880eddb97dd9a36cb2d20/content/docs/extensions/pg_embedding.md#migrate-from-pg_embedding-to-pgvector)

The requested current Neon page was retrieved on 2026-10-10 and its migration
chapter was absent. The historical chapter is a separate reference. Neither its
CASCADE/table-drop examples nor modern undocumented behavior become defaults.

Golden review bundles are in `examples/migration-transition/golden/`. Focused
command results, source audit and honest execution limits are recorded in
`docs/evidence/m9-transition-ordinary.md`. These are source/text-generated and
locally tested artifacts, not PostgreSQL execution evidence. Restricted M3/M8
durability/security workflows and independent M10 gates remain separate/open.
