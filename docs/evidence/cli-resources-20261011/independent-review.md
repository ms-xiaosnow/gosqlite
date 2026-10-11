# Independent CLI/example resource review

## Verdict

PASS for the four declared source slices and the authorized bounded combined verification. No correctness or scope blocker found. This is not full-repository, physical-durability, recovery, production-readiness, or milestone acceptance.

Reviewed baseline: `1352ae524575e4d6fe8b5434fd930c3b07be4ac8`, tree `2b91b20abec4b2d651de1dd860fe86e79b62bac9`.

Verified combined tree: `4c2af13646c973c53340450f8f1310492aae5628`.

## Source findings and coverage limits

1. Sparse example status. Wrong arity returns 2 without invoking the callback; a returned callback error is printed to stderr and maps to 1; success maps to 0. Context/path/stdout reach the callback unchanged. The exact main-body AST check verifies `os.Exit(run(...))`, and its preserved legacy main is rejected as source data only. Existing database example and original success test are unchanged. Runtime coverage establishes pure helper behavior plus the ordinary success fixture. It does not execute a process to measure actual nonzero statuses or exercise database failure cleanup.

2. CLI query close. The new named-result deferred join executes Rows.Close before the existing DB.Close join. Reviewed Rows.close unregisters itself from db.rows, so the old discarded Rows.Close error would not subsequently be recovered by DB.Close. AST checks the actual named result and acquired rows identities and rejects the exact previous defer. Header/first-row output-writer tests preserve the original errors.Is identity and exact emitted prefix on an intact readonly fixture. Those writer tests would also pass the old defer; actual Close-error propagation is statically checked, without a real Close failure injection. Ordinary query/write positives remain unchanged.

3. Readonly example. NewConnector preserves explicit ReadOnly/NoCreate even for mode=rw/rwc. Caller context reaches QueryContext; Rows.Close and Rows.Err are joined before DB.Close; reporting and signal stop precede final process exit. Twelve combinations (three page sizes, plain path and ro/rw/rwc URIs) demonstrate successful expected output and unchanged final fixture-copy directory inventory, permissions, sizes and bytes. Final-state equality does not prove absence of transient side effects. Write prevention/NoCreate, storage-close failures, interrupted iteration and signal behavior are source-level evidence here, not runtime negative tests. Output/reporting errors are tested only in pure helpers without a database. The legacy main is pinned and parsed only.

4. Write example. Deferred rollback precedes DB.Close; normal sql.ErrTxDone is ignored and other cleanup errors are joined into the named result. Success output is emitted only after the helper returns and its cleanup has finished. Tests reject the legacy source plus eight textual AST mutations; runtime coverage is pure argument/callback status behavior and ordinary two-commit/reopen/CRUD/normal-rollback/native supported Check. Failing output-writer propagation is AST-only in this slice; there is no runtime failing-writer test. No genuine Close/rollback failure or process exit is exercised. Native Check is not SQLite's unrestricted integrity_check or physical-durability evidence.

The source assertions intentionally constrain exact syntax and are not substitutes for dynamic failure-path testing. No extra runtime tests were added or run to close those deliberately excluded gaps.

## Scope and identity

Independent in-memory Git blob/tree encoding reproduced the exact combined tree without invoking Git or writing Git metadata. All 2,911 files and 137 directories were inventoried before and after the run, including raw permissions, content hashes and names. Before/after inventories are identical; the original combined frozen source also remains unchanged.

Exactly 14 declared paths differ from baseline. All 353 baseline test files are unchanged. Every owned non-progress path is byte-equal to its separately reviewed freeze. Progress consists only of the unchanged baseline prefix and each of the four original appendices. The independent verification copy is `../cli-resources-independent-work-v1`.

The three supplied all-file SHA manifests (sparse, readonly, write) were independently checked against every frozen file. Query-owned hashes and its supplied frozen-tree manifest hash were checked; full frozen-source comparisons against the already verified sparse freeze confirmed only each slice's declared deltas. The combined inventory was then verified in full.

## Executed checks

Fixed Go 1.27.2, Linux/amd64; GOTOOLCHAIN=local, GOPROXY=off, GOSUMDB=off, GOFLAGS=-buildvcs=false, GOMAXPROCS=2, fixed external GOPATH/GOCACHE, default /tmp. Ordinary/build/vet used CGO_ENABLED=0; race used CGO_ENABLED=1.

All 18 exact selected top-level tests passed in ordinary and identical race runs, each with -count=1 and -timeout=60s. There were no skips or failures:

- Sparse: 4 top-level tests; ordinary 0.003s, race 1.028s.
- Query: 5 top-level tests; ordinary 0.018s, race 1.054s.
- Readonly: 6 top-level tests; ordinary 0.040s, race 1.140s.
- Write: 3 top-level tests; ordinary 0.009s, race 1.078s.

Sourceguard passed before ordinary and before race. Scoped four-package vet passed. Four separate -trimpath CGO0 builds passed; none of their binaries was executed. Every command exited 0 on its first run, all stderr logs are empty, and there were no tool denials or retries. Exact selectors are in selectors.json; command arguments, timestamps, stdout, stderr and exit status are retained per command. summary.json and identity.json summarize the run; before-inventory.json and after-inventory.json preserve identity evidence. verify.py is the independently authored, external-only verification harness.

## Explicit exclusions and documentation note

No full suite, full race, oracle, fuzz, manual tags, performance, other-OS execution, native failure, database corruption/missing-input, crash/process-exit, namespace, recovery, wrong-key/tamper, physical-durability, blocked ANN V4/41-fuzz/unknown-chain/older-held workflow was run. No source edits, Git command, remote operation or network request occurred during this review/verification. No assertion here releases any existing HOLD/BLOCKED state.

The readonly owner proposed an external AST process and the parent selected a passing in-process AST unit assertion instead, before execution. This was an unadopted design proposal, not a platform/tool denial. The external harness remains NOT_RUN. The parent plans a docs-only wording correction after gates; this report does not modify the verified frozen candidate or extend its tree identity to future documentation edits.
