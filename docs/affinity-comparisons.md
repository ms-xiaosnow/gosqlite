# Declared BLOB affinity versus an affinity-less expression

## Correctness correction

Comparison affinity now distinguishes a physical BLOB/typeless column from an
expression with no affinity. This fixes inherited wrong-row results without
changing floating-point formatting, storage format, public APIs, type layouts,
constant-propagation planning or the Boolean increment's error/budget contracts.

For an intact table `f(t TEXT,b BLOB,u)` with `(1.0,1.0,1.0)`, stored values are
TEXT `'1.0'`, REAL `1.0`, REAL `1.0`. Pinned SQLite 3.54.0 returns:

| Expression | Result | Reason |
| --- | --- | --- |
| `t=b`, `b=t`, `t=u`, `u=t` | 0 | Both have nonnumeric declared affinity; no coercion |
| `t=+b`, `+b=t` | 1 | Unary plus removes expression affinity; TEXT coerces the REAL |
| `t=1.0`, `t=?1` with REAL 1.0 | 1 | Literal/parameter has no affinity |
| `t IN (b)` | 1 | An IN-list RHS has no comparison affinity |
| `CASE t WHEN b THEN 1 ELSE 0 END` | 0 | Simple CASE uses comparison affinity |
| `CAST(b AS TEXT)` | `'1.0'` | Explicit conversion is unchanged |

A result alias of a physical BLOB/typeless column retains BLOB affinity; an alias
of unary plus, CASE, or an ordinary function has no affinity. The public scalar,
relational and write binders already normalize a typeless physical declaration
to `BLOB`. The empty callback marker means an affinity-less result expression,
not a typeless physical column. Tests cover both paths through actual SQL.

The former evaluator treated `BLOB` and the empty marker identically in its two
TEXT-coercion branches. That incorrectly converted numeric cells held by BLOB
columns before comparison. Standalone scalar `t=b` was wrong even without a
WHERE clause, so constant substitution was not the root cause. Fixed-column
WHERE conjunctions made the same inherited coercion bug visible; they now use
the correct original column affinity with no planner change.

## Pinned evidence

`src/expr.c:350`, `sqlite3CompareAffinity`, selects numeric comparison affinity
when either declared operand is numeric, and otherwise BLOB/no conversion when
both operands have declared affinity. It uses the column's affinity when the
other expression has no affinity. `sqlite3ExprAffinity` and
`sqlite3TableColumnAffinity` retain physical/alias affinity. `src/sqliteInt.h`
distinguishes `SQLITE_AFF_NONE=0x40`, `BLOB=0x41`, `TEXT=0x42`.

Saved VDBE probes show comparison P5=81 (`0x51`, BLOB plus null-jump flag) for
`t=b`, `t=u`, and a physical-column alias; P5=82 (`0x52`, TEXT plus that flag)
for `t=+b`, `t=?1`, and the unary-plus alias. Probes also retain conjunctions in
both operand orders and their actual fixed-column code generation.

See `evidence/affinity-substitution/pinned-source-evidence.txt`,
`oracle-isolation.sql` and `oracle-isolation.txt`. The official CLI/source are
strictly test-only; production still executes SQL entirely in native Go.

## Regression evidence

- The historical independent 2160-query matrix is retained without weakened
  expectations. Pre-Boolean durable source had 44 mismatches; accepted Boolean
  v2 had 36 inherited mismatches and zero new ones. This correction has zero.
- A new explicit pure-memory regression produces 18 failed assertions using
  exact accepted Boolean-v2 evaluator bytes and passes with this correction.
- Expanded oracle matrix: 2676 queries before indexes and 1308 predicate/alias/
  parameter queries after indexes, under each of ordinary and WALShared modes:
  7968 complete-value/storage-class comparisons, zero mismatches.
- Coverage includes all six ordinary affinities, INTEGER/REAL/TEXT/BLOB/NULL
  storage, numeric-looking text, large integers, literal/parameter/function/CASE
  and unary operands, symmetric comparison/IS/order operators, BETWEEN, IN,
  simple CASE, aliases, fixed-column conjunctions, INNER/LEFT JOIN, indexed
  reads, native UPDATE affected rows/commits, DELETE rollback, and close/reopen.
- The final oracle uses genuine `.parameter` bindings and projects their values/
  classes; the earlier exploratory matrix used equivalent literal reference SQL.
  Final acceptance does not rely on that substitution.
- Cancellation and work/byte quotas remain exercised with pure-memory budgets.
  Existing scalar, Boolean, relational, expression and driver checks are retained.

Actual commands and outputs are in `evidence/affinity-substitution/`. Every
executed suite runs the source-reference guard after behavioral scope review.
Historical failure logs are preserved, including the initial independent
matrices and the deliberately red evaluator-overlay regression.

## Boundaries

This is a comparison-affinity correction, not complete SQLite compatibility or
full optimizer/index-order equivalence. Conservative index admission, join
reordering/WHERE pushdown, unsupported SQL and physical safety acceptance retain
the boundaries in `boolean-expressions.md` and the main compatibility guide.
No physical corruption/tampering, wrong-key or missing-file fixture, namespace
mutation, crash/recovery, process termination, interrupted publication, or native
I/O/close/unmap/lock/Sync/short-write/disk-full/permission fault is tested here.
