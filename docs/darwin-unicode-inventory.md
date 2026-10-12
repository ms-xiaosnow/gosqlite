# Darwin Unicode 同级名称清单的受限证明

## 当前五切片组合状态（2026-10-12）

当前冻结运行树为 `59ce26879c4301babc7648fbddc5cc66c4097263`（3,267 文件、783 个 Go 文件），包含 LEFT/Rows、encrypted inspect、Darwin inventory manual、Connector rollback manual 和 automatic UNIQUE（含 c382 空结果表头 helper 修复）；**不包含 CHECK**。该树于 **2026-10-12 05:21:06 UTC 自然完成完整 26 门 / 28 次 Go 调用 PASS**，lease 已 RELEASED。ordinary/race 各 **8,569 named PASS / 7 个固定预期 SKIP**，完整 pinned oracle **10,301 PASS / 0 SKIP**，LIMIT oracle/race **394 PASS**。新检查点提交、Git bundle、精确恢复及对应 CI 均 **PENDING**；已验证的未发布源包不代替这些步骤。

已发布 main 仍为 `8bc189474bdb2b0a683c6c059992f9acbc426250`，其 bundle、恢复 26 门和两项 CI 已闭合，只资格该精确基点。当前范围、具名输入收据和真实代码/原生余缺以[当前状态](current-status-20261012.md)及[组合证据](evidence/combined-checkpoint-20261012.md)为准。下文带旧日期的 SOURCE_ONLY、NOT_RUN、PENDING、FAIL/HOLD/UNKNOWN 均保留原时点含义；历史失败不回写，手工、物理及 M3/M8/M10 门不因此开放。

## 历史记录：2026-10-11 17:02 UTC 完整 21 门限定资格

