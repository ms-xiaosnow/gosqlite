# Independent review: transaction/ordinary-soak harness v2

## Verdict

**Do not accept v2 yet.** The deterministic workload/model, actual-duration accounting, bounded report design, identity controls, and passing ordinary-path smoke evidence are useful. Two harness correctness findings remain: permissive joined-error classification (reproduced purely in memory) and incomplete all-handle shutdown/error ownership accounting (source-only). Neither finding was tested by inducing a native failure.

The already repaired rollback-reader acquisition defect is separate engine work. This review did not rerun the known-unfixed stochastic failure hoping to obtain a pass, and does not erase any prior failed run.

## Exact reviewed and executed identities

- Immutable harness freeze: `../transaction-soak-freeze-v2`, tree `3270481eb990eb7b8eada1c1d052573e7f708dae`.
- Baseline: `../plain-final-verify`, tree `7d3184f87165e11e12560884ec6e4f92c9bfd039`.
- Harness patch: `../transaction-soak-evidence/source-v2.patch`, SHA256 `71b64b771631b39978f635aadb3348c0e42d6f7c652f25e7f21b719f1d973606`.
- Runtime-only composition: `../rollback-reader-acquisition-candidate-soak-v1`, tree `ac7242786b6e7ecffd03f9c45216ee788eaddbba`.
- Runtime composition contains the separately reviewed reader fix, engine tree `ad066d04d676b8368ca7d02d83664e50dfa47216` on grouped base `13f53e4cdadaaca2530e17e5f7a61b9dfe8e5c33`, plus byte-identical five v2 harness files.

Independent Git-object/tree hashing inspected every regular path, full bytes, Git mode and exact filesystem mode. All 1252 baseline files are unchanged; exactly five files are added under benchmarks/transactions (1257 final files). The 1329-file composition also exactly matches its asserted tree. A fresh full baseline copy received git apply --check and git apply; its every file/mode equals the freeze and it reconstructs the same tree. A separate exact composition copy was used for execution. Before/after manifests show all five source/copy trees unchanged. No Git index or main source was modified.

Evidence: `identity.json`, five `*-inventory.json` files, `post-run-source-verification.json`; independent reproducer `../transaction_soak_independent_identity.py`. The separate `pure-memory-negative` copy has one added reviewer test and is deliberately not described as an unchanged source tree.

## Findings

### 1. Blocking: unexpected joined diagnostics can be silently accepted

`benchmarks/transactions/workload.go:307–339` uses errors.Is(err, ErrBusy) and errors.Is(err, context.Canceled/DeadlineExceeded) after rejecting only errCleanup or the first errors.As sqlite.Error whose code happens to be IOErr.

Consequences, reproduced by seven pure-memory cases:

- errors.Join(ErrBusy, ErrChanged) and errors.Join(ErrBusy, an unknown diagnostic) become nonfatal BUSY.
- errors.Join(context.Canceled, unknown) and errors.Join(DeadlineExceeded, ErrChanged) become nonfatal cancellation.
- A first/outer sqlite.Error with CodeBusy hides a later/nested sqlite.Error with CodeIOErr from the first errors.As result.
- An ordinary fmt wrapper does not prevent the suppression.

Each case increments BUSY or canceled, leaves failed_outcomes at zero and returns fatal=false. ordinaryBusy also authorizes commit retry, so fixing only classify is insufficient. This violates the strict acceptance requirement and the README assurance that cleanup/I/O diagnostics cannot be masked. It does not establish that one of the retained successful native samples actually contained such an error graph; those graphs are not retained in its trace.

Use a bounded strict traversal that checks every relevant node and every leaf. Unknown, cleanup, unsafe SQLite-code, malformed or over-budget/cyclic graphs must be fatal; never invoke arbitrary custom Is/As/Unwrap while trying to prove an error ordinary. Keep the actual native BUSY adapter leaf identities and normal fmt/errors.Join wrappers supported. Mixed known BUSY/cancellation must not enter commit retry.

Reproduction: from `pure-memory-negative`, use the pinned/offline environment in checks.sh and run:

`go run ./internal/testscope`

`go test ./benchmarks/transactions -run '^TestIndependentUnexpectedJoinedErrorsRemainFatal$' -count=1 -timeout=0 -v`

Evidence: `negative-guard.exit` is 0; `negative.exit` is 1, with all seven subcases in `negative.txt`. The test source is `pure-memory-negative/benchmarks/transactions/independent_negative_test.go`. This test only constructs in-memory error values and calls helpers; it opens no database and installs no fault hook.

### 2. Blocking shutdown-report/ownership contract gap

`workload.go:352–359` joins each reader DB.Close error with errCleanup. If one reader close fails, executeEpoch returns that error, and run exits through its final writer close. At `main.go:268–276`, a successful close of the writer alone unconditionally sets shutdown to workers_joined_normal_close. That would falsely describe all handles as normally closed despite the retained reader-close error. Overall status remains failed, but the shutdown evidence is inaccurate.

Separately, `main.go:334–338` sets db=nil on an epoch Close error, bypassing the final owner. This contradicts the comment at265–266 that the owner is cleared only after successful closure. sqlite.DB.Close's explicit contract (`sqlite/sqlite.go:390–395`) says native shared/encrypted pending-close failures may retain managed owners for Close/retry APIs. Reader handles also fall out of scope after their one close attempt.

