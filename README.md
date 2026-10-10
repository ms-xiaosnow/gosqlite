# gosqlite

Historical test-scope statements are corrected in [the namespace-test audit](docs/test-scope-audit.md); the affected tests are now excluded from ordinary workflows.

A native Go database kernel targeting tested subsets of SQLite 3.54.0 files and SQL. Current development: persistent HNSW/IVFFlat, exact typed-vector KNN and same-snapshot hybrid ranking, plus experimental authenticated snapshot, dirty-page WAL and concurrent encrypted sessions, ordinary B-tree equality planning, nested savepoints, scoped INNER/CROSS/LEFT JOINs, CASE/CAST/BETWEEN/IN and explicit NULL ordering, BIT/HALFVEC/SPARSEVEC exact search, extended vector primitives, and bounded pg_embedding catalog inspection, typed offline SQL conversion and resumable CSV/NDJSON import. It is not production-ready. No CGO, SQLite subprocess, C ABI, or remote SQL engine is used in production.

## Try the implemented SQL path

Use Go 1.27.2. Plain-file native backends are implemented for Linux, Darwin and Windows/amd64. Only Linux/amd64 ordinary runtime is verified here; Darwin/Windows evidence is source review and cross-compilation, with platform limits documented in ADR0015. Encrypted native backends are implemented for Linux, Darwin and Windows/amd64; only Linux ordinary runtime is verified.

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

CREATE TABLE, nonunique BINARY ASC CREATE INDEX, INSERT, UPDATE, DELETE and retained-connection SQL/native/driver transactions with nested savepoints are implemented for the documented subset. Ordinary secondary indices are maintained on disk. Registered grams13 posting indices accelerate supported LIKE and configured MATCH predicates. Eligible rowid and ordinary leading-prefix equality predicates use real source/index seeks; unsupported shapes scan. See [ordinary index planning](docs/ordinary-index.md). Unsupported schema objects/constraints reject writes to the database rather than being ignored. Existing journals require recovery and are refused. Existing zero-byte files are conservatively rejected. Normal successful commit tests do not qualify crash durability; use only disposable data.

If home is read-only, set GOCACHE and GOPATH to writable directories. The Unicode tokenizer uses pinned pure-Go golang.org/x/text; go.sum records verified module checksums and third_party/ retains its license. Official SQLite is a separately built test oracle only:

```sh
SQLITE_ORACLE=/absolute/path/to/verified/sqlite3 ./scripts/test-oracle.sh
```

See [progress](docs/progress.md), [compatibility](docs/compatibility.md), [architecture](docs/architecture.md), [sources](docs/upstream.md), and [manual verification](docs/manual-verification.md). The unchanged [original goal](goal.md) remains the contract. [Restart protocol](docs/restart.md) and checkpoint records preserve durable development history.

Qualified crash recovery, Windows shared rekey and Darwin Unicode inventory, broader SQL and online/dependency-complete pg_embedding transition work remain unfinished implementation work. Full SQLite/PostgreSQL semantics, SEE file compatibility and production security are not claimed.


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

## Exact text similarity

```sh
CGO_ENABLED=0 go run ./examples/similarity disposable-similarity.db
CGO_ENABLED=0 go run ./cmd/gosqlite query disposable-similarity.db "SELECT id,ngram_similarity(title,'数据库',2,'dice') AS score FROM docs WHERE score>=0.1 ORDER BY score DESC,id LIMIT 3"
```

SQL exposes ngram_similarity, bigram_similarity and rune-based levenshtein. SQL score expressions currently scan; native DB/Tx.SimilaritySearch uses complete persistent posting unions when sound, then exact scoring and business filtering. Its plan and counters report the actual path. Zero threshold, empty query-token sets, dirty indices or insufficient posting budget use exact scans. Bounded operations return no partial result on error. See [similarity semantics and APIs](docs/similarity.md). The example also demonstrates weighted RRF of two text rankings, preserving contribution details. examples/vector adds same-snapshot text/vector fusion.

## Dense vectors and exact hybrid retrieval

```sh
CGO_ENABLED=0 go run ./examples/vector disposable-hybrid.db
CGO_ENABLED=0 go run ./cmd/gosqlite query disposable-hybrid.db "SELECT id,embedding <-> '[1,0,1]' AS distance FROM docs WHERE active=1 AND embedding IS NOT NULL ORDER BY distance LIMIT 3"
```

