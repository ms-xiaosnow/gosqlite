# Independent bounded B-tree cursor review

Status: PASS for this additive low-level cursor prerequisite after the v2 admission repairs. No remaining cursor-blocking finding was identified. This is not SQL planner integration, general SQLite compatibility, or physical durability acceptance.

## Exact source and scope

- Baseline: `ad73b8d6425cd86d1c15caccb0f858048df63ef9`.
- Owner's immutable v2: `../bounded-range-cursors-v2-final`.
- Full owner manifest: 921 entries, SHA-256 `c9866fcb0a1dda2a5ab5fcdc6fabf90d3c69e421edaa9c9717e5f522229dbdb3`.
- Owned source manifest SHA-256: `35ab98966ddcfcf2d69cd32f40f8ef066317c9e7ce4f17cff5d9e78adc791207`.
- Additive owner patch SHA-256: `774e59c38126cbc5b33b5b19d073115eac9862e384655949f89d4be759a2207e`.
- Independently tested tree: `../bounded-range-independent-final`, copied from that immutable tree with only `internal/btree/review_bounded_independent_test.go` added.
- Reviewer test SHA-256: `5308e8fade2ac39c32438c70d2302ac0c533135aafd01018a97e322c92c6f60e`; identical source is saved here as `review_bounded_independent_test.go.txt`.

All 837 pre-existing tracked files independently match their baseline Git blobs. All 921 freeze entries and all 494 compiled Go source files were verified after the final runs, with no changed bytes. Existing scanner/index equality/prefix APIs and layouts, JOIN cell-allocation preflight, production SQL routing and file formats remain untouched. Review writes were confined to isolated review trees and this evidence directory. No production edit, staging, commit, upload or cache cleanup was performed by the reviewer.

## Findings and correction trail

1. The initial `rangeBudget.BeforePage` returned an upstream hook's nil result without rechecking context. A descriptor-free hook that canceled context then returned nil reproduced acceptance immediately before parser allocation. `initial-cancel-admission-unit.log` and its exit 1 are preserved. V2 rechecks context after the hook, and both owner and independent regression tests pass.
2. Initial endpoint copying bypassed the configured Work limit. The original guide explicitly treated endpoint copies as separately finite, so this was a preallocation-contract hardening request rather than an assertion that PayloadBytes covered all heap allocation. The independent test supplied two 5,000-byte BLOB endpoints and Work=6,000; initial construction returned nil error and read page 2. That original failure is preserved. V2 totals both endpoint structs, interface slices and cloned BLOB bytes with checked remaining-credit arithmetic, then admits the combined charge before copying either endpoint. The same independent test now rejects before any page read. Encoded-payload accounting retains its documented meaning.
3. The initial CLI oracle was environment-gated but untagged. V2 places it behind `//go:build oracle`, requires an explicit `SQLITE_ORACLE`, and runs it in a separately selected suite. Default B-tree tests invoke no CLI.

Initial passing traversal evidence and both failing admission logs were retained rather than relabeled as final results. The initial owner hashes are recorded separately.

## Independent coverage

The independently written comparison reference uses exact `math/big.Rat` arithmetic for INTEGER/REAL, its own storage-class classification and `bytes.Compare`; it does not use production key, prefix or cut comparison helpers. It constructs independently sorted descriptor-free 3-/4-way trees with multiple interior separators.

- 42,436 exhaustive bound-pair/direction cases across those two tree shapes, over 120 complete composite keys. Cases include unequal prefix lengths, inclusive/exclusive group cuts, full-key trailing rowids, absent/empty intervals, NULL/NaN bounds, infinities, signed zero/subnormal values, 2^53 and signed-int64 edge ordering, empty and NUL-bearing BINARY TEXT/BLOB values.
- 5,000 independently selected rowid bound/direction cases over 165 keys, including MinInt64, MinInt64+1, MaxInt64-1 and MaxInt64; payloads include overflow. Expected IDs use direct signed comparisons and expected payload identity is checked.
- Exact singleton traces across root interior records and leaf endpoints in both directions. Interior singleton reads one page; leaf singleton reads three; the same full 120-key index traversal reads 21 pages. No skipped/duplicated keys appeared.
- Descriptor-free malformed cell-array and inherited parent-bound refusal, checked MaxInt64 cumulative arithmetic, negative-limit no-read refusal, cumulative Records/Returned/PageReads accounting, sticky quota/error/EOF state, both endpoint-copy admission and post-hook cancellation regressions.
- The full reviewed default B-tree package also passes unchanged scanner/equality/prefix/parser/mutation/ownership and JOIN page-allocation regressions, owner 3,000 composite properties, 2,166 intact-fixture rowid intervals across 512/4096/65536-byte pages, overflow/ownership and cancellation cases.
- Separate official SQLite oracle run: all 846 interval/direction answers passed across 512/4096/65536-byte pages. Expected results use official SQL comparison/IS/NULL-safe CASE and ORDER BY. Intact databases were hash-checked unchanged, normally closed/reopened and read again.

