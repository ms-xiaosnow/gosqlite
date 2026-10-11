# Windows/Darwin namespace 测试门控证据

2026-10-11。本批以已保存 `56bbcf3febe233197aa0a5eac965c67a67a88011` 为基点，逐 hunk 合入六条源码路径，保持 P0 公开恢复与 LIMIT 修正。没有修改数据库生产行为、依赖、CI 工作流或原始 goal。

## 精确范围

- 两个既有 native 测试完整迁入 `manual_namespace` 文件，并增加首语句 `GOSQLITE_MANUAL_NAMESPACE=independently_authorized` 环境门：`TestWindowsEncryptedRelativePublicationAndRetainedRoot` 和 `TestDarwinEncryptedRelativePublicationAndParentLifetime`。
- 删除且只删除新 gate 后，函数体与原版逐字节相同。其余断言不弱化；Windows 当前所需 context import 保留。
- `internal/testscope/main.go` 只追加通用 `Rename` / `rename` selector 检查，P0 recovery guard 原字节保留；新增 guard 回归仅解析惰性源码字符串。
- 默认/oracle 在原有适用平台减少恰这两个测试声明。manual 原生运行仍 **BLOCKED / NOT_RUN**。环境值只防误运行，不能授予执行权限。

已审阅候选 tree `01918609a55cc149c5155451da40f7389854db06` 基于 P0 运行 tree `442a7104dc780ddb20826e5ee4f1250b291f2b24`；其余3001个既有文件保持。当前组合只搬六个归属路径，后续 P0 文档、提交证据和 namespace 中文步骤单独保留。源补丁 SHA256 `18ba34e90283080cbc0a1ab1ab1e213945d2b9a05168acb33221498faaba4ee0`。

## 实际检查

- owner 于08:23:50 UTC完成：source guard、纯 AST/model、同集合 race、manual GOFLAGS预期拒绝、47目标×4标签的188次选择、6项外平台编译与精确函数符号。
- 独立审阅先完成源码/完整函数/补丁复现；之后获准的同范围检查于08:27:31 UTC自然结束、PASS。原 source-only 报告保持，追加报告记录新检查。
- 两方的完整源库存、路径、Git/POSIX modes前后一致。foreign binaries只编译和读取符号，从未运行。没有47平台原生资格结论。
- 对原始证据清单重新逐项读回哈希：owner252项、独立源码12项、独立检查229项均匹配。本目录选录关键报告、命令、矩阵和身份，完整原始材料另行归档。

证据入口：[owner](namespace-gating-20261011/owner-report.md)、[独立源码审阅](namespace-gating-20261011/independent-source-review.md)、[独立运行范围追加](namespace-gating-20261011/independent-checks-addendum.md)、[完整函数证明](namespace-gating-20261011/independent-body-equivalence.json)、[188选择矩阵](namespace-gating-20261011/independent-selection-matrix.json)、[6项编译符号](namespace-gating-20261011/independent-foreign-compiles.json)、[保存文件哈希](namespace-gating-20261011/evidence-files.json)。

## 当前限定结论

上述是具名源码、模型、选择和编译的 PASS；最终文档组合、提交、Library恢复与该提交CI须各自登记真实身份/结果。未将先前 P0 的完整普通套件冒充 namespace 新组合运行。完整 CI 仍使用既定 Linux 普通与 oracle 流程，未启用新 OS/perf runner。

中文手工步骤见[主手册第19节](../manual-verification-zh.md#19-os-raw-namespace-门控与独立原生验收2026-10-11)。明确说明两原测试的 t.TempDir 自动清理和已有 Close 断言限制，不能声称新增了失败现场永久保留或完整资源故障证明。源 guard 仍非完整调用图或物理安全证明。tmpfs/fsync=volatile、受限恢复/故障测试和 M10 未完成状态不变。


## 2026-10-11 namespace 最终组合限定门与 P0 持久闭合

运行冻结 tree `697e45c6aba7a5cf62ebbf2bab2662f10903c5be` 于09:01完成独立核验：3070文件、788非docs与已审候选完整字节/Git/POSIX模式相同；guard、4项纯模型及同集合race PASS，manual GOFLAGS按预期拒绝。实际7条Go工具调用含早于后发5条限定的只读version/env前检，完整记录保留。另157链接、45shell语法块、29选录工件/617前置原件哈希PASS。没有运行该组合完整aggregate/oracle、外平台二进制或手工物理测试。报告见[最终组合限定证据](namespace-gating-20261011/final-composition-report.md)。此后仅追加终态文档/证据；最终提交的788非docs须逐字节核验等同该运行冻结源。

父P0提交 `56bbcf3febe233197aa0a5eac965c67a67a88011` 的 CI `38125268420` 已于08:56:41 UTC SUCCESS，两job ordinary114428206934/oracle114428207050均SUCCESS；精确Library恢复21条命令PASS。终态证据归档 `libfile_4a413988d4608191aebf695542a04caf`，10211483B，SHA256 `0d6579c338465be5df401b4f2a594df8fe63737900a78dbcc9e4ef28e85bbfb3`，读回411条/410文件哈希PASS。原08:44/08:45 PENDING是先前写作时点，现仅对父P0闭合，不能当作namespace新提交CI。


## 2026-10-11 09:32 UTC：namespace 父检查点终态闭合

提交 `50b627e11e9dbce7f2fde8df35c4634c52a06d52`、tree `8f2709293287a3517a52b3f989888ded8230ef53` 的 CI `38127196102` 于09:29:33Z自然完成 SUCCESS；ordinary `114433911927`、oracle `114433912045` 均SUCCESS。Library bundle `libfile_e5dcbc0ac16c8191bce977aa165163f9` 已读回并精确恢复，207条限定检查PASS。全部manual/真实namespace仍NOT_RUN。终态证据归档 `libfile_6b7a1c37d8fc8191bed2389e8dbe14dd`，15356765字节，SHA256 `21cd1a73b622123605eb48a5031b0cc38314a04eac8a9b71c72f873ecaa3d701`，Library读回843项/842清单哈希PASS。此前“运行中”只表示当时观测；此终态不属于新增public harness提交，不代替它的保存/恢复/CI。