VECTOR(n) mutations validate finite float32 dimensions and persist versioned BLOB values. SQL exposes all four dense distance operators, conversion/dimension/norm functions and bounded exact Top-K for recognized queries. Native DB/Tx.VectorSearch provides exact filtered search and actual plans/counters. Vector and NullVector support database/sql. The example combines text/vector rankings in one snapshot and preserves raw measures plus RRF contributions. This example uses exact retrieval; separately installed HNSW/IVFFlat provide explicitly approximate paths. See [vector APIs, SQL boundaries and limits](docs/vector.md).

## Persistent HNSW (experimental)

```sh
CGO_ENABLED=0 go run ./examples/ann NEW_DISPOSABLE_DB
CGO_ENABLED=0 go run ./benchmarks/ann -rows 2048 -dims 12 -queries 48 -seed 20261010
```

Native VectorSearch defaults exact; Mode:sqlite.VectorModeApproximate opts into a matching persistent HNSW index. Eligible SQL distance ORDER BY/LIMIT can automatically choose approximate HNSW once installed; use NOT INDEXED for exact SQL. Plans expose method, approximate recall, graph work, source rescoring, widening and fallback. Transactional insert/update/tombstone/delete/reindex, old grams-file upgrade and ordinary official SQLite interop are tested. The first persistent benchmark met recall@10=1.0 on its fixed dataset but was slower than exact scan; see [ANN behavior and measured limits](docs/ann.md). IVFFlat now supplies deterministic training and real bounded centroid/list access; the small-data benchmark reports its recall/latency tradeoff without a speedup claim.

## IVFFlat and encrypted snapshots (experimental)

```sh
CGO_ENABLED=0 go run ./examples/ivfflat
CGO_ENABLED=0 go run ./examples/encrypted
```

Native exact retrieval remains the default. IVFFlat supports L2, negative inner product and cosine, immutable trained centroids between explicit rebuilds, and actual list probes with exact rescoring. Probing every list chooses exact scan. [ANN documentation](docs/ann.md) records boundaries and measured recall.

Encryption is selected only through code-configured KeyProvider and sqlite.Options.Encryption (also available through driver.NewConnector). Linux supports explicit full-snapshot mode or immutable final-dirty-page WAL segments with complete-head verification, checkpoint, correct-key rekey/backup/restore and plaintext import/export. Legacy modes use lifetime writable-exclusive/read-only-shared leases. Explicit EncryptionWALShared adds immutable reader selections and a nonblocking single writer, stale-snapshot detection, checkpoint/copy and globally quiescent rekey. Old ciphertext is retained and remains decryptable with old credentials after rekey. Whole-history replay/substitution lacks external freshness/identity protection. Live family operations use a pinned Linux parent directory, but initial resolution and hostile-filesystem protection remain limited. Do not use it for irreplaceable or security-sensitive data; see [encrypted storage](docs/encryption-storage.md).

## Encrypted WAL and migration prerequisites (experimental)

```sh
CGO_ENABLED=0 go run ./examples/encrypted-wal
CGO_ENABLED=0 go run ./cmd/gosqlite migrate pgembedding inspect --catalog testdata/pgembedding-0.3.6/catalog.json --format markdown
```

The encrypted example uses disposable in-memory keys. `EncryptionWAL` retains
authenticated dirty-page segments; normal checkpoint compacts the complete head.
This legacy mode remains exclusive. EncryptionWALShared and examples/encrypted-shared supply concurrent readers and one writer; no mode qualifies physical recovery.

The migration inspector reads bounded local catalog JSON, checks extension/type/
operator ownership and legacy option semantics, and emits a deterministic review
report. It does not connect to or modify PostgreSQL, execute source SQL or import data; those are separate explicit conversion and
local import commands. See [migration scope and examples](docs/migration-pgembedding.md).

`extension/vector` also supplies distinct immutable half, packed-bit and sparse
values, codecs, exact metrics, nullable adapters, subvector and quantization.
BIT, HALFVEC and SPARSEVEC also have typed SQL/storage/exact search. HALFVEC/BIT/SPARSEVEC now have supported real HNSW and IVF paths; see the exact matrix in docs/typed-ann.md.
See [extended value contracts](extension/vector/extended-values.md).

## Transaction controls and packed-bit search

