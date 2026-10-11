# Independent review: limited public rollback manual harness, V1

Status: SOURCE_REVIEW_CHANGES_REQUIRED. Three bounded evidence/failure-path findings should be corrected before accepting this harness/manual. No production-replay defect was identified. This is source review only: no Go invocation, Go parser, gofmt, go list, build, compiler, test binary (even with gates off), fixture generator, database Open, recovery, fault, namespace or other physical scenario was executed. All physical scenarios remain NOT_RUN. There was no remote, branch, main or CI action.

## Identity and preserved evidence

- Base commit: 56bbcf3febe233197aa0a5eac965c67a67a88011
- Independently read base tree: c31e7dcbeed9732688681741ee38ec60ff0bfb49
- Candidate: public-recovery-harness-work-v1
- Added source: sqlite/rollback_recovery_manual_linux_test.go, 9948 bytes, SHA256 fe13d38fda59e01d56760bc6ca9118e8b76d244c3ffdd2e1e5c38891c059fd06
- Added manual: docs/public-rollback-manual-zh.md, 9180 bytes, SHA256 76ddb33e7313c75783f41fa6373aa049524caf7c17bb14d5d8123c92b60e5705
- Full candidate inventory contains 3038 files. Git blob identity plus executable/symlink mode comparison against the base found exactly these two additions, no deletion and no changed baseline file. Production, goal.md, dependencies and ordinary scripts are unchanged.
- Original lead source and manual bytes are retained under original-source/; original-candidate.patch, baseline-delta.json, pre-review-inventory.json and review-identity.json capture the reviewed identity.

## Findings, with minimal recommended fixes

### R1 — blocking: exclusion checks are fail-open shell conditions

Location: docs/public-rollback-manual-zh.md:68–70.

The two `! grep -Fx ...` commands do not stop a `set -e` shell when the forbidden file is present: the nonzero result belongs to an inverted pipeline, which is an errexit exception. A grep read error can also be inverted into success. The positive manual grep verifies presence, but does not enforce the stated exact count of one. Therefore the advertised default/oracle exclusion and exact selection checks are not enforced by these instructions.

Minimal fix: replace all three checks with a single explicit checker that reads all three files, requires counts 0/0/1 and fails on a read error. A source-only Python count checker is appropriate. Do not execute any test binary to verify this fix.

### R2 — blocking for the full retained-input verification claim: cleanup omits manifest and stops after first backup error

Location: sqlite/rollback_recovery_manual_linux_test.go:170–185, 214–218; manual:14.

Only the three backup byte slices are in the cleanup map. input-sha256.json is written later and is never verified. Moreover, the cleanup loop calls manualPublicRecoveryEqual, whose Fatalf ends that cleanup callback on its first mismatch/read error. Map iteration order is unspecified, so after one failure the other two backups may never be checked. Files are retained, but V1 cannot substantiate complete backup-and-manifest integrity checking on PASS/FAIL.

Minimal fix: preserve the exact manifest bytes, include them in the retained-input verification set, and use a nonfatal per-file checker (Errorf) inside the cleanup loop so every retained input is checked even when one is wrong. Register the checker before public operations. Keep the active path assertions fatal if desired. Do not modify or delete retained files on failure.

### R3 — blocking for fail-closed failure handling/evidence: fatal paths bypass pending snapshot and the next scenario still starts

Location: sqlite/rollback_recovery_manual_linux_test.go:52–124, especially 58/84/94/123 and 73–76; Close helper:262–269.

All per-scenario PendingPlaintextCleanups checks are normal-path tails. An Open/Schema/byte assertion Fatalf bypasses that scenario's pending snapshot. The parent ignores every t.Run return value, so another public-API scenario is then started after that failure. In the success case, explicit closeDB records a Close error with Errorf but returns normally, permitting the next read-only Open on the same path after a failed Close. The current ordinary DB shape has no long-lived session, so no actual Close failure was observed or inferred; this finding concerns the harness's stated failure-handling contract.

Minimal fix: after the initial gate/root checks, register a read-only pending snapshot cleanup that reports with Errorf and does not retry; stop the top-level function when a t.Run returns false. Ensure the explicit Close step cannot continue into the reopen if Close fails. Keep its one-attempt flag, so cleanup does not repeat a failed Close. A top-level pending cleanup runs after subtest DB cleanups and covers early Fatalf exits. An immediate pending check after the first successful Close further documents the pre-reopen condition.

### C1 — nonblocking reproducibility improvement: establish Go environment before first Go invocation

Location: docs/public-rollback-manual-zh.md:35,46,53–56.

The Go version check and version capture precede GOTOOLCHAIN=local, GOENV=off and offline proxy settings. An ambient Go launcher/config can therefore select or download a toolchain before the document later establishes its controlled environment. Move these controls before the first Go invocation and explicitly verify/record host GOOS/GOARCH versus the build target. No setup/download was attempted during review.

## Reviewed properties that are sound in V1

### Isolation and scope