当前固定资格组合为 `8a324a8cedeccc8ba46234924c8074890aefa1da`，
3,157 文件 / 148 目录；其中十路径源码来自
`88c400a4caeb40d8d96fae7700e7048d69db4ff5`。本次后续文档同步保持全部
813 个非 docs 且非 README 文件的字节与模式不变，文档树有独立身份。
已发布父基点是 `3ea6d3ab15f678fe48bee312921da60828845869`；其精确恢复17门及
CI [38149634238](https://github.com/ms-xiaosnow/gosqlite/actions/runs/38149634238)
SUCCESS 已闭合，不能移为当前组合的新提交/恢复/CI 证据。

### 新的实际执行

- 完整 ordinary 与 race 各 5,614 路径 PASS、7 个固定预期 oracle SKIP；
  1,253 Test、41 Fuzz 根的 313 固定种子、6 Examples。固定种子不是 fuzz campaign。
- 完整固定 SQLite 3.54.0 oracle 为 7,246 PASS / 零 SKIP；1,358 Test、41 Fuzz、
  6 Examples、313 固定种子；两根 LIMIT oracle/race 精确 394 路径 PASS。
- guard、vet、CGO0 build、五生产目标编译、module verify 均 PASS。
- 当前实际全模块非测试闭包为 206 包 / 1,317 输入；shipping CLI/library 为
  164 包 / 1,228 输入。shipping 不可达 testoracle、testscope、os/exec 或 C；
  C-family/Swig/Syso 字段均为空。这里是实际解析闭包，不沿用旧树计数。
- Darwin/amd64 与 arm64 各精确 99 Test + 1 Fuzz 符号；Windows/amd64 为
  100 Test + 1 Fuzz。原 required/absent 全集合成立。三个目标仅编译/nm，
  没有运行 Mach-O/PE 或其 initializer。

固定工具为 Go 1.27.2（生产 CGO0；Linux race 使用其原有支持组合）及 SQLite
3.54.0/source ID `be8d059e9a49089ab2dce5ed26dd87aaf598fdc5fbe0b107c0dd758464bcd5e3`。
使用原 env-i、离线固定模块、原 cache/default /tmp 与资源门；没有新增
TMPDIR/TMP/TEMP/GOTMPDIR、下载、清理或降低阈值。SQLite 子进程继续使用已审
启动工厂的 -noinit/显式环境/固定 CWD 合同，不承诺隔离所有 OS 行为。
本环境 workspace overlay 的 fsync=volatile、/tmp tmpfs 不能证明掉电持久性。

### 记录器失败、纠正及保全

原1–16门 PASS。17号 nm 自然 exit0，但原验证器错误拒绝含空格的真实泛型
符号，保守 STOP；325 个原始记录保持。修正仅分离官方地址/类型与完整符号尾部，
纯解析已有7,851行（125个含空格名字），保持所有 exact/absent 条件。10项漏、
重复、多余、禁止及畸形负向均拒；独立核对真实输出和官方固定 nm 打印源码。
没有重新运行17号 nm。一个续跑草稿遗漏 git_controls helper，在启动前发现并
保留；修正版恢复原 helper 并独立静态复核，再只运行原18–21门。

总计21个外层命令 / 23个显式 Go 调用，零重跑；17:02:29 UTC 自然释放唯一
Go 租约，无仍活跃子进程。完整源码、工具、模块、历史、模式和资源前后检查通过；
atime 仅观察，不声称全部 POSIX 元数据一致。原始 verifier FAIL 与最终已核准
开发范围并存，不把前者改写成源码失败或干净的单次记录器全绿。

封存 `darwin-current-qualified-verification-v1/REPORT.md` SHA-256：
`65e2926fe5a9d2e09773cf64a64d62ab9a211730602fef867cc0bc078a07d23e`；
`MANIFEST.json` SHA-256：
`6ee30b2415b688c091c08637e4370e51ab1d31a904f1ea68f06bbb612f9b053a`，
446 项（不含 manifest 本身）逐项读回核对。旧20项有限测试/source审阅仍保留其身份。
源候选的较早持久归档为 Library `libfile_2c346bf2769c81918d80e46a8c16892e`；
其形成早于这21门，不冒充包含本次运行证据。

### 当前边界

这仍是开发检查点。新提交及其精确 Library 恢复/现有两项 Ubuntu CI 按实际终态
另记，本文不预写未来 PASS。ASCII 根、177字节上限、受限 local APFS/HFS、
仅可证无关的 UTF-8 sibling、受限 AST 证明以及错误图/owner 边界保持下文合同。
原生 Darwin/Windows、真实文件系统/锁、物理 namespace/故障/恢复、错密钥/篡改、
密码学/法律独立审查均未验收。原生 Unicode 手工入口已有另行开发的源码，但
不在这个固定树中，当前树的该 harness 仍 IMPLEMENTATION_MISSING。

WAL publisher、strict backup、真正只读 WAL、IPK 修正等独立候选不在本批，
不能继承本次资格。ANN V4、41目标受阻 campaign 和未知中断链路仍不重试。
整体 M3/M8/M10 未完成，不能据此宣称完整验收或生产安全。

### 以下保留15:45有限范围与更早源码合同记录

以下的旧 NOT_RUN/未发布/未来表述属于其原时点；新的普通/编译结果以上节为准，
原生及物理资格仍未取得。旧来源、失败和限定证明不删改。

日期：2026-10-11。状态：当前候选的独立源码审阅已满足限定要求；尚未发布。
本页补齐源码中的文档引用，文档编辑本身不构成执行许可或原生资格结论。
V1 文档冻结时仍记录 20 项 normal/race **NOT_RUN**，原身份及证据保持。
终态补记：固定源码的两条 Linux/amd64 精确 20 项 normal/race 均 **PASS**；
额外编译门、源码门执行及原生 Darwin 验证仍为 **NOT_RUN**。本次含文档组合
没有完整套件、独立编译或 CI 结果。

## 历史身份与能力边界（当时固定树）

- 已发布基点：提交 `3ea6d3ab15f678fe48bee312921da60828845869`，tree
  `f6edabfba3e004a0e7874441b8c0a3e38cf099c6`。
- 本页说明的十路径源码候选：`darwin-current-composed-work-v1`，tree
  `88c400a4caeb40d8d96fae7700e7048d69db4ff5`，3,156 文件、148 目录记录。
  含本文的文档候选是后续独立身份；不能把源码 tree 当成最终文档组合 tree。
- 独立源码报告：`darwin-current-composed-independent-review-v1/REPORT.md`，
  SHA-256 `ec90e774280c6669a44911ed5bebcf646f757dbc7c54d4c0d675f1fa6e96e134`。
  结论仅为限定源码审阅满足要求，未发现阻断实现缺陷。

请求的目标根名仍须为可打印 ASCII（字节 32–126，排除 `/`），且满足原有
非空、非 `.`/`..` 等组件检查，最长 **177 字节**。原有上限为最长的 **78 字节**
生成后缀预留空间，使完整 basename 不超过 255 字节。Unicode 父路径原已允许。
本次不实现任意 Unicode 目标根名，也不把所有本地文件系统纳入新支持域。

本次只缩小清单扫描时“一律拒绝非 ASCII 同级名称”的旧限制。只有有效 UTF-8
同级名称，在目标 basename 范围内、遇到任何 Unicode 或控制字节之前，已经
出现可打印 ASCII 经 A–Z 到 a–z 折叠后仍不同的字节，才能得到暂定的非成员
结论；生产路径还必须通过下述实际枚举目录的文件系统检查。

以 `k.gse` 为根名，`unrelated-Ω.txt` 和 `j\u0301.gse`（后者以转义表示组合符）
可在首字节获得该否定证明；`Ω-unrelated.txt`、`K.gse`、`k\u200dx.gse`、
`k.gse-unrelated-Ω.txt` 均不能。完整匹配的根名前缀后才出现差异，不能排除
未知家族后缀。非法 UTF-8、Unicode/控制字符先于差异、短的匹配前缀均继续
fail closed。证明完成后出现的控制字符不撤销已有否定结论；目录 wire parser
原有 NUL、分隔符、长度与终止符检查仍独立保持。

所有 ASCII 名称仍走原有折叠家族语法：根、锁及规范生成成员正常计费，未知
ID/后缀继续拒绝。本次不增加家族成员类别，也不根据显示效果或后缀长度猜测成员。

## 否定证明为何只覆盖这个前缀

依据为既有 [Darwin 原生说明中的 Apple 资料记录](native-encrypted-darwin.md#verification-and-sources)
和历史 V3 的受限推理；本次未重新获取或运行 Apple 实现、Unicode 表或原生卷。
这是在已记录 Apple 名称等价域内使用的有限推论，不是完整 Unicode 等价算法。

1. 可打印 ASCII 字符在 canonical decomposition 下保持自身，且是非忽略的
   starter。后续组合符不能越过已不同的 starter，把它改成另一 ASCII starter。
2. 先做普通 ASCII 大小写折叠后仍不同，也就保守地覆盖大小写敏感卷；本次不
   调用 Go 的通用 Unicode 表来模拟 Apple 的版本化比较规则。
3. 后续可忽略字符或字符展开不能消除已成立的前导 ASCII 差异。若 Unicode、
   控制字节、Kelvin sign、long-s、组合符或零宽字符先出现，则停止证明，不
   越过、剥离或归一化它们寻找后面的差异。
4. 根、锁、生成对象及未知家族名称都须先匹配整个根名。因此差异必须位于根名
   内；根名之后的差异不够。helper 还先检查整个目标根名均为可打印 ASCII，
   避免未来扩展 Unicode 根名时误漏反向别名。

## 实际枚举 owner 与文件系统门

[清单实现](../internal/encstore/inventory.go) 把同一个 `f` 交给 `ReadDir` 和
[Darwin 分类入口](../internal/encstore/platform_darwin.go)。`f` 来自现有 managed
`openFileAt`/`openDirectory`，不是另开的探测句柄。

名称分类先发生。成员、名称错误与 ASCII 非成员保持原路；只有新允许的
Unicode 非成员才要求非 nil 的实际枚举 owner，并在持有 `dir.mu` 时依次：

- 经 `rawLocked` 检查已有 lease 的有效性，拒绝关闭、错误、idle 或 poisoned 状态
- 检查该 descriptor 的 directory 标志
- 在该 owner 的实际 descriptor 上调用一次 `Fstatfs`
- 要求 `MNT_LOCAL`，以及 NUL 终止的精确类型 `apfs`，或精确类型 `hfs` 且
  subtype 为 0、1、2、3

APFS 不套用 HFS subtype 上限。原生 C 字符串首个 NUL 之后的字节不属于类型名。
空、未知、未终止或大小写不符的类型，exFAT/FUSE/SMB、非本地卷、HFS 旧式
subtype 128 和其他保留 subtype 均不能使用此新放行。ASCII 名称的既有本地
文件系统策略不因此扩大或缩小。

`rawLocked` 的进程级 lifecycle gate 按既有 I/O 模式在查询前释放，而 lease
mutex 一直覆盖查询；公开 Close 不能在此期间退休该 lease。现有 Drain 只选择
idle records，开放的枚举目录不会因该流程被内部父级退休。这里没有路径猜测、
第二次 open、额外 close、重试或保留目录 VFS 移植。查询错误原样进入已有 owner
错误/关闭路径，不退回未经文件系统证明的名称跳过。原生竞态、内核返回值与
Fstatfs 失败均未实际验证。

## 扫描配额与成员所有权

扫描计数递增和 `MaxDirectoryEntries` 检查仍先于分类。已证明无关的 Unicode
同级名称也消耗扫描项预算，但不进入 `ns.checked`，不打开、不收取对象字节。
这仍是扫描工作上限，不是 OS inode 配额；无关目录增长仍可能造成拒绝。

选中的成员保留现有 checked namespace 路径：nofollow 打开、regular/single-link
及 native identity 校验、owned Close、非负大小检查和 physical-byte charge。
nil namespace 仍使用其既有 checked-file 路径。返回 entry type 不会把无关名称
变为成员；含糊名称不能通过忽略、猜测或一律计费绕过家族规则。

创建、重开、prepare/rekey、备份读写、restore/import、明文导出和发布清单的
既有调用时机不改变。目录内容限制发生在 inventory 内，不承诺早于每次 provider
获取或 stable-lock 操作；本次没有新增 credential/preflight 提前拒绝保证。

## 部分批次诊断与清理

一次 `batch, readErr := f.ReadDir(n)` 单独保存读取诊断。批内六种提前失败都
调用 `inventoryBatchError(readErr, operand)`，按原始读取错误在前、条目错误
在后的顺序保留节点：

1. 扫描项超限：`encryption.ErrLimit`
2. 名称或文件系统分类：`e`
3. 选中成员打开/元数据校验：`e`
4. 选中成员 Close：`e`
5. 负对象大小：`encryption.ErrFormat`
6. 字节计费：`e`

读取错误为 nil 或字面 `io.EOF` 时，helper 原样返回条目错误；只有读取错误时
原样返回它；两者同时存在时使用 `errors.Join(readErr, entryErr)`。helper 不
格式化、匹配、遍历、展开或去重任意错误图。wrapped/joined EOF 仍是诊断，
不能用 `errors.Is(..., io.EOF)` 当成正常结束。

批后只有字面 `io.EOF` 可以结束；其他非 nil 读取错误在下一次循环/context
检查前原样返回。既有 context 检查位于 ReadDir 之前，本次没有插入可覆盖待处理
读取诊断的批内取消点，也不承诺批内立即响应取消。EOF 成功仍经过最终目录校验。

原有 named-return defer 先执行 `f.owner.remember(resultErr)`，再关闭枚举 owner。
关闭错误与原结果 join，且 total 置零，不能覆盖原诊断。现有 ownership collector
仍会遍历错误图，并对过大或循环图使用有界 ErrLimit/terminal barrier；因此
helper 的惰性错误节点保留性质不能扩大为整个 cleanup 流程不遍历、或无界保存
恶意错误图的保证。没有新增原生失败注入或 cleanup 重试。

## 各层证据与真实缺口

- 独立人工源码审阅：限定要求已满足；其实际控制流、owner 和六个第二实参审读
  不等于执行证据，也不授予后续执行许可。
- 当前有限菜单：18 个无 descriptor 的内存函数，加 2 个读取源码/Go AST 函数，
  合计 20 个。前者只使用名称、direntry 字节、数字文件系统记录、计费整数和惰性
  错误值；后者读取/解析 inventory.go，其中一个另读 platform_darwin.go。
  固定源码 tree `88c400a4caeb40d8d96fae7700e7048d69db4ff5` 的精确 normal/race
  已各运行一次并自然 exit 0：各 20 个顶层 RUN/PASS，零子项、SKIP、FAIL，一个
  package PASS，stderr 为空；耗时分别 2.383 秒、4.384 秒。这只是 Linux/amd64
  有限 helper 证据，不是完整套件、跨目标编译或含本页文档树的资格。
  封存报告 `darwin-current-composed-finite-verification-v2/REPORT.md` SHA-256
  为 `16b011b671638906b47392052779537d539c72d069848efca9f01875de945711`；其
  MANIFEST SHA-256 为
  `1ecf3217d22932ccbb75088c215147007c69f854f1a145b517d6832783ed3fb3`，
  79 个条目已静态核验。权威租约释放时刻为 **15:45:02.994806 UTC**。
- AST 检查只是有限结构回归辅助。批次 guard 核对六个 helper 形状及首实参，
  不证明各第二实参或 receiver 身份；Unicode guard 的文件级顺序与必需字符串
  不证明 dominance、绑定、mutex 范围或可达性，不能单独作为控制流证明。
- 额外独立编译/跨目标编译、ordinary/manual source-selection/symbol 检查和
  guard 执行仍 **NOT_RUN**。两条有限测试自身包含 Linux 包编译与适用初始化，
  不构成这些扩大范围的门；历史 Darwin 编译结果也不能转给当前候选。
- 初始执行提案使用了未经核实的 system Go/新 cache，历史状态保持 **HOLD**，
  该提案未执行。固定工具链的修正版 V2 经主任务明确批准后，仅两条原定命令
  串行各执行一次，无重试或额外探测。本页不复制旧命令或给出新运行命令。
- 原生 Darwin/amd64 与 arm64 执行均 **NOT_RUN**。现有
  `TestDarwinEncryptedPathPolicyMemory` 的两个分类调用均传 nil 目录且只用
  ASCII 名称，不能到达新 Fstatfs 分支；新 Unicode 测试只包含名称/数值模型。
  现有 manual namespace 测试也没有 Unicode 同级 fixture。真实 managed-directory
  加 Unicode-sibling 的完整资格 harness 仍为 **IMPLEMENTATION_MISSING**，不能
  改称“已有测试只待 macOS 运行”，也没有可据此给出的原生选择器。

历史基线对 `unrelated-Ω.txt`/`k.gse` 的拒绝是原始 **FAIL**（exit 1）；V1 的
仅凭 MNT_LOCAL 放行所有本地卷为 **HOLD**；V2 丢弃待处理 ReadDir 诊断为
**HOLD**；V3 tree `62051a4f338d2e3de249694079b742ebab70c19a` 的有限旧树结果
是历史 PASS。它们各自保留，不被当前源码审阅覆盖，也不成为当前测试 PASS。

一般 Apple Unicode 根名/等价规则仍是未实现能力。原目标要求 Unicode 数据内容
和跨平台构建，并未把任意无限文件名设为验收条件；这不代表完整平台兼容。
M3/M8/M10、独立数据安全/密码学/法律审查及生产资格保持开放。物理 namespace、
故障、错密钥/篡改、崩溃、恢复、中断发布/rekey 的既有限制，以及 ANN V4、
受阻 41-target campaign 与未知中断链路均保持。详见
[限制溯源](restriction-provenance-20261011.md)、[中文手册](manual-verification-zh.md)
与[验收状态](acceptance-status.md)。本页不改变任何现有手工命令、tag/env 门或
物理执行权限。

## 后续独立候选：有限原生 inventory harness（source-only）

2026-10-11：在冻结文档 tree `8a324a8cedeccc8ba46234924c8074890aefa1da`
之外另建 `darwin-inventory-manual-work-v1`。仅新增受
`darwin && manual_darwin_inventory` 双门保护的唯一入口
`TestManualDarwinUnicodeInventory`、独立 source-pinned AST guard/纯字符串负变体
和窄 main 接线；不修改生产逻辑或旧 manual exemption。上述旧时点的
IMPLEMENTATION_MISSING 记录保留；本新候选已补的是有限目录分类/计费 harness
源码，尚未编译、运行 guard 或原生执行，不能写成完整加密资格已补齐。

新入口要求第一动作 env=1、用户明确绝对隔离 root、真实 uid/0700/无 symlink
和保留 descriptor 上的本地 APFS 或 HFS subtype 0–3，任何失败 Fatal、不 Skip。
只创建一个私有独占子目录及四个 ordinary metadata payload；三个家族名称合计
12 字节，`notes-Ω.txt` 为 7 字节。实际 retained enumerator 分类与 nil-owner
拒绝均有断言，provided namespace/self-open 两 route 都验证精确计费、字节上限
和四项扫描计数；前后样本 size/digest 保持。它们不是加密数据库格式 fixture，
没有创建密钥或执行加密/恢复/发布操作。

fixture 成功失败均保留，ledger 先登记 owner 再做后续动作，Close checked-once、
已知 child 先于 parent，未知资源/Close/依赖 barrier 保留而不重试。日志在计数目录
之外，记录 native/build/fs 身份、11 个有限断言及 retirement diagnostics。
具体真实入口、固定工具与依赖、未来命令、证据、失败排查和用户确认后清理见
[独立中文原生规程](testing/darwin-inventory-manual-zh.md)。文档命令均未执行。

旧 `88c400...` 的 20 项 normal/race PASS 不资格这个新 candidate；当前新增源码
的 parser/gofmt/guard/selection/compile/native 均 NOT_RUN。一般 Unicode 根名、
完整加密/native 验收、M3/M8/M10、历史 FAIL/HOLD 与所有既有受限门保持。


## 历史 Storage 后继层：源码集成状态（2026-10-11）

以上 V1/旧树状态为原始历史记录，原文完整保留。当前另建
`darwin-manual-storage-source-v1`，在冻结 Storage tree
`2602cc60e8c82e3c7f900eb44439e4217cb0b8c3` 之上只集成历史七路径 hunk，
并在独立审阅完整组合 `internal/testscope/main.go` 后单独更新 WAL 守卫的
该文件 pin，合计五个源码、三个文档路径。原生文件和两个专用 guard 文件
保持已封存 V2 的完整字节；V2 tree `578c30fbf9f6e360943265a3208b25c8158dfcef`
的历史独立结论为 `SOURCE_REVIEW_ACCEPTED_R1_CLOSED`，不能移作新树执行结果。
V1 HOLD、旧“待复审”及所有旧执行结论均保持其原时点含义。

本层是未发布的未来源码候选，不替换仍由独立执行流程拥有的 Storage 基线，
也不改变已发布 `f189746671aaf4962a699292a84110e2d43387e8` 的身份。
最终全树、逐文件 SHA-256/字节/Git 与 POSIX mode、完整 main 审阅及 pin 决策
见同批封存的 `darwin-manual-storage-integration-evidence-v1`；树身份不嵌入
自身文件而形成循环。本次 Go/parser/gofmt/list/编译/test/nm/guard/native
均 NOT_RUN。未来只有 9 次 go-list、5 次 compile-only 和 5 次 nm 的 19 项
有限静态提案，仍须新树独立审阅、明确审批及队列准入；没有执行许可。

所有当前 Backup/WAL 接线、ordinary/manual exclusions、namespace/recovery
政策、Windows rekey/key-session/provider/native 行为与文档均保留，不做生产
依赖回移。原始 native 手册命令属于历史模板；后续提案已由独立 addendum
纠正：保留既有 go-cache、实际 go-path/pkg/mod 与默认 /tmp，TMPDIR/TMP/TEMP/
GOTMPDIR 不设值；只新建证据输出，不增加 timeout 或 go version -m 探测，
资源门限不降低。不得直接执行旧模板或据此启动 recorder/OS runner。

双 tag/env 门、用户明确私有隔离 root、euid/0700/无 symlink、真实本地 APFS
或 HFS subtype 0–3、初始零 group/lease、固定 ASCII 家族 basename `k.gse`、
checked-once child-before-parent owner 退休以及成功失败 fixture 保留不变。
root 校验并不禁止 UTF-8 父目录；任何未来执行授权可另要求 ASCII 私有目录
拼写，不能把它写成已有源码检查。无 rename/tamper/fault/recovery、权限扩张、
新 OS runner 或自动清理。有限目录分类/计费不是加密格式、durability、完整
Darwin/native、安全、M3/M8/M10 或其他受阻 campaign 的通过。
