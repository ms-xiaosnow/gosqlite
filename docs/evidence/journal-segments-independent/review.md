# Independent review: bounded multi-segment rollback-journal plans

## Verdict

No parser correctness defect found in the reviewed bounded, published,
finite-count, identical-geometry, globally unique-page profile. Selected
pure-memory normal/race/model/fuzz checks pass. The v1 documentation inconsistency was corrected in the docs-only v3 freeze;
its exact final identity and verification are recorded below.
The parser slice is acceptable within this expressly memory-only scope. This is not physical recovery,
SQLite-wide compatibility, native-platform, CI, or M3 acceptance.

## Exact source identities and reproduction

- Baseline: `rowid-reason-repair-freeze-v1`, independently reconstructed Git tree
  `1e413b466d10db5c7304ba231fd5d4536dfc5fa5`, 1,226 files.
- Reviewed implementation v1: `journal-multisegment-freeze-v1`, independent tree
  `a422c80a53886292c2bdd6661a26d1791ebee385`, 1,228 files.
- v1 patch SHA-256:
  `52f1bc37a3d310c4286557b6104f373e6070535eb33997b650a7ec6e6eb3127d`.
- v1 inventory SHA-256:
  `550914f296c5e3d66bea7a32838a36aad77a0f5a4789902a31feab03ce7cb8de`.
- Parser SHA-256:
  `d8263d7a29d7d56509066057524a6df56b6914081569730225cf1981b60aa404`.
- Native consumer `internal/pager/recovery.go` remains byte-identical to baseline,
  SHA-256 `d95f59bb48da4de190a4c3257a215310598aea0c06e80aa9b39674d2a381754e`.

The exact eight-path owned patch was checked and applied to a separate baseline
copy. All 1,228 resulting file hashes equal the freeze; independent Git tree
calculation agrees. Full baseline/freeze/source hashes and the patch/inventory
were rechecked after execution. Source-hash tools only read the lead source;
all experiments use isolated copies, and no commits or main-source edits were
made by this reviewer. Review tests are archived as `.go.txt` evidence and added
only to isolated `internal/journal` test directories.

## Public SQLite source comparison

The local official archive has the documented 14,881,448 bytes and SHA3-256
`a5c29342ca185abcda332448fbb27c536661245be27e20199b46c49099b0e15e`.
Its `src/pager.c` exactly matches the extracted source SHA-256
`96d7bb9c4a5a6da0909ce0ab4714311e1ffa5f13007dc4c69f028a2760104ba4`.
The source ID is
`be8d059e9a49089ab2dce5ed26dd87aaf598fdc5fbe0b107c0dd758464bcd5e3`.
See `upstream-pin.json` and `source-scope-audit.md` for independent pin proof and
specific source function/line references.

The parser correctly uses first-header geometry and original extent, advances
headers to the next aligned sector at or after the prior record end, and uses
each segment's own finite count and checksum nonce. Equal nonces are permitted.
Later geometry/extent agreement and globally unique originals are explicitly
stricter than upstream ordinary playback. Unpublished/ambiguous tails,
no-sync/safe-append sentinel, super-journals and invalid records return nil plans.

An input ending exactly at an earlier complete segment endpoint is a valid
complete input by itself; byte parsing cannot infer an omitted suffix. Tests
explicitly distinguish that case from every partial header/body/padding cut.
Sparse checksums do not establish authentication, generation, identity or hotness.

## Independent validation

All checks use pinned Go 1.27.2 with offline toolchain/module settings,
`GOFLAGS=-buildvcs=false`, shared cache/path and bounded workers. Each execution
batch begins with `go run ./internal/testscope`. Only selected journal tests run.

- Existing single-segment format, geometry, truncation, limits and extent tests
  pass unchanged, apart from the needed cumulative-count fuzz invariant update.
- Independent geometry and ownership tests cover 96 page/sector combinations,
  first/interior/final zero-count chunks, distinct and repeated nonces, preserved
  first raw header, cumulative page order, and separate owned plan images.
- Every prefix of two independently constructed multi-segment fixtures is
  checked; only complete segment endpoints succeed. Padding-only, partial,
  magic/unpublished and extra tails do not return partial plans.
- Later count/nonce/extent/geometry mutations, invalid page numbers, cross-segment
  duplicate originals, late checksum failures and locking-page records reject.
- Exact and one-below whole-input/record/extent budgets, cumulative quotas,
  huge uint32 advertisements, maximum supported int64 extent and 1,024 empty
  segments exercise bounded arithmetic and traversal.
