# Bounded multi-segment rollback replay plans (memory only)

Status: implemented conservative parser subset; selected pure-memory evidence
only. Physical rollback recovery, interruption, native failures and automatic
Open recovery remain **NOT_RUN / unqualified**. This slice neither changes
`internal/pager/recovery.go` nor invokes its `RecoverRollback` function.

## Supported input and stable result contract

`internal/journal.ParseReplay` accepts one or more complete sector-aligned,
published, finite-count segments with the following restrictions:

- The first header supplies page size (power of two, 512..65536), sector size
  (power of two, 32..65536), and original page count (0..0xfffffffe).
- Every later header must repeat those three values exactly. Each header's own
  nonce and finite record count apply only to its records. Equal nonces are
  permitted; nonce uniqueness is not a format or authentication guarantee.
- The next header is at the sector boundary at or immediately after the prior
  segment's record end. Alignment always uses the first header's sector size.
  Header-sector and inter-segment padding are unspecified, including nonzero
  bytes and apparent magic away from the prescribed boundary.
- Published zero-count segments, including first, interior and final segments,
  contain no records. No unpublished count is inferred from remaining bytes.
- Every record has a valid original page number and nonce-seeded sparse
  checksum. Page zero, the locking page and pages beyond the original extent
  are rejected. Original page numbers are globally unique across all segments.
- The final segment ends exactly at the end of the supplied bytes. There is no
  trailing padding-only success, magic search, stale-tail inference or partial
  replay plan. Every error returns a nil plan and leaves the input unchanged.

The exported structures are unchanged. `ReplayPlan.Header` remains the first
header exactly, including its first-segment `Records` and `Nonce`. The cumulative
image count is `len(ReplayPlan.Pages)`; callers must not reinterpret
`Header.Records` as that count. Images are in journal order and own independent
byte slices. Copies occur only after every segment passes validation.
`DatabaseBytes` and `ValidateExtent` still use the first original extent.

Every previously successful single-segment input retains the same values and
owned-image semantics. Existing malformed/unsupported single-segment fixtures
retain rejection. An exact prefix ending at an earlier segment's record end is
itself a complete supported journal: byte parsing cannot know that another
segment was omitted. All other partial prefixes reject. Neither a valid plan nor
a valid prefix proves the journal's generation, database identity or hotness.

## Cumulative limits and arithmetic

The existing three-field `ReplayLimits` layout is preserved; zero defaults are
64 MiB input, 65536 records, and 1 TiB original database extent.

- `MaxJournalBytes` applies to the entire byte slice, including every header and
  padding gap, before parsing.
- `MaxRecords` applies to the sum of all segments' advertised finite counts,
  using uint64 subtraction before addition. A pair of individually permitted
  uint32 counts cannot wrap the cumulative budget. Counts are also checked
  against remaining bytes before any per-record metadata is allocated.
- Total copied page-image bytes equal `len(Pages) * first.PageSize`. Records are
  disjoint input ranges, so this is no greater than actual input bytes and
  `MaxJournalBytes`. It is also at most `MaxRecords * first.PageSize` and, because
  original page numbers are unique, at most the original database extent.
- Page metadata and the global duplicate set are O(cumulative records); there
  is no retained list of segment headers. A zero-record segment still advances
  by a complete sector, at least 32 bytes, so input size bounds traversal work.
  These algorithmic limits do not promise an exact allocator or process RSS cap.
- Record-count multiplication uses uint64 (uint32 count times at most 65544).
  Remaining-input subtraction is checked before offset addition/indexing.
  Alignment padding is at most sector size minus one and checked before adding.
  Valid original extents fit int64: at most 0xfffffffe times 65536 bytes.

`ValidateExtent` continues to require an image for every original page absent
or only partially present in a shortened in-memory extent, except the unused
locking page. The tests apply plans only to byte arrays and check exact results,
unaffected pages, planned truncation and idempotence.

## Explicit unsupported matrix

