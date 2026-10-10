# Independent transaction/soak harness V4 review

## Scoped verdict

**No remaining blocking harness finding in the reviewed V4 ordinary-workload/reporting scope.** The original joined-error and pending-retirement regressions are corrected without weakening acceptance or invoking exceptional native paths. Exact source identity, patch replay, original probes, focused normal/race/vet/build gates and independent raw-history reconstruction pass.

This is not acceptance of a long soak, native failure/recovery behavior, controlled performance confidence, all-platform runtime behavior, whole-project integration, or final publication/CI identity. V1/V2/V3 findings and failed workload samples remain preserved failures.

## Exact identity and replay

- Immutable harness: `../transaction-soak-freeze-v4`, tree `65f0d9bece6732499899db84f6d4fc008b6810e3`.
- Baseline: `../plain-final-verify`, tree `7d3184f87165e11e12560884ec6e4f92c9bfd039`.
- Full patch: `../transaction-soak-evidence-v4/source-v4.patch`, SHA256 `1e5d86e5b5f1643f3e20245048deda7c068630fade57d1d5f36d90e3ca33df4f`.
- V3 delta: `../transaction-soak-evidence-v4/v3-to-v4.patch`, SHA256 `e96e1ed7d518d56a51c9340604be3f416bbe4e4d06469d69a7bb453d09f1cd71`.
- Runtime-only accepted-reader-fix composition: `../transaction-soak-fixed-freeze-v4`, tree `73d79d0e5cacf82c55777ad9756214a1859e6e1b`.

Independent Git-object/tree construction, SHA256 and exact filesystem/Git modes verified every regular file. All1252 baseline files are unchanged; exactly12 benchmark files are added (1264 harness files). The1266-file runtime composition has all12 benchmark files identical to the harness freeze. The reader repair is a separate engine change, not part of the harness patch.

A fresh full baseline replay and separate V3 delta replay both reconstruct the exact V4 harness tree with matching complete inventories. Runtime execution used a new exact composition copy. All six tree inventories were rechecked after execution with no file/mode changes. Evidence: `identity.json`, six `*-inventory.json` files, `post-run-source-verification.json`, independent script `../transaction_soak_independent_identity_v4.py`.

## Original regressions preserved and passing on V4

Read-only gofmt normalization proves the frozen tests are exactly the original seven mixed-error cases and four public-pending-retirement cases. Neither original source nor its failed log was replaced.

- Original seven-case source SHA256: `5d9cbfcce23c82351cdf74330b7325a40e30df5e2d348c497e95013839f8255c`.
- Original four-case source SHA256: `519541f9c255f8eebe6c2f7457a7c7c34015fe0152b7ecf5a3ad20f55c964ada`.
- Original four-case failed log SHA256: `7d679b9592e49346f42fb866ffe6668b75fa2c985b083e68afda84700d6f3173`.
- Both historical negative.exit files remain1. All11 cases pass in the V4 normal and race executions.

Evidence: `original-probe-verification.json`, `normal.txt`, `race.txt`. Historical controls remain in the separate V2/V3 independent-review directories.

## Source correctness assessment

### Strict outcomes

The V3 bounded all-branch classifier remains unchanged. BUSY/cancellation admission inspects every relevant branch and SQLite envelope within128 nodes, and accepts only exact known native/context leaves through recognized standard wrappers. Unknown diagnostics, unsafe/inconsistent codes, custom wrappers, nil/malformed graphs and excessive/cyclic graphs fail closed. No custom Is, As, Error or Unwrap callback is executed by classification. ordinaryBusy and classify use the same decision; known mixed cancellation/BUSY never enters commit retry. The original ErrChanged failure cannot be downgraded into expected BUSY.

### Pending ownership and truthful closure

V4 cleanup discovery recognizes all four exact public pending markers and conservatively flags CodeIOErr, poison/commit uncertainty, unknown ownership envelopes and malformed/excessive graphs. It does not need to execute an opaque token's custom methods. recordFailure stores the original error/token graph, safely renders a bounded diagnostic, and immediately latches failed status plus close_error_unverified_retirement.

This now covers an error already stored in Rows.Err even when a later Rows.Close returns nil. It also covers a failing Open that returns no DB handle: the ledger/finalizer is installed before the first Open, and original diagnostics live independently of DB references. Empty benign ownership uses no_database_handle_returned; it cannot masquerade as completed native retirement.

recordOpenFailure permits skipped_unsupported only for an entirely understood benign unsupported graph. An extra joined diagnostic remains failed. A pending/IO/opaque diagnostic latches unsafe retirement, and setReportStatus prevents later pass, rejection or skip from erasing it. Finalization resets the shutdown summary and then reapplies that latch. The actual call paths and AST ordering were reviewed, not just the helper tests.

Every acquired writer/reader is registered in the run-owned ledger. The single owner releases a handle only after that exact Close succeeds. Prior failed handles and their original diagnostics remain retained; finalization closes only unattempted handles. No blind second Close, inferred retirement from a later nil, automatic cleanup retry or global sweep exists. The ledger is used only outside running worker phases, after workers join.

