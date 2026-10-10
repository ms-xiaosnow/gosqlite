# Compatibility matrix

Behavior baseline: official SQLite 3.54.0. M1 implements a deliberately bounded read-only subset. A baseline version never implies full SQLite compatibility. Acceptance evidence is scoped to the commands in progress.md and docs/evidence.

| Domain | Status | Tested scope / boundary |
|---|---|---|
| Native Go builds | verified | Go 1.27.2, CGO_ENABLED=0; no production SQLite subprocess or C dependency |
| SQLite header/records | partial | format3, UTF-8, rollback-mode header; page sizes 512/4096/65536 exercised; integer widths/REAL/NULL/TEXT/BLOB |
| Rowid B-trees | partial | leaf/interior, signed rowids, overflow, streaming, schema root1 including schema overflow |
| Ordinary secondary-index files | partial | may coexist with scanned tables; index traversal and indexed execution unsupported |
| Native file locking | partial | Linux OFD shared byte-range locks; non-Linux Open fails explicitly |
| SELECT | partial | one rowid table or no FROM; projection, *, WHERE, parameters, ORDER BY, LIMIT/OFFSET; aliases and rowid/IPK semantics |
| Expressions | partial | NULL3VL, comparisons/affinity, arithmetic/bitwise/concat, ASCII-insensitive rune LIKE with ESCAPE, tested scalar builtins |
| Schema interpretation | partial | declared types INTEGER/INT/REAL/TEXT/BLOB/NUMERIC or typeless; supported inline PK/NOT NULL/literal DEFAULT metadata; no write enforcement yet |
| database/sql | partial | read-only Connector, prepared SELECT, named/positional binds, cancellation, streaming rows and resource closure |
| CREATE / CRUD / transactions | unsupported | parser AST exists for future MVP; execution rejects writes/BEGIN rather than pretending they ran |
| Journal/WAL/recovery | unsupported | WAL mode and any nonempty -wal/-journal rejected, including cold persistent journal |
| grams13 / indexed LIKE | unsupported | current LIKE is a truthful table scan |
| similarity / threshold / top-K | unsupported | M5 not started |
| dense vectors / exact KNN | unsupported | M6 not started |
| HNSW / IVFFlat | unsupported | M7 not started |
| encryption / rekey | unsupported | M8 not started; no SEE file compatibility |
| Neon migration / extended vectors | unsupported | M9 not started |
| release audits / native OS evidence | unsupported | M10 not started; cross-build success is not native-runtime evidence |

Unimplemented SQL/PRAGMAs return explicit errors. JOIN, aggregate/grouping, subqueries, CTEs, window functions, DISTINCT, UNION, COLLATE, CASE, IN/BETWEEN, views/triggers, STRICT, WITHOUT ROWID, autovacuum, UTF-16 and unlisted schema grammar remain unsupported. Some storage mechanisms can safely coexist in a read-only file without their SQL being executable.

ORDER BY materializes under a configurable memory cap; unsorted results stream. Existing handles hold their shared lock until Close. Concurrent readers are tested; write progress/recovery/durability are not claimed. Native read-only runtime is currently Linux only.

LIKE uses rune matching with SQLite's default ASCII case folding. Invalid UTF-8 is preserved by raw reads/equality, but rejected by rune LIKE/length rather than silently repaired. CLI JSON rejects invalid UTF-8 text; BLOBs use base64 JSON strings. Scalar builtins include abs, length, lower, upper, typeof, hex, nullif, ifnull and coalesce, within the tested argument/value subset.

Only INTEGER PRIMARY KEY is the rowid alias. Non-INTEGER inline primary-key metadata can be read but does not imply index-backed constraint enforcement. INTEGER PRIMARY KEY DESC is rejected. Missing trailing record fields accept only supported literal defaults; unsupported schema semantics are not guessed.

No general SQLite integrity_check is implemented; inspect reports validated metadata, not whole-database integrity. No unlisted feature is considered verified.
