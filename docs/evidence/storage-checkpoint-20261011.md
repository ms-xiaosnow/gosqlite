# Storage 检查点：已取得证据与待补终态

记录日期：2026-10-11 UTC。本文是旁置文档草稿，只汇总已保存的源码/有限终态证据；
未改写冻结运行树或原始证据。Storage 2602 完整普通 V2 已在 ordinary 门 **FAIL**，
修正后新树及新完整验证 **PENDING**，不预填 PASS。本文及配套手册没有新增执行
命令、flags、harness 或执行许可。

## 1. 彼此独立的精确身份

| 结果线 | 固定身份 | 本页可确认的结果 |
|---|---|---|
| 已发布 main | commit `f189746671aaf4962a699292a84110e2d43387e8`；tree `0f29261ca50576373069dd8ad23a2bd3377a743c`；3,180 文件 | Library 精确恢复 26/26 外层门 PASS；28 次实际 Go 调用；原有 CI 38179686483 SUCCESS |
| Backup 有限输入 | tree `ec9f30d26d71085791197cd8e25abed558ad236d`；3,209 文件 | 36 条有限命令 PASS；44 根 / 2,278 named PASS |
| WAL 有限输入 | tree `3f772f7e18e59e3061b72ab8b37ee1953e55ddab`；3,227 文件 | 5 条有限命令 PASS；35 根 / 100 named PASS |
| 冻结 Storage 组合 | tree `2602cc60e8c82e3c7f900eb44439e4217cb0b8c3`；3,229 文件 / 753 Go 文件 | V2 六门 PASS；ordinary 8,126 PASS / 1 FAIL / 7 SKIP；后续 19 门 NOT_RUN |
| 最小 metadata 修正后源码 | 新树待固定，与 2602 分开 | 源码修正/独立审阅及新完整 26 门 **PENDING** |
| 文档叠加后的下一检查点 | 完整树、提交待固定 | 发布、精确恢复、对应 CI 均 **NOT_RUN** |

2602 与 3f77 的差异为 11 个文档路径；全部非文档字节、Git/POSIX 模式和目录模式
相同。相对已发布 f189，2602 有 65 个变更路径，其中 56 个 Go 路径。代码身份相同
不使两个完整树相同，也不把输入树 PASS 转授给新树。本次新增旁置文档同样会改变
未来完整树；不得把 2602 当作文档叠加后提交的自指 tree。

## 2. 已发布 f189 的独立闭合

恢复副本 completion 于 `2026-10-11T23:00:52.793337+00:00` 记录
`ALL_TWENTY_SIX_RESTORED_NORMAL_PASS`，lease 随后自然 RELEASED；session 97182
工具终态 exit 0。23:01:50 是工具观察时间，不当作实际进程退出时刻。
ordinary/race 各 5,758 PASS + 七个固定 oracle SKIP；完整 pinned oracle 7,431 PASS、
零命名 SKIP；LIMIT oracle/race 394 PASS。Darwin 两架构与 Windows 映像只有编译/
符号检查，未运行。上述数值只属于 f189，不作为 Storage 新树的预期终态。

