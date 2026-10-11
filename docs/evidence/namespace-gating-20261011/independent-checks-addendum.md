# Independent approved-check addendum: namespace gating

Completed at 2026-10-11T08:27:31.145833+00:00. Checks started at 2026-10-11T08:26:58.375564+00:00 and naturally exited 0. The compiler slot was explicitly released to the parent immediately afterward; no further Go commands are planned or running from this reviewer.

## Verdict

PASS for the newly authorized narrow verification of frozen tree `01918609a55cc149c5155451da40f7389854db06`:

- Whole-tree ordinary source-reference guard.
- All `internal/testscope` inert AST/source-string model tests, freshly run with `-count=1`.
- The same pure tests with race instrumentation, freshly run with `-count=1`.
- Explicit manual-tag GOFLAGS rejection, with the expected exit 1 and expected refusal text.
- 47 supported OS/architecture targets × four tag sets = 188 fresh `go list -e -json` selections, with no package error.
- Six fresh foreign test compilations and exact expected test-symbol verification: windows/amd64, darwin/amd64, and darwin/arm64, each with `manual_namespace` and `oracle,manual_namespace`.
- Before/after exact full file-byte/SHA-256 inventories, paths, Git modes, file POSIX modes, and root/subdirectory POSIX modes. Base remains `442a7104dc780ddb20826e5ee4f1250b291f2b24`; frozen candidate and reviewer replay both remain `01918609a55cc149c5155451da40f7389854db06`.

All manual binaries, raw namespace/native physical execution, recovery, crash/kill, tamper/corruption, wrong-key, interrupted-publication, ANN V4, 41-target fuzz, unknown interrupted chains, and new OS runners remain **NOT_RUN**. Foreign binaries were compiled and inspected as data only, never executed. `GOSQLITE_MANUAL_NAMESPACE` and `GOSQLITE_MANUAL_RECOVERY` remained empty for every subprocess.

## Exact default-selection reduction and platform preservation

Every matrix row checked actual selected test files and declarations. Default/oracle select neither moved declaration. On applicable platforms, the only removed existing test declarations are:

- `TestWindowsEncryptedRelativePublicationAndRetainedRoot`
- `TestDarwinEncryptedRelativePublicationAndParentLifetime`

Every other existing selected declaration is retained. With a manual tag, each new file is selected exactly when its corresponding old native file is selected; the moved declaration then appears exactly once. No additional test declaration was introduced. Windows remains windows/amd64 only. Darwin includes Go's existing Darwin/iOS relationship; this matches the old files rather than broadening platform selection. `compiler-checks/selection-summary.json` enumerates every affected matrix row.

Each foreign binary contains exactly the applicable full test-function symbol and no symbol for the other platform's moved test. Full symbol output and filtered exact symbols are retained with the artifact SHA-256 values.

## Independent execution and resource controls

The owner’s finalized report, summary, and all 210 command records were read first. Those results were provenance/preconditions only; all results above were obtained independently on the exact frozen source. The prior independent source review and its inventories were revalidated before execution.

Used pinned Go 1.27.2, `GOTOOLCHAIN=local`, `GOPROXY=off`, `GOSUMDB=off`, `GOENV=off`, `GOWORK=off`, the existing ROOT/go-cache and ROOT/go-path, `GOMAXPROCS=2`, `-p=1`, and default `/tmp` with `GOTMPDIR` unset. CGO was disabled except for the pure AST/model race run. No package/module download was allowed. Every subprocess was sequential; no automatic test or subprocess timeout was installed.

All 207 command prechecks met the original resource gates, without cleanup, relocation, or threshold lowering:

- `/tmp`: minimum free bytes 1,413,267,456 (required 1,073,741,824); minimum free inodes 1,243,461 (required 10,000).
- `/workspace/scratch/5e06a74e5f65/durable-restart`: minimum free bytes 12,054,114,304 (required 2,147,483,648); minimum free inodes 1,185,761 (required 10,000).

No main/frozen source, Git state, remote, CI, or Library content was edited. Writes were limited to reviewer evidence/build artifacts and the approved existing Go caches.

## Relationship to the earlier report

`report.md` is intentionally unchanged. Its SOURCE REVIEW PASS and NOT_RUN statements record the earlier source-only stage. This timestamped addendum updates only the now-authorized guard/model/race, selection, and compile/symbol statuses. Its original SHA-256 remains `964bf68a93280bad0b13571ca1168f77d0c467b4dc3c08845db8a03bd5dfa357`.

The source-review limitations still apply: generic selector matching can conservatively flag unrelated methods; the guard is not complete type/call-graph analysis or a physical-safety proof; wrappers, bare helper calls, reflection/dynamic behavior, and future APIs require behavioral review. Exact manual-tag exemption alone does not establish an environment gate. Entire moved bodies and first-statement gates were proved separately in the unchanged source report.

## Evidence

- `compiler_checks.py`: independently authored exact-scope runner; no foreign-binary execution path.
- `compiler-checks.log`: sequential log and final PASS summary.
- `compiler-checks/checks.exit`: 0.
- `compiler-checks/check-summary.json`: exact status and completion timestamp.
- `compiler-checks/command-results.json`: 207 fresh commands and their exit codes; only the intended refusal exits 1.
- `compiler-checks/selection-matrix.json`, `selection-summary.json`: all 188 cases and affected declarations.
- `compiler-checks/foreign-compile-results.json`: six artifacts, SHA-256 values, exact symbols, explicit NOT_RUN execution states.
- `compiler-checks/before-identity.json`, `post-check-identity.json`: unchanged exact identities and original-report preservation.
- `compiler-checks/execution-environment.json`, `go-environment.json`: configured and tool-reported environment.
- `compiler-checks/resource-prechecks.json`, `resource-minima.json`: all original thresholds satisfied.
- `compiler-checks/owner-evidence-read.json`: owner prerequisites, separately identified from independent outcomes.
