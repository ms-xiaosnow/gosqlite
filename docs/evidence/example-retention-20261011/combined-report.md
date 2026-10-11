# Final finite closeout combined ordinary aggregate

## Result

PASS. All 18 stages completed naturally on their first attempt; aggregate and outer tool-session exit were 0 at 2026-10-11T04:23:35Z. All stage stderr logs are empty. No failed gate, tool denial or retry occurred.

- Started: 2026-10-11T04:07:57Z. Finished: 2026-10-11T04:23:35Z (15 minutes 38 seconds).
- Baseline commit: `1352ae524575e4d6fe8b5434fd930c3b07be4ac8`, tree `2b91b20abec4b2d651de1dd860fe86e79b62bac9`.
- Verified frozen tree: `d7393c254a6fca4452ae194d64d9045659f9781d`.
- Frozen source: `final-closeout-combined-freeze-v1`. Owned copy: `final-closeout-combined-verify`.
- All 2,935 files and 141 directories, bytes, SHA256, Git blob/modes, exact filesystem modes, directory membership and root mode match the supplied inventory and remain unchanged before/after. Both copies contain no `.git`.
- All 353 original test files and all 358 tests from prior CLI tree `4c2af13646c973c53340450f8f1310492aae5628` remain byte/mode unchanged.
- Exactly the eight reviewed retention paths are the Go delta over that prior passing CLI tree. Their content/modes match the independently reviewed retention freeze. Other changes are documentation/evidence only. Module files, scripts, source guard, database kernel and backends remain unchanged.

## Source and scope review

Read the repository test instructions, ordinary/sourceguard scripts, finite closeout scope and historical test-scope correction. Read the prior independent review and complete ordinary-gate report, retention independent review, and both new retention test files plus helper. Read and verified the unchanged ordinary WAL/shared tests. The new tests are pure callback/error semantics and five-file AST inspection. No new runtime fault scenario is introduced. Full composition was independently rehashed before any gate, before copying, and after execution without Git commands. Full source-review details and supporting input hashes are in `final-closeout-combined-checks.SOURCE_REVIEW.md`; the preliminary audit returned exit 0.

The five example finalizers retain incomplete/error directories and attempt cleanup only on completed ordinary success. Their delayed result checks run after existing resource defers. A failed removal may be partial, and retained ciphertext does not imply retained credentials. The successful existing WAL/shared paths ran through the ordinary suites. Snapshot, encrypted-cli and native-check have no test files and no main/run(binary) invocation was added.

## Exact environment

Pinned Go 1.27.2 Linux/amd64, `GOTOOLCHAIN=local`, `GOPROXY=off`, `GOSUMDB=off`, `GOFLAGS=-buildvcs=false`, `GOMAXPROCS=2`; existing ROOT/go-path and ROOT/go-cache. CGO0 for unit/vet/build/oracle/cross-build; CGO1 only for full race.

Default `/tmp` retained; GOTMPDIR unset. Resource precheck had 1,422,282,752 available bytes in `/tmp` and 4,636,368,896 in the workspace. No resource relocation, deletion, cleanup workaround or retry occurred. The unchanged WAL example test's original t.TempDir/TMPDIR behavior remains within its existing ordinary test.

Pinned SQLite executable: `toolchains/sqlite-build-3540000/sqlite3`. The unchanged `scripts/test-oracle.sh` guarded source, validated the exact SQLite identity `3.54.0|2026-10-09 15:46:58 be8d059e9a49089ab2dce5ed26dd87aaf598fdc5fbe0b107c0dd758464bcd5e3`, then ran `go test -count=1 -timeout=30m -tags oracle ./...` without a test or package filter.

## Complete stage ledger

