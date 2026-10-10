# Bounded native B-tree cursor prerequisite

This change adds low-level native forward/reverse interval cursors. It is a
reviewable prerequisite, **not planner acceptance**: no SQL range selection,
ORDER BY elimination, covering read, or EXPLAIN claim changes. Existing scanner,
equality seek, equality-prefix access, JOIN preallocation, public existing type
layouts, and file formats are unchanged. All implementation changes are additive
files under `internal/btree`.

## API and endpoint semantics

- `NewRowIDRangeCursor(ctx, reader, root, lower, upper, reverse, limits)` yields
  `(signed rowid, owned record fields, error)` from a rowid-table B-tree.
- `NewBoundedIndexCursor(ctx, reader, root, info, lower, upper, reverse, limits)`
  yields a complete owned index key, including its trailing signed `int64` rowid.
- Both use `Next`, `Stats`, and `Close`. Set `reverse=true` to seek from the upper
  endpoint and walk in descending physical key order; endpoint meaning does not
  change when direction changes. A nil endpoint is unbounded.
- `RowIDBound{Value, Inclusive}` specifies `>=`/`<=` when inclusive, `>`/`<`
  otherwise. The implementation never increments/decrements the endpoint, so
  `MinInt64` and `MaxInt64` remain exact.
- `IndexBound{Key, Inclusive}` takes 1 through `info.Columns+1` actual leading
  fields. A complete key's final field must be `int64`. Empty non-nil bounds and
  unsupported field types return `record.ErrUnsupported` before reading pages.
- Partial index bounds compare whole fields. For index `(a,b,rowid)`, lower
  `['x']` inclusive includes **all** `a='x'` suffixes; exclusive excludes **all**
  such suffixes. The same distinction applies to an upper bound. Lower and upper
  may have different prefix lengths. Equal inclusive prefixes form an equality
  prefix interval; differing lengths are cuts before/after whole prefix groups.
  There is no artificial NULL, minimum rowid, or substring sentinel padding.
- Ordering is SQLite storage order: NULL, numeric, TEXT, BLOB; numeric comparisons
  preserve mixed INTEGER/REAL precision at 2^53 and int64 boundaries. Text and
  BLOB comparisons are BINARY byte comparisons, including embedded NUL. NaN
  bounds compare as NULL, as in the existing index comparator. NULL endpoint
  fields use physical key ordering, **not SQL three-valued predicate logic**.
- Repeated indexed values are distinguished by all remaining indexed fields and
  the signed trailing rowid. A complete index key is unique. Input slices/BLOBs
  may be reused after construction; output mutation cannot change later results
  or validation. The reader must expose one coherent snapshot and cursor methods
  must not execute concurrently.

## Native seek and termination

The initial endpoint is located by binary search on each visited page, retaining
only the descent path. Earlier/later unrelated subtrees are not scanned or
materialized. Rowid-table interior cells are separators. Index interior cells
own records, so forward traversal is child/record/child and reverse traversal is
its mirror. Exact inclusive full-key interior seeks start directly at the record
without visiting its preceding (or following, in reverse) child.

Far endpoints are checked before returning data; strict parent separators also
allow pruning the next whole subtree. After returning an exact terminal full key
or rowid, the cursor finishes immediately, without reading an adjacent subtree.
Partial endpoints can require inspecting a following nonmatching record to prove
completion; its page and payload work is charged. Only visited pages and probed
keys are validated. These cursors are not a whole-file consistency scan.

A statically empty interval returns EOF without page reads. EOF is distinct from
quota, cancellation, and corruption errors. All terminal results are sticky and
release paths/ownership maps/copied keys. Explicit `Close` changes the terminal
state to EOF; it does not close the borrowed reader. A caller that has received
some records must still check the final error: a later quota failure does not
turn partial output into a successfully completed interval.

## Work admission and counters

`RangeLimits` uses positive cumulative limits; zero selects finite defaults and
negative values are rejected:

- `PageReads`: 1,048,576 logical underlying `ReadPage` calls, including root,
  descent, overflow, repeated overflow reads, and attempted failed calls.
- `Work`: 16,777,216 units. One unit per page call, binary-search comparison, and
  record decode admission, plus bytes reserved by endpoint-copy and page-parser
  admission. This
  intentionally combines a parser-allocation cost with traversal work and is not
  a CPU-instruction, elapsed-time, or physical-I/O metric.
- `Records`: 1,048,576 predecode admissions, including probes, repeated decoding,
  and endpoint checks, not just output records.
- `PayloadBytes`: 67,108,864 total encoded payload bytes admitted before overflow
  buffer allocation and decoding. Repeated decode attempts count again.

