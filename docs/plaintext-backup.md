# 明文 Backup：保留目录与私有候选发布

## 2026-10-12 Storage + UNIQUE：冻结组合完整 26 门 PASS

冻结运行树 `4008bd9442ffd21e1e993d4f1c72dbeed3c910f3`（3,238 文件）于 **00:49:41 UTC**
自然完成 **26/26 门、28 次 Go 调用 PASS**，lease 已 RELEASED。ordinary/race
各 **8,231 named PASS / 7 个预期 oracle SKIP**；完整 pinned oracle **9,925 PASS /
零 SKIP**，LIMIT oracle/race **394 PASS**。跨编译与外平台 test-c/nm 只证明编译/
符号，未执行外国映像。本次仅叠加文档；新提交、bundle、精确恢复及对应 CI 仍 **PENDING**。
显式普通 BINARY ASC UNIQUE 子集已纳入；严格 Linux Backup/WAL publisher 仍为
**PARTIAL / NATIVE_NOT_RUN**，包括 UNIQUE 数据库的 native Backup；M10 未验收。
输入有限结果、原失败、准确终态和真实代码缺口见[本批精确证据](evidence/storage-unique-checkpoint-20261012.md)。

### 历史组合准备摘要（2026-10-12，完整运行前；原文保留）

> 2026-10-12 当前组合状态：Storage metadata 修正树 `7389e7b7` 的三条回归
> 已实际 PASS（14 根 / 92 named PASS），UNIQUE 树 `fd4e3471` 的十二条有限门
> 已实际 PASS（31 根 / 223 named PASS / 零 SKIP）；两者均自然退出并释放 lease。
> 本文所在新 Storage + UNIQUE 组合仍是 **SOURCE_ONLY / NOT_RUN**：完整 ordinary、
> race、oracle、跨编译/依赖、发布/精确恢复/对应 CI 尚未运行。下文旧的
> SOURCE_ONLY、PENDING、FAIL 与 NOT_RUN 均保留为各自身份的历史，不覆盖上述有限结果。
> 原 V1 显示契约 FAIL 与 V2 metadata 真失败（8,126 PASS / 1 FAIL / 7 SKIP；
> 后续 19 NOT_RUN）不改写。native Backup（包括 UNIQUE 数据库）、WAL publisher
> 和物理资格仍 **NOT_RUN**；完整目标与 M10 未验收。详见[新检查点证据](evidence/storage-unique-checkpoint-20261012.md)。

本批 Backup 通过既有 checkSnapshot 间接包含 UNIQUE catalog/相邻非 NULL 前缀
校验；Backup 文件字节未变不等于语义依赖未变。默认公开 DB/Tx Backup 调用
测试只验证 native 资源获取前的拒绝；描述符无关模型不构成 native 发布证据，
不能证明 native Backup 或 UNIQUE 数据库发布成功。

## 2026-10-11 Storage 文档补记：有限验证与原生资格分开

当前实现仍为 **PARTIAL / NATIVE_NOT_RUN**。冻结组合树
`2602cc60e8c82e3c7f900eb44439e4217cb0b8c3` 已包含下述严格 Linux/ext-family
rollback 明文备份协议；完整普通 26 门 V2 在 ordinary 门真实 **FAIL**，后续 19 门
**NOT_RUN**，修正后新树/完整复验仍 **PENDING**。
Backup 输入树 `ec9f30d26d71085791197cd8e25abed558ad236d` 已独立取得 36 条
有限命令 PASS，44 根 / 2,278 named PASS，并完成源码 guard、选择、仅编译和符号
检查。它没有运行六个手动 native Backup 根或其 Backup oracle 行为。

DB.Backup、只读 Tx.Backup 与 CLI --out 的既有语义、no-replace、源/目标家族
隔离、错误留存与有序退休合同不变。WAL/只读 WAL 源、非 Linux 与更广文件系统
支持仍有实际实现和验证缺口；完整物理 fault/crash/recovery harness 仍为
**IMPLEMENTATION_MISSING**。下述严格平台/命名空间前提不能由普通门或有限模型
替代。精确身份和终态分层见[Storage 证据补记](evidence/storage-checkpoint-20261011.md)。

以下首次准备段落、旧基线与原始 NOT_RUN 均按原文保留，表示其写作时点；它们不撤回
上面的有限结果，也不授予新组合或手工原生场景 PASS。正文中的“当前已发布 67318ded”
同样是历史时点；本次已发布基点为 f189，Storage 候选尚未发布。

