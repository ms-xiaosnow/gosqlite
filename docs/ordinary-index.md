# Ordinary B-tree SELECT access

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

The first ordinary-index planner uses the existing native SQLite secondary
B-trees; it does not build a transient replacement index. It supports direct
rowid/INTEGER PRIMARY KEY equality and leading equality prefixes of supported
nonunique BINARY ASC indexes. All matching candidates still evaluate the original
WHERE. This is an incremental SQL capability, not general SQLite planner parity.

## Selection and semantics

Eligible ANN retains first precedence, then grams13. Ordinary access is considered
only when the real source is still an untouched SCAN and no ANN candidate query
was retained. NOT INDEXED keeps the complete scan except for the bounded direct
IPK equality admission described below. Unsupported index definitions
are ignored individually for read-only planning; they do not make an otherwise
supported source table unreadable. Existing writable-catalog safety rules remain.

Column identity is resolved against the physical table and supported direct
column aliases. Equality values receive the same comparison affinity as the
original predicate. NULL/storage-class and mixed INTEGER/REAL comparisons are
preserved, including signed int64 limits and values around2^53. A rowid equality
known impossible after comparison affinity returns no candidate instead of
inventing a nearest rowid.

A secondary prefix range collects bounded matching rowids, then sorts signed
rowids before base seeks. This preserves the existing native scan's unordered
LIMIT/OFFSET selection and stable ORDER BY tie behavior even when index suffixes
have another order. There is no covering-read, reverse-range, inequality endpoint
or ORDER BY elimination claim. Non-leading equality alone does not select that
index. Every returned secondary key must identify a real base row.

The general ordinary-index path retains its total-residual and safe-default
admission gate; error-capable function calls, LIKE and typed-vector interpretation
do not gain secondary-index or range admission. Predicate branch/startup behavior
is described in [Boolean execution](boolean-expressions.md). Projection still
occurs for matching rows, and existing grams/ANN precedence is preserved.

## Current unpublished direct IPK equality increment

The RO/IPK/live-writer composition adds one narrow exception before that general
gate: a declared INTEGER PRIMARY KEY table, exactly one direct top-level AND
equality (`=` or `==`) to a supported literal/parameter/unary constant, safe
defaults, and the bounded ordinary/LIKE residual envelope may install a lazy
rowid source. Eligible physical rowid aliases follow the existing shadow rules.
This table-positioning seek is also available under NOT INDEXED. NULL or an
impossible fractional key produces no candidate; a missing integer key performs
one seek and no row fetch. Matching rows recheck the original WHERE in its original
order. Row-independent startup expressions still run before row access; LIMIT 0
and EXPLAIN retain their no-execution behavior. QueryStats.Candidates is the eager
candidate-set size, so it can be one while RowIDSeeks remains zero.

CAST, general function calls, IS or extra direct rowid comparisons, tables without
a declared IPK, typed-vector columns and relational/vector plans stay outside
this exception. It does not relax secondary/range admission or implement join
pushdown. Existing snapshot-reader ownership and row-source retirement are reused.

The independent IPK V4 input tree `96e039f2a0b0990789579bfa5fe20746b6b1ecdb`
has six normal roots/49 children, 55 named PASS, after the narrow recorder-counter
correction described in [current progress](progress.md). Original negative-control
failures and STOP_HOLD remain preserved. Its later original-command-3 oracle has
24 SQL comparisons plus one identity query, 25 named PASS and natural release;
这些结果仍只属于原输入。当前组合冻结树
`cace45621c10259231a2e764cfbefdcdbe312ccf` 已有完整普通门及固定九进程
原 SQL oracle PASS；完整 26 项已于 22:05:15 UTC 自然 PASS，见[当前证据](evidence/readonly-ipk-combined-20261011.md)。
原 1,800 行 fixture 的 `SELECT id FROM t WHERE k>88 AND id=1` 及其
精确 NOT INDEXED 形式均为空；id=899 对照均返回 899。NOT INDEXED 不禁止
这类 IPK table-positioning seek。另以 `id+0` 强制完整 SCAN，保留 1,800 行
实际 fetch 断言；此表达式对照只适用于已知的小整数 fixture，不是一般 affinity
重写定律。新旧 oracle 的 SQL、进程数与源码身份分开，未修改通用 genuine-scan helper。

## Bounds and diagnostics

- At most256 candidate index definitions are considered.
- At most100000 candidate rowids, further bounded by MaxSortBytes/64 and prefix
  memory, are retained.
- At most8192 actual range-page reads, further bounded by available memory divided
  by4*page_size+512, are attempted.
- Prefix/key sizes and underlying B-tree payload/depth/visited limits still apply.
- Candidate/work admission exhaustion keeps the original untouched table source;
  no partial candidate list is returned. Cancellation remains an error.

Plan.Method is rowid or btree and Access is SEARCH only for those executable
paths. B-tree plans identify the selected index and physical equality prefix,
recheck=true, covering=false and order_satisfied=false. With no cardinality
statistics, EstimatedCandidates=-1 is explicit rather than a fabricated estimate.
EXPLAIN does not execute the candidate range.

Stats.IndexKeysRead counts matching index records returned by the range;
IndexPageReads counts actual range PageReader calls, including descent probes;
RowIDSeeks counts base seek attempts. Catalog reads and other access methods are
excluded from these counters. They are logical calls, not OS physical I/O, and
attempted index work is retained after fallback. BaseRowFetches and ReturnedRows
keep their existing meanings.

## Ordinary evidence

Independent valid-file oracle tests cover512/4096/65536 pages in rollback/shared
WAL, native versus NOT INDEXED and official SQLite results, transactional updates
and rollback, external legal SQL changes, normal checkpoint/reopen, unsupported
expression/DESC peer indexes, numeric/text/BLOB/NULL comparisons and NUL/overflow
TEXT prefixes. Unit2.597s and race4.523s passed in the expanded oracle component
run; final-source regressions and aggregate are recorded separately.

A1800-row native fixture reads20 matching index keys,3 range pages and20 source
rows versus1800 forced-scan source rows. Independent PageReader instrumentation
also measures a late two-key prefix reading3 pages versus84 for a complete
1600-key index traversal. These are scoped logical-work observations, not latency
or universal optimization claims. No physical crash/fault/recovery gate is covered.

## Later rowid interval increment

The separate [SQL rowid interval guide](sql-rowid-ranges.md) adds native bounded
rowid traversal and narrowly proven forward/reverse ORDER BY elimination. The
secondary equality-prefix path described above still sorts rowid candidates and
does not claim order satisfaction or secondary-index inequality support.


## Source-only explicit UNIQUE constraint increment (2026-10-11)

Historical author-stage record, retained verbatim below. Its NOT_RUN statements
apply to 2026-10-11 source preparation; current finite and combined evidence is
separately recorded at the top of this document.

The [explicit UNIQUE candidate](unique-index.md) adds constraint enforcement with
an equal-inclusive complete-field prefix seek using `NewBoundedIndexCursor`.
It does not add UNIQUE indexes to either ordinary SELECT planner admission set:
both equality-prefix and leading-inequality selectors explicitly skip Unique.
Reads continue to use the prior eligible nonunique index or source scan. Native
rowid/IPK positioning, NOT INDEXED, grams/ANN precedence and diagnostics are
unchanged. The new source/model/healthy SQL sources are all NOT_RUN; the ordinary
historical evidence above is not evidence for this constraint increment.
