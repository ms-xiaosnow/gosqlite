# gosqlite

## CHECK / EOF 检查点：完整正常门已通过（2026-10-12 07:22 UTC）

冻结运行树 `e06f35a022536a8ebd157731ee9a860521e70eb0` 于 **07:22:18 UTC**
自然完成 **26 门 / 28 次 Go 调用 PASS**，session 80563 exit 0，lease RELEASED。
ordinary/race 各 **8,948 named PASS / 7 个明确 SKIP**，完整 pinned oracle
**10,687 PASS / 0 named SKIP**。本副本只增加状态文档，运行源码与该冻结树一致；
本副本的新提交、bundle、精确恢复运行及对应 CI 均 **PENDING**。

CHECK 仅资格[已记录的有界 scalar 子集](docs/check-constraints.md#精确-partial-profile)。
EOF-count 仍仅为 decode-only、EOFUnproven；native owner 在写入/Sync 前继续
finite-only fail-closed，未开放 EOF 恢复。外平台只有编译/符号检查，原生手工、
物理持久性/恢复、安全及 M3/M8/M10 未验收。后续 Darwin/Windows backend 和
真实只读媒体 harness 未纳入；其 source-only/HOLD 不被本次 PASS 覆盖。

已发布基点 `e092f0d4623cb243eec6053a4b3db7eef598542f` 的 bundle、精确恢复
26 门及 CI 38209370406 两项 SUCCESS 已另行闭合，只资格该基点。
[本检查点中文报告、精确收据与未完成项](docs/evidence/check-eof-checkpoint-20261012.md)。

从下一节起保留各阶段原文；旧“当前”、SOURCE_ONLY、NOT_RUN、PENDING 与
FAIL/HOLD/UNKNOWN 均只指原时点和原树，不能覆盖本节的新执行记录，也不会被回写。

## 当前 CHECK / EOF-count 源码组合（2026-10-12）

本副本在已发布检查点 `e092f0d4623cb243eec6053a4b3db7eef598542f` / tree
`d8ff20a26b4aee9fe3281a426e41230b18c727cd` 上，仅合入已审阅的有界 scalar CHECK
与 decode-only EOF-count 增量。组合仍为 **SOURCE_ONLY / ALL_COMBINED_EXECUTION_NOT_RUN**；
前驱 V1 的 sqlite.go pin HOLD 原样保留；本 V2 已按独立完整文件 ownership/consumer
复审，仅更新 WAL publisher guard 的该一个 hash，未运行 guard。没有运行本组合的 Go、formatter、compiler、guard、tests、
race、oracle、native、restore 或 CI；输入有限 PASS 不转授本组合。

CHECK 原 tree9c3b 的三普通门已取得 18 根 / 28 named PASS，有限 oracle 为 3 根 / 7 named
PASS。EOF 原 tree1f599764 的四普通门终态记录为 38 Test 根加 2 个固定种子 Fuzz 根、
485 named PASS；只资格其具名内存/源码范围，不是物理恢复。已发布基点的精确恢复运行
与对应 CI 仍待独立闭合；本源码组合尚未发布。准确身份、原始收据及下一步见
[本组合范围与证据边界](docs/check-eof-integration-20261012.md)。

以下保留各输入与检查点原时点记录；其中“当前”、未合入、NOT_RUN、PENDING 和 PASS 均只绑定原树，不覆盖上面的本组合状态。

## 2026-10-12 五切片组合检查点完整正常门通过

冻结树 `59ce26879c4301babc7648fbddc5cc66c4097263`（3,267 文件、783 个 Go 文件）
包含 LEFT/Rows、加密 inspect、Darwin inventory 手工入口、Connector rollback 手工入口和
automatic UNIQUE（含 c382 oracle 表头修复）；**CHECK 未纳入**。
完整 **26 门 / 28 次 Go 调用于 05:21:06 UTC 自然 PASS**，lease 已 RELEASED。
ordinary/race 各 **8,569 named PASS / 7 个固定 SKIP**，完整 pinned oracle **10,301 PASS / 0 SKIP**；
LIMIT oracle/race **394 PASS**。跨编译/符号检查未执行外国映像。
本次只叠加状态文档；新提交、Git bundle、精确恢复及对应 CI 均 **PENDING**。
未发布 tree59ce 源包已保存并读回；它不代替新检查点的发布与恢复运行。
已发布 main 仍为 `8bc189474bdb2b0a683c6c059992f9acbc426250`，其 bundle、恢复 26 门和两项 CI 已闭合。
原生手工/物理资格及真实实现缺口仍在，不构成生产或 M3/M8/M10 验收。
见[当前状态与余缺](docs/current-status-20261012.md)和[本组合证据](docs/evidence/combined-checkpoint-20261012.md)。

## 历史记录：2026-10-12 Storage + UNIQUE development checkpoint

The frozen source tree `4008bd9442ffd21e1e993d4f1c72dbeed3c910f3` (3,238 files) naturally
completed **all 26 gates / 28 Go invocations at 00:49:41 UTC**, with its lease
released. Ordinary and race each passed **8,231 named cases / 7 expected oracle
skips**; the complete pinned oracle passed **9,925 / zero skips**, and LIMIT
oracle/race passed **394**. Foreign images were compiled and inspected, never run.
This checkpoint adds only documentation to that tested source; its new commit,
bundle, exact restore and matching hosted CI are still **PENDING**.
The bounded explicit ordinary UNIQUE index profile is included. Strict Linux
plaintext Backup and opt-in WAL tail publication remain **PARTIAL / NATIVE_NOT_RUN**,
including native Backup of UNIQUE databases. This is not production-ready or M10
acceptance. See [exact evidence and remaining gaps](docs/evidence/storage-unique-checkpoint-20261012.md).

### Historical RO-WAL / IPK checkpoint (2026-10-11; retained original record)

2026-10-11 当前未发布组合：Linux 显式 `ReadOnly + WALShared` 的 live-index
真只读连接、`--wal-readonly` query/check/inspect、有限 IPK equality 修正与
健康官方 live-writer 测试已纳入。精确冻结树 `cace45621c10259231a2e764cfbefdcdbe312ccf`
已取得普通门 5,758 PASS / 7 个固定 oracle SKIP、live-writer 7 named PASS、
原 SQL 九进程 oracle 9 PASS；完整 26 项于 **22:05:15 UTC 自然 PASS**，
race 5,758 PASS / 7 SKIP、完整 pinned oracle 7,431 PASS / 零 SKIP。
原组合四根 FAIL、测试专用四文件修正和输入切片结果分别保留，见
[进展](docs/progress.md)和[精确证据](docs/evidence/readonly-ipk-combined-20261011.md)。
冷 WAL、缺失/未初始化 SHM、其他 OS 的只读组合仍不支持；真实只读媒体/
非特权身份、持久性、安全、M3/M10 与完整目标仍未验收，见
[准确边界](docs/plain-readonly-wal.md)。本段更新不继承下列旧检查点资格。

Current source update (2026-10-11): this unpublished Windows shared-rekey
composition adds guarded lifecycle admission/conversion and a post-conversion
same-source-key authority check. Native Windows execution remains **NOT_RUN**.
The exact source identity, separate Linux results, completed seventeen-gate code qualification,
and published-main boundary are in [the protocol record](docs/windows-shared-rekey.md).


Historical test-scope statements are corrected in [the namespace-test audit](docs/test-scope-audit.md); the affected tests are now excluded from ordinary workflows.

A native Go database kernel targeting tested subsets of SQLite 3.54.0 files and SQL. Current development: persistent HNSW/IVFFlat, exact typed-vector KNN and same-snapshot hybrid ranking, plus experimental authenticated snapshot, dirty-page WAL and concurrent encrypted sessions, ordinary B-tree equality, leading inequalities and bounded rowid-range planning, nested savepoints, scoped INNER/CROSS/LEFT JOINs and bounded global aggregates, CASE/CAST/BETWEEN/IN, context-sensitive predicate evaluation, affinity-correct comparisons and explicit NULL ordering, BIT/HALFVEC/SPARSEVEC exact search, extended vector primitives, and bounded pg_embedding catalog inspection, typed offline SQL conversion and resumable CSV/NDJSON import. It is not production-ready. No CGO, SQLite subprocess, C ABI, or remote SQL engine is used in production.

## 示例临时目录的有限保留约定

`encrypted`、`encrypted-wal`、`encrypted-shared`、`encrypted-cli` 和
`native-check` 只在普通流程完成、输出成功且全部已有资源收尾未返回错误后，
才尝试删除自己的私有临时目录。失败或未完成时不调用删除，并在返回错误中
报告保留目录；删除本身失败会保留错误，不重试，但可能已部分删除，不能承诺
完整现场。加密 provider/密钥清理语义保持不变；保留密文不代表保留密钥或仍可
解密，也不构成安全擦除、崩溃恢复或任意故障场景的证明。此独立切片仅通过
纯 helper/源码检查及既有 WAL/shared 普通成功路径的定名普通/race 门和 scoped
vet/build；没有执行真实失败保留或 snapshot/CLI/native-check 示例。手册中的
历史限制不能用候选源码改写为已经运行的证据。

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
CGO_ENABLED=0 go run ./cmd/gosqlite inspect --extensions example.db
```

Each CLI invocation executes one statement. Writes autocommit and return JSON with rows_affected and last_insert_id. Explicit transactions use the Go API; BEGIN/COMMIT in separate CLI invocations are rejected. SELECT opens read-only and writes JSON Lines: a columns object followed by row objects. BLOB values are base64. Inspect reads header/schema metadata from one snapshot. Opt-in `inspect --extensions <db>` also reports persisted index identity, method/storage versions, opaque configuration strings and generations. It does not validate index integrity or usability or invoke registered method callbacks. Default inspect JSON is unchanged. See [inspection contract](docs/cli-inspect.md). Unsupported commands return errors.

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

上述片段要求已有 `docs` 表；`examples/readonly` 使用已有 `samples` 表，并通过 `driver.NewConnector` 显式设置 `ReadOnly:true, NoCreate:true`，即使传入 `mode=rw/rwc` 也不会放宽只读限制。该可执行示例传递上下文、检查输出错误，且在诊断和退出前依次释放 Rows 与 DB，保留迭代和关闭错误。 For writes, use a normal path or mode=rwc DSN, or mode=rw to require an existing file. Native sqlite.Options{} is writable/create; ReadOnly and NoCreate are explicit alternatives. See examples/write for prepared inserts and transactions. Native transactions can retry BUSY Commit after readers close; database/sql transactions cannot be reused after Commit, so the adapter rolls back on an error. Close Rows promptly to release their snapshot locks.

CREATE TABLE, nonunique BINARY ASC CREATE INDEX, explicit ordinary named-column BINARY ASC CREATE UNIQUE INDEX, INSERT, UPDATE, DELETE and retained-connection SQL/native/driver transactions with nested savepoints are implemented for the documented subset. The [bounded UNIQUE profile](docs/unique-index.md) enforces immediate non-NULL uniqueness and Check consistency; table/column UNIQUE constraints, implicit autoindexes, partial/expression/DESC/non-BINARY indexes and conflict policies/UPSERT remain unsupported. Ordinary SELECT planners still skip UNIQUE indexes. Ordinary secondary indices are maintained on disk. Registered grams13 posting indices accelerate supported LIKE and configured MATCH predicates. Eligible rowid equality/inequality and ordinary leading equality-prefix/leading-field inequality predicates use real source/index seeks; proven rowid ordering can stream without sorting, while unsupported shapes scan. See [ordinary index planning](docs/ordinary-index.md). Unsupported schema objects/constraints reject writes to the database rather than being ignored. Existing journals require recovery and are refused. Existing zero-byte files are conservatively rejected. Normal successful commit tests do not qualify crash durability; use only disposable data.

If home is read-only, set GOCACHE and GOPATH to writable directories. The Unicode tokenizer uses pinned pure-Go golang.org/x/text; go.sum records verified module checksums and third_party/ retains its license. Official SQLite is a separately built test oracle only:

```sh
SQLITE_ORACLE=/absolute/path/to/verified/sqlite3 ./scripts/test-oracle.sh
```

See [progress](docs/progress.md), [compatibility](docs/compatibility.md), [architecture](docs/architecture.md), [sources](docs/upstream.md), and [manual verification](docs/manual-verification.md). The unchanged [original goal](goal.md) remains the contract. [Restart protocol](docs/restart.md) and checkpoint records preserve durable development history.

Qualified crash recovery, Darwin Unicode inventory, broader SQL and online/dependency-complete pg_embedding transition work remain unfinished implementation work. Windows shared rekey is implemented in this unpublished source composition; native Windows and failure/durability qualification remain open. Full SQLite/PostgreSQL semantics, SEE file compatibility and production security are not claimed.


## Explicit exclusive WAL

The developing M3 mode is opt-in: sqlite.Options{WALExclusive:true}, or driver.NewConnector with that option and sql.DB.SetMaxOpenConns(1). It holds an EXCLUSIVE database lock for the connection lifetime, including idle time, and permits one active query/transaction lease. Other connections correctly receive BUSY. It is not shared concurrent WAL.

```sh
CGO_ENABLED=0 go run ./examples/wal disposable-wal.db
CGO_ENABLED=0 go run ./cmd/gosqlite query --wal-exclusive disposable-wal.db 'SELECT id,text FROM notes'
```

Native DB.Checkpoint(ctx) performs an explicit normal checkpoint. Default Close checkpoints; KeepWALOnClose retains successful committed frames after normal close. An exclusive WAL connection requires writable access even for SELECT. The default rollback-only ReadOnly entry still refuses mode2; the new explicit Linux live-index readonly subset is described separately above. Unsupported WAL tails and existing journals fail closed. Hot rollback-recovery code is isolated and source-only; no crash/fault durability gate has passed. See ADR0004 and manual-verification.md.

## Explicit shared WAL

Use sqlite.Options{WALShared:true}, or driver.NewConnector with that option for pooled concurrent connections. Each connection permits one active lease; independent connections retain read snapshots while a single writer commits. Stale deferred writer promotion returns ErrBusySnapshot (also matches ErrBusy).

```sh
CGO_ENABLED=0 go run ./cmd/gosqlite query --wal-shared disposable-shared.db 'CREATE TABLE notes(id INTEGER PRIMARY KEY, text TEXT)'
CGO_ENABLED=0 go run ./cmd/gosqlite query --wal-shared disposable-shared.db "INSERT INTO notes VALUES(1,'共享 WAL')"
```

DB.Checkpoint is nonblocking and may safely backfill some frames before returning BUSY for readers. Close attempts a checkpoint unless KeepWALOnClose and retains WAL/SHM inodes. Without ReadOnly, this existing WALShared route requires writable access, including for read transactions. The explicit ReadOnly combination is a bounded Linux live-index subset with exact combined-tree ordinary and healthy live-writer evidence above; denied-write media, other native platforms and broader acceptance remain unqualified. Conservative storage-sector protection defaults on; WALPowersafeOverwrite is an explicit storage assertion and is never autodetected. Live official SQLite reader/writer interoperability is tested, but crash/fault durability and broader cold recovery remain unqualified. See ADR0006 and the compatibility matrix.

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

Library encryption uses code-configured KeyProvider and sqlite.Options.Encryption (also available through driver.NewConnector). The encrypted backup/rekey CLI additionally accepts explicit environment-variable names or bounded stdin providers; credentials are never argument values. Linux supports explicit full-snapshot mode or immutable final-dirty-page WAL segments with complete-head verification, checkpoint, correct-key rekey/backup/restore and plaintext import/export. Legacy modes use lifetime writable-exclusive/read-only-shared leases. Explicit EncryptionWALShared adds immutable reader selections and a nonblocking single writer, stale-snapshot detection, checkpoint/copy and globally quiescent rekey. Old ciphertext is retained and remains decryptable with old credentials after rekey. Whole-history replay/substitution lacks external freshness/identity protection. Live family operations use a pinned Linux parent directory, but initial resolution and hostile-filesystem protection remain limited. Do not use it for irreplaceable or security-sensitive data; see [encrypted storage](docs/encryption-storage.md).

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
coordinator, with strict ASCII inventory limits. Windows shared rekey now uses
guarded conversion with a post-conversion source-authority check; native execution
remains NOT_RUN. Darwin Unicode inventory remains required implementation work. Retained-history collection,
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
subqueries, COLLATE, GROUP BY/HAVING and broader relational forms remain unimplemented.
Bounded global aggregates, context-sensitive Boolean evaluation and comparison
affinity now have documented native implementations; planner-dependent error-order
differences remain explicit code gaps.

## Review-only PostgreSQL transition bundles

The bounded offline generator and `migrate pgembedding transition` CLI emit
13 deterministic review artifacts for retained real-array casts or a typed shadow
column, with single-transaction or explicit batch plans. Mutation and post-install
SQL stays commented; all human/capability/dependency gates remain open. No generated
SQL is executed. Explicit target index retuning, global hnsw access-method collision,
PK cursor state, exhaustive binary32 verification and rollback limits are included.
See [transition contracts](docs/migration-transition.md). A bounded review-only full-table generator is now implemented. Unsupported
dependency recreation, online operation and actual PostgreSQL validation remain open.

## Windows managed metadata and encrypted storage

Windows/amd64 now has handle-relative encrypted publication and exact lifecycle/
writer/capture locks. Public plain and encrypted Open paths use managed bounded
metadata observations; full native IDs and uncertain-close tombstones are retained.
These are source/model and cross-build results, not Windows runtime acceptance.
Shared encrypted rekey has bounded guarded-conversion source in this composition;
native Windows behavior remains unqualified. Directory namespace barriers are not
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
Encrypted lifecycle cleanup is now implemented with bounded pending ownership.
Rollback Reader/Tx, WALExclusive and public plaintext metadata adoption now use
bounded minimal-owner cleanup with explicit pending/retry APIs. Native recovery
ownership and physical failure qualification remain separate work.

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

## Plaintext cleanup ownership

Plain rollback Reader/Tx and exclusive WAL now reserve bounded cleanup roots
before native acquisition. Public metadata preludes transfer the same admission
into the pager, and pending obligations remain discoverable after a failed Open
or discarded driver error. Children retire before the main database owner;
retry never repeats logical replay, checkpoint or publication. See
[plaintext ownership](docs/plaintext-cleanup.md),
[transaction cleanup](docs/plaintext-tx-cleanup.md),
[exclusive WAL cleanup](docs/plaintext-exclusive-cleanup.md) and
[public admission](docs/public-open-adoption.md). Physical failure paths and
Windows/Darwin runtime remain unqualified.

The required remaining work is separated by goal clause and evidence in
[acceptance status](docs/acceptance-status.md); full M10 acceptance is pending.

Supported bounded GROUP BY/HAVING, connection SQL counters and rollback cached
ordinary preparation are described in [grouped aggregates](docs/grouped-aggregates.md),
[connection state](docs/sql-connection-counters.md) and
[preparation boundaries](docs/sql-cached-preparation.md). The profile remains
explicitly partial; final M10 qualification is outstanding.

Required tooling now includes [encrypted CLI backup/rekey](docs/cli-encryption.md),
[caller-dataset numerical verification](docs/import-distance-verification.md) and
[grams measurements](benchmarks/grams/README.md). This source also contains a
[strict Linux rollback plaintext Backup](docs/plaintext-backup.md) API/CLI and an
[explicit exclusive-WAL tail publisher](docs/decisions/0017-wal-tail-prefix.md);
both remain partial and native NOT_RUN under their strict filesystem/namespace
contracts. General plaintext Backup, WAL/readonly-WAL backup sources, broader
platforms/filesystems, cold readonly WAL and final release qualification remain
open. The separate Linux initialized live-SHM readonly-WAL subset has prior
ordinary/live-writer evidence; it does not qualify either new publisher.

[Native full-profile DB/Tx/CLI checks](docs/native-check.md) and the
[ordinary transaction/concurrent-soak harness](benchmarks/transactions/README.md)
are available with independently reviewed bounded contracts. Whole-database
checking verifies persisted built-in semantics, not runtime custom-registry
provenance. A two-hour workload is predeclared, not yet executed or accepted.
