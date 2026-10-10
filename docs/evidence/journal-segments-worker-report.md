# Multi-segment rollback planner worker report

Final implementation freeze: journal-multisegment-freeze-v3.
Tree: 0182810a1ff30ed77209fced474191ee8ddff7fc.
Baseline: rowid-reason-repair-freeze-v1, exact verified tree
1e413b466d10db5c7304ba231fd5d4536dfc5fa5.

Complete 1228-file manifest: journal-multisegment-v3-all-sha256.txt,
SHA256 16b7f77947fee0a84e3657872517241225954b2fdcebb2524e2dfd1b46215299.
Eight-path patch: journal-multisegment-v3.patch,
SHA256 dea7d4d4c721ee5c82137e4e43bc6e4a78dc571155ed4de9556628982764066e.
Before/after hashes and command-evidence hashes: journal-multisegment-v3-owned.json.
Patch applies cleanly to the exact baseline. Full frozen inventory reverified.
Main gosqlite source was not edited, staged, committed, restored or published.

## Result

A conservative published finite-count multi-segment ParseReplay implementation,
with first-header geometry/extent, sector alignment, per-segment nonce, global
original-page uniqueness and exact final consumption. Header remains the raw
first header; Pages is cumulative. No exported layout changes. Complete input
validation precedes page-image copies, and every error returns a nil plan.
Existing single-segment success values remain intact.

Input, cumulative records, copied-image bytes, metadata and resulting extent
are bounded by the existing three ReplayLimits fields. No retained segment
list exists; zero-record segments consume a full sector. Documentation includes
arithmetic proof and an explicit unsupported matrix.

## Selected evidence

Behavioral review: all journal production and test sources have only memory
operations; no database/filesystem/pager/native-fault path, init or TestMain.
Guard: internal/testscope passed before each suite.
Official Go 1.27.2, local toolchain, proxy/checksum database off, no manual tags,
GOFLAGS=-buildvcs=false, sibling shared cache/path, GOMAXPROCS=2.

- Selected CGO0 normal tests and fuzz seeds: PASS, 0.081 seconds.
- Selected CGO1 race-tooling tests: PASS, 1.762 seconds.
- Multi-segment memory fuzz: PASS, 2717 executions, requested 5 seconds / 2 workers.
- Existing replay memory fuzz: PASS, 7983 executions, requested 5 seconds / 2 workers.
- Journal/pager vet/build: PASS.
- Untagged pager tests compile-only: PASS. The binary was not executed.
- Commands: journal_multisegment_checks.sh.
- Output: journal-multisegment-checks.txt; exit: journal-multisegment-checks.exit = 0.

All Go bytes in v3 match the tested v1. Independent reviewer found no parser
blocker and caught stale multi-segment/native-source documentation language;
v3 corrects only compatibility.md and progress.md relative to v1. V1 and the
short-lived intermediate v2 remain preserved. Independent final attestation
is delivered separately by review_memory_journal_segments.

## Exclusions

No RecoverRollback call, hot-journal generation, SQLite CLI oracle, manual tag,
physical corruption/tamper/wrong-key/missing-file test, namespace replacement,
race, crash/kill/process-exit/hot recovery, native failure, short write,
disk-full/permission fault, or interrupted publication/rekey was performed.
The race detector above checks pure-memory code, not a namespace/native race.
Native recovery source is unedited and unexecuted; its parser dependency now
accepts this broader subset. Nothing was wired into Open. Physical recovery,
M3 durability, native failure/platform and release acceptance remain unqualified.

Super/master journals, no-sync/safe-append sentinel counts, legacy unspecified
page size, repeated originals, changing geometry, unpublished headers and
ambiguous/trailing data remain unsupported. No first/last-wins rule, tail
salvage, hotness proof, database-identity claim or full SQLite recovery claim.
