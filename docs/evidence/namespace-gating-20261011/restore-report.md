# Namespace checkpoint exact Library restore verification

## Verdict

NARROW RESTORE VERIFICATION PASS. The exact Library-restored checkpoint at HEAD `50b627e11e9dbce7f2fde8df35c4634c52a06d52`, tree `8f2709293287a3517a52b3f989888ded8230ef53`, independently passed the original authorized 207-command scope. The last Go subprocess naturally completed at 2026-10-11T09:14:09.133064+00:00; there are no remaining Go commands from this reviewer. The compiler lease was promptly returned to the lead.

This is source/model, selection, and cross-compilation evidence. It is not full ordinary-suite, oracle-runtime, native namespace, physical durability, or overall project acceptance. All six foreign/manual binaries were inspected as data and NEVER EXECUTED. Every other restricted scenario remains NOT_RUN. No remote or CI operation was performed; CI 38127196102 is tracked separately by the parent.

## Exact restore identity

- Library bundle identity supplied by the lead: `libfile_e5dcbc0ac16c8191bce977aa165163f9`. This reviewer rehashed its already-materialized local bytes: 5,755,098 bytes, SHA-256 `111d9cc34e9d011c10ab9612010a5c7c3b4841900bc199fc6f1a4e7af30faaec`.
- Lead previously completed Library readback/cmp, bundle verification, clone/fsck, and HEAD/tree/goal checks. Those are prior lead evidence; this reviewer did not repeat Library materialization, clone, bundle verify, or fsck and does not claim those actions as new work.
- All 3,077 file paths, byte counts, SHA-256 values, and Git modes match `namespace-final-publication-inventory.json`; independent Git-object reconstruction gives exactly the publication tree. Restored Git HEAD, index entries and refs were read with optional locks disabled. Index paths, modes and blob identities exactly match the complete restored filesystem.
- All 788 non-doc paths have the same complete bytes, blob identities, and Git modes as both `namespace-combined-freeze-v1` (tree `697e45c6aba7a5cf62ebbf2bab2662f10903c5be`) and `namespace-gating-integration-freeze-v1` (tree `01918609a55cc149c5155451da40f7389854db06`). The current-P0 comparison base remains tree `442a7104dc780ddb20826e5ee4f1250b291f2b24`.
- Full restored, combined, source and base inventories were recorded before execution, after execution, and at report finalization. Each includes all file bytes/hashes, paths, Git/POSIX modes, the root and all subdirectory POSIX modes. All remained identical. The restored checkout contains 145 subdirectories.
- The complete restored `.git` metadata inventory includes 26 files and 12 directories, preserving bytes, POSIX modes and mtimes. It stayed unchanged across preparation, execution and finalization; this covers index, HEAD, refs, reflogs, configuration and Git object metadata. Access times are intentionally excluded from a read-only comparison.

## Git permissions versus POSIX clone normalization

A Git clone restores Git's executable/non-executable distinction; it does not preserve arbitrary owner-write and group/other-read permission bits. There are exactly 30 publication-to-clone POSIX differences: {'0o444 -> 0o644': 2, '0o600 -> 0o644': 28}. All 30 retain Git mode `100644` and exact bytes. No mode was changed by this reviewer. `clone-posix-normalization.json` lists every path. The restore's own actual POSIX modes were then held exactly invariant throughout verification; publication POSIX modes are not falsely described as restored exactly.

## Fresh authorized checks

The explicit compiler release from the lead was received before any Go invocation. `compiler-command-plan.json` was derived from the original independent review's exact 207 argv records. Finalization compared all 207 actual argv and per-command extra environments with those records, allowing only restored cwd and reviewer-owned artifact-output paths to change.

