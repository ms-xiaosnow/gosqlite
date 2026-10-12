# 六种 rekey 中断场景：源码设施与人工步骤

状态：SOURCE_ONLY / REVIEW_HOLD。本文、模型和测试函数都不是执行授权。
本次没有运行 Go、gofmt、编译器、AST 扫描器、测试、数据库创建、错误密钥 Open、子进程退出或归档恢复。以上全部为 NOT_RUN。

## 固定范围

只支持 Linux、snapshot/layout 1、RawKey256、4096 字节逻辑页、单连接、三页数据库、一张表和一个普通二级 B-tree 索引。这里的页大小不是硬件扇区大小。每次独立审批只选择以下一个 case，使用一个全新、私有且保留的 run 目录，不循环六个 case。

| case | 退出前最后一个已观察事实 | public Rekey | 预期同内核重新观察的根 |
|---|---|---|---|
| control | 正常返回并显式 Close；子进程自然退出 0 | nil | NEW |
| before-root-rename | 已验证候选；尚未把 attempted 设为 true | 未返回 | OLD |
| after-root-rename-before-dirsync | 根 rename 返回 nil；尚未执行其后目录 Sync | 未返回 | NEW |
| after-root-dirsync-before-adoption | rename 后目录 Sync 返回 nil；尚未最后 readback/adoption | 未返回 | NEW |
| rekey-returned-no-ack | 原始 public nil 帧已由父进程收到；没有 ACK 文件 | nil | NEW |
| acknowledged-exit | 父进程以原始 nil 帧完成 ACK 文件与目录同步，再授予退出 | nil | NEW |

中断退出码固定为 86；协议终止码为 87。86 本身不构成通过。信号、OOM、panic、超时、缺失帧、实际 Wait 不明确或正常退出 0 都不能冒充指定中断。前三个切点不是 ErrCommitUncertain 返回，必须记录为未返回。

这只描述同内核下受控进程退出。它不模拟掉电、SIGKILL、写入或 Sync 错误、设备丢失、ENOSPC、layout 2/3、共享 rekey 或任意重启历史。无 ACK 不代表 OLD；ACK 目录的 Sync 不能替代数据库目录的 Sync。

## 正常构建边界

生产 publishRoot 只增加三个无返回值的 observer 调用；原有分支、真实 I/O、错误传播、adoption 和 Close 顺序保留。普通或非 Linux 构建选择 `!linux || !manual_rekey_interruption` 空函数。手工能力仅在精确 `linux && manual_rekey_interruption` 下存在。

observer 不修改引擎数据、不持有引擎切片、不读取密钥、不回调 DB、不替换 syscall 或返回值。它只复制/哈希有界密文及公共头，读具体 borrow 原子计数，在私有 observer 锁释放后使用固定管道等待。调用时 DB/session 锁仍可能持有，严禁重入。cleanup snapshot 仅持 registry.mu 读取计数及 atomic pending 指针，不拿 group 锁、reap、retry 或调用资源状态方法。

## 人工执行前必须完成的源码检查

这些是检查顺序，不是可直接执行的命令配方；实际执行仍须另行批准。

1. 独立审阅最终 overlay、全部 observer/provider/错误/owner 路径、完整 inspectSource/main，以及新增和受影响的 guard。旧 A5 限制保持不变；不得以 manual tag 整体豁免。审阅后才可更新明确受影响的源码 pin，包括 A8 whole-main pin。
2. 冻结源文件和 SHA-256，确认只存在下列八个值模型根及七个 AST 根。全部输入都必须是内存中的惰性值或源码字符串。模型不能打开文件或数据库。
3. 如另行获准，按冻结 selector 分别进行有限模型/AST、普通 Linux、新 tag Linux、旧 rollback tag、非 Linux 的源码选择检查。多 manual tag 的组合检查须单独明确允许。不得扩大为 fuzz 或 case campaign。
4. 如另行获准，只编译精确 sqlite tagged test package 到独立保留输出，固定 compiler、完整 source manifest、编译选项和 binary hash。不得为了列测试而启动这个 binary。编译成功不是运行许可。
5. 任一 Go 工具、扫描、编译、实体 fixture 或进程行为若被拒绝，停在该边界；不能换 executor、CI、shell 或另一入口重建被拒链条。

八个值模型根：
- TestRekeyInterruptionSixCases
- TestRekeyInterruptionCutOrder
- TestRekeyInterruptionCredentialPurpose
- TestRekeyInterruptionSelectedGeneration
- TestRekeyInterruptionSQLIndexLedger
- TestRekeyInterruptionAckCausality
- TestRekeyInterruptionOwnersAndArtifacts
- TestRekeyInterruptionNeverPhysicalPass

