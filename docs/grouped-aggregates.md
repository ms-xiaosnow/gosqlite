# Bounded native GROUP BY and HAVING

Status: **PARTIAL**, behavior baseline official SQLite **3.54.0**. This extends
the reviewed global-aggregate slice; it does not complete M4, broad SQL,
durability, encrypted recovery, or production qualification. Execution is native
Go. Disk formats and existing exported sqlite/driver struct definitions stay
unchanged. Internal Select gains GroupBy/Having fields.

## Supported profile

- Existing named-table, no-FROM, comma, INNER/CROSS/LEFT input graphs, scalar
  WHERE/ON and COUNT/SUM/TOTAL/AVG/MIN/MAX now feed bounded grouped accumulation.
- GROUP BY accepts supported scalar expression keys, source columns, result
  aliases and integer result ordinals. Physical names take precedence over
  aliases in grouping and HAVING. ORDER BY retains alias/ordinal precedence and
  accepts expressions composed of supported grouping expressions/aggregates.
- SELECT/HAVING/ORDER BY may use complete grouping expressions, scalar wrappers
  around those expressions, aggregates, or constants. Bare source expressions
  not derivable through an explicit grouping expression return ErrUnsupported.
  For example SELECT x,count(*) GROUP BY y remains unsupported even if a caller
  knows x is functionally determined by y. Wildcards in grouped/HAVING queries
  are explicitly unsupported. No ambiguous bare-column MIN/MAX extension is
  claimed outside grouping-key expressions.
- HAVING without GROUP BY requires an aggregate in the result list. The empty
  global group still exists; grouped empty input produces no groups.
- DISTINCT, windows, aggregate FILTER/local ORDER BY, ROLLUP/CUBE/grouping sets,
  collations other than existing BINARY behavior, CTEs, subqueries and compound
  SELECT remain unsupported. Existing join/schema/forward-ON boundaries remain.
- Integer/REAL key equality is numeric and exact at large int64 boundaries.
  INTEGER/REAL zero coalesce. NULL keys coalesce; TEXT and BLOB remain different
  classes, compared by full BINARY bytes, including embedded NUL. GROUP BY does
  not apply comparison affinity to coerce different stored classes. Declared
  input affinity and existing typed-column lazy validation still apply.

Key identity is structural after binding; this is not an algebraic theorem
prover. Supported comparisons do not imply every algebraically equivalent SQL
spelling is accepted. Combined private key/representative/aggregate output slots
are capped at 2,000, independently of the inherited input-slot cap.

## SQLite distinctions preserved

Pinned select.c and func.c distinguish a saved computed grouping key from the
source row supplying grouped column references. For example an equal INTEGER
and REAL key can expose different typeof values when MIN/MAX selects a later
representative. The implementation retains the needed source cells for one
representative per group, in addition to canonical keys. It preserves the first
representative without MIN/MAX; the last distinct MIN/MAX accumulator controls
replacement, including all-NULL and equal-value behavior. Calls are deduplicated
and discovered in SELECT, ORDER BY, then HAVING source order. Complete computed
keys selected at the result root use their saved key; nested scalar expressions
use the representative. Exact native/oracle tests cover these distinctions.

Aggregate-dependent HAVING runs after the relevant group finalizers. As in
pinned SQLite, BINARY key-only/constant top-level conjuncts are first promoted
to input WHERE, except syntactically false literal terms. This is observable:
HAVING typeof(k)='integer' may remove REAL-valued input rows before counting a
numeric-equal group. It cannot be replaced with unconditional post-group
filtering without changing SQLite results. The usual bound predicate compiler
handles the promoted terms; bare columns still cannot bypass the profile.

Group states preserve source-row order within each group. Key order is native
storage-class/numeric/BINARY lexicographic order. Matching GROUP/ORDER lengths
copy order directions, following the pinned source. Identical full key order
with natural NULL placement needs no second sorter. Other ordering uses a bounded stable result sorter. With a finite positive
LIMIT it retains at most LIMIT+OFFSET candidates (saturating on overflow),
evaluates HAVING and ORDER keys first, and skips remaining scalar projection
for a worse later candidate. Equal keys retain the earlier group. ORDER aliases
and ordinals evaluate their required projection values before admission.
Unbounded ordering keeps the existing full sorter. This correction is confined
to grouped queries; it does not claim to repair older general-sort timing. SQL makes no general promise about unordered
rows or unspecified ORDER BY ties; this is not a promise of every SQLite access
plan's incidental ordering or floating accumulation order.

LIMIT 0 and EXPLAIN bind every clause but read no input. Both bounds are validated
before runtime evaluation; LIMIT 0 skips runtime OFFSET and EXPLAIN skips both.
For a directly grouped answer, OFFSET skips projection only after finalization
and HAVING; LIMIT stops later finalizers/projection. Aggregate-argument SQL errors
are retained with their group, so an earlier LIMIT can suppress a later group's
error without retaining all source rows. Work/memory/cancellation failures are
statement-wide and are never deferred. A general result sorter still needs all
finalized groups and their HAVING/sort keys, even if top-K rejects their remaining
projection. A discarded candidate cannot suppress resource/cancellation failure.