Retained native/driver connections support lazy BEGIN, IMMEDIATE/EXCLUSIVE,
SAVEPOINT, RELEASE and ROLLBACK TO with bounded aggregate undo. See
[transaction behavior](docs/transactions.md) for snapshot timing, driver cleanup
and current active-Rows restrictions. A one-statement CLI invocation cannot carry
a transaction into another invocation.

BIT(1..64000) stores strict packed GSB values. Hamming/<~>, Jaccard/<%>, native
DB/Tx.BitSearch defaults exact; matching HNSW Hamming/Jaccard and IVF Hamming
indexes provide explicitly approximate paths, with genuine graph/list visits.
Dense subvector and binary_quantize are explicit transforms. See
[bit SQL and search](docs/bit-vector.md) and the runnable examples/bit-rerank;
its deliberately truncated bit candidates demonstrate lost dense recall.

## Offline pg_embedding SQL conversion

The bounded converter binds a PostgreSQL subset to inspected catalog identities
and explicit target names. Local output is reparsed and exercised against the
native engine; PostgreSQL output is review-only and has not been run remotely.
Unknown identities or unsupported semantics block the entire apply-ready script.
Runtime bind adapters and query-setting artifacts are part of the contract,
including NULL/width/range checks. See [converter boundaries](docs/migration-converter.md)
and the runnable examples/migrate-pgembedding. Bounded native CSV/NDJSON import
and offline review-only transition bundles are separate implemented workflows;
see [import](docs/import-pgembedding.md) and [transition CLI](docs/migration-transition-cli.md).
A bounded review-only PG16 full-table copy/swap generator now exists; see
[whole-table scope](docs/migration-full-table.md). Online cutover, unsupported
dependency families and live PostgreSQL qualification remain open.

## Half-precision vectors

HALFVEC(1..16000) stores distinct GSH binary16 values, with strict typed SQL,
DB/Tx.HalfSearch and exact bounded ranking. Conversions and loss-report functions
are explicit; dense and half representations do not silently mix. See
[half-precision SQL](docs/half-vector.md) and examples/half-rerank, which measures
lost dense recall from a truncated rounded candidate set. Half HNSW supports all four numeric metrics; half IVF supports L2/IP/cosine,
with a narrower indexed4000-dimension limit.

## Native platform and encrypted concurrency boundaries

Plain Darwin uses managed inode-wide process-lock aggregation and deferred closes;
Windows/amd64 uses checked native handles, exact LockFileEx ownership and aligned
SHM views. Neither target was executed here. Windows default cross-vet retains
one documented unsafe-pointer mapping diagnostic. Darwin fsync and Windows
namespace publication do not claim device/physical-durability qualification.
See [native policy](docs/decisions/0015-native-darwin-windows-backends.md).

On Linux, EncryptionWALShared explicitly selects authenticated layout3. Readers
retain an immutable authenticated head while a writer commits or checkpoints;
legacy layouts are never silently upgraded. Run examples/encrypted-shared only
with disposable data. A Windows/amd64 encrypted backend now implements legacy
and shared-session paths under its strict local-NTFS policy, with native runtime
unverified. Darwin encrypted modes0/1/2 now use a shared plain/encrypted inode
coordinator, with strict ASCII inventory limits. Windows shared rekey and Darwin
Unicode inventory remain required implementation work. Retained-history collection,
external freshness guarantees and independent security acceptance remain open.

## Sparse exact vectors

SPARSEVEC(1..1e9) stores a distinct GSS value with at most16000 nonzero binary32
entries. Native and SQL exact ranking charge actual supports, never the declared
billion-dimensional coordinate space. Four numeric metrics, strict driver/default/
DML semantics and explicit bounded conversion are implemented. Sparse HNSW
supports all four metrics with at most1000 nonzeros per indexed/query value;
sparse IVF and unbounded densification remain refused. See [sparse SQL](docs/sparse-vector.md)
and examples/sparse-exact for true large-dimension, small-support operations.

## Offline CSV/NDJSON import and verification

The local pg_embedding importer defaults to read-only validation. Explicit --apply
creates a new native database; --resume uses a committed in-database cursor after
normal close/reopen. Exact int64 keys, NULLs and directly rounded binary32 vector
bits are checked exhaustively against the supplied immutable export. Optional
quarantine reports rejected and genuinely missing keys separately. Real HNSW/IVF
indexes build after rows, with explicit pending states on semantic/index limits.
See [import contract](docs/import-pgembedding.md) and examples/pgembedding-import.
This does not migrate server-side grants/triggers/policies or establish crash safety.

