# Architecture

The goal is a native Go engine, with SQLite 3 pages as its plaintext compatibility boundary. M2 adds native transaction overlays, local B-tree mutation, rollback-journal publication and SQL writes. M3 adds explicit exclusive/shared WAL paths and isolated recovery source. M4 adds transactional persistent inverted indices and a real candidate planner. M5 adds snapshot-bound exact text similarity and SQL scalars. Native vector, typed ANN, encrypted storage and offline migration layers are now implemented within the current compatibility matrix. Qualified recovery, security and mandatory release evidence remain unfinished.

Target data path:

    database/sql driver → public sqlite API → SQL parser/binder/planner → executor
                              ↓ transactions and schema                  ↓
                 registry / grams13 / vector index → B-tree / record → pager
                                                                         ↓
                                                             journal / codec / VFS

The engine uses bounded page access and query context cancellation. Connections and transaction snapshots, not a process-wide interpreter mutex, own execution. Writers must use native file locks before publishing SQLite pages; readers must never ignore hot journals or WAL.

Persisted extension indices share the table transaction. Encryption sits below logical pages, with an independent authenticated physical container. Unsupported formats and semantics fail closed. Go structs are not disk formats.

Initial M1 path (retained architectural history): cmd/gosqlite query/inspect or database/sql driver → sqlite pull executor → internal/sql expressions/parser + schema interpretation → streaming internal/btree cursor + internal/record decoder → read-only internal/pager and native Linux shared locks. The public sqlite package currently owns the small pull executor rather than creating a placeholder VM package. ORDER BY is separately bounded; ordinary scans stream. The PageReader abstraction decouples traversal from the physical reader for future transaction overlays. See ADR0002 for lock lifetimes, quotas and rejected modes.

Implemented M2 write path: parsed CREATE/INSERT/UPDATE/DELETE → full supported-catalog and page-ownership validation → statement undo snapshot → local table/index B-tree operations → bounded pager dirty overlay and SQLite freelist → native RESERVED/PENDING/EXCLUSIVE locks → standard DELETE rollback journal and database publication. Successful statements remain in an explicit transaction; a rejected statement restores its own overlay without erasing earlier statements. No page spill is used. Ordinary index pages are maintained; M4 uses covering extension posting indices for its supported predicates. Eligible rowid equality/inequality and leading BINARY ASC equality/inequality SELECT paths now use genuine bounded range/seek planning. Proved rowid ordering can stream; composite equality-plus-range, secondary covering/order and statistics remain open.

Idle connections release snapshots; each query or transaction owns its file handle and locks. Native Commit returning BUSY before physical I/O can be retried after readers close. database/sql invalidates a Tx after any Commit result, so its adapter rolls back after a failed commit. Once commit physical I/O begins, cancellation is deferred until the operation resolves. Any ambiguous physical-I/O result poisons the handle; it cannot be used as a successful commit.

Recovery of an existing journal is isolated source-only and not automatically invoked. All writable opens refuse any journal sidecar. Source-ordering review and normal commit tests do not prove crash atomicity. See ADR0003 and manual-verification.md. Planned packages are created only with real implementation, not placeholder interfaces.


M3 WAL route: DB(WALExclusive) → pager.Session retaining one EXCLUSIVE descriptor → Reader/Tx lease → dirty overlay → native WAL codec/committed-page index → synced WAL frames. Query, Schema and Begin all use the owning session; they never open an independent file descriptor against its lock. Checkpoint extends sector protection for imported logs, syncs WAL, copies committed images to main, syncs main, then removes sidecars and syncs the directory. Default Close checkpoints; explicit KeepWALOnClose preserves normally committed frames. Shared WAL is a separate route below.

Rollback recovery is a separate explicit internal operation. Its parser/planner is tested only in memory; physical replay is source-reviewed and compile-checked but not executed. Normal Open retains the fail-closed journal policy. This boundary prevents an ordinary test or user query from implicitly entering an unqualified recovery workflow.


M3 shared route: DB(WALShared) → pager.SharedSession retaining SHARED database and DMS locks → readmark-pinned Reader/Tx → standard mmap dual-header/hash WAL-index → single WRITE-locked append and Sync-before-publish. Checkpoint coordinates CKPT/WRITE/read slots and retains all inodes. Independent connections own independent snapshots. See ADR0006 for conservative storage assumptions and unverified fault gates.

M4 read route: parser/binder → snapshot-bound extension catalog + immutable per-connection method registry → selective necessary-gram frequency probes → actual B-tree prefix ranges → bounded rowid intersections/unions → source SeekTable → original WHERE verification. Unavailable or unsound filters return an explained scan. MATCH configuration is resolved within the same snapshot. Stable Plan/Stats show actual work, and EXPLAIN does not execute source/posting scans.

M4 write route: source mutation + registered key derivation → old/new posting delta + frequency metadata in the same statement/transaction overlay. Legal SQLite shadow schemas and exact invalidation guards permit cooperative official writers to mark indices dirty; source schema-cookie tracking covers VACUUM rowid changes. REINDEX is a bounded atomic statement and never blesses unrelated dirty indices. Registry handlers are trusted immutable application code; disk metadata never downloads or instantiates unknown code. See grams13.md for real APIs and limits.

