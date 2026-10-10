# pg_embedding migration: offline inspection checkpoint

Status: M9 checkpoint A implemented and locally tested. The full M9 migration
is **not complete**. SQL AST conversion, persisted CSV/NDJSON import/resume and
verification, and PostgreSQL migration bundle generation are the next distinct
checkpoints. The current CLI does not accept `--apply`, `--resume`, `--output`,
DSNs, credentials, or arbitrary SQL execution. Existing M3/M8 restricted physical
durability/security qualification and independent release gates remain open.

## Two separate workflows

1. **Export to local gosqlite.** Inspect a versioned offline PostgreSQL catalog,
   select verified real-array vector columns, convert every source value with
   explicit dimensions, and later import into a new real local database in
   bounded transactions. At A, inspection and value conversion work; database
   import/resume and full export verification are not implemented.
2. **PostgreSQL/Neon in-place migration.** Inspect dependencies and actual target
   availability, review extension transition/maintenance strategy, retain source
   arrays, backfill shadow vector columns, build/validate target indexes, and
   coordinate application cutover/rollback. At A, the inspector identifies the
   relevant hazards; it generates no runnable migration SQL and contacts no
   PostgreSQL server. The provided catalog exporter is a read-only SQL reference
   for manual review, not an automatic remote execution step.

## Runnable CLI

From the repository root with the pinned Go 1.27.2 toolchain:

```sh
CGO_ENABLED=0 go run ./cmd/gosqlite migrate pgembedding inspect \
  --catalog testdata/pgembedding-0.3.6/catalog.json
CGO_ENABLED=0 go run ./cmd/gosqlite migrate pgembedding \
  --catalog testdata/pgembedding-0.3.6/catalog.json --dry-run
CGO_ENABLED=0 go run ./cmd/gosqlite migrate pgembedding inspect \
  --catalog testdata/pgembedding-0.3.6/catalog.json --format markdown
```

Inspection always opens only the supplied catalog, never a database. Optional
`--report new-report.json` creates a new report file with mode 0600 and refuses
an existing path; it does not overwrite a catalog, database or previous report.
Duplicate flags, unsupported flags, positional arguments and `--dry-run=false`
are errors. JSON/Markdown reports are deterministic for identical catalog facts
and options. Exit zero means inspection/reporting completed, not that findings
are resolved: callers must check `status`, per-object status and finding codes.

The included `catalog.json` is a complete **synthetic** fixture with invented
OIDs, an extension in the custom `legacy` namespace, three metrics, a custom
index name, an unrelated array column and a claimed exhaustive row profile.
It is not a captured server inventory or observed data profile. Its selected
index parameters were intentionally set to a valid reviewed 16/64 pair so the
fixture can demonstrate a proposed target configuration; tests separately
exercise invalid legacy defaults and published examples.

## Native APIs

`migration/postgres` contains:

- `DecodeCatalog(io.Reader, Limits) (CatalogSnapshot, error)`
- `Inspect(context.Context, CatalogSnapshot, InspectOptions) (Report, error)`
- `ParseRealArray(string, dimensions) (vector.Vector, error)`
- `RenderJSON(io.Writer, Report) error` and `RenderMarkdown(io.Writer, Report) error`

A zero `Limits` uses 16 MiB input, 10,000 catalog objects including columns,
100,000 dependency edges, depth 32 and 1,048,576 JSON/work tokens. Custom limits
must be complete, positive and no greater than the defaults. Individual strings
are at most 1 MiB; identifiers are at most PostgreSQL's 63 UTF-8 bytes. Before
inspection allocates maps or canonical JSON, exported programmatic structs are
also checked for aggregate work/bytes, finite profile numbers and valid text.
The caller already owns input memory; these checks bound additional work and
allocation. The preflight byte charge conservatively bounds JSON output, so a
near-limit compact source can be rejected. No production dependency was added.

JSON requires exact field spelling and all schema fields, including explicit
false values, empty arrays and nullable norm fields. It rejects duplicate keys
at every level, unknown or missing fields, null required arrays/scalars, trailing
values, invalid UTF-8/NUL, unpaired Unicode surrogate escapes, wrong scalar
kinds, unsupported versions, duplicate identities and unresolved references.
Errors identify categories/positions without echoing source vector values.

The report's `catalog_sha256` hashes the actual inspected structure serialized
as Go JSON with stable struct-field order (`catalog_hash_kind=canonical_json_v1`).
Collection ordering is part of that identity. `source_bytes_sha256` is included
only if the structure still equals the decoded source facts; mutating exported
fields invalidates that raw-byte provenance. Hashes establish content identity,
not authenticity or proof that a supplied catalog/profile describes a server.

## Catalog collection and identity

Review `examples/migrate-pgembedding/inspect-schema.sql`. It uses a bounded-time
repeatable-read, read-only transaction, trusted `pg_catalog` names and explicit
schema scope. It reads metadata/definitions without evaluating application or
extension distance functions. The query is source-reviewed, **not executed
against PostgreSQL** in this checkpoint. Its target syntax uses PostgreSQL 12+
catalog fields such as `attgenerated`; other server versions need review.

The export query intentionally declares coverage gaps: only HNSW indexes are
selected, dependents are direct rather than transitive, policy bodies are not
fully inventoried, and privileges/RLS may require further review. It supplies
no row profile or application SQL by default. It cannot establish complete data
validity. Full-data profiling must be a separately reviewed read tied to an
immutable export; the sample shape/count profile is supplied evidence, not a
measurement performed by the inspector.

