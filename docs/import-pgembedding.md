# Offline pg_embedding import: checkpoint C

`migration/postgres.Import` imports an explicitly mapped, immutable local
CSV/NDJSON export into a new, genuine native SQLite database. It does not connect
to PostgreSQL, execute supplied SQL, or replace pgvector inside Neon. Catalog
inspection and SQL conversion remain separate read-only commands. The manifest's
source profile and decimal-fidelity descriptions are supplied export evidence,
not authentication of a PostgreSQL installation or reconstruction of lost bits.

## Run a complete synthetic example

From the repository root, with the pinned Go toolchain and dependencies available:

```sh
go run ./examples/pgembedding-import
```

The example creates a new private temporary directory containing both formats,
matching manifests, and two persisted databases. For each format it validates
without writing, deliberately stops after one committed batch, closes normally,
resumes, builds real HNSW and IVF indexes, and independently verifies all rows.
Its JSON prints exact file paths and separate native ANN plans/recall. No external
server, credentials, downloaded fixture, or SQLite subprocess is involved.

The 64-row example preserves MinInt64/MaxInt64 PKs, signed binary32 zero and the
smallest subnormal, UTF-8 text, and a multiline text field. CSV and NDJSON yield
the same canonical typed-row SHA-256. Its one bounded L2 query has recall@4 = 1
for both native indexes; this is fixture evidence, not a general recall promise.

Using the generated paths, or your own reviewed export:

```sh
gosqlite migrate pgembedding --input source.ndjson --manifest export.json
# The default above is read-only validation.
gosqlite migrate pgembedding --input source.ndjson --manifest export.json \
  --output imported.db --apply --batch-rows 500
gosqlite migrate pgembedding --input source.ndjson --manifest export.json \
  --output imported.db --apply --resume
gosqlite migrate pgembedding verify --input source.ndjson --manifest export.json \
  --output imported.db
```

`import` may also be placed explicitly after `pgembedding`. Duplicate options,
unknown options, positional extras, `--resume` without `--apply`, and an explicit
`--dry-run` together with `--apply` are rejected. `--dry-run=false` does not grant
write permission. `--stop-after-batches N` deliberately returns after N newly
committed batches and closes normally. It is an ordinary resumability control,
not a process-interruption or crash-recovery test. JSON goes to stdout; errors
are returned with a nonzero CLI exit. Index failures include their pending report.

## Manifest version 1

The example writes a complete runnable manifest. This is its logical contract:

- `format_version`: 1
- `source_profile`: exactly `pg_embedding-real-float4-v1`
- `source_id`: nonempty bounded snapshot/export identifier, no credentials
- `source_sha256`: lowercase 64-character SHA-256 of every input byte
- `source_size`: exact byte count
- `expected_rows`: optional nonnegative row count, excluding the CSV header
- `format`: `csv` or `ndjson`
- `row_order`: `ascending` (strictly increasing accepted/valid PK order) or `source`
- `decimal_fidelity`: `roundtrip_binary32` or `export_precision_only`
- `null_token`: literal `\N` for CSV, empty string for NDJSON
- `table`: one explicit local target table
- `fields`: explicit ordered field mappings
- `indexes`: explicit array, including an empty array when no index is requested

Each field requires `source`, `target`, `type`, `encoding`, `nullable`, and
`primary_key`. Exactly one field is a nonnullable `int64` PK. `dimensions` is
required for vector fields and must be absent/zero for other fields.

| Type | Encoding | Representation |
|---|---|---|
| int64 | integer | Exact signed decimal integer, never via float64 |
| float64 | decimal | Finite directly parsed binary64; nonzero underflow rejected |
| text | utf8 | UTF-8 text, no NUL |
| blob | base64 | Explicit strict base64; empty BLOB is distinct from NULL |
| vector | pg_array | PostgreSQL real-array text, CSV or NDJSON string |
| vector | json_array | NDJSON numeric JSON array only |

No implicit type inference, mixed vector encodings, arbitrary business types,
schema qualification, defaults, generated expressions, or executable DDL is
accepted. Target identifiers use a bounded ASCII identifier subset and are
quoted by generated SQL. Engine-reserved prefixes and rowid aliases are rejected.
Source JSON keys/header names are exact and case-sensitive. Column values always
use parameter binding, including SQL-looking strings.

The local subset is one table, one int64 PK, dense vectors and mapped primitives.
PostgreSQL triggers, views, foreign keys, policies/RLS, grants, partitions, array
subscript dependencies, application SQL and arbitrary server-side behavior are
not migrated by this row importer. Inspect those dependencies separately and use
the read-only converter/report workflow where supported.

