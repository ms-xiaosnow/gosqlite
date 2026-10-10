# gosqlite

A native Go database kernel targeting tested subsets of SQLite 3.54.0 files and SQL. Current development milestone: M2 SQL/write MVP. It is not production-ready. No CGO, SQLite subprocess, C ABI, or remote SQL engine is used in production.

## Try the implemented SQL path

Use Go 1.27.2. Runtime database opening currently requires Linux; other targets compile and fail closed until native locks are supported.

```sh
CGO_ENABLED=0 go test ./...
CGO_ENABLED=0 go run ./cmd/gosqlite query testdata/readonly/page4096.db \
  "SELECT id,label FROM samples WHERE label LIKE '%数据库%' ORDER BY id LIMIT 3"
CGO_ENABLED=0 go run ./cmd/gosqlite inspect testdata/readonly/page512.db
./scripts/check.sh
```

```sh
CGO_ENABLED=0 go run ./cmd/gosqlite query example.db 'CREATE TABLE docs(id INTEGER PRIMARY KEY, title TEXT NOT NULL)'
CGO_ENABLED=0 go run ./cmd/gosqlite query example.db 'CREATE INDEX docs_title ON docs(title)'
CGO_ENABLED=0 go run ./cmd/gosqlite query example.db "INSERT INTO docs VALUES(1,'数据库')"
CGO_ENABLED=0 go run ./cmd/gosqlite query example.db 'SELECT id,title FROM docs'
```

Each CLI invocation executes one statement. Writes autocommit and return JSON with rows_affected and last_insert_id. Explicit transactions use the Go API; BEGIN/COMMIT in separate CLI invocations are rejected. SELECT opens read-only and writes JSON Lines: a columns object followed by row objects. BLOB values are base64. Inspect is metadata inspection, not a full integrity check. Unsupported commands return errors.

```go
import (
    "context"
    "database/sql"
    _ "github.com/ms-xiaosnow/gosqlite/driver"
)

func read(ctx context.Context) error {
    db, err := sql.Open("gosqlite", "file:existing.db?mode=ro")
    if err != nil { return err }
    defer db.Close()
    rows, err := db.QueryContext(ctx, "SELECT id,title FROM docs WHERE id > ? ORDER BY id LIMIT ?", 0, 10)
    if err != nil { return err }
    defer rows.Close()
    for rows.Next() {
        var id int64
        var title string
        if err := rows.Scan(&id, &title); err != nil { return err }
    }
    return rows.Err()
}
```

The read-only example requires an existing database and docs table. For writes, use a normal path or mode=rwc DSN, or mode=rw to require an existing file. Native sqlite.Options{} is writable/create; ReadOnly and NoCreate are explicit alternatives. See examples/write for prepared inserts and transactions. Native transactions can retry BUSY Commit after readers close; database/sql transactions cannot be reused after Commit, so the adapter rolls back on an error. Close Rows promptly to release their snapshot locks.

CREATE TABLE, nonunique BINARY ASC CREATE INDEX, INSERT, UPDATE, DELETE and explicit Go/driver transactions are implemented for the documented subset. Secondary indices are maintained on disk, but SELECT currently scans. Unsupported schema objects/constraints reject writes to the database rather than being ignored. Existing journals require recovery and are refused. Existing zero-byte files are conservatively rejected. Normal successful commit tests do not qualify crash durability; use only disposable data.

If home is read-only, set GOCACHE and GOPATH to writable directories. There are no external Go module dependencies, so no go.sum is needed. Official SQLite is a separately built test oracle only:

```sh
SQLITE_ORACLE=/absolute/path/to/verified/sqlite3 ./scripts/test-oracle.sh
```

See [progress](docs/progress.md), [compatibility](docs/compatibility.md), [architecture](docs/architecture.md), [sources](docs/upstream.md), and [manual verification](docs/manual-verification.md). The unchanged [original goal](goal.md) remains the contract. [Restart protocol](docs/restart.md) and checkpoint records preserve durable development history.

Native recovery/WAL, grams13 indices, similarity, vectors, independent authenticated encryption and pg_embedding migration are still future milestones. Full SQLite/PostgreSQL semantics, SEE file compatibility and production security are not claimed.
