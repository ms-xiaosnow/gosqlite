# 原目标有限代码闭合清单：当前索引（2026-10-13 03:37 UTC）

R1–R6 六个有限设施项已闭合，固定范围不增加可选功能；这不是整个 goal 的完成证明。
本轮有界通读原目标确认的两个可操作差额已补入当前源码：goal §7.3 的精确 PG 风格
HNSW/IVFFlat DDL（normal/race 各 104 named PASS / 0 SKIP、source guard PASS），以及
§11.3 的离线性能比较器（独立源码审阅 CLEAR、57 纯 fixture PASS）。比较器只在既有
check.sh 加入 unittest，不新增 runner/性能 CI job，不提供真实固定基线或实测性能验收。
两项不扩大 R1–R6；分阶段 SQL 和在线/依赖完整 PG collector 不自动成为新清单义务。

本轮提交前已核验发布基点 `fd069d7a30b2e0ca8bdff67ca54c926aa118fc51` 的 CI `38339706432`
双 SUCCESS（oracle 03:09:22 UTC；ordinary 03:30:31 UTC）。上述新增源码的有限结果绑定
本轮证据清单内精确文件；发布及 CI 按新提交另行核验，不能转授 fd069 的资格。
见[本轮精确证据](../evidence/20261013-goal-closeout/README.zh.md)。

仍保留 ANN 报告三项代码/准入差额与整轮安全中断 UNKNOWN、ANN V4/41-target 持续
campaign 既有阻塞；不通过换入口或更多普通 PASS 消除。手工权威总表为 **17 组：
13 核心组、82 核心场景，另 4 原生平台组**，全部手工 **NOT_RUN**。真实 OS/FS、
PG/Neon 输入、受控性能/soak、独立安全/许可结论分别待取得。参见[有限剩余及手工总表](finite-acceptance-summary-20261012-zh.md)。
整体/M10 未验收，不能声称“所有编码完成，只剩运行命令”。

## 历史更新与原审计（以下事实绑定原时点）

旧 FAIL/HOLD/PENDING/UNKNOWN/NOT_RUN 不回写；下表“未写/下一文件/真正剩余”等
词保留当时审计含义，当前六项结论及实际手工资格以上述索引和权威总表为准。

> 2026-10-13：最后代码提交c5的配置内完整普通CI已双SUCCESS，详见[终态证据](../evidence/20261013-facility-ci/README.zh.md)。源码未再更改，本次文档记录复用c5结果。有限六项闭合不等于整体/M10通过。

> 2026-10-12 23:30:52 UTC：**有限R1–R6代码和受影响验证已闭合，整体未验收。** [当前六项准确证据和三类剩余](finite-acceptance-summary-20261012-zh.md)。受阻ANN报告修正仍是代码/报告缺口，真实物理和外部资格未通过。以下原审计表所有“未写/没有PASS/下一文件”单元格均为历史时点；当前状态以此更新和链接六项表为准。没有新加义务，也不以六项勾选折算整个项目完成率。

> 2026-10-12 23:04:54 UTC更新：**R1、R2、R5、R6代码和适用普通验证闭合；剩最后R3/R4共用设施批，另有受阻ANN修正。** [本批准确证据](../evidence/20261012-rekey-misplacement/README.zh.md)。R2/R5表格“缺失原因/下一文件”保留原审计历史，不是当前仍缺；实际物理手工均NOT_RUN。下列早期时间点不覆盖本更新。

> 2026-10-12 22:08 UTC更新：**R1与R6代码＋受影响普通验证已闭合，剩R2、R3、R4、R5四项（3组）及单列受阻ANN修正。** [准确新执行/复用证据](../evidence/20261012-publication-nonce/README.zh.md)。表中R1/R6已勾选；它们“缺失原因/下一文件”列保留原审计时点用于追溯，不是当前仍缺。没有新加验收维度，六项仅是有限设施清单，不是整个项目完成率。物理/外部资格仍逐项NOT_RUN。