CSV uses the standard comma/double-quote dialect with a required unique header
in exactly manifest order. Field count is exact. Multiline quoted fields are
supported, and offsets are measured at complete raw byte-record boundaries,
cross-checked against `encoding/csv.InputOffset`. Literal CSV text equal to `\N`
is unrepresentable in this null-token profile; quoting it does not escape its
NULL meaning. An empty text or BLOB is valid; an empty vector field is invalid.
Malformed CSV quoting/count is terminal because record resynchronization would
be ambiguous.

NDJSON uses one object per physical line. Blank lines, trailing material,
malformed values, invalid UTF-8/NUL, nonfinite numbers and invalid escapes are
rejected. Duplicate/unknown/missing keys are schema errors, even if another
field in that row is invalid. Missing keys are never treated as NULL. Nested
objects and nonnumeric vector elements are not an automatic conversion route.

Vectors preserve exact directly rounded binary32 component bits, including
signed zero and subnormals. Nonzero decimals rounding all the way to zero are
rejected to match PostgreSQL float4 input. This is intentionally stricter than
checkpoint A's standalone `ParseRealArray`, whose documented zero-underflow
behavior is unchanged. Default lower bound 1 is required; nondefault bounds,
multidimensional/ragged arrays, NULL elements, empty vectors and dimension
mismatches fail. No normalization, padding, truncation or half conversion occurs.
Low-precision decimal exports cannot recover original bits that were discarded
before export; `complete_verified` compares the supplied export faithfully.

## Bounds and quarantine

Current hard caps are 64 KiB manifest JSON, 1 GiB source bytes, 1 MiB raw record,
32 fields, 8 requested indexes, 1,000,000 source records and 10,000 rejections.
Record framing checks the raw byte cap before handing a complete record to CSV
or JSON decoding. Vector dimensions remain 1..16000; native dense ANN is limited
to 2000. Programmatic manifests receive the same structural limits.

A batch defaults to 500 rows / 8 MiB decoded values. `--batch-rows` is 1..5000;
`--batch-bytes` is 1..16 MiB. Decoded-byte accounting covers scalar, NULL, text,
BLOB and vector payloads. This is not a total process-RSS promise: parser buffers,
one bounded lookahead record, canonical encoding, SQLite pages/statement undo,
and native index budgets are additional. Native transaction or ANN quotas can
reject work earlier. Ascending dry-run is constant-space in accepted key count;
unordered dry-run retains at most 65,536 accepted keys, then fails explicitly.
Apply/verify use bounded point lookups and never retain a whole-import PK set.

Stop-on-error is the default. `--on-error quarantine` only permits deterministic
fully framed value/format, duplicate-PK, or ordering errors. Schema, oversized
framing, I/O, context, SQLite/internal and native-index errors are never bad rows.
A failed batch publishes none of its rows, rejection records, or cursor. Every
accepted PK is bound to its source record ordinal in an ordinary provenance
ledger. Duplicate PKs are rejected, including identical repeats; no replace or
upsert operation is used. Invalid rows do not silently become SQL NULL.

Rejection metadata contains only record ordinal, optional int64 PK, bounded
error code/field number and SHA-256, never row payloads. `rejected_pks` lists
known keys for all rejected records. `missing_pks` is the separately verified,
sorted unique set absent from the destination: rejected duplicates can still
have an earlier accepted destination row. Malformed PKs are counted as
`rejections_without_pk`; their original keys cannot be invented. Dry-run and
paused results use `missing_pks: null`, since exhaustive destination verification
has not run. The export itself is retained unchanged.

## Transactions, resume and index phases

Apply atomically reserves a new 0600 output using `O_CREATE|O_EXCL`, closes that
reservation descriptor before native locking, initializes the owned empty file
through the native pager, then opens through `sqlite.OpenContext(NoCreate:true)`.
No existing target is truncated, renamed, replaced, or adopted. If initialization
fails before a committed job exists, the reserved output may remain; it cannot
be resumed as a completed initialization. No cleanup deletes an unrelated file.

Input and destination paths/files must remain exclusively owned and immutable
to other actors during import/verification. Handle/path identity, size and full
source SHA-256 are checked before output creation and again before completion.
This does not defend against hostile namespace substitution races. No such
race, disk fault, corrupted file, wrong key, abrupt process termination, hot
recovery or interrupted-publication behavior was exercised or established here.

