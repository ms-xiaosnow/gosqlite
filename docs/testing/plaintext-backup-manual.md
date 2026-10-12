# 明文 Backup 手动资格验证

> 2026-10-12 当前组合状态：Storage metadata 修正树 `7389e7b7` 的三条回归
> 已实际 PASS（14 根 / 92 named PASS），UNIQUE 树 `fd4e3471` 的十二条有限门
> 已实际 PASS（31 根 / 223 named PASS / 零 SKIP）；两者均自然退出并释放 lease。
> 本文所在新 Storage + UNIQUE 组合仍是 **SOURCE_ONLY / NOT_RUN**：完整 ordinary、
> race、oracle、跨编译/依赖、发布/精确恢复/对应 CI 尚未运行。下文旧的
> SOURCE_ONLY、PENDING、FAIL 与 NOT_RUN 均保留为各自身份的历史，不覆盖上述有限结果。
> 原 V1 显示契约 FAIL 与 V2 metadata 真失败（8,126 PASS / 1 FAIL / 7 SKIP；
> 后续 19 NOT_RUN）不改写。native Backup（包括 UNIQUE 数据库）、WAL publisher
> 和物理资格仍 **NOT_RUN**；完整目标与 M10 未验收。详见[新检查点证据](../evidence/storage-unique-checkpoint-20261012.md)。

## 2026-10-11 当前资格补记

这六个 native 手工入口仍为 **NOT_RUN**，Backup oracle 行为与存储资格也仍为
**NOT_RUN**。它们的源码已经存在，完整物理故障/恢复/断电设施仍为
**IMPLEMENTATION_MISSING**；两者不得混为“只差运行”。

输入树 `ec9f30d26d71085791197cd8e25abed558ad236d` 的 36 条有限命令已经 PASS：
44 根 / 2,278 named PASS，另含源码 guard、12 条平台/tag 选择、8 条仅编译与
8 条符号读取。仅编译产物未运行；有限 CLI 范围中的两个普通加密 fixture 根也
不构成这里的明文 Backup 原生资格。组合树 `2602cc60e8c82e3c7f900eb44439e4217cb0b8c3`
的完整普通 26 门 V2 在 ordinary 门 **FAIL**、后续 19 门 NOT_RUN；修正后新树/
完整复验 **PENDING**，不能继承输入树结果。

本补记未新增或执行任何命令，下面全部命令、flags、selector、manifest schema、
隔离/留证/人工清理要求逐字保留。原段落所称 guard/选择/compile NOT_RUN 是首次
准备时点；上面的有限结果只更新对应精确输入树，不代表本文整套规程已经执行。
完整身份、失败与其余缺口见[Storage 证据补记](../evidence/storage-checkpoint-20261011.md)。

### 历史首次准备说明（原文保留）

状态：SOURCE_ONLY / NATIVE_NOT_RUN / ORACLE_NOT_RUN。本文配套入口与有界 manifest loader 已有源码，未运行 Go、格式器、parser、compiler、test、oracle 或项目脚本。未来执行须另行审查批准；以下命令是完整方案，不是已执行记录。完整物理故障 harness 为 IMPLEMENTATION_MISSING。

## 1. 范围与固定入口

三文件保留六个原名及全部原断言：

| binary | 精确顶层 selector | 成功叶数 | Backup 数 |
|---|---|---:|---:|
| sqlite-backup-manual.test | ^TestPlaintextBackupOrdinaryRoundTrip$ | 3 | 3 |
| sqlite-backup-manual.test | ^TestPlaintextBackupExistingDestinationPreflight$ | 1 | 1 |
| sqlite-backup-manual.test | ^TestPlaintextBackupPinnedReadOnlyAndConcurrentReaders$ | 1 | 2 |
| sqlite-backup-manual.test | ^TestPlaintextBackupEmptyAndLazyReadOnly$ | 1 | 2 |
| cli-backup-manual.test | ^TestCLIPlainBackupOrdinaryRoundTrip$ | 1 | 1 |
| sqlite-backup-manual-oracle.test | ^TestPlaintextBackupOracleOrdinaryInteroperability$ | 3 | 3 |

