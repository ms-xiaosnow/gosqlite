# Independent comparison-affinity review

## Decision: accept the narrow correction

No blocking finding. The two executable condition changes in
`internal/sql/eval.go` correctly distinguish declared BLOB/typeless physical
columns from expressions with absent affinity. No formatting, planner, file
format, public API/type layout, or budget/context behavior was changed.
This decision applies only to frozen affinity candidate v1, on accepted Boolean
v2. It is not a full SQL, optimizer, index-order, durability, or release claim.

## Immutable input and ownership verification

- Base full manifest: `be6ece6401dca41264cc1846ba4674db0977626724af25798d26351aa6bdc96a`.
- Candidate full manifest: `8b060026749d944fc027840e834a840659566789668292a93902e6b1cc5728be`.
- Candidate patch: `04cfa743469f0205f26ee5339ac340e5ee5609678484c6e800cb2b35970be04a`.
- Base has 846 files; candidate 879. Exactly 35 paths differ, matching all 35
  declared owned paths, with no missing, extra, or undeclared delta.
- Only changed production source is `internal/sql/eval.go`. Two executable
  conditions remove `BLOB` as an alternative to the empty-affinity marker;
  other additions in that file are explanatory comments.
- Every existing base test file is byte-identical in the candidate; test deltas
  are additions. The historical 2160-query function is independently compared
  against the preserved reviewer source, with its test name normalized only.
- Final rehash after all execution confirms both source freezes unchanged, and
  the isolated tested candidate matches every manifested candidate byte.

See `immutable-delta.json`, `production.diff`, and `final-integrity.json`.

## Primary-source proof and implementation tracing

The pinned SQLite 3.54.0 source ID is
`2026-10-09 15:46:58 be8d059e9a49089ab2dce5ed26dd87aaf598fdc5fbe0b107c0dd758464bcd5e3`.
Exact source/executable SHA-256 values and numbered excerpts are retained in
`primary-source.txt`; a fresh normal-EOF CLI probe is in `pinned-probe.*`.

1. SQLite `src/build.c:1559,1625-1628` assigns BLOB when a physical column has
   no type declaration. `src/sqliteInt.h:2370-2376` distinguishes NONE (0x40),
   BLOB (0x41), TEXT (0x42), and numeric affinities.
2. `src/expr.c:24-52` preserves physical/alias affinity.
   `sqlite3CompareAffinity`, lines 350-365, uses NUMERIC if either of two
   declared operands is numeric, BLOB/no coercion otherwise; when an operand
   has no expression affinity it uses the other's declared affinity.
3. Native `sqlite/query.go:132` normalizes omitted declarations to BLOB.
   `sqlite/rows.go:284-318`, `relational.go:264`, `write_rows.go:237`, and
   `predicate_constants.go:17` all expose that normalized physical affinity.
   Result aliases recurse through `ExpressionAffinity`; physical-column aliases
   keep affinity, while unary/function/CASE aliases return the empty marker.
4. Native parser parentheses return the enclosed AST rather than a wrapper;
   `ExpressionAffinity` recognizes Column and Cast, deliberately not Unary,
   Call, Case, Parameter, or Literal. Numeric precedence is unchanged.
5. CASE, scalar and predicate BETWEEN, IS, and ordinary comparisons route to
   the corrected helper. IN-list deliberately supplies nil for RHS affinity,
   preserving SQLite's expression-affinity removal. NULL handling occurs before
   coercion. Both mixed-bound BETWEEN directions were independently tested.
6. Native fixed-column evaluation first applies the original source affinity;
   it retains the Column AST and affinity callback. SQLite's corresponding
   EP_FixedCol rationale and guarded BLOB substitutions are in
   `src/select.c:4923-5060`. The candidate changes no substitution rule.

Fresh pinned results for `f(t TEXT,b BLOB,u,n NUMERIC)` containing REAL 1.0:
`t=b`, `b=t`, `t=u`, `u=t` are 0; `t=n`, `n=t`, `t=+b`, `+b=t`, and both REAL
parameter orders are 1. Parentheses preserve physical affinity; IN-list RHS
removes it; simple CASE and direct BETWEEN preserve it. CAST text is `'1.0'`.
The EXPLAIN probe retains P5 BLOB for physical operands and TEXT for unary and
parameter comparisons, independently of floating-point formatting.

