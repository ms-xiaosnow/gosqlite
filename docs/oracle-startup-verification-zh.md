# SQLite 启动隔离与本次独立切片的中文复验规程

日期：2026-10-11 UTC。本文的未来用户模板仍 **NOT_RUN**；新 CI 未启动。
12:31 UTC 更新：52b 精确本地源码的两 quick 加全 15 门已独立自然 PASS，含
完整 oracle 7,202 路径和末两名 LIMIT oracle race 394 路径。该结果不是本文
用户模板的执行结果，也不是新提交/恢复/CI 或物理资格。startup 独立源码
审阅及两条纯命令的历史 PASS 保留（14 Test / 198 路径和 AST guard）。本文只编写、静态核对未来用户步骤，没有执行下列命令、创建
手工 fixture、运行 Go/编译器/脚本或启动 SQLite。步骤不自动授予执行权限。

## 1 当前源码与资格

已发布基点为 `b3492ed822d59834e5159cb7964c9dbd937027f7`，tree
`ff13ded52883edfbabf747547a11b4e4ed7301ee`。startup 独立候选 tree 为
`5fec5ce5e5339f02dc2715d9a2d6bd4691b1c62c`，不是已经发布的新 commit，
也不是与 WAL、LIMIT、Example 的组合验收身份。四切片已静态组成 tree
`904ca698c3359bfe0674231f6d14ec8a9d704f30` 已保存/静态恢复，后来普通门因
生产规划回归 FAIL；首次修复 `209ba515f4dce18ae57103254b4053d4708b8736`
恢复旧 9 根 / 23 路径 PASS，但因新增测试误用 ranking 常量仍总体 FAIL。
两轮各自第 3–15 项 NOT_RUN。新树 `52b03f1d94f578fb6e3722c7bf28fc0056fad6d4`
只改新测试 shape 常量，生产不动；独立审阅后两 quick 与全 15 门已自然 PASS。
HEAD 仍为 b349，候选均未发布；原始失败不覆盖，详见
[分层证据](evidence/startup-wal-limit-20261011.md)。

候选的 46 个 Go 调用点（26 个测试文件）和 5 个直接 shell 调用点（3 个脚本）
均已在源码改造。新 `internal/testoracle` 只供测试使用；生产 SQL 没有引入外部
SQLite 依赖。`Spec` 固定以下启动输入：

- Path：经过词法检查的绝对可执行文件路径，不做 PATH 搜索
- Args：可执行文件、领先 `-noinit`、原始参数；不重排 SQL、flags 或数据库参数
- Env：恰为 `LC_ALL=C`、`LANG=C`、`TZ=UTC`、`SQLITE_TMPDIR=<root>`、`TMPDIR=<同一 root>`
- Dir：明确的绝对 package 工作目录，保留原有相对 readonly fixture 的意义
- Linux-only admission；临时根须为 `/tmp` 或其词法子目录，非法首选路径报错，不 fallback

51 是原 startup 候选的源码调用点数，不是进程运行次数。新组合另含已适配的
LIMIT 2 处；备份候选 1 处仍在组合之外。必须分别审阅，不得把它们当作原有
51 点已经动态验过。

官方 SQLite 3.54.0 源码表明：SQLITE_DEBUG_BREAK 先于正常 flags；用户初始化
先于 version/help；只加 `-noinit` 不能关闭前一环境分支，只改 HOME 也不能阻止
账号 home 查询。新构造不传上述输入，包括空值 SQLITE_DEBUG_BREAK。静态审计
没有读取真实 sqliterc、账号目录或环境变量值，也不推断历史真的受到了污染。
历史 RUN/PASS/FAIL/SUCCESS 保持，旧启动隔离资格为 **NOT_PROVEN**。

重要边界：NewSpec/TempRoot 是显式字符串模型，ForTest 不是纯函数。后者调用
`os.Getwd`，Go 1.27.2 实现可能读取 PWD、stat `.` 和 PWD 或进行系统目录查询。
PWD 不复制进 SQLite 的 Env；绝对路径检查不证明 canonical path、无 symlink、
文件身份、目录权限、挂载或未来 namespace 稳定。SQLite 在临时目录失效时仍可能
选择其他位置。文件型 CLI 也存在正常文件探测；`:memory:` 不等于绝不落临时盘。

脚本仅对 SQLite child 使用 `/usr/bin/env -i`；父 shell、trusted env/uname 的
loader、下载器、configure、C/Go 编译器和 OS 系统配置仍在可信边界内。installer
另新增了绝对 base 的早期检查，不能称整个父脚本行为或字节均未改变。这里控制
声明的子进程启动输入，不能宣称完整 OS 隔离或只执行一条显式 SQL。

