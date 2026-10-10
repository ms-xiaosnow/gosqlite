# Independent leading secondary-index interval review

Status: **PASS for the exact narrow secondary-interval slice; no remaining slice-blocking finding identified.** This is not whole-baseline or combined-tree acceptance, general optimizer/SQLite parity, physical durability/security qualification, or production acceptance. All selected independent commands passed and post-run hashes match.

## Exact bytes and isolation

- Baseline tree: `284402d1d8acba2252c85cc3d52c353cc4ed920b`, supplied as `../rowid-identifier-lead-verify`.
- Candidate: `../sql-secondary-range-freeze-v1`; full inventory SHA-256 `a151a5c69a5e0bd102ba156a474991470bffffc1a1d008f898fa8d26d5f0df9e`.
- Nine-path source/doc/test patch SHA-256 `180e9226e6839c8498f0b1ab1405c1cffe6e743bf8fffadf00ae30c175e04752`; owned inventory SHA-256 `913106a54f88459f2e38722c9dbae663d2aa799018cf241448538aa9ae8643be`.
- Reviewed additive production planner SHA-256 `798ecff28bd2b34704aab3df3059a16324471f00ff6b2c0bf08bbfa925f7543f`.
- Exact tested copy: `../secondary-range-independent-final-v1`, differing only by the three reviewer test files saved here as sqlite_review_secondary_*.go.txt and internal_testkit_review_secondary_*.go.txt.
- Patch independently applies to the exact supplied baseline and reproduces all nine owned paths. The only full-tree differences are the candidate's 14 additive evidence files; see patch-reproduction.json.

The lead's separately reviewed rowid Reason repair is excluded from this component. See rowid-reason-review.md for its exact-tree acceptance. Integration must retain that separate repair. The reviewer made no main production edit, staging, commit, push, upload or cache cleanup.

## Independent coverage

The native expectation uses exact `math/big.Rat`, separate storage-class ranks and byte comparisons, never production comparator or interval helpers.

- 4,096 deterministic mixed-storage-class two-bound queries over 123 indexed rows, with 512 additional genuine NOT INDEXED comparisons. All four strict/inclusive operators and both operand sides, reversed AND bound order, contradictory/absent intervals, NULL/NaN, infinities, subnormal/negative zero, 2^53 and signed-int64/REAL edges, numeric and nonnumeric TEXT, empty/NUL/prefix/overflow TEXT/BLOB, duplicate index suffix groups, both signed rowid extrema on non-NULL keys, residual OR filters, mixed key orders, stable ties and LIMIT/OFFSET are exercised.
- 1,452 additional official-created rollback-fixture comparison cases, 484 at each of 512/4096/65536-byte pages. They check two-bound intersections, numeric/text/blob/no-coercion behavior across supported affinities, reversed operands, residual filters, exact values/types/hex, key-order ties, limits, Boolean aliases and physical-column alias shadowing. Each query also matches a native forced scan. File hashes remain unchanged; integrity_check passes.
- Whole-planner PageReader instrumentation includes catalog/root-cursor planning and base-row seeks, as well as separately reported range index calls. It verifies retained sorting, eager candidates, LIMIT 0/EXPLAIN, static empty bounds, immutable plans and complete fallback after an abandoned index attempt.
- Independent admission probes verify stricter upstream page/record quotas, post-hook cancellation, pre-cancel no-read, tiny workspace and candidate exhaustion, private maximum-capacity/workspace refusal, endpoint BLOB copying before caller mutation, combined endpoint Work admission before any page read, and sticky cursor EOF/Close. These are normal logical quotas/cancellation and descriptor-free endpoint data, not native failure injection.
- Shared-WAL snapshot captured before the first Next remains stable across another connection's update and insert; a subsequent query sees the new commit. Early Close and pre-Next cancellation retain normal lease behavior, followed by normal close/checkpoint.
- Reproduced owner selected coverage adds 1,120 affinity/storage-class/operand-side scan cases, 3,186 pinned official cases, duplicate groups, unsupported peer indexes, overflow, normal read/write/rollback/own-write/BUSY/close/reopen/checkpoint, shared snapshots, concurrency, database/sql named prepared binds and reuse, parser/name/bind errors, and equality/rowid/grams/HNSW/IVF/relational precedence.

## Actual whole-query work

Fixture: ordinary 512-byte pages, 4,096 rows, `t(id INTEGER PRIMARY KEY,k INTEGER,s INTEGER)`, `by_k ON t(k,s)`, row i has id=i-2048, k=i/2, s=4096-i.

For `SELECT id FROM t WHERE k>=2044 AND k<2046 LIMIT 2 OFFSET 1`:
- Range: 7 planning page calls + 9 subsequent base-seek calls = 16 total; 3 range IndexPageReads, 4 IndexKeysRead/Candidates, 3 RowIDSeeks/BaseRowFetches, 2 returned.
- Forced scan: 2 planning calls, 277 total calls, 4,091 BaseRowFetches, 2 returned.

Adding `ORDER BY k DESC,s ASC` retains the actual sorter:
- Range: 19 total page calls, all during planning/materialization; 3 range index calls, 4 candidates/base fetches, 2 returned.
- Forced scan: 277 total calls, 4,096 base fetches, 2 returned.

LIMIT 0 makes 4 planning calls and EXPLAIN 3; both make zero range/iteration calls. NULL and contradictory bounds each make 4 planning calls, no range/base calls. With MaxSortBytes=16 KiB and k>=0, abandoned range work reports 3 index calls/22 emitted index records, then the complete scan fetches all 4,096 rows: 282 total calls, zero admitted Candidates/RowIDSeeks. These are logical PageReader calls on this fixture, not physical I/O, latency or universal performance claims. The owner's narrower 3-versus-82 direct-index measurement refers to a different 1,600-key fixture.

