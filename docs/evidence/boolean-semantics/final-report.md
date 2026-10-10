# Context-sensitive Boolean correction: scoped acceptance

Owner result: scoped PASS; full SQLite compatibility remains PARTIAL.
Independent reviewer reports no new blocking defect on immutable v2.
No main-worktree production edits, staging, commit, push or upload were performed.

## Exact candidate

- Immutable source: /workspace/scratch/5e06a74e5f65/durable-restart/boolean-semantics-freeze-v2
- Complete SHA256 manifest: /workspace/scratch/5e06a74e5f65/durable-restart/boolean-semantics-evidence/freeze-v2.sha256
- Manifest SHA256: be6ece6401dca41264cc1846ba4674db0977626724af25798d26351aa6bdc96a
- Machine-readable delta: freeze-v2.json, 21 changed files.
- Base: source tree ad73b8d6425cd86d1c15caccb0f858048df63ef9, identical production to initial assigned tree 2f1ab3df86cdf5aaf97143d6ad53d186da836b3f plus independently reviewed legacy JOIN rejection fixture correction.
- Patch: /workspace/scratch/5e06a74e5f65/durable-restart/boolean-semantics-evidence/boolean-v2.patch
- Patch SHA256: 76af89b7d6a46d612d7a85d2281bc38de942a35dd6b51ae97b26e9b6d9091d45
- Patch was applied to an isolated archive of exact base ad73b8d; all 21 changed-file hashes match v2. Full candidate hashes were checked unchanged after suites.

## Deliverable

Syntactic Boolean/literal-NULL folding, scalar versus branch evaluation,
NULL-aware NOT/IS/BETWEEN propagation, searched-CASE predicate context,
WHERE/INNER-ON deterministic top-level-AND startup terms, LEFT-ON context,
alias occurrence isolation, affinity-preserving fixed-column propagation,
lazy typed errors, DML predicates, cancellation/quotas, conservative index gates.
Fixed-column proof is O(1) per certified reference; expanded expression work is
bounded before propagation can introduce exponential evaluator work.

Dedicated guide: docs/boolean-expressions.md.
Dedicated test audit: docs/evidence/boolean-semantics/scope-audit.md.
Parent should reconcile older central documentation's inherited eager-gap
section during integration; this task intentionally does not modify it.

## Executed evidence

- frozen-checks.sh against v2: exit 0, frozen-v2-checks.txt/.exit.
- CGO=0: full internal/sql ordinary tests; selected sqlite/driver Boolean,
  relational, ordinary-index, expression, vector-review and grams tests.
- CGO=0 oracle: all internal/sql tagged tests; 1,053 new Boolean context cases
  and existing relational/expression conformance cases.
- CGO=1 race: selected internal/sql/sqlite/driver Boolean, relational,
  ordinary-index, expression and vector-review tests.
- go vet internal/sql, sqlite, driver: pass.
- Every test suite preceded by go run ./internal/testscope and selected behavior
  audited. No manual_* tags or prohibited storage/process workflows.
- Original exact-tree 923-case oracle test: FAILED with 291 mismatches;
  historical-oracle-failure.txt/.exit retained. This is historical evidence,
  not a current pass claim.
- Legacy eager residual assertion failed as expected in targeted-first.txt.
  Direct CLI evidence oracle-legacy-residual.txt confirms the exact query now
  returns zero rows. Its assertion remains as an explicit successful-empty
  regression; constant overflow and typed-error assertions remain.
- Intermediate implementation/test failures are retained, including the pure
  NULL NOT BETWEEN test expectation correction with direct pinned evidence.
- Independent reviewer: owner v2 byte identity and pre/post manifest checked;
  native and six independent normal/race tests pass. Additional ordering and
  literal-NULL probes found no new mismatch. Final independent report/logs are
  owned by the reviewer under gosqlite/docs/evidence/boolean-independent.

## Remaining boundaries, explicitly not hidden

The broad independent oracle remains intentionally FAIL: 5,332 contextual probes
have 5 known parser/index/pushdown boundary differences, and 2,160 affinity probes
have 36 inherited wrong-row differences. The previous durable source has 44
in that affinity matrix: zero new differences, eight resolved. Do not convert
these into a full-oracle PASS claim.

1. Ordinary index admission stays conservative. Native can scan and expose an
   earlier row-dependent error where SQLite seeks, even under NOT INDEXED for
   an INTEGER PRIMARY KEY. The source-order error regression is explicit.
2. Join WHERE pushdown/reordering remains absent. A later WHERE may not suppress
   an ON error that SQLite's planner avoids before constructing pairs.
3. Inherited TEXT-versus-BLOB/untyped column comparison affinity gives wrong
   rows independently of this Boolean increment. On f(t TEXT,b BLOB) inserted
   (1.0,1.0), both engines store t='1.0', b=REAL1.0 and CAST(b AS TEXT)='1.0',
   but native t=b and b=t are true while pinned SQLite returns false. Reviewer
   traced SQLite's nonnumeric declared-column affinity combination to BLOB/no
   conversion, unlike the native comparison-affinity rule. Unary +b removes
   column affinity and both engines agree true. A separate worker owns this fix.
4. Previously unsupported grammar and unquoted reserved-word alias boundaries
   remain. Non-Linux runtime and full durability qualification are not covered.
