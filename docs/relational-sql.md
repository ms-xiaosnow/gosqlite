# Scoped relational execution: first JOIN slice

Status: **PARTIAL**, SQLite behavior baseline 3.54.0. This is an executable
S2/S4 increment, not completion of either stage or broad SQLite SQL support.
Production execution is native Go; the official CLI is used only by tagged tests.
No file format, extension codec, catalog ownership rule or public existing
Options/QueryPlan/QueryStats layout changes in this slice.

## Executable profile

- Named rowid-table leaves, optional quoted/unquoted aliases and per-leaf
  `NOT INDEXED`, joined by comma, `JOIN`, `INNER JOIN`, `CROSS JOIN`, or
  `LEFT [OUTER] JOIN`; optional `ON` is a scalar expression. Evaluation is
  left-to-right, including mixed comma/explicit join syntax.
- Scoped qualified/unqualified columns, physical-column ambiguity errors,
  `*` / qualified stars, declared column spelling in labels, duplicate result
  labels, per-source rowid/IPK aliases and physical rowid shadowing.
- `ON` uses source affinity and default BINARY comparison semantics before
  LEFT null extension. `WHERE` runs after joins, including null extension.
  Result aliases are available in WHERE/ON when no physical name shadows them.
  A bare ORDER BY result name takes precedence; a nested ORDER expression
  resolves physical names first. Existing integer ordinal/NULL order/LIMIT/
  OFFSET/scalar/typed representation rules continue to apply.
- Ordinary projections, lazy CASE/NULL/typed-column validation, existing exact
  vector scalar math and snapshot-configured grams13_match work on joined rows.
  No ANN Top-K or candidate filtering is pushed below joins. Joined scalar
  distance ordering is exact and uses the bounded general sorter.
- No source row is read during multi-source `Query` without ORDER BY or during
  EXPLAIN. Schema is read and validated against the root snapshot. Unsorted
  rows are pulled as needed; LIMIT 0 never opens a table cursor. ORDER BY is
  eager, bounded and cancelable, preserving the existing Query startup-error
  versus Rows.Err iteration-error behavior.

## Deliberate unsupported boundaries

`USING`, `NATURAL`, RIGHT/FULL joins, parenthesized joins, derived tables,
subqueries/correlation, schema-qualified FROM, CTEs, aggregate/grouping,
DISTINCT/compound/window queries, COLLATE expressions, and INDEXED BY are not
implemented. Unlisted schema/collation grammar still fails closed.

All ON names are resolved in the full FROM scope, including ambiguity contributed
by later leaves. An ON expression (including a result alias) that references a
later relation is rejected with ErrUnsupported. SQLite accepts some such INNER/
comma forms; this intentionally narrower execution profile does not pretend
those forms are SQLite errors. LEFT forward references are also rejected. The
original parsed expressions are not mutated to accommodate this restriction.

Inner ordinary-index probes and join reordering are future optimization code.
The existing single-source planner still uses its real ordinary B-tree, grams,
exact heap, HNSW and IVFFlat leaves, unchanged. Join plans honestly report `JOIN`
and `nested_loop` with table scans and the absence of index/ANN pushdown. This is
not a speedup claim; N-by-M scans may hit the work quota.

## Remaining inherited expression/error gap

The existing evaluator eagerly evaluates binary AND/OR operands. This is an
actual SQLite semantic gap, including in the new ON/WHERE pipeline; existing
regression tests requiring that behavior are not a correctness authority. This
slice does not reconcile SQLite predicate constant folding/short-circuit error
suppression with the existing single-source planner. CASE result-arm laziness,
NULL extension and empty-source laziness above should not be read as a claim
of complete predicate/error equivalence.

On intact tables `a(x)` with `(1)` and `b(y)` with `(2)`, the pinned 3.54.0 CLI
completes the following queries without error, whereas this native revision
reports `integer overflow` through Rows.Err:

- `SELECT a.x FROM a,b WHERE 0 AND abs(-9223372036854775808)`:
  SQLite returns zero rows.
- `SELECT a.x FROM a JOIN b ON 0 AND abs(-9223372036854775808)`:
  SQLite returns zero rows.
- `SELECT a.x FROM a,b WHERE 1 OR abs(-9223372036854775808)`:
  SQLite returns the row `(1)`.

Exact ordinary probe evidence and its source are retained in
`docs/evidence/relational-eager-boundary.txt` and
`docs/evidence/relational-eager-probe.go.txt`. Existing assertions are preserved.
A coherent evaluator/planner correction remains open, alongside the unsupported
SQL grammar listed above.

## Binding, pull graph, and snapshot ownership

`internal/sql/from.go` parses a relation tree, limited to 64 leaves. Legacy
single-table Select.From/Alias/NotIndexed fields stay on their old path. The
relational binder loads the catalog with the supplied reader, resolves names
once, and lowers them to private unique slots with their declared SQL types.
These slots adapt to the existing scalar evaluator rather than inventing a
second affinity/vector evaluator. An extra hidden nullable rowid slot belongs
to each leaf; no universal rowid identifies a joined tuple. Hidden slots never
appear in wildcard output. Both total internal slots and expanded results are
capped at 2,000, checked before appending more entries.

The native nested-loop operators retain one current outer tuple and one live
right child, reopening the right cursor on the same borrowed PageReader. They
never recursively call DB.Query, acquire DB.mu, create a DB connection, start a
fresh snapshot, or materialize an unbounded inner table. Rows owns the snapshot
release exactly once. Tx query read detection includes the new relation tree;
only the root increments activeRows. Errors/cancellation/Close close descendants
before the root lease, and public Values copies remain safe to retain.