## Scalar expression expansion

SELECT ALL, lazy simple/searched CASE, CAST, BETWEEN and IN-lists now execute
natively, with SQLite affinity/NULL behavior and explicit NULLS FIRST/LAST order.
Vector representations remain distinct through supported value-preserving trees;
CAST is not a replacement for explicit vector constructors/conversion. See
[expression scope](docs/sql-expressions.md). Bounded INNER/CROSS/comma/LEFT joins
now use scoped binding and a single-snapshot pull graph; see
[relational scope and budgets](docs/relational-sql.md). USING/NATURAL/RIGHT/FULL,
subqueries, COLLATE, aggregates and broader relational forms remain unimplemented.
Inherited eager AND/OR error suppression differs from SQLite and remains a code gap.

## Review-only PostgreSQL transition bundles

The bounded offline generator and `migrate pgembedding transition` CLI emit
13 deterministic review artifacts for retained real-array casts or a typed shadow
column, with single-transaction or explicit batch plans. Mutation and post-install
SQL stays commented; all human/capability/dependency gates remain open. No generated
SQL is executed. Explicit target index retuning, global hnsw access-method collision,
PK cursor state, exhaustive binary32 verification and rollback limits are included.
See [transition contracts](docs/migration-transition.md). Full-table/dependency
recreation, online operation and actual PostgreSQL validation remain required work.

## Windows managed metadata and encrypted storage

Windows/amd64 now has handle-relative encrypted publication and exact lifecycle/
writer/capture locks. Public plain and encrypted Open paths use managed bounded
metadata observations; full native IDs and uncertain-close tombstones are retained.
These are source/model and cross-build results, not Windows runtime acceptance.
Shared encrypted rekey remains unsupported, directory namespace barriers are not
supplied, and default Windows VFS vet retains a disclosed mapping diagnostic. See
[encrypted policy](docs/native-encrypted-windows.md) and
[metadata correction](docs/native-windows-metadata.md).

## Darwin coordination and failed-open ownership

Darwin uses one bounded descriptor registry across plain/encrypted backends,
with deferred close, native relative namespace operations and direct bounded
directory enumeration. Native execution is still NOT_RUN. See
[Darwin scope](docs/native-encrypted-darwin.md). Shared-WAL unsuccessful opening
now retains nested owners and protection in bounded groups, with explicit
[retry API](docs/shared-open-cleanup.md). Native WALShared successful Close now
uses separately bounded retained ownership and explicit discover/retry APIs.
Rollback/WALExclusive and encrypted lifecycle cleanup still require code fixes.

## Persistent typed ANN

HALFVEC and SPARSEVEC support real HNSW L2/IP/cosine/L1; BIT supports real HNSW
Hamming/Jaccard. Half IVF supports L2/IP/cosine, bit IVF Hamming. Sparse IVF, IVF
L1 and IVF Jaccard remain unsupported. Dense v1 bytes and existing public struct
layouts remain unchanged, with bidirectional old/new fixture checks. See
[typed ANN limits and evidence](docs/typed-ann.md) and examples/typed-ann-rerank.
Ordinary measured recall depends on workload/probes and is not an exact guarantee.

## Native shared-WAL close ownership

`DB.Close` logically closes native WALShared once, but retries unresolved managed
retirement. `ErrCloseCleanupPending`, `RetryCloseCleanup` and
`PendingCloseCleanups` expose deliberate retry even when database/sql discarded
a Close error. At most1024 native shared sessions/in-flight opens/pending closes
share the new lifetime admission bound; the64 failed-open groups are separate.
See [scope and lifecycle limits](docs/shared-close-cleanup.md). Terminal ownership
can remain retained for process lifetime; native failure behavior is NOT_RUN.

## Encrypted cleanup ownership

Encrypted constructors, selected objects, copies and sessions now retain bounded
managed cleanup obligations separately from DB lifetime. ErrEncryptedCleanupPending,
PendingEncryptedCleanups and RetryEncryptedCleanup expose explicit discovery and
retry; logical closure and optional checkpoint happen once. Native terminal close
uncertainty never authorizes another raw handle close. Admission limits are1024
opening/live/pending scopes and16384 common managed leases, with lower platform
limits also applicable. See [encrypted ownership and remaining gates](docs/encrypted-cleanup.md).
This remains development code with native failure/OS and security acceptance open.