## Preserved failures and limits

1. Baseline negative: the unchanged baseline correctly has SCAN for the first new inequality case, so the independent required-btree_range assertion fails. Values still match. baseline-negative.log/.exit retain it.
2. Owner's first oracle core matrix passed but its extra fixture attempted unsupported native read-only WAL open. The original failure is retained in the exact freeze. The corrected fixture uses ordinary official journal_mode=DELETE after normal owner close before read-only peer-index testing. No production workaround was introduced.
3. Owner's old single k>10 SCAN assertion failed as expected after implementing that shape. The replacement preserves forced-scan value checks and strengthens required real work, without weakening other assertions.
4. The initial independent oracle fixture used unsupported SQL type spellings (STRING/VARCHAR/DOUBLE/FLOATING POINT). It was rejected at schema parsing before range execution. The preserved initial failure and source are distinct from the final fixture, which uses the documented supported declarations. Arbitrary type spellings remain a pre-existing unsupported scope.
5. The next independent oracle attempt exposed the official shell quote format truncating NUL TEXT in two direct-column alias projections. Main matrix values already use lossless hex plus typeof. Alias probes now retain their original binding purpose with bounds excluding the NUL-valued projection. Both unsuccessful harness transcripts and full sources are retained; no production behavior or test expectation was loosened to hide a candidate mismatch.
6. The final private capacity guard is independently tested red against the initial planner: a direct helper call with capacity 100,001 and 64 MiB workspace formerly returned the three results; final source rejects before reads. This hardens a private precondition; the high-level planner already capped capacity at 100,000, so it is not evidence of an exposed SQL over-allocation bug.
7. The preliminary working-tree copy was made while owner checked-v2.txt was still growing, so that one evidence log differs from a source manifest captured just after copying. All production/test/doc bytes matched. Final qualification uses the separately verified immutable freeze; see initial-integrity.json.

No physical corruption/tamper/wrong-key/missing-selected-file, namespace mutation, crash/kill/forced-exit/hot recovery, native I/O/lock/close/unmap/Sync/shortwrite/diskfull/permission fault, interrupted publication/rekey or manual-tag experiment was executed. Source-only review does not qualify native fault behavior. Full combined-tree gates, broad milestones and production qualification remain the lead's separate responsibility.

## Final commands and provenance

Go 1.27.2 Linux amd64; GOTOOLCHAIN=local, GOPROXY=off, GOSUMDB=off, GOFLAGS=-buildvcs=false. Only the supplied shared sibling go-cache/go-path are used. Pinned SQLite CLI SHA-256 is 470775f6ada29ccfb2d3046ac2b4c975df95a9c20b782332d4e8ba6b6233e6c6, SQLite 3.54.0, source ID 2026-10-09 15:46:58 be8d059e9a49089ab2dce5ed26dd87aaf598fdc5fbe0b107c0dd758464bcd5e3. The oracle-tagged helpers verify that identity; production never invokes the CLI.

Every following command exited 0, with the source-reference guard immediately preceding it and the behavioral audit completed. Each expanded command, guard output, stdout/stderr and exit is preserved under its listed stem:

1. `CGO_ENABLED=0 go test ./sqlite ./driver -run '^(TestReviewSecondary|TestSecondaryRange|TestDriverSecondaryRange|TestOrdinaryIndex|TestRowIDRange|TestDriverRowIDRange)' -count=1 -v`: PASS, sqlite 45.656s, driver 0.021s; final-native.*.
2. `CGO_ENABLED=1 go test -race ./sqlite ./driver -run '^(TestReviewSecondary|TestSecondaryRange|TestDriverSecondaryRange)' -count=1 -v`: PASS, sqlite 77.273s, driver 1.049s; final-native-race.*. CGO is used only for race instrumentation.
3. `CGO_ENABLED=0 go test -tags oracle ./internal/testkit -run '^Test(ReviewSecondaryOfficialFixtures|OracleSecondaryRange)' -count=1 -v`: PASS, 4,638 ordered comparison cases plus two lifecycle fixture instances, 71.758s; final-oracle.*.
4. `CGO_ENABLED=1 go test -race -tags oracle ./internal/testkit -run '^TestReviewSecondaryOfficialFixtures$' -count=1 -v`: PASS, 1,452 additional independent cases, 25.421s; final-oracle-race.*. Owner range oracle-race passed separately and is not relabeled as this review’s own race measurement.
5. `CGO_ENABLED=0 go build ./...`: PASS; final-build.*.
6. `CGO_ENABLED=0 go vet ./sqlite ./driver ./internal/btree`: PASS; final-vet.*.
7. `CGO_ENABLED=0 go vet -tags oracle ./internal/testkit`: PASS; final-oracle-vet.*.

No final selected test failed or skipped. The supplied baseline is independently verified against all 1,209 Git blobs of tree 284402d, not merely against a working-tree label. Every one of the candidate’s 1,229 original file hashes matches before/after testing, both in the untouched freeze and tested copy. The tested copy contains exactly the three declared added reviewer files. See exact-git-tree-verification.json, final-manifest-before.json and final-manifest-after.json. All earlier native source bytes and baseline originals remain unchanged; the one growing provisional evidence-log exception is documented above.

The owner’s terminal exact-source/freeze checks and handoff are copied with owner- prefixes. They supplement, rather than replace, these independent results. run-command.sh.txt records the environment/guard wrapper; saved test sources reproduce the added checks in a fresh exact candidate copy. No test/compiler process belonging to this review remained at final handoff.
