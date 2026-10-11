# namespace 最终组合独立核验交接

## 结论与边界

限定 PASS。最终组合 `namespace-combined-freeze-v1` 的 tree 为 `697e45c6aba7a5cf62ebbf2bab2662f10903c5be`；独立计算得到 3070 文件、145 子目录，788 个非 docs 文件与已审阅源码 tree `01918609a55cc149c5155451da40f7389854db06` 的完整字节、大小、路径、Git 模式及实际 POSIX 模式逐项相同。当前主 checkout `gosqlite` 与最终冻结 tree 一致。

本次新执行的范围仅为源码 guard、`internal/testscope` 四个惰性源码/AST 模型测试及同集合 race、预期 manual-GOFLAGS 拒绝，另有两条只读工具链前检。没有运行完整普通套件、oracle、外平台编译、任何 manual 二进制、namespace 测试本体、恢复或其他受限物理场景。本报告不宣称新提交、保存/恢复、远端 CI、原生资格或整体验收完成。

源码/目录身份在执行前、执行后及静态审阅结束时均重新计算。最终 `final-identity.json` 确认 main、final freeze 和原 source freeze 均未改变。库存包含 root/子目录权限；不包含 `.git` 元数据。本审阅未调用 Git，未修改 main、freeze、Git 状态、远端或 CI；没有复制旧候选全树。

## 实际 Go 调用与时间

顺序检查脚本于 08:55:05 UTC 开始，提交早于收到“只限五条检查命令”的后续澄清；脚本中的 `go version`、`go env` 实际于 08:55:07 UTC 执行。因此实际是七条工具调用，不是五条，也不声称完全符合后来更窄的五条限定。两条多出的调用仅读取版本/配置，不是受限数据库场景；已即时向主任务披露，没有补跑任何命令。

所有调用使用 `/workspace/scratch/5e06a74e5f65/durable-restart/toolchains/go1.27.2/bin/go`，cwd 为最终 freeze。完整 argv、各次完整配置、UTC 起止、退出码和合并 stdout/stderr 的日志/哈希保存在 `command-results.json` 及 01–07 日志。

1. `version`：08:55:07.115–08:55:07.145 UTC，exit 0，`go version go1.27.2 linux/amd64`。
2. `env -json GOVERSION GOTOOLCHAIN GOPROXY GOSUMDB GOCACHE GOPATH GOFLAGS GOTMPDIR CGO_ENABLED`：08:55:07.146–08:55:07.153 UTC，exit 0。
3. `run ./internal/testscope`：08:55:07.154–08:55:07.876 UTC，exit 0，whole-tree source-reference guard PASS。
4. `test -v -count=1 -parallel=2 -timeout=0 ./internal/testscope`：08:55:07.877–08:55:08.523 UTC，exit 0。
5. `run ./internal/testscope`：08:55:08.524–08:55:08.771 UTC，exit 0；紧邻 race 前重跑 guard。
6. `test -race -v -count=1 -parallel=2 -timeout=0 ./internal/testscope`：08:55:08.775–08:55:10.981 UTC，CGO_ENABLED=1，exit 0。
7. `run ./internal/testscope`：08:55:10.984–08:55:11.193 UTC，仅本次 GOFLAGS 加 `-tags=manual_namespace`，预期 exit 1，精确拒绝信息 `manual test tags are forbidden in ordinary checks`。

普通与 race 均实际执行以下四项，各一次 RUN/PASS、零 SKIP、无空匹配：

- `TestIndependentReviewGuardDirectAndAliasLimit`
- `TestScopeSourceReferenceGuard`
- `TestNativeNamespaceMethodReferenceGuard`
- `TestScopeRecoveryOptionTypeChains`

这些测试只调用 parser/checker、字符串与测试库；引用的数据库/rename/recovery 示例均为惰性字符串，没有数据库描述符或原生 namespace 操作。guard 仅遍历并读取测试源码。

七条 Go 调用都自然结束。08:55:11.193 UTC 后没有新的 Go 子进程，本地释放记录在 `compiler-slot-released.txt`；随后立即向主任务交还编译器。执行后库存于 08:55:11.826 UTC 完成，静态工作不再占用编译器。

## 环境与资源

Pinned Go 1.27.2；GOTOOLCHAIN=local、GOPROXY=off、GOSUMDB=off、GOENV=off、GOWORK=off；既有 ROOT/go-cache 与 ROOT/go-path；GOMAXPROCS=2；GOFLAGS=`-buildvcs=false -p=1`。CGO 仅第六条纯模型 race 为 1，其余为 0。GOTMPDIR unset，TMPDIR=/tmp，两个 manual 环境门始终为空。没有下载、另设 cache/temp、清理、降低阈值或自动超时终止。

七次调用前均检查 `/tmp` 至少 1 GiB、workspace 至少 2 GiB、两者可用 inode 各至少 10000：

- `/tmp` 最小空闲 1,413,300,224 bytes，最小可用 inode 1,243,469。
- workspace 最小空闲 11,398,025,216 bytes，最小可用 inode 1,164,241。

全部满足原门槛。详细快照在 `resource-prechecks.json`。这些容量门及普通模型通过不证明物理持久性。

## 六路径源码审阅

独立严格解析原六路径 unified patch，在内存中逐 hunk 匹配当前 P0 基线的每条旧行、偏移和计数，得到的所有输出均与最终 freeze 一致。没有 fuzzy 应用、旧整树覆盖或源文件写入。