### 历史首次准备说明（原文保留）

状态：PARTIAL / SOURCE_CANDIDATE_HOLD。本次在已发布 `67318ded6416547f26d685719d3e01e208454a87` 基线重放受审 Backup V2 源切片，并实现普通 native 测试的显式手动资格入口。仅源码组合：新 guard、编译、测试选择、native、oracle 均 NOT_RUN。旧 `fc0b1324441456f40160e81d5125dc7b97992773` 的 25 个无描述符模型与 9 个 AST 测试曾在其单独授权阶段通过；该历史结果不自动验证本次合成树。不得据此宣布一般安全备份、崩溃耐久、跨平台或目标 G1、M3、M8、M10 已完成。

## API 与实际范围

DB.Backup(ctx, destination) 与只读 Tx.Backup 保留普通最终文件路径语义；CLI backup 的 --out 仍表示该文件。没有“必须新建父目录”的新接口。已有加密 BackupEncrypted / ExportPlaintext、普通 VFS、pager、恢复、WAL、cleanup 与 CLI 密钥分支没有被这项协议改写。

DB 拒绝活动 Tx / Rows；只读 Tx 可以选择已钉住的回滚快照，惰性只读 Tx 在调用时钉住。读写 Tx、加密源、原生 WAL 会话与 WAL header 不在此范围。备份对同一 Reader 先做完整受支持 Check，再逐页复制；没有选择 RollbackRecovery，也不升级为写事务。

源库与目标主文件名必须是 1–200 字节 ASCII 字母、数字、点、下划线、连字符，不能为 . 或 ..，不能以 -journal、-wal、-shm 结尾（保守地忽略 ASCII 大小写）。最后一个限制同时排除“把另一个数据库的侧文件当成输出主库”的已知冲突。目标父目录必须已经存在；不会递归创建目录。

## 严格而未完成资格验证的平台门槛

目前只准入 Linux，且源父目录和目标父目录的 Fstatfs 类型必须为 EXT4_SUPER_MAGIC（0xef53）。这是临时 ext-family 准入信号；该值不是 ext4 独有标识，也不能证明版本、挂载参数、存储设备、缓存或断电持久性。其他文件系统明确 Unsupported，包括当前环境常见的 tmpfs / overlay。因此当前 tmpfs / overlay 上的普通测试夹具不能产生成功的原生 Backup 验证；不得通过临时移除门槛、跳过断言或把拒绝当成成功来满足验收。

必须获得 statx 的 TYPE、NLINK、INO、SIZE、MNT_ID_UNIQUE 实际返回位。缺少任何位都拒绝；不改用旧的可重用 mount ID，也不把零填充字段当证据。Openat2 必须支持 BENEATH、NO_SYMLINKS、NO_MAGICLINKS、NO_XDEV。最终发布必须使用同一已持有目录上的 Renameat2(RENAME_NOREPLACE)。不支持的系统调用/标志/文件系统行为拒绝，没有绝对路径、普通覆盖 rename、link/unlink 或恢复回退。

独立的本地内核/文件系统/存储资格验证仍未执行。更广文件系统覆盖是实质性的实现和验证缺口；魔数检查不能替代它。

## 源命名空间与快照

Reader 在获得源父目录之前先保留固定临时所有权槽，安装 memory-only BackupSourceNamespace，再获取目录。该所有者只拥有源父目录；源主文件仍由 Reader 的独立 cleanup 组持有 SHARED 锁。证明在完整 Check 和页复制期间保持存活。

statx 直接查询已经持有的源主文件、已经持有的父目录以及相对父目录的主文件目录项；不会为了获取 mount ID 重开主文件。要求：

- 主文件为普通文件、硬链接数恰为 1
- 已持有与相对命名主文件的设备/ inode 相同，并延续原 File 身份
- 主文件、相对命名文件与父目录的唯一 mount ID 一致，设备也一致
- 源父目录仍与原 Reader 捕获的物理父身份一致
- 所有 journal / SHM 目录项均拒绝；WAL 只能不存在，或是同挂载的空普通单链接直接子文件

这能拒绝已知“链接数 1，但主文件是另一个目录中数据库的 per-file bind 别名”的反例。它不是全盘隐藏别名发现机制，也不是多次 statx 的原子快照。全目录 bind 别名可能满足直接子文件要求；源/目标 family 比较仍使用物理设备/inode 父身份，不能用不同 mount ID 推断目录物理不相同。

