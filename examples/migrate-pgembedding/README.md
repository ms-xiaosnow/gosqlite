# Offline pg_embedding inspection and SQL conversion

From the repository root:

```sh
CGO_ENABLED=0 go run ./cmd/gosqlite migrate pgembedding inspect \
  --catalog testdata/pgembedding-0.3.6/catalog.json --format markdown
CGO_ENABLED=0 go run ./cmd/gosqlite migrate pgembedding convert \
  --catalog examples/migrate-pgembedding/converter-catalog.json \
  --input examples/migrate-pgembedding/source.sql \
  --target local --table 30000=documents
CGO_ENABLED=0 go run ./cmd/gosqlite migrate pgembedding convert \
  --catalog examples/migrate-pgembedding/converter-catalog.json \
  --input examples/migrate-pgembedding/source.sql \
  --target postgres --table 30000=documents --target-schema public \
  --vector-schema public --format sql
CGO_ENABLED=0 go run ./examples/migrate-pgembedding
```

The CLI commands open no database and execute no SQL. All catalogs and data in
this directory are synthetic; `converter-catalog.json` describes just the three
columns declared by `source.sql`. The original A catalog additionally has an
unrelated real-array column, which must not be silently omitted by CREATE TABLE
conversion. Conversion requires an explicit relation-OID-to-target-table mapping.

The compiled example executes only its embedded converted fixture in a temporary
local database, creates a genuine HNSW index, closes/reopens normally and checks
exact Manhattan distances and PostgreSQL NULL order:

```text
id=3 distance=0
id=1 distance=5
id=2 distance=6
id=4 distance=<nil>
```

It prints the exact source/converted SQL hashes. Tests compare L2, cosine and L1
against independent scalar calculations; no PostgreSQL server is involved.
The nullable sort adds an IS NULL key and can use the exact general sorter rather
than ANN. Native application EFSearch=64 is a separate reviewed setting, never a
CREATE INDEX reloption or fake local SET statement.

JSON contains statement findings, exact source edits, runtime bind conversions
and settings. Unsupported statements retain original text only as report evidence;
SQL output refuses a script containing any unsupported statement. `--report NEW`
creates a new artifact and refuses overwrite. PostgreSQL output is review-only
text, not an extension-transition/backfill/cutover plan or execution approval.

`inspect-schema.sql` remains a manual-review read-only PostgreSQL 12+ catalog
export query. It has not been executed against PostgreSQL and explicitly reports
inventory/profile gaps. The inspector's current-Neon-chapter, numeric semantics,
query-setting and hnsw-ownership findings remain applicable.

See [converter boundaries](../../docs/migration-converter.md) and
[inspection/local-vs-PostgreSQL workflows](../../docs/migration-pgembedding.md).
CSV/NDJSON database import/resume/verify (C) and PostgreSQL migration phase bundles
(D) remain pending. This example does not implement those later checkpoints.
