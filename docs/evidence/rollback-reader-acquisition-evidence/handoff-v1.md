# Ordinary rollback reader acquisition repair

## Result

The predeclared unchanged-harness controls both reproduced the ordinary rollback
`ErrChanged` failure. All four repaired samples completed their requested 20
active seconds, with exact final SQL model verification and normal joined-worker
closure. The new AST source regression rejects the old constructor and passes
the repaired constructor. Focused normal/race/vet/build gates passed.

This closes the identified pre-SHARED metadata acquisition defect within the
reported ordinary test scope. It is bounded evidence, not long-soak, native
failure/recovery, hostile namespace, or all-platform acceptance.

## Exact source and minimal patch

- Base: `../grouped-counters-final-verify`, tree `13f53e4cdadaaca2530e17e5f7a61b9dfe8e5c33`.
- Frozen engine source: `../rollback-reader-acquisition-freeze-v1`, tree `ad066d04d676b8368ca7d02d83664e50dfa47216`.
- Patch: `source-v1.patch`, SHA256 `ccc9bf728b3df6b0daa65562955ae54cc84fcdf690a9d8985d17888d740c3659`.
- Three owned paths: `internal/pager/pager.go`, `internal/pager/reader_acquisition_test.go`, and `docs/rollback-reader-acquisition.md`.
- Exactly six production lines are added. Of 1322 baseline files, only pager.go changes; 1321 remain byte/mode identical, and two files are added (1324 total).
- `base-inventory.json` and `freeze-v1-inventory.json` list every file's byte count, SHA256, Git blob/mode, and exact filesystem mode; matching complete SHA256 manifests are also retained.
- `freeze.py` recomputes the Git-compatible tree independently, cross-checks Git write-tree, generates the patch, applies/checks it to a fresh full baseline copy, and compares every byte/mode entry.
- Replay: `../rollback-reader-acquisition-patch-verify-v1`, exactly the frozen final tree.
- `identity-v1.json` records engine, AST-control, and harness-composition identities.

The refresh is on the already-owned descriptor immediately after successful
SHARED. Initial regular-file rejection, sidecar checks, header validation,
pathname identity, strict snapshot checking and the existing deferred retirement
owner remain unchanged. No production retry or ErrChanged-to-BUSY conversion is
introduced. The added stat failure follows the same error and ownership path.

## Attribution and original failure preservation

`original-baseline-attribution.json` records that pager.go has identical bytes in
the original failing V1 harness, the report-only V2 harness, and this grouped
baseline: SHA256 `1c08ac6cd21b11ba858b444a00af6cf68d7f28aa93453c1fccdc8fef151cb8bc`.

The original V1 ordinary rollback failed at 14.015854093 active seconds with
723 commits and 4138 successful reads; its status remains failed and final-model
verification remains false. Original report SHA256 remains
`a62f4bfed390582d1125d88a6dfbe06f06f3a33500a55fedacea934424a1e101` and its normally
closed database remains `b996a714bd220924cd93370ca257c553f17ec775159a33382606b9b29ac773e9`.
Neither was modified. A later passing V2 sample never erased that finding.

## Behavioral audit and focused gates

`audit-v1.md` was written before selected execution. No unreviewed broad prior
baseline/full suite or manual tag was run. No physical corruption/tamper,
wrong-key/missing-selected-file, namespace mutation, crash/kill/abrupt open-DB
exit/hot recovery, native I/O/lock/unmap/close/Sync/shortwrite/disk-full/permission
fault, interrupted publication/rekey, downloaded code or external write occurred.

The new ordinary test uses valid pager APIs, two independent readers, 128
schema-cookie commits and 32 rollbacks per run; every acquired reader closes
normally and workers join. It checks page/header generation and stable repeated
page reads. Its strict BUSY decision helper rejects any joined unknown leaf.
Pure source and pure-memory error-decision checks introduce no native hooks.

All runs use pinned Go1.27.2, GOTOOLCHAIN=local, GOPROXY=off, GOSUMDB=off,
GOFLAGS=-buildvcs=false and the sibling local caches. CGO=0 except race tooling:

