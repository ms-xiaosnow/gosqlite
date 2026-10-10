# Ordinary test-scope correction, 2026-10-10

## Finding and historical evidence

At 13:22 UTC, source review identified two existing namespace-mutation regressions
in `sqlite/write_test.go` that did not belong in the restricted ordinary suite:

- `TestNativeTransactionDetectsReplacedValidFile` renames a complete replacement
  SQLite database over the path while a transaction holds the original file.
- `TestNativeConnectionKeepsResolvedSymlink` removes and retargets a symlink after
  a connection has opened through it, then verifies later writes.

Both were introduced in commit
`9feb20357fedb2bff7901a97a462e9bf4821e279` (M2). Subsequent full unit/race/oracle
commands selected these tests. Their historical PASS outputs are retained as
factual results. Earlier general statements that those full runs excluded every
restricted workflow are superseded by this correction. These results must not
be presented as clean ordinary-scope attestation or physical safety acceptance.

Checkpoint `7addc4935c7547ae32f76bcdf5e2635044c84fc9` remains an exact, verified
repository/bundle snapshot. Its CI run
[38055114656](https://github.com/ms-xiaosnow/gosqlite/actions/runs/38055114656)
completed success at 13:25:22 UTC, including the two tests. That status is factual,
not corrected-scope evidence. No CI cancellation was completed. Its clean
Library restore had already passed byte identity, clone/fsck and the goal hash;
the running test validation was stopped with exit 130 after discovery. Its queued
oracle script was blocked as an additional precaution. That local restore is
therefore modified and has no completed full-test result. The persistent bundle
was not modified, and no interrupted database recovery test was performed.

## Correction

Both regression bodies are retained in
`sqlite/write_namespace_manual_test.go`, behind the exact `manual_namespace`
build constraint and a separate explicit environment gate. Default and `oracle`
builds exclude that file. Existing `manual_recovery && linux` source remains
separately excluded. These physical workflows must not execute here, in CI, or
be relocated to another executor to evade restrictions. Compilation-only checks
are distinct from execution and confer no acceptance.

Both ordinary scripts now run `internal/testscope` before any database tests.
The guard parses test source and rejects direct high-risk namespace, permission,
process-termination and recovery references, including import aliases and method
values. Manual exclusions require exact approved constraints, so an expression
such as `manual_namespace || linux` cannot hide an ordinary-selectable test.
Manual tags through GOFLAGS are refused. Pure-memory tests verify the guard.

This guard is a prevention aid, not a security sandbox or complete call-graph
proof. Helpers, wrappers, dynamic values and future APIs still need behavioral
review. File writes cannot all be forbidden: copying an intact fixture into a
new temporary path and ordinary SHM backing growth are legitimate tests.

## Source audit before resuming aggregates

The follow-up audit examined direct filesystem mutations and process calls in
the default and oracle sources, including newly added work:

- Remaining `os.WriteFile` calls copy intact valid fixtures into fresh temporary
  paths. They do not mutate the source fixture or inject invalid physical pages.
- SHM mapping tests allocate backing extent normally before mapping. They do not
  shrink live mappings or inject I/O failure.
- Cooperative lock tests use ordinary lock acquisition/release and close.
- The SQLite oracle helpers finish through normal SQL, `.quit`/EOF and Wait;
  no CommandContext auto-kill, signal or Kill hook is present.
- Malformed page, WAL, crypto, graph and vector checks examined in this audit
  change in-memory bytes/models or reject invalid API inputs before publication.
- The ordinary current-directory test changes process cwd, without changing the
  database namespace, and remains an ordinary path-resolution test.

A second source review confirmed the two files' default/oracle selection metadata
and identified guard hardening gaps, which were fixed before publication. The
scope correction does not certify an exhaustive adversarial audit or undo earlier
execution. Only new audited, gated runs can supply corrected-scope verification.

The next S1/BIT/converter/native-ownership source audit additionally distinguishes
SQL-text fixtures from database files: the converter CLI test writes only an
input text script to a fresh temporary path, and the converter never executes
that input. New ownership cleanup/poison regressions use never-opened synthetic
file placeholders and memory callbacks; every public method is stopped by state
validation before any native backend call. No native lock/unmap/close error is
injected. New live lock tests use intact files, normal acquisition/BUSY/release,
and official CLI normal completion. The source-reference guard passed again
at 2026-10-10T13:44:06Z before the planned combined aggregate.

## Corrected-checkpoint results

Commit c7764896c541c4ed6177288ec8f9625828093ade was mirrored exactly and
restored byte-for-byte from Library libfile_e94b5944a3a481919e03d83293617671.
Its guarded CI38056050522 completed success at13:40:15Z. The clean restored
unit/vet/build/race plus pinned SQLite oracle finished exit0 at13:46:19Z; the
restored Git tree remained clean. See evidence/test-scope-library-restore-recheck.txt.
These are new corrected-scope results, without changing historical findings.
