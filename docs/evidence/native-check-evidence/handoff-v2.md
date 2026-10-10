# Native integrity check: repaired frozen v2

V2 supersedes the v1 implementation handoff. Independent v1 probes found real
false-success gaps: unaccounted B-tree cell-content bytes/incorrect fragment
counts, and application registry semantics impersonating the built-in grams13
name/version (including unknown persisted version 2). The original v1 freeze,
patch, checks and independent failing logs remain intact. They are not acceptance
for the repaired slice.

## Exact identity

- Source: `native-check-freeze-v2`
- Frozen tree: `99ee54afe1900ac0e9b814ac8f57772f6b76e493`
- Same composed baseline: `0374382bdef1c61a7f5b36092a42109046b100dd`
- Full minimal patch: `native-check-evidence/source-v2.patch`
- SHA-256: `819b54f6af690a2ede21e78165dc5a07da4f5e608d70e5d398579c703487dafd`
- Additional reviewed delta: `native-check-evidence/v1-to-v2.patch`

The baseline still comprises rollback-reader-acquisition tree
`ad066d04d676b8368ca7d02d83664e50dfa47216` plus the checked reviewed CLI
backup/rekey provider patch SHA
`c7893f8a1c6f686cf137cca15dbfa4cdf670e986625f2bf863e01caf80e8c359`.

Ten owned paths, 1,339 final files versus 1,330 baseline files. The eight v1 paths
remain owned, plus two additive paths:

- `internal/btree/check_space.go`
- `sqlite/check_review_test.go`

Existing pager, B-tree cursor/parser, registry, public Options/Result and owner
contract files remain byte/mode identical to the baseline. `identity-v2.json`,
`owned-v2-paths.txt`, `owned-v2-inventory.json`, full freeze inventory/SHA list and
`native-check-patch-verify-v2` establish exact checked replay and modes. No main or
remote writes, commits, uploads or dependency downloads occurred.

## Repairs

### Exact cell-content accounting

New additive `btree.CheckTreeSpace` traverses only known catalog/table/index
B-tree pages, using already validated parsed cell lengths/ends. It reconciles
all bytes from the cell-content boundary to the usable-page end against cells,
freeblocks and 1–3-byte fragments, and checks the exact fragment-byte total.
Unrepresented gaps of four or more bytes and incorrect declared fragment counts
fail. Empty/schema/table/index/interior/leaf pages are covered. Overflow and
freelist pages are not guessed from their first byte; their existing ownership
and encoding validators remain responsible for them. The whole traversal uses
the same bounded check reader and contributes to PageReads.

Original memory negatives now fail as required. Positive synthetic fragments and
freeblocks, empty schema and actual multi-level table/index trees pass. New valid
official fixtures at 512/4096/65536 use 900 rows, normal DELETE and variable-size
UPDATE to exercise real fragmentation; official integrity_check and native Check
both pass with 600 surviving source/index entries.

### Fixed built-in persisted semantics

Full Check creates a fresh built-in-only validator and retains the connection's
existing resource options. It never calls application Configure/Keys, even when
an application supplies a registry entry named grams13 v1. Unknown persisted
name/version remains unsupported. Supported grams13 v1 data is independently
checked against actual built-in semantics.

This intentionally does not audit runtime-registry provenance. A benign wrapper
can pass only because the data satisfies the built-in profile, and callbacks
remain uninvoked. The original blanket custom-registry rejection wording has
been replaced by this exact guarantee with reviewer agreement. This is not just
a changed assertion: a new detached-memory fixture has a nonempty source with
absent built-in postings; the custom empty-key named check can accept it, while
full Check rejects it without calling either custom Configure or Keys. Version2
and callback-count regressions are included. Named application-directed extension
checks retain their established behavior.

### Explicit accounting/decoding boundaries

Docs and API comments now state that check-specific page/read quotas begin only
after snapshot acquisition. Existing reader/session and lazy writable Tx
acquisition can perform header/freelist work under the existing pager caps before
these counters start. They are traversal quotas, not total-open/heap limits.

IPK validation promises a decoded logical NULL slot, following the existing
record decoder's REAL-NaN-to-NULL conversion. It does not assert a physical
serial-type-0-only audit. No existing record decoder was altered.

## Exact frozen verification

`checks-v2.sh` and each frozen-v2 `.command`/`.txt`/`.exit` record the run. All twelve
gates passed, including source guards before normal and race:

- Go 1.27.2 and pinned SQLite 3.54.0 version/source-ID
- Scoped native/API/CLI/independent memory regressions and legacy CLI smoke:
  sqlite 2.122s, cmd/gosqlite 0.060s
- Same scoped race: sqlite 7.864s, cmd/gosqlite 1.380s
- Vet of sqlite, CLI, example and the newly owned internal/btree package
- Pure-Go executable build, no CGO production dependencies, module verification
- Actual executable API/CLI and unchanged-family proof across plaintext and
  correct-key encrypted snapshot/WAL/shared modes

The actual proof counts are identical across all four modes: 10 pages, 5 tables,
4 physical indexes, 1 logical extension, 232 table rows, 231 index entries and
960 logical reads (ten additional tree-space reads compared with v1). Counts
include shadow data. The proof prints only mode/counts.

Audit additions are in `test-audit-v2.md`. Independent original failures are
retained under `native-check-independent-evidence/negative-probes-v1.log` and
`version-negative-v1.log`; their originals were not overwritten. Independent v2
review/reverification is still owned by the reviewer and parent.

Full plaintext WAL CLI remains the explicit implementation gap approved by the
parent; its full-check flags reject before opening. Existing configured WAL
DB/Tx.Check support remains. No physical tamper/corruption/wrong-key/missing-file,
namespace mutation, native I/O/lock/unmap/close/Sync/shortwrite/diskfull/permission
failure, crash/kill/abrupt exit/hot recovery, interrupted publication/rekey or
manual-tag scenario ran. No broad old-base aggregate test was run. Ordinary
Linux checks do not establish native other-OS, crash, durability, security or
release qualification.