## Independent execution

Offline pinned Go 1.27.2, shared `../go-cache` / `../go-path`,
`GOFLAGS=-buildvcs=false`, CGO0 ordinary tests and vet, CGO1 race. The compiler
maintenance hold was observed until the lead explicitly resumed. Before every
suite, `go run ./internal/testscope` passed after source/helper behavioral review.
The audit is `scope-audit.md`; aggregate commands are in `run-independent.sh`.

- Native affinity/parser/scalar, work/byte quota and cancellation checks: PASS.
- Native public indexed/scan/relational aliases, real parameters, LEFT JOIN,
  UPDATE/DELETE affected rows, rollback and close/reopen: PASS.
- New independently authored cross-value matrix: 2016 queries, 70,744 complete
  typed rows, zero mismatches. Each of ten left inputs is paired with each of
  ten right inputs, covering NULL, INTEGER, REAL, numeric-looking TEXT,
  nonnumeric TEXT, BLOB, empty BLOB, and negative REAL. Six declared affinities,
  physical/parenthesized/unary/function/CASE/CAST forms, eight comparisons,
  simple CASE, IN/NOT IN, NULL IN entries, mixed BETWEEN bounds, physical and
  computed aliases, fixed comparisons, and before/after 12 ordinary indexes
  are exercised under both ordinary and WALShared modes.
- Owner historical matrix, independently rerun: 2160 queries, zero mismatches.
- Owner expanded matrix, independently rerun: 7968 complete-value/class queries,
  zero mismatches. Includes genuine SQLite `.parameter` bindings, joins, and
  before/after indexes. Ordinary committed UPDATE and DELETE rollback oracle
  checks pass.
- Oracle-tagged vet for internal/sql, internal/testkit and sqlite: PASS.
- CGO1 race-selected affinity tests in internal/sql and sqlite: PASS.

Raw output and exit status: `candidate-regressions.*`, `candidate-independent.*`,
`pinned-probe.*`. Tests were executed only in `../affinity-independent-work`.
No main production, source freeze, staging, commit, upload, or external state
was changed by this review; only the review evidence directory was written in
main checkout.

## Detection sensitivity / retained failing evidence

A separate isolated clone replaced only `internal/sql/eval.go` with the exact
accepted Boolean-v2 bytes, verified by SHA-256. The new reviewer test is
byte-identical in candidate and negative-control trees. No expectations,
fixtures, assertions, or query sets changed.

That unpatched negative control correctly exits 1: 8 scalar-query mismatches,
22 predicate/alias mismatches before indexes and the same 22 after indexes,
52 total failing query comparisons in one journal mode. Candidate passes the
same 1008 queries in that mode and another 1008 in WALShared mode.
`baseline-overlay.json` records evaluator hashes; `baseline-independent.*`
retains every mismatch and its first differing complete row.

## Genuine remaining boundaries

- These finite matrices do not establish complete SQLite SQL compatibility,
  planner/error-order equivalence, every custom collation/extension affinity,
  arbitrary Unicode/NUL text behavior, nonfinite numerics, subquery affinity,
  or all compound-select cases. Unsupported syntax remains unsupported.
- Indexed-result equality is checked; the new matrix does not assert exact
  index admission/plans or index source-order equivalence.
- Matrix reads compare native persisted storage with pinned read-only SQLite;
  this review is not a new independent proof of every insertion conversion or
  file-format behavior. Ordinary scalar expected results and the independently
  SQL-created pinned in-memory probe establish the specific coercion decision.
- No physical corruption/tampering, wrong-key/missing-file, namespace mutation,
  native failed I/O/close/unmap/lock/Sync/short-write/disk-full/permissions,
  crash/kill/process-exit/hot recovery, interrupted publication/rekey, or
  non-Linux runtime acceptance was attempted. Existing gaps remain open.
