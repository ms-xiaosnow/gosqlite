# Native SQL leading secondary-index intervals

This is one bounded planner increment on reviewed baseline tree
`284402d1d8acba2252c85cc3d52c353cc4ed920b`. It adds genuine native leading-key
inequalities without changing the reviewed B-tree cursors, equality access,
rowid-range implementation, file formats, or public Plan/Stats layouts.
Independent review and the lead's full integrated gates are separate from the
component checks here. This is partial SQL planner capability, not general
SQLite optimizer parity or production qualification.

## Admission and exact boundaries

A single-table SELECT whose source remains an untouched scan can extract direct
physical-column `<`, `<=`, `>` and `>=` comparisons from AND conjuncts, in either
operand position. Literal, parameter and unary signed literal/parameter bounds
are admitted. A direct physical-column result alias follows existing binder
precedence; a real physical column shadows a result alias. Multiple constraints
on that first key are intersected without changing conjunction evaluation.

The physical index must be a supported nonunique BINARY ASC index. Every suffix
field must also be BINARY ASC and non-vector. Unsupported DESC, mixed-direction,
expression, unique/autoindex and collation definitions are ignored individually
for read-only planning. At most 256 candidate definitions are considered. With
multiple eligible indexes, the name's existing ASCII fold gives deterministic
selection; no cardinality or cost-statistics claim is made.

Constants have no expression affinity in the admitted grammar. The physical
column's comparison affinity is applied to the value, using NUMERIC for INTEGER
and REAL rather than forcing an integer bound through REAL storage conversion.
Declared BLOB and absent declaration retain their storage values; neither is
mistaken for TEXT coercion. INTEGER/REAL thresholds, signed extrema, TEXT,
embedded NUL and BLOB use the reviewed full-field storage comparator. Bounds on
one field include/exclude its entire suffix group, including duplicate keys and
signed trailing rowids. No sentinel padding or prefix-string approximation is
used. NULL inequalities are statically empty. An upper-only non-NULL inequality
also uses an exclusive physical NULL lower endpoint so SQL NULLs are excluded.

Only the existing total/error-safe WHERE/default proof is admitted. Functions,
LIKE, typed-vector reads and unfamiliar residual forms keep their previous
source. Every candidate evaluates the original WHERE; comparisons between two
columns are residual only and do not lose declared BLOB versus expression-affinity
distinctions. Binding, name, startup predicate, ORDER BY and LIMIT validation
still precede this planner, including LIMIT 0 and EXPLAIN.

## Ordering, precedence and lifetime

The planner runs after existing equality, rowid, grams and ANN selection. Even
an unbounded rowid traversal selected for a proven rowid ORDER BY retains
precedence. For example, `WHERE k>1 ORDER BY id` can keep `rowid_range` SCAN;
`WHERE k>1 ORDER BY k,id` can select `btree_range` SEARCH. NOT INDEXED always
retains the genuine complete table source. Vector ranking and JOIN/LEFT JOIN
are outside this new path.

All matching index rowids are collected within finite limits, then sorted in
signed rowid order before base-row seeks. This preserves native unordered
LIMIT/OFFSET and stable sorter ties. The implementation never claims secondary
index order satisfaction, reverse-index SQL traversal or covering access. ORDER
BY still uses the existing honest sorter, including mixed directions and NULLS
FIRST/LAST. SQL without ORDER BY has no cross-engine order guarantee, so unordered
LIMIT evidence compares native indexed access with the native forced scan;
official ordered comparisons specify a complete rowid tie-breaker.

Candidate collection is eager for ordinary execution, so quota fallback occurs
before any source row is emitted. EXPLAIN and LIMIT 0 open no range cursor. An
unsafe residual leaves the preceding planner reason unchanged. A budget rejection
appends its reason rather than erasing earlier diagnostics. An
unsuccessful budgeted attempt leaves the original source untouched, discards all
partial candidates and retains attempted index counters. Cancellation and real
errors propagate. The source borrows the existing Rows/Tx reader snapshot and
reuses the reviewed ordinary rowid-seek source; no connection, transaction,
file, goroutine or external process ownership is added. The normal Rows lease,
BUSY, snapshot, Close and cancellation contracts remain.

## Admission and diagnostics

Endpoint workspace is estimated before cursor construction. Half MaxSortBytes
is reserved as the new candidate workspace, including a conservative endpoint
allowance and 64 bytes per retained candidate; the other half is available for
bounded cursor admissions. At most 100,000 candidate rowids are retained. Actual
candidate slice capacity is admitted before allocation and never grows above
that count. At most 8,192 actual index page calls are attempted, further bounded
by the cursor workspace divided by `4*page_size+512`.