M5 native route: named grams13 index plus physical field → snapshot-bound canonical MATCH config/generation → selected distinct query grams → complete union of every gram's covering posting range → source seeks → business WHERE → exact score → bounded Top-K. A positive score for a nonempty query set implies at least one common gram; zero threshold or empty sets require a full scan. Posting-budget exhaustion discards the complete attempted union and scans; it never truncates recall. Default SQL scalars use the same mathematical package with default normalization, and actual query/DML operation contexts reach bounded DP/set work. SQL expressions are not silently rewritten to native indexed search. RRF operates on explicitly supplied ranked lists and preserves one-based ranks/weights; same-snapshot native text/vector fusion is implemented by the M6 route below.

The independently written extension/vector package remains a pure mathematical/codec layer: immutable float32 values, strict text/binary validation, float64-compensated distances and bounded exact candidate scanning. It owns no database or ANN index. The sqlite layer now adds VECTOR(n) enforcement, canonical BLOB mutation and actual SQL/API plans; no ANN is implied.

M6 native route: typed column/closed metric identity → one snapshot → bounded source scan → bound business predicate → strict vector decoding → compensated exact score → bounded Top-K. SQL recognized distance ranking independently wraps the existing SCAN/grams source plan in a bounded LIMIT+OFFSET heap; NULL-first order and fallback sorter semantics are preserved. Typed values validate on reference and canonicalize only on writes. Hybrid text/vector lists share a read Tx before pure-Go RRF. See ADR0008 and vector.md.

## Persistent ANN layer

`extension/vector/ann` is a file-free deterministic graph algorithm over explicit
snapshot Reader and atomic-statement Writer ports. `sqlite/ann_storage.go` resolves
node/state/live identities through ordinary B-tree keys; query/open paths never
load an entire graph. Source data, graph deltas, tombstones, mapping and generation
commit together through the existing pager. `ann_query.go` performs cumulative
bounded widening, base identity/WHERE/vector rechecks and exact rescoring; the
planner exposes approximate recall separately from exact distance. Native exact
remains default; supported SQL may select ANN with NOT INDEXED as exact override.
The standalone full consistency checker is intentionally distinct from lazy
querying. See ADR0009 and ann.md. IVFFlat now shares the optional ANN family with a distinct canonical envelope, persistent centroid and covering membership B-trees. Training is bounded/deterministic; queries read selected lists and exactly rescore base values. See ann.md.

## Experimental encrypted snapshot storage

internal/encstore supplies independent authenticated root/base objects below the logical SQLite pager. internal/pager.EncryptedSession reuses transactional page overlays and statement undo while bypassing plaintext journal/WAL publication. Fresh file identities and fully authenticated object completeness precede page parsing. sqlite owns code KeyProvider configuration and rekey/backup/restore/import/export, and driver.NewConnector snapshots options. This first mode publishes a whole immutable base per changed transaction and retains history. The separate dirty-page WAL route follows below; the separate authenticated shared-session route implements simultaneous pinned readers and one writer within its documented platform profile. See encryption-storage.md for exact authority/locking, quotas and namespace/security limitations.

Encrypted WAL route: code-only provider -> authenticated current root -> immutable
base plus complete ordered dirty-page segment selection -> bounded logical page
location overlay -> the same native SQL/B-tree engine. Changed commits publish
final dirty images and a new root; checkpoint/rekey/copy stream the complete head.
A pinned Linux parent directory contains relative family operations. Lifetime
exclusive writer leases remain an intermediate concurrency model. See ADR0012.

Ordinary SELECT planning follows ANN and grams before rowid/IPK equality or a
nonunique BINARY ASC leading equality prefix. Bounded candidate rowids are sorted,
source rows sought and original predicates rechecked; conservative error-safety
proofs and resource fallback preserve scan behavior. Plan counters distinguish
index page-reader calls, matching keys and base-row seeks.

Migration inspection is a standalone bounded local parser/identity checker, not
a remote database client or a production SQL delegation path. Extended vector
primitives are immutable values beneath implemented HALF/BIT/SPARSE typed SQL
and supported typed ANN. They do not silently alter dense bytes or index identity.

Plain native file ownership now passes through `internal/vfs`: private managed
leases own OFD locks, stable identity checks, WAL-index mapping resources, and
close ordering. The pager still owns transaction/publication sequencing. SHM
slot mode changes require explicit release; Linux-native OFD conversion remains
separately tested. An unresolved view retains DMS and the shared connection's
main SHARED lease. Darwin process-fcntl/deferred-close and Windows managed
LockFileEx/identity/mapping backends are implemented and cross-built; only Linux
amd64 ordinary runtime is verified here. Target runtime and native failures are
separate qualification gates. See ADR0014 and its component evidence.

## Current bounded ownership and execution

Plain Reader/Tx/exclusive sessions, shared failed-open/successful-close paths
and encrypted constructors/copies retain bounded minimal managed obligations
with explicit pending/retry APIs. Public metadata admission precedes native
acquisition. Children retire before the protecting database owner; retries
retire obligations rather than repeating logical publication or checkpoint.
Recovery ownership and Open integration remain distinct pending corrections.

The SQL executor remains in sqlite with scoped pull operators, bound expression
metadata and conservative shared budgets. Global aggregates are implemented;
reviewed GROUP BY and connection-counter/cache additions await their separate
combined checkpoint. The current [compatibility matrix](compatibility.md) and
[acceptance work](acceptance-status.md) are authoritative; earlier architecture
text is preserved in history/before-acceptance-audit-architecture.md.
