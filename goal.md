# /goal — Pure Go SQLite 内核重构、1–3 字倒排索引、独立加密、文本相似度及 pgvector 兼容工程

> **文档用途**：作为 Codex 在代码仓库中的最高级、可执行工程目标与验收契约。  
> **制定日期**：2026-10-10  
> **基线**：SQLite 官方 3.54.0（2026-10-09 发布）；以下载的官方源码版本及校验值固定基线。  
> **交付原则**：正确性、数据安全、无 CGO 优先于功能数量和跑分；不得以演示程序替代真实数据库内核。

## 0. Codex 执行指令（必须先读）

你是本仓库的首席数据库内核工程师，负责将 SQLite **公开、可合法使用的 C 源码所定义的行为、文件格式和设计**重构为**原生 Go 数据库引擎**，并在其上实现三个独立但事务一致的能力域：N-gram 文本检索与相似度、类 SEE 加密、类 pgvector 向量检索；再提供 Neon `pg_embedding → pgvector` 迁移适配。

**不要**把 `sqlite3.c` 翻译成一个庞大的 Go 文件，不要用 CGO、C ABI、动态链接的 SQLite、外部 SQLite 进程、远程数据库或将 SQL 委托给其他引擎冒充“纯 Go SQLite”。可以在**仅测试/对照工具链**中使用官方 SQLite CLI/C 作为 oracle，但构建产物与生产运行路径绝不依赖它们。

执行顺序：

1. 检查仓库现状、`AGENTS.md`、已有规范和测试，保留现有有效代码；若为空仓库则自行初始化。选定合适 Go module 名称，未指定组织名时不伪造组织归属。
2. 建立 `docs/compatibility.md`、`docs/architecture.md`、`docs/security.md`、`docs/decisions/`、`docs/progress.md` 及可复现基线与测试设施；记录尚未实现的功能，**禁止虚报完整 SQLite 兼容**。
3. 按第 10 节里程碑逐项编码、测试、记录证据。先做可靠持久化与事务，再做高级索引。每个里程碑应提供可以运行的纵向闭环，而不是只有接口占位或 TODO。
4. 在每个可检查节点执行 `CGO_ENABLED=0 go test ./...`、适用的竞态测试、标准测试和差分测试；不能通过则修复或者在报告中明确阻塞和重现步骤。
5. 任何时候结束本次执行，都更新 `docs/progress.md`：已完成内容、代码位置、运行命令、测试结果、当前限制、下一步可执行任务。不能因单次上下文或时间不足而宣称整个项目已经完成。
6. 禁止通过降低测试断言、静默跳过测试、丢数据/重建数据库、使用全表扫描伪称索引、吞掉加密校验失败或返回虚构结果来满足验收。

**终局定义**：提供一个真正可被 Go 应用通过 `database/sql` 使用、无需 CGO、以 SQLite 兼容磁盘格式保存明文数据库、可正确恢复事务、可扩展并可靠运行全文/模糊/向量搜索、可独立加密和迁移的库。该目标属于分阶段的数据库内核工程，不要求一次 Codex 运行内全部完成；任何尚未完成部分必须精确标注。

---

## 1. 总目标与绝对约束

### G1. SQLite 纯 Go 重构

- 参照 **SQLite 官方公开源码**与官方文档，保留 SQLite 的 SQL 语义、页结构、记录编码、B-Tree、事务语义和错误行为，内部架构按 Go 特性重新设计。
- **生产路径 100% Go**：支持 `CGO_ENABLED=0`；不依赖 C 编译器、libsqlite3、CGO 间接依赖、系统 SQLite 或外部 SQL 服务。
- 标准明文数据库以 **SQLite 3 文件格式**为目标，实现**双向互操作**：官方 SQLite 创建的受支持数据库可读取并更新；本项目创建的**已宣告受支持特性**的普通数据库可由官方 SQLite 打开、查询并通过 `PRAGMA integrity_check`。扩展数据应存放在合法 SQLite 表/B-Tree 或明确的受控扩展结构中，不得破坏普通表。
- 逐步向 SQLite SQL 能力靠拢：DDL、DML、索引、JOIN、约束、事务、CTE、UPSERT、触发器、视图、窗口函数、JSON、FTS5 等按兼容矩阵分阶段落地；未落地功能必须返回明确 `unsupported` 错误，**不得错误解释 SQL**。
- Go API 必须符合 `database/sql/driver` 约定、支持 `context.Context`、参数绑定、`Rows` 流式迭代、事务、错误码、取消、连接及资源关闭。并提供低层原生 API 用于扩展索引和加密配置。

### G2. 1～3 字倒排全文索引

- 支持 `1-gram`、`2-gram`、`3-gram`，统一称 `grams13`；适用于中文、日文、韩文和混合 Unicode 文本，不依赖英文空格分词。
- 字的默认计数单位为 **Unicode code point（Go `rune`）而非 UTF-8 字节**；NFC 规范化、大小写折叠、空白/标点策略要可配置、版本化并在检索时一致。延伸字素簇（grapheme cluster）可后续作为独立 tokenizer 加入，不能把 `rune` 实现宣称为字素簇实现。
- 不仅建立倒排表，还应支持正确的 `LIKE '%关键词%'` 候选过滤、索引命中后原文复核、模糊候选召回、排序、增量更新、删除、事务回滚和恢复。
- **区别于 pg_bigm**：pg_bigm 核心是 **2-gram**，本项目增加 1/2/3-gram 混合索引；不得声称与 pg_bigm 二进制索引或全部 SQL 运算符兼容。

### G3. 参考 SEE 能力、独立实现数据库加密

