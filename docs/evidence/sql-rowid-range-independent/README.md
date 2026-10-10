# Independent SQL rowid interval and reverse-order review

Status: **PASS for this narrow candidate slice; no new slice-blocking finding identified.** The candidate performs actual bounded native B-tree traversal and safely elides sorting for the admitted physical-rowid order. This is not universal SQLite semantics, whole-baseline acceptance, combined-tree acceptance, physical durability qualification, or secondary-index range/covering/JOIN acceptance.

## Findings first

1. **Keep the existing projection-error compatibility gap visible.** On both unchanged `affinity-substitution-freeze-v1` and this candidate, `CREATE TABLE t(id INTEGER PRIMARY KEY); INSERT INTO t VALUES(-9223372036854775808),(-1),(1); SELECT abs(id) FROM t WHERE id<0 ORDER BY id DESC LIMIT 1;` reports native `integer overflow`, while pinned SQLite returns integer `1`. The candidate deliberately rejects order elision for the error-capable projection at `sqlite/plan_rowid_range.go:135–138`; the unchanged eager materializer at `sqlite/rows.go:420–438`, invoked from `sqlite/query.go:288–294`, reaches the overflowing row. This is a reproduced baseline code gap, not a failing newly admitted range/order shape. Exact runs: `known-gaps-baseline.log` and `known-gaps-copy.log`.
2. **The immutable older baseline rejects unquoted `AS true` / `AS false`.** The first independent oracle batch exposed this known scope boundary. Both baseline and candidate reject these aliases in `internal/sql/parser.go:176–189`; official SQLite accepts them. The separately owned Boolean identifier work is outside this candidate. Quoted alias declarations with unquoted references pass the final range oracle. Do not describe this candidate as including later identifier or Boolean-rank repairs.
3. **Lazy range execution is not zero total query I/O.** The existing `planSource` opens an ordinary root cursor before `planRowIDRange` replaces it (`sqlite/query.go:255,276`; `sqlite/plan_grams.go:223–227`; `sqlite/plan_rowid_range.go:188–194`). Whole-planner instrumentation measures **6 total logical page calls versus 829** for a complete forced-scan/sort query, including the candidate's 3 planning calls and 3 subsequent range calls. LIMIT 0 performs 3 planning calls and EXPLAIN 2, with zero iteration/range calls. The owner's 3-versus-828 direct-cursor measurement is true at that narrower boundary; neither number is physical disk I/O or latency. Existing documentation already reserves schema/planning reads.
4. **An initial reviewer reference-generation defect is retained, not concealed.** The first added oracle test substituted a negative parameter directly after unary `-`, accidentally producing SQL beginning `--922...`, a comment. That produced an official parse error. The corrected reviewer test uses actual `.parameter` bindings. `independent-oracle.log` / `.exit` (exit 1) and `review-rowid-oracle-initial.go.txt` retain both this defect and the unquoted-alias failures. No candidate production behavior or assertion was weakened.

## Exact candidate and immutability

- Candidate: `../sql-rowid-range-freeze-v1`, 905 files.
- Baseline: `../affinity-substitution-freeze-v1`, 879 files, plus exactly the six additive bounded-cursor files listed by the owner.
- Candidate full-manifest SHA-256: `332b57d5424da4fefb8a2ce309a5aa90ed9c13789d936f1e119f9d86a703bb7b`.
- Source/doc patch SHA-256: `398badb387da9e451a6a17b98a69ace661d1add2de633c3e859580d4ceaf1ae9`.
- Test tree: `../sql-rowid-range-independent-copy-v1`. It differs from the candidate only by four reviewer test files. Their complete source and hashes are saved here and enumerated in `manifest-after.json`.
- Baseline-gap test tree: `../sql-rowid-range-independent-baseline-v1`, differing from the unchanged baseline only by the one gap-probe test.
- Independent patch reproduction: `../sql-rowid-range-independent-patch-v1`. The owner's patch applied cleanly to the exact baseline, yielding all 21 owned non-evidence paths byte-for-byte and exactly the candidate tree minus its 14 new evidence files. The six cursor additions match their supplied hashes, with none present in the original baseline.

