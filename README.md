# gosqlite

A native Go database kernel project targeting tested subsets of SQLite 3.54.0 files and SQL. The current checkpoint is M0: a reproducible scaffold, not a usable database engine. No CGO, SQLite subprocess, C ABI, or remote SQL engine is used in production.

## Run the current scaffold

Use Go 1.27.2:

```sh
CGO_ENABLED=0 go test ./...
CGO_ENABLED=0 go run ./cmd/gosqlite version
./scripts/check.sh
```

If your home is read-only, set GOCACHE and GOPATH to writable local directories. There are no external Go modules at this checkpoint, so no go.sum is needed. Official SQLite is used only as a separately built test oracle.

Implementation and acceptance are tracked separately in [progress](docs/progress.md), [compatibility](docs/compatibility.md), and [manual verification](docs/manual-verification.md). The unchanged [original goal](goal.md) is the contract. The [restart protocol](docs/restart.md) explains durable checkpoints.

Planned capabilities include transactional SQLite storage, database/sql, persisted grams13 indices, similarity, vectors, independent authenticated encryption and pg_embedding migration. None is currently advertised as implemented. SEE file compatibility, full PostgreSQL semantics, and production security are not claimed.