- 名称准确使用 **SQLite Encryption Extension（SEE）**，不是“SSE”。只以其公开的功能/接口文档作需求参考；**禁止下载、访问、逆向、复制或推断闭源 SEE 源码实现**。
- 实现**功能类似**的加密能力：打开加密数据库、使用密钥、错误密钥拒绝、加解密、密钥轮换、备份恢复、`ATTACH`（达到核心内核阶段时）及 WAL/回滚保护。
- **明确不追求读取/写入官方 SEE 加密文件**，也不能宣传 SEE 算法/字节级兼容。合法独立实现与否取决于实际来源和授权、专利等事实；“自己实现就一定不侵权”不是合法性保证。留存来源清单并要求发布前法律/许可审查。
- 加密文件使用本项目独立的、**带完整性认证**的格式；不得使用 RC4、固定 IV、无认证的 OFB/CTR 或自行设计密码算法充当默认安全方案。

### G4. 文本相似度检测

- 实现精确、可解释的 `ngram_similarity`（至少 Dice、Jaccard），`levenshtein`（可选最大距离提前停止）及可选 `bigm_similarity` 兼容命名。
- 支持相似度阈值查找、Top-K、与倒排索引结合的候选召回和复核，以及与向量距离/业务条件的混合搜索。
- 分数的定义、空串、重复 token、NULL、Unicode、编辑距离阈值和排序打平策略都必须由测试固定。

### G5. pgvector 查询及 Neon 迁移支持