Full candidate and baseline manifests, original owned-path hashes, exact overlay membership, and all original bytes in both test copies were checked before/after. All remain unchanged. Evidence: `manifest-before.json`, `manifest-after.json`, `candidate-sha256.txt`, `baseline-sha256.txt`, and `patch-reproduction.*`. Existing public QueryPlan/QueryStats field layouts are unchanged: the plan/stats source diff adds comments and a Detail rendering branch, not fields.

No main production edit, staging, commit, upload or push was performed. Main-checkout writes are confined to this evidence directory. The main checkout's concurrent changes were not substituted for this immutable candidate.

## Independent correctness coverage

The independent numeric expectation uses `math/big.Rat`, direct signed-int64 ordering and explicit storage-class rules. It does not call production SQL/B-tree comparison or range-cut helpers.

- **4,096** deterministically selected SQL two-bound intersections over **191 distinct rowids**, including both signed-int64 extrema and neighbors, ±2^53 boundaries, subnormal REAL, negative zero, fractional REAL, values on either side of ±2^63, infinities, NaN/NULL, numeric TEXT, nonnumeric TEXT and BLOB. Directions, all four strict/inclusive operators, rowid/IPK/hidden aliases, residual filtering, LIMIT 0/positive/unbounded and OFFSET are varied. Every answer matches the independent reference; **512** also match genuine forced scans.
- **304** independently selected ORDER BY, physical-shadow, result-alias, positional/signed-position, tied non-rowid key, multi-term order, arithmetic, unsupported residual, LIMIT/OFFSET combinations match forced scans. Proven rowid order alone is admitted; other keys and unsafe/unproven shapes retain sorting or source fallback.
- **1,206 additional official SQLite comparisons**, 402 each at page sizes 512/4096/65536. These cover independent two-sided cuts, opposite-side operators, REAL/int64 edge literals, storage classes, aliases/qualification, ties, OR/unary-column fallback, negative OFFSET, safe arithmetic projection, and actual bound parameters with unary sign. Values, storage classes and result order match. Every intact database's before/after SHA-256 matches; read-only integrity_check passes.
- Additional ordinary shared-WAL snapshots are captured **before the first Next**, retained across another writer's commit, and checked in both directions. False-startup/empty bounds, context cancellation before the first seek, explicit DB/Rows close and checkpoint pass. Plans are identical before/during/after iteration and EOF; returned plan slices remain defensive copies.
- Descriptor-free source tests establish pre-cancel zero page/seek work; invalid limits refuse before page work; Work=1 reads one page then rejects parser allocation; quota/error/EOF remain sticky, and explicit close prevents reopening. The accepted low-level cursor prerequisite's more extensive allocation, bound ownership and post-hook cancellation evidence remains in `../gosqlite/docs/evidence/bounded-range-independent/README.md`; it was not relabeled as new SQL-only coverage.

The reviewer also reproduced the owner's **1,104** forced-scan storage-class/side/direction cases, exact rational bound law, strict/empty intersections, EXPLAIN/LIMIT-0 binding/name checks, error-order fallback and JOIN exclusion, ordinary transaction rollback/read-own-writes/active-Rows BUSY, snapshots/cancel/checkpoint/reopen, prepared driver binds and tiny-sort-budget behavior. Existing ordinary-index tests pass, including equality/grams/ANN precedence, prefix work, fallback accounting, concurrency and transaction cases.

## Actual work and diagnostics

For `SELECT id FROM samples WHERE id>=10 AND id<=12 ORDER BY id DESC LIMIT 2 OFFSET 1`, the independent whole-planner memory reader observes:

- Native range: 3 planning + 3 traversal = **6 page calls**, **3** Candidates/BaseRowFetches, **2** ReturnedRows, **1** RowIDSeek, **0** IndexPageReads/IndexKeysRead.
- Forced scan: **829 total page calls**, **1,028** BaseRowFetches and **2** ReturnedRows. Sorting/source traversal occurs during planning, as expected.

The reproduced larger ordinary SQL case fetches **5 source rows versus 1,800** for descending LIMIT 3 OFFSET 2. The direct-cursor fixture case reads **3 versus 828** page calls in either direction. These are separate, explicitly identified measurements.

