# Current compatibility matrix

2026-10-11 对账：本页描述 main 已合入能力，未发布候选的有限 PASS 不计入。
当前缺口、平台中断及中文自测步骤分别见
[验收矩阵](acceptance-status.md)、[中断记录](interruption-20261010.md) 和
[中文手册](manual-verification-zh.md)。Native Check V2 已在 776b23e 合入；
下面修正此前“没有 native full checker”的过时摘要，保留原记录于 Git 历史。

Behavior baseline: official SQLite **3.54.0**; Go **1.27.2**. This is a
conditional development checkpoint, not production or complete milestone
acceptance. `partial` means implemented and tested only within the stated
profile; `unsupported` means required code is still missing or deliberately
rejected for now. A successful ordinary test does not qualify physical recovery,
security or native-platform behavior. No production SQL delegates to SQLite,
a subprocess, CGO/C ABI, PostgreSQL or a remote engine.

This current summary supersedes contradictory older scope statements preserved
in [the historical compatibility log](history/compatibility-before-sql-ranges.md).
Exact source, command, CI and restored-commit evidence remains in
[progress](progress.md), [checkpoint records](checkpoints.json) and `evidence/`.

## SQLite files, storage and lifecycle

| Capability | Status | Implemented profile and open boundary |
|---|---|---|
| Native Go production build | verified scoped | CGO_ENABLED=0 dependency/build checks; separate enabled race detector does not introduce a production C dependency |
| SQLite format-3 rowid files | partial | Explicit header/varint/record/table/index/overflow/freelist encodings, schema root1, signed rowids, integer/REAL/NULL/TEXT/BLOB; ordinary CLI interoperability at 512/4096/65536-byte pages |
| Unsupported file/schema forms | unsupported | UTF-16, WITHOUT ROWID, autovacuum and unlisted schema/format semantics fail closed; support is never inferred from the version number |
| Table and index mutation | partial | Native page split/merge, table/index creation and maintained nonunique BINARY ASC indices; test-oracle integrity_check and the separately bounded native DB/Tx/CLI Check V2 provide distinct checks, neither implies universal SQLite feature support |
| Rollback journal | partial | Standard DELETE-journal normal commit/rollback and sector assumptions; existing journals cause ErrRecoveryRequired; existing zero-byte databases are rejected |
| Hot rollback recovery | partial source only | Explicit internal aligned finite-count multi-segment replay with bounded memory parsing and minimal cleanup ownership; automatic Open integration and unsupported modes/tails remain code work. Native RecoverRollback is source-reviewed only; physical replay/crash qualification is NOT_RUN |
| Exclusive WAL | partial | Explicit writable WALExclusive, lifetime EXCLUSIVE lock, one active lease, ordinary append/read/checkpoint/close/interoperability; not shared concurrency |
| Shared WAL | partial | Actual dual-header/hash/readmark SHM protocol, single writer/multiple independent snapshots, ordinary live official-reader/writer interoperability and reader-safe checkpoint/reset; one active lease per connection |
| Ambiguous WAL tails | partial | Accepted ordinary committed/discarded/reset cases are bounded; incomplete/current-generation ambiguous tails fail closed; broader recovery handling remains code work |
| Locking/native platforms | partial | Linux managed OFD locks; Darwin process-fcntl/deferred-close registry; Windows/amd64 LockFileEx, full file identity and managed mapping/metadata handles. Linux/amd64 ordinary runtime only; other target builds are not runtime evidence |
| Managed cleanup ownership | partial | Bounded shared failed-open/successful-close, encrypted and plaintext Reader/Tx/exclusive minimal-owner registries/retry APIs are implemented, including pre-acquisition public metadata adoption. Native recovery now shares bounded ownership and explicit initial-absence provenance; shared-session initial-absence provenance and rollback Reader post-SHARED metadata timing are corrected with independent source/ordinary/race evidence. Native failure execution remains unqualified |
| Busy/cancel/uncertain results | partial | Normal BUSY, stale shared promotion, cancellation and snapshot lifetime are exercised. Once physical publication begins it is not interrupted; ErrPoisoned/ErrCommitUncertain requires stopping use and preserving evidence |
| Native full-profile integrity checking | partial | DB.Check/Tx.Check/CLI Check V2 validate one pinned snapshot, whole-page ownership, supported tables/ordinary indices and built-in extension semantics under explicit budgets. This is not unrestricted SQLite PRAGMA integrity_check or physical recovery qualification; see native-check.md |
| Plaintext backup/native VACUUM | unsupported | Plaintext backup API/CLI and native VACUUM are not implemented on main. Unpublished Linux rollback backup candidates do not change this status; encrypted correct-key backup has a separate supported profile |