A single-table grouping tuple containing an unwrapped physical rowid/IPK column
proves unique groups and uses the physical rowid stream, even when that column
is not the first key. Before emitting a group, the engine evaluates the next
qualifying row's WHERE and grouping keys, matching SQLite's required lookahead;
that next row's aggregate arguments remain lazy. LIMIT can then stop before
later input predicates. Direct complete key ordering whose first key is rowid
uses forward/reverse native cursors. Mismatched ORDER lengths/expressions and
nonnatural NULL placement still require the result sorter. In particular, rowid
uniqueness alone does not justify eliding ORDER BY. The plan explicitly reports
`grouped_rowid`, distinct from ordinary sorted-key `grouped` accumulation.

## Pull graph and budgets

`sqlite/grouped_aggregate.go` owns grouping; the reviewed arithmetic remains in
`internal/sql/aggregate.go`. The input evaluator and argument evaluator are
private Rows adapters, never publicly registered and never given a new snapshot
or transaction lease. One borrowed raw tuple allows lazy representative-cell
retention without forcing unused typed CASE arms. Leaves close after input is
consumed or a proved rowid stream reaches the relevant LIMIT. Only root Rows releases its snapshot/activeRows. Early close, canceled
queries, budget failure, LIMIT 0 and EXPLAIN retain this ownership discipline.

Groups use a key-sorted pointer slice with binary search, not an unaccounted hash
map or external engine. Insertion movement is charged, so this implementation
can take quadratic work on many distinct reverse-ordered keys and will reject
on MaxWork. It is deliberately a bounded correctness slice, not an optimization
claim. Outside the proved physical-rowid stream there is no index grouping pushdown.
There is no secondary-index grouping, spill, plaintext temporary file,
materialized input table, goroutine, CGO or production oracle dependency.

The existing RelationalLimits statement budget is admitted before engine-owned
bound-parameter cloning. It covers binding and structural matching, composite
rewrite arrays before allocation, metadata/maps, key bytes and accumulator
arrays before cloning, one representative's detached cells and replacement
peak, retained MIN/MAX values, per-group deferred SQL errors, input/page/record
work, movement/comparison, finalization, output buffering and sorting. Top-K
array capacity and replacement peaks are admitted before allocation/ownership;
sorted-array insertion movement remains bounded by the statement work limit. Complete
relevant results are buffered before the first public row, including the
unordered/direct-key-order path, so a late quota failure cannot publish a
successful prefix. General ordered queries keep the existing eager Query error
path; other grouping failures surface on the first Next. Public Values copies
BLOBs independently; root completion/Close retires all live accounting.

Charges are conservative accounting, not allocator/RSS measurements. Earlier
array capacities and planning copies can stay charged for the statement;
existing relational page/schema and scalar-scratch overestimation remains.
Application-owned Values copies and allocations inside caller driver.Valuer code
remain outside the engine's memory contract. Cancellation after a caller has
already consumed rows can still terminate further iteration; callers must
always inspect Rows.Err.

## Sources, tests and remaining gates

Behavior sources: public-domain SQLite check-in
`be8d059e9a49089ab2dce5ed26dd87aaf598fdc5fbe0b107c0dd758464bcd5e3`,
`src/select.c` (group sorter/output loop, updateAccumulator, havingToWhere,
sqlite3CopySortOrder), `src/resolve.c` (GROUP/HAVING binding), `src/expr.c`
(constant-or-group-key and aggregate discovery), and `src/func.c` (minmaxStep).
The official [SELECT documentation](https://www.sqlite.org/lang_select.html)
and [datatype comparison rules](https://www.sqlite.org/datatype3.html) supply
additional scope context. The Go implementation is independent; C is neither
compiled nor linked into production and no additional dependency was added.

New tests cover native/driver execution and pinned values/classes/labels, mixed
keys including NUL via hex and typeof, duplicate calls, MIN/MAX representatives,
compound keys and nested scalar expressions, source/alias/ordinal precedence,
HAVING promotion, ordinary/shared snapshots and one Tx lease, prepared reuse and
rollback, retained values, grouped parameter binding, early close/cancellation,
state admission, late work/memory failure, error timing and LIMIT/EXPLAIN rules.
Four exact former parser capability-negative statements were compared positively
to the pinned oracle before promotion. The grouped wildcard parser rejection is
retained. No unrelated legacy assertion was changed.

All executed tests use intact owned fixtures, ordinary SQL/read/write/rollback,
BUSY/cancel/close/checkpoint, or pure-memory parser/state/quota checks. No physical
corruption/tamper/wrong-key/missing-selected-file, namespace replacement/retarget,
crash/kill/process-exit/hot recovery, native I/O/lock/close/unmap/Sync failure,
short writes, disk-full/permissions or interrupted publication/rekey is tested.
Focused commands and original failed probes are under docs/evidence/grouped-
aggregates. Independent review and lead combined acceptance remain separate
requirements; the inherited full-tree planner-reason regression is not repaired
or qualified by this slice. No complete repository pass is claimed here.