oracle 另有 identity 一次、每个页尺寸 create/readonly 各一次，共七次真实进程启动；identity 不是免费探测。10 个叶用例、12 次 Backup；CLI 仍调用真实生产入口，两个并发 Backup 仍为两个 goroutine。三页尺寸仍为 512/4096/65536。第一个失败立即停止后续页尺寸或 native case。

默认 Linux 五项、仅 oracle tag 六项的选择差异只相对未发布 `fc0b1324441456f40160e81d5125dc7b97992773` 候选，已发布 `67318ded6416547f26d685719d3e01e208454a87` 原无这三文件。这是保留手动覆盖，不是从新 main CI 删除六项旧测试。

## 2. 先确认环境，禁止回退

每个顶层测试使用独立、不复用、已有的私有 disposable root；子目录 work、tmp、fixtures、evidence 事先存在，owner 为运行 UID，mode 0700，无 symlink。必须是 Linux ext-family magic 0xef53，statx 实际返回 TYPE/NLINK/INO/SIZE/MNT_ID_UNIQUE，全路径同设备与同 unique mount ID；至少 256 MiB 可用及 1000 空闲 inode。魔数不能证明 ext4 版本、mount/storage 的可信锁和断电耐久。

外部先审查稳定可信命名空间、没有重挂载/替换/直接 sidecar 写者、兼容锁/存储；0700 不证明同 UID 或特权 actor 不存在。测试 metadata gate 不能自行证成这些前提。Openat2 的 required resolve/create flags 与 no-replace rename 只由实际正常 Backup 路径验证，helper 不发隐藏 smoke Backup。

root 是明确 clean absolute Linux 路径，拒绝 `/`、`/tmp`、相对/脏路径。CWD 必须恰为 root/work；GOTMPDIR 与 TMPDIR 必须恰为 root/tmp。所有 fixture、临时目录和 evidence 在 root 内。五项非 oracle 可用明确的其他合格绝对路径；oracle root 额外必须为真实 `/tmp/...` 合格目录。tmpfs/overlay 不合格；不寻找另一盘，不 mount/bind，不改 factory，也不把 `/mnt/...` 的 TMP 悄悄指回系统 `/tmp`。

ForTest 固定 child 环境仅 LC_ALL=C、LANG=C、TZ=UTC、SQLITE_TMPDIR=root/tmp、TMPDIR=root/tmp；前置 `-noinit`，没有 HOME/XDG/PATH。这里不是 OS 级沙盒：SQLite 在所选 temp 不可用时仍可能 fallback。必须保证 temp 持续真实可写/容量/稳定，且 CWD 在 root；若需要异常时绝对阻止其他路径，继续 BLOCKED，另行批准机制。

## 3. 先冻结，再编译，最后准备运行 manifest

固定新合成源树及完整 file/mode inventory，不能把旧 fc0b 或 main HEAD 充作新树身份。固定 Go 1.27.2 的 VERSION、实际 go/compiler/linker/assembler 文件摘要；module source/cache inventory 与 go.mod/go.sum；oracle 的官方来源、构建记录、绝对 binary 和 SHA256。禁止下载/自动更新、启动 `go env`/`go list -m` 或先执行 oracle --version/help/identity 来发现来源。

三个独立 build 产物只在另行批准后生成；SOURCE、GOROOT、BUILD、GOPATH、MODCACHE 是审批时必须填入并验证的明确路径。所有缓存/temp/output 是已批准目录，没有项目脚本。每次编译的实际 argv/env/CWD、输出、自然退出和产物 hash 保存到源码树外。

