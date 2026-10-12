# Storage + UNIQUE 源码检查点与证据边界

## 2026-10-12 完整 26 门终态：精确冻结组合 PASS

状态：**ALL_TWENTY_SIX_STORAGE_NORMAL_PASS**。本段于 2026-10-12T00:53:38.044504+00:00 静态读取实际终态。
运行对象为 `4008bd9442ffd21e1e993d4f1c72dbeed3c910f3`，**3,238 文件 / 759 Go**；
不是本次文档叠加后另行计算的新完整树。新检查点保持所有代码、测试、脚本、CI 字节/
模式不变，仅更新本批十一篇 README/docs；不把执行身份伪装成自指提交身份。

全部 **26 个外层命令 / 28 次实际 Go 调用** 自然 exit 0，终态写于
`2026-10-12T00:49:41.920593+00:00`；lease 于
`2026-10-12T00:49:41.921674+00:00` **RELEASED**，无强制终止、无仍活跃的已观察子进程。
工具层会话 `47763`、chunk `b77f06` 在 `00:49:55.247749 UTC` 观察到自然 exit 0。

| 已运行门 | 实际终态与边界 |
|---|---|
| 完整 ordinary | 8,231 named PASS / 7 个固定预期 oracle SKIP / 零 FAIL；00:23:21.201367 UTC 自然 exit 0 |
| 完整 race | 8,231 named PASS / 7 个固定预期 oracle SKIP / 零 FAIL；00:36:50.011911 UTC 自然 exit 0 |
| 完整 pinned oracle 脚本 | 9,925 named PASS / 零 named SKIP/FAIL；00:47:12.618185 UTC 自然 exit 0 |
| LIMIT oracle/race | 394 named PASS / 零 SKIP/FAIL；00:47:42.522397 UTC 自然 exit 0 |
| 四条先行 quick | CLI 13、pager 31、range 3、原 SQL IPK oracle 9 named PASS，全部自然 exit 0 |
| 独立 live-writer readonly oracle | 7 named PASS，全部自然 exit 0；不等于冷 WAL 或新 publisher 资格 |
| Guard、vet、CGO0 build | 每门实际 PASS；现有手工标签/源码边界不因此移除 |
| 五目标 production build | Linux amd64/arm64、Darwin amd64/arm64、Windows amd64 各 PASS；没有执行外国目标 |
| Module verify 与两个 no-CGO 依赖闭包 | 三门 PASS；依赖输入重新哈希且 shipping 禁止依赖为空 |
| 三个 encstore test-c + 三个 nm | Darwin amd64/arm64、Windows amd64 的测试映像编译与完整符号检查 PASS；三个外国映像全部未执行 |

ordinary/race 各 50 包、1,377 Test roots、41 个 Fuzz 固定 seed roots、六个 Example；
完整 oracle 51 包、1,488 Test roots、41 Fuzz roots、六个 Example。各完整门 313 seed
PASS 是有限固定种子执行，不是持续 fuzz。全批累计 **26,844 named PASS / 14 SKIP**
是各参与门的重复路径累计，**不是 26,844 个独立测试**；包级无测试 SKIP 不混作 named SKIP。

共 5,502 次资源采样，**零已观察阈值违规**；不宣称未采样时刻或普遍资源上界。
最终读回确认 3,238 个源文件和全部 209 个原失败文件 unchanged；完整 toolchain
rehash 未再次重复，既有已核验终态工具/模块 manifests 仅复用为身份，未继承测试 PASS。
每门结果和最终 lease 均来自保存的原记录；未修改运行记录、失败、source pins 或手册。

### 可复核的终态证据身份

下列独立外部证据保存在本批验证/终态记录中，内容摘要来自实际文件：
- `completion.json`：SHA-256 `cd407844e89e457ee85d1646581d9889583268e8439811588f411186be4878ee`。
- `COMPILER-LEASE-RELEASE.json`：SHA-256 `9d524b75507b3ff32ccf9476d3708f4374eb5aed2b4a0ec73bcc9af9c287abf0`。
- `FINAL-READBACK.json`：SHA-256 `668b05af901b5617d13e49d9c54b3ad6aa33f8409d16f65d920077600837b3e4`。
- `storage-unique-tool-terminal-observation.json`：SHA-256 `b04c342e9377bfb0a4f25bd8ea8b9add2ad711c9e61ddad923185bd6a39b2a6d`。
- `storage-unique-full26-terminal-summary.json`：SHA-256 `6de462db0bf41641219e53920803e9407d68a99f5005a2035348f94ac5ae93e5`。

