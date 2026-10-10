# Native SQL leading secondary-index range handoff

Status: component source frozen and scoped checks PASS. Independent review and
lead full integrated gates are separate. No complete SQLite/milestone/production
claim. No main-tree edits, commits, uploads or changes to the baseline.

## Exact ownership

- Baseline: rowid-identifier-lead-verify, lead Git tree
  284402d1d8acba2252c85cc3d52c353cc4ed920b.
- Immutable source: sql-secondary-range-freeze-v1 (1,229 files).
- Full inventory: sql-secondary-range-freeze-v1-sha256.txt, SHA256
  a151a5c69a5e0bd102ba156a474991470bffffc1a1d008f898fa8d26d5f0df9e.
- Nine-path owned inventory: sql-secondary-range-owned-v1-sha256.txt, SHA256
  913106a54f88459f2e38722c9dbae663d2aa799018cf241448538aa9ae8643be.
- Patch: sql-secondary-range-v1-source.patch, SHA256
  180e9226e6839c8498f0b1ab1405c1cffe6e743bf8fffadf00ae30c175e04752.
- Ownership/base-final hashes are also in the immutable tree's
  docs/evidence/sql-secondary-range/owned-paths.txt and owned-base-final.tsv.
- Patch applied to a fresh exact-baseline sibling with exit 0. All nine owned
  paths match the frozen hashes; 1,206 existing unowned files are byte-identical
  in both frozen and patch-apply trees. Source inventory stayed unchanged through
  exact-freeze tests, build and vet. The lead's rowid-reason repair is excluded.

Owned paths: docs/progress.md, docs/sql-secondary-ranges.md,
driver/secondary_range_test.go,
internal/testkit/secondary_range_conformance_test.go,
sqlite/plan_index_test.go, sqlite/plan_secondary_range.go,
sqlite/plan_secondary_range_internal_test.go,
sqlite/plan_secondary_range_test.go, sqlite/query.go.

Production changes are only the new planner file and one hook after the existing
rowid planner. Public structures, B-tree cursor bytes, schema/writer/storage and
existing equality/rowid implementation bytes are unchanged.

## Semantics and limits

The path genuinely seeks a supported nonunique BINARY ASC index's first field
for direct <, <=, > or >= against literal/parameter/unary signed bounds on either
side of AND. It intersects comparison-affinity-correct whole-field cuts, preserves
int64/REAL precision and TEXT/NUL/BLOB storage classes, makes NULL comparisons
empty, and excludes physical NULL from upper-only ranges. The full WHERE remains
a recheck. Only existing total/error-safe WHERE/default shapes are admitted.

Equality, rowid including order-only rowid traversal, grams, ANN and vector rank
retain precedence. NOT INDEXED is still an actual full table source. Candidate
rowids are bounded, eagerly collected and signed-rowid sorted before base seeks;
sorting is never claimed eliminated. LIMIT 0/EXPLAIN do no new range work, while
normal schema/planning reads and prebinding remain. Prior rejection reasons stay
intact; attempted budget fallback appends its diagnostic and keeps actual work
counters. Work/endpoint/record/page admissions and upstream hooks run before
copies/allocations; ordinary cancellation is propagated. No snapshot owner or
public layout is added.

Plan.Method is btree_range, Access SEARCH, version 1; estimates are unknown -1
or proved-empty 0. IndexKeysRead counts matching returned records, IndexPageReads
counts logical range calls including probes, and source stats retain their normal
meaning. Schema/planner and base-row seek page calls are not in index counters.

Mandatory later code: equality-prefix plus next-key bounds, richer extraction,
secondary reverse traversal and full suffix/tie/rowid/NULL ordering proof,
covering, JOIN probes, statistics/cost selection and broader compatibility.

## Commands and actual results

Environment: sibling toolchains/go1.27.2/bin, GOTOOLCHAIN=local, GOPROXY=off,
GOSUMDB=off, GOFLAGS=-buildvcs=false, sibling go-cache/go-path, absolute official
SQLITE_ORACLE=toolchains/sqlite-build-3540000/sqlite3. Oracle identity is 3.54.0,
source 2026-10-09 15:46:58
be8d059e9a49089ab2dce5ed26dd87aaf598fdc5fbe0b107c0dd758464bcd5e3.
Every selected suite/build/vet was behavior-audited and preceded by
`go run ./internal/testscope`. No manual physical-risk tags or broad full-suite
runs were used. Default build/vet CGO=0; race CGO=1 only for Go's race runtime.