```sh
cd "$SOURCE"
/usr/bin/env -i PATH="$GOROOT/bin" GOROOT="$GOROOT" GOPATH="$GOPATH" \
  GOOS=linux GOARCH=amd64 GOAMD64=v1 CGO_ENABLED=0 GOMAXPROCS=2 \
  GOTOOLCHAIN=local GOENV=off GOWORK=off GOFLAGS= GOPROXY=off GOSUMDB=off \
  GOMODCACHE="$MODCACHE" GOCACHE="$BUILD/cache" GOTMPDIR="$BUILD/tmp" TMPDIR="$BUILD/tmp" \
  "$GOROOT/bin/go" test -c -mod=readonly -buildvcs=false -p=1 \
  -tags manual_backup_publication -o "$BUILD/sqlite-backup-manual.test" ./sqlite

/usr/bin/env -i PATH="$GOROOT/bin" GOROOT="$GOROOT" GOPATH="$GOPATH" \
  GOOS=linux GOARCH=amd64 GOAMD64=v1 CGO_ENABLED=0 GOMAXPROCS=2 \
  GOTOOLCHAIN=local GOENV=off GOWORK=off GOFLAGS= GOPROXY=off GOSUMDB=off \
  GOMODCACHE="$MODCACHE" GOCACHE="$BUILD/cache" GOTMPDIR="$BUILD/tmp" TMPDIR="$BUILD/tmp" \
  "$GOROOT/bin/go" test -c -mod=readonly -buildvcs=false -p=1 \
  -tags manual_backup_publication -o "$BUILD/cli-backup-manual.test" ./cmd/gosqlite

/usr/bin/env -i PATH="$GOROOT/bin" GOROOT="$GOROOT" GOPATH="$GOPATH" \
  GOOS=linux GOARCH=amd64 GOAMD64=v1 CGO_ENABLED=0 GOMAXPROCS=2 \
  GOTOOLCHAIN=local GOENV=off GOWORK=off GOFLAGS= GOPROXY=off GOSUMDB=off \
  GOMODCACHE="$MODCACHE" GOCACHE="$BUILD/cache" GOTMPDIR="$BUILD/tmp" TMPDIR="$BUILD/tmp" \
  "$GOROOT/bin/go" test -c -mod=readonly -buildvcs=false -p=1 \
  -tags oracle,manual_backup_publication -o "$BUILD/sqlite-backup-manual-oracle.test" ./sqlite
```

这些是 linux/amd64 的未来命令模板，所有变量须替换成已核对路径。arm64 需单独完整批准、对应 target env/pins，不能直接沿用本表。-run 不隔离编译/初始化；必须先审全 package/import/init closure。上述是 test -c，不执行测试 binary。不直接 go test manual 套件，不执行 ./... 聚合脚本。

## 4. 旁置 manifest 的实际手工创建步骤

没有 prepare-manifest.sh，也没有测试内自动发现器。按以下真实编辑/静态文件操作完成；没有完备输入就 BLOCKED，不填虚构 hash。

1. 在源码树外保存审查后的 `source-inventory.json`（含真实新 source tree/全 path/hash/mode）、`toolchain-inventory.json`（VERSION/发行包/实际工具 hash）、`module-inventory.json`（三模块声明、source/cache 及归档 pin）。把这三份已经核验的文件和准确 go.mod/go.sum 副本放到该 root/evidence 的同名位置。loader 只 hash 五个固定文件，不把 inventory 内容当动态路径列表。无现成完整 inventory 时先完成独立静态审查，不假装 collector 已运行。
2. 编译自然退出后，用只读文件工具对明确产物及上述五文件计算 byte size 与 SHA256，例如 `wc -c < '/ABS/EXACT_FILE'` 和 `sha256sum '/ABS/EXACT_FILE'`。它们读取文件，不启动该 binary。oracle 额外从独立来源/构建证据核对官方 3.54.0/source ID。
3. 用普通文本编辑器新建 root/evidence/input-manifest.json，schema 恰为下表，所有值来自已保存审批与实际编译记录。每个 root 对应一个 selector；非 oracle 的 oracle 字段必须明确为 null。不要省略字段、写重复 key、写任意环境变量或秘密。
4. 保存 JSON 后静态计算其原始 bytes 的 SHA256，仅将 64 位小写 hex 写入 input-manifest.sha256，可有一个末尾换行。不要写文件名，不把 digest 填回自身 JSON，不回写 source 或 test binary。将该 digest 另保存在审查记录中；同目录 digest 只能发现意外损坏，不能抵御同时修改二者的 actor。
5. 执行前审核原文件/JSON/digest、binary 路径稳定、root 资格与准确命令。manifest 是 declared_input，不是授权文件，也不证明 loaded image。

