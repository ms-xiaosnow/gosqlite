# 普通 rowid 表自动 UNIQUE 约束：范围与证据

## 当前五切片组合状态（2026-10-12）

当前冻结运行树为 `59ce26879c4301babc7648fbddc5cc66c4097263`（3,267 文件、783 个 Go 文件），包含 LEFT/Rows、encrypted inspect、Darwin inventory manual、Connector rollback manual 和 automatic UNIQUE（含 c382 空结果表头 helper 修复）；**不包含 CHECK**。该树于 **2026-10-12 05:21:06 UTC 自然完成完整 26 门 / 28 次 Go 调用 PASS**，lease 已 RELEASED。ordinary/race 各 **8,569 named PASS / 7 个固定预期 SKIP**，完整 pinned oracle **10,301 PASS / 0 SKIP**，LIMIT oracle/race **394 PASS**。新检查点提交、Git bundle、精确恢复及对应 CI 均 **PENDING**；已验证的未发布源包不代替这些步骤。

已发布 main 仍为 `8bc189474bdb2b0a683c6c059992f9acbc426250`，其 bundle、恢复 26 门和两项 CI 已闭合，只资格该精确基点。当前范围、具名输入收据和真实代码/原生余缺以[当前状态](current-status-20261012.md)及[组合证据](evidence/combined-checkpoint-20261012.md)为准。下文带旧日期的 SOURCE_ONLY、NOT_RUN、PENDING、FAIL/HOLD/UNKNOWN 均保留原时点含义；历史失败不回写，手工、物理及 M3/M8/M10 门不因此开放。

### automatic UNIQUE 独立有限终态

c382 tree `c3824a47a9fc03a9c1808bf62e84bf924b0e771e` 的 E2/session 90085 于 2026-10-12 04:40:44 UTC 自然完成原第 11 门：3 根、6 子项、9 named PASS，零 SKIP/FAIL。仅运行固定 4096 页有限 oracle；289 个官方进程结果是源码推导的完整 PASS 路径计数，未独立逐子进程留存收据。它不替代五切片组合的完整 oracle、普通/race 或发布资格。

原 cf1e read-only keeper fixture FAIL、7dd6 空结果表头 FAIL、E1/session 76084 初始化 FAIL（零 Go）、AU-1 与组合 source-pin HOLD 均保留。[完整来源与限制](evidence/combined-checkpoint-20261012.md)。

### 历史源码准备记录（V1/V2；下列 NOT_RUN 属于当时）

> 2026-10-12 V2 精确 schema 预算修正：前驱 `c065143baa0e61d0c4aae942ae2b5051b26b28f2`
> 因 AU-1（owner/table/已有目录和行数预算遗漏）保留 **SOURCE_HOLD**。本 V2 仅做
> 共享 ReadSchema 真实计费、完整当前目录与新增行的写入前准入，以及纯模型/小型 quota
> 原子失败测试源码；仍 **SOURCE_ONLY / NOT_RUN**，待独立复审，不能继承前驱或基线结果。

状态：2026-10-12，IN_PROGRESS / SOURCE_ONLY / NOT_RUN。本切片从冻结
Storage + 显式 UNIQUE 树 `4008bd9442ffd21e1e993d4f1c72dbeed3c910f3`
复制，独立保留原始 3,238 文件基线。父任务已经记录该基线完整 26 门 PASS；
这里没有重跑或继承这些结果。本切片尚未 gofmt、编译、测试、race 或官方 oracle，
也未提交、推送、恢复验证或获得新 CI。源码阅读和 Python 字节/哈希/差异清单不是
运行正确性的证明。LEFT JOIN、Darwin 手工门与 encrypted inspect 独立候选未合入。

## 原目标与有界语法

对应 goal.md 的 G1、§4.2 与 §11：既要执行约束，也要在受支持文件中保留索引，
允许官方与本引擎互相打开并继续写入。不能只增加 parser 或本地 CREATE 而漏掉
重开目录绑定。本切片是这一闭环的源码实现，不代表完整 SQLite 兼容或 M10 验收。

