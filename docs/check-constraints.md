# 普通 rowid 表 CHECK：有界 scalar 源码候选

## CHECK / EOF 检查点：完整正常门已通过（2026-10-12 07:22 UTC）

冻结运行树 `e06f35a022536a8ebd157731ee9a860521e70eb0` 于 **07:22:18 UTC**
自然完成 **26 门 / 28 次 Go 调用 PASS**，session 80563 exit 0，lease RELEASED。
ordinary/race 各 **8,948 named PASS / 7 个明确 SKIP**，完整 pinned oracle
**10,687 PASS / 0 named SKIP**。本副本只增加状态文档，运行源码与该冻结树一致；
本副本的新提交、bundle、精确恢复运行及对应 CI 均 **PENDING**。

CHECK 仅资格[已记录的有界 scalar 子集](check-constraints.md#精确-partial-profile)。
EOF-count 仍仅为 decode-only、EOFUnproven；native owner 在写入/Sync 前继续
finite-only fail-closed，未开放 EOF 恢复。外平台只有编译/符号检查，原生手工、
物理持久性/恢复、安全及 M3/M8/M10 未验收。后续 Darwin/Windows backend 和
真实只读媒体 harness 未纳入；其 source-only/HOLD 不被本次 PASS 覆盖。

已发布基点 `e092f0d4623cb243eec6053a4b3db7eef598542f` 的 bundle、精确恢复
26 门及 CI 38209370406 两项 SUCCESS 已另行闭合，只资格该基点。
[本检查点中文报告、精确收据与未完成项](evidence/check-eof-checkpoint-20261012.md)。

从下一节起保留各阶段原文；旧“当前”、SOURCE_ONLY、NOT_RUN、PENDING 与
FAIL/HOLD/UNKNOWN 均只指原时点和原树，不能覆盖本节的新执行记录，也不会被回写。

## 组合状态说明（2026-10-12）

本文件的源码合同已合入 CHECK / EOF-count 新组合；组合所有执行仍 NOT_RUN；V1 的旧 sqlite.go
source pin HOLD 已在本 V2 按独立完整源审作唯一 hash 窄修，guard 尚未运行。CHECK 与 EOF 的输入有限结果只绑定各自原树；下面原源码阶段
NOT_RUN 与待审阅叙述保留历史含义。格式化和单片源码审阅已另有记录，不能代替组合
默认闭包复审、运行、恢复/CI 或物理验收。见[当前组合状态](check-eof-integration-20261012.md)。

以下保留各输入与检查点原时点记录；其中“当前”、未合入、NOT_RUN、PENDING 和 PASS 均只绑定原树，不覆盖上面的本组合状态。

> 2026-10-12 CHECK V2 源码修正：前驱 `54d552d0b43dd67a4e64dd670cfbdf68e00d1671`
> 保留 **SOURCE_HOLD**。本后继修复 CHECK 空显式限定名和 descriptor 过晚准入/长类型
> 放大两项问题，仍 **SOURCE_ONLY / ALL_EXECUTION_NOT_RUN / 待独立复审**。未合并
> 父任务正在处理的自动 UNIQUE 测试 fixture 改动，不继承任何基线执行资格。

状态：**SOURCE_ONLY / NOT_RUN / 待独立审阅**。本切片没有运行 Go、gofmt、编译器、
测试、race、SQLite oracle、native 数据库程序、fuzz 或 CI。源码审阅、静态哈希及将来
的测试提案都不等于运行通过。普通 UNIQUE 基线的结果不自动传递给本树。

本次固定来源是 `automatic-unique-formatted-source-v3`，tree
`cf1e27556d9b5aec314d72db2ae02a3810992580`。其有限验证由父任务独立管理；本报告不
断言那条队列已经通过。这里没有改基线目录，也没有修改 pager、文件格式、B-tree、
journal、WAL、物理恢复、Backup、ANN 算法或 oracle 工厂。

## V2：独立审阅 blocker 的窄修正

前驱 V1 不能进入执行门。独立审阅发现：CHECK 的显式空表限定名会与无前缀混淆；
手工 CHECK-bearing descriptor 会在 count/name/SQL/cancellation/budget 验证前建立名字
map 和求值 DEFAULT，且任意长列类型可在 IN 的每次比较中触发低估的 affinity 扫描。
V2 只补上述边界：

- 仅 CHECK mode 在读到点号后拒绝空表限定名，例如 `CHECK("".a>0)`，包含将被
  false-AND/empty-IN/CASE 隐藏的 source 分支；不改 SELECT 的旧 qualification 行为。
  合法的空物理列名 `CHECK("">0)` 与物理列名 `"0"`/`"-1"` 仍正常绑定、分别执行。
- `writableTableContext` 在创建名字 map、hash 名字、运行通用 DEFAULT validator/
  evaluator 或派生 UNIQUE 之前先完成 CHECK-bearing descriptor 预准入。数量、字段
  长度、source span、CHECK/DEFAULT AST shape/cycle/depth、取消与资源预留均在前面。
  对可变长名字/原文/AST metadata 的工作与保守空间按 owner 增量计费。
- 列 Type 只允许 CREATE parser 的 canonical 拼写：空类型、INTEGER/INT/TEXT/REAL/
  BLOB/NUMERIC，以及合法范围、无前导零的 VECTOR/HALFVEC/BIT/SPARSEVEC(n)。先做
  最大 canonical 长度检查再解析；因此任意 65,536 字节普通类型不会进入 IN 比较。
  这不改变 CAST typename 文法。未引用的合法 typed-vector 列保留，引用仍拒绝。
- 通过预准入后复制 column descriptor slice，防止调用者把已接纳的短 Type 改成长
  Type。CHECK-bearing 表的 DEFAULT 求值及后续 affinity 也使用同一 scalar owner；
  DEFAULT shape 受既有 MaxTokens 总节点上界约束，不挪用 CHECK 的 8,192 节点额度。
  默认值的已有验证/affinity 顺序保持；非 CHECK 表的旧路径不变。

没有提高任何既有 SQL/token/column/schema/private CHECK quota。DML 的 NOT NULL →
CHECK → UNIQUE/row/index 写入顺序、UPDATE 依赖和 statement undo 没有改变。
新增两根纯内存修复回归及原有 source 回归扩充，没有新增物理 fixture 或 oracle 工作。


## 精确 partial profile

- 普通 rowid CREATE TABLE 的列级/表级 `[CONSTRAINT name] CHECK(expr)`；允许重复，
  按原始声明顺序保留；保留整条原始 CREATE SQL。CHECK 不新增 schema 行、autoindex
  或根页。可与已声明支持的 IPK、NOT NULL、DEFAULT、普通显式索引和自动 UNIQUE 共存。
- 只支持已存在的普通 scalar 文法：字面量、当前行列名/匹配的表名限定、IPK、按真实
  列遮蔽处理的 rowid/oid/_rowid_、TRUE/FALSE 名字、普通一元/二元算子、IS/NULL、
  CASE、CAST、BETWEEN、有界 IN-list、LIKE/ESCAPE。不改变 SELECT 的优先级或折叠。
- 首片拒绝所有显式函数调用，连 abs/length 也拒绝；这是项目的窄范围，绝不是说
  SQLite 不支持这些 CHECK。拒绝参数、子查询、跨表/未知列、private vector 算子、
  对 typed-vector 列的引用、scalar COLLATE、非 BINARY 列声明、foreign key、generated
  columns、STRICT、WITHOUT ROWID、ON CONFLICT（包括显式 ABORT）、INSERT/UPDATE OR、
  UPSERT、table/noninteger/compound PK 及其余未支持 DDL。类型拼写和库限定名没有扩展。
- 未选中的 CASE 分支、0 AND 右项、空 IN 左项也必须先通过完整源码准入。未知/不支持
  内容不能靠常量折叠隐藏。对 extension-owned shadow table 的 CHECK 不作支持承诺，
  本片明确拒绝；不调用应用 registry 或连接状态函数。

## AST、绑定、求值与失败原子性

CHECK 解析期间保留未折叠的 source AST，记录 name/exact text/span；其 source walker
先验证所有节点、深度、形状及引用，之后才复制并应用原有 parse-time false-AND、
empty-IN、literal IS NULL 折叠，再绑定 Boolean 名字。执行树与 source AST 的 composite、
leaf、BLOB 字节不共用可变内容。执行列名绑定成私有短 ordinal，避免长 SQL 名字在
每一行/IN 比较中重复 case-fold/hash；原始持久 SQL 不变，SQL affinity 不变。

`EvalCheck` 使用 `evalJump(expr, ..., true, true)`。数值零失败，NULL/非零通过；TEXT/BLOB
沿用已有 numeric-prefix 真值转换。NOT、AND/OR、BETWEEN 采用分支惰性，CASE 仍惰性；
错误、取消和预算失败都保留为错误，不能变成通过的 NULL。

INSERT 在 rowid 分配、affinity/NOT NULL 之后、UNIQUE/主表/索引/扩展变更之前验证全部
CHECK。UPDATE 的所有赋值仍取旧行值，但 CHECK 看完整新行；只选中执行树中引用了
“被赋值列”或被赋值 rowid/IPK 的声明。SET a=a 也算赋值；常量 CHECK 和不相交依赖
跳过。这里不能用原始源码引用，也不能把执行时 Boolean 短路进一步误当成依赖消失。
DELETE 和普通 SELECT 不主动验证行 CHECK。创建空表时不求值，所以 CHECK(0) 合法。

约束失败保留 ErrConstraint/primary CodeConstraint，不声称增加 extended error code
兼容。错误名取声明名字或原表达式文本。仍只有原来的 storage.Statement/statement undo
作为回滚权威：多行语句后行失败撤销本语句之前的行/索引变更，保留先前事务工作。
失败 Result 零值、changes() 清零、total_changes() 不加入失败语句；last_insert_rowid()
可保留同一失败语句中此前已成功插入行的 id。没有新建另一套提交/回滚路径。

完整 DB.Check/Tx.Check 在每个主表行的 primary scan 验证全部声明一次；反向索引 seek
里的 checkTableValues 不重复执行 CHECK。违反返回 ErrCorrupt 与零 CheckResult，且不
修改数据、不重施写入 affinity。quota/unsupported/取消/求值错误同样不能报告通过。

## 边界与资源合同

原有 1 MiB SQL、32,768 tokens、2,000 列、深度 128 及 schema 16 MiB/32,767 行上限不变。
额外限制：每表 2,000 个 CHECK、总计 8,192 个未折叠 source AST 节点。解析和 descriptor
构建双重检查；手工 AST 的 nil/非法形状/cycle/超深/超量也不能绕过。

私有 CHECK owner 覆盖一次修改语句或一次完整 Check 的目录绑定和所有行/声明：

- 累计 work 2^24；不按行或声明重置；源码/名字/绑定和跳过声明的选择也计费
- 整个 owner 的 CHECK metadata 保守计费上限 16 MiB，并受 Options.MaxSortBytes 约束
- 当前表达式 scratch 加 retained metadata 不得超过 Options.MaxSortBytes；只有 bool
  结果已丢弃才释放 scratch；metadata 到 owner 结束才释放
- numeric TEXT/BLOB coercion、concat、LIKE 扫描和中间值使用已有 scalar preflight 的
  防溢出计费；没有把 relational Query 的公开 limits 偷换成 CHECK 配置

解析器 token/source 临时对象仍受既有 SQL/token/depth 上限和新增节点上限约束；
Options.MaxSortBytes 的 CHECK owner 计费在 descriptor 准入时检查，并不等于整个 SQL
前端的 allocator 上限。

这是私有、保守的逻辑配额，错误使用 ErrMemoryLimit 加 CHECK 诊断。不是精确 allocator
账本、全事务/RSS 上界、性能资格或新公开可调 quota API。即使 page-read 配额未耗尽，
scalar work 也可能先拒绝；增加索引不会重复消耗每行的 CHECK 表达式预算。

## 新增测试源码及后续门

这里只提交测试源码，全部 **NOT_RUN**：

- internal/sql：有序声明/原文、折叠前拒绝、节点/声明/深度 exact 与 one-over、NULL/
  TEXT/BLOB/整数边界真值和 branch-lazy LIKE 错误。
- sqlite 纯内存：完整来源验证、forward/qualified/shadowed 引用、绑定不可变、最小
  依赖和常量跳过、manual AST 形状/循环、累计配额/取消、primary scan 不按索引重复。
  不合规逻辑行只存在 descriptor-free PageReader 内存模型。
- sqlite/driver 普通健康小文件：affinity/default/IPK、后行 INSERT/UPDATE 拒绝、先前 Tx
  工作、savepoint/retry、计数器、UNIQUE/普通索引共存、schema cookie、重开与 prepared
  database/sql。私有 scalar quota 后行失败复用 statement undo，属于正常资源限制。
- pinned oracle 源码：独立的 CHECK-only 诊断 helper，保持现有 UNIQUE helper 不变；
  使用原 testoracle.ForTest/confOracle/confCLI 和自然结束。对照历史、双向正常目录、
  原始 SQL、integrity_check，以及官方 abs/length 正控。外部官方正常 rename column/
  table 和 literal ADD COLUMN 只验证重写后 CREATE SQL 的导入；不新增 native ALTER。

旧拒绝测试没有删除；原本 `CHECK(a>0)` 负例改为仍拒绝的 `CHECK(abs(a)>0)`，同时新增
前者的正向源码和后者的官方正控。没有生成/运行 ignore_check_constraints 物理 fixture。
没有全套 ./...、ANN V4、41-target fuzz、人工 native 故障、未知中断链或恢复测试授权。

下一步：以封存的新 tree/hash/patch/closure 做独立源码审阅；任何修正/格式化都另封
后继身份，然后由父任务提出并审批该精确身份的有限 runtime/race/oracle 门。本文
不授予执行许可，不声称完整 SQL、完整目标或 M10 验收。

## 一手来源

- 固定公开 SQLite 3.54.0 source：src/insert.c 1685–1733 与 2057–2096（UPDATE 依赖、
  CHECK 发生顺序、NULL-pass、ABORT）；src/build.c 1968–1997/2819–2827（标签/绑定）；
  src/resolve.c 1412–1448（self-reference 参数/子查询限制）；src/pragma.c 2039–2061
  （完整 integrity checking）。先读该源树 AGENTS.md，未修改上游。
- https://www.sqlite.org/lang_createtable.html#check_constraints
- https://www.sqlite.org/lang_expr.html#booleanexpr
- https://www.sqlite.org/lang_conflict.html

以上网页地址是审计给出的一手来源索引；本片直接读取固定本地官方源代码，没有重新
浏览网页，也没有运行官方程序。