> 最新结果：A8六格源片的八项有限验证已于21:47:51 UTC通过，原FAIL与修正完整保留；见[精确证据](../evidence/20261012-root-auth/README.zh.md)。下方独立审计保留其取证时点，R1–R6是真正有限编码收口项；实际物理六格仍NOT_RUN。

# 历史审计：原目标有限代码闭合清单（2026-10-12）

审计日期：2026-10-12 UTC。范围：原目标中此前列为剩余工作的持久化、安全、迁移、恢复计时及发布资格；不是重开全 SQL／全 OS 功能审计。

## 结论与计数

**本次确认 4 组可继续推进的必需代码缺口，共 6 个有限子项；另保留 1 组既知、受阻的 ANN 报告修正。** 最确定且最高优先级的是“崩溃中断 rekey”的有限调用／验证设施。A8 六格 root caller 已写，不应重做；其有限验证、提交仍按实际终态记账。

四组是：①剩余明确故障类别的断言／调用（errno、错位）；②checkpoint 中断与扩展索引恢复账本；③rekey 中断；④已有普通加密生命周期中的 nonce 唯一性关联断言。下面 R1–R6 是这四组的有限闭合点，不是四个大框架，更不是 OS × I/O × 六种 profile 的笛卡尔积。

**已完成代码 ≠ 当前源码全套新 PASS ≠ 物理资格。** 生产状态机配合 fake resource 的故障测试，以及真实 AEAD／生产 verifier 的内存负面测试，均是有效覆盖。没有依据要求把每条负面测试再复制成一个磁盘 caller。平台受限物理项目继续 NOT_RUN；本清单不执行、不迁移、不授权任何受限流程。

## 身份与证据口径

- `W`：本目录下 `gosqlite`。审计基线 HEAD 为 `6d668f80d199b2c49647a3cbc949442d125988ae`；工作树另有 A8 六格 source candidate。A8 后续集成验证在另一任务进行，本报告不预报结果。
- 原目标就是 `W/goal.md`，不是另一个待找文件：43,100 字节，SHA-256 `9ae7b1d776856669a10eb753408c7754d015d266f8783240165450e96e4bac81`，本次只读复核一致。
- 同时读了 `W/docs/testing/remaining-goal-boundary-20261012-zh.md` 与本目录 `remaining-facility-map.md`。两文是历史审计输入，不能凌驾于 goal 或较新的实际源码。
- **E1：既有普通覆盖。** `W/docs/evidence/20261012-prefix-error/a49-ordinary-115000599728.decoded.log`：a49 精确 CI 的 `scripts/check.sh`，ordinary 中 encryption:192、encstore:223、pager:225、vfs:235、migration/postgres:238、sqlite:239 行为实际包级 PASS，随后有独立 race 结果。`a49` 到 `6d` 的本报告所引 encryption／encstore／vfs、普通 encryption 测试、WAL elapsed 测试和 migration/postgres 文件无 Git 差异。本报告只复用这些不变覆盖，不把日志变成 6d/A8 新跑的全仓 PASS，也不把包级日志编造为逐个 named-test 日志。
- **E2：九格 rollback 增量。** `W/docs/evidence/20261012-postdb/README.zh.md`、`result.json`、`commands.json` 和原始 gzip 事件流：rollbackfault 模型复用 1,565 named PASS；testscope 新 1,110 PASS；两包 race 新 2,675 PASS；sourceguard、双 manual-tag compile/nm PASS，映像未执行。首次 guard FAIL 仍保留。
- **E3：A5 和 recovery 已有源码／编译。** `W/docs/evidence/20261012-rollback-facilities/{README.zh.md,new-run-result.json,reuse-map.json}`：五格 controller 及双 manual-tag 编译／符号检查已有；物理退出／恢复未运行。普通旧 FAIL 与后续精确复用分开。
- **E4：WAL elapsed 既有普通验证。** `W/docs/evidence/readonly-acceptance-checkpoint-20261012.md` 中 f8192e 的 ordinary／race／source／compile 真实 PASS；真实 publication／elapsed 仍 NOT_RUN。E1 后续未变普通覆盖可同时适用。
- **E5：M9 有界离线工具。** `W/docs/evidence/m9-transition-ordinary.md` 有精确 focused ordinary/race、vet/build、两类 golden 生成与比较结果；E1 包级结果为后续覆盖。均不是 PG／Neon 真实执行。
- 本次仅执行文件读取、有限文本检索、只读 Git 差异／身份和本报告写入。没有 Go、测试、compiler、fixture、数据库、native、network、CI 或生产源码修改。

