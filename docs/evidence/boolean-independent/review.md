# Independent Boolean-context correction review

Review date: 2026-10-10 UTC. Decision: accept the scoped Boolean correction in
immutable owner freeze v2, with the explicitly retained boundaries below. This
is not broad SQLite equivalence or full database/durability acceptance.

## Exact source

- Durable baseline: `8f75ccf927b8dc0048d43c65736bdc3ad6fee20c`.
- Owner comparison tree: `ad73b8d6425cd86d1c15caccb0f858048df63ef9`.
- Reviewed immutable candidate: sibling `boolean-semantics-freeze-v2`.
- Owner full-manifest SHA-256:
  `be6ece6401dca41264cc1846ba4674db0977626724af25798d26351aa6bdc96a`.
- All owner files were verified before and after the final runs. The isolated
  tested copy matched every owner file, with only the six additional reviewer
  test files enumerated in `tested-copy-overlay.json`.
- `v2-exact-delta.json` records all 21 candidate changed paths and their exact
  base/candidate hashes. No main production file, staging state, commit or
  published artifact was changed by this reviewer.

The first reviewer snapshot and v1 evidence remain intact. They are not used as
v2 acceptance evidence. Owner v1 differed from the first reviewer snapshot in
three production files, recorded in `v1-production-since-initial.txt`.

## New blocker found and corrected

v1 recursively re-proved fixed-column constancy through an expanding substitution
graph, without accounting for the expanded work. The pure-memory diagnostic
visited 65,536 leaves at depth 16 while leaving a one-unit work budget untouched.
An ordinary, empty-table LIMIT 0 query at depth 18 spent 43.286 ms planning and
reported only 118,837 work units. Timing is diagnostic, not a performance claim.

The owner supplied v2 with compiler-certified fixed-column constancy and bounded
substitution-node weights. Independent verification shows zero expanded leaf
visits during constancy classification, approximately 0.803 ms for the bounded
depth-18 diagnostic, and ErrQueryWorkLimit for depth 24 before opening a source.
Runtime evaluation still checks context and charges visited scalar operations.
The owner regression additionally checks the relational planning quota. The
weight map follows already-certified immutable substitutions and is charged to
retained metadata; it does not recursively build the expanded expression.

See `constant-graph-work-results.txt`, `v2-native-results.txt`, and the versioned
pure-memory/ordinary planning probe sources. v1 acceptance was paused until this
change was frozen and reverified.

## Final executed checks

All commands used Go 1.27.2, GOTOOLCHAIN=local, GOPROXY=off, GOSUMDB=off,
GOFLAGS=-buildvcs=false, the shared sibling go-cache/go-path, and only the pinned
official SQLite 3.54.0 CLI as a test oracle. Normal checks used CGO_ENABLED=0;
the separate race check used CGO_ENABLED=1. Every suite invocation was preceded
by `go run ./internal/testscope`. Exact invocations are in `commands.txt`.

- Full `internal/sql` normal suite: PASS.
- Focused sqlite/driver Boolean, relational, ordinary-index and expression
  suites: PASS (`v2-normal-focused-results.txt`, exit 0).
- Five independent typed/budget, driver, function-argument and substitution-work
  probes: PASS (`v2-native-results.txt`, exit 0).
- Those independent probes under race: PASS
  (`v2-independent-race-results.txt`, exit 0).
- Expanded independent differential run: FAIL, exit 1, intentionally preserved.
  5,332 context/error/value cases have five known boundary mismatches; 2,160
  affinity/value cases have 36 inherited mismatches. Do not report this as an
  all-pass oracle run.
- Exact durable baseline with the same corrected affinity harness: 44
  mismatches. Every candidate affinity mismatch is in that baseline set, and
  eight baseline mismatches are resolved. No new affinity mismatch was found.

The context matrix independently exercises projection versus WHERE/ON/searched
CASE, simple CASE, three-valued NULL logic and branch polarity through NOT,
IS TRUE/FALSE, BETWEEN and IN, scalar casts/comparisons/functions, literal versus
physical operands, aliases, physical-name precedence, parse-fold validation
exceptions, LEFT null extension/empty sources, index versus forced-scan residual
ordering, multiple INNER/LEFT ON constant ordering, and literal IS NULL metadata.
Error-kind comparisons were added to the final matrix. Values and storage
classes are compared, not merely row counts.

