# Public rollback Open combined ordinary aggregate

Result: PASS. Aggregate exit 0.

Exact tree: `442a7104dc780ddb20826e5ee4f1250b291f2b24`. Baseline commit: `c1502790c4b233757b0f9a3c4d5e5bdee1321310`.

3,004 regular files; 143 directories; 366 test files (360 preserved, 2 reviewed existing edits, 4 new source/model files); 785 non-doc files; exactly 13 changed source paths match reviewed owner candidate `75c074548e8b5208beb06e6ead314d835cd16f4f`.

Started 2026-10-11T07:59:40Z; last completed stage 2026-10-11T08:22:39Z. 18/18 stages completed. No pending stage is counted as PASS.

## Gates

- resource-precheck: exit 0; 2026-10-11T07:59:40Z to 2026-10-11T07:59:40Z (0s). Command: `python3 /workspace/scratch/5e06a74e5f65/durable-restart/public-open-combined-independent-v1/precheck.py`
- resource-snapshot: exit 0; 2026-10-11T07:59:40Z to 2026-10-11T07:59:40Z (0s). Command: `bash -c df\ -B1\ /tmp\ \"\$1\"\;\ df\ -i\ /tmp\ \"\$1\"\;\ free\ -b\;\ printf\ \"TMPDIR=%s\\nGOTMPDIR=%s\\n\"\ \"\$\{TMPDIR:-/tmp\ \(default\)\}\"\ \"\$\{GOTMPDIR:-unset\}\" _ /workspace/scratch/5e06a74e5f65/durable-restart`
- source-identity-before: exit 0; 2026-10-11T07:59:40Z to 2026-10-11T07:59:41Z (1s). Command: `python3 /workspace/scratch/5e06a74e5f65/durable-restart/public-open-combined-independent-v1/identity.py before`
- environment: exit 0; 2026-10-11T07:59:41Z to 2026-10-11T07:59:41Z (0s). Command: `bash -c set\ -e\;\ go\ version\;\ test\ \"\$\(go\ env\ GOVERSION\)\"\ =\ go1.27.2\;\ test\ \"\$\(go\ env\ GOOS\)/\$\(go\ env\ GOARCH\)\"\ =\ linux/amd64\;\ go\ env\ GOVERSION\ GOOS\ GOARCH\ GOTOOLCHAIN\ GOPROXY\ GOSUMDB\ GOPATH\ GOCACHE\ CGO_ENABLED\ GOFLAGS\;\ printf\ \"GOMAXPROCS=%s\\nSQLITE_ORACLE=%s\\n\"\ \"\$GOMAXPROCS\"\ \"\$SQLITE_ORACLE\"\;\ sha256sum\ \"\$1/toolchains/go1.27.2/bin/go\"\ \"\$SQLITE_ORACLE\"\ \"\$EVIDENCE/run.sh\"\ \"\$EVIDENCE/identity.py\"\ \"\$EVIDENCE/precheck.py\"\ \"\$CONFIG\"\ \"\$EVIDENCE/SOURCE_REVIEW.md\" _ /workspace/scratch/5e06a74e5f65/durable-restart`
- sourceguard-before: exit 0; 2026-10-11T07:59:41Z to 2026-10-11T07:59:42Z (1s). Command: `go run -p=1 ./internal/testscope`
- cgo0-unit: exit 0; 2026-10-11T07:59:42Z to 2026-10-11T08:02:26Z (164s). Command: `go test -p=1 ./... -count=1 -timeout=0`
- cgo0-vet: exit 0; 2026-10-11T08:02:26Z to 2026-10-11T08:02:30Z (4s). Command: `go vet -p=1 ./...`
- cgo0-build: exit 0; 2026-10-11T08:02:30Z to 2026-10-11T08:02:44Z (14s). Command: `go build -p=1 -trimpath ./...`
- full-ordinary-oracle: exit 0; 2026-10-11T08:02:44Z to 2026-10-11T08:07:09Z (265s). Command: `./scripts/test-oracle.sh`
- sourceguard-before-race: exit 0; 2026-10-11T08:07:09Z to 2026-10-11T08:07:09Z (0s). Command: `go run -p=1 ./internal/testscope`
- cgo1-race: exit 0; 2026-10-11T08:07:09Z to 2026-10-11T08:19:51Z (762s). Command: `env CGO_ENABLED=1 go test -p=1 -race ./... -count=1 -timeout=0`
- cross-build-linux-arm64: exit 0; 2026-10-11T08:19:51Z to 2026-10-11T08:20:24Z (33s). Command: `env CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build -p=1 ./...`
- cross-build-darwin-arm64: exit 0; 2026-10-11T08:20:24Z to 2026-10-11T08:20:58Z (34s). Command: `env CGO_ENABLED=0 GOOS=darwin GOARCH=arm64 go build -p=1 ./...`
- cross-build-darwin-amd64: exit 0; 2026-10-11T08:20:58Z to 2026-10-11T08:21:30Z (32s). Command: `env CGO_ENABLED=0 GOOS=darwin GOARCH=amd64 go build -p=1 ./...`
- cross-build-windows-amd64: exit 0; 2026-10-11T08:21:30Z to 2026-10-11T08:22:04Z (34s). Command: `env CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build -p=1 ./...`
- cross-build-windows-arm64: exit 0; 2026-10-11T08:22:04Z to 2026-10-11T08:22:37Z (33s). Command: `env CGO_ENABLED=0 GOOS=windows GOARCH=arm64 go build -p=1 ./...`
- module-integrity: exit 0; 2026-10-11T08:22:37Z to 2026-10-11T08:22:37Z (0s). Command: `go mod verify`
- source-identity-after: exit 0; 2026-10-11T08:22:37Z to 2026-10-11T08:22:39Z (2s). Command: `python3 /workspace/scratch/5e06a74e5f65/durable-restart/public-open-combined-independent-v1/identity.py after`

