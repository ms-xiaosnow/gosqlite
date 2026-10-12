# 2026 年 10 月 12 日完整正常门终态说明

本手册对应的冻结候选 tree `f8192e090acb9acedb07f811a66d043c1a9a24c2`
已于 13:31:44 UTC 自然完成完整普通 26 门 / 28 次 Go 调用 PASS，session
`75199` exit 0，lease RELEASED；[准确终态与验收边界](../evidence/readonly-acceptance-checkpoint-20261012.md)
同时列明普通/race/oracle、source/AST、闭包和 foreign compile 的不同范围。

这包含真实 Linux 健康文件/WAL 路径，不能统称全部 native 未执行；但本手册的
真正 manual/物理场景仍 **NOT_RUN**。被审选择和闭包只资格已核验的初始化，
单一 selector 不隔离 init，也不能为未知初始化或其他环境担保。新提交/bundle/
精确恢复/新 CI 仍 PENDING。本说明没有更改下方任何命令、tag/env 门或授权要求。

## 历史前置说明和完整原手册

以下在途 PENDING、SOURCE_ONLY/HOLD 与旧 pin 状态保留原时点含义；
最新普通门终态以上文及检查点为准，旧原文与命令逐字保留。

# 2026 年 10 月 12 日只读验收 V3 检查点说明

本手册所述源码已纳入冻结候选 tree
`f8192e090acb9acedb07f811a66d043c1a9a24c2`。对应 source/纯模型/编译有限证据、
已核验的 ordinary 阶段与仍在执行的 full26 必须分开阅读：
[当前检查点与准确证据边界](../evidence/readonly-acceptance-checkpoint-20261012.md)。
本稿快照的完整 26 门、race、完整 oracle、新提交/恢复/CI 均未闭合。

真实 layout1→2 升级及 archive→全新 snapshot 目标恢复均 **NATIVE_NOT_RUN**；这是有限源码设施，不是原地 downgrade、全格式矩阵或中断恢复资格。

下列原文逐字保留，包括作者阶段 SOURCE_ONLY/HOLD、旧 pin 状态和既有命令模板。
这些是历史记录；新状态以上述检查点为准。这里没有生成新命令，任何 tag、环境变量、
模板、编译产物或本说明均不构成 native/物理执行授权。

## 历史手册原文完整保留

# 加密 layout1→2 与保留归档恢复：有限人工验收

状态：SOURCE_ONLY / REVIEW_HOLD / NATIVE_NOT_RUN。本文件中的任何命令均未执行。Go AST、纯内存测试、格式化、编译、原生运行均为 NOT_RUN。所有新 source pin 是 HOLD；原有 WAL guard 的 main.go pin 未机械更新。须先独立完整源审，再分别批准有限验证和人工原生执行。源码存在不等于数据安全资格。

## 1. 精确范围

唯一原生入口：sqlite/encryption_upgrade_manual_linux_test.go 中 TestManualEncryptedSnapshotToWALUpgrade。

- build tag 必须恰为 linux && manual_format_upgrade；实际编译 tags 值恰为 manual_format_upgrade
- 第一条语句检查 GOSQLITE_MANUAL_FORMAT_UPGRADE=independently_authorized。设置变量本身不是授权
- 固定顺序三项：512、4096、65536；不支持子项选择，不并行，不继续失败后的余项
- 页大小依据：internal/pager/header.go:58 与 encryption/format.go:96 接受 512..65536 的二次幂；512 是下界，4096 是默认值，65536 是上界。不是为此测试新加页大小
- 只构造新的、正确测试密钥的健康 layout1 family；调用已支持的 UpgradeEncryptedWAL 将它显式升级成 layout2
- BackupEncrypted 产生独立 portable archive。其返回 StorageLayout=0 是 portable 信息，不是 layout1 family；实现锚点 internal/encstore/backup.go:51
- RestoreEncrypted 将升级前 archive 恢复到完全不同、此前不存在的目标，正确模式为 snapshot/layout1。此“回退”是旧快照到新目标的恢复，不是原地降级
- 不调用 shared/layout3 升级、Rekey、plaintext export/import、ANN、oracle、fault、kill、tamper、错误密钥、用户准备的数据库或 namespace 破坏流程

生产 API 原合同见 sqlite/encryption.go:179–207、sqlite/encryption_copy.go:48–78、118–151。Backup/Restore 的传递实现未改动。原始测试入口 encryption_shared_test.go 与 encrypted_wal_review_test.go 只用于源码核对，未调用它们的 fixture/helper，也未重跑。

## 2. 必须先满足的环境与隔离条件

