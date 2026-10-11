# LIMIT combined ordinary aggregate

Result: PASS. Aggregate exit 0.

Exact tree: `ca8947811d60266c19db2d6b155bd62b843d6e48`. Baseline commit: `63c2ebb10e3a2c4ff2378ee9f33c46133134c70f`.

2,965 regular files; 142 directories; 362 test files (360 preserved plus 2 reviewed new files); 779 non-doc files exactly match reviewed owner candidate `3fc9087ad121d423174a658e118ac455d1ace8c2`.

Started 2026-10-11T07:23:25Z; last completed stage 2026-10-11T07:39:21Z. 18/18 stages completed. No pending stage is counted as PASS.

## Gates

- resource-precheck: exit 0; 2026-10-11T07:23:25Z to 2026-10-11T07:23:25Z (0s). Command: `python3 /workspace/scratch/5e06a74e5f65/durable-restart/limit-combined-independent-v1/precheck.py`
- resource-snapshot: exit 0; 2026-10-11T07:23:25Z to 2026-10-11T07:23:25Z (0s). Command: `bash -c df\ -B1\ /tmp\ \"\$1\"\;\ df\ -i\ /tmp\ \"\$1\"\;\ free\ -b\;\ printf\ \"TMPDIR=%s\\nGOTMPDIR=%s\\n\"\ \"\$\{TMPDIR:-/tmp\ \(default\)\}\"\ \"\$\{GOTMPDIR:-unset\}\" _ /workspace/scratch/5e06a74e5f65/durable-restart`
- source-identity-before: exit 0; 2026-10-11T07:23:25Z to 2026-10-11T07:23:28Z (3s). Command: `python3 /workspace/scratch/5e06a74e5f65/durable-restart/limit-combined-independent-v1/identity.py before`
- environment: exit 0; 2026-10-11T07:23:28Z to 2026-10-11T07:23:28Z (0s). Command: `bash -c set\ -e\;\ go\ version\;\ test\ \"\$\(go\ env\ GOVERSION\)\"\ =\ go1.27.2\;\ test\ \"\$\(go\ env\ GOOS\)/\$\(go\ env\ GOARCH\)\"\ =\ linux/amd64\;\ go\ env\ GOVERSION\ GOOS\ GOARCH\ GOTOOLCHAIN\ GOPROXY\ GOSUMDB\ GOPATH\ GOCACHE\ CGO_ENABLED\ GOFLAGS\;\ printf\ \"GOMAXPROCS=%s\\nSQLITE_ORACLE=%s\\n\"\ \"\$GOMAXPROCS\"\ \"\$SQLITE_ORACLE\"\;\ sha256sum\ \"\$1/toolchains/go1.27.2/bin/go\"\ \"\$SQLITE_ORACLE\"\ \"\$EVIDENCE/run.sh\"\ \"\$EVIDENCE/identity.py\"\ \"\$EVIDENCE/precheck.py\"\ \"\$CONFIG\"\ \"\$EVIDENCE/SOURCE_REVIEW.md\" _ /workspace/scratch/5e06a74e5f65/durable-restart`
- sourceguard-before: exit 0; 2026-10-11T07:23:28Z to 2026-10-11T07:23:28Z (0s). Command: `go run ./internal/testscope`
- cgo0-unit: exit 0; 2026-10-11T07:23:28Z to 2026-10-11T07:25:25Z (117s). Command: `go test ./... -count=1 -timeout=0`
- cgo0-vet: exit 0; 2026-10-11T07:25:25Z to 2026-10-11T07:25:27Z (2s). Command: `go vet ./...`
- cgo0-build: exit 0; 2026-10-11T07:25:27Z to 2026-10-11T07:25:35Z (8s). Command: `go build -trimpath ./...`
- full-ordinary-oracle: exit 0; 2026-10-11T07:25:35Z to 2026-10-11T07:29:32Z (237s). Command: `./scripts/test-oracle.sh`
- sourceguard-before-race: exit 0; 2026-10-11T07:29:32Z to 2026-10-11T07:29:32Z (0s). Command: `go run ./internal/testscope`
- cgo1-race: exit 0; 2026-10-11T07:29:32Z to 2026-10-11T07:38:33Z (541s). Command: `env CGO_ENABLED=1 go test -race ./... -count=1 -timeout=0`
- cross-build-linux-arm64: exit 0; 2026-10-11T07:38:33Z to 2026-10-11T07:38:43Z (10s). Command: `env CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build ./...`
- cross-build-darwin-arm64: exit 0; 2026-10-11T07:38:43Z to 2026-10-11T07:38:52Z (9s). Command: `env CGO_ENABLED=0 GOOS=darwin GOARCH=arm64 go build ./...`
- cross-build-darwin-amd64: exit 0; 2026-10-11T07:38:52Z to 2026-10-11T07:39:01Z (9s). Command: `env CGO_ENABLED=0 GOOS=darwin GOARCH=amd64 go build ./...`
- cross-build-windows-amd64: exit 0; 2026-10-11T07:39:01Z to 2026-10-11T07:39:10Z (9s). Command: `env CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build ./...`
- cross-build-windows-arm64: exit 0; 2026-10-11T07:39:10Z to 2026-10-11T07:39:18Z (8s). Command: `env CGO_ENABLED=0 GOOS=windows GOARCH=arm64 go build ./...`
- module-integrity: exit 0; 2026-10-11T07:39:18Z to 2026-10-11T07:39:19Z (1s). Command: `go mod verify`
- source-identity-after: exit 0; 2026-10-11T07:39:19Z to 2026-10-11T07:39:21Z (2s). Command: `python3 /workspace/scratch/5e06a74e5f65/durable-restart/limit-combined-independent-v1/identity.py after`