The exact-source runner is frozen at
`docs/evidence/sql-secondary-range/run-checked.sh`. It ran from the work tree,
then every owned byte was verified equal to the immutable tree. Terminal output
is external to the freeze at sql-secondary-range-v1-exact-source-checks.txt
(SHA256 d27bcdc090f1553c5abc6968d2410d02bbc15193d85ac90ba2d5283c0d1f9d78),
with exit 0 and completion 2026-10-10T19:57:41Z:

- Selected new range + ordinary-index/rowid/Boolean regressions in sqlite,
  driver, internal/sql: 9.530s / 0.027s / 0.005s, PASS.
- Selected official new range + ordinary-index/rowid/identifier/alias oracle:
  internal/testkit 38.380s, PASS.
- New range unit race: sqlite 42.937s, driver 1.131s, PASS.
- New range official oracle race: internal/testkit 60.970s, PASS.
- Whole CGO=0 build ./... and vet ./...: PASS.

Additional checks actually executed from the exact immutable tree are in
sql-secondary-range-v1-freeze-checks.txt (SHA256
e5f68986321ba32c145ec22a064664dce7c131d6504e7f6d0db8df5a777643de),
exit 0, completion 2026-10-10T19:58:49Z:

- New range sqlite/driver tests: 13.859s / 0.019s, PASS.
- Official unsupported-peer/lifecycle cases: 0.183s, PASS.
- Whole CGO=0 build/vet and before/after full frozen inventory: PASS.

Core matrix: 1,120 native storage-class/affinity/operator/operand-side cases
against genuine forced scans, plus intersections/aliases/errors/lifecycle,
prepared-statement reuse, quotas/cancellation and concurrency. Official core:
1,062 exact values/types/order/parameter/error cases per page size at
512/4096/65536 bytes, total 3,186 in ordinary and race. NUL text is compared
losslessly with hex plus original typeof, because CLI quote mode truncates it.

A scoped 1,800-row fixture reads 40 matching keys using 4 logical index pages
and 40 base rows versus 1,800 forced-scan base rows. Unordered LIMIT 7 OFFSET 3
still collects 40 index keys, then fetches 10 base rows versus 450 scan rows.
Independent index PageReader instrumentation reads 3 pages/4 keys versus
82 pages/1,600 keys from the same full index. These exclude schema/base-seek page
work from index counters and are not latency/universal optimization claims.

## Preserved failures and known baseline context

The immutable evidence retains the first oracle failure: the core matrix passed,
but the extra shared fixture attempted unsupported native read-only WAL reopen.
Only the fixture was corrected to use ordinary official PRAGMA journal_mode=DELETE
after checkpoint/close before read-only unsupported-peer-index checks.

The first expanded regression gate also failed the previous k>10 SCAN assertion.
Only that exact newly implemented shape now requires SEARCH/btree_range/by_prefix,
one returned index key, positive index-page work and one base seek, while keeping
its original full-value forced-scan comparison. All other unsupported-shape
assertions are unchanged. The original failed transcripts remain intact.

An administrative baseline-manifest comparison was first run in the modified
work tree and reported its three expected owned modifications. Its transcript is
secondary-range-base-verification-in-wrong-cwd.txt. The actual unchanged baseline
was then fully verified in its own directory (secondary-range-baseline-verified.txt).
This was neither a database experiment nor a baseline source mutation.

Lead separately identified inherited order-only rowid fallback-reason regressions
and owns rowid-reason-repair.patch. This nine-path delta does not incorporate or
relabel that unrelated repair. Full integrated acceptance is the lead's gate.

No physical corruption/tamper/wrong-key/missing-file/namespace mutation,
crash/kill/process-exit/hot-recovery, native lock/unmap/close/Sync/short-write/
disk-full/permission failure, interrupted publication or rekey experiment was
executed or claimed by this component. No production external SQL or CGO was
introduced. Independent reviewer findings belong to that separate review packet.