接纳现有普通 rowid 表上的两种声明：

```sql
CREATE TABLE t(
  id INTEGER PRIMARY KEY UNIQUE,
  a TEXT COLLATE BINARY CONSTRAINT uq_a UNIQUE,
  b NUMERIC,
  UNIQUE(a),
  CONSTRAINT uq_ab UNIQUE(a COLLATE BINARY ASC,b ASC)
);
```

- 列级 `[CONSTRAINT name] UNIQUE` 与逗号分隔的末尾表级
  `[CONSTRAINT name] UNIQUE(column [COLLATE BINARY] [ASC],...)`。
  表级约束之后不再接受列。约束标签不是索引名；引号标识符遵从现有名称规则。
- 列可显式 `COLLATE BINARY`。所有列和索引键仍只支持 BINARY；不接纳列级
  `UNIQUE ASC`。不为其他种类的约束扩大 `CONSTRAINT name` 语法。
- 键必须是已声明的普通标量列，或者现有 INTEGER PRIMARY KEY 别名列。
  同表其他位置可以有 VECTOR/HALFVEC/BIT/SPARSEVEC；这些类型不能作为 UNIQUE 键。
  隐藏 rowid/oid/_rowid_ 不可当作键名，除非同名列确实已声明。
- 使用现有 SQL 1 MiB、token 32,768、列/每键 2,000 上限；新限制为最多
  2,000 个保留声明、合计 8,192 个键项。重复声明在预算中也计数，绝不静默截断。
  推导出的索引描述符按名称、键项和固定开销计费，独立受 16 MiB 瞬时描述符上限约束；V2 另按下述 ReadSchema 真实合同检查整个持久目录。

显式 `ON CONFLICT` 的所有策略（包括 ABORT）、INSERT/UPDATE OR、UPSERT、DESC、
非 BINARY、表达式/partial/NULLS FIRST 或 LAST 键、UNIQUE extension 方法、
表级 PRIMARY KEY、非 INTEGER 或复合 PK、IPK DESC/AUTOINCREMENT、WITHOUT ROWID、
STRICT、CHECK/FK/generated、CREATE TABLE AS 仍拒绝。原有可以读取的部分非整数 PK
元数据不意味着写入或约束执行被允许。未实现的 DROP TABLE/INDEX、ALTER TABLE、
普通 REINDEX 不变；不会通过新名字前缀放开约束所属索引的独立删除。

## 名称、所有权和 NULL 元数据

AST 按实际声明顺序保留每个 UNIQUE。完整列定义确定后解析列序号，再按有序键列
（本范围的有效 collation 恒为 BINARY）合并。交换列顺序不是等价；键内重复列保留。
编号只在新索引生成后递增：重复约束不消耗后缀。

上述示例产生 `sqlite_autoindex_t_1` = id、`_2` = a、`_3` = (a,b)。
普通 INTEGER PRIMARY KEY 本身没有 autoindex，也不消耗后缀；单独的 UNIQUE(id)
必须有真实索引，即使看似冗余。显式 CREATE UNIQUE INDEX 不与约束合并。

每个索引有单独且不共享的 >=2 根页，记录完整 `(键字段..., signed rowid)`。
表 schema 行存原 SQL；自动索引行是 type=index、精确派生名、精确拥有表名、真实根、
真正 SQL NULL。没有虚构 CREATE INDEX SQL、rootpage=0 或内存 map 替代持久树。

目录装载不依赖 sqlite_schema 行顺序。先解析表并导出完整期望集合，再逐一绑定
名称/类型/owner/root/NULL；缺失、额外、孤儿、重复名、共享根或伪造 SQL 不能授权写入。
内部保留 `constraintOwned`，与 `unique` 区分。保留公共新名字的 sqlite_ 禁止规则；
只让已由表声明证明并精确匹配的隐式对象通过，而非允许任意 sqlite_autoindex 前缀。