Existing payload, column, depth (64), and visited-page limits also remain active.
`Stats` keeps admitted counters and the count of successful `Next` results after
EOF/error/close. Rejected requests do not increment the rejected counter. Read
attempts admitted before cancellation are retained. `BeforeRecord(int)` and
`BeforePage(int64)` hooks on the supplied reader are forwarded. Rowid parsing
retains the existing exact JOIN allocation-preflight path; the new index cursor
uses a conservative cell/span allocation admission before the unchanged index
parser. No existing cursor struct or helper was modified.

Before allocating either index endpoint, the combined endpoint structs,
interface slices, and cloned BLOB bytes are admitted against Work with checked
addition. Immutable strings share their existing backing storage. Statically
empty intervals copy neither endpoint. Every endpoint must also pass the existing
maximum-key validation. PayloadBytes remains specifically predecode payload
work; it does not count endpoint copies. These policies are not an exact total
heap limit: caller retention, shared string storage, and normal Go allocator
rounding/overhead are not represented. Optional upstream hooks can impose tighter
reservations. Cancellation inside an upstream BeforePage hook is rechecked before
parser allocation, just as it is for BeforeRecord.

## Independent ordinary evidence

Official test-only CLI: SQLite 3.54.0, source ID
`2026-10-09 15:46:58 be8d059e9a49089ab2dce5ed26dd87aaf598fdc5fbe0b107c0dd758464bcd5e3`.
Reference source locations are `src/btree.c` table/index seek and next/previous
routines, and `src/vdbeaux.c` `sqlite3IntFloatCompare`/`sqlite3MemCompare`.
The production implementation remains pure Go and invokes no oracle process.

Coverage includes:

- 3,000 randomized composite endpoint/direction cases over descriptor-free
  balanced trees, including different bound lengths and exclusive prefix cuts.
- 2,166 rowid interval/direction combinations over intact official fixtures at
  512/4096/65536-byte page sizes, signed extremes, and overflowing records.
- 846 direct official-CLI interval/direction answers at those page sizes. Expected
  results use the CLI's NULL-safe CASE, IS, comparison and ORDER BY operations,
  independently of the Go comparator. Affinity-free indexed columns expose
  NULL, INTEGER, REAL, TEXT, and BLOB storage-class distinctions.
- Exact in-memory page traces proving selective traversal and termination:
  an interior singleton in either direction reads `[2,4]`; forward `q..t`
  reads `[2,4,5]`; reverse `z..t` reads `[2,4,6]`, without the adjacent child.
- Actual `PageReader` calls on a fresh 1,800-row official database:

| Page size | Index singleton forward/reverse | Complete index scan | Rowid 3-row interval, each direction | Complete table scan |
|---|---:|---:|---:|---:|
| 512 | 6 / 3 | 345 | 3 | 366 |
| 4096 | 5 / 5 | 112 | 2 | 46 |
| 65536 | 2 / 2 | 4 | 2 | 4 |

These index counts include overflow probe reads, and are not just unique pages.
The instrumentation and `Stats.PageReads` must agree exactly. Full traces are
preserved in the final tagged-oracle unit/race logs. Database SHA-256 checks are unchanged after
all ranges, followed by ordinary close/reopen and an extreme-rowid read.

Quota admission, inherited hooks, caller ownership, cancellation (including
rereading already-probed overflow), sticky errors/EOF, and ordinary close are
also checked. The complete default-tag B-tree test package retains the original
scanner/equality/prefix/mutation/parser and JOIN-allocation regressions.

## Reproduction and remaining code

Read `docs/evidence/range-cursor/behavioral-audit.md`, then from the source root run
`bash docs/evidence/range-cursor/run-checked.sh`. It uses the sibling approved Go
1.27.2 toolchain, shared `go-cache`/`go-path`, offline module settings, and the
fixed official CLI. Every unit/race/tagged-oracle/build/vet command is preceded by
`go run ./internal/testscope`. Unit/build/vet use CGO=0; race is separate CGO=1.
The CLI oracle test is isolated behind the established `oracle` build tag. The
separate `go test -tags oracle ./internal/btree -run '^TestBoundedSQLite354Oracle$'`
selection requires `SQLITE_ORACLE`; missing configuration fails explicitly. The
default package contains no CLI-backed test and silently skips no required
oracle. Recorded tagged runs set the approved path and run all 846 cases.

Still required before planner acceptance: independent core review; extraction
of SQL inequality/prefix constraints with comparison affinity and eager-error
safety; planner admission/fallback policy; range diagnostics; ordering/LIMIT/tie
semantics; SQL-level differential, transaction, and quota integration tests.
DESC/mixed-direction indexes, custom collations, WITHOUT ROWID trees, covering
reads, SQL ORDER BY elimination, reusable repositioning, and concurrent cursor
mutation are not implemented here. No physical fault/crash/recovery gate was run
or claimed. Main-repository staging, integration, publication, and Library work
belong to the lead after review.