## Count and identity evidence

- cgo0-unit: 37 successful test packages; 11 packages report no test files; 0 FAIL lines.
- full-ordinary-oracle: 38 successful test packages; 11 packages report no test files; 0 FAIL lines.
- cgo1-race: 37 successful test packages; 11 packages report no test files; 0 FAIL lines.

Non-verbose aggregate output does not enumerate leaf assertions. The prior dedicated review separately establishes 1,016 native and 344 fixed-oracle leaves passing normal/race plus 118 independent controls. The additional 118 controls are not integrated and were not rerun here. No leaf count is invented from package-only output.

Full before/after source inventory equality: True. Inventories cover every byte, path, length, Git blob/tree, executable mode, actual POSIX mode, all directories and root mode. No source was edited by this verifier. Exact eight-path composition, unchanged scripts/workflows/guard/modules/goal, all 360 prior tests and 779 reviewed-owner non-doc files were rechecked.

Full per-stage command, stdout, stderr, timestamps and exit files, aggregate.stdout/stderr, checks.stages.tsv, approved-config.json, run.sh/precheck.py/identity.py, SOURCE_REVIEW.md and complete SHA256 inventories are preserved in this evidence directory. SUMMARY.json contains the same machine-readable facts.

## Scope and limits

Existing Linux ordinary full suite plus the two reviewed LIMIT conversion files. Complete current oracle script. Five foreign-target production builds only. No new campaigns or restricted physical workflows; no wider completion or safety claim.

Default /tmp was retained. The resource precheck required at least 1 GiB in /tmp and 2 GiB in workspace, at least 10,000 available inodes and writable cache/module directories. No cleanup/relocation/retry or kill/cancel/watchdog was used. Oracle retains only its existing script timeout. No Git, remote, CI, new runner provisioning, source changes or new input was performed.

The manual recovery and namespace suites stayed excluded; known Windows/Darwin raw namespace tests stayed excluded by OS constraints on this Linux host. Cross-platform go build commands do not run native code or tests. Physical durability, recovery, fault/tamper, broader SQL semantics, old planner integration, ANN V4, 41-target fuzz and whole-project readiness remain outside this result.
