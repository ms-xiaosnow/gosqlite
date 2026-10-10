# gosqlite

A native Go database kernel targeting tested subsets of SQLite 3.54.0 files and SQL. Current development milestone: M1 read-only SQLite. It is not production-ready. No CGO, SQLite subprocess, C ABI, or remote SQL engine is used in production.

## Try the implemented read-only path

Use Go 1.27.2. Runtime database opening currently requires Linux; other targets compile and fail closed until native locks are supported.

```sh
CGO_ENABLED=0 go test ./...
CGO_ENABLED=0 go run ./cmd/gosqlite query testdata/readonly/page4096.db \
  "SELECT id,label FROM samples WHERE label LIKE '%数据库%' ORDER BY id LIMIT 3"
CGO_ENABLED=0 go run ./cmd/gosqlite inspect testdata/readonly/page512.db
./scripts/check.sh
```

The query command writes JSON Lines: a columns object followed by row objects. BLOB values are base64. Inspect is metadata inspection, not a full integrity check. Unsupported commands return errors.

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

The database and docs table must already exist. Writes and transactions fail explicitly. Use sqlite.Open plus sqlite.Options for the native API, or driver.NewConnector with sql.OpenDB for configured connections. Close handles promptly: M1 keeps a shared lock for their lifetime.

If home is read-only, set GOCACHE and GOPATH to writable directories. There are no external Go module dependencies, so no go.sum is needed. Official SQLite is a separately built test oracle only:

```sh
SQLITE_ORACLE=/absolute/path/to/verified/sqlite3 ./scripts/test-oracle.sh
```

See [progress](docs/progress.md), [compatibility](docs/compatibility.md), [architecture](docs/architecture.md), [sources](docs/upstream.md), and [manual verification](docs/manual-verification.md). The unchanged [original goal](goal.md) remains the contract. [Restart protocol](docs/restart.md) and checkpoint records preserve durable development history.

Transactional writes, grams13 indices, similarity, vectors, independent authenticated encryption and pg_embedding migration are still future milestones. Full SQLite/PostgreSQL semantics, SEE file compatibility and production security are not claimed.
