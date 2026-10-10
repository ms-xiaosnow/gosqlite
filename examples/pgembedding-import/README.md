# Resumable local pg_embedding import

```sh
go run ./examples/pgembedding-import
```

Creates a new temporary directory, two synthetic source exports (CSV and NDJSON),
two explicit manifests and two real native SQLite databases. The JSON output
identifies their exact paths and shows dry-run, committed pause, normal resume,
full source/database verification, and separately measured actual HNSW/IVF plans
and recall@4 for a fixed clustered query.

Use `-dir /path/to/a/new-directory` to retain files at a chosen new destination.
Existing directories/files are refused. No PostgreSQL server, credentials,
external database process, abrupt termination or crash recovery is involved.
The full format, bounds, status and fidelity contract is in
[`docs/import-pgembedding.md`](../../docs/import-pgembedding.md).