## 2 版本、工具与准备条件

仅面向未来另行批准的真实 Linux 用户环境，Bash 及 `/usr/bin/env`、`/usr/bin/uname`
须已安装且可信。还需 Git、tar、mktemp、find/sort/xargs、SHA-256、stat、df、findmnt、
Python 3 和固定 **Go 1.27.2**。新 helper 的原生 Windows/Darwin 启动合同尚未实现，
不能换 OS 执行或用交叉编译冒称原生 oracle 资格。

模块固定于 go.mod/go.sum：x/crypto **v0.58.0**、x/sys **v0.49.0**、x/text
**v0.43.0**；预先准备已经校验的离线 module cache，缺失即停止。普通生产检查
CGO=0；单独 race 需支持的 C 工具链和 CGO=1，不意味着生产 SQL 依赖 C。

官方测试 oracle 为 **SQLite 3.54.0**，source ID：

```text
2026-10-09 15:46:58 be8d059e9a49089ab2dce5ed26dd87aaf598fdc5fbe0b107c0dd758464bcd5e3
```

源码 ZIP `sqlite-src-3540000.zip` 为 14,881,448 字节，SHA3-256
`a5c29342ca185abcda332448fbb27c536661245be27e20199b46c49099b0e15e`；生成的
sqlite3.c SHA3-256 为 `8efd453d08a7cfc4c79de5923153036cdc4dcaa254d4f1e617be6a2a98828f9e`。
来源记录见 [upstream](upstream.md)。必须保留实际 CLI 二进制哈希、构建 log/options
和依赖来源；source ID 字符串不足以证明任意二进制的构建可信度。

`scripts/build-oracle.sh /absolute/new/directory [/absolute/sqlite-src-3540000.zip]`
与 `scripts/install-baselines.sh /absolute/empty/toolchains-dir` 是已存在的脚本形状。
它们也会启动 SQLite，包括版本查询；本规程不执行、不自动安装或下载。准备构建
仍须另行审阅完整脚本/bootstrap/工具来源，不能为避开 HOLD 改用系统 sqlite。

当前实际执行任务的既有 cache、默认 `/tmp` 和资源政策保持；下面的新目录只属于
未来用户自行开展的新复验，不能拿本文替换当前任务的缓存或降低资源门。

## 3 独立副本、私有临时根与启动器

先人工审阅所选提交的完整 delta、helper、全部调用点、测试 body 和依赖。
`SOURCE_COMMIT` 必须显式填已经包含新 helper 的审阅 commit 全 SHA，不填 tree，
不填旧 776b/b349。没有这种已审阅提交时就在此停止，不导出未提交候选冒充发行版本。

