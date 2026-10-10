# Compatibility matrix

Behavior baseline: official SQLite 3.54.0. M2 implements a deliberately bounded read/write SQL subset; M3 adds explicit exclusive and shared-WAL paths with durability gates still blocked. A baseline version never implies full SQLite compatibility. Acceptance evidence is scoped to the commands in progress.md and docs/evidence.

| Domain | Status | Tested scope / boundary |
|---|---|---|
| Native Go builds | verified | Go 1.27.2, CGO_ENABLED=0; no production SQLite subprocess or C dependency |
| SQLite header/records | partial | format3, UTF-8, rollback-mode header; page sizes 512/4096/65536 exercised; integer widths/REAL/NULL/TEXT/BLOB |
| Rowid B-trees | partial | leaf/interior, signed rowids, overflow, streaming, schema root1 including schema overflow |
| Ordinary secondary indices | partial | create/maintain nonunique BINARY ASC keys; actual rowid/IPK equality and leading-prefix equality SELECT seeks with residual recheck; unsupported shapes scan; no general range/covering/order optimization |
| Native file locking | partial | managed role-specific file/mapping ownership, Linux OFD shared/reserved/pending/exclusive byte-range locks; native Darwin/Windows implementations still absent |
| SELECT | partial | one rowid table or no FROM; projection, *, WHERE, parameters, ORDER BY, LIMIT/OFFSET; aliases and rowid/IPK semantics |
| Expressions | partial | NULL3VL, comparisons/affinity, arithmetic/bitwise/concat, ASCII-insensitive rune LIKE with ESCAPE, tested scalar builtins |
| Schema interpretation | partial | declared types INTEGER/INT/REAL/TEXT/BLOB/NUMERIC, strict VECTOR(n)/BIT(n), or typeless; supported INTEGER PRIMARY KEY, NOT NULL and literal DEFAULT enforced on writes; non-INTEGER PK metadata read-only |
| database/sql | partial | Connector, prepared reads/writes, named/positional binds, transactions, cancellation, streaming rows and resource closure |
| CREATE / CRUD / transactions | partial | native table/index creation, multirow INSERT, UPDATE/DELETE, lazy deferred and immediate/exclusive transactions, real nested savepoints/release/rollback-to, bounded statement/named undo, read-own-writes and transactional DDL; active Rows restrictions remain |
| Journal publication | partial | standard SQLite DELETE journal and ordered Sync in successful commits; sector-size assumption explicit; error/crash paths unqualified |
| WAL binary codec | partial | native header/frame/salt/checksum validation, both byteorders, bounded committed-prefix index and strict tail refusal; memory tests |
| Exclusive WAL | partial | explicit WALExclusive; one connection-lifetime EXCLUSIVE lock and one local lease; ordinary commit/read/checkpoint/normal-close interop; requires writable access |
| Shared concurrent WAL | partial | actual Linux mmap dual-header/hash/readmark protocol; single writer/multiple readers, live SQLite interop, reader-safe checkpoint/reset; conservative imported-sector gates |
| Hot-journal recovery | partial source only | explicit internal RecoverRollback, single finite-count segment; memory parser tests and static review; physical replay NOT_RUN, no automatic Open integration |
| grams13 / indexed LIKE | conditional ordinary implementation | legal persistent posting/frequency trees, registry, transactional deltas/reindex, actual LIKE/MATCH candidate plans, diagnostics, external-write invalidation; restricted recovery unverified |
| similarity / threshold / top-K | conditional ordinary implementation | SQL Dice/Jaccard/Levenshtein, native exact posting-union or scan search, business WHERE, diagnostics and same-snapshot text/vector Go RRF |
| dense vectors / exact KNN | conditional ordinary implementation | VECTOR(n), strict versioned BLOB writes, native exact search, four distance operators/nine functions, bounded SQL Top-K, driver and ordinary SQLite interop |
| HNSW | conditional ordinary first slice | real persisted lazy graph, transactional tombstones/reindex, filtered widening/exact fallback, native explicit approximate and eligible SQL planner; measured recall with slower small-data latency |
| IVFFlat | conditional ordinary implementation | deterministic bounded training, fixed-centroid transactional membership, real selected-list reads, exact rescoring/fallback and diagnostics; no universal recall/speedup claim |
| encrypted snapshot / rekey / backup | experimental ordinary slice | authenticated root/base, Linux stable locks, full snapshot publication, correct-key rekey/backup/restore and plaintext import/export; retained history and namespace/freshness limitations |
| encrypted WAL | experimental ordinary slice | immutable authenticated final-dirty-page segments, bounded complete-head replay, checkpoint/rekey/copy and explicit snapshot upgrade; concurrent encrypted writer/readers remain required code |
| pg_embedding migration tooling | partial | bounded catalog/real-array inspector and typed source-span SQL converter with local execution fixtures and review-only PostgreSQL text; resumable import/transition templates remain required code; no live PostgreSQL execution |
| extended vector values | partial | distinct half/bit/sparse memory codecs and metrics; BIT(n) SQL/storage/driver/exact ranking plus dense subvector/quantization implemented; half/sparse SQL and all extended ANN remain required code |
| release audits / native OS evidence | unsupported | M10 not started; cross-build success is not native-runtime evidence |