- resource-precheck: exit 0; 2026-10-11T04:07:57Z to 2026-10-11T04:07:57Z (0 seconds). Command: `python3 /workspace/scratch/5e06a74e5f65/durable-restart/final-closeout-combined-checks.precheck.py`.
- resource-snapshot: exit 0; 2026-10-11T04:07:57Z to 2026-10-11T04:07:57Z (0 seconds). Command: `bash -c df\ -B1\ /tmp\ \"\$1\"\;\ df\ -i\ /tmp\ \"\$1\"\;\ free\ -b\;\ printf\ \"TMPDIR=%s\\nGOTMPDIR=%s\\n\"\ \"\$\{TMPDIR:-/tmp\ \(default\)\}\"\ \"\$\{GOTMPDIR:-unset\}\" _ /workspace/scratch/5e06a74e5f65/durable-restart`.
- source-identity-before: exit 0; 2026-10-11T04:07:57Z to 2026-10-11T04:08:00Z (3 seconds). Command: `python3 /workspace/scratch/5e06a74e5f65/durable-restart/final-closeout-combined-checks.identity.py before`.
- environment: exit 0; 2026-10-11T04:08:00Z to 2026-10-11T04:08:00Z (0 seconds). Command: `bash -c set\ -e\;\ go\ version\;\ test\ \"\$\(go\ env\ GOVERSION\)\"\ =\ go1.27.2\;\ test\ \"\$\(go\ env\ GOOS\)/\$\(go\ env\ GOARCH\)\"\ =\ linux/amd64\;\ go\ env\ GOVERSION\ GOOS\ GOARCH\ GOTOOLCHAIN\ GOPROXY\ GOSUMDB\ GOPATH\ GOCACHE\ CGO_ENABLED\ GOFLAGS\;\ printf\ \"GOMAXPROCS=%s\\nSQLITE_ORACLE=%s\\n\"\ \"\$GOMAXPROCS\"\ \"\$SQLITE_ORACLE\"\;\ sha256sum\ \"\$1/toolchains/go1.27.2/bin/go\"\ \"\$SQLITE_ORACLE\"\ \"\$REVIEW_REPORT\"\ \"\$1/final-closeout-combined-evidence-v1/manifest.json\"\ \"\$1/final-closeout-combined-checks.sh\"\ \"\$1/final-closeout-combined-checks.identity.py\"\ \"\$1/final-closeout-combined-checks.precheck.py\"\ \"\$1/example-retention-review-evidence-v1/report.md\"\ \"\$1/cli-resources-combined-checks.REPORT.md\" _ /workspace/scratch/5e06a74e5f65/durable-restart`.
- sourceguard-before: exit 0; 2026-10-11T04:08:00Z to 2026-10-11T04:08:01Z (1 seconds). Command: `go run ./internal/testscope`.
- cgo0-unit: exit 0; 2026-10-11T04:08:01Z to 2026-10-11T04:09:55Z (114 seconds). Command: `go test ./... -count=1 -timeout=0`.
- cgo0-vet: exit 0; 2026-10-11T04:09:55Z to 2026-10-11T04:09:57Z (2 seconds). Command: `go vet ./...`.
- cgo0-build: exit 0; 2026-10-11T04:09:57Z to 2026-10-11T04:10:02Z (5 seconds). Command: `go build -trimpath ./...`.
- full-ordinary-oracle: exit 0; 2026-10-11T04:10:02Z to 2026-10-11T04:14:05Z (243 seconds). Command: `./scripts/test-oracle.sh`.
- sourceguard-before-race: exit 0; 2026-10-11T04:14:05Z to 2026-10-11T04:14:05Z (0 seconds). Command: `go run ./internal/testscope`.
- cgo1-race: exit 0; 2026-10-11T04:14:05Z to 2026-10-11T04:22:48Z (523 seconds). Command: `env CGO_ENABLED=1 go test -race ./... -count=1 -timeout=0`.
- cross-build-linux-arm64: exit 0; 2026-10-11T04:22:48Z to 2026-10-11T04:22:56Z (8 seconds). Command: `env CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build ./...`.
- cross-build-darwin-arm64: exit 0; 2026-10-11T04:22:56Z to 2026-10-11T04:23:05Z (9 seconds). Command: `env CGO_ENABLED=0 GOOS=darwin GOARCH=arm64 go build ./...`.
- cross-build-darwin-amd64: exit 0; 2026-10-11T04:23:05Z to 2026-10-11T04:23:14Z (9 seconds). Command: `env CGO_ENABLED=0 GOOS=darwin GOARCH=amd64 go build ./...`.
- cross-build-windows-amd64: exit 0; 2026-10-11T04:23:14Z to 2026-10-11T04:23:23Z (9 seconds). Command: `env CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build ./...`.
- cross-build-windows-arm64: exit 0; 2026-10-11T04:23:23Z to 2026-10-11T04:23:31Z (8 seconds). Command: `env CGO_ENABLED=0 GOOS=windows GOARCH=arm64 go build ./...`.
- module-integrity: exit 0; 2026-10-11T04:23:31Z to 2026-10-11T04:23:32Z (1 seconds). Command: `go mod verify`.
- source-identity-after: exit 0; 2026-10-11T04:23:32Z to 2026-10-11T04:23:35Z (3 seconds). Command: `python3 /workspace/scratch/5e06a74e5f65/durable-restart/final-closeout-combined-checks.identity.py after`.

## Exact aggregate command

```sh
set -o pipefail; set -o noclobber; bash /workspace/scratch/5e06a74e5f65/durable-restart/final-closeout-combined-checks.sh /workspace/scratch/5e06a74e5f65/durable-restart/final-closeout-combined-checks.SOURCE_REVIEW.md 2>&1 | tee /workspace/scratch/5e06a74e5f65/durable-restart/final-closeout-combined-checks.txt
```

## Preserved evidence

Every stage has `.command`, `.started`, `.finished`, `.stdout`, `.stderr` and `.exit`. The full combined transcript is `final-closeout-combined-checks.txt`; terminal status is `.exit`; stages are `.stages.tsv`; machine-readable results and input hashes are `.summary.json`. The executed shell, resource precheck and complete composition/identity script are `.sh`, `.precheck.py`, and `.identity.py`. Full `.inventory-audit/before/after.json` and `.sha256-audit/before/after.txt` are retained. Source inventory and SHA256 files are byte-identical in all three phases. `.evidence-sha256.json` and `.SHA256SUMS` cover the entire evidence prefix, except the integrity indexes themselves.

Preparation included a directory-listing pipeline whose `head` closure caused an incidental `ls: write error: Broken pipe`; no verification gate ran in that command and no failed gate was retried. A missing optional prior-report filename was handled by a read-only listing fallback; the actual prior REVIEW.md and aggregate REPORT.md were then read. These preliminary read diagnostics did not alter source or execute tests.

## Coverage boundaries

No manual tags, new fuzz/soak/benchmarks, native other-OS execution, physical fault injection, process exit/crash, tamper/wrong-key disk test, namespace interference, hot recovery, ANN V4, new 41-target fuzz or unknown/held native-recovery/WAL-tail/backup/planner/backend candidate was run or added. Five target checks were compilation only. No Git, remote, runner or billing action occurred. No frozen/main source edit occurred. The complete preexisting ordinary suites include their preexisting seed/example tests; no fuzz campaign was started. Nonverbose package output does not provide test-level skip accounting.

This is a complete ordinary-gate PASS for the exact frozen tree, not physical-durability, actual failure-fixture-retention, recovery, production-readiness, broader SQLite-support or whole-project/milestone acceptance. Existing HOLD/BLOCKED items remain unchanged. Future source or documentation edits have a different identity and are not silently covered by this tree-specific result.