- Exact first line is `//go:build manual_recovery && linux`; filename also ends in _linux_test.go.
- Test first statement is the exact environment comparison followed by Fatal. No Helper, TempDir, fixture construction, Open or pending query precedes that guard.
- No new package-level var initializer, init or TestMain exists. A repository-wide textual init/TestMain search found only existing driver registration, which calls database/sql.Register and does not open a DB. No TestMain was found.
- Default and oracle-only selection logically exclude this source. Linux plus manual_recovery selects it. Combining oracle and manual_recovery would also select it; that combination is not a permitted ordinary command. This is a source-level deduction, not go-list evidence.
- internal/testscope/main.go:20 explicitly exempts the exact approved first line. No guard production change is added.
- The helper call chain is entirely in the new file except public sqlite APIs and pure in-memory journal encoding. The harness does not call internal RecoverRollback or the private public bridge directly.
- There is one top-level test with exactly the four approved subtests. There are no new Skip, Parallel, subprocess, fault hook, forced exit, Remove/RemoveAll, automatic TempDir cleanup, cleanup retry or direct recovery calls.
- Root checks require nonempty, absolute, clean, existing non-symlink directory, EvalSymlinks identity and emptiness. Each scenario uses a fresh MkdirTemp child. The scope explicitly excludes adversarial namespace replacement; no stronger race-resistance is claimed.

### Fixture source and public assertions

- Fixture construction at 138–168 matches internal/pager/recovery_manual_test.go:39–62 together with emptyDatabasePage at internal/pager/tx.go:222–236. The original is a valid 512-byte-page empty sqlite_schema with page 2 as an empty freelist trunk, 2 pages total and consistent change/version counters.
- The current image is 3 pages with incremented counters and altered unused trunk byte. The original 2 pages are supplied by two finite journal records, sector/page size 512 and the same fixed nonce as the existing internal fixture. The resulting journal layout is 512 + 2*(512+8) bytes. This was reasoned from source; no fixture generator ran.
- Fixture writes use O_EXCL and mode 0600, check short writes and join Write/Sync/Close diagnostics. Directory Sync and Close are joined. No generated input is automatically deleted.
- Default nil option checks nil DB, public CodeIOErr, ErrRecoveryRequired, and current DB/journal byte preservation.
- Explicit opt-in checks public Open success, restored metadata, empty Schema, full original bytes, journal absence, one explicit Close, and a separate read-only NoCreate reopen plus unchanged bytes/journal absence.
- MaxJournalBytes=1 checks public CodeIOErr plus both ErrRecoveryRequired and ErrMemoryLimit, and preserves active DB/journal bytes. Production wrapping at sqlite/sqlite.go:80–99 maps pager.ErrLimit to the public memory cause while the recovery cause gives CodeIOErr, so the assertion is appropriate.
- NoCreate uses an absent path in its own empty directory, checks nil DB and os.ErrNotExist, and asserts absence of DB/journal/WAL/SHM and every other directory entry.
- Unexpected returned DBs are closed once and their Close diagnostic is reported. Success DB ownership registers a cleanup fallback and marks attempted closure before calling Close, preventing implicit retry.
- No fixture/manual content claims crash durability, broad M3 acceptance, driver recovery, other-platform recovery or full fault qualification. The main boundary document and acceptance status retain historical implementation-missing entries; those are unchanged baseline documents, not current V1 compile/runtime evidence.

### Manual and syntax review

- The manual records exact source commit/tree, tool version, kernel, capacity/inodes/mount, Go environment and module verification; isolates source with git archive; compiles only ./sqlite with manual_recovery; reads symbols without executing; and documents an exact anchored runtime selector with count=1.
- Runtime command has both explicit environment values and no expanded package/test selector. It does not use `go test -tags manual_recovery ./...`.
- Pass criteria explicitly require exit zero, the top-level and all four subtests PASS, zero FAIL/SKIP, complete assertions, and identity/evidence. Unmatched selector, gate refusal, partial PASS or abnormal interruption do not qualify.
- The entire RUN directory is retained on success/failure; subsequent deletion is a separate manual choice after archival and process/handle review.
- Extracted all three shell blocks and ran only `bash -n` against them: all three returned zero. This establishes shell parse validity only and does not validate operational behavior or R1.
- Python post-run evidence code was read, not executed. It hashes retained files but does not compare those hashes with the input manifest, which is why R2 should be fixed in the harness.

## Compiler-only next stage, not executed or authorized by this review

After fixes, preserve a new exact identity and obtain the explicit compiler release. The later allowed checks can use the pinned Go 1.27.2 tool with the established offline/local settings to:

1. Inspect syntax/AST without executing a test package or generated fixture.
2. `go list -f '{{join .XTestGoFiles "\n"}}' ./sqlite`
3. `go list -tags oracle -f '{{join .XTestGoFiles "\n"}}' ./sqlite`
4. `go list -tags manual_recovery -f '{{join .XTestGoFiles "\n"}}' ./sqlite`
5. Compare exact source filename counts 0/0/1 with a fail-closed external check.
6. `go test -c -p=1 -vet=off -tags manual_recovery -o <review-evidence>/public-recovery.test ./sqlite`
7. `go tool nm <review-evidence>/public-recovery.test` and require the exact named top-level function symbol, then hash source and binary.

Use explicit build target/tool/cache paths already reviewed by the task owner; the placeholder is not an executable command. Never invoke the produced binary, not even for -test.list, a mismatching selector, gate-off, or a smoke check. Compiler success cannot change physical NOT_RUN status.

## Review stopping condition

V1 review is complete, with original bytes and findings preserved. Await a separately identified corrected source/manual and new review/compile instruction. The candidate was not edited by the reviewer.