Before decoding a record, its payload must fit a conservative `128*n+512`
workspace bound; before page-parser allocation, the cursor's conservative exact
cell/span admission must fit the workspace. Tighter upstream BeforeRecord and
BeforePage hooks are forwarded and cancellation is rechecked after them.
The unchanged cursor separately caps cumulative work (16,777,216 units), records
(1,048,576 admissions) and payload bytes (67,108,864), including probes/redecodes.
Endpoint copies are admitted by that cursor before either copy. This is bounded
workspace/work policy, not a connection-wide heap/RSS or exact allocator cap.
Existing sorter quotas are unchanged, and no plaintext spill is introduced.

Plan.Method is `btree_range`, version 1, Access SEARCH, with the real index and
leading physical field. Recheck=true. EstimatedCandidates=-1 except a proved
empty interval's zero. Reason explicitly reports forward traversal, rowid-ordered
candidates, order_satisfied=false, actual retained-sort use, and covering=false.
Plans retain their existing defensive slice ownership and stay fixed during
iteration. LIMIT 0/EXPLAIN plans describe eligible execution, with no candidate
collection work.

Stats.IndexKeysRead counts returned matching index records, not binary-search
probes. IndexPageReads counts actual range PageReader calls, including descent
and overflow calls. Catalog/planning reads, table seeks, and OS physical I/O are
excluded. Candidates is the complete admitted rowid set; RowIDSeeks and
BaseRowFetches count actual source attempts/rows. Abandoned range work remains
visible on SCAN fallback. LIMIT can reduce base-row seeks but this slice still
collects the full bounded candidate interval first.

## Scoped evidence and remaining work

The ordinary matrix compares 1,120 affinity/storage-class/operator/operand-side
cases with genuine NOT INDEXED native scans. It includes duplicate keys and
oppositely ordered suffixes, int64/REAL boundaries, NULL, text/NUL, BLOB, aliases,
intersected/contradictory intervals, tie orders, bounds in either AND order,
prepared-statement reuse with named parameters, malformed/name/bind errors,
ordinary rollback/shared WAL snapshots, quotas, cancellation and concurrent
queries. The official SQLite 3.54.0 matrix performs 1,062 ordered exact
values/types/parameter/error comparisons at each of 512/4096/65536-byte pages.
NUL-containing TEXT is compared losslessly as hex plus original typeof because
the official CLI quote mode truncates raw NUL text.

One 1,800-row native fixture admits 40 matching index keys using 4 logical index
page calls and 40 base-row fetches, versus 1,800 base-row fetches in its complete
forced scan. With LIMIT 7 OFFSET 3 and no ORDER BY it still collects those 40
keys, then fetches 10 base rows versus 450 native scan rows. Separate independent
PageReader instrumentation reads 3 logical index pages for 4 keys in a late
interval, versus 82 pages for a full 1,600-key traversal of that same index.
These observations exclude schema/planning and base-seek page calls from the
index counters; they are scoped work evidence, never latency or universal
performance claims.

The first tagged run's core matrix passed, but its extra shared-WAL lifecycle
fixture attempted an unsupported native read-only WAL reopen and failed. The
original output is retained in `oracle-initial.txt`. The fixture was corrected
by an ordinary official SQL transition to rollback mode after checkpoint/close,
then reopened read-only for unsupported-peer-index checks. No production code
was changed for that unsupported API use. A later selected regression gate also
retained one obsolete assertion requiring SCAN for `k>10`; its failing transcript
is preserved. Only that exact newly implemented shape now requires a real
`btree_range` SEARCH with one index key and one base seek, in addition to the
unchanged forced-scan value check. Every other unsupported-shape assertion stays.
Final commands/results and the source
inventory live in `docs/evidence/sql-secondary-range`.

Reference semantics were checked against pinned public SQLite source:
`src/expr.c:sqlite3CompareAffinity`, `src/wherecode.c` range affinity, seek/end
opcodes and NULL exclusion, and the already reviewed B-tree numeric comparator.
Production remains pure Go and invokes no SQLite CLI or external SQL engine.

Still required code: equality-prefix plus next-key inequalities, richer compound
range extraction (OR/BETWEEN/IN/expressions), secondary reverse traversal and
fully proven key/suffix/rowid/NULL ordering, covering reads, JOIN probes,
statistics/cost selection, and broader planner completeness. Full SQLite error
ordering, M3/M4/M5/M10 acceptance and production qualification remain open.
No physical fault/tamper/wrong-key/crash/kill/forced-exit/hot-recovery, interrupted
publication or rekey experiment is run or claimed by this component.
