# SQLite expression prerequisite slice

Status: **partial ordinary implementation**, based on the pinned SQLite 3.54.0
source and test-only executable. This is a prerequisite subset of S2, not the
scope-aware relational binder or a completed SQL compatibility milestone.

## Added syntax and behavior

- Simple and searched `CASE`, including omitted ELSE, lazy selected result
  evaluation and one evaluation of a simple CASE operand. Simple CASE uses the
  same affinity rules as equality. Every reachable syntax node, including a
  runtime-unselected result arm, is validated before reading rows.
- Ordinary SQLite `CAST(expression AS type-name)`: integer decimal-prefix
  parsing/saturation, REAL conversion, NUMERIC's existing-number preservation
  and 51-bit text-to-integer rule, TEXT and BLOB storage conversion. Type size
  tokens affect affinity only; this does not broaden CREATE TABLE declarations.
  Empty CAST type names default to NUMERIC. The parser preserves SQLite's
  typename-span/dequoting behavior, including quoted leading names and comments.
- `BETWEEN` and `NOT BETWEEN`: one lhs evaluation; each bound independently
  applies comparison affinity; false dominates NULL when combining comparisons.
  Their precedence is SQLite's comparison tier, without changing vector tiers.
- Scalar `IN (...)` and `NOT IN (...)`: RHS values have no comparison affinity,
  even when they are columns or CAST expressions. The lhs supplies affinity;
  a match wins over NULL, no match with NULL returns NULL, and the empty list
  returns false/true even for a NULL or error-producing lhs. Like pinned SQLite,
  function-free empty-IN operands are folded before name binding. Operands with
  functions still undergo name/function validation, but are not evaluated.
  Lists with more than two constant RHS expressions evaluate every RHS item
  even after a match, preserving SQLite's later-value errors; shorter or
  dynamic lists short-circuit. Parameters and bound constant result aliases
  qualify; physical columns and unknown/custom functions do not. Evaluation
  streams these values without materializing an unbounded RHS set.
- Explicit `SELECT ALL` and `ORDER BY ... [ASC|DESC] NULLS FIRST|LAST`. NULL
  placement is independent of direction. Ordinals, result aliases and stable
  tie behavior retain the existing rules.

CASE and IN evaluate one arm/list value at a time; no RHS set or row materializer
is allocated. CAST TEXT/BLOB conversion is at most input-sized, using ordinary
storage bytes. The 1 MiB SQL input, 32,768 tokens, 128 expression nesting/tree
height, 32,766 bind slots and existing function/result limits remain. Scalar
node evaluation now checks the operation context, including between large-list
items. Parser work is bounded rather than asynchronously cancellable. General
sorting still charges `MaxSortBytes` and returns an error instead of truncating.
A query-wide retained/transient allocator remains separate required S2 work;
this slice does not claim that arbitrary existing scalar outputs or bind buffers
are covered by one combined budget.

## Representation and planner contracts

Dense VECTOR, HALFVEC, BIT and SPARSEVEC identity stays separate from ordinary
SQLite affinity. CASE result arms merge known identity just as existing
coalesce/ifnull choices do; competing representations yield `mixed` and reject
implicit cross-kind metric/constructor use. This is conservative across all
result arms, including an arm not selected by a particular row.

TEXT/BLOB casts retain originating representation evidence, but never decode,
re-encode, quantize or reinterpret the private GSV/GSH/GSB/GSS bytes. Numeric
casts produce ordinary SQLite scalars. In particular `CAST(value AS VECTOR(3))`
has SQLite NUMERIC affinity; use the explicit extension constructors/converters
for actual vector conversion. Casting a valid typed external TEXT to BLOB does
not magically make it a valid private vector BLOB.

ASC NULLS FIRST can retain the existing bounded exact vector heap. NULLS LAST
uses the bounded general sorter and cannot enter the existing NULL-first ANN or
heap path. CASE-wrapped metrics are ordinary scalar expressions rather than
claimed index plans. Ordinary equality and grams planning remain conservative;
new expression forms are not mined for unproven candidate constraints.

`VisitChildren` and `Walk` include all new nodes. `Rewrite` copies composite
nodes and child lists so TRUE/FALSE physical-column fallback binding does not
mutate parsed expression trees. CAST result aliases carry their comparison
affinity; physical columns retain precedence in WHERE aliases, including
columns with an empty declared type. An empty physical declaration must never
acquire a colliding vector result alias's representation identity.

## DDL, writes and boundaries

INSERT, UPDATE, DELETE, WHERE, ORDER BY and LIMIT/OFFSET use the same expression
validation/evaluation paths. Native CREATE retains its pre-existing acceptance
of parenthesized constant expressions, now including the new constant forms.
Column/parameter/function defaults remain refused. Missing trailing fields in
externally changed records still accept only the existing literal-default
subset. No whole-catalog ownership check is relaxed and no disk format changes.

Not implemented by this slice: scoped slots/relations, JOIN, subqueries (including
IN-subqueries), aggregation/GROUP/HAVING, COLLATE, CTEs, DISTINCT/compounds,
window functions, function-kind registry, PRAGMA registry or general schema
qualification. These remain required code, independently of restricted physical
durability, native-platform, independent security/licensing and M10 gates.

## Evidence

See `evidence/s2-expressions-review.txt` for source audit and commands,
`s2-expressions-scalar.txt` for the complete scalar suite and independent pinned
oracle matrices, `s2-expressions-components.txt` for native/driver checks and
CGO0 vet/build, `s2-expressions-race.txt` for separate CGO1 race tooling, and
`s2-expressions-database-oracle.txt` for ordinary rollback/shared-WAL query and
integrity comparisons. Final owned source hashes are recorded separately.
These are focused ordinary checks, not a full-repository acceptance claim.