Native probes additionally cover legal typed values passed to the wrong scalar
family, lazy rejection versus eager projection, function-call order, NULL/CASE
argument behavior, invalid names/arity and unbound parameters despite LIMIT 0,
pre-canceled and mid-stream contexts, work/memory limits, zero retained bytes
after close, and normal reuse after errors. The public database/sql driver is
checked with repeated prepared parameter bindings, normal transaction rollback,
cancellation, ordinary close and reopen.

## Explicit remaining differences

The five context mismatches are retained verbatim in
`differential-classification.json` and the failing logs:

1. Two unquoted `AS true` / `AS false` alias grammar boundaries.
2. Two index/planner-reordering cases where a later CAST equality avoids an
   earlier invalid ESCAPE in SQLite but the conservative native plan scans.
3. One LEFT JOIN case where SQLite pushes a rejecting left-only WHERE term
   before an error-producing ON, while the native pipeline keeps ON before
   WHERE.

The 36 affinity differences are inherited, separately tracked wrong-result
behavior. A minimal intact fixture isolates them without constant propagation:

    CREATE TABLE f(t TEXT,b BLOB);
    INSERT INTO f VALUES(1.0,1.0);
    SELECT t,b,typeof(t),typeof(b),CAST(b AS TEXT),t=b,b=t FROM f;

Both engines report the same first five values: text '1.0', real 1.0, 'text',
'real', and text '1.0'. Native reports the last two comparisons as 1,1; SQLite
reports 0,0. Standalone WHERE t=b and b=t also differ on both the durable baseline
and v2. Applying unary + to b removes its affinity and both engines agree.

Pinned `src/expr.c:350`, sqlite3CompareAffinity, chooses no conversion when both
operands have nonnumeric declared affinities. The existing native comparison
helper instead applies TEXT conversion to a BLOB-affinity operand opposite TEXT.
This is a comparison-affinity selection defect, not a float-formatting finding.
The complete baseline/v2 diagnostic values and VDBE listings are in
`baseline-affinity-isolation.txt` and `v2-affinity-isolation.txt`. A separate owner
has the complete matrix for the follow-on correction; it was not silently folded
into this Boolean acceptance.

## Source and assertion review

Reviewed the applicable goal and relational eager-boundary documentation and
pinned expr.c/parse.y/where.c/select.c implementations for Boolean simplification,
branch NULL polarity, BETWEEN, constant startup terms, fixed-column substitutions,
affinity and LEFT-ON exclusions. The candidate preserves ordinary scalar
argument eagerness, unknown custom-function nonconstancy, bind-before-runtime
validation except the verified function-free parse folds, predicate-local alias
identities, typed representation boundaries, snapshot ownership and conservative
ordinary-index admission.

The only changed existing test is an oracle-backed correction of
`id=9999 AND k LIKE 'x' ESCAPE 'ab'`: it now explicitly checks zero rows, no error,
normal Close and unchanged SCAN admission. The original query remains, while
constant-overflow and typed-error assertions remain unchanged.
`v1-existing-test-changes.patch` records the exact change. This is not blanket
AND/OR laziness or removal of a failing historical case.

## Behavioral safety audit and evidence corrections

Reviewer tests use pure-memory AST/custom-function/quota diagnostics and intact
SQL-created ordinary databases. Test-only CLI processes are read-only queries
or :memory: checks, use the pinned binary and finish normally. Helpers
confOracle/confQuoted/writeExec were inspected; they check the oracle pin, parse
quoted output and execute ordinary transactions. Native typed failures pass a
valid HALFVEC to a VECTOR scalar; they do not write malformed cells. Normal
rollback, cancellation, Close and reopen are the only lifecycle operations.
Selected inherited helpers likewise use intact fixtures or valid in-memory
records/pages. No manual tags, tampering, corruption, wrong keys, missing-file
probes, namespace replacement, crash/kill/process-exit, hot recovery or native
I/O/lock/unmap/close/Sync failure injection was run.

Two reviewer-harness mistakes are explicitly preserved:

- Initial native assertions expected `0 AND missing` to bind. The pinned CLI
  proves function-free false-AND folding happens before binding. The assertion
  was corrected to `0.0 AND missing`; original sources/logs remain.
- The first affinity harness replaced the C in CASE while substituting its
  column placeholder. The corrected harness uses an explicit placeholder for
  that expression. Both candidate and baseline were rerun with the corrected
  harness; their 36-versus-44 classification is unchanged.

The diagnostic affinity-isolation test logs outcomes and VDBE instructions; its
exit 0 indicates successful evidence collection, not oracle equivalence. The
strict differential suite remains failed and preserves all wrong-result cases.