同一提交的 [CI 38179686483](https://github.com/ms-xiaosnow/gosqlite/actions/runs/38179686483)
attempt 1 已完成 SUCCESS：ordinary job 114589410321、oracle job 114589410454 均
SUCCESS。非 JSON、非 verbose CI 输出不用于推算逐测试 PASS 数。既有 CI 收据中
“local restored runtime RUNNING”保留为较早观察，随后由新的恢复终态补充。

恢复终态归档 Library `libfile_cf83c1e1ff1481919dedcb52bde9ded4`，version 0，
82,631,098 字节，SHA-256
`1e3fc17842a40d88ee998659240b4dd3676d8639b8e80081366e5860d77aa58b`；
完整读回 4,690 个常规成员，逐成员字节/模式与封存清单一致。单独 CI 归档
Library `libfile_63383643d818819192124bfd9fea08cd`，version 0，14,149 字节，
SHA-256 `e6cf3da808625d1751e0bfaf5af04ffcdc88dc2e6d72d7049a3cca08ef7e1574`。
Library 保存/读回证明证据保存，不授予额外运行或物理资格。

## 3. Storage 有限输入真正覆盖了什么

Backup 的 36 条是六个 normal 范围、源码 guard 构建/扫描各一条、12 条平台/tag
选择、8 条仅编译、8 条符号读取。44 个根 / 2,278 个 named RUN/PASS 对含 2,234 个
子路径，零 SKIP/FAIL；其中 schema 的 103 接受和 1,880 拒绝案例共 1,983 子路径。
CLI 范围含两个普通加密文件 fixture 根；六个手动 plaintext Backup native 根、
12 次 Backup 及其七个官方 oracle 进程并未由这些有限命令执行。

WAL 的五个 normal 范围为 testscope、vfs、pager、sqlite、driver；35 根、65 子路径，
合计 100 named RUN/PASS 对，零 SKIP/FAIL。测试直接使用真实生产纯接口/状态转换与
源码约束，不是单独演示发布器；未启用公开 native publisher。旧 34 根 / 99 PASS
仍属于旧源码身份，不能替换新的 35 根 / 100 结果。

两个完整源树、原始日志/记录器、有限产物、独立核对与原始失败封存在
`gosqlite-storage-finite-wal-backup-3f772f7e-ec9f30d2-terminal-v2.tar.gz`：
Library `libfile_fba20f13283c8191ba51d6ac50eb4a63`，version 0，125,540,290 字节，
SHA-256 `a602a942d067eca127b71e432f28990b3c013ef1ca864918943244ef20d2a6a8`；
读回 8,525 成员 / 8,184 常规文件，所有路径、类型、字节、SHA-256、模式一致。
v1 归档器因自身目录 mtime 检查中止的原始产物保留，不作为有效交付或测试失败。

## 4. 组合 V1 记录器失败、V2 真实源码断言失败与下一步

V1 session 94153 只执行 guard：guard 子进程自然 exit 0，stdout 138 字节、stderr
为空；严格记录器仍期待旧 display line，故记录 **FAIL_TEST_VALIDATION**，后续
25 门 **NOT_RUN**。源码/输入/资源复核通过，lease 自然释放。不能把 guard exit 0
改称“完整 26 通过”，也不能把记录器失败描述为生产数据库断言失败。

V1 失败原包已保存到 Library `libfile_4bd42701c4008191961a3c37da9a1b3c`，version 0，
10,775,225 字节，SHA-256
`6469c6de38416126d7a9294a4c06df328d419fb3a6b5b0917703de272c65afcd`；145 成员
（137 常规文件）完整读回。原失败、guard exit 0 与 25 NOT_RUN 均不追改。

V2 只修正记录器与实际冻结源码一致的精确 guard 文本契约，并重绑新的证据目录与
会话身份；项目源码、全部 26 门、四个 quick repeats、选择器、环境、完整预期集合
及严格检查体不变。独立审阅完成后 guard-first 重新运行，不继承 V1 或输入树 PASS。

V2 的 guard、四 quick repeats、livewriter 共六门 PASS。ordinary 于
`2026-10-11T23:24:21.249485+00:00` 自然 exit 1，实际 8,126 named PASS、1 FAIL、
七个预期 oracle SKIP；后续 19 门未运行。session 85683 后续自然结束，lease 释放。
唯一失败根 `TestMetadataOwnershipCallChainSource` 的原始日志报告：

- `backup_namespace_linux.go:75:16` 直接 `os.Stat`，绕过 platform-owned metadata。
- `wal_publication_linux.go:108:16` 存在同一问题。

该 old guard 的真实源码断言确实失败，不能像 V1 那样归因于显示文本记录器错误。
虽然 V2 记录器同样使用 FAIL_TEST_VALIDATION 状态，它的 test_status 明确为 FAIL，
Go 也真实 exit 1；两次失败都原样保留。源码/输入身份与资源复核通过，不消除失败。

下一步是把这两个直接调用最小转接到已有平台所有权 metadata helper，保持相同
实参、错误与所有权语义；在独立新树中审阅并重跑精确回归和新完整 26 门。本草稿
没有实施或认证该修正，也不放宽旧 guard/更新期望去掩盖它。未来状态保持待补：

- Storage 2602 V2：**FAIL / 后续 19 NOT_RUN**，记录已闭合且不改写
- 最小修正后新树、独立审阅与新完整 26 门：**PENDING**
- 文档叠加后完整树与新 commit：**NOT_RUN / 待固定**
- 新提交的精确 Library 恢复与独立运行：**NOT_RUN**
- 新 Storage 提交的 hosted CI：**NOT_RUN**
- 手动 native Backup / WAL-tail publisher、物理存储资格：**NOT_RUN**

补记必须另引实际 completion、自然退出、全部命名生命周期、身份/资源复核、归档
读回及精确 CI commit；不能只凭零退出、局部 PASS、编译成功或旧树结果预填。

## 5. 当前合同与尚未实现/验收的边界

- Backup 只支持合同内的普通明文 rollback 快照；DB 的活动 Tx/Rows 和读写 Tx
  拒绝，只读 Tx 保持其钉住快照。最终文件路径/CLI --out 语义不变，目标父目录
  预先存在。源和目标 family 受限 ASCII 名、单链接、物理身份与唯一 mount ID、
  retained-directory/Openat2 准入，私有候选先锁再写；file Sync 后 no-replace，
  目录 Sync/退休后才释放主保护者。不是四个名称的原子保留或隐藏别名发现器。
- WAL-tail publication 仅显式非 nil policy 的 Linux 可写明文 WALExclusive；
  完整物理/逻辑 header、extent、页覆盖及相同错误对象 provenance 先于发布。
  同一 retained dirfd 单次受检替换旧 WAL，与 Backup 的 no-replace 协议不同。
  nil 默认拒绝、只读/shared/encrypted/recovery/PSOW 不兼容保持；无新 DSN/CLI/env。
  不确定发布保持 ErrCommitUncertain，清理只退役已有责任，不重放发布或恢复。
- ext-family magic 只是准入信号。实际 unique mount-ID、Openat2、可信合作命名空间、
  正确锁/设备/cache/fsync 前提缺一不可；不能靠 tmpfs/overlay、volatile fsync、
  0700、交叉编译或更换环境证明原生资格。更广 FS/架构/挂载/存储支持需要工程与
  验证；Backup 的 WAL/只读 WAL/非 Linux 支持仍是真实缺口。
- 现有 Linux 真只读共享 WAL 使用活动且已初始化 SHM，是独立有限能力；冷 WAL、
  真只读媒体/非特权身份与其他平台尚未取得对应资格。更广 rollback/public
  recovery、尾部形式及恢复/格式升级矩阵仍需分别补足，不是只剩手工测试。
- Backup 六入口和 WAL 四公开场景已有手工源码，均未取得 native 运行资格；这些
  有限 harness 也不覆盖完整 short write/Sync/ENOSPC、未知 owner、namespace
  干扰、跨进程锁、crash/power-loss/restart、checkpoint/rekey 中断与持久性矩阵。
  完整物理 fault/crash/recovery harness：**IMPLEMENTATION_MISSING**。

完整 ordinary/race/oracle 的受审范围包含已有正常 native 文件/锁/数据库/WAL、
正确密钥 BackupEncrypted/RestoreEncrypted/Rekey、健康 oracle child 与固定 fuzz
seed；也含内存 wrong-key/tamper/recovery 模型。它不是 memory-only。它排除新
native Backup/publisher、manual recovery/namespace、外平台映像运行和真实物理
故障/错误密钥落盘/篡改/中断链；也不是全平台或物理耐久性证明。上述是范围说明，
V2 的 race/完整 oracle 等后续 19 门实际 NOT_RUN，不能从范围推算执行结果。

ANN V4、41-target 持续 fuzz、未知中断链限制不变，原会话 54102 仍 UNKNOWN。
原始 9ca 普通 FAIL、旧归档/记录器失败、历史 NOT_RUN 与 POSIX 模式差异保留各自
身份。G1、M3/M8/M10、完整 SQLite 兼容、安全/密码学/许可、稳定 API 和生产可用
均不能由本补记宣布完成；[原目标](../../goal.md)的验收标准保持不变。

## 6. 来源与继续填写规则

精确来源为已保存的 `f189-restored-normal-verification-v1/completion.json`（小写），
`f189-restored-terminal-archive-v1/TERMINAL-REPORT.zh-CN.md`、
`f189-ci-terminal-verified.json`、`f189-restored-terminal-library-verified.json`、
`f189-ci-library-verified.json`、`storage-finite-terminal-archive-v1/REPORT.zh-CN.md`、
`storage-finite-terminal-library-verified.json`、`storage-v1-tool-terminal-observation.json`、
`storage-v1-failure-library-verified.json`、V1 `command-statuses.json`；V2 的
`command-statuses.json`、`02_ordinary.result.json`、`02_ordinary.natural-exit.json`、
原始 `02_ordinary.stdout`、`STOP.json`、`stop-final-checks.json`；以及组合源码/
范围审阅和 V2 精确 guard 更正/绑定审阅。这些是外部封存证据文件名，未伪造为本
仓库中不存在的相对链接；本旁置草稿清单记录实际读取文件的完整 SHA-256。

已有[Backup 手册](../testing/plaintext-backup-manual.md)、
[WAL 手册](../testing/wal-tail-publication-manual.md)和
[中文总手册](../manual-verification-zh.md)的命令完全未改。相对链接仅按文本路径
核对存在，不访问网络、不执行 Markdown 中任何命令。下一份终态补记应追加新事实
与自己的精确身份，保留本次失败和后续 PENDING 的当时状态，不改写原始运行记录。
