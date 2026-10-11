# Current compatibility matrix

## 2026-10-11 RO-WAL / IPK 组合：当前执行证据与终态边界

本次实际运行冻结树为 `cace45621c10259231a2e764cfbefdcdbe312ccf`，
3,178 文件；基于原组合 `9ca02ee7fee55f0eb4abe83330b37b80d02aa4bd`，
只修三个既有测试文件并新增一个 oracle 测试文件，生产源码零差异。
本段是该精确树的新执行记录；后面的作者阶段 SOURCE_ONLY / NOT_RUN、
输入切片与其他检查点结果分别保留自己的时间和源码身份。

- 四个修复普通根及 43 子项已实际取得 47 named PASS，零 SKIP/FAIL。
- 新原 SQL 对照根及八子项已实际取得 9 named PASS；一次身份查询、
  六次结果查询、两次 EXPLAIN 共九个官方进程正常完成。
- 新鲜 live-writer 只读 WAL 单根及六子项已实际取得 7 named PASS；
  三种页大小 × 两种 KeepWALOnClose，六个健康 peer 加一次身份查询。
- **完整 26 项于 22:05:15 UTC 自然完成 PASS**，28 次实际 Go 调用，
  lease RELEASED；所有阶段 exit 0，无强制终止或重试。
- ordinary/race 各 5,758 PASS / 7 个固定 oracle SKIP；完整 pinned oracle
  7,431 PASS / 零 SKIP，LIMIT oracle/race 394 PASS。
- guard、vet、CGO0 build、五目标生产编译、模块核验、两个无 CGO 依赖闭包、
  Darwin 两架构和 Windows 的六条 test-compile/nm 门均 PASS。
  外平台测试镜像仅编译/读符号，未执行；全部结果只绑定本次冻结树。

原 9ca02ee7 在新环境的普通门真实结果仍为 5,711 PASS / 7 SKIP /
四根 FAIL，exit 1；后续 19 项未运行。四项修正有独立源码审阅和上述
重新执行证据，未删弱断言、未改生产代码或用旧 PASS 替代新门。
新提交、精确恢复与 hosted CI 在取得各自实际身份前仍 **NOT_RUN**。

[本次证据、失败分层与归档](evidence/readonly-ipk-combined-20261011.md)
列出准确阶段、原 SQL、工具身份和剩余验收。
[环境替换原始记录](history/ENVIRONMENT-REPLACEMENT-20261011.md)
保留旧会话 54102 **UNKNOWN**，不推定其退出或成功。
本有限开发资格不等于完整 SQLite 兼容、M3/M10、生产、安全或持久性验收；
ANN V4、41 目标持续 fuzz、未知中断链、全部物理/原生手工限制保持。
以下历史段落中的“当前”“最新”“待运行”均只指其自身写作时点。

## 历史组合作者阶段记录（运行前）：只读 WAL、IPK equality 与 live-writer 测试源码

本组合以 main `1e8e7fbb694b9234b7797414ae7422a38e08ab7e`、tree
`4e82f2859975a1d8df11b692f732ed29f6a83e6d` 为基点，纳入只读 WAL V2、
IPK equality V4 和一个 `oracle && linux` 的健康官方 live-writer 测试文件。
本次只组合已审字节并修正文档；新组合的编译、普通/race/完整 oracle、
精确恢复及 CI 均 **NOT_RUN**。输入切片的下列结果不转移为本组合 PASS。

- 只读 WAL V2 tree `b19ee07536c83d554f70b40d461b0b96ee4e2872` 的有限
  14 门组合已闭合：23 根 + 27 子项 = 50 named PASS，零 SKIP/FAIL。
  其中普通 Linux 选择为 8 根 + 6 子项；compile/list 只列根，不算测试体 PASS。
  保留 V3/V4 原始中断与未资格结果，不把它们改写为一次完整成功运行。
- IPK V4 tree `96e039f2a0b0990789579bfa5fe20746b6b1ecdb` 的原始 baseline
  为预期 RED：22 PASS、29 FAIL（含两根），exit 1；candidate 普通六根及
  49 子项共 55 PASS、exit 0。仅修正记录器 Candidates 计数期待后重新核对
  已闭合原流；原 STOP_HOLD 和错误记录保留。随后仅执行原第三条官方 oracle：
  24 个 SQL 比较加一次身份查询，25 named PASS、零 SKIP/FAIL，20:24:46 UTC
  自然结束并释放；没有重跑前两条。这只属于 IPK V4 输入，不是新组合资格。