Membership checks combine `embedding` version 0.3.6, exact type OIDs/names,
`pg_depend` ownership, method handler, opclass input type/default identity,
strategy-1 operator, support-1 function, and argument/result signatures. Index
names themselves are irrelevant. An unrelated `real[]` column is a candidate,
not automatically selected. Explicit `InspectOptions.Selected` entries identify
relation OID, attribute number and dimension; they cannot override conflicting
index dimensions. Domains, unknown opclasses, unknown versions or forged
same-name objects are not treated as the known extension.

Automatic eligibility is narrower than inspection: ordinary permanent heaps,
no RLS/inheritance, one non-NULL int4/int8 primary key, selected real[] columns,
no generated/default/identity expression or custom typmod, and single-key, unfiltered,
non-expression HNSW indexes. Dependencies and incomplete coverage require
review. `attndims` does not prove actual shape. Primary-key identity/sequence defaults
require explicit mapping review; they are not silently treated as local rowid
allocation. Exhaustive supplied profiles must
agree on dimensions, bounds, NULL policy and zero/nonfinite status. No dimension
is inferred from the first row.

`safe` on an object means it passes this inspector's supported subset checks,
not production safety or migration authorization. Invalid/review-required
objects do not expose a usable `proposed_target`. Global warnings still require
review even for individually eligible configurations. No report approves remote
execution or certifies target privileges/version.

## REAL[] value conversion

Accepted non-NULL values are one-dimensional `{1,-2.5,3e-4}` arrays, including
quoted numeric elements with decoded PostgreSQL array escapes, ASCII whitespace,
and explicit default bounds such as `[1:3]={1,2,3}`. Decimal tokens round directly
to binary32 using the existing dense parser. Signed zero, finite extrema and
subnormals are preserved; decimal underflow may produce signed zero.

SQL NULL belongs to a row/nullability layer, not `ParseRealArray`. `{}` is not
NULL. Bare NULL elements, quoted `"NULL"`, multidimensional/ragged arrays,
nondefault lower bounds, inconsistent bounds, nondecimal values, NaN/infinity,
binary32 overflow, trailing syntax and inconsistent dimensions are rejected.
No normalization, padding, truncation, flattening or precision conversion is
performed. Text is limited to 2 MiB, components to 16,000, and a decoded numeric
element to 128 bytes. `ValueError` gives stable code, one-based element ordinal
and expected/observed dimensions without embedding contents.

This preserves source real values represented accurately by the export; it
cannot recover binary32 bits already lost by insufficient decimal precision.
Local metric arithmetic is compensated float64 while old extension results are
float32/architecture-sensitive. Cosine zero norms are an explicit local error.
No exact output-bit equivalence is claimed.

## Options and extension-transition hazards

Pinned mappings are `ann_l2_ops` → `vector_l2_ops`, `ann_cos_ops` →
`vector_cosine_ops`, and `ann_manhattan_ops` → `vector_l1_ops`. Legacy Manhattan
`<~>` must later be converted only in a positively bound legacy-vector context
to dense `<+>`; modern bit `<~>` means Hamming. This inspector reports identities;
it does not yet rewrite SQL.

Effective legacy defaults are m=100, efconstruction=16, efsearch=64. Target HNSW
requires m=2..100, ef_construction=4..1000 and ef_construction >= 2*m. Both old
100/16 defaults and the published 3/5 example are invalid. The report preserves
the old effective values and emits `target_option_constraint`; it never clamps
or silently retunes. Dense values can store 16,000 dimensions, but the target
HNSW cap is 2,000. Unsupported index options are not dropped.

`efsearch` becomes a reviewed PostgreSQL session/transaction setting or local
`VectorSearchOptions.EFSearch`, not an index creation option. Conflicting old
per-index values require application-specific handling. Query settings greater
than the target/local 1..1000 bound are rejected.

Both old `embedding` and target `vector` define the database-wide `hnsw` access
method. Different extension schemas do not avoid the collision. In-place
transition requires reviewed dependencies, a maintenance-window or separate
branch/database choice, confirmed target extension version/opclasses and a
rollback plan. No default CASCADE, source-column deletion or cleanup is provided.
Retained arrays help rollback only if later writes keep them current; restoring
old ANN service after a committed extension transition is a separate operation.

## Sources and exact checkpoint evidence

Pinned legacy source: `5d48508aeaeb86b9e9c630468ada7a124b905795`, extension 0.3.6.
Pinned target: pgvector v0.8.7, `f37c13f68b57d2c3472b2214fbcff699d6d34876`.
`testdata/pgembedding-0.3.6/manifest.json`, LICENSE and NOTICE preserve exact source
file hashes and licenses. The Go implementation is independently written.

The requested [current Neon page](https://neon.com/docs/extensions/pgvector#migrate-from-pg_embedding-to-pgvector)
was retrieved on 2026-10-10; its migration chapter is absent. The separate
[official historical chapter](https://github.com/neondatabase/website/blob/d4c1cf37f79acbf4c02880eddb97dd9a36cb2d20/content/docs/extensions/pg_embedding.md#migrate-from-pg_embedding-to-pgvector)
is the 2024-01-23 source reference. Historical destructive examples are not
current migration instructions or executable defaults.

Focused tests exercise real CLI dry-run/reporting, strict catalog validation,
ownership/signature drift, invalid option defaults, dimension/profile conflicts,
byte-exact binary32 conversion and source/provenance mutation. Only ordinary
in-memory/parser and intact-file CLI tests run here. No PostgreSQL execution,
remote write, crash/fault/forced-exit, recovery, wrong-key or physical-tamper
workflow was performed. Exact executed commands/results are recorded in the
checkpoint evidence by the integration lead.