- Frozen scope guard: PASS.
- `go test ./internal/pager -run '^Test(ReaderAcquisition|ReadOnlyPageAccess$|TxCreatePageSizesAndReadSnapshot$)' -count=20 -v`: PASS.
- SQL deferred snapshot timing, count10: PASS.
- `go vet ./internal/pager ./sqlite` and `go build ./internal/pager ./sqlite`: PASS.
- The focused pager selection under race, count5: PASS.
- SQL deferred snapshot timing under race, count5: PASS.
- The source-only control (baseline plus the new test file) failed exactly as expected, exit1: missing post-SHARED `info, err = statNativeFile(file)`. No live database operation is executed by that test.
- Both harness compositions passed the guard and CGO0 native build.
- Unchanged V2 harness unit tests and its separate race tests: PASS.

Each command has its own raw `.txt` and `.exit`. `check-v1.py` is the exact
orchestrator, `check-v1.txt` the timestamped sequence, and `checks-v1.json` the
status ledger. Orchestrator exit0 includes the intentionally expected source
control failure and preserves the two real baseline workload failures.

## Harness composition and bounded ordinary SQL trials

The checked V2 patch (SHA256
`71b64b771631b39978f635aadb3348c0e42d6f7c652f25e7f21b719f1d973606`) is applied only
in separate test-composition directories, not the engine-owned patch. All five
harness files were verified byte-identical to `transaction-soak-freeze-v2`.

- Baseline composition: `../rollback-reader-acquisition-baseline-soak-v1`, tree `dcd64974d36cb35413f78dd1c006a55a79fe9a5b`.
- Candidate composition: `../rollback-reader-acquisition-candidate-soak-v1`, tree `ac7242786b6e7ecffd03f9c45216ee788eaddbba`.

All six runs have the same workload identity and original parameters:
rollback, 20 actual active seconds requested, rows64/readers2/batch2,
seed20261010, writer pause0, reader pause1ms, snapshot hold1ms. The predeclared
order was baseline1, candidate1, baseline2, candidate2, candidate3, candidate4.
This was not a run-until-pass procedure.

| Run | Status | Active seconds | Commits | Successful reads |
|---|---|---:|---:|---:|
| Baseline1 | FAILED ErrChanged | 19.673626858 | 1559 | 8345 |
| Candidate1 | passed | 20.000108642 | 1618 | 8592 |
| Baseline2 | FAILED ErrChanged | 1.098452608 | 92 | 428 |
| Candidate2 | passed | 20.002908472 | 1840 | 9153 |
| Candidate3 | passed | 20.000854830 | 1513 | 8521 |
| Candidate4 | passed | 20.000633911 | 1664 | 9128 |

Both controls fail specifically at reader generation acquisition and preserve
`final_model_verified=false`. Every candidate has no errors and final exact
model verification true. Every run reports `workers_joined_normal_close`.
Candidate total is 80.004505855 active seconds, 6635 commits and 35394 successful
reader transactions. No corruption is inferred from the baseline error itself.

`soak-v2-*.json`, matching `.exit`, `.stderr` and `-command.json` retain every
outcome. `retained-data-and-reports-v1.json` records report and normally closed
data-file hashes. The bounded deterministic writer traces were independently
reconstructed by the read-only Python verifier in `verify_reports.py`;
`report-model-verification-v1.json` validates all six reports' final model hash,
exact transfer history, generation, conservation and outcome counts while
retaining both failures. It invokes no database engine or physical mutation.

The host was shared Linux/amd64, AMD EPYC9V74 with nine visible logical CPUs;
report build settings contain the exact Go/CGO/config identities. Rates are
observations only; no controlled performance or confidence-bound claim is made.

All source trees and file-mode inventories were rechecked unchanged after the
runs (`final-verification-v1.json`). The lead independently reviewed the six-line
production delta and source/tests/helpers, reconstructed the same complete
1324-file tree, and ran separately isolated focused gates; its result remains
separate evidence. No commit, central progress edit, upload or remote write was
made by this worker. The lead owns integration and the final project status.