- 新 live-writer 文件 SHA-256 `b36e8739e69e67f77ed539d2a9e768de92ac9f5675a5ee3d5a557b112b455006`
  已独立源码审阅，实际运行 **NOT_RUN**：一根、六个串行子项，正常选择运行
  时为六个健康 peer 加一次身份查询，共七个官方进程，均要求正常结束。
  它不是缺失的源码，但尚无互操作 PASS；保留成功/失败 fixture，不进行物理故障。

Linux 活动且已初始化 SHM 的[只读契约](plain-readonly-wal.md)与
[有限 IPK equality 边界](ordinary-index.md)保持分开。IPK 只放宽有界直接
等值/LIKE 条件，保留 residual 顺序和 startup 行为；CAST、一般函数、
二级/范围索引与 LEFT JOIN pushdown 不因此取得更广资格。
Darwin/Windows 原生执行、真实只读媒体/非特权身份、物理故障/恢复、
M3/M10 和安全验收仍 **NOT_RUN / 未验收**。以下有日期的旧记录保留各自
当时状态；“harness 缺失”“尚未运行”等旧文不覆盖本段当前源码说明。

## 历史源码准备记录（运行前）：2026-10-11 只读 WAL V2

本次源码组合以已发布 main `1e8e7fbb694b9234b7797414ae7422a38e08ab7e`、tree
`4e82f2859975a1d8df11b692f732ed29f6a83e6d` 为新基点；保留全部 Darwin 代码与
证据。Darwin/Windows 原生执行仍 **NOT_RUN**，新的精确恢复/CI 终态另记，
不能用下文较早普通资格或 bundle 字节身份代替。

只读 WAL 的 30 个非重叠变更路径与已封存 V1 逐字节/模式相同；只对本页及
另外两份中央状态文档作 hunk 组合，没有新增函数、harness、测试根或 API。
V1 的 `3ea6d3…` 基点与专题手册中的 SOURCE_ONLY 记录保留为首次实现来源；
本 V2 不是该旧树，也不是已发布 main 的只读 WAL 能力。23 根/27 子项保持，
其中 8 根普通 Linux 生命周期只有测试源码；此功能编译、模型、native、race、
oracle 仍 **NOT_RUN**。官方 live-writer harness 仍缺，真实只读媒体/非特权
身份另待外部资格。冷 WAL、无/未初始化 SHM、其他 OS 与 M3/M10 边界保持。
本次没有运行 Go、gofmt、parser/compiler、Git、项目脚本、测试或原生操作；
后续仅在固定新树独立复审与队列释放后，按不扩大的 14 条提案另行安排。


2026-10-11 当前未发布组合：Linux `ReadOnly + WALShared` 的 live-index 子集及
`--wal-readonly` query/check/inspect 已纳入；本树实际普通与 live-writer 结果见页首，
输入切片与运行前记录单列，完整 26 项已于 22:05:15 UTC 自然 PASS。
冷 WAL、缺/未初始化 SHM、其他 OS 仍 unsupported；详见
[只读契约](plain-readonly-wal.md)。此项不继承下面 writable WAL 的既有资格。

## 2026-10-11 17:02 UTC 开发资格补记

本次 Darwin 有界清单/诊断源码组合 `8a324a8cedeccc8ba46234924c8074890aefa1da`
的 21 项限定普通验证已全部闭合。ordinary/race 各 5,614 PASS、7 个固定预期
SKIP；完整 SQLite 3.54.0 oracle 为 7,246 PASS、零 SKIP；LIMIT oracle/race
394 PASS。Darwin 两架构及 Windows 测试二进制仅编译并核对符号，未运行。
完整身份、原始记录器失败和修正、支持域及缺口见
[Darwin 清单专题的本次补记](darwin-unicode-inventory.md)。