每个 filePin 恰为 `{"path":"已批准绝对路径","size":实际正整数,"sha256":"64位小写hex"}`。JSON 键及值：

| 键 | 必填内容 |
|---|---|
| schema | 1 |
| root | 本次明确 root |
| profile | linux-ext-family-unique-mount-openat2-noreplace-v1 |
| namespace | stable-private-disposable-no-replacement-no-remount-compatible-locks-storage |
| source_tree | 静态冻结的新 Git tree 40 位小写 hex |
| source_inventory | root/evidence/source-inventory.json 的 filePin |
| toolchain_inventory | root/evidence/toolchain-inventory.json 的 filePin |
| module_inventory | root/evidence/module-inventory.json 的 filePin |
| go_mod | root/evidence/go.mod 的 filePin |
| go_sum | root/evidence/go.sum 的 filePin |
| go_version | go1.27.2 |
| goarch | amd64（对应上述方案） |
| modules | 三个对象，字段 path/version/sum，见下表 |
| build_tags | manual_backup_publication 或 oracle,manual_backup_publication，原顺序 |
| build_argv | 实际 Go argv 数组，首元素为绝对 go，含 test/-c/-mod=readonly/-buildvcs=false/-p=1/-tags/标签/-o/产物/单 package |
| build_env | 上述 env -i 后全部明确 build key/value；无其他 key |
| runtime_argv | 长度 9 的准确 argv：binary、-test.run、selector、-test.v、-test.count=1、-test.failfast、-test.parallel=1、-test.shuffle=off、-test.timeout=0 |
| runtime_env | 第 5 节八个固定 key/value；oracle 再加 SQLITE_ORACLE |
| selector | 表中一个精确锚定 selector |
| executable | 本次实际 test binary 的 filePin |
| oracle | 五项为 null；第六为 {binary:filePin,version:"3.54.0",source_id:下面准确字符串} |

| path | version | sum |
|---|---|---|
| golang.org/x/crypto | v0.58.0 | h1:COkYLr4k7nDI4r6QtMML2AfzeIswWHAJthLS6K/M/54= |
| golang.org/x/sys | v0.49.0 | h1:XbzkgYJHdqh/8m2Uu0W/dQv8nktxx4BFHp1M0gROTXA= |
| golang.org/x/text | v0.43.0 | h1:1QivrlhwAsnMOcqgOdl9my7s9OyZl8O5/UGUTJVXoWc= |

oracle source_id 为 `2026-10-09 15:46:58 be8d059e9a49089ab2dce5ed26dd87aaf598fdc5fbe0b107c0dd758464bcd5e3`。这些是固定声明值，非本次 oracle 运行发现的值。h1 与原始归档 SHA256 不是同一种摘要。

loader 的实际限额：manifest 256 KiB、JSON depth 16/12000 个递归 value（含容器）/string 4096 bytes/key 256 bytes、完整 case plan 1 MiB、单 case event evidence 16 MiB；source/toolchain/module inventory 分别 8/16/32 MiB，go.mod 64 KiB、go.sum 1 MiB；binary 512 MiB，流式 hash。拒绝未知/重复 key、多个值、缺字段、重复 module、非法 hash/size、runtime 选择或 metadata 冲突；不自动扩容、不更新 expected。