Plaintext paths are resolved once per connection. Idle rollback connections do
not hold a shared snapshot. Query Rows and explicit Tx retain their snapshot;
active transaction Rows still prevent mutation/control/Commit. Shared WAL retains
sidecar inodes and requires writable access. Conservative storage-sector
protection defaults on; WALPowersafeOverwrite is an explicit caller assertion,
not hardware detection. Darwin fsync is not F_FULLFSYNC; Windows has no claimed
Unix directory barrier. Default Windows cross-vet's disclosed mapping
unsafe-pointer diagnostic remains unqualified. See [native platform
ADR0015](decisions/0015-native-darwin-windows-backends.md) and
[manual verification](manual-verification.md) for exact existing commands.

## SQL and planner

| Capability | Status | Implemented profile and open boundary |
|---|---|---|
| Basic DDL/DML | partial | Simple rowid CREATE TABLE, nonunique BINARY ASC CREATE INDEX, multirow INSERT, UPDATE/DELETE, transactional DDL, REINDEX for supported extension indices |
| Constraints/schema | partial | INTEGER PRIMARY KEY rowid alias, NOT NULL and supported literal DEFAULT; noninteger PK metadata can be read but does not imply enforcement. UNIQUE/CHECK/FK/compound PK/generated columns/STRICT/general schema alterations remain unsupported |
| Transaction control | partial | Native/driver and retained-connection BEGIN, COMMIT/END, ROLLBACK, SAVEPOINT, RELEASE, ROLLBACK TO; bounded statement/savepoint undo and read-own-writes. Connection changes()/total_changes()/last_insert_rowid() now have bounded native/driver state. General cursor relaxation remains code work |
| SELECT and JOIN | partial | No FROM/single table and bounded named-table comma/INNER/CROSS/LEFT nested loops, aliases/qualified columns/stars, WHERE/ON, binds, ordering/NULL placement/LIMIT/OFFSET; one root snapshot/Tx lease. USING/NATURAL/RIGHT/FULL/derived/subquery relations remain unsupported |
| Boolean and affinity semantics | partial | Context-sensitive scalar versus predicate/CASE branching, three-valued logic, bounded constant startup/substitution and declared-versus-absent comparison affinity; unquoted TRUE/FALSE identifier precedence and scoped SQLite direct LEFT-ON result-alias provenance are implemented. Known remaining planner-dependent differences include CAST/IPK equality suppressing an earlier error in SQLite and LEFT-only WHERE pushdown ahead of an error-producing ON; these are code gaps, not full oracle parity |
| Other scalar expressions | partial | CASE/CAST/BETWEEN/IN-list, arithmetic/bitwise/concat, default BINARY comparisons, supported scalar functions and rune LIKE/ESCAPE; scalar COLLATE/general collation registration and unlisted functions remain unsupported |
| Global aggregates | partial | Bounded native COUNT(*)/COUNT(expr)/SUM/TOTAL/AVG/MIN/MAX, supported scalar result wrappers and existing joins; exact storage-class/NULL/arithmetic/error comparisons against pinned SQLite. GROUP BY/HAVING are implemented for bounded declared key/expression profiles, including proved rowid grouping/ordering and top-K projection admission. DISTINCT/FILTER/windows and ambiguous bare-column projections remain unsupported |
| Rowid equality/ranges/order | partial | Genuine equality seeks and signed bounded forward/reverse inequality cursors, conservative residual recheck and proven rowid ORDER BY elimination. Error-capable projections/residuals retain the original path; NOT INDEXED forces scan/sort |
| Secondary equality | partial | Genuine leading equality-prefix B-tree cursor, bounded rowid candidates and source seeks with residual recheck; ANN/grams precedence preserved |
| Secondary leading inequalities | partial | Genuine leading BINARY ASC index bounds, bounded signed-rowid candidates and base seeks with complete residual recheck; existing ordering remains materialized and diagnostics count actual attempted work |
| Composite ranges/covering/join plans/statistics | unsupported | Composite equality-plus-range, general secondary reverse/covering/order plans, join index pushdown and ANALYZE statistics remain code work |
| Diagnostics | partial | EXPLAIN QUERY PLAN and immutable Plan/Stats report actual supported access. Aggregate plans distinguish global and grouped execution, including documented rowid grouping. JOIN plans report the actual nested-loop path. Range counters do not invent work or include unmeasured speedups |
| Remaining SQL | unsupported | Subqueries, CTEs, DISTINCT/compound UNION, windows, JSON, ON CONFLICT/UPSERT, general ALTER/view/trigger, FTS5, ATTACH/DETACH and unlisted PRAGMAs remain required implementation work |
| database/sql | partial | Connector, prepared SQL/binds, normal transactions, streaming rows, cancellation and connection disposal; statement preparation is not a claim of reusable compiled AST execution |

ORDER BY generally materializes under a memory cap except proven rowid or
supported bounded rank shapes. LIKE uses SQLite-default ASCII case folding over
runes; invalid UTF-8 is preserved by raw reads/equality but rejected by rune
LIKE/length, and CLI JSON text rejects it. Typed extensions are separate from
ordinary SQLite storage classes and do not change SQLite token precedence.