Unimplemented SQL/PRAGMAs return explicit errors. JOIN, aggregate/grouping, subqueries, CTEs, window functions, DISTINCT, UNION, scalar COLLATE (CREATE INDEX COLLATE BINARY is accepted), CASE, IN/BETWEEN, general views/triggers, STRICT, WITHOUT ROWID, autovacuum, UTF-16 and unlisted schema grammar remain unsupported. Some storage mechanisms can safely coexist in a read-only file without their SQL being executable.

ORDER BY materializes under a configurable memory cap; unsorted results stream. Idle rollback-mode handles do not hold shared locks. Query Rows and explicit Tx own snapshots. WALExclusive holds EXCLUSIVE even when idle and allows only one local reader/transaction lease. Ordinary concurrent readers/writers, BUSY retry and cross-process official SQLite locks are tested; recovery and crash durability are unqualified. Native runtime is currently Linux only.

LIKE uses rune matching with SQLite's default ASCII case folding. Invalid UTF-8 is preserved by raw reads/equality, but rejected by rune LIKE/length rather than silently repaired. CLI JSON rejects invalid UTF-8 text; BLOBs use base64 JSON strings. Scalar builtins include abs, length, lower, upper, typeof, hex, nullif, ifnull and coalesce, within the tested argument/value subset.

Only INTEGER PRIMARY KEY is the rowid alias. Non-INTEGER inline primary-key metadata can be read but does not imply index-backed constraint enforcement. INTEGER PRIMARY KEY DESC is rejected. Missing trailing record fields accept only supported literal defaults; unsupported schema semantics are not guessed.

No general SQLite integrity_check is implemented; inspect reports validated metadata, not whole-database integrity. No unlisted feature is considered verified.


Writable SQL limits: single rowid table; declared simple types; INTEGER PRIMARY KEY; NOT NULL; literal DEFAULT; ascending BINARY nonunique indices. General views/triggers (except exact owned invalidation guards), sqlite_* internal objects (including AUTOINCREMENT state), noninteger/compound PK, UNIQUE, CHECK, foreign keys, partial/expression/descending/collated indices, generated columns and unknown schema objects reject the entire writable catalog. Updates/deletes bound their selected-rowid worklist with MaxSortBytes. Dirty pages default to4096; mutable index keys are capped at16MiB; record reads retain their64MiB cap. Public SectorSize configures the pager's sector assumption; it is not hardware verification.

Path resolution occurs once per connection; later working-directory changes or retargeting the original symlink cannot redirect that connection. Writable Open accepts absent files (unless NoCreate); existing zero-length files are currently rejected. Context cancellation is honored before commit I/O, but cannot abandon a commit once physical publication begins. All normal opens still return ErrRecoveryRequired for existing journals. Default rollback opens reject mode2; WALExclusive and WALShared explicitly permit their documented subsets. ErrPoisoned or ErrCommitUncertain requires stopping use and preserving evidence; no success may be inferred.


Exclusive WAL boundaries: native format3 UTF-8 rowid schema, writable file access, local Linux locking, configured sector assumption, no shared-WAL concurrency or mode switch back to DELETE. Normal close checkpoints unless KeepWALOnClose is explicit. Checkpoint requires no active lease and protects sector neighbors, including imported WAL generated under a smaller sector assumption. Main database size may lag logical WAL extent; reads use the committed overlay. Limits default to1GiB WAL,1,048,576 frames/index entries and16,777,216 charged index-work units; index accounting includes committed and pending entries. Growth can return ErrMemoryLimit and require a checkpoint. Transaction page budget includes unchanged sector-neighbor images. Valid but unsupported reset tails, cold journals, multi-segment recovery, no-sync journals and ambiguous files fail closed without deletion.


