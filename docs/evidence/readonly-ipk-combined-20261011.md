# RO-WAL / IPK 组合开发检查点：精确证据与资格分层

日期：2026-10-11 UTC。本文件是独立文档覆盖层，未写入执行冻结树。
完整 26 项已于 **22:05:15 UTC 自然 PASS**；所有阶段 exit 0，lease RELEASED。
这是有限开发检查点，不是原始 goal、完整 SQLite 兼容或生产验收。

## 1. 固定输入与身份

- 已保存基点：commit `1e8e7fbb694b9234b7797414ae7422a38e08ab7e`，tree
  `4e82f2859975a1d8df11b692f732ed29f6a83e6d`。
- 只读 WAL V2：tree `b19ee07536c83d554f70b40d461b0b96ee4e2872`。
  有限 14 门的 23 根 / 27 子项共 50 PASS 属于这个输入树。
- IPK V4：tree `96e039f2a0b0990789579bfa5fe20746b6b1ecdb`。
  其 baseline expected-RED 22 PASS / 29 FAIL、修正版普通 55 PASS 与
  原第三条官方 oracle 25 PASS 各自保留，不改写为组合结果。
- 原组合：tree `9ca02ee7fee55f0eb4abe83330b37b80d02aa4bd`，3,177 文件。
- 本次执行冻结树：`cace45621c10259231a2e764cfbefdcdbe312ccf`，3,178 文件，
  目录名 `readonly-ipk-combined-guard-source-v2`。
- 只读 live-writer 源码：`internal/testkit/readonly_wal_live_conformance_test.go`，
  8,738 字节，SHA-256
  `b36e8739e69e67f77ed539d2a9e768de92ac9f5675a5ee3d5a557b112b455006`。
- 原始 `goal.md` 保持 43,100 字节，SHA-256
  `9ae7b1d776856669a10eb753408c7754d015d266f8783240165450e96e4bac81`。

只读 V1/V2、IPK V1–V4 和 live-writer V1–V4 的作者报告及独立审阅中的
SOURCE_ONLY / NOT_RUN 是当时状态；不修改原报告，也不借新树通过追认旧树。
当前新结果以实际阶段 JSON / 原始输出为依据，不能只凭作者报告标题判断。

### 已保存父基点的 hosted CI，单独闭合