七个源码/AST 根：
- TestRekeyGuardInventoryGate
- TestRekeyGuardPublicationOrder
- TestRekeyGuardObserverIsolation
- TestRekeyGuardPublicReturnACK
- TestRekeyGuardProcessOwnership
- TestRekeyGuardProbeLedger
- TestRekeyGuardIntegrationDefault

## 另行授权后的人工操作步骤

1. 为一个选定 case 明确批准受控退出、两个独立错误/正确密钥 probe、保留目录、复制与 ACK 的文件操作，以及卡住子进程的后续处置边界。只有 control 可以选择 ArchiveControl=true；这只是对照附属检查，不是第七 case。
2. 选定已审阅的 Linux/文件系统，准备当前用户拥有且权限 0700 的稳定父目录。run Root 必须尚不存在。不可复用、覆盖、删改旧 run；不要求自动清理。绑定中的 Filesystem 必须和实际 Statfs 类型一致。
3. 准备 canonical JSON source manifest：Files 是所有 .go、go.mod、go.sum 的完整相对路径映射，各项具有 Bytes 与 SHA256。上限 4096 项，逐项核验真实源字节，不接受仅声明树 hash。SourceRoot 必须可读、绝对且无符号链接路径。
4. 准备 canonical compile receipt，Schema 为 rekey-interruption-compile-v1，固定 SourceManifest、Compiler、Binary、Package=./sqlite、Tags=manual_rekey_interruption、GoVersion、CompileOnly=true 和源码中唯一 Flags 列表。保存实际 compiler/binary 的 SHA-256 和路径。runtime 会哈希 /proc/self/exe 并比对同 inode 的 BinaryPath；argv[0] 或同名文件不是证据。
5. 准备 canonical binding，字段为 Schema、Trial、Case、Root、Filesystem、Declaration、SourceRoot、SourceManifestPath、SourceManifest、CompilerPath、Compiler、BinaryPath、Binary、CompileReceiptPath、CompileReceipt、ArchiveControl。Schema 为 rekey-interruption-binding-v1；Declaration 为 linux-process-exit-no-powerloss-retain-all。Trial 是最长 64 字符、无空白或路径分隔符的本次标识。
6. 用已批准的唯一测试根 TestManualRekeyInterruptionController 和源码中的固定 flags 启动已冻结的同一 binary。该入口首先核对独立 gate、角色、GOFLAGS 为空、非特权用户、绑定 hash、源码和 image。不要直接手工调用 writer/probe；它们只接受父进程创建的 fd3/fd4 和 fresh challenge。
7. 控制器建立原始 fixture、验证并关闭，完整复制 trial；启动一次 writer 并实际 Wait 一次。每个 child 的 role-ready 必须得到父进程的 continue grant 才能进行 native 角色工作。父进程只通过自己持有的 exec.Cmd 启动/等待，不从帧取 PID，不使用 shell、PATH lookup、Kill、Signal 或 CommandContext。
8. 若报告 CHILD_LIVE/WAIT_UNKNOWN，控制器本身保持阻塞，不返回测试主进程；保留进程和管道对象，不启动 probes，不以超时假定退出，不自动替换或杀死它。后续处置需要用户明确决定。即使 Wait 调用返回，若没有实际 ProcessState，仍保持未知并持续保留，不重复 Wait。正常终止必须在实际 Wait 后读取事件管道至精确 EOF，额外一个字节也拒绝。
9. 实际 Wait 后，将 trial 完整冻结复制到 evidence/post-exit，再分别复制 probe-old 和 probe-new。两个 fresh process 各固定一个 purpose，只 Open 一次自己的 ReadOnly/NoCreate 副本。不得尝试另一密钥、retained-root fallback、RecoverRollback 或修复原始家族。
10. 只有两个独立结果形成一成一败、成功方完整逻辑账本和选定 generation 一致、失败方是精确 Open authentication，才记录该 case 的观察事实。保留全部目录和 control 内的绑定、帧、进程、诊断、SQL、对象、nonce 及最终观察记录；没有物理 PASS 字段。

必要 gate 和传输环境变量名称已固定在源码中：GOSQLITE_MANUAL_REKEY_INTERRUPTION、GOSQLITE_REKEY_ROLE、GOSQLITE_REKEY_BINDING、GOSQLITE_REKEY_BINDING_SHA256；父进程为 child 增加 GOSQLITE_REKEY_CHALLENGE。gate 字符串不授予执行权限，不要把它当作一次新授权。

## 密钥 purpose、逻辑账本与错误分类