R-1 精确 schema 修复：先验证原始 manifest digest，再调用无文件描述符的 `decodeManifest`，其有界 token 检查和每层精确 schema 检查全部完成后才做 typed struct decode，随后才读取其他 sidecar。仅 `DisallowUnknownFields` 不能拒绝 Go JSON 的大小写折叠别名。现在顶层 21 键、每个 filePin/module/oracle 对象及两种 env map 都按大小写敏感允许集核对全部必需键；`source_tree`/`SOURCE_TREE`、`path`/`PATH` 无论值相同与否、顺序如何都拒绝。转义后相同的重复键也拒绝，不规范化、不取最后值、不忽略警告。除了明确非 oracle 的 `oracle:null`，对象/数组/字符串/整数均不接受 null 或其他错误 shape。modules 的纯 schema 长度为 1–16、两个 argv 数组为 1–32，随后原有语义检查仍要求本方案准确三模块/argv。build_env 有 17 个必需键及唯一可选 GOAMD64（amd64 时必需）；runtime_env 非 oracle 恰为八键，oracle 恰为九键，两者不混用允许集。

### 4.1 纯内存 schema 回归的独立未来命令

状态仍为 PROPOSED / NOT_RUN。便携 `manifest_schema.go` 与 `manifest_schema_test.go` 无 build tag、init、包级可变状态或 native import；默认只选择这两个文件，不选择 `manual_linux.go`。四个精确测试名合计 1,983 个有限叶项（14 / 891 / 562 / 516），直接调用真实纯 decoder 或其 token 边界检查，覆盖规范 non-oracle/oracle、14 对象位置/81 字段位置的大小写别名、缺键/未知键/错误 shape、重复/转义等价键和准确 quota 边界。fixture 仅是内存合成值，不是可运行 manifest 或文件。没有 ForTest、环境/文件读写、DB、oracle、并发或 fuzz campaign。其通过不能证明 native gating、物理存储或完整资格。

先另行审查默认包及标准库闭包、固定 Go 1.27.2/源码/缓存目录，再批准以下单独命令；这不是前面三个 `test -c` 的隐含执行授权。SOURCE、GOROOT、GOPATH、MODCACHE、PUREBUILD 都须为审批中确认的绝对路径，PUREBUILD/cache 与 PUREBUILD/tmp 为独立已批准目录，源码树外保存实际 argv/env、完整 stdout/stderr、精确 case 数及自然退出状态。

```sh
cd "$SOURCE"
/usr/bin/env -i PATH="$GOROOT/bin" GOROOT="$GOROOT" GOPATH="$GOPATH" \
  GOOS=linux GOARCH=amd64 GOAMD64=v1 CGO_ENABLED=0 GOMAXPROCS=2 \
  GOTOOLCHAIN=local GOENV=off GOWORK=off GOFLAGS= GOPROXY=off GOSUMDB=off \
  GOMODCACHE="$MODCACHE" GOCACHE="$PUREBUILD/cache" GOTMPDIR="$PUREBUILD/tmp" TMPDIR="$PUREBUILD/tmp" \
  "$GOROOT/bin/go" test -mod=readonly -buildvcs=false -p=1 -count=1 -parallel=1 -shuffle=off -timeout=0 -v \
  -run '^TestManualBackupManifest(ExactSchema|RejectsCaseAliases|RejectsMissingAndUnknownFields|RejectsDuplicateAndUnboundedJSON)$' \
  ./internal/testkit/backupmanual
```

不带 manual/oracle tag 或 acknowledgement，不运行 native binary。期望四根及全部有限叶项 PASS、零 Skip/FAIL/no-tests-to-run；任一失败或无法自然退出立即停止后续计划并保存证据，不扩大 selector 或转跑 manual 套件。本次没有运行该命令或 Go AST guard。