`sqlite/plan_rowid_range.go:173–197` reports SEARCH only for physical/empty bounds; tautological nonnumeric inequalities or order-only traversal remain SCAN. Unknown cardinality remains -1, proved-empty 0. Direction/order/sort claims agree with actual traversal. Candidate counters increase only for yielded range rows; table work is not mislabeled index work (`:224–239`). Original WHERE remains in the evaluator, and the prior equality/grams/ANN paths keep precedence (`:147`, `sqlite/query.go:255–276`). No secondary inequality, covering read or JOIN pushdown claim is made.

## Commands and results

Toolchain: Go 1.27.2 Linux amd64. `GOTOOLCHAIN=local GOPROXY=off GOSUMDB=off GOFLAGS=-buildvcs=false`; only sibling go-cache/go-path used. SQLite test oracle SHA-256: `470775f6ada29ccfb2d3046ac2b4c975df95a9c20b782332d4e8ba6b6233e6c6`, verified against the accepted pinned identity. Tagged tests enforce SQLite 3.54.0 and source ID `2026-10-09 15:46:58 be8d059e9a49089ab2dce5ed26dd87aaf598fdc5fbe0b107c0dd758464bcd5e3`.

All final commands below exited 0, with the source-reference guard run immediately before each command. Exact expanded commands and all outputs are retained in matching `.log` / `.exit` files.

1. `CGO_ENABLED=0 go test ./sqlite ./driver -run '^(TestReviewRowIDRange|TestRowIDRange|TestDriverRowIDRange|TestOrdinaryIndex|TestStreamingMemoryCancellationAndClosure|TestConnectorAndContext)' -count=1 -v`: PASS; sqlite 6.655s, driver 0.010s (`final-native.*`).
2. Same selected suite under `CGO_ENABLED=1 go test -race`: PASS; sqlite 23.326s, driver 1.035s (`final-native-race.*`).
3. Independent official batch under `CGO_ENABLED=0 go test -tags oracle ./internal/testkit -run '^TestReviewRowIDRangeIndependentSQLite354$' -count=1 -v`: PASS; 1,206 comparisons, 5.128s (`independent-final-oracle.*`).
4. Owner official batch under `CGO_ENABLED=0 go test -tags oracle ./internal/testkit -run '^TestOracleRowIDRangeSQL$' -count=1 -v`: PASS; 1,062 comparisons, 4.452s (`owner-oracle.*`).
5. Both official batches under `CGO_ENABLED=1 go test -race -tags oracle ./internal/testkit -run '^Test(OracleRowIDRangeSQL|ReviewRowIDRangeIndependentSQLite354)$' -count=1 -v`: PASS; **2,268 comparisons**, 10.872s (`final-oracle-race.*`).
6. `CGO_ENABLED=0 go build ./...`: PASS (`final-build.*`).
7. `CGO_ENABLED=0 go vet ./sqlite ./driver ./internal/btree`: PASS (`final-vet.*`).
8. `CGO_ENABLED=0 go vet -tags oracle ./internal/testkit`: PASS (`final-oracle-vet.*`).
9. Baseline-gap probe in baseline and candidate copies: PASS as an assertion of the documented existing differences (`known-gaps-{baseline,copy}.*`). This is not a baseline compatibility PASS.

No skipped final test or remaining running compiler/test process is part of this report. Saved `.go.txt` files can be copied to the paths encoded in their names inside a fresh candidate copy; `run-review.sh.txt` records the exact final batch. The known-gap oracle and separate pure-Go official batches are reproduced by their recorded log commands. The behavioral audit is in `behavioral-audit.md`.

## Remaining scope

Parent combined-tree validation is still required after integrating independent Boolean/provenance/pager/aggregate work. General error-order parity, secondary-index intervals and ordering, covering scans, JOIN pushdown, planner statistics/completeness, non-Linux execution and full M3/M4/M5/M10 qualification remain open. This review neither executes nor qualifies physical corruption/tamper/wrong-key/missing-file, namespace mutation, native failure injection, crash/kill/process-exit/hot recovery, interrupted publication or rekey. No production CGO or external SQL execution dependency was introduced.