## A. 已有代码，停止重复开发

表中的勾只表示该行有限代码合同已存在，不表示整个里程碑 VERIFIED。

| 勾选／状态 | 精确 goal 条款 | 实际源码／测试证据 | 已有适用 PASS 与未运行范围 | 该行停止条件／后续代码 |
|---|---|---|---|---|
| [x] CODE_DONE：rollback 九格 SQL caller | §4.1:159–164；M3:356；§11.1:373–374 | `internal/testkit/rollbackfault/sql_ledger.go` 的九种 `SQLCase`；`sqlite/rollback_fault_sql_manual_linux_test.go`；`internal/pager/rollback_fault_sql_manual_linux.go`。含 prefix nil／non-nil、body/header/publish-dir Sync-before、DB Sync-before、unlink-before、final-dir Sync-before。 | E2 普通／race／guard／compile；物理全部 NOT_RUN。最后一格是观察 NEW + uncertain，不是持久 ack。 | 不再补“第四格”或已完成的 pre/post-DB 五格；不把 fail-before 写成 OS Sync 已失败。无新增文件。 |
| [x] CODE_DONE：I/O、短写及 Sync 失败的基础覆盖 | §11.1:374“随机 I/O 错误、短写、Sync 失败” | `internal/pager/rollback_io_contract_test.go:TestRollbackWriteFullAtMemoryContract` 覆盖零／部分／异常计数及原错误优先；`internal/vfs/wal_publication_model_test.go:TestWALPublicationMemoryOneShotFailures` 通过实际 `WALPublication` 注入 read/write/candidate-sync/directory-sync 等返回失败；`backup_publication_model_test.go:TestBackupPublicationModelStateAndFailures` 通过实际 `BackupPublication` 验证 file-sync/directory-sync。 | E1；这是有效生产 transition 覆盖。真实硬件 Sync／掉电资格 NOT_RUN。 | 不新增“必须让本机 fsync syscall 真失败”的要求；剩余明确 errno、错位仅见 R1/R2。 |
| [x] CODE_DONE：A5 五格及一／三页 recovery caller | M3:356；§11.1:374 的进程退出和重启恢复 | `internal/testkit/rollbackrestart/contract{,_test}.go`、`internal/pager/rollback_restart_manual_linux.go`、`sqlite/rollback_restart_manual_linux_test.go`，五格含外部 durable ack 和独立恢复角色；`internal/pager/rollback_fault_recovery_manual_linux_test.go` 与 SQL 九格的三页独立副本 replay。 | E3 模型／源审／编译；实际退出、replay NOT_RUN。 | 不再称“没有 controller／只有接口”。该设施普通索引账本不冒充 grams/HNSW/IVF；后者见 R4。 |
| [x] CODE_DONE：公开有界 rollback 恢复入口 | §0:23；§4.1:159–164；M3:356 | `internal/pager/recovery.go:RecoverRollback`；`sqlite/rollback_recovery*` 与 `connector_rollback_recovery_manual_linux_test.go`；finite-count admission、public/OpenContext/Connector 已有有界 caller。 | E1 中普通 policy／source tests；物理恢复 NOT_RUN。 | 保持显式 opt-in、finite-only。cold journal、EOFUnproven、默认自动恢复不因本清单扩大。 |
| [x] CODE_DONE：AEAD 错 key/salt/字节/tag/版本负面 | §8.2:308–309；§8.3:316；§11.1:378 | `encryption/record_test.go:TestTamperEveryHeaderAndRecordByteInMemory` 逐字节与截断；`TestWrongKeyAndAuthenticatedSubstitution` 的盐／DBID／epoch；`encryption/format_test.go`、`key_test.go` 的格式／KDF／redaction；encstore root/manifest verifier negatives。 | E1；是实际 AEAD 和 verifier 逻辑覆盖，不能抹成“完全未测试”。磁盘负面不从中推断。 | 不重写 crypto framework；原始“不出现未认证 plaintext/reader”断言保留。 |
| [x] CODE_DONE：跨页与 main/WAL 绑定安全不变量 | §8.2:308；§8.3:316“跨页调换、主文件/WAL 不匹配” | `record_test.go:149` 检查 PageNumber／FrameOrdinal／role／FileID／DBID／epoch，调换密文并伪改公开 sequence 仍失败；`:214` role 子密钥隔离。`internal/encstore/wal_review_test.go:69` 调生产 `verifyRootBytes` 拒绝 wrong-parent/base/head/sequence；`:252` 调生产 `verifySegment` 拒绝调换 frame 和所有字节损坏；`shared_review_test.go:15` 验证 shared authority。 | E1；生产验证器+内存 bytes.Reader 的有效覆盖。**跨页／base-segment 替换的磁盘 public Open caller 目前没有，真实该场景 NOT_RUN。** | goal 未要求每条负面必须再次走磁盘。缺该 caller 是明确证据边界，不单独列 REQUIRED_CODE_MISSING，不因此创建六 profile 矩阵。 |
| [x] CODE_DONE：承诺的 A8 六格 root 磁盘 caller 源码 | §8.3:316 中 key/salt/ciphertext/tag/version 子集 | 已写 `internal/testkit/encryptedroot/{contract.go,contract_test.go}`、`sqlite/encrypted_root_negative_manual_linux_test.go`、`internal/testscope/encrypted_root_negative{,_test}.go`、中文手册；control/wrong-key/root-salt/metadata-ciphertext/metadata-tag/unknown-major；生产 `OpenContext` 使用 ReadOnly+NoCreate。 | 源码六格存在；首次集成失败与修正分开保存。当前有限新验证仍以父任务实收终态为准，不预填 PASS；物理 NOT_RUN。 | 完成本批已有 guard 集成、获准有限验证和 Git 保存即可闭合该片；不承诺它覆盖 main/WAL 替换、nonce 或 crash rekey。 |
| [x] CODE_DONE：普通加密生命周期及明文不泄漏 | §8.1:301；§8.2:310；§8.3:316 | `sqlite/encryption_test.go` 普通 rollback/rekey/backup/restore/import/export；`encryption_wal_test.go:16` password、取消、正常 reopen 后写入、目录文件扫描 sentinel、无原生 plaintext sidecar；`encryption_shared_test.go` 并发与 pinned head；`encryption_shared_rekey_test.go:17` latest-head rekey。 | E1 普通 Linux 路径已跑；不说“所有 native 都未运行”。fault/crash 路径另列。 | 不重做功能；nonce 的窄关联断言见 R6，中断 rekey 见 R5。 |
| [x] CODE_DONE：已声明 encrypted 升级边 | §3 原则7:153；§11.3:400；§12.3:452；§13.9:468 | `sqlite/encrypted_wal_review_test.go:259` 实际 1→2，检查 DBID、layout、sequence、mode mismatch 和 reopen；`sqlite/encryption_shared_test.go:238` 分别实际 1→3／2→3、数据、DBID、sequence、backup/restore/import/export 与旧模式拒绝；`sqlite/encryption_upgrade_manual_linux_test.go` 已有 1→2、旧 archive 与新目标 Restore，`:625` 明确新 DBID。 | E1 普通升级已跑；manual 格式／archive 验收 NOT_RUN。 | 不新增不存在的未来版本、原地 downgrade；不把“每条边 × 每种 archive × 每种故障”升级成必须编码矩阵。没有本次发现的确定缺格。 |
| [x] CODE_DONE：WAL 有界恢复／尾部发布耗时工具 | §11.3:396“WAL 恢复耗时” | `sqlite/wal_tail_publication_manual_linux_test.go:735–774`：紧邻三个 public entry 的 Now/Since；`wal_tail_elapsed_contract_test.go` 绑定 source/binary/hardware/Go/cache、main/WAL/ledger hash、bytes/pages/frames、owner，失败／refusal／普通 reopen 不计样本；`benchmarks/wal-tail-publication/README.md` 与既有手册。 | E1/E4 普通合同／guard/compile 已有；真实样本 NATIVE_NOT_RUN。 | CODE_DONE 仅为已支持 tail-publication 的总公开调用耗时合同，n=1、warm/unspecified。不能改称 recovery-only、cold-cache、P95；无需为每个 profile 新建计时器。真正测量／报告待外部资格。 |
| [x] CODE_DONE：M9 最低离线输入／转换／模板链 | §9.2:339–345；M9:362 | `migration/postgres` 的 CatalogSnapshot/Inspect、AST 转换、CSV/NDJSON 批事务续传、quarantine/数据与距离验证；`transition_render.go:34–44` 保留完整 supplied catalog/definitions/SQL corpus 的 evidence-only inventory；`transition.go:304–319` 缺依赖计划 blocked/manual-plan；`transition_sql.go:18–24` 的 preflight。 | E1/E5；真实 PG/Neon、用户 catalog/应用SQL/profile、人工计划复核未完成。 | 提供完整真实输入、生成绑定报告、人工复核并取得获准真实验证；不强制新增在线采集器、pgx 或 server executor。 |