本文形成时的已发布父基点为 `3ea6d3ab15f678fe48bee312921da60828845869`，
其精确 Library 恢复 17 门和 CI [38149634238](https://github.com/ms-xiaosnow/gosqlite/actions/runs/38149634238)
已通过；只属于该父基点。本批仅同步文档，所有 813 个非 docs 且非 README
文件与上述已验证组合逐字节/模式一致。新提交、精确恢复和 CI 要分别读取
其实际终态，不预写自指 SHA 或未来 PASS。历史段落的“未发布”、旧 main、
NOT_RUN、PASS/FAIL 保留各自写作时点，不能混作当前资格。

原生 Darwin/Windows、实际持久性/故障/恢复、安全审查和 M10 均未验收。
本次没有合入另行开发的原生 Unicode 手工入口、strict plaintext backup、
WAL-tail publisher、只读明文 WAL 或 IPK 修正；相关未实现/未合入项仍分别跟踪。

本批仍为 PARTIAL：仅可证无关的 UTF-8 sibling、实际枚举 owner 的限定 local APFS/HFS 门，以及批内读取诊断保留。ASCII 根/177 字节上限不变；更广文件名/文件系统和原生资格不从本次普通 PASS 推出。

2026-10-11 对账：除下面明确标注的 Windows 九路径未发布源码增量外，本页保留 main 的能力分层；候选有限 PASS 不计为 main 或原生资格。当前身份见 [Windows shared-rekey 专题](windows-shared-rekey.md)。
当前缺口、平台中断及中文自测步骤分别见
[验收矩阵](acceptance-status.md)、[中断记录](interruption-20261010.md) 和
[中文手册](manual-verification-zh.md)。Native Check V2 已在 776b23e 合入；
下面修正此前“没有 native full checker”的过时摘要，保留原记录于 Git 历史。
2026-10-11 P0 更新：公开 Open 已接入默认关闭的有限 Linux 明文 rollback
恢复源码；独立源码/内存模型/编译审阅 PASS，后续当前组合默认关闭的完整普通/oracle/race 门亦 PASS。真实恢复和开启选项的
真实 Open 仍未运行；新提交恢复及 CI 需各自精确身份，
不能沿用旧 LIMIT 或候选结果，详见[公开恢复边界](rollback-recovery-open.md)。

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
| Rollback journal | partial | Standard DELETE-journal normal commit/rollback and sector assumptions; nil-default Open still refuses existing journals with ErrRecoveryRequired / CodeIOErr. Explicit RollbackRecovery opts into the bounded source-only profile below; existing zero-byte databases remain rejected |
| Hot rollback recovery | partial experimental source/model | Open/OpenContext and configured Connector now have a nil-default, Linux writable-plaintext rollback opt-in, with retired-reader path proof and at most one recovery plus one fresh no-create probe. Independent descriptor-free model/source/compile checks PASS; real RecoverRollback, the production bridge and recovery-enabled real Open remain BLOCKED / NOT_RUN. The four-case public Open manual harness is implemented with independent source/compiler-only PASS; all physical cases remain NOT_RUN. Driver Connector physical coverage, default-on recovery, broader journals, native WAL-tail publication, other native platforms and full fault harnesses remain separate gaps |
| Exclusive WAL | partial | Explicit writable WALExclusive, lifetime EXCLUSIVE lock, one active lease, ordinary append/read/checkpoint/close/interoperability; not shared concurrency |
| Shared WAL | partial | Actual dual-header/hash/readmark SHM protocol, single writer/multiple independent snapshots, ordinary live official-reader/writer interoperability and reader-safe checkpoint/reset; one active lease per connection |
| ReadOnly + Shared WAL | partial / scoped Linux ordinary + live-writer PASS | 显式加入活动已初始化 SHM；三个 O_RDONLY 句柄、只读 mmap、旧 mark 保护完整 header 快照、无 checkpoint。新树健康官方 live-writer 一根六子项实际 PASS；真实只读媒体/非特权身份未验证。冷/缺失/未初始化 sidecar 和其他 OS 不支持；不是通用 readonly WAL |
| Ambiguous WAL tails | partial | Accepted ordinary committed/discarded/reset cases are bounded; incomplete/current-generation ambiguous tails fail closed; broader recovery handling remains code work |
| Locking/native platforms | partial | Linux managed OFD locks; Darwin process-fcntl/deferred-close registry; Windows/amd64 LockFileEx, full file identity and managed mapping/metadata handles. Linux/amd64 ordinary runtime only; other target builds are not runtime evidence |
| Managed cleanup ownership | partial | Bounded shared failed-open/successful-close, encrypted and plaintext Reader/Tx/exclusive minimal-owner registries/retry APIs are implemented, including pre-acquisition public metadata adoption. Native recovery now shares bounded ownership and explicit initial-absence provenance; shared-session initial-absence provenance and rollback Reader post-SHARED metadata timing are corrected with independent source/ordinary/race evidence. Native failure execution remains unqualified |
| Busy/cancel/uncertain results | partial | Normal BUSY, stale shared promotion, cancellation and snapshot lifetime are exercised. Once physical publication begins it is not interrupted; ErrPoisoned/ErrCommitUncertain requires stopping use and preserving evidence |
| Native full-profile integrity checking | partial | DB.Check/Tx.Check/CLI Check V2 validate one pinned snapshot, whole-page ownership, supported tables/ordinary indices and built-in extension semantics under explicit budgets. This is not unrestricted SQLite PRAGMA integrity_check or physical recovery qualification; see native-check.md |
| Plaintext backup/native VACUUM | unsupported | Plaintext backup API/CLI and native VACUUM are not implemented on main. Unpublished Linux rollback backup candidates do not change this status; encrypted correct-key backup has a separate supported profile |

The journal-presence diagnostic text and private wrapping changed, including
with nil policy. Default refusal, `errors.Is(err, ErrRecoveryRequired)` and
`CodeIOErr` are preserved; byte-identical errors or error-chain shape are not
promised. See [opt-in rollback recovery](rollback-recovery-open.md) for API,
limits, supported journals and evidence boundaries.

Plaintext paths are resolved once per connection. Idle rollback connections do
not hold a shared snapshot. Query Rows and explicit Tx retain their snapshot;
active transaction Rows still prevent mutation/control/Commit. Shared WAL retains
sidecar inodes. Its normal writable route requires writable access; the explicit
ReadOnly live-index subset has exact combined-tree ordinary/live-writer evidence above;
real denied-write media and other native platforms remain unqualified. Conservative storage-sector
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
| Boolean and affinity semantics | partial | Context-sensitive scalar versus predicate/CASE branching, three-valued logic, bounded constant startup/substitution and declared-versus-absent comparison affinity; unquoted TRUE/FALSE identifier precedence and scoped SQLite direct LEFT-ON result-alias provenance are implemented. The current unpublished composition adds a bounded direct IPK equality/LIKE admission correction, including under NOT INDEXED; the exact combined tree now has normal and fixed original-SQL oracle evidence, separately recorded above. CAST/general-expression planning and LEFT-only WHERE pushdown ahead of an error-producing ON remain gaps, not full oracle parity |
| Other scalar expressions | partial | CASE/CAST/BETWEEN/IN-list, arithmetic/bitwise/concat, default BINARY comparisons, supported scalar functions and rune LIKE/ESCAPE; scalar COLLATE/general collation registration and unlisted functions remain unsupported |
| Global aggregates | partial | Bounded native COUNT(*)/COUNT(expr)/SUM/TOTAL/AVG/MIN/MAX, supported scalar result wrappers and existing joins; exact storage-class/NULL/arithmetic/error comparisons against pinned SQLite. GROUP BY/HAVING are implemented for bounded declared key/expression profiles, including proved rowid grouping/ordering and top-K projection admission. DISTINCT/FILTER/windows and ambiguous bare-column projections remain unsupported |
| Rowid equality/ranges/order | partial | Genuine equality seeks and signed bounded forward/reverse inequality cursors, conservative residual recheck and proven rowid ORDER BY elimination. The bounded direct IPK equality/LIKE envelope can seek under NOT INDEXED, with original residual/startup semantics; broader error-capable shapes and NOT INDEXED range/secondary paths retain their previous scan/sort behavior |
| Secondary equality | partial | Genuine leading equality-prefix B-tree cursor, bounded rowid candidates and source seeks with residual recheck; ANN/grams precedence preserved |
| Secondary leading inequalities | partial | Genuine leading BINARY ASC index bounds, bounded signed-rowid candidates and base seeks with complete residual recheck; existing ordering remains materialized and diagnostics count actual attempted work |
| Composite ranges/covering/join plans/statistics | unsupported | Composite equality-plus-range, general secondary reverse/covering/order plans, join index pushdown and ANALYZE statistics remain code work |
| Diagnostics | partial | Immutable Plan/Stats report actual supported access. EXPLAIN QUERY PLAN describes planning; the unpublished conservative bound-hint correction can differ from actual SELECT. Its exact contract, the two preserved failures and the completed exact local qualification of the test-only correction are documented in [SQL ranking](vector.md#sql-bounded-ranking-and-truthful-plans) and [current evidence](evidence/startup-wal-limit-20261011.md). Aggregate plans distinguish global and grouped execution, including documented rowid grouping. JOIN plans report the actual nested-loop path. Range counters do not invent work or include unmeasured speedups |
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
| Encrypted platforms | partial | Linux, Darwin and Windows/amd64 native backends; only Linux ordinary runtime. Windows shared-mode rekey has guarded admission/conversion and post-conversion same-source-key authority checks in this unpublished composition; native Windows is NOT_RUN. Darwin 当前未发布候选保持可打印 ASCII 根名上限 177 字节及 78 字节后缀预留；有效 UTF-8 同级名称仅在根名内、Unicode/控制字节之前已有折叠 ASCII 差异，且实际枚举 owner 的 Fstatfs 确认本地 APFS 或 hfs subtype 0–3 后可略过。Unicode 父路径原已允许；一般 Unicode 根名与真实目录资格 harness 仍缺，固定源码的 Linux/amd64 精确 20 项模型/AST normal/race 已 PASS；额外编译门及原生验证仍 NOT_RUN，见[受限清单证明](darwin-unicode-inventory.md) |
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

## 2026-10-11 OS namespace 测试选择边界

2026-10-11 namespace 后续更新：本批以已保存 P0 提交
`56bbcf3febe233197aa0a5eac965c67a67a88011`（tree
`c31e7dcbeed9732688681741ee38ec60ff0bfb49`）为组合基点，只移出两个
Windows/Darwin raw namespace 测试并追加通用 Rename/rename selector guard。
恢复 guard 和 LIMIT 成果保持。六路径候选的 source/model/race、188 项选择矩阵
及六项外平台编译/符号核对已有独立 PASS；原生/manual 二进制全部 NOT_RUN。
详见[本批手册](manual-verification-zh.md#19-os-raw-namespace-门控与独立原生验收2026-10-11)及[范围审计](test-scope-audit.md)。
本批新组合、提交、恢复和 CI 必须各自记录，不能继承 P0 或候选 PASS。

| 范围 | 当前状态 | 准确边界 |
|---|---|---|
| 两个 encstore 原生 namespace 回归的默认选择 | verified scoped source/model/compile | default 与仅 oracle 在适用平台排除这两个完整移走的函数；manual_namespace 加 env gate 保留它们，其他原声明保持。测试覆盖变化被明确记录 |
| Rename/rename source-reference guard | verified scoped | 追加通用 selector 检查，当前 P0 recovery guard 全部保持；bare helper/wrapper/reflection 等仍需行为审阅，不是完整安全证明 |
| Windows/amd64、Darwin/amd64、Darwin/arm64 原生 namespace 行为 | unqualified / NOT_RUN | 两种 manual tag 的六项外平台编译与符号核对仅证明编译/链接；不证明文件锁、持久性、加密安全或原生故障资格 |


## 2026-10-11 公开恢复手工测试的有限补齐

[TestManualPublicRollbackRecovery](../sqlite/rollback_recovery_manual_linux_test.go)
只覆盖 nil-default/input-preservation、opt-in/只读重开、journal quota refusal、
NoCreate absence 四个拟议物理场景。独立 V3 的源码审阅、AST、选择 0/0/1、
编译及唯一顶层符号 PASS；没有执行 test binary 或生成 fixture，全部物理
**NOT_RUN**。没有生产行为改动，不新增兼容格式、平台或自动恢复策略。
[中文步骤](public-rollback-manual-zh.md)只追加当前发布状态，原命令/断言保持；
原 V3 手册字节另存证据。当前资格、准确候选与 namespace 组合身份见
[验收补记](acceptance-status.md#2026-10-11-有限公开恢复-harness-源码与编译资格)。