`btree.SchemaEntry.SQLNull` 保留真实存储类别；不能从 `SQL == ""` 猜测 NULL。
公有 `sqlite.SchemaEntry.SQL` 继续为 string；新增 `SQLNull bool`，JSON 标签为
`SQLNull,omitempty`。DB.Schema、DB.Inspect 与 plaintext CLI inspect 均传递该字段：
普通有 SQL 文本的行仍只有原来五个字段、原名和原顺序，SQLNull=false 不输出；
规范 autoindex 行新增 `"SQLNull":true` 且仍有 `"SQL":""`。
这是 autoindex/其他 NULL schema 行的显式加法字段，不把旧 SQL 字段改成 pointer 或 null。
Go 调用者宜使用具名字段构造结构体；新增字段不承诺兼容外部未具名 struct literal。
SQL 查询 sqlite_schema 不通过此展示结构体，直接得到 NULL，typeof(sql) 为 null。

官方 3.54.0 loader 同时容许 NULL 与空 TEXT 的隐式 SQL。这里有意只发出并装载
规范 NULL：精确拥有的空 TEXT 自动索引返回 ErrUnsupported（非规范元数据），
不称其为 SQLite 损坏。只读 Schema/Inspect 仍显示 `SQL=""`、SQLNull=false，
普通表扫描不以索引完整性为前提；写入及全库 Check 采用严格范围。

## V2：AU-1 精确持久目录预算修正

前驱 c065 的单表 descriptor 预算漏掉每条 implicit 行中重复的完整 owner 名、
表行 SQL/两个表名、已有目录占用，以及 32,767 条上限。独立审阅提供的纯算例：
600,000 字节表名配 20 个唯一列，SQL 600,236 字节、descriptor 12,005,671 字节，
实际 schema 计费却为 25,806,108 字节；三个 300,000 字节表名配10列的独立语句，
每个6,903,188、前两者13,806,376、第三者后20,709,564。旧树能准入这些模型，
无法保证提交后的下次 schema reload。这是源码 blocker，旧树与报告原样 SOURCE_HOLD。
没有建立这些大名字的真实数据库，没有运行重现。

V2 保留既有 16,777,216 字节/32,767 行读取上限；不放宽预算来掩盖问题。
`btree.SchemaUsage.Add` 与 ReadSchema 共用完全相同的逐行合同：
`256 + len(type) + len(name) + len(tbl_name) + len(sql)`；真实 NULL 只令最后一项为0，
owner 仍全额计费。逐项以剩余额度相减，避免整数和溢出；失败不推进 usage。
ReadSchema 原来的 count-before-field-validation、字段/type/root/NULL 检查次序不变。

CREATE TABLE 完成语义去重后，在任何 storage 根分配前，从当前 Tx.reader 重新
ReadSchema，给全部原始行和拟新增的表行/每个 implicit 行计费。包括 owned extension
trigger，即使 writeCatalog.names 会跳过该行。声明数预算仍在去重前，持久行数预算
则只数实际发出的去重后 index。IF NOT EXISTS 仍在准入之前返回，不读/预留/追加额度。
explicit CREATE INDEX 也在根分配前执行相同全量准入。

公共内部 `addSchema(string)` wrapper 保持签名和 cookie 语义，但每次实际 append 前
重新读取当前事务 schema，检查旧目录+这一行；因此既有显式索引/extension shadow/
guard 行也不能提交超过读限制的目录。旧扩展流程可能在调用它之前分配根，这种
预算错误仍返回原 storage.Statement 进行完整撤销；本修正不重写或运行 ANN 流程。
底层 nullable addSchemaRecord 只供已一次准入完整 delta 的 compound CREATE，以及
已 fresh 准入的 wrapper 调用。不存在新的无预算调用者。

没有跨 statement/catalog/savepoint 缓存 quota；多个 CREATE、失败重试、ROLLBACK TO
和完整 rollback 都直接看到当前 reader 的真实行，不需要维护可失真的计数器。
这是额外的有界 schema 读取成本，不是 query acceleration 或整个事务 heap 上限。

