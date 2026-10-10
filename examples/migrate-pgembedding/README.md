# Offline pg_embedding inspection

From the repository root:

```sh
CGO_ENABLED=0 go run ./cmd/gosqlite migrate pgembedding inspect \
  --catalog testdata/pgembedding-0.3.6/catalog.json --format markdown
CGO_ENABLED=0 go run ./cmd/gosqlite migrate pgembedding \
  --catalog testdata/pgembedding-0.3.6/catalog.json --dry-run
```

These commands run the real native Go inspector and open no database. The
catalog is synthetic, not a PostgreSQL capture. JSON status `review_required`
and its warnings are expected: source arithmetic, application SQL inventory,
query-setting scope, old/new hnsw ownership and the absent current Neon chapter
all need review. Old default100/16 and example3/5 are explicitly rejected in tests.

`inspect-schema.sql` is a manual-review read-only export query for PostgreSQL
12+ catalog fields. It has not been executed against PostgreSQL. It intentionally
reports incomplete HNSW-only index/direct-dependent/policy coverage and contains
no full-data profiling or arbitrary application SQL execution.

See `docs/migration-pgembedding.md` for schema/API details and the separate local
import and PostgreSQL in-place workflows. At checkpoint A, SQL conversion, local
CSV/NDJSON import/resume/verify and PostgreSQL migration templates remain pending.