See [relational SQL](relational-sql.md), [Boolean expressions](boolean-expressions.md),
[comparison affinity](affinity-comparisons.md), [global aggregates](aggregate-sql.md),
[ordinary index planning](ordinary-index.md), and [rowid ranges](sql-rowid-ranges.md).
All unsupported writable catalog semantics reject mutation rather than silently
ignoring constraints. Full SQLite compatibility is not claimed.

## Text, vectors, encryption and migration

| Capability | Status | Implemented profile and open boundary |
|---|---|---|
| grams13 | partial | Persistent legal posting/frequency B-trees, incremental transactional maintenance, real 1/2/3-rune LIKE/MATCH candidate ranges, configured normalization, residual checks, dirty external-writer fallback and REINDEX; physical recovery consistency unqualified |
| Text similarity | partial | Native/SQL Dice, Jaccard, Levenshtein; exact posting-union or scan threshold/Top-K, business filters, deterministic ties and same-snapshot Go fusion; no pg_bigm binary/score compatibility |
| Dense vector/exact KNN | partial | VECTOR(n), GSVv1 BLOB codec, finite float32 storage/float64 accumulation, four distance operators, exact bounded SQL/native ranking and driver support; explicit mathematical differences from pgvector remain documented |
| Extended representations | partial | Distinct GSH/GSB/GSS half/bit/sparse codecs and SQL/storage/driver/exact ranking; explicit conversion/loss policies and support-bounded sparse work, not PostgreSQL binary COPY/wire formats |
| ANN | partial | Genuine persistent HNSW and IVFFlat selected graph/list reads, snapshot rescoring, transactional maintenance/rebuild, budgets and truthful fallback. Typed HNSW half/bit/sparse and IVF half/bit; sparse IVF, IVF L1 and bit Jaccard remain unsupported. Dense v1 cross-version bytes/configuration preserved |
| ANN performance | partial | Fixed-fixture recall and logical-work evidence; no universal recall or latency guarantee. Small-data latency may be worse than exact scan; no scan is called an ANN hit |
| Independent encryption | partial experimental | Independent GSQLENC1 authenticated snapshot/dirty-WAL/shared layouts, KeyProvider/KDF, correct-key ordinary rekey/copy/backup/restore and plaintext import/export; not SEE format compatibility |
| Encrypted platforms | partial | Linux, Darwin and Windows/amd64 native backends; only Linux ordinary runtime. Windows shared-mode rekey is unsupported code; Darwin ASCII basename/directory inventory limits still require Unicode work |
| Encryption security/recovery | unsupported qualification | Correct-key ordinary tests and descriptor-free parser/crypto/ownership laws do not prove physical tamper/wrong-key/crash/interrupted-rekey safety. Retained history, namespace/freshness and secure-deletion limits remain explicit |
| pg_embedding migration | partial | Bounded identity-aware catalog inspector, typed offline SQL conversion, resumable mapped CSV/NDJSON import and deterministic verification reports; old Manhattan mapping is explicit |
| PG/Neon transition | partial | Review-only retained-array, shadow-column and bounded PG16 full-table copy/swap text with mutations commented; unsupported FK/views/functions/triggers/RLS/partitions/custom-expression/collation/ACL dependencies, online/batched cutover and live execution remain open |
| M10 release | unsupported qualification | Independent security/cryptographic/legal/API review, native-OS execution, long-duration soak and large fixed-hardware benchmark gates are not complete; no stable/production acceptance claim |

Encryption retains old ciphertext generations; key rotation does not revoke
historical keys or securely delete prior contents. Bounds cover represented
engine state rather than arbitrary application allocations or error objects.
Whole-family rollback/replacement cannot be excluded without external identity
and freshness. See [security](security.md), [file formats](file-formats.md),
[ANN](ann.md), [typed ANN](typed-ann.md), and
[migration](migration-pgembedding.md).

The unchanged [goal](../goal.md) remains the contract. The user permitted
independent later development while restricted tests remain documented for
manual verification; that sequencing change does not waive required code or
acceptance gates. No restricted test is relocated to CI or another environment.

Required code, automatically runnable evidence, restricted manual gates and real
external environments are separated in [acceptance status](acceptance-status.md).
The dated clause-by-clause source audit remains a historical evidence snapshot.

Bounded rollback cached-schema preparation distinguishes invalid ordinary SQL
from valid execution-BUSY without exposing cached metadata as data authorization.
The strict official warm-schema BUSY control now passes. Session/extension/
constant-query warming and general stale-schema reprepare remain separate gaps;
this is not reusable compiled-AST or complete SQLite prepare parity. See
[connection counters](sql-connection-counters.md),
[cached preparation](sql-cached-preparation.md) and
[grouped aggregates](grouped-aggregates.md).

Encrypted backup/rekey CLI uses explicit redacting env-name or bounded stdin
providers. Caller-dataset distance verification reports derived outward numerical
bounds and exact-search/Top-K coverage, while ANN remains separately not_run.
NFC grams membership and build/space/latency tools are implemented; measured
space amplification and selective slowdowns are preserved. These additions do
not qualify native failure recovery, long soak or statistical performance.