A first append to a reopened committed WAL may copy/pad and atomically replace only the WAL file under retained EXCLUSIVE before appending. This requires extra temporary space up to the WAL limit and local atomic-rename/file/directory-sync semantics. Main-file replacement never occurs. Interrupted WAL replacement is unqualified; ordinary interop and independent source review are the current evidence only.


Shared WAL boundaries: WALShared uses retained SHARED database and DMS locks with standard SQLite SHM hash blocks and five reader-lock slots (slot0 for fully backfilled database snapshots). Each connection has one active lease; distinct connections support simultaneous snapshots. Writers are nonblocking single-writer; stale deferred promotion returns ErrBusySnapshot, which also matches ErrBusy. Checkpoint is nonblocking FULL-style and may make safe partial progress before returning BUSY. Close attempts checkpoint unless KeepWALOnClose, ignores only ordinary reader BUSY, and always retains WAL/SHM inodes. Default rollback opens still refuse mode2; read-only filesystem shared WAL is unsupported.

Shared cold index reconstruction accepts fully validated committed data, checksum-valid discarded ordinary-rollback frames (never exposed), and old-generation reset suffixes. Incomplete/checksum-invalid current-generation tails are refused. Native appends pad and protect assumed sectors; imported unsafe append or sector-neighbor-unprotected checkpoint may refuse unless safe full checkpoint/reset is possible. WALPowersafeOverwrite is an explicit caller storage assertion, defaults false, and does not certify hardware. Native live-oracle tests disable SQLite PSOW. Fault/recovery interruption paths remain unverified.


M4 project extension SQL is CREATE INDEX name ON table(columns) USING grams13 WITH(...), REINDEX name, EXPLAIN QUERY PLAN SELECT, FROM table [alias] NOT INDEXED and SELECT grams13_match(index_name,value,needle). Only legal ordinary schema objects are persisted. The registry supports immutable configured inverted-key handlers, not dynamic code loading or general ANN planner hooks. Exact native DML maintenance, official writer dirty guards and schema-cookie checks protect cooperative lifecycle operations, including VACUUM. Dirty intact metadata yields explained scans; unknown versions/configuration/ownership refuse. Full details and limits: grams13.md. General source schema changes require reindex or fail explicitly; no automatic arbitrary migration is claimed.

M5 SQL ngram_similarity(a,b,n,metric), bigram_similarity(a,b), and levenshtein(a,b[,maximum]) use case-sensitive NFC, distinct unpadded rune grams, preserved NUL and strict budgets. Any NULL argument yields NULL. SQL score expressions do not receive an implicit indexed rewrite. Native SimilaritySearch explicitly names a grams13 index/field and uses that snapshot's persisted MATCH normalization. Threshold is inclusive; ties sort signed rowid ascending; zero limit returns no hits. Limits/error/cancellation discard the entire result. A maximum edit distance yields maximum+1 when exceeded, not the exact larger distance. No pg_bigm binary/operator/score compatibility is claimed. See similarity.md.

M6 prerequisite vector arithmetic deliberately uses float64 compensated accumulation over finite float32 inputs instead of pgvector's float32/FMA-dependent accumulation. Cosine with zero norm errors, rather than returning NaN or silently skipping rows. Published pgvector v0.8.7 fixtures and independent exact mathematics are references; PostgreSQL/pgvector has not been executed. The independent GSV BLOB is not PostgreSQL binary COPY/wire format. See vector-primitives.md and the fixture manifest.

M6 supports VECTOR(n) dimensions1..16,000, native GSV BLOB canonical mutation, valid external SQLite TEXT/BLOB interpretation, four dense operators and nine conversion/metric functions. A narrow one-ASC-key finite-LIMIT shape uses bounded exact heap ranking; other shapes keep the existing general sorter. SQL NULL-first order differs intentionally from native NULL-candidate exclusion. No pgvector vector equality, PostgreSQL expression grammar, binary COPY/wire or ANN compatibility is claimed. Operators share SQLite's bitwise precedence tier. See vector.md for exact APIs, quotas, plans and external-value behavior. The hybrid example uses one read transaction for text and vector lists.

M7 HNSW supports dimensions1..2000, four matching metric/opclass pairs, immutable construction config and bounded queries. SQL ANN is automatic only for supported NOT NULL vector shapes; NOT INDEXED remains exact. Native default is exact. Tombstones retain immutable vectors; rebuild compacts. Cosine zero source norms reject instead of silently excluding. No PostgreSQL index binary format or identical graph topology is claimed. See ann.md for actual APIs, persistent performance results and IVFFlat behavior.

