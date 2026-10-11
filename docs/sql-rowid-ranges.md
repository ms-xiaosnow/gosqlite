# Native SQL rowid intervals and proven rowid ordering

This is a deliberately narrow planner increment. Single-table SELECT can use a
native rowid B-tree interval for direct rowid/IPK `<`, `<=`, `>` and `>=`
constraints on either side of an AND. Constants are literals, parameters, or
unary signed literals/parameters. Physical shadowing and direct physical-column
result aliases follow the existing binder. OR, general BETWEEN extraction,
expression bounds, secondary-index inequalities, covering access, and JOIN range
pushdown are not introduced. Existing equality, grams and ANN access keep their
precedence. For this range path, NOT INDEXED still forces the old complete
table source and sorter. The separate bounded direct IPK equality exception in
[ordinary access](ordinary-index.md) can seek under NOT INDEXED; it does not
expand this range path.

## 2026-10-11：NOT INDEXED 的有限 IPK 等值例外

本次是组合测试合同纠正，未改变本页区间规划的生产源码。原 1,800 行
fixture 的 `SELECT id FROM t WHERE k>88 AND id=1`，以及在表后插入
`NOT INDEXED` 的精确对应 SQL，都允许直接 IPK seek；id=1 的 k=1，
完整 residual 复核后应为空。匹配对照 id=899 的 k=89，应返回 899。
官方 SQLite 3.54.0 的新固定根实际执行原 SQL、NOT INDEXED、`id+0` 真扫描
两组结果及两次 EXPLAIN，加一次身份查询，共九个正常完成的官方进程；
一根八子项 9 named PASS。不能用原 24-case IPK oracle 冒充这条原 SQL 的证据。

native 普通与精确 NOT INDEXED 形式分别断言 SEARCH/rowid、一次 seek/fetch、
无 secondary work 和原 WHERE 复核；`id+0` 形式独立断言 SCAN、1,800 fetch、
零 seek/index work。值不变的说明仅限该已知小整数 fixture，未修改通用
`ordinaryCompare` 的 genuine-scan 合同，未扩大 NOT INDEXED 范围路径。
原四根 FAIL、四个测试文件 delta 与新树阶段结果见
[组合证据](evidence/readonly-ipk-combined-20261011.md)。完整 26 项已于 22:05:15 UTC 自然 PASS。

## Exact bounds

Comparison NUMERIC affinity is applied before constructing integer-space cuts.
INTEGER bounds never round through REAL. Fractional REAL lower/upper cuts use
ceil/floor in the appropriate direction; values outside the signed int64 domain
are resolved without overflowing a conversion or incrementing an extreme.
NULL inequalities produce an empty interval. Remaining TEXT/BLOB after NUMERIC
affinity sort after every numeric rowid, so `<`/`<=` are unbounded and `>`/`>=`
are empty. The full original WHERE program still evaluates every candidate.
Intersected cuts preserve exclusive equality and contradictory interval cases.

Only the pre-existing total/error-safe scalar WHERE grammar and safe missing
column defaults are admitted. Error-capable functions, LIKE, typed-vector reads,
and unfamiliar residual nodes keep the prior source. This is conservative rather
than a complete predicate proof system.

## Real forward/reverse ORDER BY

Exactly one ORDER BY term, proven to resolve to the unique non-NULL physical
rowid, can eliminate retained sorting. ASC uses forward traversal and DESC uses
reverse traversal; positional/result aliases are resolved before proof. Explicit
NULLS FIRST/LAST have no effect on the non-NULL rowid. There are no rowid ties.
A shadowing physical column, an alias of another expression, multiple terms,
expressions such as `id+0`, or vector ranking retain the sorter. Unordered range
results stay in ascending rowid order.

Sort elimination also requires total/error-safe projections. It therefore cannot
hide a late projection error behind LIMIT/OFFSET compared with the former eager
native sorter. The source is opened on the first Next, and LIMIT/OFFSET can then
stop actual traversal. LIMIT 0 and EXPLAIN execute no range cursor work; ordinary
schema/planning reads still occur, and bind/name/LIMIT validation is unchanged.