OLD 和 NEW 是源码内 domain-separated 公共一次性测试材料，绝不可用于真实数据。无用户 key、secret 文件、DSN、argv secret 或环境 secret。provider 只依据固定 purpose/path/operation 选择标签，不根据尚未认证的 DBID/epoch 选密钥。每次 Acquire 克隆 borrow，一次 Release 清零；重复释放或未释放拒绝。writer 的 OLD 和 NEW borrow 原子计数在 observer 处均须为 1/1。进程退出不证明内存安全擦除。

基线是 `(1,red,one),(2,blue,two),(3,red,three)`，严格 int64/string/string。三条固定 autocommit SQL 后必须实际观察到 3 页及 sequence=4，rekey 候选 sequence=5。不能从文件数量猜这些数字。

七个查询分别覆盖全表扫描、blue 索引/扫描、red 索引/扫描、green 空结果索引/扫描。每个查询独立比对字面预期，验证 SEARCH/btree/by_k/k/recheck 或 SCAN/NOT INDEXED requested；空 green 也必须有真正的 index-page probe。逐一记录 Rows.Stats、EOF、Rows.Err 和一次 public Close。Check 固定 MaxPages=3、MaxPageReads=512，并要求 3 页、1 表、1 普通索引、3 行/3 索引项、无 free/reserved/extension，正且有界的读数。

精确 wrong-key 结果要求 nil DB、原始 *sqlite.Error、Op=open、CodeNotADB=26、errors.Is(ErrAuthentication)，并排除 cleanup/poison/busy/changed/recovery/uncertain/format/limit/provider/cancellation/path/syscall/EOF 等污染。原始 error 对象留在进程中；JSON 只记固定分类码，不输出任意 Error() 文本。失败 Open 后不 Query、Verify 或 Close 一个不存在的 DB。

成功 probe 在 VerifyEncrypted 后核对 layout、mode、页数、DBID、完整 root header/hash、sequence，以及完整 SQL/Check。任何未知 Close、pending token、丢失 owner 或缺少实际 Wait 都不能变成成功。

## 保存布局与 nonce 观察

inputs/original 是已关闭且不可修改的基线家族；trial 只给 writer 修改；evidence/post-exit 在 writer Wait 后生成；probe-old/probe-new 分属两个独立 inode 家族；control 保存证据；可选 inputs/old-archive.gse 是 BackupEncrypted 创建的便携归档，restore-new 是新的恢复目的地。

家族复制枚举全部条目，最多 64 项、总计 1 MiB、每项 64 KiB。保留 root、零长 .gse-lock、base、pending、retained；拒绝未知类型、名称/头身份不一致、符号/硬链接、同 inode 别名及明文 sidecar。不对 source 增加 Sync，只对独立 O_EXCL 目标文件及其目录同步。前后 manifest 记录类型检查、dev/inode、长度和 SHA-256。

native nonce 观察遍历完整 original 与 post-exit 家族，包括 seed sequence 1–3 的历史 root/base、基线 sequence 4 及候选 sequence 5；旧历史页数可以是 1–3。逐个解析固定范围内的 metadata/data/footer 记录，记录 credential label、DBID、epoch/RootSalt、FileID、文件/记录角色、物理 offset/extent、sequence、NoncePrefix 和 12 字节 nonce。全对象 hash 相同的副本/retained 别名只算同一历史事件；不同密文事件复用 domain/nonce、sequence 或同 FileID 不同字节则拒绝。模型针对选定 OLD/NEW root/base 的有限值关系，并覆盖 control 有/无 archive 两种附属选择；canonical JSON transport mirror 单独覆盖 native wire schema，聚合模型 trace 编码不冒充 fd 协议。

模型中的 LogicalPages/PageOneCounters 仅为合成值关系。native 目前没有观察明文页 hash 或页一计数器的逐字节相等性；完整 SQL/index/Check 和认证根身份不能冒充这一证明。

这只是公共派生输入和固定 fixture 的观察，不导出派生密钥、不证明随机性或整个重启/失败写入 nonce 安全性。模型 Result.PhysicalValidated() 和 Result.Pass() 永远为 false；实际记录的 PhysicalValidated 也保持 false。

## 可选 control 归档

归档须在 rekey 前以 OLD archive-target purpose 创建并保留其独立身份。control companion 使用 OLD archive-source 与 NEW restore-target，将它恢复到新建且不存在 root 的 restore-new。再以独立 NEW restore-open purpose 只读打开一次，验证全部七查询与 Check，关闭一次。新 DBID 必须不同于 source 和 archive，epoch/file identity 也须新建，source/archive manifests 必须不变。

这不覆盖 uncertain root、不回退 active format、不恢复 source DBID、不提升 retained 根，也不撤销 OLD 读取历史归档的能力。实际 archive execution 为 NOT_RUN。
