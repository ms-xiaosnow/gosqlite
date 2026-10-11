# Boolean execution increment

Status: PARTIAL. Baseline: official SQLite 3.54.0, source ID
2026-10-09 15:46:58 be8d059e9a49089ab2dce5ed26dd87aaf598fdc5fbe0b107c0dd758464bcd5e3.
This increment corrects the inherited eager AND/OR gap documented in
`relational-sql.md`. It does not claim complete SQLite planner/error ordering.
Production execution remains native Go. No storage format or exported public
SQLite Options/Rows/QueryPlan layout changes are introduced.

## Execution contexts

- Scalar AND/OR retains eager evaluation of non-folded operands. `0.0 AND
  abs(minint)` and `? AND abs(minint)` are not universally lazy. Small integer
  tokens and bound TRUE/FALSE permit syntactic Boolean simplification in either
  operand order. Unary signs, numeric separators, REAL/TEXT literals, parameters
  and current physical row values are not treated as such syntax constants.
- Function-free false AND folding happens at parse time, matching SQLite's
  validation exception (`0 AND missing` is zero). Function-bearing dead branches
  still undergo full name and arity validation. Non-NULL literal IS [NOT] NULL
  folding and empty-IN Boolean annotations follow the pinned parser as well.
- WHERE, ON and searched CASE use branch evaluation. NULL rejection is propagated
  through NOT, IS TRUE/FALSE and BETWEEN instead of manufacturing an intermediate
  scalar Boolean. Scalar comparison/function operands stay in scalar context;
  simple CASE WHEN operands stay scalar. BETWEEN evaluates its lhs once.
- WHERE is split into top-level AND terms. Deterministic row-independent terms
  run in source order before the first row fetch, even on empty tables/empty
  candidate sets. Thus `WHERE flag AND abs(minint)` still errors, even if flag is
  false everywhere; `CASE WHEN flag AND abs(minint)` can skip the error.
  `WHERE abs(minint) AND 0` errors, whereas scalar `abs(minint) AND 0` is zero.
- INNER/CROSS ON constant terms participate in that startup pass. LEFT ON is
  evaluated only on real candidate pairs; it disables the pinned literal
  simplifier throughout its expression and does not hoist terms. Empty right
  sources still null-extend without evaluating ON.
- WHERE aliases are expanded before predicate compilation. Each occurrence owns
  its own column identity, so fixed-column substitution cannot mutate a prepared
  expression or leak into its projection. Physical name precedence is retained.
- Equality-based constant propagation is predicate-local and preserves the
  original column affinity, including SQLite's BLOB-affinity restrictions and
  transitive equality handling. LEFT ON and extension representation-bearing
  columns/results are excluded. Unknown/context-supplied functions do not gain
  a determinism guarantee. Extension functions remain bound to their owning
  operation context and snapshot.

LIMIT 0 and EXPLAIN do not execute startup terms. Unsorted startup failures are
reported by Rows.Err; ORDER BY retains its eager Query error behavior. Context
cancellation, lazy typed-cell interpretation and scalar work/memory budgets
remain effective. Fixed-column constancy is compiler-certified without walking
an exponentially expanded substitution graph. Propagation computes expanded
node weights and rejects any single expanded constant above 1,048,576 nodes
with ErrQueryWorkLimit, including legacy single-source queries. New predicate metadata is conservatively charged to relational
work/retained-memory limits. No index admission expansion or new join pushdown
is included.

## Current bounded IPK correction and remaining planner boundary

The current unpublished composition adds the [bounded direct IPK equality
exception](ordinary-index.md) before the conservative general total-residual gate,
including under NOT INDEXED. On intact `t(id INTEGER PRIMARY KEY,k INTEGER)`
containing `(1,1)`, the retained regression is:

    SELECT id FROM t [NOT INDEXED]
    WHERE k LIKE 'x' ESCAPE 'ab' AND id=9999

The bounded candidate now seeks the missing IPK and returns no rows without
evaluating the row-dependent ESCAPE error. Reversing the conjuncts also returns
no rows. A matching id still evaluates the residual in order and reports the
error when reached; row-independent startup errors still occur before row access.
The separate V4 input has finite normal and 24-case pinned-oracle PASS; those
results do not qualify the new composition. CAST/general-expression
planning and broader index/error-order transformations remain outside the fix.

Join reordering/WHERE pushdown remains absent. For a LEFT JOIN with ON
`abs(t.v) AND t.flag`, row `(flag=0,v=minint)`, and WHERE `t.flag=99`, native ON
runs before that later WHERE and errors; SQLite can reject the left row first.
The review also found inherited TEXT-versus-BLOB/typeless wrong-row comparisons,
including standalone `t=b` and conjunctions on `(t='1.0', b=1.0)`. The subsequent
[comparison-affinity correction](affinity-comparisons.md) resolves these: declared
BLOB affinity is distinct from an affinity-less expression, so two nonnumeric
declared operands do not gain TEXT coercion. The historical 2160-case matrix
now has zero differences. This does not change the planner/error-order boundary
above.

Full error-order equivalence requires matching those planner transformations,
not more universal laziness in the evaluator. Existing unsupported SQL forms
and unquoted reserved-word alias boundaries are unchanged.

## Sources and checks

Pinned public-domain source references:

- `src/expr.c`: sqlite3ExprAnd; sqlite3ExprSimplifiedAndOr;
  exprCodeTargetAndOr; sqlite3ExprIfTrue/sqlite3ExprIfFalse; exprCodeBetween;
  fixed-column affinity handling in TK_COLUMN.
- `src/parse.y`: INTEGER annotation and sqlite3PExprIsNull.
- `src/where.c`: false-WHERE-term bypass after sqlite3WhereSplit(TK_AND).
- `src/select.c`: findConstInWhere, propagateConstantExprRewrite,
  propagateConstants; BLOB-affinity and outer-ON exclusions.

New regressions: internal/sql/eval_predicate_test.go, sqlite/boolean_test.go,
driver/boolean_test.go and the tagged internal/testkit/boolean_conformance_test.go.
The original 923-case harness was run unchanged against exact pre-increment
source tree 2f1ab3df86cdf5aaf97143d6ad53d186da836b3f: 291 mismatches were recorded.
The expanded matrix currently checks 1,053 cases. Actual frozen-source commands,
hashes and outcomes accompany this guide; intermediate failures are preserved.

One legacy ordinary-index assertion changed with direct oracle evidence:
`id=9999 AND k LIKE 'x' ESCAPE 'ab'` must return zero rows rather than error.
The exact historical query remains in a stronger result assertion, and both
constant overflow and typed-error assertions remain. No assertion was removed
to hide a mismatch.

All executed checks use pure-memory parser/math/evaluator inputs or intact
ordinary create/read/write/rollback/cancel/close fixtures. The CLI is a test-only
oracle and finishes normally. No physical corruption, wrong-key, missing-file,
namespace mutation, crash/kill/process-exit/hot-recovery/native-I/O-fault work is
qualified. Full database acceptance and non-Linux runtime validation remain open.