Report retains the DB ledger and original token graphs through JSON emission via private fields and runtime.KeepAlive. **Holding references through emission and ordinary process completion are not native retirement proof.** The source and README explicitly preserve this lead-approved limitation; terminal native uncertainty is neither recovered nor claimed clean. No exceptional native workflow was executed to validate it.

### Model, timing, bounds and reporting

The immutable per-generation model and deterministic seed/ordinal transfer math remain unchanged from the earlier independently reviewed harness. Commit/model publication is synchronized; readers check exact IDs/values twice in one snapshot; rollback/savepoint histories have exact overlays; pruning follows joined workers and verified ordinary reopen. Histograms, model history, row count, workers, epochs, traces and declared storage/memory checks remain bounded.

Actual accumulated concurrent epoch time, including cooperative drain, is distinct from bootstrap/maintenance/end-to-end time. Quota, memory or epoch exhaustion cannot satisfy requested soak duration. Counts and C/R/B/X/F traces distinguish commits, intentional rollback, retries, cancellations and failures. Latency quantiles remain explicit bucket upper bounds. Correlated unequal-duration window rates are descriptive and confidence bounds remain null.

Every report now carries harness_revision=ordinary-transactions-v4. Consumers must require that exact corrected revision plus status, closure, invariant and source/workload/hardware conditions; old schema/workload identifiers or absent/zero closure fields are insufficient. The revision is an admission marker, not cryptographic source authentication; exact source must still be established externally as done here.

## Independent executed gates

`AUDIT.md` records behavioral review before execution. All commands use sibling Go1.27.2, local caches, GOTOOLCHAIN=local, GOPROXY=off, GOSUMDB=off and GOFLAGS=-buildvcs=false. CGO0 is used except CGO1 race tooling. Go test process timeout is disabled; ordinary native cancellation is cooperative.

Executed on the unchanged fixed-reader composition copy:

- `go run ./internal/testscope`: PASS.
- `go test ./benchmarks/transactions -count=1 -timeout=0 -v`: PASS.
- `CGO_ENABLED=1 go test -race ./benchmarks/transactions -count=1 -timeout=0 -v`: PASS,1.358s.
- Focused `go vet`: PASS.
- Focused native pure-Go build: PASS.

The normal/race selections include all original regressions, bounded graph/refused-callback tests, descriptor-free closure ledger/report tests, pre-handle pending+unsupported controls, immutable status latches, bounded opaque/typed-nil/cyclic rendering, AST finalizer-before-Open and centralized-close laws, explicit revision rejection, intact ordinary shared lifecycle and cooperative cancellation. No extra native profile was necessary. Every command/status is in `checks.sh`, `checks.txt/.exit`, and named logs/.exit files; all exits are0. No command remained running at completion.

## Independent raw-report audit

The read-only `verify_reports.py` independently reconstructs all three owner fixed64-attempt reports. It validates workload/dataset identity, seed/ordinal transfers, contiguous outcomes, every successful commit, rollback/savepoint counts, final generation/model hash and conservation, global/window accounting, active/end-to-end times, throughput arithmetic, histogram counts/upper quantiles, null confidence bounds, V4 revision and explicit clean-closure fields.

- Rollback:64 attempts,48 commits,0.108698444 active seconds,passed.
- Shared WAL:64 attempts,48 commits,0.062026606 active seconds,passed.
- Correct-key encrypted shared:64 attempts,48 commits,0.468003002 active seconds,passed.

All are rows16/readers1/batch1 fixed-count smokes with the declared16MiB ceiling. These are bounded observations on a shared host, not independent confidence-qualified comparisons or duration-soak substitutes. Evidence: `report-verification.json`, `.txt/.exit` (0), and `verify_reports.py`.

## Failure preservation

The previous four V3 report hashes and every retained file/hash of its failed WAL fixture were compared against this reviewer's earlier manifest and are unchanged. The WAL requested3-second soak remains failed at1.675996218 active seconds after17497216 observed bytes exceeded16777216; the later64-attempt smoke does not replace it.

The original V1 rollback remains failed at14.015854093 active seconds, final_model_verified=false. Report hash `a62f4bfed390582d1125d88a6dfbe06f06f3a33500a55fedacea934424a1e101` and closed-data hash `b996a714bd220924cd93370ca257c553f17ec775159a33382606b9b29ac773e9` were reverified unchanged. Prior corrected samples do not change these statuses.

## Explicit limits and integration advice

No long soak was run or accepted. The proposed two-hour release observation is still future work and quota/capacity planning is not evidence of completion. Only Linux/amd64 ordinary behavior was executed here; foreign builds are not native runtime qualification. Shared-host smoke rates lack controlled repetition/confidence and must not be called performance-regression acceptance.

No broad old-base/full suite, manual tag, native failure injection, physical corruption/tamper/wrong-key/missing-selected-file, namespace mutation, crash/kill/abrupt DB-exit test/hot recovery, native I/O/lock/unmap/close/Sync/shortwrite/disk-full/permission fault, interrupted publication/rekey, main/remote write, publication or cleanup occurred. Future whole-source integration, CI and restored/published identities require their own exact gates. Performance-CI and lead were notified of this scoped V4 acceptance and the continuing limits.