- Five hundred deterministic randomized histories compare original image/order,
  missing-page coverage, partial current-page extents, unchanged pages and
  idempotent application with a separate byte-array model.
- All selected tests pass normally and under Go race tooling. Scoped journal
  vet and journal/pager builds pass, including compile-only Linux/386,
  Linux/arm64, Darwin/arm64 and Windows/amd64 targets.
- Three fuzz targets each use a requested 10-second duration and two workers:
  existing replay 10,920 executions; multi-segment byte fuzz 342,897; independent
  structured generation/model 132,941. All pass. Execution counts are observed
  local results, not coverage guarantees.
- A separate baseline/final differential digest covers 576 supported
  single-segment samples. Both produce SHA-256
  `f404724d8de2ccfa6a9f0d4d6c717b68b98d0a5f5fd12289f244c5fe5017bf35`.

### Negative controls retained

1. The original baseline rejects the independent valid multi-segment fixture,
   establishing that the positive test actually requires the new behavior.
2. Replacing the segment nonce with the first nonce makes valid segments fail.
3. Removing cumulative quota subtraction makes an over-quota plan succeed,
   which the independent limit assertion detects.
4. Stopping after the first segment makes a partial prefix succeed, which the
   exhaustive-prefix assertion detects.
5. Returning input-backed image views makes input mutation alter plan output,
   which the ownership assertion detects.

The four mutations were made only to parser Go source in separate copies. Full
source inventories verify exactly that one mutation and the test harness, with
no hidden source changes. Mutation patches, hashes and expected-failure output
are retained. These are software assertion controls, not physical fault tests.

### Audit chronology correction

The initial test-source review preceded the freeze. The freeze added one small
zero-count/already-aligned-header test after that initial read. The first
selected normal/race run is retained in `selected-checks.txt`, but is superseded
for final source-audit attestation. The exact final test file, including that
new byte-only test, was then reread in full and the selected normal/race/vet/build
batch was rerun successfully in `selected-final-audited-checks.txt`. The extra
test has no filesystem, native or process operations. Fuzz targets were already
reviewed and selected independently with `-run '^$'`. No prohibited operation
was executed in either run.

## Findings and resolution

V1 documentation finding: `docs/compatibility.md:49` still blanket-listed
multi-segment recovery as an unsupported input that always fails closed. This
contradicts expanded parser admission by the existing internal consumer, even
though no physical replay is qualified. The worker was asked to replace that
blanket statement and clarify that native replay source was not edited or run,
rather than claiming all transitive native behavior was unchanged.

Resolved in final `journal-multisegment-freeze-v3`, tree
`0182810a1ff30ed77209fced474191ee8ddff7fc`. Only
`docs/compatibility.md` and `docs/progress.md` differ from v1; all Go/test and
other source hashes are unchanged. The final patch was independently checked,
replayed onto baseline and all 1,228 resulting hashes verified. A further
guarded selected pure-memory normal run on the exact final copy passes.

Final baseline patch SHA-256:
`dea7d4d4c721ee5c82137e4e43bc6e4a78dc571155ed4de9556628982764066e`.
Final full inventory SHA-256:
`16b7f77947fee0a84e3657872517241225954b2fdcebb2524e2dfd1b46215299`.
See `final-attestation.json`, `v1-to-v3-exact-docs-delta.json` and the `v3-*`
verification/test logs. V1 source, earlier checks and all negative controls
remain preserved, and v2 is correctly treated as superseded.

## Evidence and boundaries

- `run-selected.sh`, `selected-final-audited-checks.txt`, corresponding `.exit`:
  final source-reviewed selected normal/race/vet/build checks.
- `run-fuzz.sh`, `fuzz-checks.txt/.exit`: three bounded fuzz targets.
- `run-single-compatibility.sh`, `single-compatibility*`: baseline/final digest.
- `negative-controls.json`, `negative-*.patch/.txt`, and negative-control script:
  exact source mutations and expected assertion failures.
- `owned-delta-verification.json`, full verification logs, tree files and
  `whole-source-post-verification.json`: exact ownership/source attestations.

No journal/database fixture is written or opened by any reviewed test. No
SQLite CLI, RecoverRollback, manual tag, corruption/tamper/wrong-key workflow,
missing-file scenario, hostile namespace change, crash/kill/process-exit,
native I/O/lock/close/unmap/Sync failure, short-write/disk-full/permission fault,
or interrupted publication/rekey executes. Native source was inspected only;
cross-builds do not qualify those platforms. No broader aggregate or CI result
is claimed. Main Open still refuses existing journals. The lead owns subsequent
ordinary aggregate integration and any decision on final acceptance.