## Count and identity evidence

- cgo0-unit: 37 successful test packages; 11 packages report no test files; 0 FAIL lines.
- full-ordinary-oracle: 38 successful test packages; 11 packages report no test files; 0 FAIL lines.
- cgo1-race: 37 successful test packages; 11 packages report no test files; 0 FAIL lines.

Non-verbose aggregate output does not enumerate test leaves or skips. This report counts successful packages only; the earlier selected public-recovery source/model results remain separate evidence. The complete unchanged LIMIT native/oracle regressions remain included. No test-leaf count or no-skip claim is inferred from package-only output.

Full before/after source inventory equality: True. Inventories cover every byte, path, length, Git blob/tree, executable mode, actual POSIX mode, all directories and root mode. No source was edited by this verifier. Exact thirteen-source-path composition, unchanged scripts/workflows/modules/goal, all 360 unchanged prior tests, both reviewed old test edits, all four new test files, and retained LIMIT source/tests were rechecked. The source guard is among the reviewed changed files. The 13 changed paths exactly match the independently reviewed source owner; the other frozen source remains baseline-identical by byte and Git mode.

Full per-stage command, stdout, stderr, timestamps and exit files, aggregate.stdout/stderr, checks.stages.tsv, approved-config.json, run.sh/precheck.py/identity.py, SOURCE_REVIEW.md and complete SHA256 inventories are preserved in this evidence directory. SUMMARY.json contains the same machine-readable facts.

## Preserved preparation diagnostics

The initial report SHA mismatch was caught before any Go execution. Exact reconstructed old bytes and pinned final bytes establish that finalization removed duplicated environment wording and reflowed one sentence (-15 bytes), with no changed verdict or code/test result. Both finalization evidence and original mismatch remain. Original source-integration.json retains its historically correct old report hash.

An external preparation checker initially required baseline Git-archive POSIX permissions to equal preserved frozen permissions. It stopped on a fixture mode difference. The complete delta is 30 files and 2 directories, with identical file bytes/Git modes. The checker was corrected to record this archive distinction while preserving all frozen/copy/audit/before/after POSIX checks; no source permission changed. The failed checker version and exact correction record remain.

Neither diagnostic was a resource failure, test failure, denied action or source defect. No runtime stage was retried.

## Scope and limits

Existing Linux ordinary full suite with nil recovery policy, including retained LIMIT regressions and four new source/memory-only public-recovery test files. Complete unchanged oracle script. Five cross-target production builds only. Actual recovery and recovery-enabled real Open/Connect remain BLOCKED / NOT_RUN; no wider completion or physical safety claim.

The exclusive compiler slot was released by the parent before any Go execution. Default /tmp was retained. The resource precheck required at least 1 GiB in /tmp and 2 GiB in workspace, at least 10,000 available inodes and writable cache/module directories. No cleanup/relocation/retry or kill/cancel/watchdog was used. Oracle retains only its existing script timeout. No Git, remote, CI, new runner provisioning, source changes or new input was performed.

The manual recovery and namespace suites stayed excluded; known Windows/Darwin raw namespace tests stayed excluded by OS constraints on this Linux host. Cross-platform go build commands do not run native code or tests. Physical durability, recovery, fault/tamper, broader SQL semantics, old planner integration, ANN V4, 41-target fuzz and whole-project readiness remain outside this result.
