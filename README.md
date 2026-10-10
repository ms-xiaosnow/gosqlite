# gosqlite

A native Go database kernel targeting tested subsets of SQLite 3.54.0 files and SQL. Current development milestone: M4 persistent grams13 and indexed LIKE (ordinary M2 SQL/write MVP verified). It is not production-ready. No CGO, SQLite subprocess, C ABI, or remote SQL engine is used in production.

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

CREATE TABLE, nonunique BINARY ASC CREATE INDEX, INSERT, UPDATE, DELETE and explicit Go/driver transactions are implemented for the documented subset. Ordinary secondary indices are maintained on disk. Registered grams13 posting indices accelerate supported LIKE and configured MATCH predicates; other SELECT paths still scan. Unsupported schema objects/constraints reject writes to the database rather than being ignored. Existing journals require recovery and are refused. Existing zero-byte files are conservatively rejected. Normal successful commit tests do not qualify crash durability; use only disposable data.

If home is read-only, set GOCACHE and GOPATH to writable directories. The Unicode tokenizer uses pinned pure-Go golang.org/x/text; go.sum records verified module checksums and third_party/ retains its license. Official SQLite is a separately built test oracle only:

```sh
SQLITE_ORACLE=/absolute/path/to/verified/sqlite3 ./scripts/test-oracle.sh
```

See [progress](docs/progress.md), [compatibility](docs/compatibility.md), [architecture](docs/architecture.md), [sources](docs/upstream.md), and [manual verification](docs/manual-verification.md). The unchanged [original goal](goal.md) remains the contract. [Restart protocol](docs/restart.md) and checkpoint records preserve durable development history.

Qualified crash recovery, integrated similarity ranking, vectors, independent authenticated encryption and pg_embedding migration remain unfinished milestones. Full SQLite/PostgreSQL semantics, SEE file compatibility and production security are not claimed.


## Explicit exclusive WAL

The developing M3 mode is opt-in: sqlite.Options{WALExclusive:true}, or driver.NewConnector with that option and sql.DB.SetMaxOpenConns(1). It holds an EXCLUSIVE database lock for the connection lifetime, including idle time, and permits one active query/transaction lease. Other connections correctly receive BUSY. It is not shared concurrent WAL.

```sh
CGO_ENABLED=0 go run ./examples/wal disposable-wal.db
CGO_ENABLED=0 go run ./cmd/gosqlite query --wal-exclusive disposable-wal.db 'SELECT id,text FROM notes'
```

Native DB.Checkpoint(ctx) performs an explicit normal checkpoint. Default Close checkpoints; KeepWALOnClose retains successful committed frames after normal close. A WAL connection requires writable file access even for SELECT. Plain read-only Open/CLI inspect still refuse mode2. Unsupported WAL tails and existing journals fail closed. Hot rollback-recovery code is isolated and source-only; no crash/fault durability gate has passed. See ADR0004 and manual-verification.md.

## Explicit shared WAL

Use sqlite.Options{WALShared:true}, or driver.NewConnector with that option for pooled concurrent connections. Each connection permits one active lease; independent connections retain read snapshots while a single writer commits. Stale deferred writer promotion returns ErrBusySnapshot (also matches ErrBusy).

```sh
CGO_ENABLED=0 go run ./cmd/gosqlite query --wal-shared disposable-shared.db 'CREATE TABLE notes(id INTEGER PRIMARY KEY, text TEXT)'
CGO_ENABLED=0 go run ./cmd/gosqlite query --wal-shared disposable-shared.db "INSERT INTO notes VALUES(1,'共享 WAL')"
```

DB.Checkpoint is nonblocking and may safely backfill some frames before returning BUSY for readers. Close attempts a checkpoint unless KeepWALOnClose and retains WAL/SHM inodes. Writable access is required, including for read transactions. Conservative storage-sector protection defaults on; WALPowersafeOverwrite is an explicit storage assertion and is never autodetected. Live official SQLite reader/writer interoperability is tested, but crash/fault durability and broader cold recovery remain unqualified. See ADR0006 and the compatibility matrix.

## Persistent text indexing

```sh
CGO_ENABLED=0 go run ./examples/grams disposable-grams.db
CGO_ENABLED=0 go run ./cmd/gosqlite check disposable-grams.db docs_grams
```

CREATE INDEX docs_grams ON docs(title) USING grams13 builds legal transactional SQLite posting structures. One/two/three-rune LIKE literals can use actual posting ranges and source-row seeks, followed by original predicate rechecking. EXPLAIN QUERY PLAN and native Rows.Plan/Stats show the access path and work. Configured normalized containment uses grams13_match('docs_grams',title,needle). REINDEX rebuilds safely; ordinary external SQLite writes or schema changes invalidate the index and force explained scans. See [grams13 APIs, semantics and limits](docs/grams13.md). The check command verifies one extension index's source/posting/frequency consistency, not general SQLite integrity.
