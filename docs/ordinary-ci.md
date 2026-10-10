# Ordinary unit and SQLite oracle CI

The workflow `.github/workflows/check.yml` has two independent ordinary jobs:

1. `ordinary` runs `scripts/check.sh` with Go 1.27.2. It guards test scope,
   runs pure-Go unit tests, vet and build, then the separate Go race-tooling gate.
2. `oracle` builds the pinned official SQLite 3.54.0 CLI in the runner's temporary
   directory, then runs `scripts/test-oracle.sh`. The script requires Go 1.27.2
   and an explicit absolute `SQLITE_ORACLE` executable path, runs the scope guard,
   verifies the exact SQLite version/source ID, and executes the complete
   ordinary `go test -count=1 -timeout=30m -tags oracle ./...` suite with
   `CGO_ENABLED=0` and `GOTOOLCHAIN=local`.

Both jobs run on push and pull request. The oracle job has no `-run` filter,
package subset, manual build tag, optional-oracle fallback, or cached test pass.
The C compiler and SQLite CLI are test-only; production remains pure Go and does
not execute or link SQLite C. The builder runs no upstream SQLite tests.

## Reproducing the pinned oracle

The build recipe and upstream pins come from `docs/upstream.md` and the existing
`scripts/install-baselines.sh`; this smaller builder does not download Go.
Use a host with a C compiler/make, Python 3.11 or newer, unzip and curl. The builder fails
explicitly if a required tool is absent; CI does not install system packages.
Choose a new absolute destination whose parent exists; existing destinations are
refused so another toolchain cannot be overwritten.

```sh
./scripts/build-oracle.sh /absolute/new/sqlite-oracle
export SQLITE_ORACLE=/absolute/new/sqlite-oracle/sqlite-build-3540000/sqlite3
./scripts/test-oracle.sh
```

To reuse an already downloaded official archive without a network request:

```sh
./scripts/build-oracle.sh /absolute/new/sqlite-oracle /absolute/sqlite-src-3540000.zip
```

Both routes require the 14,881,448-byte archive and stream its SHA3-256 before extraction, the generated
`sqlite3.c` SHA3-256 after compilation, and the executable's version/source ID:

- Archive: `a5c29342ca185abcda332448fbb27c536661245be27e20199b46c49099b0e15e`
- Generated source: `8efd453d08a7cfc4c79de5923153036cdc4dcaa254d4f1e617be6a2a98828f9e`
- SQLite: `3.54.0`
- Source ID: `2026-10-09 15:46:58 be8d059e9a49089ab2dce5ed26dd87aaf598fdc5fbe0b107c0dd758464bcd5e3`

Configure/build logs remain in the destination's `evidence/` directory. These
checks do not claim byte-identical CLI binaries across compilers/build paths.

## The coverage correction

Before this change, CI ran only `scripts/check.sh`; it neither installed the
pinned oracle nor invoked `scripts/test-oracle.sh`. Historical CI run
[38076830650](https://github.com/ms-xiaosnow/gosqlite/actions/runs/38076830650)
therefore passed its narrower unit/vet/build/race scope. It did not establish a
full tagged-oracle pass. A scoped clean-restore oracle command likewise verifies
only the selected packages/tests, not every tagged test in the repository.

The subsequent complete tagged-oracle run exposed an obsolete assertion in
`TestOracleErrorsAndResources`: an implemented `JOIN ... ON` query was still
expected to return `ErrUnsupported`. The exact historical failure log
`relational-lead-v2-checks.txt` is retained unchanged by the lead. This correction
moves the original query into positive pinned comparison and replaces only that
negative case with unsupported `JOIN ... USING`, retaining the explicit error
assertion. No assertion was relaxed and no test was skipped.

The source/helper audit and local focused/fresh-builder evidence are under
`docs/evidence/oracle-ci/`. Focused checks and a validated workflow definition do
not establish a new hosted CI pass. The lead records full corrected acceptance
and the hosted run for the exact published commit separately.

## Scope boundary

Read `docs/test-scope-audit.md` and review the bodies/helpers of changed tests
before execution. Ordinary scope includes intact, correct-key database access,
normal writes, rollback, BUSY/cancellation, close/reopen, checkpoint, successful
rekey and encrypted copy/backup/restore, and pure-memory negative cases.

Physical corruption/tamper/wrong-key/missing-file tests, hostile namespace
mutation, interrupted publication/rekey, native I/O or resource failures,
short-write/disk-full/permission injection, crash/kill/forced process-exit and
hot-recovery workflows remain outside both jobs. Manual tags are forbidden by
the scope guard. Its static reference checks supplement behavioral review and
are not a sandbox or an exhaustive safety proof.
