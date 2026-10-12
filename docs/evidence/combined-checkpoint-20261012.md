# 五切片组合检查点证据（2026-10-12）

## 当前结论

**精确 tree59ce 的完整正常 26 门已自然 PASS。** 运行树 `59ce26879c4301babc7648fbddc5cc66c4097263`，3,267 文件、783 个 Go 文件，目录身份 `qualified-slices-automatic-unique-source-v2`。本文件及同批状态文档是树外静态 overlay；未修改运行源树。新文档检查点的提交、Git bundle、精确恢复 26 门与 CI 均 **PENDING**，不预写未来 PASS。

范围为 LEFT/Rows、encrypted inspect、Darwin inventory manual、Connector rollback manual 和 automatic UNIQUE（含 c382 oracle 空结果表头 helper 修复）。CHECK9c3b 未合入。完整源码/缺口矩阵见[当前状态](../current-status-20261012.md)。

## 组合完整运行的实际终态

`combined-checkpoint-normal-verification-v1` / session 93750 于 04:47:59 UTC 启动，04:48:12 UTC 初始化，**05:21:06.611522 UTC 自然完成 26/26 门、28 次 Go 调用 PASS**。所有门自然 exit0、具名验证 PASS；没有继承先前成功；本轮没有失败后的重试或强制终止。lease 为 RELEASED，观察子进程均自然收尾。旧 executor 54102 的 UNKNOWN 保留。

| 门 | 实际结果 |
|---|---|
| guard、四条 quick、live-writer | 六门 PASS；CLI 26、pager 31、range 3、IPK oracle 9、live-writer 7 named PASS |
| 完整 ordinary / race | 各 8,569 named PASS / 7 个固定预期 SKIP；8,576 named；1,436 Test、41 Fuzz、6 Examples，313 固定种子；50 个包事件（39 PASS / 11 无测试文件 SKIP） |
| 完整 pinned oracle | 10,301 named PASS / 0 测试 SKIP；1,551 Test、41 Fuzz、6 Examples，313 固定种子；51 个包事件（40 PASS / 11 无测试文件 SKIP） |
| LIMIT oracle/race | 2 根 / 394 named PASS，零 SKIP |
| vet、CGO0 build、五目标生产编译、模块核验 | 全部 PASS |
| 完整非测试依赖闭包 | 207 包 / 1,347 输入；Cgo/C-family/Swig/Syso 为空；包含工具支持包，不把全模块闭包写成 shipping 闭包 |
| shipping CLI/library 闭包 | 164 包 / 1,254 输入；无 testoracle、testscope、os/exec；Cgo/C-family/Swig/Syso 为空 |
| Darwin 两架构及 Windows 的 test-c/nm | 六门 PASS；Darwin 各 99 Test + 1 Fuzz，Windows 100 Test + 1 Fuzz；exact/absent 条件均成立；未执行外国映像 |

严格 RUN/terminal 配对未发现漏项、额外/重复/畸形事件、cached、zero-match 或非预期测试失败。包级“无测试文件”SKIP 与固定测试 SKIP 分开计数。固定种子不等于 41-target fuzz campaign。完整 oracle 已覆盖此组合当前的 automatic 与既有 explicit UNIQUE helper 路径，不依赖 c382 的单页有限结果。

5,689 个实际采样全部满足原门限（/tmp ≥ 1 GiB、workspace ≥ 2 GiB，各 ≥ 10,000 可用 inode）。采样最低值：/tmp 4,931,354,624 字节 / 1,274,528 inode；workspace 15,458,631,680 字节 / 1,835,107 inode。无资源失败 latch；这只报告样本，不声称未采样瞬间或物理持久性。

源码、工具、依赖与既有历史输入均通过前后字节/模式/已记录 metadata 身份检查；tree59ce、3,267 文件 / 150 目录记录保持。读取 atime 不声明不变。固定工具为 Go 1.27.2、SQLite 3.54.0 及原 pinned source ID，模块为 crypto v0.58.0、sys v0.49.0、text v0.43.0。使用原 cache、default /tmp 和门限，没有用资源样本为 overlay/tmpfs 的断电持久性背书。

终态锚点：completion SHA-256 `76eec9eac167905d0272f6a5ed04c1dbb474e82e3c4bb7cef00bac750c07e7f2`；lease SHA-256 `58e91cc2481e3528820f82991c38f8c63085a6f397b8907c0b0637663c6de6bc`；lead 实际工具回执 session93750、exit0、chunk947027，SHA-256 `4e27e1b688c652f2a0693bc1ddf5d04bbbf09bbe56042864e0051eaeabcd0386`。

## 已保存 main8bc 的闭合仅属于该基点

