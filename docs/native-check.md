# Native supported-profile integrity checking

`DB.Check(ctx[, CheckOptions])`, `Tx.Check(ctx[, CheckOptions])` and
`gosqlite check <db>` implement a native bounded integrity/index-consistency
outcome for goal §12.2. Production checking never invokes an external SQL engine.
Success names `gosqlite-supported-profile-v1`; it does not claim general SQLite
`PRAGMA integrity_check` parity, crash durability or a security audit.

## What a pass covers

One pinned logical snapshot is used throughout, including catalog, freelist,
ordinary tables, indexes, overflow and every extension check. The checker:

1. Revalidates the selected logical header, SQLite magic, page geometry, UTF-8
   encoding and supported schema format. WAL's selected logical extent is kept
   distinct from the physical main-file size; no live main/WAL file is copied.
2. Parses the complete supported catalog and rejects unknown/unimplemented
   objects and constraints. It reuses the conservative writable-catalog grammar
   solely as a read-only validator. Exact owned extension invalidation triggers
   are recognized; arbitrary triggers/views are not silently skipped.
3. Validates freelist trunks/leaves, all tree/overflow encodings and ordering,
   cycles, depth, exact cell/freeblock/1–3-byte-fragment accounting, record shape,
   key arity/rowid type, cross-tree page ownership,
   and every logical page's accounting. The reserved SQLite lock-byte page is
   the sole explicitly reserved page, rather than a fabricated tree owner.
4. Reads every table record. INTEGER PRIMARY KEY decoded storage values must be logical NULL and
   resolve to the signed rowid; short records use supported literal defaults.
   Declared NOT NULL constraints and strict VECTOR/HALFVEC/BIT/SPARSEVEC
   interpretations are checked. Ordinary SQLite affinity remains dynamic typing.
5. Reconciles ordinary indexes in both directions. Every table row must have its
   expected complete BINARY ASC key plus rowid. Every index entry must resolve to
   a source row and all indexed values must compare equal under SQLite storage
   classes and exact mixed INTEGER/REAL comparison. Cardinality alone and a
   structure-only traversal cannot pass missing/orphan/extra-wrong-value keys.
6. Checks every grams13 index's source, postings and exact frequencies, and the
   existing supported dense/typed HNSW graph or IVFFlat source/state/membership
   invariants. The ordinary B-tree audit also covers every shadow lookup index.
   Shadow table rows must have a real logical owner of the correct method;
   orphan rows outside a named index's range cannot escape a full check.

Tables/indexes are traversed in sorted names for stable work/count reporting.
No source rows or synthetic replacement indexes are retained wholesale. A pass
is about the checked snapshot; another connection may commit after it ends.

## Fail-closed profile

Supported files have no reserved page bytes, schema format 4 (or an empty
format-0 database), UTF-8, ordinary rowid tables with the engine's supported
simple column types/defaults/NOT NULL/INTEGER PRIMARY KEY, and nonunique ascending
BINARY ordinary indexes. Supported owned extensions are grams13 and the current
native dense/typed HNSW and IVFFlat configurations.

Unsupported features fail instead of being reported as checked: WITHOUT ROWID,
STRICT, autovacuum, UTF-16, schema formats 1–3, UNIQUE/autoindexes, other primary
keys, arbitrary CHECK/foreign-key constraints, partial/expression/DESC/non-BINARY
indexes, generated columns, arbitrary views/triggers, sqlite_* internal objects,
unknown schema/extension formats or persisted custom method identities. Full
checks use a fresh built-in-only registry and never invoke application-provided
Configure/Keys callbacks, including implementations registered as grams13 v1.
Persisted grams13 v1 data is checked against actual built-in semantics regardless
of the application registry. This does not audit the provenance of that registry.
A same-name custom implementation can pass only when its persisted data also
satisfies the built-in profile; unsupported persisted versions fail. Named
extension checks keep their application-directed registry behavior. This is narrower
than the engine's ability to SELECT an otherwise readable table while ignoring
unsupported peer objects.

