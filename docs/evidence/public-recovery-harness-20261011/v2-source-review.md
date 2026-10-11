# V2 source-only addendum

Result: SOURCE_REVIEW_PASS for the limited V2 source/manual. The three V1 blocking findings are resolved. This result permits no physical execution and is not compiler, runtime, recovery, durability or overall project acceptance. Explicit compiler release is still required. No Go command was invoked.

## Exact identity

- V2: public-recovery-harness-work-v2
- Base: 56bbcf3febe233197aa0a5eac965c67a67a88011 / c31e7dcbeed9732688681741ee38ec60ff0bfb49
- Go source: 10657 bytes, SHA256 dd5a63e2a8e7d4fdd969ba03273a8f8147d5eff81babf8fce44a5228c5cdaf5e
- Chinese manual: 9674 bytes, SHA256 b103371c81fd69a7916c6ec72ee479359fce3409b37c14ecc94e67dc827f3915
- Complete 3038-file V2 inventory differs from V1 only at the same two approved paths. Since independently verified V1 differs from the base only by those additions, V2 also has exactly those additions and no changed/deleted baseline file. All production code, goal and dependencies are unchanged.
- V1 complete inventory still matches its original reviewed inventory. V1 report and original files were not edited.
- V2 copies, v1-to-v2.patch, identity.json and pre/post inventories are in this v2 evidence directory.

## Resolution of each V1 finding

R1, manual:68–77: the standalone Python command reads all three lists and requires counts exactly 0/0/1. An unexpected count raises SystemExit; an unavailable/unreadable file raises an uncaught read exception. This command is not inverted, conditional or piped; under the preceding set -e either failure stops before compile. No checker snippet was executed.

R2, source:187–208: the exact manifest bytes, including their final newline, are inserted into the same retained-input map. Cleanup visits a fixed ordered list of all four filenames. It uses direct ReadFile plus Errorf per failure, so later files are still checked. The registered verification precedes publication of working inputs and every public Open. The map and slices are not subsequently mutated. The manifest deliberately hashes the three database/journal inputs rather than hashing itself; the immutable exact manifest bytes are the separate verification oracle. A fixture-construction failure before the four-file set is built is not a completed fixture or PASS; whatever was written remains retained.

R3, source:50–54,57–141,276–298: a top-level cleanup enumerates pending errors read-only with Errorf, so it still runs after a subtest's Fatalf and after the subtest DB cleanup. Each of four t.Run results is checked and a failed scenario returns immediately. Nonfatal backup cleanup errors also make the subtest fail and therefore stop later scenarios. Close now returns a cached success/failure bool; it marks the attempt before calling DB.Close and its cleanup fallback never repeats that attempt. Both explicit closes return from the subtest on failure, so no read-only reopen follows a failed first Close. Those paths preserve diagnostics without retry or deletion.

The manual:114 explicitly describes the new fail-stop, nonretrying pending diagnostics and complete constructed-input verification semantics. The original first-statement environment guard, exact build constraint, fixture bytes, actual public calls and four scenario assertions are unchanged.

## Remaining nonblocking note

V1 C1 remains applicable: manual:35/46 invoke Go before the environment controls at 53–54. Moving local/offline controls before the first Go invocation would make the manual more reproducible. For any compiler stage in this task, set the controlled environment before all Go invocations regardless of this document ordering.

## Verification actually performed

- Read every V1-to-V2 hunk and the complete affected functions/manual command flow.
- Preserved V2 originals and full byte/size/mode inventory.
- Compared complete V1/V2 inventories, and reconfirmed original V1 identity unchanged.
- Extracted all three V2 shell fences; `bash -n` returned 0 for each.
- Parsed the two embedded Python snippets with Python ast.parse only. Neither snippet ran.
- Read host/tool metadata only: host Linux/x86_64; pinned tool executable exists at toolchains/go1.27.2/bin/go. No version command, Go parser, Go listing, compile, test, fixture generator or binary was run.

## Exact proposed compiler-only commands, after explicit release

Use the already approved isolated compiler environment, its verified offline module cache and resource/writability thresholds. In particular verify GOPATH/GOMODCACHE externally; this review did not discover or change module-cache configuration. These commands are a proposal only:

    ROOT=/workspace/scratch/5e06a74e5f65/durable-restart
    WORK="$ROOT/public-recovery-harness-work-v2"
    EVIDENCE="$ROOT/public-recovery-harness-independent-review-v1/v2"
    GO="$ROOT/toolchains/go1.27.2/bin/go"
    cd "$WORK"
    export GOTOOLCHAIN=local GOPROXY=off GOSUMDB=off GOENV=off GOWORK=off
    export GOFLAGS=-buildvcs=false GOMAXPROCS=2 CGO_ENABLED=0 GOOS=linux GOARCH=amd64
    unset GOSQLITE_MANUAL_RECOVERY GOSQLITE_MANUAL_RECOVERY_DIR GOSQLITE_MANUAL_NAMESPACE
    "$GO" run -p=1 "$EVIDENCE/check_ast.go" -- "$WORK/sqlite/rollback_recovery_manual_linux_test.go"
    "$GO" list -p=1 -f '{{join .XTestGoFiles "\n"}}' ./sqlite > "$EVIDENCE/default-files.txt"
    "$GO" list -p=1 -tags oracle -f '{{join .XTestGoFiles "\n"}}' ./sqlite > "$EVIDENCE/oracle-files.txt"
    "$GO" list -p=1 -tags manual_recovery -f '{{join .XTestGoFiles "\n"}}' ./sqlite > "$EVIDENCE/manual-files.txt"
    python3 "$EVIDENCE/manual-block-2-python-1.py.txt" "$EVIDENCE"
    "$GO" test -c -p=1 -vet=off -tags manual_recovery -o "$EVIDENCE/public-recovery.test" ./sqlite
    "$GO" tool nm "$EVIDENCE/public-recovery.test" > "$EVIDENCE/symbols.txt"
    python3 - "$EVIDENCE/symbols.txt" <<'PY'
    import pathlib, sys
    target = 'github.com/ms-xiaosnow/gosqlite/sqlite_test.TestManualPublicRollbackRecovery'
    matches = [row.split() for row in pathlib.Path(sys.argv[1]).read_text().splitlines()
               if row.split() and row.split()[-1] == target]
    if len(matches) != 1:
        raise SystemExit(f'expected one exact top-level symbol, got {len(matches)}')
    print('exact top-level symbol: PASS')
    PY
    sha256sum "$WORK/sqlite/rollback_recovery_manual_linux_test.go" "$WORK/docs/public-rollback-manual-zh.md" "$EVIDENCE/public-recovery.test"

Capture argv, UTC clocks, output, natural exit codes and resource checks for each approved stage and stop on a failure. Compare the complete source inventory afterward. check_ast.go is a newly supplied external inspector, not a project test: it imports only standard AST/parsing libraries and reads the candidate as text. Its proposed `go run` executes only that inspector; it never imports or calls the candidate package. If the release does not include running a source-only AST inspector, omit that command and request the exact additional scope rather than substituting a target binary. The inspector is itself uncompiled/unexecuted at this source checkpoint.

The proposed code checker examines the exact constraint truth table, unique entrypoint, absence of package variable/init/TestMain hooks, exact first-statement environment refusal, four ordered fail-stop scenario calls and five public Open references, with no cleanup retry/deletion calls. This complements the behavioral source review; it is not full static verification.

Never invoke public-recovery.test, including -test.list, a nonmatching selector, gate-off or a smoke run. Do not enable manual tags in aggregate tests. Physical scenarios remain NOT_RUN regardless of compile/symbol outcome.