新增 `internal/btree/schema_budget_test.go` 与
`sqlite/automatic_unique_budget_internal_test.go` 的纯内存源码覆盖精确字节/条数及
one-over、无效/极大计数器、owner/SQL 计费、已有占用、owned guards、去重后行数、
no-op 和准入前错误。长名算例只作为字符串/descriptor 模型，绝不创建 native file。
`sqlite/automatic_unique_budget_test.go` 只用4096页、4脏页预算的小型健康 rollback
文件：先前 INSERT 保留，table/首个 autoindex 已分配并写过 schema 后，第二个 index
触及普通页预算，检查 statement 撤销、cookie/roots/counters、savepoint 和重试。
它不是 I/O 故障注入；仍仅有测试源码，尚未执行。

## CREATE、DML 与检查的纵向闭环

先验证全部声明、派生名和已有目录加完整新增行的精确预算，再在已有同一个 storage.Statement 中分配空表根和每个空
索引根、插入全部 schema 行，最后只递增一次 schema cookie。只有全部成功才更新
statement-local catalog。原 `addSchema(string)` 作为 wrapper 保留显式索引与扩展调用
的 cookie 行为。IF NOT EXISTS 遇到已存在普通表是真 no-op，不追加索引或变更 cookie。

INSERT/UPDATE/DELETE、NOT NULL、storage affinity、唯一前缀探测、signed rowid
自排除、NULL 豁免、statement ABORT、先前事务工作、savepoint/full rollback、
changes/total_changes/last_insert_rowid 全部复用已有实现。NULL 只豁免冲突检查，
完整索引项仍必须存在。复合非 NULL 前缀按存储类/BINARY 和精确 INTEGER/REAL
比较；不把整数先转 float64，也不延迟多行交换冲突。

完整 index 描述符加入已有 writeTable.indexes，因此所有 implicit 根参加原有
checkWriteOwnership、addIndexes/removeIndexes、Check 的双向键一致性和相邻唯一
前缀检查。CheckScope 仍是 gosqlite-supported-profile-v1，绝不宣称 integrity_check
全范围等价。准备缓存仍是 names-only，cookie/rollback 失效流程未改；自动索引没有
被加入普通 equality/range 规划，不新增查询加速承诺。扩展 owned shadow/guard 的精确
检查没有改动；grams 事务参与不意味着 ANN 算法或 V4 工作获准。

## 上游来源与兼容决策

只读研究固定的 SQLite 3.54.0 源码，source id/check-in：
`be8d059e9a49089ab2dce5ed26dd87aaf598fdc5fbe0b107c0dd758464bcd5e3`。
下载、公开来源和许可记录保留在 [upstream](upstream.md)。没有修改或运行官方 C 源码，
没有复制 SEE 或其他闭源材料。采用的行为依据：

- src/parse.y 的 ccons/tcons UNIQUE 与 constraint label 语法。
- src/build.c sqlite3CreateIndex：自动名、有序列/collation 合并、真实根和 NULL SQL；
  sqlite3AddPrimaryKey：正常 IPK 无索引，但独立 UNIQUE 仍创建；EndTable 单次 cookie。
- src/prepare.c sqlite3InitCallback：隐式目录绑定、root、孤儿检查，以及空 TEXT 的可读性。
- src/insert.c 与 vdbe.c OP_NoConflict：affinity、逻辑 IPK、NULL-prefix 与更新自排除。
- test/index.test index-16/index-17、test/unique.test unique-3/unique-4 的命名与复合键。
- src/build.c sqlite3DropIndex、alter.c 的约束索引生命周期：本切片继续拒绝 native DDL。

来源审计 REPORT SHA-256 为
`3b531416e578245e5d64b6fca0afd00e757df45aa29ab0530b47f3ddcd0eceeb`；
其 43 个引用文件 SHA-256 已全部读回核对。来源证明不等于新引擎行为证明。