## B. 历史审计当时剩余的 4 组／6 个有限代码子项（现已闭合）

这些子项只计实际缺失的断言／caller，不把已有模型或普通路径归零。新文件名若尚未存在，明确标为“建议新增”；实现可以采用同样有界的等价组织。

| 未勾选／组 | 精确 goal 条款 | 为什么是 REQUIRED_CODE_MISSING；已有证据仍算数 | 当前验证 | 有限闭合条件 | 下一源文件集合 |
|---|---|---|---|---|---|
| [x] R1／组①：errno 场景断言 CODE_DONE＋普通PASS | §11.1:374“磁盘满、权限变化” | 已有 `TestWALPublicationMemoryUnsafeBranchPreservesErrno` 的 ENOSPC 是 acquisition 错误图保留，现有只读媒体 EACCES 是静态访问准入；它们不是 publication 中 disk-full／permission-change 的语义断言。源码检索未见生产 publication transition 在具体 Write/Sync/rename 阶段收到 ENOSPC／permission 后的有限 state/result 控制。无需真实塞满盘或 chmod 才能补这一代码差。 | 相邻通用错误／Sync 模型 E1 PASS；新 errno 场景 NOT_RUN、断言未写。 | 使用**已有实际生产 facade 的 fake ops**，选择有限适用写入／发布阶段返回有身份的 ENOSPC、permission 错误；断言原 cause、是否可见／uncertain、无虚假成功、正确退休。不要复制所有 OS/profile；真实 OS errno 资格另列。 | 首选仅 `internal/vfs/wal_publication_model_test.go`、`backup_publication_model_test.go`（若确需模型字段则在同测试文件）；只在发现真实语义缺陷时改对应 production transition。 |
| [x] R2／组①：写错位故障调用及恢复判定 CODE_DONE＋有限PASS | M3:356 原文“断电/进程退出/写错位故障注入测试” | 现有 prefix-short-write 是正确 offset 上的前缀；ledger 的 wrong-offset 负例是拒绝伪造 receipt，不能声称真正模拟了写到错误位置。WAL/parser 损坏输入和 sector-neighbor 普通测试有效，但没有闭合这条“写错位”的调用／完整恢复账本。**goal 未单独要求每设备 torn-sector 实验**。 | 既有 prefix/source/model E1/E2；错位调用 NOT_RUN，caller 未写。 | 冻结一个有日志保护的明确适用写操作、目标与错误位置、完整 OLD/NEW 允许集及独立输入；source-gated 模拟／caller 真实连接既有写路径，拒绝混合行／索引或伪造 ack。source、有限模型、guard、compile 完成即可称该设施代码完成；物理资格仍 NOT_RUN。 | 在既有 `internal/testkit/rollbackfault/{contract,sql_ledger}{,_test}.go`、`internal/pager/rollback_io_manual_linux.go` 与 SQL manual caller/guard 中作一片有界增量；不得顺手扩为任意 offset／动作注册框架。若采用等价生产 fake-resource 模型，应明确实际被测 transition。 |
| [x] R3／组②：checkpoint 中断设施 CODE_DONE＋有限PASS | §11.1:374 明文“checkpoint 时崩溃”；§4.1:159；M3:356 | exclusive `internal/pager/checkpoint.go`、shared `shared_checkpoint.go`、encrypted `encstore/publish.go:Checkpoint` 都存在；普通 checkpoint/readmark/sector tests 有效。`WALPublication` 的尾部替换不是 checkpoint。尚无把持久化边界、已提交账本和重新打开结果连接起来的有限 checkpoint 中断 caller/controller。 | 普通 checkpoint E1；中断设施未写，物理 NOT_RUN。 | 至少覆盖实际支持的**不同 checkpoint authority**：plain exclusive 的 main/retire-WAL，shared 的 stable WAL/backfill，encrypted 的 immutable base/root。共用一个有界 receipt／调度结构、每种只选必要 authority cut，记录 durable committed head、完整恢复结果、owner；无须每个 I/O×profile。encrypted 可以与 R5 共享 root cut，不能将 rekey 密钥语义混为 checkpoint。 | 建议新增 `internal/testkit/checkpointrestart/{contract.go,contract_test.go}`、有界 `sqlite/checkpoint_restart_manual_linux_test.go` 及 guard/手册；对 `internal/pager/{checkpoint,shared_checkpoint}.go`／`internal/encstore/publish.go` 只增加确有必要的受门控观察 seam，保留 default 直通。 |
| [x] R4／组②：扩展索引崩溃恢复账本 CODE_DONE＋有限PASS | §5.1:188；§5.2:200；M4:357；§7.2:258、261；M7:360 | A5／SQL 九格 fixture 只有普通表与普通二级索引，`SQLCheckEvidence.ExtensionIndexes` 不把它变成扩展 fixture。grams 普通 DML/rollback/checkpoint、HNSW/IVF 持久化/reopen/reindex 测试已在；未见将有限中断及恢复结果与 grams posting、HNSW graph、IVF membership 的真实检查连接起来的 caller。 | 普通索引／扩展测试 E1；扩展中断 caller 未写，物理 NOT_RUN。 | 复用 A5 或 R3 的有限切点，固定最小真实扩展 fixture；恢复后行、posting／graph／membership、真实索引路径与 Check 一致，无错漏。不用再为三索引×六 profile 建 18 个 controller，也不运行 ANN V4 测量链。 | 首选在 `internal/testkit/rollbackrestart` 旁新增一个有限扩展账本文件及测试，受门控 public manual caller＋guard/手册；复用现有 grams/ANN/IVF 检查 API，不改生产索引算法，除非出现真实失败。 |
| [x] R5／组③：rekey 中断设施 CODE_DONE＋有限PASS | §8.1:301“防止中途失败产生半新半旧”；§8.3:316“崩溃中断 rekey”；M8:361“全量 crash/rekey 测试”；§11.1:378 | `sqlite/encryption_shared_rekey_test.go` 明确只做 intact successful rekey；snapshot review 也只成功 rekey。production `publishBase`／`publishRoot` 真实存在，rename-attempt 后 uncertain 的正确设计不是中断试验本身；没有该场景的 controller/caller。 | ordinary rekey E1；中断设施未写，物理 NOT_RUN。 | 对真正的公共 Rekey→base/root publication 选择有限 authority cuts；完整旧／新 key generation、可读 ledger、DBID 保留、nonce domain 新鲜、retained history 与 ownership 可核对。禁止将一个任意错误或任意可开库当 PASS。封闭 source/ledger/guard/manual compile 即代码终点；各物理场景仍 NOT_RUN。共享 root publication 可复用，不要求三 layout 重复同一切点；layout3 独有 maintenance 只验证其真实不同义务。 | 优先独立受审一片：`internal/encstore/publish.go` 附近的受门控最小 seam（default 不变）、建议新增 `internal/testkit/rekeyrestart/{contract.go,contract_test.go}`、public manual caller、testscope guard 与中文手册。具体切点由该片源审冻结，不在本清单扩写通用故障框架。 |
| [x] R6／组④：nonce 普通生命周期的窄关联断言 CODE_DONE＋普通PASS | §8.2:307“测试重复写入、重启、回滚和 rekey” | 重复写已由 `record_test.go:241` 的公开派生域＋nonce set 覆盖，失败 writer 不复用由 :280 覆盖；`encryption_review_test.go:20` 覆盖 rollback 不改 root、同 credential rekey 改 EpochID/RootSalt/root FileID；`encryption_wal_test.go:16` 确实正常关闭、重开并再次 INSERT。**缺的是将重开前后这些写入与 key-domain/nonce 唯一性直接关联的断言，不是缺普通 restart 功能。** | 对应普通流程 E1 PASS；缺失关联断言没有 PASS。 | 在已有普通场景观察只读公开 header/record identity，比较跨 reopen 的新候选与 retained 旧对象的符号 `(key-domain,nonce)`，rollback 不产生可复用 writer，rekey 新 epoch；失败／中断 attempt 的额外关联可在 R5 内完成。无需单建 nonce crash controller，不记录真实 key。 | 首选 `sqlite/encryption_review_test.go`、`encryption_wal_test.go`，加有界 test helper；若 needed，仅在 `encryption/record_test.go` 补“新 Master 实例／重开”的纯内存控制。生产 nonce 算法目前未发现必须重写。 |