Preserve an all-handle close outcome and any still-owned pending handle, do not overwrite a prior closure error when another handle closes, and report incomplete/error closure honestly. Handle supported retirement retries only with bounded, source-reviewed ownership semantics. Regress this with pure-memory owner/report models and/or AST laws; no native close/I/O failure test is authorized. This finding is source-derived, not an induced native reproduction.

## Source assessment apart from findings

- One writer derives transfers from seed/ordinal, not execution timing. Committed immutable slices are published while the model mutex covers Commit, preventing a reader from observing a committed generation before its model exists. All expected rows and IDs are checked, not just a conserved sum.
- Full rollback verifies the unchanged model; savepoint rollback restores the exact outer overlay; release commits both transfers. Reader transactions read generation then verify all rows twice while holding one snapshot. History is pruned only after workers join and ordinary reopen is verified.
- C/R/B/X/F outcome strings are bounded to64 attempts per window and ordinal-contiguous. Successful commits can be independently reconstructed. Retry events are separately counted. Trace identity changes with actual outcomes; workload identity captures complete configuration excluding source/hardware labels.
- Model memory is bounded by rows and at most65 generations per epoch; at most8 reader workers plus1 writer; fixed histograms and4096 windows bound retained report state. Memory is explicitly soft/sampled, not claimed as hard RSS or true peak. Data bytes are sampled at epoch boundaries; documented bounded plaintext overshoot and encrypted engine quotas are not mislabeled exact caps.
- Requested soak time is accumulated monotonic concurrent epoch time including cooperative drain. Bootstrap, handle-opening/closing outside the active interval, checkpoint, reopen and verification remain in total elapsed time. Passing never substitutes maintenance time, attempt quota or storage/memory/epoch exhaustion for requested active duration. A max-epoch stop fails.
- Workers synchronize their start, cancel cooperatively, send exactly one result each, and are joined before handle closure. Ordinary success paths close rows/transactions normally. The closure-failure contract gap above is separate.
- Attempts, commits and end-to-end rates have distinct denominators; intentional rollback is not a commit. Histogram quantiles are explicitly upper bounds and all attempts, including failed/BUSY/canceled, are included. Unequal correlated window rates are descriptive;95% confidence bounds are null. No external-engine scores or recovery timings are invented.
- Source/hardware labels are user-supplied declarations, not independently measured in the binary. This review supplies the exact externally verified source identities and observed host details. Shared-host rates are observational only.

## Independently executed gates

`AUDIT.md` records behavior review before execution. All commands use sibling Go1.27.2, GOPROXY=off, GOSUMDB=off, GOTOOLCHAIN=local, GOFLAGS=-buildvcs=false and local caches. CGO0 except race CGO1. Go test process timeouts were disabled rather than risking forced open-DB termination.

On the unchanged fixed-engine composition copy:

- `go run ./internal/testscope`: PASS.
- `go test ./benchmarks/transactions -count=1 -timeout=0 -v`: PASS,0.096s.
- Separate `CGO_ENABLED=1 go test -race ./benchmarks/transactions -count=1 -timeout=0 -v`: PASS,1.224s.
- Focused `go vet` and pure-Go native build: PASS.
- One predeclared three-second rollback soak: PASS,3.001993480 active seconds,3.011302198 total seconds,418 attempts,264 commits,105 intentional rollbacks,1430 successful reads,7 ordinary verified reopens. Exact final SQL model verified and normal closure reported. This ordinary run contained no reported unexpected errors.

Raw commands/logs/status: `checks.sh`, `checks.txt/.exit`, `guard`, `normal`, `race`, `vet`, `build`, and `soak-rollback-3s` artifacts. Data and report hashes are in `retained-smoke-data.json`. Host observation: Linux/amd64, AMD EPYC9V74,9 visible logical CPUs, shared cloud host; `host.txt`. No controlled-performance assertion.

## Independent report reconstruction and failure preservation

The new read-only `verify_reports.py` independently reconstructs14 v2 reports: seven owner harness reports, six separate engine-comparison reports, and this review's smoke. It verifies seed/ordinal transfer results, contiguous trace accounting, rollback/savepoint counts, generation and exact model hash, conservation, per-window/global counts, active/total timing, throughput arithmetic, histogram counts/upper quantiles and null confidence bounds. Both engine baseline controls remain failed with their original errors. `report-verification.json` retains every report SHA256 and status; `report-verification.exit` is0.

The original V1 rollback remains FAILED at14.015854093 active seconds with723 commits. Its report hash remains `a62f4bfed390582d1125d88a6dfbe06f06f3a33500a55fedacea934424a1e101`, final_model_verified=false. Its normally closed data hash remains `b996a714bd220924cd93370ca257c553f17ec775159a33382606b9b29ac773e9`. Both hashes were independently checked without mutation. A later passing sample does not convert it to passed.

## Explicit limits

No long soak was executed or accepted. The two-hour-per-profile proposal is capacity planning, not passed evidence; any quota or epoch stop remains failed/incomplete. Three seconds and prior20-second samples cannot establish multi-hour reliability or confidence-qualified performance. Only Linux/amd64 was executed here. Owner foreign-target builds are compilation evidence only and were not rerun or executed by this review.

No broad old-base/full suite, native failure injection, physical tamper/corruption/wrong-key/missing-selected-file, namespace mutation, abrupt DB exit/process kill/crash recovery, interrupted publication/rekey, manual-tag execution, remote/main write or cleanup occurred. Native failure/recovery, other native platforms, combined-source integration, CI and final publication identity remain separate acceptance work. V3 repair/review, if supplied later, must remain separate from this immutable v2 verdict and its preserved failing controls.
