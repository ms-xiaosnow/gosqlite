# Independent source and execution-scope audit

Read the full `goal.md`, `docs/test-scope-audit.md`, `docs/ordinary-ci.md`,
`docs/decisions/0004-exclusive-wal-and-recovery-gates.md`, relevant
compatibility/file-format/upstream text,
all `internal/journal` sources, the scope-guard implementation and its tests,
and `internal/pager/recovery.go`. No applicable AGENTS.md exists in the Go
checkout or its ancestors. The pinned SQLite source has its own AGENTS.md,
which was read; that upstream tree is not modified or built by this review.

## Source-only reference alignment

Pinned SQLite `src/pager.c`:
- `journalHdrOffset`, lines 1403-1413: round up to the next sector boundary,
  leaving an already aligned offset unchanged.
- `writeJournalHdr`, lines 1486-1600: each header carries its count, checksum
  initializer, original extent and geometry. Header padding is unused; later
  geometry repetition follows the writer format.
- `readJournalHdr`, lines 1627-1732: count, nonce and original extent are read
  from each header; only offset-zero header page/sector sizes configure playback.
- `pager_cksum`, lines 2289-2297: nonce plus bytes 200 apart backwards from the
  page end, excluding index zero; arithmetic wraps at uint32.
- `pager_playback_one_page`, lines 2336-2410: zero and locking-page numbers stop
  ordinary playback; out-of-extent pages are skipped by upstream. This planner
  instead rejects them. Upstream non-savepoint playback has no pDone duplicate
  filter; the planner's globally unique originals are a conservative subset.
- `pager_playback`, lines 2872-3000: first header determines original truncation
  extent; each segment uses its own advertised count. A hot zero count remains
  zero. No-sync/safe-append sentinel, stop-at-invalid-record behavior, and the
  non-hot unpublished final-body inference are deliberately outside this plan.

The verified archive's pager.c is byte-identical to the extracted reference.
`upstream-pin.json` records archive size/SHA3-256, source identity and pager hash.
No SQLite process or native recovery consumer is executed.

## Memory ownership and quotas

The source retains only validated input views until every segment is accepted;
these views cannot escape on error, because every failure returns a nil plan.
Success copies each page separately. Journal bytes bound input, scans, empty
headers and copied image bytes. Cumulative record limit bounds metadata. First
page-size/sector bounds and uint32 counts make count multiplication fit uint64;
remaining-byte subtraction precedes offset addition and slicing. Original
extent is at most `(2^32-2)*65536`, safely within int64. Zero-count chunks advance
by at least the first sector size and do not create a separate metadata list.

## Actual selected test/helper behavior

Existing `format_test.go` and `replay_test.go` construct byte slices and call
format/parser/model functions only. New `replay_segments_test.go` constructs
published segment byte slices directly, uses math/rand deterministic data,
mutates only in-memory input bytes, and models application in allocated slices.
Its fuzz target calls only the parser/extent validator. No helper imports or
calls filesystem, native pager/VFS, process, CLI or recovery operations.

Independent `replay_independent_test.go.txt` follows the same boundary in an
external journal_test package. Its synthetic sparse checksum and geometry
builder do not call the production encoder or checksum. Its every-prefix loop
recognizes complete earlier segment endpoints as complete input in their own
right. Every other cut rejects without a partial plan. Model histories compare
missing-page coverage, unchanged pages and idempotence entirely in byte slices.
Source-mutation negative controls, if executed, alter only isolated parser Go
source and run these byte-only tests; they are not physical fault injection.

Before every execution batch run `go run ./internal/testscope`; manual tags
are never passed. Builds and cross-compilation may inspect native consumers
without running them. No aggregate suites, ordinary databases or native
recovery cases are run in this review. No CI run is started. No cache is
cleared and no process is killed. Physical durability, native platform behavior,
M3 acceptance and automatic Open integration remain unverified/excluded.
