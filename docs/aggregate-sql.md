# Bounded native global aggregates

Status: **PARTIAL**, official SQLite **3.54.0** behavior baseline. This is a
reviewable SQL execution increment, not GROUP BY implementation or completion
of M4, broad SQLite compatibility, durability or production acceptance.

## Executable profile

- Global `COUNT(*)`, `COUNT()`, `COUNT(expr)`, `SUM(expr)`, `TOTAL(expr)`,
  `AVG(expr)`, `MIN(expr)`, and `MAX(expr)`.
- Zero or one legacy table source, or the existing bounded comma/INNER/CROSS/
  LEFT JOIN relation tree. WHERE and ON use the same bound predicate pipeline.
- Aggregate arguments may be existing scalar expressions. Aggregate results may
  occur inside supported scalar expressions, CASE, arithmetic, CAST and type
  inspection. Non-aggregate result expressions must contain no source columns.
- Result labels, duplicate labels, qualified input references, Boolean fallback
  names, positional/named parameters, prepared driver statements, LIMIT/OFFSET,
  and ORDER BY result aliases or integer ordinals. A single global result is
  already ordered; no general sorter or sort-key evaluation is needed. OFFSET
  removing that row suppresses final scalar projection, but input evaluation
  and aggregate finalization (including SUM overflow) still run. Both bound
  expressions are validated first; LIMIT 0 then skips runtime OFFSET evaluation,
  and EXPLAIN skips runtime evaluation of both bounds.
- Normal native, shared-WAL, SQL-control and native/driver transaction paths.
  The aggregate reads its root snapshot and transaction's own writes.

Every query without GROUP BY produces one global row, even with zero input
rows, unless LIMIT/OFFSET removes it. COUNT returns INTEGER zero for no inputs;
SUM/AVG/MIN/MAX return NULL; TOTAL returns REAL zero. COUNT(expr) ignores NULL.
MIN/MAX ignore NULL, compare with default BINARY/storage-class rules, and retain
the earliest equal value, including its INTEGER versus REAL storage class.

SUM retains exact INTEGER accumulation until a non-integer numeric type or
integer overflow switches to compensated REAL accumulation. It follows the
pinned source's delayed overflow rule: a later non-integer type clears an
already observed integer overflow. TOTAL/AVG never report integer-sum overflow.
REAL overflow can produce infinity; indeterminate opposing infinities produce
NULL. AVG is REAL for nonempty non-NULL input. Numeric TEXT conversion stops at the first embedded NUL (without truncating
MIN/MAX values). Fully numeric TEXT prefixes are converted without collapsing
decimal/exponent spelling to INTEGER; partial/nonnumeric
TEXT and all BLOB inputs use SQLite's double-prefix conversion (nonnumeric is
zero). Large INTEGER inputs use split compensated additions after REAL mode.

Ordinary SQL storage classes stay intact. The existing typed-column boundary
still lazily validates referenced VECTOR/HALFVEC/SPARSEVEC/BIT data; COUNT(*)
does not decode unreferenced values. Valid vector constructors/encodings return
ordinary BLOBs and aggregates apply BLOB semantics, not vector arithmetic.
Unknown Go value types are explicitly rejected by the internal state machine.

## Explicit unsupported boundaries

- GROUP BY and HAVING; no grouping hash table, external sort, spill or grouped
  output exists.
- DISTINCT aggregates/results, aggregate-local ORDER BY, FILTER, windows and
  compound queries; existing parser boundaries remain fail-closed.
- Bare source columns or wildcard results mixed with aggregate projections,
  including SQLite's special MIN/MAX bare-column behavior. SQLite permits some
  of these forms; this narrower slice deliberately returns ErrUnsupported.
- Multiargument scalar MIN/MAX and aggregate functions outside the named set.
- General aggregate ORDER BY expressions, including ORDER BY SUM(x); use a
  selected result alias or ordinal for the supported one-row ordering profile.