进程只观察 runtime.Version、白名单 BuildInfo settings/Main/Deps、os.Executable 路径文件的有界 hash/CWD/env/root facts。BuildInfo 可能缺 VCS，-buildvcs=false 明确关闭采集；VCS revision 从来不是 source tree。os.Executable 和 `/proc/self/exe` 路径语义不能无条件证明当前加载映像未被替换；这里依赖稳定可信 binary 命名空间。缺必要 metadata 就非零 BLOCKED，不启动 go/git/python/oracle 修补。没有全环境、HOME、XDG、sqliterc 或 credentials 读取。

## 5. 六次独立手动运行

每次仅一个顶层 entry 独立进程，ROOT_1…ROOT_6 必须不同，且有各自 manifest。SELECTOR 与 BINARY 按第 1 节逐项填写；不允许 regex alternation 或宽匹配。示例展示第一项完整 argv/env；其他四项只替换独立 root/binary/selector。第六项再加 SQLITE_ORACLE，root 必须在已合格 `/tmp/...`。

```sh
R="$ROOT_1"
(cd "$R/work" && /usr/bin/env -i LC_ALL=C LANG=C TZ=UTC GOMAXPROCS=2 \
  GOSQLITE_MANUAL_BACKUP_PUBLICATION=1 GOSQLITE_MANUAL_BACKUP_ROOT="$R" \
  GOTMPDIR="$R/tmp" TMPDIR="$R/tmp" \
  "$BUILD/sqlite-backup-manual.test" \
  -test.run '^TestPlaintextBackupOrdinaryRoundTrip$' \
  -test.v -test.count=1 -test.failfast -test.parallel=1 -test.shuffle=off -test.timeout=0)

R="$ROOT_6"
(cd "$R/work" && /usr/bin/env -i LC_ALL=C LANG=C TZ=UTC GOMAXPROCS=2 \
  GOSQLITE_MANUAL_BACKUP_PUBLICATION=1 GOSQLITE_MANUAL_BACKUP_ROOT="$R" \
  GOTMPDIR="$R/tmp" TMPDIR="$R/tmp" SQLITE_ORACLE="$ORACLE" \
  "$BUILD/sqlite-backup-manual-oracle.test" \
  -test.run '^TestPlaintextBackupOracleOrdinaryInteroperability$' \
  -test.v -test.count=1 -test.failfast -test.parallel=1 -test.shuffle=off -test.timeout=0)
```

不要加 `|| true`，不要失败换 root 再试。必须另行指定已有受审的外部 stdout/stderr 接收方式、磁盘/日志预算和观察负责人；本文没有伪造一个已存在的外部 runner。没有这个完整执行计划就不运行。test.timeout=0 不发超时 panic/kill；固定数据/操作量是有限的，但不保证墙钟有限。若挂起，记录 BLOCKED 并请求下一步，不能自行 signal/kill/重启/恢复。保持原进程和 fixture。

oracle 的合并 stdout/stderr writer 消费完整流，保留至多 1 MiB 输出、完整总数与 SHA256；超额仍消费直到自然 Wait，不通过 writer error 终止 child，最终 OUTPUT_LIMIT 非 PASS。正常原预期输出完全保留。没有伪装成已保留完整异常大输出。

## 6. 证据与通过标准

每次 root/evidence/observed-runtime.json 用 O_EXCL 新建；既有文件或 case 目录拒绝，避免复用旧 root。case 在 root/fixtures/<序号-label>，evidence 在 root/evidence/<序号-label>；source/data 目录不塞日志，保留原 sourceDir len==1 断言。完整 SQL+换行、每个绑定类型/原值（BLOB 全 base64）、Options、路径、原期望与操作顺序在任何 DB/oracle 之前写入 input-plan.json；随后 events.jsonl 保存实际结果与 owner 状态。

