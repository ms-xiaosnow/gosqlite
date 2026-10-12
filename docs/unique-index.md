# Explicit ordinary UNIQUE indexes: bounded development profile

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

Historical author-stage status (2026-10-11, before finite validation): IN_PROGRESS / SOURCE_ONLY / NOT_RUN. This independent
candidate starts from published commit f189746671aaf4962a699292a84110e2d43387e8,
tree 0f29261ca50576373069dd8ad23a2bd3377a743c (3,180 files). That baseline and all
historical evidence remain unchanged. None of its PASS results qualify this new
source. No Go, gofmt, parser/compiler, test, race, official oracle, project script,
Git, remote or Library operation was executed to implement this slice.

## Requirement and supported source profile

Goal G1 / sections 4.2 and 11 require staged constraints, SQLite write/error
semantics and supported-domain file interoperability. The goal does not literally
name CREATE UNIQUE INDEX; this is the next bounded part of the documented UNIQUE
constraint gap, not a claim that all SQL features must land together.

The candidate accepts:

```sql
CREATE UNIQUE INDEX [IF NOT EXISTS] index_name
ON table_name(column_name [COLLATE BINARY] [ASC], ...);
```

The table is an already-supported ordinary rowid table. Indexed fields must be
ordinary scalar declarations or an INTEGER PRIMARY KEY alias; typed vector
declarations elsewhere on the table do not make an independent scalar index
unsupported. The existing named-column, identifier, token and SQL-size bounds
remain. The original explicit declaration is stored in sqlite_schema and is
reparsed on each writable-catalog load. No on-disk index format changes: complete
keys remain `(indexed values..., signed rowid)`.

Table/column UNIQUE constraints and implicit autoindexes remain unsupported.
So do typed-vector unique keys, UNIQUE USING grams13/HNSW/IVFFlat/custom methods,
expression, partial, DESC and non-BINARY indexes, conflict policies and UPSERT.
The existing parser still represents DESC for index ASTs; write/catalog admission
rejects it. Noninteger/compound primary keys, CHECK and foreign keys remain
separate work. IF NOT EXISTS on an existing nonunique index is a no-op and never
converts it to UNIQUE.

## Enforcement and statement semantics

- Each non-NULL key is checked with the existing bounded index cursor using
  equal inclusive endpoints containing all indexed fields and no suffix rowid.
  This is a real prefix seek, with BINARY storage-class ordering and exact
  INTEGER/REAL comparison. It does not stringify composites, round int64 keys
  through float64, retain a whole-index map, or scan all preceding keys.
- Any NULL indexed field exempts that key, including repeated composite keys.
  Affinity and existing NOT NULL processing precede the check, and the logical
  INTEGER PRIMARY KEY value comes from the source rowid.
- INSERT preserves the preexisting rowid-before-index conflict ordering with a
  rowid lookup only when unique indexes are present. The unique probe happens
  before source/index mutation and last_insert_rowid assignment. A first-row
  unique conflict leaves last_insert_rowid unchanged; an aborted later row leaves
  the last earlier successful insertion's rowid in connection state. Failed
  statements set changes() to zero and contribute no total_changes().
- UPDATE checks before deleting old index keys or changing source rows. It
  excludes exactly the old signed rowid, using an explicit exclusion boolean.
  Zero and negative rowids are not sentinels. An unchanged key with a changed
  rowid is allowed; a different row's occupied key fails immediately. A proposed
  final-state swap does not defer conflicts until the end of a statement.
- CREATE builds with the same probe before each index insertion. The existing
  storage Statement boundary owns undo of allocation, freelist, schema, rows and
  index/extension participation. Failed builds never publish their declaration.
  Savepoint and full transaction undo use the existing protocol. No pager, VFS,
  journal, recovery, backup/publisher or ANN algorithm code changes are included.
- Probes run after writer promotion and keep the same writer transaction.
  Cancellation, quota and corrupt-page errors propagate; failure to complete a
  probe cannot authorize a write. Every created cursor is closed. Default finite
  per-probe RangeLimits are 1,048,576 page reads, 16 Mi work units, 1,048,576 record
  decodes and 64 MiB decoded payload. Existing record/depth/transaction quotas
  remain; these are not an aggregate heap or whole-statement work guarantee.

The public write-conflict contract is errors.Is(ErrConstraint) and primary
CodeConstraint (19). Exact diagnostics or extended SQLite error codes, including
which diagnostic wins for simultaneous constraint violations, are not promised.

