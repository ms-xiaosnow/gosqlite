# LIMIT combination: source-scope audit before aggregate

Result: scoped source/composition audit PASS. Runtime aggregate has not started when this record was written.

## Exact input

Combined frozen tree ca8947811d60266c19db2d6b155bd62b843d6e48 has 2,965 regular files and 142 directories, independently recomputed without Git commands or metadata. Baseline is commit 63c2ebb10e3a2c4ff2378ee9f33c46133134c70f, tree d735409ebd1c43d90ee1650c8a7c2a084e270674. The full exact inventory is checks.inventory-audit.json; config pins all source manifests and the completed independent report.

There are exactly eight changed paths: sqlite/query.go, sqlite/limit_conversion_test.go, sqlite/limit_conversion_oracle_test.go, and five documentation files (acceptance-status, closeout-scope-20261011, interruption-20261010, progress, restriction-provenance-20261011). All 360 original test files remain byte/Git-mode identical; all 779 non-doc files exactly match the independent-review candidate tree 3fc9087ad121d423174a658e118ac455d1ace8c2. Every existing script, workflow, sourceguard file, module file and goal.md is unchanged. Frozen POSIX modes and all directory membership match the owner inventory. No symlinks, deletion, held-candidate code, or supplemental independent test is introduced.

## Test/body/helper review

Read the completed independent REVIEW.md and verified SHA256 2a00770dd9331d9afffb3934925d6417b56dfd62947d5400e329f66acb3c4137, plus its evidence manifest SHA256 709d50a658f096a8d164ae1d8548082852f0e341de224477f4b8cc8f60cb83d1. It accepts only the bounded conversion slice; baseline production FAIL and initial v1 oracle transport FAIL are retained. It reports unchanged fixed-v2 native 1,016 and oracle 344 leaves passing normal/race, plus 118 independent controls, with no production correction. The supplemental file is not integrated and will not execute in this aggregate.

Independently read both new test files in full, including every local helper and corpus. The native tests use finite typed scalar values and in-memory budget callbacks, context cancellation/deadline checks on scalar evaluation, and normal DB creation, SQL insertion/read, a read-only transaction with normal Rollback and normal Close. The scalar sentinel callbacks do not inject disk/OS faults. The four SQL shapes are single-source, LEFT JOIN, global aggregate, and grouped query. Conversion rejection asserts ErrMismatch/Code 20, nil Rows and no retained Rows lease; overflow retains its independent error. Zero-valued valid bounds are outside the public conversion slice, not removed from any existing test. No TestMain/init hook appears in sqlite/internal-sql test files.

The oracle helper runs the pinned official SQLite CLI on :memory: with fixed SQL. It uses exec.Command and natural Run completion, not CommandContext, a watchdog, a signal or Kill. Natural SQL datatype rejection must exit 1 with actual leading callback code (20), datatype-mismatch text and empty stdout. Success uses exact JSON integer parsing and row comparison. The corpus is 43 expressions x 4 shapes x LIMIT/OFFSET = 344 fixed comparisons. It neither loads an extension nor invokes an external observer. Only existing SQLITE_ORACLE configuration and the existing oracle tag are used.

Read the exact production diff, unchanged Eval/normalization, numericText/ApplyAffinity, full eval_budget.go, LIMIT caller and conversion paths. The production delta is only limitValue plus removal of unused strconv import. Full evaluated values are preflighted before embedded-NUL prefix conversion, which follows REAL then NUMERIC affinity. Other values follow NUMERIC and must yield int64; actual conversion rejection wraps ErrMismatch. Existing evaluation/budget errors remain original. No pager/VFS/recovery/backup/native backend code or global affinity code changes.

## Execution scope

Read current frozen scripts/check.sh, scripts/test-oracle.sh, .github/workflows/check.yml and the full internal/testscope/main.go. Reviewed prior complete ordinary source audit and scope records, test-scope-audit.md, restriction-provenance-20261011.md and manual-verification-zh.md section 12.3. Full unchanged ordinary baseline behavior is supported by those source reviews plus exact preservation; the guard is a prevention aid, not a call-graph proof or physical safety attestation.

The exact manual_recovery && linux and manual_namespace build constraints remain excluded. Two known raw namespace tests remain in OS-specific source: internal/encstore/native_windows_test.go requires windows && amd64; internal/encstore/native_darwin_test.go requires darwin. They cannot execute in this Linux/amd64 aggregate. Foreign-target go build checks only production compilation and do not compile/run test binaries or qualify native behavior.

The 18-stage external runner uses Go 1.27.2 and the pinned SQLite 3.54.0 executable, GOTOOLCHAIN=local/GOPROXY=off/GOSUMDB=off/GOFLAGS=-buildvcs=false, ROOT/go-path, ROOT/go-cache and GOMAXPROCS=2. Default /tmp remains, with >=1 GiB /tmp and >=2 GiB workspace prechecks. Full default and race suites use -count=1 -timeout=0. The complete existing ordinary oracle script runs unchanged with its already-existing 30m timeout and no filter/manual tag. No new timeout, cancellation, watchdog or campaign is added.

Every gate has a separate command, stdout, stderr, start, finish and exit file. Stop on the first failure without automatic rerun, cleanup, relocation or new input. Before/after full identity covers bytes, paths, lengths, Git modes, actual POSIX modes, directory membership and root mode. No source edits, Git, remote, CI activation, new runner provisioning, new fuzz/soak/benchmark, fault, crash, namespace, wrong-key/tamper, missing-selected-file, interrupted publication/rekey, ANN V4, 41-target fuzz or unknown-chain retry is authorized or performed.

An ordinary PASS would apply only to the exact frozen tree. It would not accept physical durability, crash/recovery safety, native foreign-platform behavior, general SQL parity, LIMIT-zero/EXPLAIN/binding phase semantics, old planner candidate integration, production readiness, or whole-project completion.

Prepared UTC: 2026-10-11T07:23:18.522209+00:00

## Pinned evidence

- /workspace/scratch/5e06a74e5f65/durable-restart/limit-combined-evidence-v1/manifest.json: b9d3bbcbd38a96681f8409f2f9f9d2c02c465f05f00a2509bc5b366c322d4926

- /workspace/scratch/5e06a74e5f65/durable-restart/limit-combined-evidence-v1/baseline-inventory.json: 2043b0fb276a17d4744b2135b5b1e70c50c2e7a487a7ba01b53ab9593f7ed9dc

- /workspace/scratch/5e06a74e5f65/durable-restart/limit-combined-evidence-v1/frozen-inventory.json: e628a49128c10ddc4e249f878ca14d0d3322e0fdfe8c1e9ce565ba1ce33d0bd8

- /workspace/scratch/5e06a74e5f65/durable-restart/limit-combined-evidence-v1/owner-inventory.json: 77d7cbde4126d70e7d75926d4f2ae8ffc6d95f485afdd6ceb451b47447f2fd18