Measured actual PageReader calls in the reproduced official fixture:

| Page size | Index singleton forward / reverse | Full index | Three-row rowid interval, each direction | Full table |
|---|---:|---:|---:|---:|
| 512 | 6 / 3 | 345 | 3 | 366 |
| 4096 | 5 / 5 | 112 | 2 | 46 |
| 65536 | 2 / 2 | 4 | 2 | 4 |

These are logical calls, including repeated overflow probes, not physical I/O or latency. Stats.PageReads matched the instrumentation. Exact traces are in `final-oracle.log` and the independent singleton traces are in `final-unit.log`.

## Final commands and results

Toolchain: Go 1.27.2 Linux amd64. Shared sibling go-cache/go-path only; GOTOOLCHAIN=local, GOPROXY=off, GOSUMDB=off, GOFLAGS=-buildvcs=false. Before every suite, `go run ./internal/testscope` passed. The separate behavioral audit is in `behavioral-audit.md`; guard success alone did not authorize any suite.

1. CGO_ENABLED=0 `go test ./internal/btree -count=1 -v`: PASS, 30.991s.
2. CGO_ENABLED=1 `go test -race ./internal/btree -count=1 -v`: PASS, 79.446s. CGO is limited to race instrumentation.
3. CGO_ENABLED=0 `go test -tags oracle ./internal/btree -run '^TestBoundedSQLite354Oracle$' -count=1 -v`: PASS, 1.644s.
4. CGO_ENABLED=0 `go build ./...`: PASS.
5. CGO_ENABLED=0 `go vet ./internal/btree`: PASS.

All five exits are 0; no skip or failure appears in the three final test logs. The approved test-only CLI binary SHA-256 was independently checked against `470775f6ada29ccfb2d3046ac2b4c975df95a9c20b782332d4e8ba6b6233e6c6`. Its observed identity is SQLite 3.54.0, source ID `2026-10-09 15:46:58 be8d059e9a49089ab2dce5ed26dd87aaf598fdc5fbe0b107c0dd758464bcd5e3`. Production additions are native Go and invoke no external SQL process. This reviewer did not rerun the tagged oracle under race; the independent race suite covers default native tests, while the owner's separate oracle-race evidence is outside this review's measured timings.

Reproduction: copy the saved `.go.txt` reviewer source to its documented package path in a copy of the immutable v2 tree; read the audit; invoke `bash run-review.sh.txt SOURCE_DIR EVIDENCE_DIR` from this directory. Use an existing evidence directory. Each exact expanded command, guard output, result and exit status is preserved separately. No compiler/test command remained after final batch completion; additional invocations were held for the lead's cache-maintenance window.

## Remaining scope

This review validates true native bounded forward/reverse traversal, not SQL inequality extraction, affinity/error-safe planner fallback, ORDER BY or LIMIT elimination, covering reads, range diagnostics or transaction-level planner integration. DESC/mixed-direction indexes, custom collations, WITHOUT ROWID and concurrent cursor mutation remain unsupported. Work is a logical admission policy, not a complete heap or CPU-instruction metric. Cursors validate only visited/probed data, not entire unrelated subtrees. The current main branch with unrelated concurrent changes was not used as this review's test tree.

No physical database corruption/tamper/wrong-key/missing-file, namespace mutation, native failed I/O/close/unmap/lock/Sync/shortwrite, disk-full/permission fault, process kill/exit/crash, hot recovery or interrupted publication was exercised. Restricted native branches were source-reviewed only. Non-Linux native execution and full durability acceptance remain unqualified.