1. 已完成独立完整源审，冻结源目录、所有 .go 与 go.mod/go.sum 的源摘要、编译命令和二进制摘要。源审应包括 testscope main 的所有旧守卫传播和新纯 helper 的默认调用闭包；不要只更新 hash 使守卫变绿。
2. Linux/amd64 或 Linux/arm64；准确 Go 1.27.2、CGO_ENABLED=0。runtime 与 build info 必须都能证明此版本、OS/arch、CGO 与 tags。缺失 build info 或不匹配一律拒绝，不允许跳过。
3. 已离线准备并校验 go.mod/go.sum 所需依赖：golang.org/x/crypto v0.58.0、golang.org/x/sys v0.49.0、golang.org/x/text v0.43.0；不允许 replace。命令禁止自动下载 toolchain/modules。
4. 使用普通非 root 用户。用户提供已存在、属当前有效用户、权限恰为 0700、绝对规范路径、无 symlink 的已知测试根目录。根目录可以已有其他文件，harness 不读取/修改那些文件；仅新建随机 format-upgrade-* 子目录。它不 chmod 根目录，不 remount，不配置操作系统。
5. 用户独立确认该根及祖先命名空间稳定、无别名/替换、无其他进程参与，无远程共享或不支持锁/文件与目录 Sync/原子发布语义的存储。GOSQLITE_MANUAL_FORMAT_UPGRADE_NAMESPACE 的声明无法机械证明这些前提。harness 记录实际 Linux statfs 与 uname 事实；成功仅绑定这一次环境，不授予其他文件系统、硬件或掉电资格。
6. 源目录、二进制和证据输出都使用独立保留副本，不置于其他用户数据家族中，不覆盖既有证据；证据 stdout/stderr 文件放在本次人工独立创建的证据目录，绝不在目标 family 内。
7. 不启动其他测试、不使用 go test ./...、项目脚本、模糊 selector、oracle tag、-race、fuzz、bench、shuffle、自动 timeout/kill、远端/CI 代跑。每项 API context 2 分钟是协作式取消，不承诺所有内核 I/O 都会在期限内返回；若调用卡住，应保留现场并停止宣称完成，不引入未经批准的 kill。

## 3. 将来独立获准后的命令形状

以下只是未执行的准确命令模板。替换路径和批准编号，并保留每一步真实退出码、stdout、stderr。任何一步失败就停止，不自动修复环境或重试原生入口。已有 source pin HOLD 时不得进行本节。

首先只编译人工二进制（编译也需独立许可，不能把本文件当授权）：

    cd "$REVIEWED_SOURCE"
    env GOTOOLCHAIN=local GOPROXY=off GOSUMDB=off CGO_ENABLED=0 GOFLAGS= \
      go test -c -tags=manual_format_upgrade -o "$EVIDENCE/format-upgrade.test" ./sqlite

使用外部已审定的哈希工具计算二进制 SHA-256。源摘要算法必须与源码 sourceDigest 完全一致：递归枚举源目录所有 .go、根 go.mod、根 go.sum（仅排除名为 .git 的目录），拒绝 symlink，按相对路径的斜杠形式排序；逐项写入 UTF-8 文本“相对路径 + TAB + 文件小写 SHA-256 + LF”，再对这份文本做 SHA-256。不要把目录身份字符串当作源摘要。源清单最多 4096 文件/1024 目录、单源文件 8 MiB、累计 128 MiB；二进制最多 128 MiB。构建记录负责建立已审定 source/binary 配对，两个相等 hash 本身不能证明编译来源。

在拥有独立原生运行许可后，才执行准确入口一次；不要用管道隐藏退出码：

    env GOFLAGS= \
      GOSQLITE_MANUAL_FORMAT_UPGRADE=independently_authorized \
      GOSQLITE_MANUAL_FORMAT_UPGRADE_APPROVAL="$APPROVAL_REFERENCE" \
      GOSQLITE_MANUAL_FORMAT_UPGRADE_NAMESPACE=private-stable-no-alias-no-other-process \
      GOSQLITE_MANUAL_FORMAT_UPGRADE_SOURCE_ROOT="$REVIEWED_SOURCE" \
      GOSQLITE_MANUAL_FORMAT_UPGRADE_SOURCE_SHA256="$SOURCE_SHA256" \
      GOSQLITE_MANUAL_FORMAT_UPGRADE_BINARY_SHA256="$BINARY_SHA256" \
      GOSQLITE_MANUAL_FORMAT_UPGRADE_ROOT="$KNOWN_PRIVATE_ROOT" \
      "$EVIDENCE/format-upgrade.test" \
      -test.run='^TestManualEncryptedSnapshotToWALUpgrade$' \
      -test.count=1 -test.timeout=0 -test.parallel=1 -test.shuffle=off -test.v \
      >"$EVIDENCE/native.stdout" 2>"$EVIDENCE/native.stderr"
    status=$?
    printf '%s\n' "$status" >"$EVIDENCE/native.exit"

