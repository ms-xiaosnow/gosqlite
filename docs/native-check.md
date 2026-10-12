# Native supported-profile integrity checking

## CHECK / EOF 检查点：完整正常门已通过（2026-10-12 07:22 UTC）

冻结运行树 `e06f35a022536a8ebd157731ee9a860521e70eb0` 于 **07:22:18 UTC**
自然完成 **26 门 / 28 次 Go 调用 PASS**，session 80563 exit 0，lease RELEASED。
ordinary/race 各 **8,948 named PASS / 7 个明确 SKIP**，完整 pinned oracle
**10,687 PASS / 0 named SKIP**。本副本只增加状态文档，运行源码与该冻结树一致；
本副本的新提交、bundle、精确恢复运行及对应 CI 均 **PENDING**。

CHECK 仅资格[已记录的有界 scalar 子集](check-constraints.md#精确-partial-profile)。
EOF-count 仍仅为 decode-only、EOFUnproven；native owner 在写入/Sync 前继续
finite-only fail-closed，未开放 EOF 恢复。外平台只有编译/符号检查，原生手工、
物理持久性/恢复、安全及 M3/M8/M10 未验收。后续 Darwin/Windows backend 和
真实只读媒体 harness 未纳入；其 source-only/HOLD 不被本次 PASS 覆盖。

已发布基点 `e092f0d4623cb243eec6053a4b3db7eef598542f` 的 bundle、精确恢复
26 门及 CI 38209370406 两项 SUCCESS 已另行闭合，只资格该基点。
[本检查点中文报告、精确收据与未完成项](evidence/check-eof-checkpoint-20261012.md)。

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
[本组合范围与证据边界](check-eof-integration-20261012.md)。

CHECK 仅支持普通 rowid 表的有界、无显式函数 scalar profile：保留折叠前 source
准入、NULL-pass、UPDATE 赋值依赖、NOT NULL → CHECK → UNIQUE/storage 顺序及原
storage.Statement undo。DB.Check 只在 primary scan 每行评估一次；配额仍是累计 work
2^24、metadata 16 MiB 并受 MaxSortBytes 约束、2,000 声明和 8,192 原始节点；一般
函数、FK、STRICT、WITHOUT ROWID 与更广 SQL 不因此开放。详见[CHECK 合同](check-constraints.md)。

以下保留各输入与检查点原时点记录；其中“当前”、未合入、NOT_RUN、PENDING 和 PASS 均只绑定原树，不覆盖上面的本组合状态。

## 当前自动 UNIQUE 组合入口（2026-10-12）

本文件下方自动 UNIQUE V1/V2 与 Storage 基点段落保留各自身份的历史；最新组合见[当前源码与证据](current-status-20261012.md)。本次在已审阅 LEFT/inspect/Darwin/Connector v4 上并入 automatic UNIQUE 原基线增量，并接入独立复审通过的 c382 空结果表头观察器窄修；未加入 CHECK。前序 d79 组合及旧 sqlite.go pin HOLD 原样冻结，本后继仅按独立完整 ownership 复审更新该一个 pin。组合 **SOURCE_ONLY / NOT_RUN**；原 automatic UNIQUE 六门 oracle FAIL 保留，修后原 gate11 运行资格仍待其独立终态，输入成功不转授组合。

> 2026-10-12 V2 精确 schema 预算修正：前驱 `c065143baa0e61d0c4aae942ae2b5051b26b28f2`
> 因 AU-1（owner/table/已有目录和行数预算遗漏）保留 **SOURCE_HOLD**。本 V2 仅做
> 共享 ReadSchema 真实计费、完整当前目录与新增行的写入前准入，以及纯模型/小型 quota
> 原子失败测试源码；仍 **SOURCE_ONLY / NOT_RUN**，待独立复审，不能继承前驱或基线结果。

> 2026-10-12 自动 UNIQUE 新切片：**SOURCE_ONLY / NOT_RUN**。本树在冻结
> `4008bd9442ffd21e1e993d4f1c72dbeed3c910f3` 上新增普通 rowid 列/表 UNIQUE、
> 精确 autoindex 所有权/NULL 元数据和原子 CREATE；基线的完整 26 门 PASS 不传递。
> 本切片尚未格式化、编译、测试或 oracle；下方旧“当前”段落是各自身份的历史。
> [准确范围、API/JSON 兼容决策、来源与有限提案](automatic-unique.md)。

## 2026-10-12 Storage + UNIQUE：冻结组合完整 26 门 PASS

冻结运行树 `4008bd9442ffd21e1e993d4f1c72dbeed3c910f3`（3,238 文件）于 **00:49:41 UTC**
自然完成 **26/26 门、28 次 Go 调用 PASS**，lease 已 RELEASED。ordinary/race
各 **8,231 named PASS / 7 个预期 oracle SKIP**；完整 pinned oracle **9,925 PASS /
零 SKIP**，LIMIT oracle/race **394 PASS**。跨编译与外平台 test-c/nm 只证明编译/
符号，未执行外国映像。本次仅叠加文档；新提交、bundle、精确恢复及对应 CI 仍 **PENDING**。
显式普通 BINARY ASC UNIQUE 子集已纳入；严格 Linux Backup/WAL publisher 仍为
**PARTIAL / NATIVE_NOT_RUN**，包括 UNIQUE 数据库的 native Backup；M10 未验收。
输入有限结果、原失败、准确终态和真实代码缺口见[本批精确证据](evidence/storage-unique-checkpoint-20261012.md)。

### 历史组合准备摘要（2026-10-12，完整运行前；原文保留）

> 2026-10-12 当前组合状态：Storage metadata 修正树 `7389e7b7` 的三条回归
> 已实际 PASS（14 根 / 92 named PASS），UNIQUE 树 `fd4e3471` 的十二条有限门
> 已实际 PASS（31 根 / 223 named PASS / 零 SKIP）；两者均自然退出并释放 lease。
> 本文所在新 Storage + UNIQUE 组合仍是 **SOURCE_ONLY / NOT_RUN**：完整 ordinary、
> race、oracle、跨编译/依赖、发布/精确恢复/对应 CI 尚未运行。下文旧的
> SOURCE_ONLY、PENDING、FAIL 与 NOT_RUN 均保留为各自身份的历史，不覆盖上述有限结果。
> 原 V1 显示契约 FAIL 与 V2 metadata 真失败（8,126 PASS / 1 FAIL / 7 SKIP；
> 后续 19 NOT_RUN）不改写。native Backup（包括 UNIQUE 数据库）、WAL publisher
> 和物理资格仍 **NOT_RUN**；完整目标与 M10 未验收。详见[新检查点证据](evidence/storage-unique-checkpoint-20261012.md)。

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
   interpretations are checked. The new source-only CHECK composition additionally evaluates each admitted scalar declaration once per primary row (not per index); NULL passes. Ordinary SQLite affinity remains dynamic typing.
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
simple column types/defaults/NOT NULL/INTEGER PRIMARY KEY, and ascending BINARY
ordinary indexes, including the bounded explicit named-column UNIQUE profile
described below. Admitted ordinary column/table automatic UNIQUE and the narrow
function-free scalar CHECK profile are also represented in the current source
composition; its runtime qualification is separately pending. Supported owned extensions are grams13 and the current
native dense/typed HNSW and IVFFlat configurations.

Unsupported features fail instead of being reported as checked: WITHOUT ROWID,
STRICT, autovacuum, UTF-16, schema formats 1–3, UNIQUE constraints/autoindexes
outside the admitted profile, typed-vector/extension UNIQUE, other primary
keys, CHECK expressions outside the admitted function-free scalar profile,
foreign-key constraints, partial/expression/DESC/non-BINARY
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


## Source-only explicit UNIQUE profile increment (2026-10-11)

Historical author-stage record, retained verbatim below. Its NOT_RUN statements
apply to 2026-10-11 source preparation; current finite and combined evidence is
separately recorded at the top of this document.

The [explicit UNIQUE candidate](unique-index.md) extends catalog admission to
ordinary named-column BINARY ASC explicit unique indexes. For each admitted
unique index, Check also compares adjacent indexed prefixes excluding rowids,
using exact mixed INTEGER/REAL comparison and NULL exemption. Equal non-NULL
prefixes are ErrCorrupt even if every complete key has a matching source row;
SQL write conflicts are separately ErrConstraint / primary CodeConstraint (19).
The scan retains one previous prefix under the existing check budgets and does
not retain a whole-index set. CheckScope keeps its existing supported-profile-v1
identifier; it is not a versioned claim that all SQLite UNIQUE forms are checked.

This candidate is NOT_RUN. The earlier fail-closed paragraph describes the
published baseline; candidate admission removes only explicit ordinary UNIQUE
from that rejection list. Table/column UNIQUE and implicit autoindexes, typed
vector or extension UNIQUE, DESC, partial, expression and non-BINARY indexes
remain rejected. Descriptor-free logical fixtures are confined to PageReader
memory; no malformed fixture is written to native storage or routed to VFS.

## 自动 UNIQUE 目录/Check 增量（2026-10-12，未运行）

[自动 UNIQUE 新候选](automatic-unique.md)在完整目录期望集合匹配后，将每个约束根
加入既有 Check 所有权、source/index 双向核对及相邻非 NULL 前缀唯一性检查。
不会从 sqlite_ 前缀推断已支持，也不合成缺失索引。NULL 仍有真实索引项；任意字段
NULL 的重复键合法。空 TEXT 自动索引 SQL 是明确不支持的非规范形式，不是损坏断言。
原来的全面拒绝 autoindex 段落是历史范围；这个新范围尚未运行验证，CheckScope 名称
不变且不宣称完整 SQLite integrity_check 等价。生产 Check 实现本身未修改。