- Windows/Darwin 两个完整原函数移入精确首行 `//go:build manual_namespace` 文件。首条语句就是精确环境 gate，先于 TempDir/原生操作。
- 去掉且只去掉 gate 后，完整函数字节/SHA-256 与原版相同；原 native 文件只删除对应完整函数和确实不再需要的 import，其余字节全部保持。Windows 当前所需 context 仍保留。
- 原 P0 `internal/testscope/main.go` 只增加 `Rename` / `rename` 两个 selector；逆向删除这两项复原整个原文件字节。同文件 alias、dot-import、unkeyed Options、恢复函数/字段检查及全部既有 guard 测试保持。
- 新模型文件没有实际 namespace 操作。保守 selector 误报、bare helper/wrapper/reflection 等局限在文档中披露；exact manual tag 豁免不自动证明环境门或安全。

`body-and-p0-preservation.json`、`six-source-hunks.json` 提供完整比较结果。原 owner 与 independent 在 01918609 上取得的 188 项选择与六项编译证据属于同一完整非 docs 源码身份；本次只读回核验这些记录，没有重跑选择或编译，也没有将它们写成原生运行。

## 文档、手工命令与历史范围

九个归属文档与文档组合候选逐字节/POSIX 模式一致；独立逆向严格重放九文档补丁恢复基线 SHA-256 与 Git blob，证明没有覆盖非归属内容。`progress.md` 完整原文件仍是新文件前缀。

本组合相对原基点修改 13 个既有路径（3 源码、9 文档及 `docs/checkpoints.json`），新增 34 路径（3 源码、lead 证据页、29 工件及1清单）。goal、生产实现、依赖、CI 和其他既有文件保持。P0 checkpoint 追加与 namespace 文档范围分开记录。

- 157 个最终树中的相对链接/锚点通过；直接使用最终树解析，不以候选路径替代缺失目标。
- 45 个 Bash/sh fence 只经 `/bin/bash -n` 语法检查；一个新增 Python heredoc 只经 AST parse。没有执行任何文档示例。
- 中文 §19 的平台、包名、两条现存精确函数名、首行 tag、首句 env gate、单函数锚定 selector、固定版本/离线设置和 testing flags 均静态核对。二进制 flags 对照固定 Go 的 `src/testing/testing.go`，没有启动二进制或另发 Go 查询。
- Windows 锁语义、Darwin parent/child 的原有断言、Close 断言不足和 t.TempDir 自动清理限制均准确保留。编译只称 COMPILE_PASS，未来精确单函数、零 SKIP、自然 exit 0 及证据齐备才可单独记录 NATIVE_CASE_PASS。
- default/oracle 在原适用平台有意减少恰这两个既有 encstore 测试声明；不同于历史两个 plaintext namespace 测试。原 Darwin/iOS 关系保留，不扩大原生资格。既有选择矩阵的实际记录再次核对，其他测试声明未减少。
- sentinel 被明确标为防误启动措施；没有授权新 OS runner、原生 aggregate、故障/恢复/错密钥/篡改/中断发布、ANN V4、41-target fuzz 或未知受限链路。

没有把旧 P0 的 full ordinary/oracle/race PASS 移到 namespace 新组合，没有把源码/模型、选择或 compile PASS 写成 native PASS。文档中未虚构当前 namespace 新提交的自指 SHA。

## 保存证据与 P0 时点

lead 证据页及其目录 29 个工件、1 个 manifest 已逐项读回。所有材料 SHA-256/长度匹配；四个 gzip 同时验证压缩字节与解压原件身份。再独立重哈希四份前置清单的全部 617 个原始文件：owner 252、独立源码 12、独立编译检查 229、P0 精确恢复 124，全部匹配。

P0 checkpoint 的提交为 `56bbcf3febe233197aa0a5eac965c67a67a88011`，bundle 与 08:43:14 UTC 的 21 命令/11 项每模式恢复结果保持；CI `38125268420` 明确记录 `IN_PROGRESS as of 2026-10-11T08:45Z`，ordinary 当时进行中、oracle completed/success。中文文档 08:44 的旧快照同样未宣称终态。本次只核对保存记录与其时点，未查询远端当前 CI；它不能替代 namespace 新提交及其独立终态。

## 交接文件

- `report.md`：本中文交接。
- `command-results.json`、01–07 logs、`check-summary.json`、`checks.exit`：真实七条 Go 调用与有限结论。
- `before-*`、`after-*`、`final-*-inventory.json`、`final-identity.json`：完整源码/路径/Git-POSIX/目录前后身份。
- `six-source-hunks.json`、`body-and-p0-preservation.json`、`nine-docs-reverse-replay.json`：源码及文档精确重放证明。
- `doc-links.json`、`shell-fence-syntax.json`、`manual-command-and-checkpoint-review.json`、`static-summary.json`：静态检查。
- `evidence-hashes.json`、`prior-results-scope-readback.json`：材料与既有结果范围核对。
- `verify.py`、`static_review.py`、`finalize.py`：本次独立检查代码；仅为证据，不是项目新增 runner。
- `EVIDENCE-MANIFEST.json`：上述本次工件的字节/哈希，清单不包含自身。

没有阻断源码/文档发现；后续只能按另有授权推进真实提交、保存/恢复及 CI，不据此扩大运行范围。