Go flag 实际字符串把 timeout=0 规范化为 0s，harness 检查 0s。所有非白名单显式参数都被拒绝，test.v 必须明确为 true（遗漏和 false 都拒绝），允许的参数只有上述六项；空 selector、宽 selector、重复运行、隐藏子项、-test.list 等均不允许。环境变量中的批准编号为 8–128 字符的非秘密记录标签，不是密钥。

## 4. 每项实物断言与保留目录

每个 pageNNN 下有 source、archive、restored 三个新 0700 目录。文件必须是本用户的直接 regular file、0600、nlink=1；每个清单最多 256 项、总和最多 64 MiB。hash 读取有界，先捕获 file owner，再核对 held/lstat 身份、大小和 mtime，Close 只尝试一次。清单记录名、大小、SHA-256、device/inode、mode，并绑定已核对的目录身份和精确阶段。receipt 保留 source root、known root、run directory、每项 case/source/archive/restored 的 path/device/inode/UID/mode；验证直接 owned-child 关系和源/归档/恢复物理身份不重合。restored 的完整清单属于 Case receipt，不只散落在日志中。

每项流程：

1. 创建 docs 三行固定 ledger，验证整数/浮点/文本/BLOB/NULL 类型和值，显式 UNIQUE docs_token 和 grams13 docs_grams；dense VECTOR(2) 逐行验证规范化向量文本、维度和精确距离，无 ANN。
2. VerifyEncrypted 确认 layout1；Check 通过支持范围与资源界限。UNIQUE 的 sqlite_schema SQL 必须仍是原来的 CREATE UNIQUE INDEX；现有 planner 明确排除 UNIQUE index lookup，因此 equality 查询必须走 SCAN、空 Method/Indexes，并返回准确行；Check 与重复插入拒绝仍验证 UNIQUE 内容/语义。没有新增非 UNIQUE index 或扩展 planner；grams 查询必须走具名 grams13，并和 NOT INDEXED 扫描及固定期望结果一致；完整 row ledger 与精确向量值逐项一致。
3. 在自己刚创建的 fixture 中尝试一次普通 UNIQUE 重复插入，要求 ErrConstraint、原 head/sequence 与全部 family 字节/身份不变，并重验 ledger/index。它不是 I/O fault 或未知用户数据库变更。
4. BackupEncrypted 保留升级前 portable archive，记录归档返回信息及全部文件 hash。归档由后面的 RestoreEncrypted 再完整认证和 page ownership 验证，不虚构不存在的公开 archive-only Verify API。
5. 无活动 Rows/Tx 时显式 UpgradeEncryptedWAL。确认 layout2、WAL mode、同 DBID、sequence 恰 +1、零 WALSegments、PageCount 不变。
6. source Close 一次成功后，原 immutable 对象与 lock 的 inode/字节不变。mutable source.gse 的 inode/hash 必须合法替换；其旧 ciphertext 必须出现在 source.gse.gse-<64个十六进制字符>.retained 中。不能要求 mutable root 自己的旧字节不变。
7. 旧 snapshot ReadOnly+NoCreate Open 必须 ErrUnsupported、CodeError、nil DB、无 pending cleanup；拒绝前后完整 source family 清单完全一致。
8. WAL ReadOnly+NoCreate 重开，head 完全一致，重验全部 rows/index/精确向量，再 Close 一次。
9. 再确认 restored.gse absent；用保留 archive 和正确 synthetic provider RestoreEncrypted。新目标必须是 layout1/snapshot，非零新 DBID，且不同于原 source 和 archive。正确 snapshot ReadOnly 重开并验证返回/实际 head、所有原始 rows/index/精确向量，再 Close 一次。
10. source 的 layout2 最终清单与 upgrade 后相同、archive 与 upgrade 前相同。新 restored 的完整清单也输出并保留。PASS 与 FAIL 都不删除任何目录/文件。

## 5. owner / 返回 / 失败语义

每个 OpenContext、Query、os.Open 的 owner 与 error 在下一条语句纳入 ledger；新目录、BackupEncrypted、UpgradeEncryptedWAL、RestoreEncrypted 等无 owner API 的结果立即写事件。每条 event 保存真实返回/error；只输出 provider 标签，绝不输出 key/provider/options 对象。fixture key 是源码可见算法产生的公共测试材料，不读取用户密码、环境秘密或真实 KMS；绝不可用于真实机密。