Malformed UTF-8 TEXT is explicitly outside this check profile and returns
`ErrUnsupported`; the engine's raw-byte read/equality behavior is unchanged.
A typed value mismatch fails the check. Dirty, missing-guard or schema-mismatched
extensions return `ErrReindexRequired`, never an incomplete successful report.
The checker does not rebuild them. ANN consistency does not certify recall,
optimal topology, speed, training quality or pgvector binary compatibility.

## Snapshot and ownership contract

Without an active transaction, `DB.Check` opens one reader and closes it before
returning a result. A read or close error returns the zero `CheckResult` and an
error. It never acquires/promotes a writer, creates a transaction journal,
checkpoints, repairs or changes `changes()`/`total_changes()`/`last_insert_rowid()`.

With an active transaction, `DB.Check` and `Tx.Check` use its reader, including
uncommitted own writes. First access pins a lazy transaction through its existing
read path. A writable deferred transaction may therefore retain a read-state
pager transaction, but is not promoted to a writer by Check. The caller still
owns that transaction and its Rows; a failed check does not roll it back or
invalidate its cursor. Logical NULL follows the existing record decoder, which maps a stored REAL NaN
to NULL; this checker does not promise a physical serial-type-0-only IPK audit.
A transaction-context or operation-context cancellation
fails the check. Existing single-session lease restrictions still apply to
unrelated live Rows. Public `Options` and statement `Result` layouts are unchanged.

Encrypted APIs operate on the existing authenticated logical snapshot with its
configured key provider. Plaintext buffers receive the same lifecycle as normal
readers; this checker makes no absolute erasure or new cryptographic audit claim.

## Bounds and reporting

`CheckOptions` is a separate additive type:

- `MaxPages`: maximum logical extent/ownership metadata; zero is 1,048,576 pages,
  which is also the supported maximum. Larger requested bounds are unsupported.
- `MaxPageReads`: cumulative logical PageReader calls across the entire check;
  zero is 16,777,216; the supported maximum is 2^32. Repeated catalog reads, source/index seeks, overflow and
  extension validation all share this bound. Admission fails before the next
  read; there is no partial-success fallback.

These check-specific quotas start after snapshot acquisition, when the check
reader is installed. Acquiring the existing reader/session or a lazy writable
Tx read-state snapshot can perform pager/header/freelist work under the existing
pager limits before these counters begin. These are traversal quotas, not a cap
on every open/acquisition read or allocation.

The existing separate 64 MiB record, 16 MiB retained schema, cursor depth/visited,
connection `MaxSortBytes` and extension construction/query limits remain in force.
These are bounded component limits, not a new aggregate live-heap budget or a
wall-clock SLA. Work can be substantially greater than a table scan because
index reconciliation performs seeks. Cancellation and quota errors return no
successful result and do not weaken the requested scope.

Result fields: `scope`, `pages`, `free_pages` (including trunks), `reserved_pages`,
`tables`, `indexes`, `extension_indexes`, `table_rows`, `index_entries`, and
`page_reads`. Tables/rows and ordinary indexes/entries include extension shadows;
`sqlite_schema` is checked but excluded from those counts. `extension_indexes`
counts logical grams/ANN definitions. Each physical index entry is counted once.
Page reads include repeated logical calls, not physical disk I/O. An empty
database can legitimately report zero tables/rows/indexes.

## CLI

```sh
gosqlite check ordinary.db
gosqlite check ordinary.db docs_grams
gosqlite check encrypted.gse --key-provider env-hex:DB_KEY --mode snapshot
gosqlite check encrypted.gse --key-provider stdin-password --mode shared < key-input
```