- commit `8bc189474bdb2b0a683c6c059992f9acbc426250`，tree `c0959fb87e8aaedd946e6bb7abbaddb67cf5ced0`。
- 源码 bundle `libfile_cbcb5098b9fc8191b818e0e7af348546`，SHA-256 `d8bd36da36e505292be417e2cc39a9ed64c42078473eaf82aafd0d4686adda87`，已精确读回。
- 恢复 V2 于 02:16:40 UTC 自然完成 26 门 PASS；ordinary/race 各 8,231 PASS / 7 SKIP，完整 oracle 9,925 PASS / 0 SKIP。completion SHA-256 `a71a444c115a2dc7a6b9c1246da4cdd755cafe704812284c966e224178269013`；终态归档 `libfile_1d6c010c88e88191837aadcecb23649a` 已完整读回。
- [CI 38190480842](https://github.com/ms-xiaosnow/gosqlite/actions/runs/38190480842) 的 ordinary job 114621985040、oracle job 114621984795 均 SUCCESS，精确 head 为上述 8bc。终态归档 `libfile_3d72dca130188191af17abbcb7eb3c5b` 已读回。

旧 Storage+UNIQUE 文中“提交/bundle/恢复/CI PENDING”是原作者时点历史，不能再作为 main8bc 当前状态；同样，上述 PASS 不能作为 tree59ce 的结果。原 Storage metadata 真 FAIL 与恢复观察器失败仍保留。

## 输入切片证据与失败保留

- LEFT V4 tree `abdcb5e13eda8ab11c49a6b18d1829352bd42ec9`：有限四门 PASS；ordinary/race 各 338 named PASS，oracle 17 named PASS。原 formatting STOP 保留；归档 `libfile_e7204c62cc208191a6e151a4bba6045e` 已读回。
- encrypted inspect tree `1969514118a82f1ec1730ad6e32e7d7909d17430`：原 gates1–7 PASS、gate8 FAIL、gate9 NOT_RUN；同源后续 gates8–9 PASS，合为有限九门两段闭合。ordinary/race 各 136 named PASS、31 个不同根；不是一次全绿运行。归档 `libfile_4b00bcebae60819199eca2a35cd86c1f` 已读回。
- Darwin tree `8a9c62c53fdb818ae7881ab69db0c61b66ac8c39`：九次 selection、五次 compile、五次 nm 原命令自然 exit0；stage19 observer FAIL_TEST_VALIDATION / 总流程 FAIL_TEST_OR_IDENTITY 原样保留。同五份 nm 输出经独立纯解析形成 supplementary static closure，未重跑 19 门、未运行原生映像。
- Connector tree `029344a38b91960eacd06bad156a3e50ed174129`：六项静态流程于 03:36:59 UTC 自然 PASS，2 个 AST 根 + 26 子项 = 28 named、三次 list、compile/nm 的 404 required / 13 absent。手工镜像、fixture、Connect/recovery 仍未运行；旧 executor UNKNOWN 不被覆盖。
- automatic UNIQUE c382 tree `c3824a47a9fc03a9c1808bf62e84bf924b0e771e`：E2 `automatic-unique-oracle-one-verification-v2` / session 90085 于 04:40:44 UTC 完成原第 11 门，3 根 / 6 子项 / 9 named PASS，零 SKIP/FAIL，自然 exit0、lease 释放。completion SHA-256 `26060aae5c10a0c660f6e863ccb4620b8fda513c403b4b2bfdf515dc9d2cf3b0`。289 个官方进程结果是源码推导的完整 PASS 路径计数（286 exit0、3 expected exit1），不是独立逐子进程收据。仅固定 4096 页有限范围，不代替完整页大小/方向组合或现有显式 UNIQUE helper 使用者的完整回归。

必须并存的历史：AU-1 V1 `c065143baa0e61d0c4aae942ae2b5051b26b28f2` SOURCE_HOLD；cf1e `cf1e27556d9b5aec314d72db2ae02a3810992580` read-only keeper fixture gate08 FAIL；7dd6 `7dd67b039ecfdfddafe28f2ee5b6745ab8cb40ed` 的 03/07/08/09/10 PASS 与 gate11 空结果表头 FAIL；E1/session 76084 recorder 初始化 FAIL（零 Go）；组合 v3 Rows pin HOLD 及 d79 `d79cd45d10a14095a337fdb61bf65063ee1dbcef` 35/36 pin HOLD。修正有各自独立审阅/后继身份，不能改写前驱为 PASS。c382 仅把观察器的非空 rows 条件改为非空 columns，SQL、断言与 launcher 保持；两处 source pin 只有完整 ownership 独立审阅后才窄修。

## CHECK 独立且未发布

后续独立状态补充（2026-10-12 05:27:58 UTC）：CHECK tree `9c3b0f09c1948c2c8b66337643677697d54852dc` 的 session 90221 已自然完成单条有限 oracle，**3 根 / 7 named PASS，零 SKIP/FAIL**，工具 exit0、lease RELEASED。这补充本节保留的 05:22:24 UTC 未运行快照；CHECK 仍未合入本组合、未发布，不把该有限 PASS 转授 tree59ce、恢复/CI 或物理资格。 completion SHA-256 `ce560207ee26af1f83208d4832060a14db530f20d43255c37ee838d156434826`；session90221 实际工具终态收据 SHA-256 `151d3164035b545930395b214b905647dc74d07a9877a27bcb4e9aecabd8c1e0`。

完整终态归档补充：tree59ce full26 终态包 `libfile_39e4c60cdfac8191b3b21d2da9cd2356` 已完整读回，32,581,319 字节，SHA-256 `156be5b9b2a914e4843999969a6e7a0f815cc5cde2be7661b9b4e803900de1a6`；610 个成员的字节/模式、GZIP CRC/EOF、静态终态及原始冻结输入不变检查通过。这保存实际运行证据；新检查点 Git bundle、精确恢复及对应 CI 仍 PENDING。

CHECK tree `9c3b0f09c1948c2c8b66337643677697d54852dc` 的 `check-constraint-finite-verification-v2` / session 27563 于 04:35:59 UTC 完成三普通门，18 根 / 28 named PASS、零 SKIP/FAIL；completion SHA-256 `9b65060b9eae8452b9345250c4b273daa9c72d6f713659483415c80e05fcac50`。截至 05:22:24 UTC，oracle 已批准但未运行；随后于 05:22:42 UTC 建立 holder 90221 等待独立 release。full26 已自然结束/lease 释放，CHECK 后续仍须自己的准入和实际收据。当前组合没有 CHECK，不用这三门扩展已发布兼容声明。

CHECK9c3b 与 automatic UNIQUE c382 的未发布源码备份 `libfile_74dc1f89dabc8191970b909821546cb5`，SHA-256 `85c29472a9a87d87df72f0af1d6fd49d95486949c22fbd8544ec30c2a1df164d`，已读回，6,855 个归档成员及两棵树的字节/模式完整核验。备份形成早于后续运行，原备份内 NOT_RUN 原样保留；静态完整性不代表运行恢复或发布。

## 已验证的独立归档不混同发布

- tree59ce 未发布源码包 `libfile_28d074da55e08191a43edef799057a45`：15,001,538 字节，SHA-256 `91bcbf846d55fc55478cf47d28c8a33c633cc3c1ead91eb8832682dcf4867150`。3,487 成员逐字节/模式与 GZIP CRC/EOF 核验，并从归档字节重建 tree59ce（3,267 source / 783 Go）。保存时 full26 仍运行，活动运行目录未收进该包；因此它是源码完整性保存，不是 Git bundle 或 full26 终态归档。
- CHECK/automatic UNIQUE 四有限终态包 `libfile_af0b03bfd1148191bdb207cab09a77d2`：33,005,864 字节，SHA-256 `8b1b81b6332370c261f7b0e846e891cf0388fd97f2ae901fb6416ac74cb11a2b`。1,025 成员 / 965 文件逐字节/模式、GZIP CRC/EOF、静态终态与原输入保留检查通过。同时保留 7dd6 oracle FAIL、E1 初始化 FAIL、CHECK 普通 PASS 与 c382 E2 oracle PASS；不把多个阶段抹平为一次无失败成功。

## 文档来源与静态核验

文档事实来自已存在的本地原件，按下列身份分层引用；同批树外 `PROVENANCE.json` 记录每份实际读取文件的 SHA-256/字节数与用途，避免把早期 REPORT 的 NOT_RUN 当作后续 completion：

- `qualified-slices-automatic-unique-integration-evidence-v2` 的 SOURCE-MANIFEST、FINAL-IDENTITY、源合并报告；`combined-checkpoint-independent-review-v2` 的 RESULT/REPORT。
- `combined-checkpoint-normal-verification-v1` 的 completion、lease、26 门 result/summary、final identity/source/input/resource、完整 resource-observations 与 lead 实际工具终态回执。
- `automatic-unique-oracle-one-verification-v2` 的 completion/summary/release 和 session90085 终态工具回执；session76084 零 Go 自然失败回执、cf1e/7dd6 原失败结果分别保留。
- main8bc restore completion、Library 恢复读回验证、CI SUMMARY 与其 Library 读回验证；四个输入切片的既有具名收据/当前状态原记录。
- CHECK completion、session27563 回执与 CHECK/c382 Library 源码备份读回；后续 CHECK oracle 审批及 05:22:24 UTC holder 状态来自本次协调任务，未将审批写成实际执行。

本次文档工作只读取源码/证据并写入独立 overlay，使用标准库静态哈希、Git 对象字节 framing（未调用 Git）、文本差异及本地文档链接核验。所有原有 fenced/indented code blocks 逐字节不变，无新命令、manual tag/env、launcher 或执行许可；原源树前后身份保持 tree59ce / 3,267 文件 / 783 Go。发布流程若应用 overlay，须另定新文档树并独立验收，不能把本文档 overlay 当作已提交检查点。

## 不变的真实余缺

native/manual/物理资格仍 NOT_RUN/BLOCKED；完整 fault/crash/restart、durability、格式升级/回退、安全/密码学/法律和长测矩阵仍有真实实现或外部资格缺口。健康正确密钥普通路径与现有正常 native 文件用例不等于这些资格。ANN V4、41-target campaign、未知中断链不得重启或改名绕过；M3/M8/M10、生产安全与原始 goal 未验收。
