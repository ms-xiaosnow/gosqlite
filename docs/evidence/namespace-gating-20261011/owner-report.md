# Current-P0 namespace gating integration

Status: PASS for exact six-path source integration, replay identity, source guard/model/race checks, all 188 selection cases, and all six foreign compilations with symbols. Checks naturally exited 0 at 2026-10-11 08:23:50 UTC after explicit compiler-slot release. All manual/native physical runtime remains NOT_RUN.

## Exact source identity

- Read-only base: `public-open-combined-freeze-v1`, tree `442a7104dc780ddb20826e5ee4f1250b291f2b24`, 3,004 files.
- Worker-owned candidate: `namespace-gating-integration-work`.
- Frozen no-.git candidate: `namespace-gating-integration-freeze-v1`, tree `01918609a55cc149c5155451da40f7389854db06`, 3,007 files and 143 directories excluding the root directory.
- Six-path patch: `source.patch`, SHA-256 `18ba34e90283080cbc0a1ab1ab1e213945d2b9a05168acb33221498faaba4ee0`.
- Accepted historical patch SHA-256 was verified as `65d746d7db22a9a2abfc3450af5220d571a953bd84679ac0c3ab9f0eb581de65`. Its three added source files are retained exactly; current files were merged minimally, not replaced with an old guard.
- Independent Git-object hashing, full path/byte/SHA-256 inventory, Git modes, and POSIX modes verify exact base-to-candidate replay. The same inventory verifies the freeze. Root and subdirectory POSIX modes are included. The base is unchanged.

Only these paths differ:

1. `internal/encstore/native_windows_test.go`
2. `internal/encstore/native_darwin_test.go`
3. `internal/encstore/namespace_manual_windows_amd64_test.go` (added)
4. `internal/encstore/namespace_manual_darwin_test.go` (added)
5. `internal/testscope/main.go`
6. `internal/testscope/native_namespace_test.go` (added)

No production implementation, project documentation, CI, runner, billing, Git commit, or remote was changed.

## Function preservation and test selection

The complete original functions remain byte-identical after removing only their newly inserted first-statement environment gate. Both require `GOSQLITE_MANUAL_NAMESPACE` to equal `independently_authorized` before reaching TempDir or a native operation. The sentinel is a source safeguard, never permission to run restricted work.

Exact manual entry points are:

- `TestWindowsEncryptedRelativePublicationAndRetainedRoot`, in `namespace_manual_windows_amd64_test.go`.
- `TestDarwinEncryptedRelativePublicationAndParentLifetime`, in `namespace_manual_darwin_test.go`.

Both new files use the exact first-line build constraint `//go:build manual_namespace`. Windows/amd64 selection is retained by its filename. Darwin selection is retained by its filename, including Go's existing iOS/Darwin relationship. The completed 47-target, four-tag-set matrix independently verifies actual selection, with no package errors: default/oracle exclude both moved functions everywhere; manual selection exactly follows the old file platform selection.

Default and oracle suites intentionally lose exactly these two native namespace tests on their applicable platforms. Their assertions were moved intact rather than weakened. The remaining content of each original test file is unchanged except unused imports and one separating blank line. Windows retains the `context` import because the current `windowsForbiddenProvider` and `TestWindowsEncryptedSharedRekeyPreflightMemory` need it. Blind application of the old patch's import-removal hunk would have regressed this current source.

Of the base's 366 test files, 364 remain exactly byte/mode identical. Only the two named native test files were modified, with every other existing function in them retained byte-for-byte. Three test files were added. Function hashes and complete source proofs are in `body-equivalence.json`.

## Recovery-guard coexistence

The sole guard implementation change appends `Rename` and `rename` to the existing generic selector-name switch. Reversing exactly that substitution reproduces every byte of current P0 `internal/testscope/main.go`. All current recovery name references, unkeyed Options checks, same-file alias propagation, dot-import handling, and existing regression files remain untouched.

The accepted inert namespace fixture covers calls, captured method values, generic Rename selectors, and exact manual-tag exemption. Separate reviewer probes cover method expressions, chained receivers, defer/go, aliases, rejected OR constraints, comments/strings, and the explicitly acknowledged bare-helper limitation.

This remains a conservative source-reference prevention aid, not full type/call-graph analysis, a sandbox, or a physical-safety proof. Bare helper calls, differently named wrappers, reflection/dynamic calls, and future APIs still require behavioral review. Exact manual-tag exemption does not itself prove a gated body is safe; the full-body/gate proof is recorded separately.

## Checks and hard boundary

Preparation used only source reading, Python identity computation, owned-directory copying, and Git patch check/replay. No Go command ran before the parent explicitly released the compiler slot after the separate P0 aggregate naturally exited 0. The release is retained in `compiler-slot-release.txt`.

After explicit slot release, all completed checks used pinned Go 1.27.2; GOTOOLCHAIN=local; GOPROXY/GOSUMDB off; existing ROOT/go-cache and ROOT/go-path; GOMAXPROCS=2 and -p=1; default /tmp; GOTMPDIR unset. Every invocation requires /tmp free bytes >=1 GiB, workspace free bytes >=2 GiB, and free inodes >=10,000 on each. There is no cleanup, relocation, or threshold-lowering fallback.

Completed checks: six-file gofmt check (no rewrite); whole-tree source guard; pure AST/source-string tests and race instrumentation; expected refusal when GOFLAGS contains the manual tag; additional inert reviewer source probes with ordinary and race instrumentation; 47 supported targets × default/oracle/manual_namespace/oracle+manual_namespace source listing; six foreign test compilations and symbol reads for windows/amd64, darwin/amd64, darwin/arm64 with the two manual tag combinations. All PASS. Foreign test binaries were never executed. The environment gate remained empty. No automatic test timeout or subprocess timeout was installed; the wrapper finished naturally.

No native namespace, native failure, crash, kill, recovery, disk corruption/tamper, wrong-key, interrupted-publication, ANN V4, new 41-target fuzz, or unknown-trigger workflow ran. Current P0 aggregate results are separate from this narrow source-gating integration and cannot be claimed here.

## Documentation proposal for lead composition

After independent review and actual integration into the advancing main tree, replace statements saying these two raw namespace tests remain ungated with the exact integrated identity, moved entry points above, both source gates, default-selection reduction, and compiler-only evidence. Keep every native/manual physical gate NOT_RUN. This report intentionally makes no project-document edits outside the authorized six paths.

## Final evidence and resource checks

- `checks.exit`: 0; `checks.log`: full sequential log; `command-results.json`: all 210 command records. Only the explicitly expected manual-GOFLAGS rejection has exit 1.
- `check-summary.json`, `selection-matrix.json`, `foreign-compile-results.json`, complete per-binary `.nm.txt` outputs and filtered `.symbols.json` outputs record the exact results.
- `post-check-identity.json` independently rehashes unchanged base, work, freeze, and replay trees. Full byte and POSIX/Git-mode inventories still match.
- `go-environment.json` records Go configuration; `execution-environment.json` records the subprocess runtime settings, including GOMAXPROCS=2, which `go env` itself does not expose.
- Minimum pre-command free space for `/tmp`: 1,413,267,456 bytes; free inodes 1,243,461. Every configured threshold passed.
- Minimum pre-command free space for `/workspace/scratch/5e06a74e5f65/durable-restart`: 12,132,941,824 bytes; free inodes 1,186,213. Every configured threshold passed.
- No cleanup, relocation, threshold lowering, parallel Go subprocess, source mutation during testing, or foreign binary execution occurred. The compiler slot was released back to the parent on natural completion.