| Input or operation | Result / boundary |
|---|---|
| Published finite-count, consistent geometry, globally unique pages | Bounded owned memory plan |
| Repeated page within/across segments, even identical bytes | Unsupported; no first/last-wins policy |
| Later geometry or original-extent disagreement | Unsupported; first geometry is never overwritten |
| 0xffffffff count (no-sync / safe-append), any segment | Unsupported |
| Legacy first page size zero | Unsupported |
| Unpublished first header | Invalid; no plan |
| Unpublished or unrecognized subsequent header | Unsupported; no partial plan |
| Partial published header-sector/records | Rejected; no partial plan |
| Trailing partial header/padding/extra data or unaligned header | Unsupported |
| Apparent super/master-journal trailer, including malformed magic trailer | Unsupported; no external lookup |
| Invalid page/checksum/first geometry | Invalid; no partial plan |
| Any cumulative quota violation | Resource-limit error; no partial plan |
| Native replay, hotness/identity/locking, crash or I/O-failure behavior | Unchanged source only; NOT_RUN |
| Automatic recovery through Open, public SQL or driver | Not implemented by this slice |

## Pinned public-source basis

Read-only reference: official SQLite 3.54.0 source archive already pinned in
[upstream.md](upstream.md), source ID
`be8d059e9a49089ab2dce5ed26dd87aaf598fdc5fbe0b107c0dd758464bcd5e3`.
The reviewed `src/pager.c` SHA-256 is
`96d7bb9c4a5a6da0909ce0ab4714311e1ffa5f13007dc4c69f028a2760104ba4`.

- `journalHdrOffset`, lines 1388..1413: next boundary at or after record end.
- `writeJournalHdr`, lines 1471..1608: fields, finite-count publication, fresh
  checksum initializer and unused header-sector bytes.
- `readJournalHdr`, lines 1627..1731: each segment's count/nonce; page/sector
  geometry is read only from the first header.
- `pager_playback`, lines 2909..3005: hot zero counts mean zero records; no-sync
  count inference is a separate case; original extent is adopted only once.
- `pager_playback_one_page`, lines 2336..2420: ordinary playback receives no
  duplicate-suppression bitset and replays in order. This planner deliberately
  rejects repeated pages rather than adopting savepoint first-wins semantics.
- `syncJournal`, lines 4356..4434: published finite counts and invalidation of
  an apparent old next header. This parser does not infer stale-tail provenance.

No SQLite CLI was run and no hot journal or database fixture was generated for
this slice. Public-source inspection is not physical recovery acceptance.

## Reproducible selected checks

The fixture builder uses only byte arrays and independent alignment/checksum
math, never the production journal encoder. Coverage includes 96 page/sector
pairs, every prefix of a multi-segment fixture, nonzero padding, zero-count
chains, equal/different nonces, cross-segment duplicates, late errors, cumulative
quota boundaries and uint32 overflow advertisements, independent-copy ownership,
128 deterministic randomized extent/application models, and bounded byte fuzz.

All ordinary test sources and helpers in `internal/journal` were behaviorally
reviewed as memory-only; no `TestMain`, init hook, filesystem or pager path is
present. Before each selected normal/race/fuzz suite, run the source guard:

```sh
go run ./internal/testscope
go test ./internal/journal -run '^(TestReplay.*InMemory|TestSQLiteRollbackFormatInMemory|TestRejectMalformedJournalBytesInMemory|FuzzReplay.*BytesOnly)$' -count=1
# Repeat the source guard before this separate race-tooling gate:
CGO_ENABLED=1 go test -race ./internal/journal -run '^(TestReplay.*InMemory|TestSQLiteRollbackFormatInMemory|TestRejectMalformedJournalBytesInMemory|FuzzReplay.*BytesOnly)$' -count=1
# Repeat the source guard before each bounded pure-memory fuzz gate:
go test ./internal/journal -run '^$' -fuzz '^FuzzReplaySegmentsBytesOnly$' -fuzztime=5s -parallel=2
go test ./internal/journal -run '^$' -fuzz '^FuzzReplayPlanBytesOnly$' -fuzztime=5s -parallel=2
```

Use pinned Go 1.27.2, `GOTOOLCHAIN=local GOPROXY=off GOSUMDB=off
GOFLAGS=-buildvcs=false`, sibling shared Go cache/path, and `GOMAXPROCS=2`.
Normal and fuzz runs use `CGO_ENABLED=0`; only race tooling uses CGO. Build/vet
and untagged pager test compilation are separate non-executing checks. No manual
build tag, restricted physical/native-failure test, or aggregate gate is part of
these commands. The lead owns aggregate validation and independent review.