完整记录归档/新 checkpoint bundle、远端 main 提交、新提交的精确恢复及对应 hosted
CI 当前 **PENDING**；尚无新的已发布身份，不预写提交 SHA、Library ID 或未来 PASS。
本轮仅静态读取和复制文档，不重新执行 Go、编译器、测试、oracle、native 或 Git。

### 已核验的源码归档身份，不是执行验收

组合源码归档已在 `2026-10-12T00:13:17.545610+00:00` 完成压缩字节及全部成员读回：
Library `libfile_83ab1d6c7a78819196c9605d26f88c8f`，version 0，14,514,878 字节，
SHA-256 `2fa38713de9101f8a18f596d1fbf425f2dd23a3b5d72f706ea1e08edb7158c47`；3,430 成员 / 3,278 常规文件。
其中项目源码为上述 3,238 文件；归档文件数包含外部静态审阅/保存证据，不能混算。
其保存时的 full_combination NOT_RUN 是历史快照，不覆盖之后实际完成的 26 门 PASS。
下文原准备记录中的“尚无 Library 身份”和“全部 NOT_RUN”按该阶段日期原样保留。

### 当前支持范围与未完成工作

本组合已包含严格 Linux/ext-family rollback Backup、显式 exclusive-WAL tail
publisher 和普通命名列 BINARY ASC 显式 UNIQUE。UNIQUE 输入十二有限门
**223 named PASS**（normal 106 / race 96 / oracle 21），Storage 7389 三快速门
**92 named PASS**，均只证明自己的输入选择。原 2602 V1 显示契约 FAIL、V2
metadata 真失败 **8,126 PASS / 1 FAIL / 7 SKIP / 后续 19 NOT_RUN** 原样保留。

native Backup（包括 UNIQUE 数据库）、WAL publisher、物理故障/持久性/恢复、
Darwin/Windows 原生资格仍 **NOT_RUN / NOT_QUALIFIED**。广泛故障恢复 harness、
WAL/readonly-WAL 与非 Linux Backup、更广 FS、冷/未初始化 SHM 只读 WAL、其余
SQL/约束仍有真实代码缺口，不能概括成仅剩手工执行。表/列 UNIQUE 与自动索引、
partial/expression/DESC/非 BINARY、typed-vector/扩展 UNIQUE、conflict policies/
UPSERT 均不支持；SELECT planner 继续跳过 Unique。原协议、手动 tags/env 与限制不变。
待合入 LEFT JOIN、Darwin manual 与 inspect 切片不属于本批。ANN V4、41-target
持续 fuzz、未知中断链、安全审查、长期压力及 M3/M8/M10 未验收均不因本批解除。

已发布 main 仍为 f189；其精确恢复 26 门与 CI 38179686483 SUCCESS 只绑定旧身份，
不得转移为本树或未来文档提交的结果。以下保存的所有有限 Library ID 仍按原身份有效。

## 历史源码组合准备记录（2026-10-12 00:08 UTC；原文保留）

记录日期：2026-10-12 UTC。当前状态 **IN_PROGRESS / SOURCE_ONLY / NOT_RUN**。
本页记录已经组成的新源码检查点，不把任何输入树的 PASS 转授给新树。
本次仅以静态文件复制、文档 insertion hunk 合并、Python 标准库哈希和 Git 对象
序列化完成组合；没有执行 Go、gofmt、编译器、测试、oracle、native、Git、网络、
项目脚本或 Library 操作，没有创建提交或修改远端。

## 1. 精确输入与当前组合