1. Pinned `go version` and explicit `go env -json` readbacks.
2. Whole-tree ordinary source-reference guard.
3. `go test -count=1 -parallel=2 -timeout=0 ./internal/testscope`.
4. The same pure model package with `-race`; CGO=1 only here.
5. `go run ./internal/testscope` with `GOFLAGS=-buildvcs=false -p=1 -tags=manual_namespace`: expected exit 1 and exact manual-tag refusal text confirmed.
6. Pinned `go tool dist list`: 47 unique targets.
7. 47 targets × default/oracle/manual_namespace/oracle,manual_namespace = 188 fresh `go list -e -json -mod=readonly` selections. No package Error occurred. For every row, selected files and test declarations were checked against the current base: default/oracle omit exactly the applicable moved declaration; all other existing declarations are retained; manual tags restore exactly the applicable moved declaration once. Existing Darwin/iOS selection is preserved; Windows remains windows/amd64.
8. Six fresh `go test -c -mod=readonly` compilations for windows/amd64, darwin/amd64 and darwin/arm64, each under manual_namespace and oracle,manual_namespace, followed by six `go tool nm` inspections. Each binary contains exactly one full expected platform test-function symbol and no opposite-platform moved-test symbol. Artifact bytes, hashes, complete nm output and exact selected symbols are retained.

The pure package has the same four source/model declarations proved by the immutable source comparison: `TestIndependentReviewGuardDirectAndAliasLimit`, `TestScopeSourceReferenceGuard`, `TestNativeNamespaceMethodReferenceGuard`, `TestScopeRecoveryOptionTypeChains`. The original non-verbose test argv were deliberately preserved; logs establish fresh package PASS, not invented per-test RUN/PASS lines. These tests use inert source strings/parser checks and are not namespace physical execution.

All actual Go argv, UTC start/end timestamps, elapsed times, exits, configured environments, output paths, lengths and SHA-256 values are in `compiler-checks/command-results.json`. Exactly one command returned exit 1: the explicitly expected refusal, command 006. All other commands returned 0; the overall runner naturally returned 0. Last Go completion: 2026-10-11T09:14:09.133064+00:00.

## Environment and resource controls

Pinned executable: `/workspace/scratch/5e06a74e5f65/durable-restart/toolchains/go1.27.2/bin/go`. GOTOOLCHAIN=local; GOPROXY=off; GOSUMDB=off; GOENV=off; GOWORK=off; original ROOT/go-cache and ROOT/go-path; GOMAXPROCS=2; GOFLAGS normally `-buildvcs=false -p=1`. Default `/tmp` is used with TMPDIR=/tmp and GOTMPDIR unset. Both manual environment variables were actually UNSET in every subprocess, not enabled or inherited. CGO=0 except command 005's pure-model race run. All subprocesses ran sequentially.

Each of 207 subprocesses had a fresh resource gate: /tmp ≥1 GiB, workspace ≥2 GiB, and ≥10,000 available inodes at both locations. Actual minima: /tmp 1,413,300,224 bytes and 1,243,469 inodes; workspace 10,990,202,880 bytes and 1,153,863 inodes. No threshold reduction, alternate cache/temp, download, cleanup, automatic watchdog, timeout termination, signal, retry or fallback was used.

## Preparation stop and correction

The first Python-only preflight stopped at an unnecessary assertion that the mutable main checkout equal the published restore. No Go command had run. The lead then explicitly clarified that main was undergoing a separately authorized next integration and must not be an invariant for this task. The failed script, log, exit and inventories are preserved in `attempt1-main-equality-stop/`; `phase1-revision.json` records the correction. Only the mutable-main scan/equality/Git checks were removed. The restored checkout and fixed namespace/base inputs retained all checks. No source defect was inferred from this known unrelated main change.

## Evidence and boundaries

- `phase1-summary.json`, publication readback hashes, pure model declaration inventory and the exact compiler plan.
- `compiler-release.json`, `compiler-slot-released.txt`, `checks.log`, `checks.exit`, `runner.exit`.
- `compiler-checks/command-results.json` and 207 per-command logs.
- `compiler-checks/selection-matrix.json`, `selection-summary.json`, `foreign-compile-results.json` and six unexecuted artifacts.
- Complete phase-1/before/after/final file and directory inventories, restored Git metadata snapshots, and `final-identity.json`.
- `prepare.py`, `verify.py`, `finalize.py`: review-owned checks, not new project runners.
- `EVIDENCE-MANIFEST.json`: hash/length inventory of the final evidence, excluding itself.

No main/source/frozen repository files, Git state, remote, CI or Library content were edited. Writes were limited to this evidence directory and the approved existing Go caches. All Go checks used the exact restored checkpoint rather than an earlier source copy. Source-guard limitations from the original review remain: it is not full call-graph analysis or a proof of physical safety.