Ordinary data and four ordinary metadata tables share the native transaction:
`gosqlite_migration_jobs`, `gosqlite_migration_batches`,
`gosqlite_migration_rejections`, and `gosqlite_migration_accepted`. Each batch
commits rows, provenance, bounded rejection digests, cumulative framed hashes,
record cursor, source byte offset and batch identity together. A sidecar or log
cannot advance resume. The source is rehashed; resume independently replays and
compares the complete committed prefix, job, schema fingerprint, provenance,
rejections and every batch boundary before seeking to the authoritative cursor.
The manifest, converter version and rejection policy must match. Batch sizing
may change because it does not change conversion semantics.

An uncertain commit is returned as an error with no automatic reinsertion or
claim of a successful rollback. A later explicit resume must open normally and
reconcile the committed job; native open may itself require unsupported recovery.
This checkpoint proves deliberate committed stop/normal close/reopen and ordinary
cancellation behavior, not crash safety or general hot-recovery support.

Requested `hnsw`/`ivfflat` indexes are built only after the data phase through the
real native index APIs. The manifest explicitly gives name, vector column,
method and metric (`l2`, `l1`, `cosine`, `negative_inner_product`), plus applicable
construction parameters. Zero-valued omitted options receive fixed native v1
defaults. IVF rejects L1 and requires enough trainable rows for its list count.
Cosine-zero data remains present and explicitly blocks the requested index.
Native node, memory, transaction and training-work quotas still apply.

Each successful index and its metadata progress commit in one transaction.
Failures retain `data_complete_index_pending`, accepted data, and previously
committed indexes. Resume can retry the unchanged pending build. No conflicting
object is overwritten. Verification checks native consistency and exact
method/config/source/column identity with bounded read-only native v1 metadata
queries; it never edits extension-owned tables or assumes an unrecognized
storage version is compatible. IVF must actually be trained.

Statuses are:

- `dry_run_validated` / `dry_run_with_rejections`: data parsing complete; no DB
  created and no index was built or performance claim verified
- `paused_committed`: deliberate normal return; persisted job remains `importing`
- `importing`: source data phase incomplete or a count error stopped completion
- `data_complete_index_pending`: rows retained; requested index build unfinished
- `data_complete_verify_pending`: indexes built; verified completion unpublished
- `complete_verified`: no rejections and exhaustive export/database checks passed
- `complete_with_rejections`: exhaustive checks passed for the declared quarantine
- `verification_failed`: a readback operation failed; no completion claim

## Verification evidence and boundaries

The public read-only `VerifyImport` holds one native read snapshot. Import also
closes/reopens normally and reconciles again under the writer-reserved snapshot
that publishes completion, including a final source hash and index identity
check. It checks every accepted PK, NULL, primitive value, vector dimension and
component bit; exact destination/provenance/rejection/batch counts; and every
committed cursor/batch hash. No unsupported SQL aggregate, JOIN, or PostgreSQL
execution is invented; counts use bounded `Rows` iteration.

Canonical row framing is `GMI1`, big-endian field count, then for each mapped
field a one-byte type-name length, type bytes, NULL marker and typed content.
Int64 and binary64 use 8-byte big-endian bits; text/BLOB use 8-byte lengths and
bytes; vectors use 4-byte dimension followed by big-endian binary32 bits. A
stream hash length-prefixes each canonical row with 8 bytes and follows source
record order. Independently generated database canonical bytes must match
source bytes component by component, not just by hash. Raw-record and rejection
streams have separate explicit framing and hashes. Hash equality supplements
exact fixture comparisons and counts; it is not a mathematical adversarial proof.

Full vector NULL/non-NULL/zero-norm and float64 norm-range statistics are reported.
`exact_distances` and `ann_recall` remain `not_run` in ordinary import reports;
data fidelity alone proves neither. A separate 34-row distance fixture compares
all four metrics to independent 256-bit arithmetic at
`1e-12 * max(1, abs(reference))`. Mathematically tied cosine directions may differ
by normalization roundoff: every PK retains its distance assertion, native
computed-distance/signed-PK ordering is checked, and reference order is required
outside combined numeric tie bounds. Extreme float32 overflow and subnormal
cases remain retained; no universal tolerance or upstream bit-exact arithmetic
claim is made. Separate 64-row ANN checks require actual approximate native plans
and exact recall@4=1 on their fixed clustered query. Broader M7 recall/work targets
and any external PostgreSQL comparison remain separate evidence.

Component evidence is in `docs/evidence/m9-import-components.txt`; independent
review evidence is owned separately by the reviewer. These focused checks do
not replace the lead's whole-repository aggregate, cross-platform release gates,
or official SQLite interoperability checks.
