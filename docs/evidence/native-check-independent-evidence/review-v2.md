# Native integrity checker independent review

Verdict: ACCEPT for the documented bounded native supported-profile integrity/index-consistency slice, after v2 repairs. This is not general SQLite integrity_check parity, full-repository acceptance, physical durability/security qualification, or the missing plaintext read-only WAL CLI implementation.

## Exact identity and ownership

- Accepted baseline: 0374382bdef1c61a7f5b36092a42109046b100dd, 1,330 files.
- Reviewed v2 frozen candidate: 99ee54afe1900ac0e9b814ac8f57772f6b76e493, 1,339 files.
- Owner patch: native-check-evidence/source-v2.patch, SHA-256 819b54f6af690a2ede21e78165dc5a07da4f5e608d70e5d398579c703487dafd.
- Fresh independent patch replay: native-check-independent-replay-v2. Complete bytes, exact filesystem modes and path inventory equal the frozen candidate. Ten owned paths: nine additions plus the narrow cmd/gosqlite/main.go dispatcher/help change. Every other baseline file/mode is unchanged; Options/Result/provider/pager contracts remain unchanged.
- Independent full inventory was recomputed directly, rather than trusting owner manifests. See identity-v2.json and final-source-readback-v2.json. The four additional independent probe files live only in native-check-independent-probes-v2; all its existing source/test bytes/modes match the frozen candidate.

## Issues found and repaired

V1 tree384f19e2 was not accepted. Retained pure-memory failures:

1. False successful full checks with incorrect fragment counts on schema/table/index pages, and an eight-byte unrepresented gap inside table cell-content space. Existing parsers checked overlaps but not complete byte accounting. V2 adds internal/btree/check_space.go after ownership validation, traversing only known B-tree pages, checking cells/freeblocks/1–3-byte fragments and exact header fragment count. No old parser was changed. Overflow remains separately validated/owned by existing cursors.
2. Application-supplied grams13-name registry implementations, including unknown persisted version2, could redefine checker truth. V2 constructs a fresh built-in validator, preserves the connection limits, and never calls custom Configure/Keys. Unknown persisted versions fail. A detached source whose expected builtin postings are absent fails even when a custom empty-key named checker would pass it. Documentation now accurately describes persisted builtin semantics, rather than runtime registry provenance rejection.
3. Scope wording clarified: check quotas start after snapshot acquisition; existing reader/Tx header/freelist preparation retains its pager caps outside traversal counts. IPK validation requires decoded logical NULL; it does not claim a physical serial-type0-only audit because record decoding maps REAL NaN to NULL.

Original findings and failures are retained in negative-probes-v1.log and version-negative-v1.log. The benign custom-wrapper assertion changed intentionally with the clarified contract: v2 must pass genuinely builtin-consistent data while making zero custom callback calls. Unknown versions and missing builtin postings still must fail, with zero CheckResult.

## Independently executed evidence

Pinned Go1.27.2, offline module settings/local caches, CGO0 except explicit CGO1 race. Central scope guard preceded each final selected suite; bodies/helpers were behaviorally audited. No broad old-baseline aggregate was run.

- Exact clean v2 replay: all TestNativeCheck*, TestCLINativeCheck*, and included TestIndependentCheck* normal and race passed. This includes intact plaintext/correct-key encrypted modes, page sizes512/4096/65536, overflow/freelist/rollback, official supported and unsupported fixtures, typed grams/HNSW/IVF, and memory malformed fixtures.
- Additional independent native/driver/math probes passed normal and race. These cover deferred Check without writer promotion (a peer can acquire RESERVED), quota failure retaining caller Rows and transaction, own writes/savepoint rollback, changes()/total_changes()/last_insert_rowid() preservation through database/sql Raw, and unrelated Rows lifetime with ordinary WAL-session BUSY.
- Concurrent shared-WAL source plus grams/HNSW shadows remain in the old pinned snapshot across eleven normal peer commits; later post-rollback Check sees the new state.
- Twelve valid official unsupported DDL profiles fail closed: UNIQUE, CHECK, FK, WITHOUT ROWID, STRICT, non-BINARY column/index collation, expression/partial index, view, arbitrary trigger and generated column.
- Independent official fragmented fixtures created by valid INSERT/DELETE/shortening UPDATE pass official integrity_check and native Check at512/4096/65536, with150 source rows/300 physical index entries. Owner's separate900-row valid fragmented fixtures also passed in exact replay.
- Exact numeric/storage-class probes cover signed int64 limits, adjacent binary64 values around2^53 and2^63, infinities, subnormals, signed zero, NULL/text/BLOB. Equal integer/REAL numeric keys pass; same-cardinality wrong-value replacements fail.
- Real CGO0 executable proof passed in plaintext and correct-key encrypted snapshot/WAL/shared. API and CLI full result/counts match; source family names/bytes unchanged. Each mode reports10 pages,5 tables,4 physical indexes,1 extension,232 table rows,231 index entries,960 traversal reads.
- Historical named --wal-exclusive/--wal-shared CLI routes passed independently, while full plaintext-WAL flags reject before open and leave family hashes unchanged.
- Focused vet passed for sqlite, driver, cmd/gosqlite and internal/btree. All final .exit files are retained with commands/logs.

## Source review conclusions and limits

A single checkReader wraps the held source snapshot and is used for catalog, free pages, ownership, space accounting, ordinary table/index reconciliation and all extension checks. Both index directions are mandatory; per-entry numeric comparison and rowid lookup prevent cardinality-only success. Global shadow ownership closes named-index orphan omissions. The APIs do not call writer promotion or repair; they retain caller-owned resources and counters, and return zero results on errors. Standalone reader errors join close errors. CLI success serialization occurs only after DB.Close; encrypted failures use the reviewed redacted adapter.

Native full DB/Tx checks work on already-configured pinned WAL sessions. Full plaintext WAL CLI remains explicitly unsupported before opening; named extension WAL routes intentionally retain historical writable coordination. Read quotas are cumulative after acquisition, not aggregate heap limits or a wall-time guarantee. Profile exclusions, ANN-quality limits, logical-NULL IPK semantics, and initial acquisition accounting remain explicit in docs/native-check.md.

NOT_RUN: physical tamper/corruption/wrong-key/missing-selected-file or selected-file deletion, namespace replacement/race, crash/kill/abrupt exit/hot recovery, native I/O/lock/unmap/close/Sync/shortwrite/diskfull/permission faults, interrupted publication/rekey, manual tags, Darwin/Windows native runtime and security/release acceptance. No forbidden case was moved to CI or another executor. Final aggregate integration, central docs/status, remote publication and CI belong to the parent.