IVFFlat supports dense dimensions up to2000, L2/cosine/negative-IP, bounded minhash training and fixed centroids until REINDEX. Too few trainable rows persist untrained state; DML does not silently retrain. Cosine zero norms reject; IP zero source values remain members and zero queries work. Probes count selected centroid ranges; all-list requests truthfully SCAN. Cumulative centroid/member/source work is retained through fallback. First matching catalog index can mask a later trained index; native Index selects explicitly. See ann.md.

Encrypted files use an independent GSQLENC1 container, not SQLite plaintext or SEE. Both experimental snapshot and dirty-segment WAL modes lock writers exclusively for connection lifetime; read-only connections may share. Native WAL options reject. Commit/rekey retain old ciphertext; key rotation is not historical revocation or secure deletion. MaxGenerations/MaxRetainedBytes/MaxDirectoryEntries bound publication inventory/work, not automatic garbage collection. Whole valid-family replacement or historic replay under reused credentials is not prevented without external identity/freshness. Linux live family operations use pinned parent-directory descriptors with relative opens/publication and identity checks; initial path resolution and hostile namespace protection remain limited. Plaintext import currently accepts only rollback mode and a single-link source. See encryption-storage.md and security.md.

Ordinary equality planning is conservative: ANN/grams plans retain precedence,
NOT INDEXED forces scans, candidate rowids are sorted before source seeks, and
error-capable residual predicates keep the prior scan path. BINARY ASC nonunique
leading equality is supported; inequality ranges, reverse scans, covering reads,
ORDER BY elimination and general collations remain unimplemented. See ordinary-index.md.

Extended values preserve dense GSVv1 and use distinct GSH/GSB/GSS encodings, not
PostgreSQL binary wire/COPY. Half conversion is explicit and may lose precision;
sparse dimensions can reach 1e9 while work scales with at most 16000 nonzeros.
Bit zero/zero Jaccard is 1 per the pinned reference. Finite float64 accumulation
and stricter malformed sparse-zero-index validation intentionally differ from
some upstream outputs. Published fixtures are source evidence, not a running
PostgreSQL comparison. BIT(n) now has typed SQL/storage/driver and exact search with distinct Hamming/Jaccard diagnostics. Half/sparse SQL and all extended ANN remain unsupported. See bit-vector.md.


Transaction control now supports BEGIN [DEFERRED|IMMEDIATE|EXCLUSIVE], COMMIT/END,
ROLLBACK, SAVEPOINT, RELEASE and ROLLBACK TO on a retained connection. Lazy BEGIN,
empty controls and constant SELECT do not pin a snapshot; first actual database
access does. Released inner names merge into the parent and outermost RELEASE
commits only a savepoint-started transaction. Undo images, freelist state and
metadata share MaxTransactionUndoBytes; exceeding it rejects before the next
retained copy. Active transaction Rows still prevent mutations/control/Commit;
full rollback invalidates them. Connection-level changes()/last_insert_rowid()
SQL functions and general cursor relaxation are absent. See transactions.md.

Managed native ownership is a Linux prerequisite for later ports. Every adopted
plain pager/SHM descriptor participates in explicit lifetime ownership; a mapping
retains its complete native view and owner. Native Darwin process-fcntl close
coordination and Windows lock/mapping/publication policies are still missing
code, regardless of successful cross-compilation. See ADR0014.

Native ownership error handling latches uncertain lock state before allowing
further operations. A failed normal Close can retain mappings and protecting
locks for retry. If shared-session construction itself fails and cleanup cannot
retire a mapping, the resources are retained fail-closed without a reachable
session retry API; process-lifetime retention is a remaining lifecycle code gap.
No native lock/unmap failure execution is claimed.


The offline converter accepts only a typed, identity-proven PostgreSQL subset.
It preserves bound legacy distance identity, comments and source spans, explicit
parameter conversion and NULL ordering, and blocks unsupported scripts rather
than mixing original statements into executable output. Local scalar arithmetic,
unverified text collation, ambiguous coercions/labels and unadapted bit binds
are refused. LIMIT binds must be non-NULL and nonnegative: PostgreSQL NULL-limit
behavior is outside this narrow conversion contract. Source-SQL array conversion
rejects nonzero decimal underflow; A's general dense-value parser retains its
separately documented rounding policy. See migration-converter.md.