No-provider full checks open existing rollback databases read-only/no-create.
Encrypted full checks also open read-only/no-create and support existing snapshot,
exclusive encrypted-WAL and shared encrypted-WAL selections. Modes are explicit;
there is no conversion. Provider descriptors reuse the separately reviewed
[CLI encryption adapter](cli-encryption.md), including explicit raw/password
kinds, bounded pipe/regular-file stdin, lazy acquisition, redacted provider/errors
and best-effort owned-buffer clearing. Password stdin is exact bytes; EOF is
required and an unfinished producer can still block. Keys, passwords or DSNs
must never be supplied as arguments. Environment and immutable-string exposure
limitations remain. JSON includes no decrypted values or provider descriptors.
Success JSON is emitted only after DB.Close completes.

The optional extension-index argument retains the existing narrower named check
and its `"extension source/posting/frequency consistency only"` scope. Named
checks retain the historical leading `--wal-exclusive`/`--wal-shared` syntax;
those routes require writable native WAL coordination and are not filesystem
read-only opens. `inspect` remains metadata inspection only.

显式 `gosqlite check --wal-readonly <db> [<extension-index>]` 已接入
Linux 活动且初始化 SHM 的真只读有限入口。当前冻结树
`cace45621c10259231a2e764cfbefdcdbe312ccf` 已有普通门与健康官方 live-writer
实际 PASS；完整 26 项于 **22:05:15 UTC 自然 PASS**，真实只读媒体/非特权身份
仍未验证。V2 输入与本树结果分开，完整合同与资格分层见
[明文 WAL 只读范围](plain-readonly-wal.md)和[本次证据](evidence/readonly-ipk-combined-20261011.md)。
它通过 `ReadOnly:true, NoCreate:true, WALShared:true` 使用现有快照 checker，
不转换模式、不创建/重建 sidecar、不写 readmark，也不在 Close checkpoint。
缺失/冷/未初始化 sidecar、其他 OS 和无 SHM fallback 仍 unsupported。
原 `--wal-shared/--wal-exclusive` 无 named index 时继续在打开前拒绝，
其 named route 仍需可写权限；默认无 flag 的只读打开仍不自动识别 WAL。
这项新源码不能继承本页以下历史 ordinary 结果。

## Ordinary evidence and unexecuted gates

The focused tests cover memory-only malformed PageReader/schema/record/index
fixtures; missing/extra/orphan ordinary keys; constraints/types/encoding; page
accounting; default/exact/insufficient budgets and normal context cancellation;
intact native and official SQLite 512/4096/65536-page fixtures; overflow values,
short-record defaults, signed rowid extremes and exact numeric keys; DML,
freelist, rollback, transaction cursor/counter ownership, same-snapshot WAL
readers, normal checkpoint/reopen; grams plus dense/half/bit/sparse HNSW and
supported dense/half/bit IVF; correct-key snapshot/WAL/shared encryption; CLI
full/named/inspect/provider paths and unchanged read-only family bytes.

Official SQLite is used only to generate valid test fixtures and independently
check valid SQL output. It never supplies the native check result. Review test
bodies/helpers and run `go run ./internal/testscope` before scoped tests.

No physical corruption/tamper/wrong-key/missing-selected-file test, selected-file
removal/namespace mutation, crash/kill/abrupt exit/hot recovery, native
I/O/lock/unmap/close/Sync/shortwrite/diskfull/permission failure, interrupted
publication/rekey or manual-tag test is part of this slice. No such scenario was
moved into CI. Native Darwin/Windows runtime, hostile namespaces, storage
freshness, crash durability and security/release acceptance remain unqualified.
A successful logical check cannot close any of those gates.

A disposable API-versus-real-executable proof is available:

```sh
CGO_ENABLED=0 go build -o /tmp/gosqlite-check ./cmd/gosqlite
CGO_ENABLED=0 go run ./examples/native-check /tmp/gosqlite-check
```

It creates intact plaintext and correctly keyed encrypted snapshot/WAL/shared
fixtures, invokes the actual CLI with bounded stdin-hex for encrypted checks,
compares complete API/CLI counts and verifies all source-family bytes/names stay
unchanged. Each connection and subprocess finishes normally before the private
temporary directory is cleaned up. It prints only mode and non-secret counts.
