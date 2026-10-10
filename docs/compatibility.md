# Compatibility matrix

Behavior baseline: official SQLite 3.54.0. M2 implements a deliberately bounded read/write SQL subset. A baseline version never implies full SQLite compatibility. Acceptance evidence is scoped to the commands in progress.md and docs/evidence.

| Domain | Status | Tested scope / boundary |
|---|---|---|
| Native Go builds | verified | Go 1.27.2, CGO_ENABLED=0; no production SQLite subprocess or C dependency |
| SQLite header/records | partial | format3, UTF-8, rollback-mode header; page sizes 512/4096/65536 exercised; integer widths/REAL/NULL/TEXT/BLOB |
| Rowid B-trees | partial | leaf/interior, signed rowids, overflow, streaming, schema root1 including schema overflow |
| Ordinary secondary indices | partial | create/traverse/seek/maintain nonunique BINARY ASC keys; index pages interoperate with official SQLite; SELECT index optimization unsupported |
| Native file locking | partial | Linux OFD shared/reserved/pending/exclusive byte-range locks; non-Linux Open fails explicitly |
| SELECT | partial | one rowid table or no FROM; projection, *, WHERE, parameters, ORDER BY, LIMIT/OFFSET; aliases and rowid/IPK semantics |
| Expressions | partial | NULL3VL, comparisons/affinity, arithmetic/bitwise/concat, ASCII-insensitive rune LIKE with ESCAPE, tested scalar builtins |
| Schema interpretation | partial | declared types INTEGER/INT/REAL/TEXT/BLOB/NUMERIC or typeless; supported INTEGER PRIMARY KEY, NOT NULL and literal DEFAULT enforced on writes; non-INTEGER PK metadata read-only |
| database/sql | partial | Connector, prepared reads/writes, named/positional binds, transactions, cancellation, streaming rows and resource closure |
| CREATE / CRUD / transactions | partial | native table/index creation, multirow INSERT, UPDATE/DELETE, deferred/immediate transactions, read-own-writes, transactional DDL, statement rollback; no savepoints |
| Journal publication | partial | standard SQLite DELETE journal and ordered Sync in successful commits; sector-size assumption explicit; error/crash paths unqualified |
| WAL / hot-journal recovery | unsupported | WAL mode/nonempty -wal and any -journal are rejected, including empty/cold journals |
| grams13 / indexed LIKE | unsupported | current LIKE is a truthful table scan |
| similarity / threshold / top-K | unsupported | M5 not started |
| dense vectors / exact KNN | unsupported | M6 not started |
| HNSW / IVFFlat | unsupported | M7 not started |
| encryption / rekey | unsupported | M8 not started; no SEE file compatibility |
| Neon migration / extended vectors | unsupported | M9 not started |
| release audits / native OS evidence | unsupported | M10 not started; cross-build success is not native-runtime evidence |

Unimplemented SQL/PRAGMAs return explicit errors. JOIN, aggregate/grouping, subqueries, CTEs, window functions, DISTINCT, UNION, scalar COLLATE (CREATE INDEX COLLATE BINARY is accepted), CASE, IN/BETWEEN, views/triggers, STRICT, WITHOUT ROWID, autovacuum, UTF-16 and unlisted schema grammar remain unsupported. Some storage mechanisms can safely coexist in a read-only file without their SQL being executable.

ORDER BY materializes under a configurable memory cap; unsorted results stream. Idle handles do not hold shared locks. Query Rows and explicit Tx own snapshots, including read-only connections. Ordinary concurrent readers/writers, BUSY retry and cross-process official SQLite locks are tested; recovery and crash durability are unqualified. Native runtime is currently Linux only.

LIKE uses rune matching with SQLite's default ASCII case folding. Invalid UTF-8 is preserved by raw reads/equality, but rejected by rune LIKE/length rather than silently repaired. CLI JSON rejects invalid UTF-8 text; BLOBs use base64 JSON strings. Scalar builtins include abs, length, lower, upper, typeof, hex, nullif, ifnull and coalesce, within the tested argument/value subset.

Only INTEGER PRIMARY KEY is the rowid alias. Non-INTEGER inline primary-key metadata can be read but does not imply index-backed constraint enforcement. INTEGER PRIMARY KEY DESC is rejected. Missing trailing record fields accept only supported literal defaults; unsupported schema semantics are not guessed.

No general SQLite integrity_check is implemented; inspect reports validated metadata, not whole-database integrity. No unlisted feature is considered verified.


Writable SQL limits: single rowid table; declared simple types; INTEGER PRIMARY KEY; NOT NULL; literal DEFAULT; ascending BINARY nonunique indices. Views/triggers, sqlite_* internal objects (including AUTOINCREMENT state), noninteger/compound PK, UNIQUE, CHECK, foreign keys, partial/expression/descending/collated indices, generated columns and unknown schema objects reject the entire writable catalog. Updates/deletes bound their selected-rowid worklist with MaxSortBytes. Dirty pages default to4096; mutable index keys are capped at16MiB; record reads retain their64MiB cap. Public SectorSize configures the pager's sector assumption; it is not hardware verification.

Path resolution occurs once per connection; later working-directory changes or retargeting the original symlink cannot redirect that connection. Writable Open accepts absent files (unless NoCreate); existing zero-length files are currently rejected. Context cancellation is honored before commit I/O, but cannot abandon a commit once physical publication begins. All existing journal sidecars return ErrRecoveryRequired. ErrPoisoned or ErrCommitUncertain requires stopping use and preserving evidence; no success may be inferred.