### 不再凭推测添加的代码项

1. **replay 再中断**：`RecoverRollback` 在原页写前 Sync journal、之后写页/Truncate/Sync/retire，源码未发现专用 replay-interruption caller。goal 明确要求恢复正确与故障注入，但没有把“恢复过程中再次退出”单列一项。已有一／三页 recovery 与 R2/R3/R4 的恢复检查应保留；不仅因旧 A6 标签就强行新增独立控制器。如果 R3 或已支持恢复行为的具体失败证明需要它，再以那个具体失败立项，不能以泛泛“全覆盖”扩张。
2. **upgrade／backup 的每种物理中断**：原 goal 对 rekey 明文要求 crash；对普通 upgrade／backup 要求安全转换、验证与不丢源。已有 ordinary 升级、独立 Restore、实际 `BackupPublication` 的失败状态机不能作废。没有新具体差异或失败，不自动要求再做三套 interrupted-publication controller。
3. **cross-page/main-WAL 磁盘重演**：当前 caller 缺失如 A 表所记，但 production verifier 的内存测试已覆盖不变量。不是仅靠“不是磁盘”就判无效。
4. **任意 torn sector／原生 fsync-error／每恢复 profile 计时**：这些不是本次新造的代码门槛。硬件事实与平台能力分级资格另列，不把真实短写与 torn、uncertain 与 ack 混淆即可。