源现有 header、extent、大小、mtime、路径稳定性检查保持有效；页仍来自同一钉住的 Reader。在目标创建前与发布前重新检查源证明。父路径改名可能令普通 Reader 安全拒绝；没有宣称普通 Reader 可无条件容忍改名。

Check 保持默认页数/逻辑读取/目录与扩展资源限制。复制一次只保留一页，长度/偏移受限；返回页在使用后清零。复制全部有效物理页，包括空闲页和原 header 字段，不进行 VACUUM、归一化或安全擦除。因此不得声称备份不含历史明文。

## 私有候选与普通同目标 SQLite 竞争

目标所有权组先保留容量并安装唯一 BackupPublication 复合所有者，然后才允许任何目标描述符获取。其原生内部 File 不暴露给调用者。

1. 获取并保留已有可信父目录。检查最终 main / journal / WAL / SHM 全部不存在；只有实际 ENOENT 才表示不存在。
2. 随机生成有界私有候选名，检查候选全 family 与源/最终 family 物理分离、也全部不存在。随机名降低碰撞概率，不提供同 UID 隔离。碰撞即失败，不循环选新名。
3. 相对同一 held dirfd，用 Openat2(O_RDWR | O_CREAT | O_EXCL | O_NOFOLLOW | O_CLOEXEC | O_NONBLOCK, 0600) 获取候选。返回的每个描述符先进入预装所有权槽，再进行任何可能失败的验证。
4. 候选获得 SQLite SHARED → RESERVED → EXCLUSIVE，之后才允许写页。没有公开最终文件的“创建到加锁”窗口。
5. 核对完整长度、源证明、目录/候选身份和最终 family；候选 file Sync 成功后，单次消耗发布许可。
6. 同一目录 Renameat2(RENAME_NOREPLACE) 将同一个仍打开、仍 EXCLUSIVE 的完整 inode 发布为最终主文件。失败不重试、不覆盖、不回滚 rename。
7. 发布后进行身份/sidecar 观察与单次目录 Sync；Sync 结果独立保存。即使观察失败，也尝试该次目录 Sync，然后返回错误，不能声称完整成功。
8. 封闭全部命名空间/I/O 能力，解除只用于目录生命周期的内存引用，先完成目录原生退役，再关闭候选/最终主文件并释放其保护锁。

正常 SQLite 客户端可以同时打开同一个最终主文件：若它的主文件创建先完成，no-replace 发布拒绝，Backup 不会写它的 inode；若 Backup 先发布，它获得的是完整且已加锁的同一 inode，正常事务必须等相冲突的 SHARED 锁能够获取。sqlite3_open 本身不保证返回 BUSY，客户端可能等待、返回锁错误或在保护锁释放后成功写入。Backup 不能承诺释放保护锁后文件一直不变。

以上不是四个目录名的原子保留。正常 main-first、lock-before-sidecar 竞争与“直接操作侧文件”是不同的情形，不能将正常同目标竞争一概排除后声称协议修复了它。

## 外部必须满足的信任条件

源/目标命名空间稳定且可信；各数据库使用物理分离的普通 SQLite family；源的所有使用者遵守同一个已准入 family。没有 actor 读取/写入私有候选，没有链接、改名、删除、替换、重新挂载所选目录项，也没有绕过锁的 VFS 写入。尤其不能直接创建/修改目标 sidecar，也不能把 target-wal 等名称当成另一个独立数据库主文件。

0700、目录 inode、进程内 registry 或 flock 均不能证明同 UID/特权/继承描述符的 actor 不存在。此协议没有遍历 /proc 或扫描文件系统来“证明全局独占”。immutable / nolock 等绕过锁模式不属于锁排除保证。任意不可信同权限 actor 的一般安全支持仍为 HOLD；不能靠更短的 API 注释抹去这些前提。

## 退役、错误与保留产物

保护性 Sync 的结果、子对象对父目录的生命周期引用、原生描述符退役完成是三种不同事实。SealForRetirement 只做内存状态/互斥处理：不可逆地阻止后续 create/stat-by-name/write/lock/publish/Sync，解除引用时不 close、不 unlock。目录原生 Close 完成后，主文件保护锁才可释放。