- Nested aggregates and aggregates in WHERE, ON or LIMIT/OFFSET report aggregate
  misuse. No dead arm, empty source or LIMIT 0 hides name/function validation.

Existing unsupported relation/schema/collation/CTE/subquery boundaries continue.
There is no aggregate index pushdown, fast COUNT metadata shortcut or MIN/MAX
index optimization. Plans honestly report `AGGREGATE`, method `global`, over
bounded table scans and left-to-right joins. This changes only aggregate SELECT
routing; non-aggregate single-source index/ANN planning keeps its prior path.

## Pull graph, ownership and accounting

`internal/sql/aggregate.go` contains native arithmetic state. `sqlite/aggregate.go`
lowers aggregate calls to private result slots and introduces a pull operator
above the existing relational input graph. No production path imports an
external engine, invokes the official CLI, uses CGO or delegates SQL.

The internal input evaluator is never registered as public Rows, never locks DB,
opens a database, starts a fresh snapshot or acquires a transaction lease. Join
ON callbacks and restarted right children retain the input scope. Only the root
Rows owns snapshot release and activeRows. Leaf cursors close before final scalar
projection; early Close, cancellation, terminal errors and LIMIT 0 close all
children. Public Values returns independent BLOB copies.

No input table is materialized. Each accumulator is fixed-size except one
retained MIN/MAX value; selected TEXT/BLOB values are detached from input record
backing bytes. Replacements reserve their new copy before releasing old state.
The existing RelationalLimits context API now also applies to global aggregates,
including single-table/no-FROM aggregates. Public existing struct layouts and
all disk/extension formats remain unchanged.

One statement budget is admitted before engine-owned bound-parameter copying.
It covers binding, aggregate lowering/state metadata, leaf/page/decode work,
WHERE/ON and aggregate-argument evaluation, numeric coercion, comparison, retained
MIN/MAX bytes, final values and output expressions. Cancellation remains shared.
The parser's pre-existing SQL/token/depth caps bound the pre-binding detection
walk. RelationalStats and QueryStats report real leaf work and root output,
including an input scan for COUNT(*), not fictitious indexed work. LIMIT 0 and
EXPLAIN perform binding but open no source cursor. A late budget failure returns
an error rather than a partial aggregate row.

Accounting is conservative rather than allocator/RSS measurement: relational
leaf page reservations remain charged until the leaf closes, and scalar scratch
reservations can overestimate actual usage. Large scans can therefore hit a
quota even though arithmetic state itself is constant-sized. Caller allocations
inside driver.Valuer and Values copies retained by the application remain outside
the engine quota, as in the existing relational contract.

## Provenance and verification

Arithmetic behavior was checked against the pinned public source check-in
`be8d059e9a49089ab2dce5ed26dd87aaf598fdc5fbe0b107c0dd758464bcd5e3`:
`src/func.c` (SumCtx, compensated sum, aggregate finalizers, MIN/MAX),
`src/vdbe.c` (sqlite3_value_numeric_type), and `src/util.c` (overflow classification).
These are public-domain SQLite sources. Go implementation is modular and native;
no C source is compiled or linked into production. See upstream.md and notices.

New tests cover complete labels/values/storage classes/errors against that exact
CLI, empty/all-NULL/mixed classes, extreme integers, compensated sums, REAL
infinity/NULL outcomes, source affinity, aliases/parameters, joined multiplicities,
NULL extension, ordinary/shared execution, lazy typed validation, one root Tx
lease, prepared driver reuse/rollback, cancellation, work/memory admission,
retained-output lifetime, plans and live accounting retirement.

Four former capability-negative statements were first run unchanged and their
failures retained, then promoted with exact original SQL positive official
comparisons: COUNT(*) FROM t, SUM(x) FROM t, COUNT(*) FROM samples and COUNT(*)
FROM edge. Other rejection assertions remain intact. The initial new test's
`count(*) AS n WHERE n>0` expectation was corrected after the official oracle
confirmed that physical column n shadows the alias; the positive shadow case is
retained. A second new test initially treated vector() as a non-SQL Go type;
source inspection showed its existing BLOB contract. Positive BLOB aggregate
semantics and lazy malformed-typed-value negatives now test the actual contract.