The conservative fallback is not complete SQLite error-order parity. For example,
with negative rowids including MinInt64, SQLite 3.54.0 can use rowid order to return
`SELECT abs(id) FROM t WHERE id<0 ORDER BY id DESC LIMIT 1` without ever visiting
the overflowing minimum. The existing native eager sorter reports integer
overflow. This slice refuses to elide that error-capable projection, preserving
the established native error behavior. The first new oracle run exposed this
pre-existing non-admitted-shape discrepancy; the failing transcript is retained.
No full SQLite compatibility claim follows from the selected differential pass.

## Ownership, quotas and diagnostics

The new source borrows the same immutable reader snapshot as Rows/Tx; it owns no
connection, transaction, goroutine, external process, or file descriptor. Existing
Rows close/cancel/lease and transaction BUSY rules apply. JOIN/LEFT JOIN retain
their existing shared budget/operator paths and never enter this planner.
Native cursor limits are cumulative: default 1,048,576 pages and record admissions,
16,777,216 work units, and 67,108,864 encoded payload bytes. EOF, quota failures,
and cancellation stay distinct/sticky. There is no retry or partial fallback once
streaming starts; callers must check Rows.Err. MaxSortBytes still caps retained
sort rows; eliminated sorts do not allocate that buffer. This does not introduce
a new connection-wide memory cap or permit plaintext spill.

No existing public struct layout changes. Plan.Method is `rowid_range`, version 1.
Access is SEARCH only for a physical bound or statically empty interval; a full
unbounded traversal is SCAN, including tautological TEXT/BLOB inequalities.
Reason reports direction, physical boundedness, order_satisfied, actual sort use,
and covering=false. Unknown cardinality is -1; proved-empty is 0. Plan snapshots
retain the existing defensive slice copies and do not change during iteration.

Stats.Candidates counts range rows yielded so far before WHERE recheck, rather
than an eagerly retained whole candidate set. BaseRowFetches and ReturnedRows
keep their meanings. RowIDSeeks counts one initial positioning attempt (bounded
seek or table-edge descent), and zero for statically empty intervals. Table-page
reads and records are not mislabeled IndexPageReads or IndexKeysRead; those stay
zero. Exact range page calls are independently instrumented in the internal test,
not exposed as fabricated index I/O. Existing equality-prefix accounting is
unchanged.

## Scoped evidence and remaining work

The selected SQL/storage-class matrix exercises 1,104 bound/side/direction cases
against genuine NOT INDEXED native scans, plus intersected/empty cuts, alias and
rowid-shadowing proof, lazy plans, limits and immutability. Exact rational laws
independently check int64/REAL boundary conversion. Ordinary tests cover rollback,
read-own-writes, active-Rows BUSY, shared reader snapshots across another writer's
commit, cancellation, checkpoint, close/reopen, and database/sql named prepared
binds and streaming.

A descending LIMIT 3 OFFSET 2 interval fetches 5 base rows versus 1,800 for the
forced scan. An independently counted immutable in-memory copy of the intact
512-byte fixture reads 3 native range pages for 3 rows, in either direction,
versus 828 page calls for 1,028 source rows in the complete scan. These are logical
work observations, not latency or production benchmarks.

The tagged oracle compares exact values, INTEGER/REAL/TEXT/BLOB/NULL types,
ordered results, real bound parameters, and selected ordinary error categories
against pinned official SQLite 3.54.0 at 512/4096/65536-byte pages. See the full
commands/results and preserved initial failure in docs/evidence/sql-rowid-range.
The runner executes a source-reference guard before every selected unit, race,
oracle, build and vet command. The production closure never invokes SQLite CLI.

Independent review is required before integration. Secondary BINARY ASC leading
index intervals, composite order proof, mixed directions/collations, covering,
statistics, general planner completeness, full SQLite error-order parity, broad
M3/M4/M5/M10 acceptance and production qualification remain open. No physical
fault/tamper/wrong-key/crash/forced-exit/hot-recovery or interrupted publication
experiment was executed or claimed.