Close 返回 nil 不等于 RetirementComplete。已消费描述符但返回错误时可按明确完成事实继续，整体仍返回该错误；未完成或消费状态不明时保留所有者和保护锁。内部 cleanup token 经有界诊断封装保留 Is/As 原因，但不向外部通用 adopter 暴露可独立重试的内部 File；未知、循环或超预算所有权错误成为终止屏障。

源证明和目标复合所有者始终独立。源证明退役只关闭临时父目录，不关闭只读 Tx 的源主文件。DB Backup 随后关闭自己创建的 Reader。若源证明已在本次调用中 Close 一次但未完成，DB 专用关闭路径不会立即再试；转交显式 RetryPlaintextCleanup。源证明/源关闭失败都抑制 FileInfo 成功。

一旦尝试候选创建，任何后续错误保守携带 ErrCommitUncertain。失败可能留下私有候选，也可能留下最终文件；既有竞争者文件始终不被收养、删除、截断或覆盖。产物保留供单独审查，不自动修复、移除、换输出名或重做备份。ErrPlaintextCleanupPending 仅表示还有原生所有权责任，不表示发布或耐久性成功。RetryPlaintextCleanup 只能退役已表示的资源，不能重放创建、页写入、Sync、rename、源 Check 或恢复。

## 证据与下一道门槛

新增 descriptor-free 模型直接调用生产状态接口的有限方法，包含每个阶段失败、短写、普通同目标两种发布顺序、检查后/rename 前竞争、独立 sidecar 反例、旧 pin 循环、nil/error 与 complete/incomplete 的所有退役组合、未知/循环所有权、源/目标独立、同次重复关闭抑制、缺 statx 返回位及 bind/硬链接/sidecar/名称反例。

AST 源码检查调用实参与直接控制依赖：held dirfd、Openat2 必需标志、RENAME_NOREPLACE、descriptor 先所有权后验证、锁/完整复制/Sync 发布门槛、目录先于主保护者关闭、源 token 不被目标收养、Check 在目标前、重试仅退役。负面源码变体覆盖删 resolve 标志、覆盖 rename、AT_FDCWD、丢 O_EXCL、丢返回 mask、早关主文件、伪造源准入、seal 解锁及源 token 错误归属。这些模型/AST 在旧冻结 V2 上的有限历史 PASS 与本次合成后的 NOT_RUN 必须分别记录。

原有六个测试名、10 个叶用例、12 次 Backup 和 7 次 oracle 进程调用全部保留。三个测试文件现在要求 `linux && manual_backup_publication`；oracle 文件另要求 `oracle`。共享 helper 强制 `GOSQLITE_MANUAL_BACKUP_PUBLICATION=1`、明确绝对 root、同 profile 的 CWD/TMP/fixture/evidence、先验旁置来源 manifest。失败为非零，不能 Skip 当 PASS；成功和失败都保留完整原 fixture，正常资源只结束一次，用户确认进程自然退出且证据保存后才人工清理。

默认少选 5 项、仅 oracle tag 少选 6 项，比较的是**未发布 fc0b 候选迁移前后**；当前已发布 67318ded main 原先没有这三测试文件，不能描述为从其 CI 删除六项已合入覆盖。模型、加密维护与其他历史测试不迁走。

只有本次新增 backup oracle caller 改接已发布的 `internal/testoracle.ForTest`：固定前置 `-noinit`，精确五键 child Env，CWD 来自明确 root/work；factory 及原 startup guard 原字节保留。oracle root 必须是已真实合格的 `/tmp/...`，不从 `/mnt` 悄悄退回 `/tmp`，不挂载、不 bind、不改 factory。env 隔离不是 OS 沙盒；临时目录持续可用是外部前提，不能声称异常 fallback 已被内核阻止。

完整步骤、manifest schema、未来命令、失败保留和手工清理见 [手动资格验证手册](testing/plaintext-backup-manual.md)。源码/选择检查仍需独立批准，native/oracle 没有因此获准运行。

还需对精确源码进行独立所有权/导入/初始化/模型闭包审查，再分别申请纯模型/AST执行、普通 ext-family 存储验证、固定官方 oracle，以及新的有限手工原生锁/命名空间/挂载/故障/断电 harness。此修订没有实现或启动该 harness。当前 ANN V4、41-target fuzz 和未知中断链保持 HOLD，不能顺带恢复。发布、集成与总体生产可用声明仍需另外批准。