Rows 完整读到 EOF 时，该生产 API 已自动 close；立即记录同一个 Rows 的 Err 和 EOF 一次退休，不再追加一个 Close 把未知结果抹掉。提前失败时只对仍明确 live、从未尝试 close 的 owner 按逆序 Close 一次。Close 有任何诊断都保守记为 unknown，即使真实实现可能已经释放；绝不盲重试。unknown Rows child 存在时其 parent 不做隐式再次关闭。每条 API 结果在任何日志编码或 Fatal 前先保存原错误及 PendingEncryptedCleanups tokens；公开 token 无法精确绑定 DB 时保守阻止全部 live DB/Rows 的 parent Close，避免 Close 再次驱动未返回临时 owner 的 cleanup group。阻止状态和历史 pending 标志留在终态证据中，即使之后 registry 变空也不放行。全局保留指针和错误 token 直到本测试进程结束；不使用 finalizer、RetryEncryptedCleanup 或 sweep。进程退出释放 OS 资源不算成功退休证据。

失败停止余项，保留已创建路径、部分归档、新 root、旧 immutable/retained、错误和 pending registry。未返回的调用、unknown、pending、quota、unsupported prerequisite、编码失败、预算越界、任何 skip 都不能记为 PASS。harness 不跳过失败，也不会更换目录权限、存储、OS 或测试模式求通过。

## 6. 最终证据和判据

应保存：独立源审记录、批准引用、精确 source 清单与 digest、工具链与依赖身份、编译命令/退出码、二进制 digest、运行原始 stdout/stderr/exit、每个 FORMAT_UPGRADE_EVENT、完整 terminal-receipt、三个 pageNNN 目录的全部原始文件。日志有内部事件上限 65536；日志截断不算通过。

成功必须同时满足：

- 外部观察到正常测试进程退出码 0；精确一个测试正常 PASS，无 skip/FAIL/panic/timeout，且完整终态 receipt 的同一次 source/binary/approval/runDirectory 身份一致
- FORMAT_UPGRADE_COMPLETE cases=3 preserved=true owners=retired 恰出现一次，不能单靠这一行判定
- receipt 三项恰为 512、4096、65536，十个断言顺序完全一致，无缺失/重复/替代；全部 head、manifest、archive conservation、fresh identity 合同通过
- 每项必须有 3 个实际 DB、1 个精确旧模式 nil-owner 拒绝、20 个具名 Rows，全部 parent 指向正确 DB；变动数量的 source/hash file owner 必须逐一对应 capture 时保存的 acquisition inventory，不能用一个退休 owner 代替。全部 owner 一次退休或精确预期拒绝；每项三个 provider 标签均有正数 Acquire/Release 且守恒、无重复 release，无历史 internal-pending，pending registry 为零
- 人工核对日志与实际保留目录相符；归档/源/恢复目标都存在，未混入旧 receipt 或另一二进制

纯内存 contract_test.go 有缺项/重复/旧身份/错误布局/序列/DBID/错误 old-mode 判定/归档或 immutable 改动/未知或重复退休等反例。testscope 新 AST 源码反例刻意绕开 hash 检查，仅测试结构和闭包拒绝，包含 gate 顺序、宽 selector、漏页、verbose 遗漏/false、owner capture 错对象/错 error/非邻接、helper 借用、删除、alias、unknown retry、吞 EOF 错误等。关键 ledger 调用与 row/vector/index 条件绑定实际 AST call/argument 和函数位置；将条件替换为 true、把 ledger 调用改成 no-op、仅在注释里保留原文字都必须被拒绝。当前反例也全部 NOT_RUN，不能称已验证。

只有用户在保存证据、确认没有 pending/unknown owner 或独立处理完成后，才可另行明确清理本次随机子目录。harness 不提供自动删除命令，不删除 known root，不在失败后自动重跑。

本项即使日后通过，也只关闭这个正常升级与 fresh-snapshot-restore 有限格子。它不关闭崩溃/掉电/Sync/ENOSPC/短写/篡改/错误密钥/重启重放/长期 soak/跨平台原生/旧二进制兼容/生产加密安全或全部格式矩阵。

## V2 源码更正记录

V1 独立审阅为 SOURCE_REVIEW_BLOCKED；V1 源码/证据/HOLD 全部原样保留。V2 更正 capture 的真实六参数/error 槽 5、实际 retained/.base 名称、受支持 UNIQUE scan、pending 内部 owner 的 parent-close 屏障、强制 verbose、真实 AST 断言绑定、完整 owner/provider 清单及目录/manifest provenance。源清单计数分为 798 个 .go 文件与 800 个 manifest entries（另含 go.mod/go.sum）。V2 仍 SOURCE_ONLY / REVIEW_HOLD / NOT_RUN，等待新的完整独立源审。
