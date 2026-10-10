# Independent review: Boolean/ranking predicate repair v1

## Decision

**No blocking finding in the exact six-path repair. Accept this narrowly scoped
candidate for integration, subject to the lead's integrated-tree verification.**
This is not a full-repository acceptance, full SQLite compatibility claim, physical
failure-safety certification, ANN-recall certification, or non-Linux validation.

The original three failing tests now pass unchanged. Both the owner's six-group
negative control and this review's independent negative control establish that
relevant assertions fail against the unmodified baseline production. The repair
has no production changes beyond the four files described below and adds exactly
two owner test files. No existing test was changed or weakened.

## Immutable identity and scope

- Main repository HEAD observed: ed4a38e4738a6906027a87cf42dc9f9ac84bc32b.
- Baseline snapshot: ../boolean-affinity-lead-verify, tree
  1336a5d7b5e1a336fb01e012334cec776a61c4a7; all 1046 files verified.
- Candidate snapshot: ../boolean-rank-filter-freeze-v1; all 1048 files verified.
- Candidate full-file manifest SHA256:
  c6a97ba00dbc2ed114ed5979aeb0317fabd203278c09cf6c33854e3fb2e993ff.
- Owned manifest SHA256:
  7683d73e4ac5a4ee726c7c871f4f72613fc34f70b3f25e1e3a129ccd59f82790.
- Repair patch SHA256:
  1121baa6b58ef3388e96c8c1b667c7fb9c0b4743766ae00b3b5c77d41638b13e.

The complete baseline/candidate inventories differ on exactly the six paths in
owner-owned-manifest.json. All baseline/candidate inventories and hashes are
unchanged after review. Original lead failed-run bytes remain unchanged. The
independent candidate and baseline copies contain their original snapshot bytes
plus only sqlite/boolean_rank_independent_test.go. Source integrity statements
and full before/after inventories accompany this report.

No production source, Git index, commit, remote or upload was changed by this
review. All main-repository writes are under docs/evidence/boolean-rank-independent/.

## Findings and source reasoning

Line references in this section identify the immutable candidate.

1. **Terminal startup state is correctly retained.**
   sqlite/predicate.go:106–137 checks operation cancellation before cached state,
   and a named return plus deferred assignment caches success, io.EOF, compiler
   failure or runtime startup failure. A repeated false predicate cannot become
   true, and a repeated arithmetic failure cannot become success. The lazy
   prepareWhere call at 119–123 makes directly constructed Rows use the same
   predicate compiler. Independent descriptor-free probes verify repeated false
   and overflow results, cancellation after cached terminal results, cached
   compiler quota failure and no source fetch before startup.

2. **Every relevant execution entry reaches startup before row evaluation.**
   sqlite/rows.go:190–193 preserves the streaming/general-sort gate;
   rows.go:223–229 adds the direct source-row gate before default/typed value
   interpretation, candidate accounting and projection. This repairs directly
   constructed memory Rows and ANN source rechecks. Alias compilation retains
   physical-column precedence and predicate context. Independent cases cover
   ordinary streaming, full sort, DESC, secondary keys, unbounded sort, exact
   heap, ANN, a constant-false alias and a physical-column-shadowing alias.

3. **ANN and representation scratch cannot bypass constant WHERE startup.**
   sqlite/plan_vector.go:275–290 gates before ANN materialization, query decode,
   graph traversal or half-vector scratch admission. False startup closes the
   existing source normally and clears it, without vector work. The existing
   fallbackANNVector path (plan_ann.go:84–102) re-enters this gate safely.
   Owner tests exercise dense/half/sparse/bit HNSW; independent tests additionally
   exercise half/bit IVF Lists 1, 2 and 4, covering all-list exact fallback,
   trained selection and untrained exact fallback. SQL false predicates suppress
   invalid query decode; error-first predicates retain their arithmetic error.
   No planner admission, index selection, ANN algorithm or recall change exists.

4. **Public filters use predicate semantics without moving candidate charging.**
   sqlite/similarity_search.go:445–461 still binds/validates names, functions and
   arguments upfront, retains snapshot-owned gram functions, then prepares the
   shared predicate. Lines 463–491 start constants at the first candidate and
   evaluate residual terms with original column affinity/typed interpretation.
   Existing callers in vector_search.go, half_search.go, sparse_search.go,
   bit_search.go and similarity_search.go are byte-identical. In particular,
   vector_search.go:301–353 still counts fetched candidates before filtering;
   WHERE 0 cannot evade the candidate quota. The owner regression proves this.
   Independent probes cover all five public search families in exact/approximate
   (or full-scan/posting) modes, name/arity errors, empty source, NULL query,
   zero limit, public eager invalid-query rejection, and TEXT/BLOB versus
   affinity-less expression comparisons. Existing timing/affinity probes also
   pass unchanged against baseline, making their compatibility role explicit.

