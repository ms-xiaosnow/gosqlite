# V3 independent source and compiler-only addendum

Result: PASS for the exact limited source/manual and six authorized AST/selection/compile/symbol stages. The compiler lease was released to the lead immediately after completion. No test binary, gate-off check, unmatched-selector check, -test.list, public Open, physical replay, native namespace/fault workload or fixture generator was executed. Every physical scenario remains NOT_RUN. No main, remote, CI or production source was changed.

## Exact reviewed identity

- Base commit: 56bbcf3febe233197aa0a5eac965c67a67a88011
- Base tree: c31e7dcbeed9732688681741ee38ec60ff0bfb49
- V3 candidate tree: 1755d67431f9de082449b08fa24b26b2b35eb612
- Candidate source: public-recovery-harness-work-v3
- Go source SHA256: dd5a63e2a8e7d4fdd969ba03273a8f8147d5eff81babf8fce44a5228c5cdaf5e (10657 bytes)
- Manual SHA256: a49ae1882c4a5dcb8c58860fa8df3b011f88c9ee99963a5721831049477ec4d2 (9711 bytes)
- Compiled, never-executed test binary SHA256: a7a9a4813ec9550708ff2d2addc97d957b686e48ec16aa9b8e2c94710a0c226e (14757553 bytes)

The 3038-file candidate differs from the 3036-file base only by the two approved additions. Independent Python Git blob/tree hashing reproduced the known base tree and calculated the candidate tree without writing Git objects or refs. Full pre/post candidate byte/size/mode inventories match. Original V1 and V2 complete inventories remain unchanged. V1 CHANGES_REQUIRED and V2 source PASS remain separate historical reviews; this addendum does not rewrite either.

## V3 source delta review

Only docs/public-rollback-manual-zh.md differs from V2. Its Go source is byte-identical to reviewed V2.

1. Manual:33–35 moves existing local/offline toolchain controls and manual-gate unsets ahead of the first Go invocation. This resolves V1/V2 nonblocking C1.
2. Manual:43–44 replaces the archive pipeline with `git archive` to a retained source.tar followed by separate `tar -xf`. Under set -e a failed archive or extraction stops; successful tar can no longer mask an archive failure. No archive or extraction command was executed during this review.

All V2 blocker resolutions are retained: fail-closed exact source-selection counts, deterministic nonfatal backup plus manifest verification, failed-subtest stop, one-attempt Close with stop before reopen, and final read-only pending diagnostics. No new source defect was found.

All three V3 shell blocks passed bash -n. Their embedded Python snippets parsed with ast.parse; only the explicitly authorized compiler file-selection text checker was subsequently executed. No runtime-evidence collector was executed.

## Preflight and environment

Preflight recorded at 2026-10-11T08:58:28.653278+00:00:

- /tmp: 1413267456 available bytes, 1243461 available inodes, writable; required >=1 GiB and >=10000 inodes.
- Workspace: 11326042112 available bytes, 1161008 available inodes, writable; required >=2 GiB and >=10000 inodes.
- Host Linux/x86_64; build target explicitly linux/amd64, CGO_ENABLED=0.
- Pinned Go path: toolchains/go1.27.2/bin/go. Static VERSION reads go1.27.2, time 2026-10-02T20:28:03Z. Binary SHA256 548d3c32f83fa81152f543eb1526be7884cfe0526eb5e9ce29d3eca14786e7e0 matches historical independently recorded local executable evidence.
- Local/offline controls were installed before all Go invocations: GOTOOLCHAIN=local, GOENV=off, GOWORK=off, GOPROXY=off, GOSUMDB=off, GOFLAGS=-buildvcs=false, GOMAXPROCS=2, CGO_ENABLED=0, GOOS=linux, GOARCH=amd64.
- GOPATH points to ROOT/go-path and GOCACHE to ROOT/go-cache. Existing module/cache directories were verified. TMPDIR remains /tmp; GOTMPDIR and every GOSQLITE_MANUAL_* environment value were unset for the child commands. No cache relocation, manual cache deletion or temporary-storage modification occurred.
- No inherited custom Go proxy/privacy/insecure override was present. go.mod/go.sum were unchanged; no replace directive is present; required existing cache directories correspond to x/crypto v0.58.0, x/sys v0.49.0 and x/text v0.43.0. This was static pin/cache inspection, not an unapproved `go mod verify` run.
- No Go version, env, gofmt, broad build/test or other seventh Go invocation was performed.

## Six authorized Go stages actually executed

All returned natural exit 0, with empty stderr. Exact argv, cwd, start/finish UTC and separate stdout/stderr are in compiler/commands.json and its referenced files.

1. AST inspector: 08:58:28.672856–08:58:29.201815 UTC. The approved standard-library-only inspector was run with the candidate path as text. It verified the exact build constraint truth table, first-statement environment refusal, unique test entrypoint, no package variable/init/TestMain hook, four ordered fail-stop scenario calls, five public Open references, and no cleanup retry/deletion call. It did not import or execute the candidate package.
2. Default go list: 08:58:29.202255–08:58:29.287199 UTC.
3. Oracle-only go list: 08:58:29.287603–08:58:29.344943 UTC.
4. manual_recovery go list: 08:58:29.345286–08:58:29.394850 UTC.
5. `go test -c -p=1 -vet=off -tags manual_recovery -o <evidence>/public-recovery.test ./sqlite`: 08:58:29.427741–08:58:37.838217 UTC. Compiled and linked only; did not execute.
6. `go tool nm <evidence>/public-recovery.test`: 08:58:37.839346–08:58:37.962572 UTC. Read symbol metadata only.

The separately authorized Python selection checker ran between stages 4 and 5, exited 0 and required exact counts default=0, oracle=0, manual=1. The subsequent text-only symbol check found exactly one symbol named github.com/ms-xiaosnow/gosqlite/sqlite_test.TestManualPublicRollbackRecovery, type T. Runtime scenarios remain NOT_RUN; compiler stages are not four physical PASS results.

The result and post-inventory record completed at 08:58:38.246326 UTC; the lead received immediate compiler-lease release notification. No further Go commands followed.

## Evidence index

- exact-tree-identity.json: independently derived Git identity and two-path-only delta.
- preflight.json: static tool/module/environment/resource facts.
- original-source/: exact V3 source/manual copies.
- v2-to-v3.patch: complete documentation-only correction.
- manual-block-1.sh through manual-block-3.sh and manual-syntax.json: extracted unexecuted shell and parse checks.
- pre-review-inventory.json / post-review-inventory.json: complete matching candidate inventory.
- compiler/commands.json: all six Go invocations plus the one text-only Python checker, exact argv and UTC/exit evidence.
- compiler/*.stdout, *.stderr, *-files.txt, symbols.txt: raw outputs.
- compiler/exact-symbol.json and artifact-identity.json: text-only linkage proof and binary/source hashes.
- compiler/public-recovery.test: preserved compiled artifact; NEVER EXECUTED.
- result.json: natural completion, unchanged source/history and explicit runtime NOT_RUN.
- evidence-sha256.json: hashes every V3 evidence file except itself.
- ../all-reviewed-evidence-v3-sha256.json: complete review-directory evidence hash manifest, excluding itself; includes V1/V2 history and the exact external runner.

No remaining source-review blocker was found for this limited harness/manual. This establishes syntax, isolation selection and linkage only. It does not authorize or establish real recovery, crash durability, native failures, driver recovery, other platforms, M3/M10 or overall project acceptance.
