# Final finite closeout: source-scope review before aggregate

Result: composition and new-test audit PASS. Runtime aggregate has not started when this review was written.

## Exact composition

The independent read-only audit reproduced tree d7393c254a6fca4452ae194d64d9045659f9781d using in-memory Git object encoding, with 2,935 files and 141 directories. It exactly matches final-closeout-combined-evidence-v1/manifest.json and frozen-inventory.json. No Git command or metadata was used.

All 353 baseline test files remain byte/mode unchanged. All 358 prior CLI-combination test files remain unchanged. Against the already independently reviewed and full-gate-passed CLI tree 4c2af13646c973c53340450f8f1310492aae5628, the only Go changes are the eight declared retention paths. Every one exactly matches the retention freeze reviewed independently as SCOPED PASS. The remaining changes are README/docs/evidence only. Module files, all scripts, the source guard, database kernel, platform backends, and goal.md are unchanged from the prior CLI combination. No held candidate code is composed.

## New tests and activated behavior

Read both added test files and helper in full. cleanup_test.go uses only in-memory callbacks, call counts, joined sentinel errors, and delayed deferred-result checks. No files, database handles, credentials, child processes, or native faults are created by those new tests. source_test.go parses five existing main.go files and compares AST source text. It never imports or executes those example packages; the strings naming os.RemoveAll and Close are source expectations only.

The five retention run changes delay the finalizer's evaluation until later resource defers finish. Completion is assigned only on final ordinary success, checked output failures return first, and removal failures are reported without retry. Existing provider, key clearing, Rows/DB Close aggregation and other helpers are unchanged. The unchanged encrypted-wal and encrypted-shared tests still run their existing intact-file, correct-key, ordinary successful workflows. The WAL test sets TMPDIR to its own standard t.TempDir as before. No snapshot, encrypted-cli or native-check main/run(binary) is newly invoked.

The prior CLI review and aggregate report cover the unchanged five previously added test files: pure status/callback/error tests, intact fixture query reads, ordinary success create/commit/reopen/normal rollback/Check, and static AST comparisons of legacy source and mutations. They do not invoke preserved legacy mains or inject storage failures. The new aggregate contains no added tests or probes beyond these supplied sources.

## Reviewed execution instructions

Read the software-engineering skill, README testing instructions, docs/closeout-scope-20261011.md, docs/test-scope-audit.md, scripts/check.sh, scripts/test-oracle.sh, internal/testscope/main.go, the prior aggregate shell/precheck/identity scripts and reports, and the retention independent report. The exact existing ordinary aggregate is mirrored under a new evidence prefix with a new owned copy. Source identity checks independently revalidate composition before copying and after execution; actual modes and directory membership are included. No source edits are permitted.

The ordinary oracle script is unchanged. It guards source, pins Go 1.27.2 and SQLite 3.54.0 source ID, then runs the entire existing oracle-tag suite without a run filter. Existing exact manual_recovery && linux and manual_namespace files remain excluded. Sourceguard is a prevention aid rather than a proof of runtime fault behavior; the source/body review and byte-identity checks supply the scope evidence.

## Limits and stop conditions

Use the previous pinned toolchain/environment and default /tmp only. Stop on first failed gate; preserve stdout, stderr, command, times and exit. No relocation, deletion, cleanup workaround or retry. No manual-tag, physical fault, crash, process-exit, tamper/wrong-key disk, namespace interference, hot recovery, ANN V4, new 41-target fuzz, unknown interruption or held native-recovery/WAL-tail/backup/planner/backend chain. No new fuzz/soak/performance work, native non-Linux execution, runners, billing, Git or remote operations. Cross-platform checks compile only.

Neither this source audit nor a subsequent ordinary PASS proves failure-fixture retention under real faults, intact removal after partial failure, physical durability, recovery, production readiness, or completion of original milestones. Every existing HOLD remains.

## Sources and SHA256

- final-closeout-combined-checks.identity-audit.json: 6b9fe8acd0106726b0b52226c7a364d59b7fd5ee1188bdb161fd0dd2295f825a
- final-closeout-combined-evidence-v1/manifest.json: 42e86ecc5dacd170693e3d798d0e9512a35cc168b9a35484d0228b1403a8b5f1
- final-closeout-combined-evidence-v1/frozen-inventory.json: b3f85ee50b04ef37d0adb45f8482673cdec1853340ac7f2045cca0b7a8dace7e
- cli-resources-independent-evidence-v1/REVIEW.md: 49b719d3b3ac4b40b1083b0c476897839613f834c7fbd3f421a6f2fd5cb9f2d7
- cli-resources-combined-checks.REPORT.md: d9b56dbb8e6650065a84c7dd24182fcf225417da622859e51aad48859e98b78e
- example-retention-review-evidence-v1/report.md: 7cee0bd0589b03b0edbcaac7c38a1651def88f29d4471513b40c7a17c8bc277a
- example-retention-freeze-v1.json: 7c3906728139107b7ea58729d2f2f868e641a170e249b8c7435323194c856478

Prepared UTC: 2026-10-11T04:07:44.649500+00:00
