# Independent pre-DB SQL overlay review

Verdict: SOURCE-ONLY REVIEW PASS for the seven frozen files below. No blocking source defect found. This is not a test, compilation, physical fault, recovery, durability, or release PASS.

Latest amendment, 21:01 UTC: the initial source review missed the guard-positive representation defect described below. The one-file correction is independently CLEAR at SHA-256 `57c8af016ba160d2322971c63b845394ec19329d439d8441e74ad72ea5d3d28d`, superseding only the source-guard hash in the initial table. No production/manual/model/test file changed for this correction. Execution verification of the corrected guard remains the coordinator's next gate.

Reviewed on 2026-10-12 against base commit `a49d9bf2cb98101569fc15f6cbebf7ffc43069a1`, tree `d997f7ff3bd5b19f7b0cbb63a0975cf608f1bf30`. The base checkout remained clean when checked after the overlay was frozen. Overlay root: `presync-overlay/`. Manifest SHA-256: `f5adafc676f82f53f163b1b361011e18ce8f81ff0841c691b387ad9a9adbb46c`.

## Findings

- The appended IDs 5/6/7 select exactly JBODY_SYNC, JHEADER_SYNC, and PUBLISH_DIRSYNC occurrence 1, respectively. Their traces have 11/13/14 calls and 27/31/33 events, include the skipped target, then require successful ordinary cleanup unlink and directory sync. They contain no DB_PAGE or DB_SYNC and end as CommitFailed, logically closed, with retired owners.
- The public Commit return is captured unchanged and immediately passed to Finish. The pre-DB acceptance path jointly requires the original `*sqlite.Error` object, CodeIOErr, ErrPoisoned, and a chain bound to the actual private injected target and actual Tx.poisoned. The bounded observer rejects custom Is/As before its Unwrap operations, accepts only the runtime concrete errors.Join node type and exact injected/ErrPoisoned leaves, and rejects unknown, nil, extra, or over-bound causes. Actual cleanup/Close diagnostics therefore cannot be silently reduced to the chosen injected failure. The unchanged retirement paths were inspected to confirm their diagnostics are retained.
- Pre-DB trial bytes must equal the complete baseline. Original/copy/replay journal artifacts must be absent, recovery evidence must contain zero calls, and independent fresh read-only NoCreate readback must match the complete OLD rows, by_k index and independent scans, actual query-work counters, EOF/Err/Close, and DB.Check. Both physical-claim methods remain false.
- Existing four case IDs/plans and all 15 model roots are retained; six finite model roots are added. The nine existing SQL source-guard roots remain unchanged. Existing semantic negatives are retained alongside pre-DB negatives, including repinned cause-shape/order, target/count/classification, cleanup, journal, baseline, and recovery-selector changes. The four final byte pins match the independently hashed source files; complete shape snapshots remain supplemental to semantic checks.
- The seven-path boundary is respected. Shipping/default commit, VFS, recovery, A5 source files, main.go wiring/pins, and capacity limits are unchanged. No retries, cleanup repair path, native callback, process control, unsafe operation, or additional manual test root was introduced.
- One documentation ambiguity about the retained fourth-case compile scope was clarified before freeze: dual-tag compatibility compilation is distinguished from A5 execution or physical qualification.
- Accounting correction: the exact immutable base commit contains 71 literal variants in TestRollbackFaultSQLRepinnedSemanticCounterexamples; this overlay contains 141, comprising all 71 old variants plus 70 new ones. No old variant name is lost, and preserved-row edits only update the required diagnostic wording for the seven-case contract. The initial manifest's 69/139 accounting is inaccurate; it does not identify a source defect or justify changing assertions. Nine guard roots remain nine. These are source-row counts, not executed test/subtest results.

## Exact dependency and future verification boundary

Affected pure test packages are exactly `./internal/testkit/rollbackfault` and `./internal/testscope`. SQL model imports remain empty; model tests still import only testing; source-guard imports are unchanged. There is no new init or TestMain and no new default runtime dependency. The manual-only pager bridge adds standard-library errors and reflect; its reflect use is limited to the closed error-node type observation.

Pure `internal/testkit/rollbackrestart` does not call SQLPlan, ObserveSQLLedger, or use SQLLifecycleEvidence. Its shared SQLState, SQLRecoveryLimits, SQLSnapshotEvidence, and ValidateSQLSnapshot dependencies are unchanged, so its prior pure normal/race results need not be rerun for this delta. The actual SQLPlan(Control) dependency is in the manual pager restart armer; Control output is unchanged. The A5 caller's shared baseline/read/open/readback/recovery/owner helpers and types are unchanged and it does not call manualRollbackSQLCommit or use ManualRollbackSQLReport.

