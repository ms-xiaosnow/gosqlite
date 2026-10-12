# ADR 0017：显式独占 WAL 的已提交前缀发布

## 2026-10-12 Storage + UNIQUE：冻结组合完整 26 门 PASS

冻结运行树 `4008bd9442ffd21e1e993d4f1c72dbeed3c910f3`（3,238 文件）于 **00:49:41 UTC**
自然完成 **26/26 门、28 次 Go 调用 PASS**，lease 已 RELEASED。ordinary/race
各 **8,231 named PASS / 7 个预期 oracle SKIP**；完整 pinned oracle **9,925 PASS /
零 SKIP**，LIMIT oracle/race **394 PASS**。跨编译与外平台 test-c/nm 只证明编译/
符号，未执行外国映像。本次仅叠加文档；新提交、bundle、精确恢复及对应 CI 仍 **PENDING**。
显式普通 BINARY ASC UNIQUE 子集已纳入；严格 Linux Backup/WAL publisher 仍为
**PARTIAL / NATIVE_NOT_RUN**，包括 UNIQUE 数据库的 native Backup；M10 未验收。
输入有限结果、原失败、准确终态和真实代码缺口见[本批精确证据](../evidence/storage-unique-checkpoint-20261012.md)。

### 历史组合准备摘要（2026-10-12，完整运行前；原文保留）

> 2026-10-12 当前组合状态：Storage metadata 修正树 `7389e7b7` 的三条回归
> 已实际 PASS（14 根 / 92 named PASS），UNIQUE 树 `fd4e3471` 的十二条有限门
> 已实际 PASS（31 根 / 223 named PASS / 零 SKIP）；两者均自然退出并释放 lease。
> 本文所在新 Storage + UNIQUE 组合仍是 **SOURCE_ONLY / NOT_RUN**：完整 ordinary、
> race、oracle、跨编译/依赖、发布/精确恢复/对应 CI 尚未运行。下文旧的
> SOURCE_ONLY、PENDING、FAIL 与 NOT_RUN 均保留为各自身份的历史，不覆盖上述有限结果。
> 原 V1 显示契约 FAIL 与 V2 metadata 真失败（8,126 PASS / 1 FAIL / 7 SKIP；
> 后续 19 NOT_RUN）不改写。native Backup（包括 UNIQUE 数据库）、WAL publisher
> 和物理资格仍 **NOT_RUN**；完整目标与 M10 未验收。详见[新检查点证据](../evidence/storage-unique-checkpoint-20261012.md)。

本批 metadata 转接采用已有 statParentPath helper；发布策略、nil-default、
一次替换、错误/owner 退休合同不变。新增普通 UNIQUE 连接未启用 WALTailPublication，
普通 WAL 成功不计入手工 publisher 资格。

## 2026-10-11 当前协议与资格补记

此 ADR 的 Linux 显式 opt-in、nil-default 拒绝、完整已提交前缀校验、同一 retained
directory 单次受检替换、保护性目录 Sync 与 owner 退休合同保持不变。它仍是
**PARTIAL / NATIVE_NOT_RUN**；只读/shared/encrypted 尾部发布、非 Linux 与更广
尾部形式没有因此获得支持。普通 Linux 只读共享 WAL 是另一条已有有限实现，
不能把它误读为只读尾部修复；见[只读 WAL 契约](../plain-readonly-wal.md)。

WAL 输入树 `3f772f7e18e59e3061b72ab8b37ee1953e55ddab` 的 5 条有限命令、35 根 /
100 named PASS 已闭合。当前组合树 `2602cc60e8c82e3c7f900eb44439e4217cb0b8c3`
的完整普通 26 门 V2 在 ordinary 门 **FAIL**、后续 19 门 NOT_RUN，修正后新树/
完整复验 **PENDING**。公开 Open / OpenContext / driver Connector
[四场景手工源码](../testing/wal-tail-publication-manual.md)现已存在，实际运行仍
NOT_RUN；完整物理 fault/crash/recovery harness 仍 IMPLEMENTATION_MISSING。

下文首次实现时的“全部执行 NOT_RUN”“公开/driver harness 缺失”和下一步说明
原样保留其历史身份；不得继续用它们否认已有源码/有限结果，也不得以有限结果
宣布原生资格。精确证据和未知/失败记录见[Storage 补记](../evidence/storage-checkpoint-20261011.md)。

### 历史首次实现说明（原文保留）

日期：2026-10-11 UTC。状态：新候选 SOURCE_ONLY，独立审阅与全部执行 NOT_RUN。
本文件新写于 3ea6d3ab 基点，补齐现存 admission 注释引用；不复制旧 ADR 的
历史 PASS、冻结身份或验收结论。源码存在不证明原生或物理安全。

## 范围与启动

公开 Options.WALTailPublication 是 nil-default 指针策略，初版没有扩展参数。
纯验证和值快照在任何路径/owner 获取前完成。只有 Linux、可写明文、WALExclusive
且没有共享 WAL、加密、RollbackRecovery、powersafe overwrite 才可启用。
OpenSession 和 OpenSessionAdmitted 继续传 false，遇到相同尾部仍保留原拒绝。
显式入口 OpenSessionTailPublicationAdmitted 传 true；无 DSN/CLI/env 或 init 启用。
空策略结构不承诺不同分配的指针地址；driver 按值复制其配置。

## 字节准入先于任何发布能力

openWALLocked 只扫描并保存私有 pendingWALTail，保留 ScanPrefix 返回的完全相同
错误对象。仅 prefix.Allows(同一个非 nil 错误, TailUncommitted) 可继续；Join、
包装或 errors.Is 匹配不能授予权限。old-salt、无效/部分尾部、I/O、quota、取消
仍拒绝，错误原因保持。pending Snapshot 从未安装为成功 Session 的快照。