## 历史测试源码与有限验证提案（作者阶段全部 NOT_RUN）

V2 在以下29个前驱测试根声明外新增10个预算/原子错误根声明。这是源码计数，
不是实际执行数；精确 selector 和动态子项结果必须由后续获准的运行记录提供。

- internal/sql/automatic_unique_parse_test.go：有序 lowering、命名/引号、重复/重复项、
  支持列表、拒绝列表、声明数/单键/总键项的有限边界。unknown-column 属语义装载阶段。
- internal/btree/schema_test.go：纯内存有效记录的 NULL 与空 TEXT 区分，以及非 text/NULL
  SQL 字段拒绝。没有把异常记录写入文件。
- sqlite/automatic_unique_internal_test.go：纯内存推导、预算、向量排除、行顺序无关目录、
  缺失/多余/owner/type/root/nullable metadata 异常、完整 Check 和 planner 边界。
- sqlite/automatic_unique_test.go、driver/automatic_unique_test.go：健康 CRUD、affinity、
  数值/NULL/signed IPK、默认值、重复声明、显式索引共存、原子 DDL/事务/savepoint、
  counters、Schema/Inspect/JSON、driver prepare、关闭重开和有界页大小/分裂/overflow。
- cmd/gosqlite/automatic_unique_test.go：真实 NULL SQL、旧普通 inspect 行 JSON 不增加字段、
  autoindex 加法字段、重新打开的约束执行、Check 与只读文件哈希不变。
- internal/testkit/automatic_unique_conformance_test.go：复用现有 uniqueOracleHistory、
  testoracle.ForTest 固定隔离工厂；同历史错误/计数器，以及双方向重开后继续写入。
  官方 forced autoindex 读取、origin=u/index_xinfo、integrity_check 与健康官方 rename/
  rename column/add ordinary literal-default column 均只作为待跑源码。

只改旧拒绝列表中现在有正向来源覆盖的六个条目：internal/sql/sql_test.go、
internal/sql/unique_index_parse_test.go 与 sqlite/write_test.go；替换成仍然明确拒绝的
ON CONFLICT、DESC 或非 BINARY 变体，不删弱原先其他断言，不改写任何历史结果。

下一步提案不是执行授权：

1. 对完整新树、diff、生产必要 hunk 和测试/helper/启动闭包进行独立只读审阅。
2. 审阅后单独批准格式检查；若 gofmt 或任何修正改变字节，生成后继身份再复审。
3. 固定工具链身份、Go 1.27.2、CGO0、vendor/offline 依赖、私有 /tmp 与 oracle hash；
   有限 vet/build/test-compile 范围仅覆盖本切片六个包，生产依赖不得引入 C/oracle。
4. 只选新 parser/schema-memory/catalog-memory/native/driver/CLI 根，外加被修改的旧
   拒绝根与既有显式 UNIQUE 相关回归；分开普通与受支持 CGO1 race 门，逐条保存原始
   terminal、exit、实际子项/skip。不得用编译/list 当作 test-body PASS。
5. 仅执行新 oracle 三类有界历史及必要显式 UNIQUE 回归；沿用原固定隔离工厂，不另起
   launcher，不继承 HOME/XDG/PATH/loader 或 SQLITE_DEBUG_BREAK。记录确切 selector、
   argv、cwd、stdio、环境白名单、自然完成及恢复后的文件身份。不得比较独立文件根页
   编号或 schema rowid 顺序来代替逻辑不变量。
6. 遇到失败保存原始记录；只做被批准的修正/后继验证。有限结果不能自动扩展成完整
   ./...、fuzz、native manual、Backup/WAL publisher 或 crash/tamper/wrong-key 资格。

这里没有预写命令数、PASS/子项数、跨平台或物理资格。准确的完整文件 manifest、
Git blob/tree 的 Python 序列化身份、所有 changed paths/diff、静态依赖闭包与未来
selector 清单由外部 source-evidence 封存；不是运行命令或生产依赖。