5. **Prior Boolean, affinity, function and budget contracts remain intact.**
   prepareWhere and predicate evaluation retain their existing alias cloning,
   fixed-column map, BLOB-affinity exclusion and top-level AND term ordering.
   The rest of internal/sql and predicate_constants.go is byte-identical.
   Independent memory probes confirm context-supplied abs is not hoisted and
   executes once per qualifying source-row evaluation. Existing selected Boolean
   tests retain lazy typed cells, name/arity validation, overflow order, NULL
   truth, custom functions, context cancellation and logical quotas. LIMIT 0 and
   EXPLAIN skip runtime startup as before (query.go:277–297), which independent
   SQL probes also verify. No exported API signature/field or persistent format
   changes were made; Rows gains only the necessary private error state.

## Important retained boundaries

- Public search startup is intentionally lazy until a candidate exists. An
  empty public search, NULL query or zero limit does not execute a runtime WHERE
  overflow, although filter syntax/name/arity/parameter validation still occurs.
  This differs from SQL WHERE startup on an empty table and is preserved public
  API timing, not an assertion that every public API is SQL-identical.
- The SQL gate precedes execution work, not all planning metadata access. IVF
  eligibility/state metadata may already have been read by prepareANNVector.
  Zero graph/centroid/member/scoring work does not mean zero total file reads.
- Native public query-input validation still precedes candidate filtering;
  WHERE 0 does not suppress an invalid vector query there.
- Existing conservative ordinary-index admission, JOIN/WHERE pushdown and
  source-order-dependent error boundaries in boolean-expressions.md remain.
  This patch does not implement complete SQLite planner/error-order equivalence.
- Only successful normal source Close behavior was executed. No failed native
  close/unmap/I/O/lock/Sync or other prohibited fault path was tested.

## Independently executed evidence

Environment: Go 1.27.2 linux/amd64, GOTOOLCHAIN=local, GOPROXY=off, GOSUMDB=off,
GOFLAGS=-buildvcs=false and shared pinned caches. Ordinary tests use CGO_ENABLED=0;
only the explicit Go race instrumentation uses CGO_ENABLED=1. Official SQLite
3.54.0 source ID be8d059e9a49089ab2dce5ed26dd87aaf598fdc5fbe0b107c0dd758464bcd5e3
is test-only. No new production SQL subprocess or CGO dependency exists.

verify-selected.sh and verify-selected.txt preserve exact commands and outputs:

- Selected sqlite/internal/sql regressions: PASS. Includes unchanged original
  TestSparseReviewMemoryLazyNullAndSupportBudgets,
  TestSparseReviewMemoryExternalDefaultsAndAliases,
  TestANNReviewEmptyIndexAndAllFilteredSQL; all six owner groups; existing
  Boolean tests; and four independent groups described above.
- TestBooleanRankFilterOracle: PASS, 21 scalar predicates through four native
  paths, 84 comparisons with pinned SQLite. It compares scalar filters and tied
  rowid ordering, not SQLite vector math.
- Selected race over all four independent groups: PASS.
- Independent baseline negative control: expected exit 1. Unplanned startup
  fails; trained half/bit IVF fails to surface the preceding overflow. The
  public timing/affinity and execution-shape compatibility groups pass baseline.
  This is precisely two failing top-level groups, not all independent groups.

verify-selected.exit = 0; negative-independent.exit = 1. The initial independent
probe run also passed. The only subsequent probe edit removed a no-op loop;
final checks and the preserved probe source use the cleaned version.

Each test command was preceded by internal/testscope after behavioral source
review. scope-audit.md records the reviewed helpers/behavior and restrictions.
Independent execution is selected, not a fresh full go test ./... run.

## Owner evidence independently inspected, not rerun in full here

The owner finished verification against the exact immutable candidate:

- Full sqlite and full internal/sql ordinary package tests: PASS.
- Selected driver and extension/vector/... tests: PASS.
- go vet ./sqlite ./internal/sql ./driver and go build ./...: PASS.
- Selected tagged oracle packages internal/testkit, internal/sql and driver:
  PASS. Included historical affinity and expanded affinity matrices report zero
  differences, preserving the earlier Boolean/affinity contracts.
- Selected sqlite/internal/sql/driver/vector race: PASS, exit 0.
- Separately, full internal/sql race package: PASS.
- Owner's new six top-level groups on unchanged baseline production: expected
  FAIL in every group, exit 1. Original full baseline run failed the three
  unchanged tests listed above and remains retained with exit 1.

Their scripts, scope audit, result report, exact logs/exits, manifest and patch
are copied with owner- prefixes. Neither owner's selected checks nor this
review's selected checks should be relabeled as full repository validation.
The owner retains an initial incorrect new-test expectation (bounded_heap versus
actual ann_exact_rescore) in its historical evidence and documents its correction;
no historical assertion was weakened to obtain a pass.

## Design/source references read

Read goal.md and the Boolean, affinity, ANN, typed ANN, vector and ordinary
scope-audit guides, plus relevant inherited Boolean/relational audits. The pinned
public source confirms the applicable model: where.c:6950 splits WHERE into
AND terms; where.c:7007–7045 performs the row-independent false-WHERE bypass;
expr.c:6396–6470 handles predicate branches; expr.c:350–367 distinguishes declared
BLOB affinity; select.c:4931–4965 preserves fixed-column identity/affinity.
These references support the narrow review, not universal optimizer equivalence.

Final review artifacts are frozen by SHA256SUMS.txt. Future changes require a
new review version rather than rewriting this report or its evidence.