PASS 必须：全部原 SQL/页尺寸/完整值、bytes.Equal、Schema DeepEqual、完整 Check、查询 DeepEqual、source 不变与 sidecar 条件、Tx snapshot/并发/Autocommit、empty/lazy、CLI JSON/credential fake callbacks/check/query、oracle 身份/metadata/readonly/精确输出断言实际执行并成功；所有 DB/Tx/Rows/metadata FD 的正常结束明确成功；无 pending/unknown；进程自然退出 0，实际精确 entry/leaf 数完整，无 Skip、无 no-tests-to-run。通过也保留原 fixture。源码 guard/compile/list 通过不等同这些 native 事实。

每个 owner 为 LIVE → END_ATTEMPTED → ENDED_OK；结束前就标记，错误变 RETAINED_UNKNOWN。Tx Commit 失败不 Rollback；Rows Next(false) 可能已内部 close，Err 非 nil 不追加 Close；任何未知 child 阻止父 DB.Close。失败后强引用保留原 DB/Tx/Rows/FD/error，冻结整个进程，后续 case 不启动。cleanup callback 只冻结/记录已知路径，不 Close/Remove/快照。自然退出时 OS 仍释放句柄，不声称退出后永久持锁。

MANUAL_SELECTION_REFUSED、PROFILE_UNAVAILABLE、BLOCKED、FAIL/CLEANUP_BLOCKED 都不是 PASS。发生 Open/Query/Check/Backup/Commit/Close/证据 I/O 或断言失败，不重新读取未知 DB 来确认，不重做 Backup，不换目标，不自动 RetryPlaintextCleanup/归档/删除/恢复。保留已经形成的 source/target/private candidate/sidecars。错误记录对未知图只保留强引用与类型，不调用任意 Error/Unwrap 递归；clean existing-target 只接受受审有限 producer 形状，拒绝 pending/poison。

## 7. 用户人工清理

1. 确认该测试与全部 oracle 子进程已经自然退出，保存确切退出状态；未知/仍活进程不清理，不为清理而 kill。
2. 对照 manifest/日志核对准确 absolute case 目录，仅有 synthetic fixture，没有用户库、共享资料、mount/链接或其他使用者。
3. 用户保存所需完整输入/输出与完整原 fixture（含候选/源/目标/sidecar），验证保存可读；失败/unknown 默认继续保留，不能只保存日志就删唯一实物。
4. 用户明确确认该 scope 可删除后自行逐项处理。可在文件管理器删除；命令仅为未来人工示例：`rm -ri -- '/ABS/APPROVED_ROOT/fixtures/EXACT_RECORDED_CASE_DIRECTORY'`。不删整个 ROOT，不通配符，不 -f，不 sudo，不 exit trap。
5. 保留 evidence/manifest，记录 case 与清理时间。身份、内容、权限或删除错误立即停止；其他 case/tmp/work/evidence 另行核对，不批量递归 root。

helper 不自动 unlink/RemoveAll/rename/snapshot-copy；保留文件不是 CLEANUP_BLOCKED。该取舍避免引入第二套 namespace 删除/快照协议，代价是磁盘持续占用到用户清理。

## 8. 仍是真实缺口

六例只验证完整普通 rollback 库与 clean existing-target 前置拒绝，不完成物理故障矩阵。Native WAL、readonly WAL、非 Linux Backup 支持仍是真实缺口；更广 FS/architecture/mount/storage 资格、锁和同 final cross-process 竞争、每个 I/O/Sync/no-replace/retirement 失败、short read/write/disk full、missing statx/Openat2 flags、未知 owner、namespace aliases/remount/替换、崩溃/断电/恢复/长期 soak/race、加密 backup/export/rekey/WAL 机密性、quota/cancel 与发布审计等仍需独立 harness/授权。

完整 fault/recovery/power-loss harness：IMPLEMENTATION_MISSING；native/oracle/storage qualification：NOT_RUN；本次 guard/选择/compile：NOT_RUN。旧 V2 的 34 个无描述符/AST PASS 不能替代上述缺口。本次不恢复 ANN V4、41-target campaign 或未知中断链，不修改挂载/factory/CI/远端，不宣称整个 goal 完成。