Preparation caching remains names-only. No root, index descriptor or record is
retained as authority to bypass fresh catalog validation. Ordinary SELECT
equality and leading-range selectors explicitly skip Unique, so parser admission
does not silently add a new acceleration profile. The extension dispatcher also
rejects Unique before method dispatch, independently of parser rejection.

## Native Check

Check preserves source/index bijection and additionally rejects adjacent equal
non-NULL indexed prefixes in admitted unique indexes. It retains one previous
prefix, never a whole-index set. Rowids are excluded from equality. A logically
inconsistent unique index returns ErrCorrupt and no CheckResult; it must not pass
merely because each physical full key still has a source row. NULL exemption,
storage classes and exact mixed numeric comparison match write enforcement.

CheckScope remains gosqlite-supported-profile-v1. The accepted subset is listed
here and in native-check.md; it does not imply general integrity_check parity.

## Historical authored finite validation (2026-10-11; all NOT_RUN at that stage)

These source files define bounded tests. They have not been formatted with Go,
compiled, run or compared to an oracle:

1. internal/sql/unique_index_parse_test.go: four parser roots for ordinary and
   quoted/IF NOT EXISTS/composite/BINARY/ASC acceptance, legacy nonunique forms,
   unsupported constraints and syntax, and the fixed column-count boundary.
2. sqlite/unique_index_internal_test.go: descriptor-free PageReader roots for
   key classes/numeric edges/NULL, exact signed self exclusion, real bounded seek
   versus full traversal, quotas/cancellation, logical Check duplicates and
   split trees, catalog reload, typed-vector/extension admission and planner
   exclusion. Fixtures are memory-only; no malformed bytes go to disk or VFS.
3. sqlite/unique_index_test.go: ten healthy native SQL roots for build rollback
   (including allocations/freelist/catalog), IF NOT EXISTS, composite NULLs,
   affinity/typeof and exact int64/REAL distinctions, signed rowids, immediate
   updates and counters, normal transaction/savepoint/DDL undo, grams statement
   undo, long NUL/Unicode/BLOB keys, page splits/overflow, 512/4096/65536-byte
   rollback and shared-WAL files, normal checkpoint/close/reopen and a healthy
   two-connection BUSY-to-constraint transition. One 4096-byte exclusive-WAL
   smoke root is separate.
4. driver/unique_index_test.go: three roots for prepared bound writes, sql.Tx
   error propagation/earlier-statement preservation, schema rollback, reopen,
   counters and single-connection pool reuse. No driver production edit.
5. internal/testkit/unique_index_conformance_test.go (oracle tag): bounded same-
   connection histories continuing through expected logical constraint errors,
   counters and state comparisons, and healthy two-way explicit-index files
   with official integrity_check and forced-index reads. Ordinary CREATE UNIQUE
   INDEX is a positive native/official contrast to its prior unsupported status.
   Existing testoracle.ForTest supplies pinned startup isolation; no production
   path gains a subprocess or delegation dependency.

Two previous rejection cases, in internal/sql/sql_test.go and
sqlite/write_test.go, change only `CREATE UNIQUE INDEX i ON t(x)` to
`CREATE UNIQUE INDEX i ON t(x) WHERE x>0`. The ordinary form is now the requested
source feature; partial indexes remain unsupported. This is not a rewritten
historical failure or a weakened assertion. New positive sources cover it.

The finite proposal is: inspect the exact source and test selectors first; then
separately approve read-only formatting inspection, scoped CGO0 build/vet,
four normal package selections (parser, memory models, native SQL, driver), the
same memory/native/driver roots under supported CGO1 race tooling, and three pinned
official-oracle roots. No broad ./... invocation or fuzz campaign is implied.
Exact argv, selectors, binary identities and source hashes belong to the external
frozen source/validation report. A formatting edit or review correction requires
a new source identity before execution. Each future command must record its own
terminal outcome and preserve any failure; a proposal supplies no PASS.

## Historical author-stage completion boundary (2026-10-11)

Independent exact-source review and separately approved ordinary/model/driver/
race/oracle results are still needed. Storage-candidate integration is separate;
this work does not import unpublished storage changes or inherit their evidence.
Physical fault/crash/recovery/tamper/namespace work, held ANN V4, the 41-target
fuzz campaign and unknown interrupted chains are unchanged and untouched.
This slice does not establish M3/M8/M10, production durability or security.