- 提供 pgvector **核心语义兼容层**（不是宣称 PostgreSQL SQL 整体兼容）：向量类型、序列化、距离算子、精确 KNN、HNSW、IVFFlat 与匹配索引的查询优化。
- 至少支持 `vector(n)` 的 dense float32；后续阶段提供 `halfvec(n)`、`bit(n)`、`sparsevec(n)`、量化与相关距离，版本支持范围以能力矩阵为准。
- 提供以用户指定 [Neon pgvector 文档的迁移章节](https://neon.com/docs/extensions/pgvector#migrate-from-pg_embedding-to-pgvector) 为目标的**迁移工具和语义适配**：识别旧 `pg_embedding` 的 `REAL[]` 数据与 HNSW 元数据，转换为本项目的 pgvector 兼容表示，并生成用于 PostgreSQL/Neon 侧原地迁移的迁移报告和 SQL 模板。
- **迁移边界**：本项目是嵌入式 Go SQLite 引擎，不是 PostgreSQL 扩展。所称“支持 Neon 迁移”指能理解/转换相关向量数据、语义与迁移 SQL，不表示本库能在 Neon/PostgreSQL 内安装或直接替换 `pgvector` 扩展。

## 2. 版本、依赖、合法来源

| 基线 | 要求 |
|---|---|
| SQLite | 官方 **3.54.0** 公开源树作初始行为基准；记录 tag/check-in/下载地址、SHA-256 或 SHA3-256 与下载日期。未来升级单独执行兼容性评审。 |
| SQLite 授权 | 官方公开 SQLite 可交付源码处于 public domain；逐项核实实际复制文件与构建脚本的许可。 |
| SEE | **仅参考公开文档**；不得引入任何 SEE 闭源代码、付费产物、反编译材料或未授权测试数据库。 |
| pgvector / pg_bigm | 参考文档、接口与算法；若复用任何第三方代码，按具体文件许可证保留 NOTICE、作者及许可；优先实现独立 Go 代码。 |
| Go | 使用项目初始化时受支持且适合 CI 的稳定 Go 版本，将精确版本固定在 `go.mod` 和 CI。 |
| 依赖 | 允许经过许可审查的**纯 Go**依赖（如 `golang.org/x/crypto`、`golang.org/x/text` 和可选 `pgx`），不得引入 CGO。 |
| 测试 oracle | 官方 SQLite CLI 仅可在单独的对照/CI 环境使用；生产程序不链接/启动它。 |

建立 `docs/upstream.md`：记录所有官方资料、版本校验值、差异说明及所采纳的测试向量。建立 `THIRD_PARTY_NOTICES.md`，不能通过复制有许可证限制的第三方实现来规避审查。

## 3. Go 风格的目标架构

要求**模块化而非 C 文件逐行镜像**；清晰区分公有 API、内部机制、SQL 扩展和安全边界。

```text
root/
├── go.mod
├── go.sum
├── README.md
├── goal.md                         # 本目标文档在仓库内的副本
├── cmd/gosqlite/                   # CLI: query / inspect / rekey / backup / migrate / check
├── driver/                         # database/sql/driver 适配器
├── sqlite/                         # 公共 Go API 与选项、错误码
├── internal/
│   ├── sql/                        # tokenizer / parser / AST / binder / planner / expr
│   ├── vm/                         # 执行器：可选择 VM/IR，禁止全局大锁式万能解释器
│   ├── schema/                     # sqlite_schema、约束、元数据
│   ├── record/                     # varint、serial type、row 编码
│   ├── btree/                      # SQLite table/index B-Tree、overflow、freelist
│   ├── pager/                      # page cache、事务提交、锁、I/O、校验
│   ├── journal/                    # rollback-journal、WAL、checkpoint、恢复
│   ├── vfs/                        # OS 文件接口、锁、sync、纯 Go 平台实现
│   ├── codec/                      # logical page 与物理 page 编解码边界
│   ├── catalog/                    # 扩展索引元数据、版本与迁移
│   └── testkit/                    # 差分测试、故障注入、模型测试
├── extension/
│   ├── registry/                   # tokenizer、index method、UDF、planner hook
│   ├── grams13/                   # posting list、统计、位置、top-k
│   ├── similarity/                # Unicode similarity 与编辑距离
│   └── vector/                    # dense types、metrics、exact、hnsw、ivfflat
├── encryption/                     # AEAD、KDF、密钥提供器、rekey、格式
├── migration/
│   ├── sqlite/                     # 明文 SQLite 兼容导入导出
│   └── postgres/                   # pg_embedding/pgvector 转换、SQL 模板、校验
├── examples/
├── testdata/                       # 小型 fixture；大数据基准按脚本生成
├── benchmarks/
└── docs/
    ├── architecture.md
    ├── compatibility.md
    ├── file-formats.md
    ├── security.md
    ├── migration-pgembedding.md
    ├── upstream.md
    ├── progress.md
    └── decisions/
```

此目录是**目标边界而非僵化要求**：可以作出更合理的包结构，但须在 `docs/architecture.md` 解释变更。应优先定义并遵守以下内部抽象（名称可变）：

```go
// 仅表示架构期望；最终 API 以经过编译测试的真实代码为准。
type PageStore interface {
    ReadPage(ctx context.Context, pageNo uint32) ([]byte, error)
    WritePage(ctx context.Context, pageNo uint32, page []byte) error
    Sync(ctx context.Context) error
}

type IndexMethod interface {
    Name() string
    Build(ctx context.Context, source IndexSource) error
    Plan(ctx context.Context, predicate Predicate) (IndexPlan, bool)
    Scan(ctx context.Context, plan IndexPlan) (RowIDIterator, error)
    Apply(ctx context.Context, changes []RowChange) error
}
```

**关键设计原则**：

1. **文件格式优先**：原始 SQLite 页、B-Tree、schema 是兼容性锚点；外部扩展必须保证索引与普通行记录事务原子性。优先把索引作为被同一 pager 管理的影子表/受控 B-Tree；若采用 sidecar，必须有事务一致性协议，不能默认主库提交后再异步写索引。
2. **单写者、多读者**：准确复现 SQLite 支持范围内的锁语义、读快照和 WAL checkpoint；goroutine 安全不等于允许同一 `Rows` 或事务随意并行复用。
3. **面向接口组织依赖**：执行层不依赖物理磁盘实现；查询计划可选择 scan、B-Tree、grams 或 vector index；事务管理层负责编排所有二级索引的提交与回滚。
4. **不滥用 goroutine**：每查询创建无限 goroutine、依赖 GC 偶然关闭文件、长期保留整库明文均不可接受。
5. **加密在持久化边界**：SQL、B-Tree 和索引只处理逻辑明文页；`EncryptedPageStore`/安全 journal 在页读写时加解密，确保所有元数据/索引、WAL、临时文件和备份都进入安全威胁模型。
6. **支持取消和资源配额**：每次查询有 context、最大内存/索引构建预算和清晰的 OOM/磁盘满/取消错误路径。
7. **扩展可版本升级**：tokenizer 配置、索引版本、向量距离度量、加密格式版本都持久记录，不可只保存在内存中。

## 4. SQLite 兼容性具体范围

### 4.1 磁盘与事务

必须实现并证明：SQLite DB header（100 字节）、页大小/usable size、varint、serial types、table/index B-Tree、rowid、overflow 页、free-list、页分裂/合并、`sqlite_schema`、排序规则、INTEGER PRIMARY KEY、rollback-journal；随后 WAL 格式、恢复、checkpoint、数据库 busy/锁及跨进程一致性。

- Plain 模式必须保持对外 **SQLite 3 原生文件兼容**。受支持的 WAL 场景应和官方 SQLite 双向互操作；在没有明确通过 WAL 互操作测试前，限制模式和并发能力并写明。
- 禁止直接把 Go `struct` 二进制序列化当作 SQLite 页格式；所有大小端、对齐、字段取值必须遵守 SQLite 文档。
- 默认拒绝无法正确处理的数据库模式，如不支持的 WITHOUT ROWID、表格式、文本编码、extension schema 或 page size；拒绝优于暗中损坏文件。
- 文件锁、`fsync`/目录同步、热 journal 恢复、WAL 文件配对、崩溃重启后的持久性为 P0 级数据安全问题。

### 4.2 SQL 与 `database/sql`

建议按下列能力矩阵推进，具体状态由 `docs/compatibility.md` 逐项以 `unsupported / partial / verified` 标注：

- 第一批：`CREATE TABLE/INDEX`、`INSERT/UPDATE/DELETE`、`SELECT`、`WHERE`、参数、`ORDER BY`、`LIMIT`、`NULL`、常用类型/affinity、索引、`BEGIN/COMMIT/ROLLBACK`。
- 第二批：JOIN、聚合、子查询、`ON CONFLICT`、`UPSERT`、外键与约束、`SAVEPOINT`、事务 DDL、`ALTER TABLE`、view/trigger、CTE。
- 第三批：窗口函数、JSON、`VACUUM`、`ANALYZE`、FTS5、ATTACH/DETACH、backup API、更多 SQLite PRAGMA 与边缘 SQL 语义。

**受支持 SQLite 语法与兼容扩展语法分开**；不能为模仿 PostgreSQL SQL 而意外破坏 SQLite token/expr 优先级。实现 `EXPLAIN QUERY PLAN` 或等价的稳定诊断输出，能证明实际使用索引。

## 5. `grams13` 倒排索引规范

### 5.1 tokenizer 与数据结构

对于示例 `数据库`：

- 1-gram：`数`、`据`、`库`
- 2-gram：`数据`、`据库`
- 3-gram：`数据库`

每个 posting 最少包含 `(index_id, field_id, gram_length, gram, rowid)`；如支持短语/精确位置查询需记录 token 起始位置；可采用排序压缩的 rowid 列表、delta-varint 或其他纯 Go 结构，并通过同一事务保存。不要无限制预存所有长度 1～3 的原文排列或全库重复计数而无空间约束。

索引建设：全量 build / 增量 INSERT、UPDATE、DELETE / 事务回滚 / 崩溃恢复 / `REINDEX` / schema 变更 / 索引完整性检验 / 可取消重建。

Unicode：统一 UTF-8 处理、NFC、可选 Unicode casefold；记录 tokenizer 配置版本，配置改变要求重建。测试汉字、假名、韩文、emoji、组合附加符号、大小写、标点、空串、超长文档、非法 UTF-8（显式策略）。

### 5.2 查询语义

应支持：

- `LIKE '%内容%'` / `LIKE '%数据%'` / `LIKE '%库%'`，包括 **1 字、2 字、3 字** 查询。索引可先筛候选，但**必须执行原始 LIKE 语义复核**以保证无假阳性/假阴性。
- 多字符 AND/OR 条件、多个字段、`ESCAPE`、`%`、`_`、大小写与 collation；难以安全提取连续字面的模式应回退 scan，而不是作出错误索引过滤。
- 可解释的 `MATCH` 接口与相似度候选接口；允许索引加速 `LIKE` 或暴露专用 `VIRTUAL TABLE` / `USING grams13` 语法（取决于已实现的 SQL 扩展架构）。
- 估算 posting 选择性，短 gram 在高频情况下允许放弃索引并选择 scan；“可用索引”不等于所有场景必须强制索引。
- 检查事务读自己的写入、撤销后的倒排项清理、跨连接可见性和 WAL 恢复后的索引一致性。

**目标 SQL（实现与文档必须二选一确定为实际支持语法）**：

```sql
-- 类 FTS 风格，属于项目扩展语法；不是声称 SQLite 官方自带 grams13
CREATE VIRTUAL TABLE docs_grams USING grams13(
  content='docs', content_rowid='id', columns='title,body', min_gram=1, max_gram=3
);
SELECT d.id, d.title
FROM docs AS d JOIN docs_grams AS g ON g.rowid=d.id
WHERE g MATCH '数据库';

-- 优化器能证明可安全索引时，加速正常 SQLite LIKE
SELECT id FROM docs WHERE title LIKE '%数据%';
```

验收：使用经规范化的随机文本产生 ≥1 万组合用例，`LIKE` 结果集与未建索引扫描**完全相同**；1、2、3 字分别具有真实索引命中的可观测执行计划；数据操作与 checkpoint 后零过时 posting。

## 6. 文本相似度与混合检索

至少定义如下 SQL 函数（具体名称及参数在 API 文档固定）：

```sql
SELECT ngram_similarity('数据库查询','数据检索',2,'dice');
SELECT ngram_similarity('数据库查询','数据检索',3,'jaccard');
SELECT levenshtein('kitten','sitting');
SELECT bigm_similarity('全文检索','全文搜索');
```

- `Dice(A,B)=2·|A∩B|/(|A|+|B|)`；`Jaccard(A,B)=|A∩B|/|A∪B|`。明确 A/B 为**去重集合还是带重复的 multiset**，第一个交付版本建议默认**集合**并提供可选 multiset 策略；空-空=1，空-非空=0（SQL NULL 则为 NULL），测试中固定。
- `bigm_similarity` 如声明“pg_bigm 兼容”，必须核验其实际 2-gram 边界填充、分数定义、类型/阈值行为；若只复用普通 Dice/Jaccard，命名为 `bigram_similarity`，不能误用兼容标签。
- Levenshtein 采用 rune 序列、有限内存 DP、支持阈值早停，不能把 UTF-8 字节编辑距离冒充“字”编辑距离。
- 通过 gram posting 做候选召回，再执行精确相似度计算；对于声明为**精确阈值**的查询，证明召回界或回退完整扫描，严禁漏召回。近似 Top-K 必须明确带 `approximate` 语义与评估指标。
- 提供 `RRF`（Reciprocal Rank Fusion）等纯 SQL/Go 混合排序支持，将文本相似度和向量距离融合；保留可解释的命中字段、分数、距离及融合权重。

## 7. pgvector 语义与向量索引

### 7.1 类型与数学语义

第一阶段以 dense `vector(n)`（float32）为强制能力，支持向量字面量 `['1','2']` 等解析时必须严格验证实际数值，推荐标准字面量 `'[1,2,3]'`；同时接受 Go `[]float32` 或项目公开 Vector 类型的参数绑定（通过 `driver.Valuer` 与 `sql.Scanner` 或明确类型适配）。存储编码需要版本标记、维数、精度和端序，与普通 SQLite BLOB 持久存储兼容。

必须正确实现：

| pgvector 算子 | 语义 | 备注 |
|---|---|---|
| `<->` | 欧几里得 L2 距离 | 越小越近。 |
| `<#>` | **负**内积（negative inner product） | 为保持升序 KNN 排序；不可误实现为正内积。 |
| `<=>` | 余弦距离 | 通常为 `1 - cosine_similarity`；零范数按明确策略处理并一致测试。 |
| `<+>` | L1 / Manhattan 距离 | 注意旧 `pg_embedding` 的冲突算子。 |
| `<~>` | `bit` 的 Hamming 距离 | **不是** dense vector 的 Manhattan。 |
| `<%>` | `bit` 的 Jaccard 距离 | 作为后续阶段。 |

精度与边界：维数必须相同，拒绝 NaN/Inf、截断数据、不合法编码；遵循所声明的 pgvector 兼容版本限制（dense float32 索引维数上限、半精度/稀疏/bit 各有不同上限）。NULL、零向量、相同距离并列、正负内积、浮点误差容忍度都应有 golden tests。

### 7.2 精确检索和 ANN

- **先**提供精确（brute-force）KNN 作为正确性基准：`WHERE` 与 `ORDER BY distance ASC LIMIT K`，支持过滤、事务可见性与批量插入。
- **再**实现 HNSW：`m`、`ef_construction`、查询 `ef_search`，持久化邻接图和元数据，支持新增/删除/更新、重建、进程重启恢复、过滤后候选补偿、距离指标区分及运行时诊断。
- **再**实现 IVFFlat：`lists`、`probes`、聚类训练/空表行为、增量写入、过滤、重建和精确检索回退。不能只在文档写出 IVFFlat 但内部仍全量扫描。
- ANN 允许近似结果，但需要可配置精确模式，按数据集报告 `recall@K`、p50/p95 延迟、构建时间、索引大小；对同一输入的确定性测试使用固定随机种子。
- 二级索引与普通表共享提交原子性；未实现可靠持久化 ANN 前，可以明确只开放**精确 KNN**，禁止宣传“支持 HNSW”而偷偷调用暴力扫描。
- 查询计划需体现 index type、metric、候选数量、回表及精确重排路径。`LIMIT` / `ORDER BY` / 过滤条件决定是否可使用向量索引。

### 7.3 pgvector 兼容 SQL 面（限定范围）

以下是**目标兼容语句**。可通过独立 PG 兼容解析层/迁移转译器实现，但必须可从用户 API 确实执行、并在文档中说明哪些语句是 SQLite 扩展、哪些仅能转换；不可宣称全面支持 PostgreSQL 方言。

```sql
CREATE TABLE items (id INTEGER PRIMARY KEY, embedding VECTOR(3));
INSERT INTO items (id, embedding) VALUES (1, '[1,2,3]'), (2, '[2,3,4]');
SELECT id FROM items ORDER BY embedding <-> '[1,2,3]' LIMIT 10;
SELECT id FROM items ORDER BY embedding <=> '[1,2,3]' LIMIT 10;
SELECT id FROM items ORDER BY embedding <#> '[1,2,3]' LIMIT 10;
SELECT id FROM items ORDER BY embedding <+> '[1,2,3]' LIMIT 10;

-- 目标 PG 兼容语法；若执行器当前不支持，迁移器须转写成真实可执行本地语法
CREATE INDEX items_embedding_hnsw ON items USING hnsw
  (embedding vector_cosine_ops) WITH (m=16, ef_construction=64);
CREATE INDEX items_embedding_ivf ON items USING ivfflat
  (embedding vector_l2_ops) WITH (lists=100);
```

提供 `vector_dims`、`l2_distance`、`cosine_distance`、`inner_product`、`l1_distance` 等函数入口，后续实现 `halfvec`、`sparsevec`、`bit`、二值量化、`subvector`、Hamming/Jaccard 距离。统一为可测试的 `Metric` 接口；**HNSW/IVFFlat 的索引 operator class 与查询 metric 不匹配时不得误用索引**。

## 8. 自主加密模块（参考公开 SEE 文档，不兼容 SEE 文件）

### 8.1 能力与 API

至少提供原生 Go 配置 API：

```go
// 示例为期望使用方式，具体类型名称须以实际实现为准。
opts := sqlite.Options{
    Encryption: &sqlite.EncryptionOptions{
        KeyProvider: myKeyProvider, // 支持 raw key / passphrase 派生 / 后续外部 KMS
    },
}
db, err := sqlite.Open("secure.gsqlite", opts)
```

`database/sql` 驱动应可通过**代码配置**传入密钥提供器；禁止把密码放在 DSN、文件名、日志、报错、trace 或 URL。支持 `Open`、显式 `Rekey`、明文到加密的安全转换、加密到明文的显式导出、密钥轮换、带校验的备份恢复和数据库复制。切换密钥应使用临时文件+原子替换/安全事务协议，防止中途失败产生半新半旧的不可恢复数据。

### 8.2 加密格式与威胁模型

- **独立加密容器格式**（例如 `GSQLENC1`，以实际版本记录在 `docs/file-formats.md`），与官方 SQLite 明文 `.db` 格式、官方 SEE 密文格式**明确区分**。对上层暴露解密后的逻辑 SQLite 页；标准 SQLite 无密钥不能打开加密文件。
- 首选成熟 AEAD：**AES-256-GCM**（Go 标准库）或经独立审查的 **XChaCha20-Poly1305**（纯 Go 依赖）；不实现自创 cipher。不足以放 nonce/tag 的 SQLite 预留字节时，使用自定义物理帧/容器而非强行覆盖合法 SQLite 页。
- 每一次页/日志帧加密使用满足算法约束的**唯一 nonce**；按每页写入使用经过审查的随机 nonce 或崩溃安全计数方案，禁止固定 nonce、单纯 `pageNo` 当 nonce，以及恢复/重写时复用 `(key,nonce)`；测试重复写入、重启、回滚和 rekey。
- 绑定 AAD：数据库唯一 ID、物理文件类别（db/wal/journal/temp）、页号或帧号、格式版本、密钥代际及必要元数据；拒绝将一份密文页错误地替换到另一个位置。使用标准、安全、独立版本化的子密钥派生（例如 HKDF）。
- 密码输入推荐 `Argon2id` + 随机盐 + 可调参数，raw key 与 password 两种模式显式区分。KDF 参数要设有安全上下限，以防恶意文件引起资源耗尽；错误密钥与篡改返回确定的认证失败，不泄露秘密。
- 敏感暂存：加密 `WAL`、rollback journal、spill/temp、事务索引和备份；禁用明文 mmap 与不受保护的临时落盘；及时覆盖可控制的密钥缓冲区，并文档说明 Go GC/内存转储无法提供绝对内存擦除保障。
- 全文件快照回滚、文件长度/流量信息泄漏、磁盘层攻击、进程内存读取等若未额外防护，必须列为明确剩余风险。AEAD 页认证**不等于**防止整库历史回滚。
- **不要照搬 SEE 公开文档中的历史模式作为“安全默认值”**；SEE 的接口/能力可借鉴，但此处须采用现代认证加密方案，接受第三方密码学审计之后才能发布“生产级安全”表述。

### 8.3 安全验收

强制覆盖：错误密钥、错误盐、任意一字节密文损坏、tag 损坏、跨页调换、主文件/WAL 不匹配、崩溃中断 rekey、数据库恢复、并发读、密文中不出现已知明文/SQL/索引内容、日志不泄密；对未知版本**fail closed**。测试中禁止只验证“密文看起来乱码”。

## 9. Neon `pg_embedding → pgvector` 迁移

### 9.1 先确认事实，避免算子误迁移

公开的旧 `pg_embedding` 示例使用 `REAL[]` 列、`ARRAY[...]` 查询和 HNSW；现代 pgvector 使用 `vector(n)`、不同 operator classes 与参数：

| 项目 | 旧 `pg_embedding` | 新 `pgvector` / 本项目兼容层 |
|---|---|---|
| 数据类型 | `REAL[]` | `vector(n)` dense float32，显式维度校验 |
| L2 | `<->` | `<->` |
| Cosine | `<=>` | `<=>` |
| Manhattan | **`<~>`** | **`<+>`**，必须转写 |
| Hamming | 不等同旧 Manhattan | pgvector **`<~>` 为 bit Hamming** |
| 向量参数 | `ARRAY[1,2,3]`、`'{1,2,3}'` | `'[1,2,3]'`、`vector` 参数绑定 |
| HNSW 定义 | `dims`、`efconstruction`、`efsearch` 等 | 维数来自类型；`m`、`ef_construction`；查询时 `hnsw.ef_search` |
| 索引类 | `ann_cos_ops`、`ann_manhattan_ops` 等 | `vector_cosine_ops`、`vector_l1_ops`（须核实目标 pgvector 版本） |

迁移器**不得**把旧 `<~>` 机械复制进 pgvector，因为会变成另一种数学意义。对所有未识别的 operator class/索引选项必须报告不支持，禁止猜测映射。

### 9.2 输出要求

1. `migration/postgres` 有一个 **schema inspector**：识别 `embedding` extension、`REAL[]` 向量列、HNSW 索引、距离方式、参数、依赖 SQL，输出 JSON/Markdown report，标记不可安全自动转写的情形。
2. 提供从 `pg_embedding` 数据集迁入**本 Go 引擎**的途径：`CSV/NDJSON` 导入为最低保证；可选使用纯 Go `pgx` 直接连接 PostgreSQL/Neon 流式抽取（仅迁移工具需要，绝不影响核心无网络运行）。导入应可断点续传、批量事务、异常行隔离、校验行数/主键/向量维数/范数/距离结果。
3. 提供可审查的**PostgreSQL/Neon 侧迁移 SQL 模板**，包含：检查扩展和列 → 创建/验证 pgvector 依赖 → 影子 `vector(n)` 列 → 分批/可回滚回填 → 建对应 HNSW/IVFFlat 索引 → 应用 SQL/参数更换 → 业务校验 → 切换和回滚方案。模板**默认只生成，不自动执行破坏性 DDL**。
4. 支持 `pg_embedding` 的 SQL 片段 AST 级解析/转换，至少覆盖旧向量字面量、L2/Cosine/Manhattan 运算符、关键 HNSW 选项及 `REAL[] → vector(n)`。保证对普通字符串中的文本、注释和非向量位运算不误替换；不可用全局字符串替换实现 SQL 转换。
5. 添加两类金标准 fixture：旧 `pg_embedding` 示例数据和 pgvector 兼容查询数据。迁移后同一向量的精确距离误差限必须显式固定；ANN 仅比较 recall、不是逐行完全一致。

迁移指南须明确：**PostgreSQL 原地迁移和导出数据到本地 SQLite-Go 是两条不同工作流**，分别给出前置条件和验证清单。

## 10. 里程碑 / 实施顺序 / 完成定义

**强制顺序**：完成一个里程碑的核心验收再宣布进入下一个；可提前搭接口，但不能用“以后实现”替代当前数据安全要求。

| 阶段 | 交付闭环 | 最低验收 / 完成门槛 |
|---|---|---|
| **M0：基线与脚手架** | 锁定 SQLite 版本、源码校验；Go module；目录、测试框架、架构 ADR、能力矩阵、CLI 骨架 | `CGO_ENABLED=0 go test ./...`；记录正确来源；可复现构建。 |
| **M1：纯 Go 只读 SQLite** | header/varint/record/B-Tree/overflow/schema；有限 `SELECT` | 读取官方 CLI 创建的真实 fixture；不同页大小、多页、非 ASCII、BLOB、NULL 对照一致；不修改源库。 |
| **M2：写入与 SQL MVP** | 表/索引创建、CRUD、B-Tree 分裂/合并、事务、`database/sql` | 与官方 SQLite 差分测试；新建库由官方 SQLite 打开并 `integrity_check = ok`；失败事务不留下半行。 |
| **M3：持久性与并发** | journal/WAL、锁、rollback/checkpoint、恢复、并发取消与跨进程校验 | 断电/进程退出/写错位故障注入测试；无已提交数据丢失、无部分事务；race 与多连接测试通过。 |
| **M4：可扩展索引 + grams13** | index registry、1/2/3 字 posting、真实 planner、增量一致性、LIKE | 1、2、3 字命中可观测索引计划；与 scan 结果一致；事务/崩溃恢复索引无错漏。 |
| **M5：文本相似度** | Dice/Jaccard、Levenshtein、相似度阈值/Top-K、混合查询 | 函数 golden 测试；精确阈值零漏召回；中文/日文/emoji 测试；完整执行计划。 |
| **M6：向量类型与精确 KNN** | `vector(n)`、float32 codec、4 个 dense 距离、SQL/API 查询 | 与独立数学参考和 pgvector fixture 比对；非法向量拒绝；读写恢复无损。 |
| **M7：ANN 索引** | HNSW → IVFFlat、planner、参数、rebuild、删除和恢复 | 在固定数据集给出 `recall@10`；先以 ≥0.95 为 HNSW 默认参数的评估目标（不作为所有数据集保证）；相对精确扫描有可量测的有效索引路径。 |
| **M8：独立加密** | AEAD 格式、KeyProvider、KDF、加密 WAL/临时数据、rekey、备份 | 密文篡改不可通过；错误密钥失败；全量 crash/rekey 测试；纯 Go；安全设计文档。 |
| **M9：Neon 迁移与拓展类型** | `pg_embedding` 转换工具、生成 PG/Neon 迁移模板、数据导入校验；halfvec/bit/sparsevec 按兼容矩阵推进 | 旧曼哈顿 `<~>` 被转写为 `<+>`；迁移报告/复核结果；支持列表与不支持列表明确。 |
| **M10：生产化与发布审查** | 大规模基准、长期压力、跨平台、模糊测试、安全/许可审计、稳定版本 | 详见第 11 节；未过安全/一致性门槛不得宣称生产可用。 |

M0–M3 是所有后续能力的基础；如果当前尚未具备可靠写入与崩溃恢复，应优先做到 M3，而不是在一个缺乏 SQLite 事务引擎的内存 demo 上同时堆叠 grams 和向量功能。

## 11. 测试策略、量化指标和 CI

### 11.1 必须自动化的正确性测试

- **差分**：同一明文数据库和 SQL 在官方 SQLite 与本引擎中执行，比较结果值/NULL/排序/错误类别。可用 C oracle 测试，但部署无 C。
- **磁盘互操作**：官方生成 → 本引擎读写 → 官方 `PRAGMA integrity_check`；本引擎生成 → 官方查询 → 本引擎查询。按页大小、编码、索引、journal/WAL 与特性矩阵逐项验证。
- **事务模型**：以事务历史与 reference state machine 对照；包括 rollback、savepoint、死锁/忙等待、断线和恢复，索引与数据保持同事务可见性。
- **故障注入**：随机 I/O 错误、短写、`Sync` 失败、进程崩溃、checkpoint 时崩溃、磁盘满、权限变化、加密页篡改、重启恢复。
- **属性/Fuzz**：页面边界/varint/record、SQL tokenizer/parser、grams 生成和查询、向量编码、距离计算、加密解析；资源受限文件不能导致不受控内存申请或 panic。
- **并发**：多连接、多 goroutine、读写争用、取消、超时、`-race`；跨进程互操作至少在 Linux 运行完整矩阵。
- **向量**：距离精确值、NULL/NaN/Inf/零范数/维度错误、delete/update/reindex、HNSW/IVFFlat recall 与持久化。
- **加密**：使用公认算法标准测试向量、随机负面样本、错误口令、掉电/崩溃中途 rekey、备份恢复；必要时安排独立密码学审查。

### 11.2 推荐 CI 命令（以实际仓库命令为准）

```bash
CGO_ENABLED=0 go test ./...
CGO_ENABLED=0 go test -race ./...            # 在受支持的纯 Go/平台组合上运行
CGO_ENABLED=0 go vet ./...
CGO_ENABLED=0 go build ./...
go test ./internal/... -run 'TestDiff|TestCrash|TestRecovery'
go test ./extension/... -run 'TestGram|TestVector|TestSimilarity'
go test ./encryption/... -run 'TestTamper|TestRekey|TestWrongKey'
```

注意 `go test -race` 的 Go race detector 在部分平台/工具链内部可能依赖启用 CGO；若 `CGO_ENABLED=0 -race` 不支持，**改为单独的纯 Go 产物检查与可运行平台上的 `go test -race` 两道门禁**，不要因此关闭并发测试或误称生产依赖 CGO。

### 11.3 基准与发布标准

- 在 `benchmarks/` 固定硬件、Go 版本、页大小、冷/热缓存条件、数据集和随机种子，报告 1/2/3 字索引构建耗时/空间、匹配正确率、P50/P95、向量 recall@1/10/100 与 build time、事务吞吐和 WAL 恢复耗时。
- 正确性门槛为 **100%** 的可测试一致性（受支持语义范围内），不存在通过“比 SQLite 快”抵消数据损坏的情况。
- 性能目标只能相对**同机同数据集**建立，不能编造 SQLite、pg_bigm、pgvector 基准成绩；性能回归由 CI 检测并说明置信程度。
- 提供 linux/amd64、linux/arm64、darwin/arm64、windows/amd64 的**无 CGO 构建**；文件锁和崩溃恢复验证根据平台能力分级运行。
- 首个可公开使用的稳定版本须过长时间并发 soak test、数据安全评审、格式升级测试、依赖许可证审查、加密威胁模型审查及 API 文档检查。

## 12. 必须提供的用户体验与文档

### 12.1 Go 驱动使用方式

示例展示最终**期望**，在真实 API 落地之前不可伪装为已经可运行：

```go
import (
    "context"
    "database/sql"
    _ "<module>/driver"
)

func example() error {
    db, err := sql.Open("gosqlite", "file:example.db")
    if err != nil { return err }
    defer db.Close()

    if _, err = db.ExecContext(context.Background(),
        `CREATE TABLE IF NOT EXISTS docs (id INTEGER PRIMARY KEY, title TEXT NOT NULL)`); err != nil {
        return err
    }
    _, err = db.ExecContext(context.Background(),
        `INSERT INTO docs(title) VALUES (?)`, "数据库检索")
    return err
}
```

密钥只能通过独立配置/driver connector 注入（示例使用默认未加密库）。给出可编译示例测试：普通 SQLite SQL、1 字/2 字/3 字 LIKE、相似度阈值、vector exact/HNSW/IVFFlat、加密打开/rekey、`pg_embedding` 数据迁入。

### 12.2 CLI 目标命令

```text
gosqlite query <db> '<sql>'
gosqlite check <db>                    # integrity / index consistency
gosqlite inspect <db>                  # format / page / feature / index metadata
gosqlite backup <db> --out <target>
gosqlite rekey <db> --key-provider ...  # 凭据不能进入 process argv / logs
gosqlite migrate pgembedding --input <export> --output <db> --dry-run
gosqlite migrate pgembedding --pg-dsn-env <ENV> --output <db> --dry-run
gosqlite migrate pgembedding --generate-neon-sql --schema <file>
```

命令示例是目标行为；实现时确保参数设计不通过命令行明文密码/URL 泄露凭据，支持环境变量名、stdin 受控输入或系统密钥提供器等安全方式。

### 12.3 文档交付清单

- `README.md`：定位、可运行示例、无 CGO 构建、明确的非目标和兼容状态。
- `docs/architecture.md`：文字+组件/数据流图，事务、pager、扩展调用关系。
- `docs/compatibility.md`：以测试证据标记 SQL 功能/PRAGMA/文件格式/pgvector 扩展兼容状态。
- `docs/file-formats.md`：明文 SQLite 兼容约束、加密独立容器、索引持久化格式及向前迁移策略。
- `docs/security.md`：威胁模型、KDF、AEAD、nonce、AAD、journal/WAL、剩余风险与审计结论。
- `docs/migration-pgembedding.md`：Neon 文档术语映射、旧 SQL 兼容性、导出/导入与 PG 原地迁移示例、校验/回滚。
- `docs/upstream.md` 与 `THIRD_PARTY_NOTICES.md`：资料出处、固定版本、许可记录。
- `docs/progress.md`：每阶段状态、代码/测试证据、阻塞、后续计划。

## 13. 明确非目标与风险阻断

1. **不**声称自动兼容官方 SEE 已加密数据库；不获取 SEE 源码；不以“同样算法”推断格式兼容。
2. **不**把 PostgreSQL/Neon 的完整 SQL 方言、存储协议或服务器网络 API 当成 SQLite 引擎功能；pgvector/迁移支持有明确边界。
3. **不**以简单 `map[string][]RowID` 的纯内存服务、外部搜索引擎或缓存模拟持久化倒排索引；可作为算法早期单元测试但不是交付。
4. **不**在任何已宣告准确的文本相似度/LIKE 查询中因候选裁剪造成漏结果。
5. **不**把 ANN 的近似输出标记为精确；不把纯 CPU 精确扫描标记为使用 HNSW/IVFFlat。
6. **不**在 WAL、临时文件、崩溃转储、日志/备份中故意落地数据库明文，同时宣称端到端加密。
7. **不**把未验证文件格式功能的库写为可与官方 SQLite 无条件双向互操作。
8. **不**提供硬编码密钥、无认证加密、弱默认密码 KDF 或在进程参数中暴露凭据的“安全示例”。
9. **不**在不保留快照和数据迁移验证的情况下自动更改已有库格式、执行破坏性迁移或静默丢弃未知数据。
10. 代码生成工具或 AI 写出的逻辑必须接受完整测试，不能因为编译成功就标记完成。

## 14. Codex 每阶段输出格式

每完成一个阶段，请在 `docs/progress.md` 和最终交付摘要按以下模板记录：

```markdown
### Mx：阶段名称
- 状态：NOT_STARTED / IN_PROGRESS / VERIFIED / BLOCKED
- 本阶段具体交付：
- 关键架构/ADR：
- 代码路径：
- 测试命令与实际结果：
- 官方 SQLite / pgvector 对照结果：
- 已知限制或未达标项：
- 风险、安全影响及迁移影响：
- 下一步最小可执行任务：
```

只有当里程碑的可执行测试和数据安全验收均满足时才允许状态为 `VERIFIED`。所有未实现 SQL、索引与安全能力统一标注 `NOT_STARTED` / `PARTIAL`，让后续 Codex 执行能够无歧义地接续。

---

## 15. 官方参考资料（以这些资料校正细节）

1. SQLite 官方架构：<https://www.sqlite.org/arch.html>
2. SQLite 官方公开源码与版本：<https://www.sqlite.org/src/>、<https://www.sqlite.org/changes.html>、<https://www.sqlite.org/download.html>
3. SQLite 磁盘文件格式：<https://www.sqlite.org/fileformat2.html>
4. SQLite WAL：<https://www.sqlite.org/wal.html>
5. SQLite 数据损坏/恢复风险：<https://www.sqlite.org/howtocorrupt.html>
6. SQLite public domain 声明：<https://www.sqlite.org/copyright.html>
7. SQLite SEE 公开使用文档（**仅作能力参考**）：<https://sqlite.org/see/doc/trunk/www/readme.wiki>
8. SQLite SEE 文档入口与闭源源代码访问边界：<https://sqlite.org/see/doc/trunk/www/index.wiki>
9. pg_bigm 2-gram 原理与 SQL 示例：<https://github.com/pgbigm/pg_bigm/blob/master/docs/pg_bigm_en.md>
10. pgvector 官方类型、距离与 ANN 索引：<https://github.com/pgvector/pgvector/blob/master/README.md>
11. Neon pgvector 与迁移章节（用户指定）：<https://neon.com/docs/extensions/pgvector#migrate-from-pg_embedding-to-pgvector>
12. Neon 停止维护旧 pg_embedding 的公告：<https://neon.com/blog/sunset-pgembedding>
13. 旧 pg_embedding 官方项目及语法示例：<https://github.com/neondatabase/pg_embedding>

**资料核验备注**：本目标以 2026-10-10 可获得的公开资料制定。Neon 的目标文档页面可能因网站内容协商而不能通过所有自动抓取器直接读取；迁移映射首先由 Neon 公告、旧 `pg_embedding` 公开仓库与 pgvector 官方说明交叉确认。Codex 实施 M9 时，须重新核对用户指定迁移章节的现行内容、固定快照日期，并在版本存在差异时追加 ADR。不得在未经核验时声称完全复现 Neon 指南的每一个步骤。

---

### 最终验收声明

**这个项目的最终目标不是“能运行一些 SQLite SQL”，而是一个具有 SQLite 兼容存储基础、Go 原生架构、无 CGO、具备事务一致的 1–3 字倒排索引、相似度查询、pgvector 检索、独立安全加密和 Neon 迁移工具的可测试数据库内核。**

执行中始终以 **不损坏用户数据 > SQL/文件语义正确 > 加密安全 > 可维护性 > 性能 > 功能覆盖广度** 作为冲突裁决优先级。