逐块采用 [主手册第 4.2 节](manual-verification-zh.md#42-建立独立源码与数据目录)
和第 4.3 节的完整可复制步骤，它们会：

1. 验证 clean HEAD 与指定 SOURCE_COMMIT 相同，核对真实 helper/source 文件存在
2. 用 git archive 导出该 commit 到全新 RUNROOT/source，保存完整 commit/tree/archive/source hash
3. 在 `/tmp/gosqlite-oracle-XXXXXXXX` 创建用户本次私有 0700 根，并保存实际路径
4. 明确 unset GOTMPDIR、SQLITE_ORACLE、TMP/TEMP 和三个 manual gates；TMPDIR 指向该私有根
5. 用未导出的 ORACLE_BIN 保存绝对 CLI 路径，固定 reviewed ORACLE_CWD，记录 binary hash
6. 定义逐次保存 argv、UTC、stdout/stderr、native exit 的 capture，不覆盖旧证据

presence 检查只是防错，不代替完整源码审查。Go 1.27.2 test TempDir 的非空
GOTMPDIR 优先于 TMPDIR；所以这里明确 unset GOTMPDIR，使 Go fixture 与 SQLite
选择同一个已批准私有根。临时根不是数据库物理持久性 fixture：真正的持久性
资格要另置于独立核验的持久设备。tmpfs 或 `fsync=volatile` 不能取得该资格。

在上述变量、目录、capture 均已建立后，可单独核对这里的关键启动器形状；
它与主手册定义一致，不需要重复定义两次。没有 command 被此定义自动执行。

```bash
# 未来用户模板；本次 NOT_RUN。仅 Linux，路径先经过主手册前置检查。
unset SQLITE_ORACLE GOTMPDIR
export -n ORACLE_BIN
export TMPDIR="$ORACLE_TMP"
oracle_child() (
  cd "$ORACLE_CWD" || exit 1
  exec /usr/bin/env -i LC_ALL=C LANG=C TZ=UTC \
    "SQLITE_TMPDIR=$ORACLE_TMP" "TMPDIR=$ORACLE_TMP" \
    "$ORACLE_BIN" -noinit "$@"
)
```

普通命令使用未导出 ORACLE_BIN 不会意外打开 `checkOracle` 的可选测试。
完整 oracle 的 SQLITE_ORACLE 只在单条调用环境中显式提供，不使用全局 export。
不要调用 Cmd.Environ、打印全部环境或读取用户初始化配置来“验证”；这里没有这类步骤。

每条将来的 Go 调用前至少检查 `/tmp` 可用 1 GiB、工作目录文件系统 2 GiB、两者
各 10,000 free inodes，并核对 temp/cache 可写可搜索；更大负载另定更高门槛。
这是入场余量，不是运行峰值或资源隔离证明。不要用 source 目录的 df 替代临时根。

```bash
# 未来用户模板；每次名称必须唯一，本次 NOT_RUN。
capture preflight-temp-mount findmnt -T "$ORACLE_TMP" || exit 1
capture preflight-data-mount findmnt -T "$DATA" || exit 1
capture preflight-bytes df -B1 "$ORACLE_TMP" "$RUNROOT" || exit 1
capture preflight-inodes df -i "$ORACLE_TMP" "$RUNROOT" || exit 1
capture preflight-permissions stat -c '%a %u %n' "$ORACLE_TMP" "$RUNROOT/go-cache" || exit 1
```

上面是记录命令，必须人工判定实际数值达到门槛才进入下一步；缺数据或阈值不足
即 BLOCKED，不自动清理、降阈值、迁移缓存、改 mount/权限或换环境重试。

## 4 startup 纯选择器与 source guard：先单列资格

在已审阅新提交和单独编译/测试许可下，以下精确名字均有实际源码。五个 spec
测试只检查字符串和 unstarted Cmd；八个新增 guard 测试与一个原有 guard 测试
检查源码/字符串 fixture，不执行里面的 shell/Go 文本。compiler/runtime/cache
本身仍有正常 OS 活动，不能称整个 `go test` 无描述符或完全纯净。

```bash
# 未来用户模板。本模板 NOT_RUN；既有独立候选 pure PASS 不构成当前新执行许可。
PURE_SELECTOR='^(TestSpecArguments|TestSpecRejectsInvalidPathsAndPlatforms|TestSpecEnvironmentAndCopies|TestTempRootPrecedence|TestSpecPreservesRelativeFixtureCWD|TestScopeSourceReferenceGuard|TestOracleSourceConstructorGuard|TestOracleSourceImportAndMutationGuard|TestOracleSourceFactoryLiteralGuard|TestOracleSourceNativeExceptionGuard|TestOracleSourcePathScope|TestOracleShellFiveSiteContract|TestOracleShellStartupMutationGuard|TestOracleFrozenSourceContract)$'
capture startup-pure /usr/bin/env -u SQLITE_ORACLE -u GOTMPDIR CGO_ENABLED=0 \
  "$TOOLCHAINS/go1.27.2/bin/go" test -mod=readonly -buildvcs=false \
  -p=1 -parallel=1 -count=1 -json -vet=off -timeout=60s \
  ./internal/testoracle ./internal/testscope -run "$PURE_SELECTOR" || exit 1
capture startup-source-guard /usr/bin/env -u SQLITE_ORACLE -u GOTMPDIR CGO_ENABLED=0 \
  "$TOOLCHAINS/go1.27.2/bin/go" run -mod=readonly -buildvcs=false -p=1 \
  ./internal/testscope || exit 1
```

必须有且仅有这 14 个具名顶层测试实际 run/pass，零 fail/skip、无空匹配、无
缓存标记、无未解释 stderr，process 自然 exit 0。单独 source guard 的新输出应为：

```text
Ordinary test and oracle startup source-reference guards PASS; behavioral review is still required.
```

还须核对全部预期子项、原始 JSON、代码和工具前后身份。source guard 是语法/
冻结哈希防错辅助，不是完整 dataflow/type/reflection/OS 证明。不能仅修改 guard
中的 expected hash 让新源码通过；任何 launcher/helper 修改都需全量重新审阅。

这组通过也只可记 SOURCE_MODEL_PASS，不会把 oracle、WAL/LIMIT 组合、CI、
Windows/Darwin 或物理资格变为 PASS。本模板的有限 `-vet=off` 不提供 vet 结论；
完整 vet/build/race 如需执行，必须另列范围和证据。

## 5 未来官方 CLI 及完整 oracle：用户模板仍待独立许可

52b 的另一次受控执行已有精确 oracle PASS；下面仍是未执行的未来用户模板。
块中 HOLD / NOT_RUN 限于这些模板，不能借别的源码或运行许可自动启动。

只有完整 startup review、二进制/build 身份、CWD、临时根、全部调用闭包与具体
SQL/stdio/正常退出方式都已另行获准，才考虑下面两项。不能以 version/help
或 `:memory:` 名称替代审查，也不要先跑 `sqlite3 --version` 试探。

```bash
# 当前 HOLD / NOT_RUN。仅供未来完成独立前置的用户逐步采用。
capture startup-oracle-identity oracle_child -batch -bail -noheader -list :memory: \
  'SELECT sqlite_version(),sqlite_source_id();' || exit 1
capture startup-full-oracle /usr/bin/env -u GOTMPDIR \
  SQLITE_ORACLE="$ORACLE_BIN" TMPDIR="$ORACLE_TMP" \
  /bin/sh ./scripts/test-oracle.sh || exit 1
```

identity stdout 必须逐字为：

```text
3.54.0|2026-10-09 15:46:58 be8d059e9a49089ab2dce5ed26dd87aaf598fdc5fbe0b107c0dd758464bcd5e3
```

`scripts/test-oracle.sh` 的实际顺序是 Go 版本、source guard、Linux/path/temp/CWD
preflight、五键/noinit identity child，再 `go test -count=1 -timeout=30m -tags oracle ./...`。
完整门保留 `./...` 和 oracle tag，没有 -run/package 缩减，不包含 manual tags。
参数中 30m 是脚本已存在的 Go 时限，不是新的 crash/强杀试验。不能用限定子集
或零匹配成功充当完整门；逐个核对匹配、skip 和失败，没有结果不得填 PASS。
这条 shell 模板并不清洁整个 bootstrap，仍须信任它的 PATH/toolchain/loader。

主手册五个直接 SQLite 例子均改用 oracle_child；原始 SQL、数据库与 CWD 意义
保持。新用户可以选择不运行这些 oracle 步骤，记录 NOT_RUN；不能把 ordinary
结果或历史 CI SUCCESS 当作补足它们的证据。

## 6 LIMIT 与 Example 的独立复验入口

这里的候选结果已经存在，但以下命令是未来已审阅集成版本的用户模板，本文
没有重跑。先确认指定 commit 中有对应实际测试文件及函数，不能对旧 b349 空选。
完整原始 16 Test / 1,145 子项名单见[证据摘要](evidence/startup-wal-limit-20261011.md)。

```bash
# 未来明确批准的 focused 普通模板；没有 oracle/manual tag，不是完整组合门。
LIMIT_SELECTOR='^(TestPlannerParityV4cLimitErrorOwnership|TestLimitPhaseBindingPrecedence|TestLimitPhaseGroupedAndConnectionPreservation|TestLimitPhaseResourceOwnership|TestLimitConversionTypes|TestLimitConversionErrorOwnership|TestLimitConversionPublicSQL|TestGroupedAggregateSemantics|TestGroupedAggregateBinding|TestGroupedAggregateLimitsAndLifetime|TestGroupedAggregateOffsetAndScalarFailureAtomicity|TestGroupedAggregateTopKAdmissionAndFailures|TestGroupedAggregateRowidLookaheadAndTopK|TestConnectionCountersLazyTransactionReads|TestCachedPreparationOutcomes|TestReviewPreparationErrorPrecedenceAndIsolation)$'
capture limit-phase-normal /usr/bin/env -u SQLITE_ORACLE -u GOTMPDIR CGO_ENABLED=0 \
  "$TOOLCHAINS/go1.27.2/bin/go" test -json -count=1 -p=1 -parallel=1 \
  -vet=off -timeout=90s ./sqlite -run "$LIMIT_SELECTOR" || exit 1
capture limit-phase-race /usr/bin/env -u SQLITE_ORACLE -u GOTMPDIR CGO_ENABLED=1 \
  "$TOOLCHAINS/go1.27.2/bin/go" test -race -json -count=1 -p=1 -parallel=1 \
  -vet=off -timeout=180s ./sqlite -run "$LIMIT_SELECTOR" || exit 1
capture example-discovery-normal /usr/bin/env -u SQLITE_ORACLE -u GOTMPDIR CGO_ENABLED=0 \
  "$TOOLCHAINS/go1.27.2/bin/go" test -json -count=1 -p=1 -parallel=1 \
  -vet=off -timeout=60s ./extension/similarity -run '^ExampleEngine_Levenshtein$' || exit 1
capture example-discovery-race /usr/bin/env -u SQLITE_ORACLE -u GOTMPDIR CGO_ENABLED=1 \
  "$TOOLCHAINS/go1.27.2/bin/go" test -race -json -count=1 -p=1 -parallel=1 \
  -vet=off -timeout=60s ./extension/similarity -run '^ExampleEngine_Levenshtein$' || exit 1
```

LIMIT：每个 lane 恰 16 Test 与 1,145 子项，即 1,161 test run/pass 事件，另有
1 package pass；零 skip/fail。要核对绑定错误先于执行、零 LIMIT 抑制 OFFSET
执行、EXPLAIN 抑制执行、计数状态与资源归还等原断言，不能只检查退出码。
基线的 49 条实际断言 FAIL 已保留；这里不提供覆盖它的新“成功基线”，更广的
CAST/IPK/LEFT-ON/WHERE/排序阶段缺口仍在。

Example：每个 lane 恰 1 次具名 run/pass 与 1 package pass，无其他测试或空匹配。
保留 `Output: 3 false`，由 Go 的标准 example harness 捕获和比对；成功时通常
没有单独 stdout `3 false` 事件，不要伪造这行日志。旧 WAL 树只有 5 个 runnable
Example、该 sixth 未运行的事实不变，新一行空行修正有自己独立的 source/hash。

WAL 本次完整非 oracle 普通门已在另一个树运行，详见证据，不在本文编造一个
缺失的 native tail publisher/harness 命令。跨编译、内存 proof、正常 close/reopen
均不证明坏尾物理安全发布。也不重述受限 fault/ANN V4/campaign 为可运行项。

## 7 证据、排查、清理与通过标准

建议把新结果存于自己本次 `RUNROOT/evidence`，至少包含：完整 commit/tree、
archive/source 前后清单、二进制和构建来源 hash、精确 argv/CWD、明确指定的
五键 child env 与临时路径、主机/文件系统/资源记录、每步 UTC/stdout/stderr/exit、
具名 run/pass/fail/skip 数与断言、未覆盖项及复核人。只记本次指定值，不导出
真实完整环境、用户配置、秘密或生产数据。capture 记录函数名时还要保存本次
wrapper 定义和 `oracle-path-and-cwd.txt`，以便重建实际展开的 child argv/env。

- identity、source/hash、路径或 Linux admission 不符：停止；不能换系统 sqlite、
  改 expected、只保留 version 字符串或借旧候选 PASS
- source guard 拒绝：保存原始 stderr 与身份，重新审阅实际 delta；不删 guard/
  改标签/加豁免/更名绕过；此前未审阅文件不能用旧冻结 hash 自动盖章
- SKIP、空匹配、缺少 named event 或 source 变化：NOT_RUN/FAIL/待查分列；
  不凭 native exit 0、父测试 PASS 或外层 shell 成功补齐覆盖
- 临时目录不可用、资源不足、hang、未知退出/Close 或意外输入：停止后续启动，
  保留现场与日志；不强杀、不重试数据库操作、不删除残留求 PASS
- 父 shell、compiler 或 loader 不可信：清洁 SQLite Env 不解决这个问题；应停在
  前置审查，而不是宣称已建立完全隔离

全部进程自然结束、资源退休明确、原始失败与成功证据已归档且读回校验后，才
由用户分别决定是否删除自己创建的 RUNROOT 和 ORACLE_TMP。失败、不确定或
不属于本次的任何路径都保留；没有自动清理、`rm -rf`、namespace sweep 或
缓存清空步骤。Go t.TempDir 自身可能删除普通失败 fixture，日志保存不保证完整
现场仍在；物理恢复 harness 的保留规则是另一份规程，不因本文改变。

允许的结果名是按精确身份区分的 SOURCE_REVIEW、SOURCE_MODEL、FOCUSED_NATIVE、
FULL_ORDINARY、ORACLE、RESTORE、CI。只有对应真实执行与全部断言/日志齐全
才能给那一栏有限 PASS；startup 独立源码/纯模型/AST 的历史 PASS 保留。904ca
与 209ba 两轮均 guard PASS、普通门 FAIL、其余阶段 NOT_RUN；52b 两 quick / 全
15 门已有精确本地 PASS，含实际 oracle。新提交、精确恢复和新 CI 尚无结果，
不将这一开发范围的成功写成物理持久性、所有原生平台或 M10 通过。
已保存证据与归档只证明原身份，不证明未来组合。M9 实现继续，M10 未验收，
没有完整 SQLite 兼容、原生平台、物理持久性、加密安全或生产就绪的总通过结论。