After separate authorization, bounded normal/race checks for the two affected pure packages, the existing source-guard runner/selection checks, and `./sqlite` manual test-c plus read-only nm inventory can close only model, source-boundary, build-selection, and compilation questions. If the authorized matrix includes both manual_fault_rollback and manual_restart_rollback, dual-tag test-c checks the existing A5 compatibility dependency only. Do not execute either manual binary or expand to whole-repository tests. New source/tool/binary provenance must identify these final bytes. Physical SQL injection, real recovery, process-exit/controller, hardware, and M3/M8/M10 acceptance remain NOT_RUN.

## Frozen SHA-256

| Path | SHA-256 |
|---|---|
| internal/testkit/rollbackfault/sql_ledger.go | 386477212655f9f8e22f3f1af278353a01ffcf373270be448c41dc287b1bdcbf |
| internal/testkit/rollbackfault/sql_ledger_test.go | 8600d32f3eb49298f1e8e1cf8075e0242835e20b99713d674b5c16be18dfdda5 |
| internal/pager/rollback_fault_sql_manual_linux.go | b152cd7fa33e2625114534b4e5477336fed66ba7575b6fee723d5d9dec916de1 |
| sqlite/rollback_fault_sql_manual_linux_test.go | 67097bbc8bc57ee171d40c78f9e3e58bcab50b34bb9e13ba8f1d696353f20bed |
| internal/testscope/rollback_fault_sql.go | 525ab5b4cf18db3a34511fe9956abf373b6fc62ee9e3565372f0ec71f5377c3b |
| internal/testscope/rollback_fault_sql_test.go | eb01a17464d2a531b8a39ccd4f328b85f61c2e8f7c9c27706ce889b335906b3f |
| docs/testing/rollback-fault-sql-manual-zh.md | a3e92de957120f50d313a0ccac41b16721f4ac8a7b71b9aa7bb011266f761931 |

Review method: read-only source/diff/dependency inspection, shell text/hash checks, and read-only Git identity/status. Relevant goal.md and existing support/guard/cleanup code were read; no applicable project AGENTS.md was found. This reviewer ran no Go, gofmt, tests, parser/scanner runner, compiler, fixture, native target, recovery, oracle, Git write, or network write. The implementation worker's separately authorized formatting is not execution evidence for this review.

## Guard-positive correction after the first normal run

The coordinator's retained `presync-validation/02_normal.stdout` records one failed root: TestRollbackFaultSQLSourceContracts, with only the closed cause-traversal declaration diagnostic. The retained failure summary reports rollbackfault 47 roots + 1,290 children passed; testscope 71 roots + 971 children passed and that one root failed. There were no skips and the remaining four gates were not run. These are observed coordinator results, not tests executed by this reviewer.

Exact root cause: the SQL guard parses with parser.ParseComments, so the cause helper's ast.FuncDecl includes its six-line Doc comment. readOnlyWALManualCompact prints the complete node using printer.Fprint, but the expected literal begins at `func` and omits Doc. The pinned local Go 1.27.2 source confirms printer.funcDecl calls p.setComment(d.Doc), at src/go/printer/nodes.go:1938. Body-part checks therefore pass while the full declaration comparison falsely rejects the unchanged valid helper.

The correction is exactly one seven-line hunk: if cause is nonnil, make a shallow local copy, clear only its Doc field, and use that copy for the existing full comparison. Signature, body, order, exact-type checks, bounds, custom-Is/As rejection, sentinel leaves, and expected literal are unchanged. The original AST, complete source pins/shapes, all 141 variants, and other guards are untouched. No general normalization layer or new type assumption is introduced.

Corrected source: `presync-correction-overlay/internal/testscope/rollback_fault_sql.go`, SHA-256 `57c8af016ba160d2322971c63b845394ec19329d439d8441e74ad72ea5d3d28d`. Minimal diff: `presync-correction-evidence/CORRECTION.diff`, SHA-256 `030b7301104be01a9239bd8a16191e856d18508ccf79025c68434fb68d2d8301`.

CLEAR to continue the already-authorized bounded verification: repeat only testscope normal, then the original unrun gates. The unchanged rollbackfault normal result can be retained. This correction does not authorize any manual/native target, SQL fault, recovery, or controller execution, and establishes no physical result.