All executed tests use intact owned fixtures and ordinary SQL, read/write,
rollback/BUSY/cancel/close/shared snapshots/checkpoint, or pure-memory parser,
arithmetic and quota checks. No physical corruption/tamper/wrong-key, missing
selected file, namespace mutation/race injection, crash/kill/process-exit/hot
recovery, native I/O/lock/unmap/close/Sync failures, short writes, disk full,
permission failures or interrupted publication/rekey is exercised or qualified.
Final commands/results and independent integration review are separate evidence;
no full-tree pass is claimed while known baseline regressions remain.

The first global implementation exposed scalar output errors even when OFFSET
removed its sole result. An intact official comparison established the distinction
between input/finalizer errors (still visible) and scalar output errors (suppressed).
The preserved pre-fix regression and corrected tests cover both sides.

## Prior v2 owner check results

The v2 source completed guarded targeted pinned-oracle tests and the same
expanded aggregate/relational/error selection under race, whole-tree vet/build,
and core cross-builds for Linux arm64, Darwin arm64/amd64 and Windows arm64/amd64
at 2026-10-10T19:35:11Z. Final source matches the immutable review tree.
`go mod verify` passed and the native sqlite/driver production dependency closure
contains no CgoFiles. All existing exported sqlite/driver struct definitions
remain text-identical to the baseline.

The complete default suite intentionally remains a failure: the frozen baseline's
three Boolean ANN/sparse rank-path regressions are still present, with original
assertions unmodified. No new aggregate failure occurred. See
`docs/evidence/global-aggregates/full-checks-v2.txt` and `run-v2.sh` for exact
commands, outputs and exit statuses. Broader independent integration, full combined
passing checks and physical safety/release gates are not inferred from these
focused results.

The inherited binder cap is 2,000 total source slots, including one hidden rowid
slot per leaf; aggregate state count is independently capped at 2,000. These
intentional bounded-profile rejections are not claims of SQLite's full limits.

## Independent-review corrections

The first independent review found two related compatibility gaps: aggregate
bounds were evaluated before OFFSET binding was complete, and embedded-NUL TEXT
could be classified as REAL instead of INTEGER during SUM numeric conversion.
The bounds gap also surfaced incorrect runtime OFFSET errors after LIMIT 0 and
runtime-bound errors during EXPLAIN. The v3 correction validates both bound trees
before evaluation, skips runtime bounds for EXPLAIN, and skips runtime OFFSET for
LIMIT 0. TEXT numeric conversion now stops at NUL while identical BLOB bytes
remain REAL inputs. This also preserves an earlier integer SUM overflow when a
later NUL-terminated integer TEXT input must not clear it.

The exact reviewer probes are retained in aggregate_review_test.go, with added
native and official NUL/BLOB/overflow/binding checks. Pre-fix owner output is
retained in independent-pre-fix.txt; its earliest EXPLAIN helper could not parse
the CLI's pretty plan format and was corrected to a direct completed CLI check
before the final positive runs. Other pre-fix numeric/bounds failures are genuine
native/oracle mismatches. These fixes affect only the aggregate paths. No new
broad suite was run against the known-failing baseline; fresh combined acceptance
belongs to the independently reviewed integrated source.

Corrected v3 owner checks completed at 2026-10-10T19:43:35Z: expanded aggregate,
independent-probe, relational and original-rejection oracle selection PASS;
the same selection with race PASS; whole-tree vet/build and all five core
cross-builds PASS. Exact command/output files are checks-v3.txt and
run-v3-focused.sh under docs/evidence/global-aggregates. Independent final-source
review and the fresh complete integrated gate remain required.