- 已发布基线：[f189746671aaf4962a699292a84110e2d43387e8](https://github.com/ms-xiaosnow/gosqlite/commit/f189746671aaf4962a699292a84110e2d43387e8)，tree `0f29261ca50576373069dd8ad23a2bd3377a743c`，3,180 文件。
- Storage donor：tree `7389e7b746b6cc977cac9b47ec438aadc82f7eb9`，3,229 文件；只取已审阅的 65 个变更路径（56 Go / 九文档）。
- UNIQUE donor：tree `fd4e3471361718c7fff5626bbd98a5c049585814`，3,187 文件；只取已审阅的 23 个变更路径（16 Go / 七文档）。
- 两个切片没有非文档路径冲突；72 个变更 Go 文件保持各自 donor 完整字节和 Git/POSIX 模式。只有 compatibility、progress、restriction-provenance 三篇文档重叠，按已审阅 hunks 合并，全部历史段落保留。
- 核心组合为 3,236 文件 / 759 Go。另加入原 Storage 草稿的八个精确标题后插入 hunks、新历史证据页和本页，最终为 **3,238 文件 / 759 Go**；相对 f189 **88 个变更路径：72 Go / 16 文档**。
- 完整组合 tree 必须在所有文档写完之后计算，记录于配套外部 `SOURCE-IDENTITY.json` 和 `SOURCE-MANIFEST.json`。本页不嵌入自己的 tree 以免伪造自指身份。当前组合没有 commit，publication、精确恢复和 hosted CI 均 **NOT_RUN**。
- Darwin manual 修正和 LEFT JOIN / WHERE 独立切片不属于本批。已有原基线相关源码保留；不将那两条新候选混入。

## 2. 已实际闭合的有限输入门

Storage 的最小 metadata 转接修正保持两处调用的原实参与所有权语义，通过既有
`statParentPath` helper 代替绕过平台 metadata 的 `os.Stat`；只更新已审阅对应源码
pin。输入 7389 的结果于 `2026-10-11T23:59:47.008446+00:00` 记录
**ALL_THREE_METADATA_REGRESSIONS_PASS**，三条 CGO0 命令各自然 exit 0，全部观察到
的子进程自然安静，lease 随后 RELEASED，没有强制终止：

1. `00m_metadata_exact`：`TestMetadataOwnershipCallChainSource`，一根 / 一个 named PASS。
2. `00n_metadata_vfs_models`：十一根 / 89 named PASS；真实生产纯接口/内存状态与源码边界模型。
3. `00o_metadata_pin_guards`：两个现有 WAL production/factory guards，两根 / 两个 named PASS。

合计 **14 根 / 92 named PASS / 零 FAIL / 零 SKIP**。精确 argv、固定环境、日志和
前后身份/资源复核在原三条 `.command.json` 与 completion 中；这里只读取既有结果。
独立 Storage 完整 26 门未重新运行，状态 **DEFERRED_TO_NEW_COMBINED_TREE**；这不
更改旧 V2 的 19 个 NOT_RUN，也不把未运行 gate 算作 PASS。

UNIQUE 输入 fd4e347 的十二条有限门于 `2026-10-11T23:44:44.700817+00:00` 记录
**ALL_TWELVE_UNIQUE_FINITE_PASS**，自然退出并释放 lease：31 个 distinct roots，
54 次根执行，**223 named PASS = normal 106 + race 96 + oracle 21**，零 SKIP/FAIL。
其中包含格式检查和有界单包 normal/race/oracle，不是完整组合普通套件；有限 race
选择未覆盖 parser roots。141 个官方进程结局是源码完整成功路径推导（134 个 exit 0、
七个预期逻辑 UNIQUE 拒绝 exit 1），不是逐 child 独立收据；不能伪造为物理故障测试。

## 3. 原失败保留，不用新有限 PASS 改写

- Storage 2602 V1 的 guard 子进程实际自然 exit 0，但记录器仍期待旧 display line，记录 **FAIL_TEST_VALIDATION**；后续 25 门 **NOT_RUN**。这是显示契约记录失败，不能改为完整成功。
- 2602 V2 的 guard、四 quick repeats、live-writer 共六门 PASS。完整 ordinary 于 `2026-10-11T23:24:21.249485+00:00` 自然 exit 1：**8,126 named PASS / 1 FAIL / 7 预期 oracle SKIP**；后续 **19 NOT_RUN**。
- V2 唯一失败 `TestMetadataOwnershipCallChainSource` 确实发现 `backup_namespace_linux.go:75:16` 与 `wal_publication_linux.go:108:16` 绕过 platform-owned metadata 的 `os.Stat`。它是真实源码断言失败，不能归因于 V1 显示记录问题。
- 7389 三条快速门证明这次有界修正的已运行范围；不能代替新组合完整验证。历史格式停止、原归档/记录器失败、原 9ca 普通 FAIL、原会话 54102 UNKNOWN 与其他未闭合中断链保持各自身份和限制。
- [上一份 Storage 记录](storage-checkpoint-20261011.md)及所有 donor 原始状态段落原样保留。其中“下一步修正 metadata”等 PENDING 表达是当时历史；本页以实际 7389 三条 PASS 更新现状，不删除旧证据。

## 4. 组合交互与尚未验证的 native 路径

UNIQUE 对 `sqlite/check.go` 的相邻键前缀检查通过 `equalUniquePrefixes` 扩展现有
`checkSnapshot` 语义，因此 Storage `sqlite/backup.go` 对 `checkSnapshot` 的调用现在
也包含 UNIQUE catalog/prefix 检查。Backup 文件字节保持不变，不代表其语义依赖不变。
默认 Backup tests 只验证 unsupported / closed / busy / canceled / nil-context 等
reader 与 destination 获取之前的拒绝，不到达 native snapshot publication。
**包含 UNIQUE 数据库的 native Backup 是 NOT_RUN；所有新 native Backup 和
WAL-tail publisher 资格仍 NOT_RUN。**没有据此增加物理测试或执行权限。

所有 22 个 Storage whole-source pins 原值保持且与组合实际字节匹配；其目标与
UNIQUE 改动路径完全不交叉。十个 manual-excluded 源文件、13 个手工 Test 声明保持
原字节/模式/标签。Backup 六入口仍要求 Linux + manual_backup_publication，oracle
入口额外要求 oracle；WAL 四场景公共入口仍要求 Linux + manual_wal_publication。
普通新 UNIQUE 连接不设置 WALTailPublication，特殊 publisher 仍须显式非 nil policy。
guard 读取全部 759 个 Go 源文件与原三份 shell 合同作源码数据，不执行 manual bodies。

Backup 仍限制普通明文 rollback 快照，WAL、encrypted、session 与不支持的平台在
快照/目标获取前拒绝。可信合作命名空间、受限 ASCII family、单链接、物理身份、
唯一 mount-ID、Openat2、retained dirfd、候选先锁再写、file Sync、no-replace、目录
Sync/退休仍是合同要求。WAL-tail 的单次受检替换与 Backup no-replace 不同；只读、
shared、encrypted、recovery、PSOW 拒绝和 ErrCommitUncertain 语义不变。
ext-family magic、0700、volatile fsync、tmpfs/overlay 或交叉编译都不能证明物理资格。

## 5. 已发布基线与保存证据

远端 f189 的已保存 ref/commit 读回与 tree 一致；本次未做新的远端读取或更改。
f189 精确 Library 恢复独立完成 26/26 门（28 Go 调用）：ordinary/race 各 5,758 PASS
+ 七个固定 oracle SKIP，完整 pinned oracle 7,431 PASS / 零 named SKIP。既有
[CI 38179686483](https://github.com/ms-xiaosnow/gosqlite/actions/runs/38179686483)
attempt 1 **SUCCESS**，ordinary job 114589410321 和 oracle job 114589410454 均成功。
该 CI 只属于 f189，非 JSON/非 verbose 日志不能推算逐测试 PASS 数；也不授予新组合资格。

以下 ID、version、长度和摘要逐项读取已有 `*library-verified.json`；本次没有上传
新组合，也不把归档保存/读回算作运行验收：

- 已发布 f189 精确 bundle：Library `libfile_29ce444c42948191aebf3cd9ff19b1be`，version 0，6,012,092 字节；SHA-256 `c6e6ce5f5764c9a2f07d9a96dc62f8f7babf8050ad8afff9dbd42f4710c7736f`。
- f189 精确恢复运行终态：Library `libfile_cf83c1e1ff1481919dedcb52bde9ded4`，version 0，82,631,098 字节；SHA-256 `1e3fc17842a40d88ee998659240b4dd3676d8639b8e80081366e5860d77aa58b`。
- 既有 CI 38179686483 终态：Library `libfile_63383643d818819192124bfd9fea08cd`，version 0，14,149 字节；SHA-256 `e6cf3da808625d1751e0bfaf5af04ffcdc88dc2e6d72d7049a3cca08ef7e1574`。
- Backup/WAL 两条有限输入终态：Library `libfile_fba20f13283c8191ba51d6ac50eb4a63`，version 0，125,540,290 字节；SHA-256 `a602a942d067eca127b71e432f28990b3c013ef1ca864918943244ef20d2a6a8`。
- 原 Storage V1 显示契约失败：Library `libfile_4bd42701c4008191961a3c37da9a1b3c`，version 0，10,775,225 字节；SHA-256 `6469c6de38416126d7a9294a4c06df328d419fb3a6b5b0917703de272c65afcd`。
- 7389 修正源码与保留失败证据：Library `libfile_39beca4612f0819191b1ed117c5321b0`，version 0，24,971,687 字节；SHA-256 `8cc458480efba3855662a5f845ec3e5d4235fe538836b589f95279ec5462dbee`。
- 7389 三条 metadata 快速回归终态：Library `libfile_b9b3d50b60f881919f36cc9ec6b86981`，version 0，11,283,108 字节；SHA-256 `a2e7a3a06fa7c0eedf29c28b3daadcb2019035ce72470bfe11818c228562712d`。
- fd4e3471 十二条有限门终态：Library `libfile_86f1a74c9b8c8191a3c8285ef178ba5a`，version 0，27,564,000 字节；SHA-256 `3a85d4dc68d85f5fa9d0e690ea2dbc960ae0c099364ba7b275fcea4630b36d13`。

新三条 Storage quick 的上述 Library 终态包仅证明其三条门 / 92 named PASS 的保存，
146 成员 / 142 常规文件已完整读回。新组合还没有 Library 身份，不预填保存结果。

## 6. 接下来必须绑定新完整树，不能扩大验收结论

当前新组合的 full ordinary、race、pinned oracle、oracle-race、vet/build、五目标
production cross-build、依赖与外国 encstore test-c/nm 均 **NOT_RUN**。下一步先独立
审阅最终树的真实选择、测试 roots/helpers、import/init 与 guard 读取闭包、oracle
factory、命令/资源/session 绑定，然后才可按批准的完整门顺序运行，不继承输入结果。
必须保留默认全套选择，不能用 package 子集、-run 或额外 manual tag 替代。

当前静态投影 normal/race 为 50 包 / 676 Go / 1,377 Test roots，oracle 为 51 包 /
725 Go / 1,488 Test roots，各保留 41 个普通 fuzz seed roots 与六个 Example。这些
是源选择/声明数，不是动态命名 PASS 总数，仍需在最终完整树独立复核。完整默认范围
包括既有健康 native 文件/锁/数据库/WAL、正常加密操作、ANN 算法测试、oracle child
和固定 fuzz seeds，不能描述成 memory-only 或完全不含 native。跨编译不执行外国
映像，production build 不证明所有 test files 编译；限定 encstore test-c 也不证明
UNIQUE 外平台测试覆盖。

真实 short-write / Sync / ENOSPC、namespace 干扰、未知 owner、跨进程锁、crash /
power-loss / restart / hot recovery、checkpoint/rekey 中断和完整物理 fault harness
仍是未实现或未验收缺口，不能说只剩手工运行。Backup 的 WAL / 只读 WAL / 非 Linux、
冷 WAL / 缺失或未初始化 SHM / 真只读媒体 / 非特权身份以及更广 FS/架构/恢复矩阵
也保留独立缺口。UNIQUE table/column 约束、implicit autoindexes、partial/expression/
DESC/non-BINARY、typed-vector keys、USING extensions、conflict policies / UPSERT
及其他 SQL 范围仍不支持；SELECT planner 对 UNIQUE 的跳过保持。

ANN V4、41-target 持续 fuzz、安全/密码学/许可评审、长期压力、稳定 API、完整 SQLite
兼容、G1 与 M3/M8/M10 均未由本检查点验收。[原目标](../../goal.md)保持不变；所有
手册命令与限制保持。组合成功仅指精确源码构成与静态保存，**不宣称生产可用或 M10 PASS**。