先以同一持锁主文件/旧 WAL 检查完整物理 mode-2 header、对齐 extent、逻辑已提交
page one、UTF-8/schema/reserved/autovacuum 等既有政策、三方 page size、一切主文件
之外的已提交页覆盖。未提交 page one/页不能满足这些条件。零 commit 以已验证主库
作为逻辑状态；缩小 commit 不恢复已删除页。PlanPrefixReplacement 先完成预算，
无论 committed end 是否 sector 对齐都在独立候选中替换，不写或截断旧权威 WAL。

## 所有权与锁序

同一原 main 的 SHARED/RESERVED/EXCLUSIVE 锁由原根持有。先预留现有 temporary
槽，再安装纯内存 WALPublication，之后才调用 Acquire。没有新增 registry 槽或
修改 generic nested → temporary → WAL → main 顺序。

- 替换前：temporary composite 持有 candidate → directory，根持有 original WAL → main。
- 已知替换后：内存 checked exchange 同时保留两文件；根持有 new WAL，composite
  持有 superseded WAL → directory，main 一直不释放。
- 成功前：superseded WAL 必须先于 directory 完成退休，之后才安装快照/appendSafe。
- 未完成 child 保留 directory、current WAL、main；未完成 directory 保留 current WAL、main。

锁序为 Session 私有/串行使用 → root mutex（只在角色交换）→ composite mutex →
main/original/candidate/directory 固定 File 锁序。native 方法不反向回调 registry、
Session 或应用；交换不获取 native 资源、不 Close、不 unlock、不做路径观察。
composite 完成后不保留 Session、DB、Tx、context、callback 或页缓存。

Close 的诊断与 RetirementComplete 分离。确认消费的错误仍使 Open 失败，但可以
继续有序退休；nil+未完成和未知 close 成为终态 owned tombstone，不裸重试。
当前 closeTemporary 已挂起时，constructor unwind 通过 childClosePending 跳过
立即重复 close。显式清理只处理已有责任，不能重扫、重建、unlink、Sync 或重试 rename。
内部 cleanup token 经有界净化后不暴露给根的独立 adopter；外来/cyclic/超预算错误
成为不释放保护者的终态屏障，安全 errno、标准 OS 错误的 Is/As 保持。

## Linux 与协作命名空间契约

初始仅接纳 ext-family magic；它不是 ext4 功能、可信设备/cache 或持久性资格证明。
必须实际返回 STATX_MNT_ID_UNIQUE 等所需字段；零值/旧 recyclable mount ID 不可替代。
device/inode 物理身份独立核对。主库、WAL、候选必须普通单链接、指针与物理身份
不同；同一 retained parent/mount。主名受限 ASCII、非 sidecar、最多 200 字节，
候选后缀有界。Openat2 必须 no-follow、beneath、no-symlinks、no-magiclinks、no-xdev。
缺 syscall/字段/profile 直接 Unsupported，无更弱 fallback。所有返回 fd 先进入
预存 lease 槽；attach 失败保留终态责任，不让裸 fd 失去 owner。

journal 在获取和发布前必须不存在。SHM 必须已证明不存在，或在同一目录明确验证为
普通单链接、与所有文件不同的旧 SHM；不读取/映射其内容。发布后再次验证它，再
最多一次相对 dirfd unlink，之后核对不存在。意外出现或重绑定不能改为路径删除。
所有 namespace 效果、身份核对和 directory Sync 使用同一保留目录。

目录句柄和 SQLite byte locks 不保留名称槽位。此协议要求全部参与者使用同一普通
物理家族、遵守锁，并且没有非协作 link/rename/unlink/retarget/remount、绕锁写入、
候选读写或把 sidecar 当其他数据库。前后观察只能发现部分违约，不能击败恶意
same-UID/特权 actors，不能发现全部别名，也不声称原子 inode compare-and-swap。

## 一次性发布及不确定结果

构造候选时及最终不可逆边界前检查取消。进入 candidate Sync 之后完成有限的安全
序列，不因新取消跳过必要 barrier。顺序为精确候选长度/源稳定检查、candidate Sync、
源/家族复查、消费一次 replacement permit、同一 dirfd 的 Renameat2(flags=0)。
这是覆盖已核对旧 WAL 的替换，不是 Backup 的 RENAME_NOREPLACE；不使用 exchange、
unlink-old、绝对路径 fallback、rename-back 或重试 rename。

syscall nil 明确记已知替换，随后进行内存角色交换。未知 namespace 结果不探测猜测
成功、不交换，仅保留两个旧表示的 owner。无论替换结果是否不确定，预定 protective
阶段仍最多尝试一次同目录 Sync；已知替换的 post identity/SHM 失败也不能跳过它。
barrier 成功不消除任何先前错误。替换尝试后的错误返回 ErrCommitUncertain，Open
失败；后续清理成功不能恢复 Session 或宣称发布成功。候选 artifact 不自动 unlink，
也不另找名字；退休描述符与删除名字是不同动作。

## 证据缺口

描述符无关测试直接调用生产 facade、prospective validation、orchestration 和 registry
adapter；没有单独演示性发布器。其测试定义、AST 负例及 testscope guard 均 NOT_RUN。
公开/driver 物理 harness 为 IMPLEMENTATION_MISSING；原生 profile、实际恢复、
跨进程锁、原生失败、崩溃/掉电/重启与物理持久性为 BLOCKED / NOT_RUN。
非 Linux、只读/shared/encrypted WAL 与更广尾部形式仍不支持。
本 ADR 不解除限制，也不宣布 M3/M10 完成；下一步是固定源码独立审阅。