## C. 已知受阻项与外部资格：代码是否存在单列

| 项目／goal | 代码／手册是否存在 | 当前事实与闭合条件 |
|---|---|---|
| [ ] ANN 报告三项准入修正；§7.2:260；M7:360；§11.3:396 | **既知 REQUIRED_CODE_MISSING／修正未证完成，单列第 5 组受阻项。** `docs/testing/remaining-goal-boundary-20261012-zh.md`、`docs/acceptance-status.md:464,498,526` 保留 V3 的 HNSW 计数、IVF 全 lists membership、同 query-row rescore 报告问题；本次没有对应 V4 修正源的可核验完成证据。主树 `benchmarks/ann/main.go` 及历史测量本身存在。 | 不能把既知 FAIL 改成只需 rerun，也不能推断生产 ANN 算法已坏。V4 链不执行、不换名、不转 CI/remote。本报告不创建替代工具；需既有允许边界明确且修正源／可信报告可核验后才关闭。 |
| [ ] 已写的 restricted manual 场景；§4.1/M3/M8 | **CODE_DONE 对应有界 caller**：rollback public/Connector、A5、一／三页 recovery、九格 fault、WAL-tail/elapsed、plaintext backup、encrypted1→2/Restore、A8 六格源码、readonly-media A/B 和 namespace 手册。 | 实际运行仍 NOT_RUN。编译／tag/env 不是授权；后续只有独立合法运行记录才能补物理结果。R1–R6 尚缺的 source 不能塞进本行伪装成“只剩手工”。 |
| [ ] 平台锁、Sync、设备／掉电资格；§11.3:399 | 支持 backend、foreign compile/nm、相关平台／namespace 人工步骤已有；实际 media／身份／硬件是外部输入，不能靠模拟宣称具备。 | 普通 Linux 健康路径已运行，Darwin/Windows 等 native 资格与真实持久介质仍未证明。按平台能力分级，无全 OS×设备矩阵，不在本任务准备 mount／改权限／付费环境。 |
| [ ] 长期 soak、同机冷热基准；§11.3:396–400 | `benchmarks/transactions` 的真实持续运行 runner 与报告、`benchmarks/grams`、ANN 历史工具、WAL elapsed 合同已有；不能再称缺普通 soak 工具。 | 长时／固定硬件／冷热实际资格未完成；WAL 有界 n=1 不可当 P95。无需因为未测每 profile 再造工具。ANN 报告缺口仍保留上一行。 |
| [ ] Fuzz；§11.1:375／M10 | 41 根 fuzz 和 313 固定 seeds 的普通运行证据存在，持续 campaign runner 已有。 | 固定 seed PASS 不等于持续 campaign；41-target 持续链保持停止／受阻，不执行新 campaign，不换路。 |
| [ ] M9 真实输入与迁移复核；§9.2:339–345、508 | complete supplied catalog／definition／应用 SQL／profiles 输入、report＋inventory＋commented plans 代码已有；手工依赖计划与两工作流文档已有。sample collector 自身 coverage gaps 必须公开。 | 等真实同一 quiescent snapshot、目标版本资料、人工计划、获准 disposable PG/Neon 复核。无需新增自动在线 collector／executor；不得借此启动 ANN V4。 |
| [ ] 独立安全／密码学／许可／API 发布审查；§8.2:312；§11.3:400 | 安全文档、来源／许可清单与 API 文档存在；这是外部审查结果，不能用更多源码代替。 | 安全、一致性与发布门未全部满足，不宣称生产可用／完整 goal VERIFIED。无需为“代码全部写完”伪造审计结果。 |

## D. 每批真正的有限结束点

1. 只选择 R1–R6 中一个具体子项，或完成正在验证的 A8 六格；已有功能不再重复开发。每片先冻结有限场景／真实 authority／允许结果，不能在实现中不断增加验收维度。
2. 代码、必要正反断言、source guard 和具体手册齐全；保留原 failure、not-run、unknown；执行范围以已有授权为限。代码可以结束于 reviewed source/model/compile，受限物理继续单列 NOT_RUN，不能因此说整个 goal 完成。
3. 受影响的获准检查在精确源码上取得实际终态，复用未变覆盖写明来源；不要重复无关全仓门。某一测试尚未跑就写 NOT_RUN，不能继承邻近 PASS。
4. 按用户要求每批独立 Git 保存并核验实际提交／远端／归档状态；检查点文档写结果、未完成项和下一片。此报告本身不改 W、不提交、不启动 CI。
5. R1–R6 与受阻 ANN 修正关闭以后，可以准确报告“本有限剩余清单中的必需代码已闭合，物理／外部资格仍待完成”。在这些前置代码还缺时不能说“所有编码已做完，只剩人工跑”；外部资格未完成时也不能说生产 VERIFIED。