已恢复并核对 `darwin-ci-terminal-evidence-v1/run.json` 与 `jobs.json`，
两文件及原 tool-response 均匹配其封存 SHA256SUMS。
父 commit `1e8e7fbb694b9234b7797414ae7422a38e08ab7e` / tree
`4e82f2859975a1d8df11b692f732ed29f6a83e6d` 的
[CI 38160811706](https://github.com/ms-xiaosnow/gosqlite/actions/runs/38160811706)
为 run 47 / attempt 1，真实 SUCCESS，run 更新于 18:02:03 UTC：
ordinary job `114533143796` 为 17:43:11–18:02:02，
oracle job `114533143863` 为 17:43:11–17:52:04，两者 completed/success。
这只闭合父基点，不能作为未发布 cace4562 的 hosted CI 或当前远端新提交证明。
本任务只读历史 receipt，没有新联网查询、触发 CI 或改写环境替换原报告。

run.json：12,194 字节，SHA-256
`97fe9f693e6a8e8d875cb3e3bbf9e2857be602131ca6bc466d6ff61a2bd049fd`；
jobs.json：4,285 字节，SHA-256
`8a002f5f3a2d5fd8bcc9782c52af21e5c5ae9d71e39ff036094b18bfe8117030`。

## 2. 环境替换与原组合 FAIL 均保留

[环境替换记录](../history/ENVIRONMENT-REPLACEMENT-20261011.md)按原字节复制，
6,530 字节，SHA-256
`45c0b2f8efd2df1f586a9dad0b1a6b12f111593e40d4f93604e37c355237979c`。
20:38 启动的旧会话 54102 因 executor key 改变而不可恢复，终态仍 **UNKNOWN**；
不能声称自然退出、释放或通过。新环境源码恢复等同原 9ca02ee7，不等于恢复
旧执行结果、inode、ACL、所有权、时间戳或全部 POSIX 身份。

新环境的原组合会话 18940 是另外一次执行。其 ordinary 于
21:13:51 UTC 自然 exit 1，5,711 PASS / 7 SKIP / 四根 FAIL；后续 19 项
未运行，已记录正常释放，无 kill/retry。原 `02_ordinary.stdout` 为
5,496,287 字节，SHA-256
`ad14fd909552bb7a4aeb6f71ae199d31bd7e06ad8262e6a0953846a626f176ea`。
四个原始失败根是：

1. `TestInspectSourceForwardsCallerContext`
2. `TestSharedAbsenceSourceInitialProvenance`
3. `TestSharedAbsenceSourceAdmissionAdoptionAndErrorExit`
4. `TestSecondaryRangeWorkOrderingPrecedenceAndBinding`

诊断为三项源码断言仍指向旧表达式/旧函数体，以及一项错误地要求
NOT INDEXED IPK 等值一定完整扫描。未据这四项失败建立生产缺陷，也没有
把“未发现生产缺陷”写成已通过。原 guard/live-writer 的通过仍只属于原树。

## 3. 四文件测试修正，不变的生产合同

`combined-test-repairs-independent-review-v1/REPORT.md` 与
`audit-combined-guard-failures-v1/DIAGNOSIS.md` 是本次独立源审与诊断来源。
源审只证明审阅范围，没有运行任何测试。相对原树只变更以下四个测试文件；
其余 3,174 文件保留字节和 Git mode，生产、factory、guard、scripts、依赖
与通用 `ordinaryCompare` 均未改变。

| 测试文件 | 原字节 / SHA-256 | 新字节 / SHA-256 |
|---|---|---|
| `cmd/gosqlite/inspect_context_test.go` | 7,523 / `9a1626e5a84a768e8ca919379c8a7ac35ac7c99f3ffaf581198a734ba16b3c21` | 10,502 / `45e9d1da01784e0882ff4ec92dec3f2f6086bad3d2d88605bd01e8349a4c1d7c` |
| `internal/pager/shared_absence_test.go` | 6,803 / `8d4c257d07113287bbe4c15879b8ea22d7425daaf1fb0b92033d0747bb7add82` | 14,928 / `703e7f953c090b6d68dba540c8fe17c3cbb8408b84a1535207536c272db44865` |
| `sqlite/plan_secondary_range_test.go` | 12,693 / `fb15a534e15dc7a211efd84df69c4ff18c1eee2a2ead9ca42fcd41f04e91a53c` | 14,839 / `454dd982e0f549dfc50963ac8c974ba92eccdd695e467707e42919b92de3b73f` |
| `sqlite/plan_secondary_range_oracle_test.go` | 不存在 | 6,596 / `a64aa8f94767bec829bd25b4291e54f1e26c59fac59482a49817cfa80c1ba7d5` |

inspect 绑定真实 caller ctx、已解析路径、ReadOnly、NoCreate 与显式只读 WAL
选项，并增加 12 个内存源码负控。两个 pager 根沿精确 wrapper 转入真实 helper，
保留 provenance、admission/adoption、关闭所有权和晚错误断言，增加 13 + 16
个内存负控。没有执行破坏性替身或新物理失败路径。

### 原 SQL、NOT INDEXED 与真扫描的区别

完整保留 `SELECT id FROM t WHERE k>88 AND id=1`。在原 1,800 行 fixture 中，
id=1 来自 i=901，k=1，因此 residual 拒绝，结果为空。精确对应
`SELECT id FROM t NOT INDEXED WHERE k>88 AND id=1` 也允许 IPK 定位；
NOT INDEXED 不禁止 rowid/IPK table-positioning seek。id=899 来自 i=1799，
k=89，是匹配对照，结果为 899。

两组普通/NOT INDEXED 形式分别要求 SEARCH/rowid、一候选/seek/fetch、
原 WHERE 复核和零 secondary-index work。各自的 `id+0` 形式必须是真正
SCAN，1,800 fetch、零 seek/index work，且结果相同。加零的值保持说明仅限
这些已知小整数，不推广为任意 affinity、极值或表达式重写规则。

新 tagged oracle 保持官方 SQLite 3.54.0 完整版本/source ID；原 SQL、精确
NOT INDEXED、真扫描的两组结果各一条，加两条 EXPLAIN 和一次身份查询，
固定九个正常官方进程。官方 typed rows 与预期/native rows 比较，plan 和
native work 独立断言。原 IPK 24-case oracle 不含这个精确 SQL/fixture，
保留为不同证据；不能由那 24 条记录推称此条原 SQL 已对照。

## 4. 本树完整 26 项实际终态

来源目录为 `combined-readonly-ipk-repaired-verification-v1`，会话 89612。
阶段 `.command.json` 保存原实际 argv / 环境，`.result.json`、原 stdout/stderr、
具名 RUN/terminal pairing、前后源码/工具/输入/资源身份分别保存。
原 21:45 UTC 的九阶段待定草稿已单独保留；下表仅据实际终态更新：

| 阶段 | 实际结果 | 自然结束 UTC |
|---|---|---|
| `00a_repaired_cli` | 1 根 + 12 子项，13 PASS，exit 0 | 21:35:49 |
| `00b_repaired_pager` | 2 根 + 29 子项，31 PASS，exit 0 | 21:35:58 |
| `00c_repaired_range` | 1 根 + 2 子项，3 PASS，exit 0 | 21:36:11 |
| `00d_repaired_ipk_oracle` | 1 根 + 8 子项，9 PASS，九个官方进程正常完成，exit 0 | 21:36:24 |
| `01_guard` | PASS，exit 0 | 21:36:33 |
| `01a_livewriter_readonly_oracle` | 1 根 + 6 子项，7 PASS，七个官方进程正常完成，exit 0 | 21:36:42 |
| `02_ordinary` | 5,758 PASS / 7 固定 oracle SKIP，exit 0 | 21:39:10 |
| `03_vet` | PASS，exit 0 | 21:39:26 |
| `04_build_nocgo` | PASS，exit 0 | 21:40:00 |
| `05_race` | 5,758 PASS / 7 固定 oracle SKIP，exit 0 | 21:53:22 |
| `06_compile_linux_amd64` | Linux/amd64 生产编译 PASS，exit 0 | 21:53:36 |
| `07_compile_linux_arm64` | Linux/arm64 生产编译 PASS，exit 0 | 21:54:09 |
| `08_compile_darwin_amd64` | Darwin/amd64 生产编译 PASS，exit 0 | 21:54:44 |
| `09_compile_darwin_arm64` | Darwin/arm64 生产编译 PASS，exit 0 | 21:55:20 |
| `10_compile_windows_amd64` | Windows/amd64 生产编译 PASS，exit 0 | 21:55:55 |
| `11_modules_verify` | 固定模块核验 PASS，exit 0 | 21:56:02 |
| `12_whole_module_nontest_closure` | 全模块非测试无 CGO 闭包 PASS，exit 0 | 21:56:11 |
| `13_shipping_cli_library_closure` | 交付 CLI/library 无 CGO 闭包 PASS，exit 0 | 21:56:18 |
| `14_complete_pinned_oracle_script` | 完整 pinned oracle 7,431 PASS / 零 SKIP，exit 0 | 22:03:20 |
| `15_limit_phase_conversion_oracle_race` | LIMIT oracle/race 394 PASS，exit 0 | 22:03:39 |
| `16_compile_darwin_amd64_encstore_tests_only` | Darwin/amd64 测试镜像仅编译 PASS，exit 0 | 22:04:00 |
| `17_inspect_darwin_amd64_encstore_symbols` | Darwin/amd64 符号核验 PASS，exit 0 | 22:04:08 |
| `18_compile_darwin_arm64_encstore_tests_only` | Darwin/arm64 测试镜像仅编译 PASS，exit 0 | 22:04:30 |
| `19_inspect_darwin_arm64_encstore_symbols` | Darwin/arm64 符号核验 PASS，exit 0 | 22:04:38 |
| `20_compile_windows_amd64_encstore_tests_only` | Windows/amd64 测试镜像仅编译 PASS，exit 0 | 22:05:01 |
| `21_inspect_windows_amd64_encstore_symbols` | Windows/amd64 符号核验 PASS，exit 0 | 22:05:08 |

普通门 5,765 条具名路径中有 41 个 Fuzz 根/313 固定种子与 6 个 Example；
没有启动持续 fuzz campaign。四个 quick 普通根合计 47 PASS，不能与完整
ordinary 同一批出现的路径相加宣传覆盖数。九进程 oracle 的 completion/原始
输出记录是源码归因证据；OS process sampling 仅是不完整下界，不当作完整
exec trace。live-writer 单根是七进程；完整套件可能共享 identity，不能机械
将每个根的数字相加。新 IPK oracle 的 identity 无缓存，每次根调用均九进程。

### 完整 26 项闭合

`completion.json` 于 22:05:15.919603 UTC 记录
`ALL_TWENTY_SIX_REPAIRED_NORMAL_PASS`，同树 26 个外层命令、28 次实际
Go 调用，未继承原树门。26 份 result.json 均为自然 exit 0，具名测试、
源码/输入/工具与资源后检通过。22:05:15.920742 UTC 的
`COMPILER-LEASE-RELEASE.json` 记录 RELEASED、全部外层子进程自然返回、
无仍活动的已观察子进程、无强制终止；没有 retry 或额外执行。

ordinary/race 各 1,278 Test 根、41 Fuzz 根、6 Example，313 固定种子；
完整 oracle 为 1,386 Test 根、41 Fuzz 根、6 Example，313 固定种子，
7,431 named PASS，零 SKIP。LIMIT 两根 oracle/race 为 394 named PASS。
全套中的新原 SQL oracle 仍独立记录九次正常官方完成；live-writer 的共享
identity 计数不被误加。外平台 test-compile/nm 的六项只是编译与符号门，
没有执行任何外平台测试镜像。完整普通门不提升物理/安全/原生资格。

completion.json：47036 字节，SHA-256
`db148e52fd4de9792f4181574b9280c081d35003af97ae8681d363d17e7d37ca`；
lease release：268 字节，SHA-256
`efeade207bca6ed2a079a5432b193ad7417eb0562d6eb7d50967151fe660d945`。
运行前 REPORT.md 的 SOURCE_ONLY 仍是历史作者说明；实际终态由上述
completion、逐阶段结果和 release 证明，不改写旧报告。
后续文档组合不继承 cace4562 的全树身份，必须另核未变运行字节。

精确新提交、持久恢复与 hosted CI 均需各自真实记录；本次没有推送、远端
写入、创建新 CI、执行手工模板或将别树 PASS 变成本树 PASS。

## 5. 新环境工具与持久对象

- Go 1.27.2 Linux/amd64 可执行文件 SHA-256 保持
  `548d3c32f83fa81152f543eb1526be7884cfe0526eb5e9ce29d3eca14786e7e0`。
- 官方 SQLite 源码包 14,881,448 字节，SHA3-256
  `a5c29342ca185abcda332448fbb27c536661245be27e20199b46c49099b0e15e`；
  生成 sqlite3.c SHA3-256
  `8efd453d08a7cfc4c79de5923153036cdc4dcaa254d4f1e617be6a2a98828f9e`。
- 新编译 SQLite CLI 为 8,708,688 字节，SHA-256
  `86169de25c972197f7346d2ecf1176ed449133c09e7f09df3cda19e330350ecd`。
  源码身份相同但二进制与旧环境不同；版本/source ID 一致不代表二进制相同。
- 精确版本/source ID：`3.54.0|2026-10-09 15:46:58 be8d059e9a49089ab2dce5ed26dd87aaf598fdc5fbe0b107c0dd758464bcd5e3`。
- 原工具引导 DNS/download 失败与其修正记录保留；不是数据库测试 PASS 或
  安全豁免。新 `/tmp` 为 tmpfs，普通 Sync 成功不证明断电持久性。

以下仅引用已有本地 identity/readback JSON 中已确认的 Library ID、长度与
哈希，没有新增上传或使用签名下载 URL：

| 内容 | Library ID | 字节 / SHA-256 |
|---|---|---|
| 恢复的原 9ca02ee7 源码 | `libfile_6e270b5803a88191a036503886487c5d` | 12,196,455 / `10e11be7528d7668daf6701fc974ee52b4d9e60996bb8135595396e250c6c66b` |
| 原四根失败终态 v2 | `libfile_263359ca574881918cec9eee8c68bfbc` | 10,131,535 / `6dcbfac27f74b7525bbf370b96389f0bcbbb912a3fc9638091b071e7d5af4f40` |
| cace4562 测试修正源审包 | `libfile_93faf3a624748191aed34f3e52cf847d` | 12,328,935 / `2e6edd035af7dd7a86f85a19eb683d7116fa93a31e25f992c096145b70b5e9fb` |
| cace4562 本次启动控制包 | `libfile_95471d7ad5a08191834796c1440dc4d7` | 2,310,342 / `d8135709e8c7cafbf93d516a3469ec5da5a0f15df50af21bcd22da114b0cdf9c` |
| 官方 SQLite/Go 依赖来源包 | `libfile_2daeff6610308191b6cedee4b404e60a` | 25,648,768 / `2a7bedf1b85cc4cec4dc4a53cd0ac13f7a8cbceab20fd6fee42616736f4b4bda` |
| 原环境替换说明 | `libfile_d330f98271e88191855a884a82bfbe9b` | 6,530 / `45c0b2f8efd2df1f586a9dad0b1a6b12f111593e40d4f93604e37c355237979c` |

源审包封存时的 runtime NOT_RUN 和启动包的 ongoing 状态保持历史含义；它们
不是未来终态归档。最终结果持久对象必须在实际保存/读回后另记。

## 6. 能力合同、剩余实现与受限链

Linux 显式 `ReadOnly + WALShared` 在活动且已初始化的合作 SHM 上，以
main/WAL/SHM 全部 O_RDONLY、PROT_READ mapping、已有 readmark 与完整
header 快照运行；不创建、重建、发布、写 readmark 或 checkpoint。
KeepWALOnClose 两值都不赋予写能力。名称/长度/内容无写入断言只在 peer
静止区间比较；并发合法 writer 区间只比较逻辑快照，不声称 atime/全部 metadata
或锁状态不变，也不抵御恶意 peer、同名替换或任意外部 mmap 截断。

冷/无 DMS holder、缺失/未初始化 SHM、immutable/heap-index fallback 与
其他 OS 仍 unsupported；真实只读媒体及非特权身份仍需真实环境，NOT_RUN。
本地普通 PASS 不赋予 Darwin/Windows 原生、文件系统持久性或错误注入资格。
native 外平台 test binary 即使编译/符号检查通过也不代表已执行。

有限 IPK 不扩展 CAST、一般函数、额外 rowid 比较、secondary/range admission、
LEFT JOIN pushdown 或完整错误阶段一致性。native WAL-tail publisher、strict
plaintext Backup 的另树源码/模型/手册与本树分开，不属于本次生产实现；
更广恢复、冷 WAL、SQL/格式、后端、物理 harness 仍有代码和集成缺口。

ANN V4、41 目标持续 fuzz、未知中断链不重发或换路。损坏/错误密钥、chmod、
namespace 替换、截断/短写、Sync/ENOSPC、kill/crash/强制关闭、hot-recovery、
中断发布/rekey、全部 manual 物理工作流继续保持原限制。已有手工源码与用户
模板不等于执行许可，更不等于运行 PASS；缺 harness 的项目也不虚构现成命令。
长 soak、固定环境性能、独立安全/密码学/法律审查、M3/M8/M10 和整体目标
尚未完整验收。参见[验收矩阵](../acceptance-status.md)、[只读合同](../plain-readonly-wal.md)
与[原目标](../../goal.md)。