LEFT matching records whether any ON result was true, independently of the
later WHERE. An unmatched left tuple is emitted once with every right slot,
including its rowid and defaulted fields, set to NULL.

## Work and memory contract

`sqlite.WithRelationalLimits(ctx, sqlite.RelationalLimits{MaxWork: ..., MaxBytes:
...})` scopes limits to a multi-source query. Zero selects 16,777,216 work units
and Options.MaxSortBytes (64 MiB by default). MaxBytes may lower but never raise
that connection cap. Negative values are rejected before parameter binding. These settings
are independent of legacy single-source/ANN budgets; they do not change those
APIs. `Rows.RelationalStats()` returns cumulative source opens/rows, logical page
read attempts, pair checks, null extensions, work, live/peak charged bytes.
Existing QueryStats.BaseRowFetches counts real leaf fetches, not joined tuples.

One budget is created after parsing and before engine-owned parameter cloning.
It is shared across binding/planning, all leaf opens/restarts, page reads,
record decoding, tuple assembly, ON/WHERE/projection expressions, retained
results, and sorting. Checked binder rewriting stops at the first failure;
expanded alias validation and metadata walks also consume work. Work includes
source record bytes, declared type names, scalar expression visits, conservative
operand-byte work, and worst-case LIKE/edit-distance cell comparisons. Existing
individual codec/tokenizer/scalar limits still apply as additional ceilings.
Budget exhaustion returns ErrQueryWorkLimit or ErrMemoryLimit, never successful
truncation; callers consuming streaming rows must check Rows.Err. The in-memory
stable merge sorter stops on quota/cancellation rather than completing a sort
with a failed comparison callback. No plaintext or encrypted spill is attempted.

Charged memory is deliberately conservative, not an allocator/RSS measurement:

- converted parameter retention and BLOB clones, admitted before copying;
  plan/expression copy reservation; parsed catalog backing bytes retained for
  the operation; per-column binding/output metadata
- all cursor-read page/map/frame charges until that child closes, even after
  a page leaves its path; cell/span arrays use exact count-scaled capacities
  admitted before parsing allocates them
- payload/decode reservation before overflow allocation via the additive
  internal B-tree cursor preflight; current records, hidden rowids and tuples
- scalar coercion/concat/hex/typed codec/tokenizer scratch before allocation,
  retained until the current ON or output-row evaluation ends
- retained sort rows and their original backing data, append capacity margin,
  and the merge-sort scratch array

Scalar reservations can substantially overestimate actual usage: 8x ordinary
operand bytes, 64x LIKE operand bytes, 128x vector encoded inputs, 1024x
similarity/tokenizer inputs,
plus fixed overhead, bounded float64 decimal-staging scratch and bounded
sparse-to-dense scratch. This means a small
quota can reject a query that could theoretically fit. It is safer than silently
omitting rows. The parser's pre-existing 1 MiB SQL/token/depth caps apply before
binding; root budget accounting is not a process-wide memory guarantee or a
replacement for those parser limits. Parameter conversion through driver.Valuer is caller code and its own
allocations/execution are outside this quota; its returned bytes are admitted
before this engine retains or clones them. Values copies retained by the
application are application-owned. Retained schema/page accounting is intentionally not
recycled optimistically. Rows.Close/terminal completion clears live accounting.

## Verification and remaining evidence

New ordinary tests:

- `internal/sql/from_test.go` and `eval_budget_test.go`: relation AST/parameters/
  legacy adapter, unsupported grammar/depth bounds, checked rewrite stopping,
  searched CASE/LIKE ESCAPE/LIKE scratch preflight
- `internal/btree/cursor_budget_test.go`: valid dense in-memory page cell/span
  capacity accounting before allocation
- `sqlite/relational_bind_budget_test.go`: descriptor-free BLOB/Valuer admission
  before an engine-owned copy, plus defensive-copy behavior
- `sqlite/relational_test.go`: streaming multiplicities and null extension,
  scoped aliases/ambiguity/lazy errors, early Close/cancel/work/memory errors,
  one shared-WAL snapshot across restarts, one Tx lease, overflow-record
  preflight before reading overflow pages, wildcard expansion bounds,
  conservative scalar growth bounds, Boolean name/label rules, LIMIT 0 planning
  quotas, previous-output lifetime, type families and grams exact filtering
- `internal/testkit/relational_conformance_test.go`: pinned quote-mode oracle
  comparisons of values/classes/labels, defaults, errors and wildcard rules
- `driver/relational_test.go`: repeated prepared/bound queries, row metadata,
  transaction lifecycle, early close, cancellation, quotas, shared snapshots,
  and explicit forward-ON unsupported behavior

Existing audited scalar, ordinary-index and vector-rank tests are rerun to
check that single-source access and lazy errors remain intact. Frozen-source
commands, exact hashes and actual results are recorded under docs/evidence.
Independent source review and final aggregate integration are separately
reported; no prior source snapshot’s evidence is silently repurposed.

All executed database tests use intact owned fixtures, ordinary read/write,
normal rollback/BUSY/cancel/close/reopen/checkpoint, or pure-memory negative
parser/math/quota cases. No database corruption/tamper/wrong-key/missing-file,
namespace replacement, crash/kill/process-exit/hot recovery, native I/O/lock/
unmap/close/Sync failure, disk-full/permission, publication or interrupted-rekey
workflow is qualified by this slice. The existing separately gated manual
sources remain excluded. Native non-Linux runtime, broad SQL and full durability
acceptance remain open.
