# 当前集成说明：Darwin managed-directory 有限手工 harness

## 当前状态入口（2026-10-12）

本文件旧段落保留各自写作时点的源码、FAIL/HOLD/UNKNOWN 与 NOT_RUN；其中“当前”“缺失”“待合入”或 PENDING 不能覆盖[当前源码/证据矩阵](../current-status-20261012.md)。已保存 main8bc 的发布、精确恢复与 CI 已闭合；本次 LEFT/inspect/Darwin/Connector/automatic UNIQUE 组合仅 SOURCE_ONLY；前序 Rows pin 已修正，新增 sqlite.go 的 WAL-tail 完整源 pin 经独立完整 ownership 复审在本后继窄修，automatic UNIQUE 原 oracle FAIL 及 d79 HOLD 保留，已接入独立审阅的空结果表头修复，组合运行仍全部 NOT_RUN。输入切片的有限成功不转授本组合，原生手工/物理门不变。

以下第一个“Darwin Unicode 同级清单”标题开始的全文，是 sealed manual578 的
原始 V2 手册，完整保留其命令和历史表述；“V2 尚待复审”、旧 main 及
“尚未编译”等仅是原封存时点状态。原 V2 后续独立源码结论已为
`SOURCE_REVIEW_ACCEPTED_R1_CLOSED`，V1 HOLD（R1）原记录继续保留。


## 当前 Storage 后继层：源码集成状态（2026-10-11）

以上 V1/旧树状态为原始历史记录，原文完整保留。当前另建
`darwin-manual-storage-source-v1`，在冻结 Storage tree
`2602cc60e8c82e3c7f900eb44439e4217cb0b8c3` 之上只集成历史七路径 hunk，
并在独立审阅完整组合 `internal/testscope/main.go` 后单独更新 WAL 守卫的
该文件 pin，合计五个源码、三个文档路径。原生文件和两个专用 guard 文件
保持已封存 V2 的完整字节；V2 tree `578c30fbf9f6e360943265a3208b25c8158dfcef`
的历史独立结论为 `SOURCE_REVIEW_ACCEPTED_R1_CLOSED`，不能移作新树执行结果。
V1 HOLD、旧“待复审”及所有旧执行结论均保持其原时点含义。

本层是未发布的未来源码候选，不替换仍由独立执行流程拥有的 Storage 基线，
也不改变已发布 `f189746671aaf4962a699292a84110e2d43387e8` 的身份。
最终全树、逐文件 SHA-256/字节/Git 与 POSIX mode、完整 main 审阅及 pin 决策
见同批封存的 `darwin-manual-storage-integration-evidence-v1`；树身份不嵌入
自身文件而形成循环。本次 Go/parser/gofmt/list/编译/test/nm/guard/native
均 NOT_RUN。未来只有 9 次 go-list、5 次 compile-only 和 5 次 nm 的 19 项
有限静态提案，仍须新树独立审阅、明确审批及队列准入；没有执行许可。

所有当前 Backup/WAL 接线、ordinary/manual exclusions、namespace/recovery
政策、Windows rekey/key-session/provider/native 行为与文档均保留，不做生产
依赖回移。原始 native 手册命令属于历史模板；后续提案已由独立 addendum
纠正：保留既有 go-cache、实际 go-path/pkg/mod 与默认 /tmp，TMPDIR/TMP/TEMP/
GOTMPDIR 不设值；只新建证据输出，不增加 timeout 或 go version -m 探测，
资源门限不降低。不得直接执行旧模板或据此启动 recorder/OS runner。

双 tag/env 门、用户明确私有隔离 root、euid/0700/无 symlink、真实本地 APFS
或 HFS subtype 0–3、初始零 group/lease、固定 ASCII 家族 basename `k.gse`、
checked-once child-before-parent owner 退休以及成功失败 fixture 保留不变。
root 校验并不禁止 UTF-8 父目录；任何未来执行授权可另要求 ASCII 私有目录
拼写，不能把它写成已有源码检查。无 rename/tamper/fault/recovery、权限扩张、
新 OS runner 或自动清理。有限目录分类/计费不是加密格式、durability、完整
Darwin/native、安全、M3/M8/M10 或其他受阻 campaign 的通过。

## 以下为原封存 V2 手册全文（历史模板，不是当前执行许可）

# Darwin Unicode 同级清单：独立原生手工规程

日期：2026-10-11。候选：`darwin-inventory-manual-work-v2`。
**本次只写源码与文档：Go/parser/gofmt/编译/测试/原生/项目脚本均 NOT_RUN。**
当前环境没有可用原生 Darwin 硬件/OS。本页命令是未来独立用户模板，未执行，
不能作为代理运行、改换环境、恢复既有受阻链路或扩大原生权限的授权。

V2 有限源码修正：V1 tree `60d41d9e3c28b3e5eba741583573653617568ca6` 的
独立源码审阅为 **HOLD（R1）**，原记录不覆盖。初始 known 健康检查不能单独
证明 registry 为空；V2 在建立 ledger/defer 和任何文件系统动作前，持 registry
mutex 读取 groups/leases，任一非零即带计数 Fatal，不采用、关闭、reap 或重试
既有 owner；原后续 known 检查保留。新增惰性 AST 变体检查该门不能遗漏、弱化
或后移。此修正仍是 source-only，未运行 parser/guard/编译/原生，也不改变本页
未来命令或既有控制；V2 尚待独立源码复审。

## 1 身份与验收范围

- 冻结文档基线 tree `8a324a8cedeccc8ba46234924c8074890aefa1da`，3,157 文件、
  148 目录记录（含根目录）；其生产源码 tree 为
  `88c400a4caeb40d8d96fae7700e7048d69db4ff5`。
- 已发布 main 是 `3ea6d3ab15f678fe48bee312921da60828845869`，tree
  `f6edabfba3e004a0e7874441b8c0a3e38cf099c6`。它没有本入口，不能拿它运行本规程。
- 本候选仅有四个源码路径和三个文档路径；生产代码未改。候选最终全树身份、
  每文件 SHA-256、POSIX mode 和前后保全证据由独立封存 MANIFEST 给出，
  本文不把旧生产 tree 冒充当前组合 tree。执行前必须取得当前完整审阅源码。
- 原生文件 `internal/encstore/inventory_manual_darwin_test.go` 的 SHA-256：
  `af6d46be245a224648be56be4bbe2712e35464e8015ce805254ace9a3fdf80ce`。guard 固定这一完整字节身份；不能仅为通过 guard 而更新 pin。
- 新普通 guard：`internal/testscope/darwin_inventory_manual.go` 及 `_test.go`；
  `main.go` 只接入该独立合同检查，没有扩大已有 manual exemption。

本 harness 只证明实际 managed 枚举目录上的有限名称分类、扫描项计数和家族
物理字节计费。不是加密数据库格式测试，不调用加密创建、密钥 provider、恢复、
rekey、备份、发布或 fault。真实加密/native 完整验收仍未闭合。

历史 `darwin-current-composed-independent-review-v1` 只审阅旧生产候选。
`darwin-current-composed-finite-verification-v2` 的 Linux/amd64 20 项 normal/race
PASS 只属于 `88c400...`，不能转给这个新 harness 或含它的新组合。
历史 baseline FAIL、V1/V2 HOLD、旧 V3 PASS 和初始工具提案 HOLD 保留。
M3/M8/M10、安全/密码学/法律独立审查、任意 Unicode 根名、ANN V4、受阻
41-target campaign、未知中断及其他手工 tag/env 门均不改变。

## 2 唯一入口与四项失败关闭前置

唯一原生入口：`TestManualDarwinUnicodeInventory`。源文件第一行精确为：

```text
//go:build darwin && manual_darwin_inventory
```

它在普通 Linux、Windows、Darwin 无 manual tag 的选择中均不启用。
不能用 `manual_namespace`、`manual_recovery`、`-run .`、全包或全仓执行替代。

1. 函数第一项动作核对 `GOSQLITE_MANUAL_DARWIN_INVENTORY=1`；未满足直接
   `Fatal`，不 Skip，也不执行 fixture 动作。
2. 用户明确提供 `GOSQLITE_MANUAL_DARWIN_INVENTORY_ROOT`。必须是已存在、
   绝对且 clean 的独立可丢弃目录，非 `/`，路径不超过 1,024 字节；无默认目录。
3. root 实际 uid 必须等于 euid、mode 精确 0700，所有路径组件都必须是真目录，
   不允许 symlink；先 Lstat，再从保留 managed descriptor 核对 device/inode、
   ownership/mode。macOS `/tmp`、`/var` 等别名应使用用户核实的真实 `/private/...`
   拼写，不能改 harness 去跟随 symlink。已有 root 的权限、文件不会被改动。
4. 对实际保留 descriptor 的 Fstatfs 要求 `MNT_LOCAL`，精确 `apfs`，或者精确
   `hfs` 且 subtype 0、1、2、3。APFS 不套用 HFS subtype 上限。非本地、exFAT、
   SMB/FUSE、旧 HFS subtype 128、未知或无法读取者直接 Fatal，不 mount、修权限、
   换卷重试或降级。root 已关闭或 owner 失效同样失败关闭，不能当作支持。

还要求真实 Darwin amd64/arm64、固定 Go 1.27.2、外部已验证的完整 source tree
标识，以及初始空的 encstore ownership registry。每进程只能调用入口一次。
在前置全部满足前不会创建 fixture。读取 OS/kernel/build 元数据也可能失败，
失败必须保留证据并停止，不能跳过。`source_tree_external` 是用户外部核验后
提供的声明，不能单凭日志把它当成二进制自证的源码 tree。

前置满足后只在 root 下做一次 exclusive `mkdirat(...,0700)` 创建新子目录，
命名为 `inventory-<pid>-<nanoseconds>`；EEXIST 直接失败，不重新命名或重试。
再次核对新目录 private mode、uid、fsid，以及 root-relative device/inode。
它不会读取/清空 root 中旧文件，也不改 root 权限。初始路径选择与同 uid 恶意
OS 进程的主动替换不在该合作式 owner 模型的安全保证内；运行环境必须隔离。

## 3 四个 ordinary payload 与有限断言

四个样本是普通文件，只使用家族 basename 语法，没有加密数据库格式：

| 名称 | 固定字节 | 长度 | 预期 |
|---|---|---:|---|
| `k.gse` | `R01` | 3 | 家族根，计费 |
| `k.gse.gse-lock` | `L001` | 4 | 家族锁名，计费；不实际获取锁 |
| `k.gse.gse-` + 64 个 `a` + `.base` | `G0001` | 5 | 规范生成名，计费 |
| `notes-Ω.txt` | `N000001` | 7 | 无关同级，不打开计费，仍占扫描项 |

它们是普通元数据样本，非可打开的加密数据库；不得尝试 public Open、恢复或
用这些固定字节推断加密行为。四个固定 payload 的预期 SHA-256 如下，创建后和
清单检查后都应与同一值完全相等（12 是三个家族文件长度之和，不是第五个文件）：

- `R01`：`e0a64b0b6d837fa4edc328ab9ddea0e3e7e0e4f715304c1d6bf3d0adc9d5292a`
- `L001`：`668b2245fe89b82c883add46d887175478e14a1467e264169755830659ff4c4b`
- `G0001`：`e3cfff020d7ebf0ddd162d386644c8d84cf7304fc686f1c984773d56680bf0e3`
- `N000001`：`8eeb1704ff04b3c68d80f5ab109e1e8ad61e0cb0f2b60faf07834193f19b7a56`

所有文件 exclusive 创建，mode 0600，固定短写入一次，无 truncate。创建后及
清单检查后分别核对 regular/single-link、size 与 SHA-256。无真实秘密或密钥。

在同一个实际 retained enumerator 上 ReadDir，再把这个 owner 直接传给
`platformFamilyEntry`；其身份须与 fixture 目录相同，明确看到四项及 EOF。
另核对 Unicode sibling 配 nil owner 时得到 `ErrUnsupported`。

对 provided namespace 与 self-open enumerator 两条 route 分别做：
- maxBytes=12、maxEntries=4：成功且 total 精确 12，证明 7 字节 sibling 没计费
- maxBytes=11、maxEntries=4：total=0 且 `encryption.ErrLimit`
- maxBytes=12、maxEntries=3：total=0 且 `encryption.ErrLimit`，证明 sibling 消耗扫描项

没有 mount、chmod、remove/unlink、rename/link、kill、child process、故障注入，
没有 t.TempDir/自动清理。不是磁盘持久性、Unicode 等价全覆盖或完整加密接受性。

## 4 所有权、失败现场与日志

每次显式 acquisition 返回后，owner 先进入 ledger，随后才执行其他操作；
生产 constructors 内部仍先登记 registry，再做验证。目录创建前记录候选路径。
清理只关闭描述符，不移除任何文件。每个 consuming Close 先记 attempted，
最多一次，再分别核对 RetirementComplete 与 physical raw=nil/children=0。
已知子 owner 未退休、未知 registry resource、terminal/dependency barrier、
unknown Close 或 native child pin 存在时保留 parent，不重试、Drain、closeAll
或 RetryCleanup。生产 inventory 内部资源由现有 managed registry 登记；
每次 inventory 返回后核对不存在未记入 ledger 的遗留 owner，再做后续操作。

fatal 路径也执行 deferred retirement 检查。无法证明退休的资源在全局 ledger
及既有 registry 中保留至进程退出，并输出 RETAINED_OWNER、剩余 scopes/leases；
这不承诺失败后由外部工具强制回收未知 handle。进程自然结束不等于所有
Close 已经正确完成。日志中的成功退休断言也不能替代其他失败断言。

日志由测试框架输出到 fixture 外的 evidence 目录。harness 不创建旁路日志文件，
不打印整份环境、凭据、key 或任意用户数据。日志包含：实际 kernel release/
build、hw.machine、Go/GOOS/GOARCH、build module 信息、外部 source tree 声明、
实际 root/enumerator 的 device/inode/fsid/type/subtype/local、固定样本 digest、
11 个具名断言、每个 Close 的诊断，以及 retained fixture 路径。

## 5 未来用户：独立审批、固定依赖与干净副本

以下每一块必须先人工审阅并独立明确允许；当前全部 **NOT_RUN**。不得从本文
提取后整篇自动执行。授权应明确具体源码、工具、原生机器、卷、root、证据位置、
限额和停止规则。需要新审批时先停止；本页不是换机器绕过限制的途径。

准备官方来源且已独立核实的本机 Go 1.27.2。`go.mod` 固定：
`golang.org/x/crypto v0.58.0`、`golang.org/x/sys v0.49.0`、
`golang.org/x/text v0.43.0`。离线已准备且核验 go.sum 的 module cache；
不自动下载、不替换工具链、不联网退回其他依赖。预先核准内存/磁盘限额，
source/build-cache/tmp/evidence 至少各预留合理空间；空间不足停，不自动清理。

SOURCE_COMMIT/TREE 必须是已经独立审阅且包含七路径本候选的完整提交与 tree。
本候选未发布，故没有可直接填入的发布提交。不能将 3ea6d3ab、88c400 或
8a324a 的旧身份填在下面；未取得新完整审阅身份时记 BLOCKED。

```bash
# 未来独立用户模板，当前 NOT_RUN。全部路径由用户填写和核验。
SOURCE_REPO=/absolute/reviewed/gosqlite
GO_BIN=/absolute/reviewed/go1.27.2/bin/go
TOOLCHAIN_BIN=/absolute/reviewed/go1.27.2/bin
MODULE_CACHE=/absolute/verified/modules
GOPATH_ROOT=/absolute/verified/gopath
RUNROOT=/absolute/new-disposable-inventory-run
: "${SOURCE_COMMIT:?填写已审阅、包含新harness的完整commit}"
: "${SOURCE_TREE:?填写该commit的完整tree}"
: "${NATIVE_ARCH:?填写本机amd64或arm64}"
test -x "$GO_BIN" && test -d "$MODULE_CACHE" || exit 1
test ! -e "$RUNROOT" || exit 1
test -z "$(git -C "$SOURCE_REPO" status --porcelain)" || exit 1
test "$(git -C "$SOURCE_REPO" rev-parse "$SOURCE_COMMIT^{commit}")" = "$SOURCE_COMMIT" || exit 1
test "$(git -C "$SOURCE_REPO" rev-parse "$SOURCE_COMMIT^{tree}")" = "$SOURCE_TREE" || exit 1
umask 077
mkdir -m 700 "$RUNROOT" || exit 1
WORK="$RUNROOT/source"; EVIDENCE="$RUNROOT/evidence"; NATIVE_ROOT="$RUNROOT/native-root"
mkdir -m 700 "$WORK" "$EVIDENCE" "$NATIVE_ROOT" "$RUNROOT/tmp" "$RUNROOT/build-cache" || exit 1
git -C "$SOURCE_REPO" archive "$SOURCE_COMMIT" > "$EVIDENCE/source.tar" || exit 1
tar -xf "$EVIDENCE/source.tar" -C "$WORK" || exit 1
printf '%s\n' "$SOURCE_COMMIT" > "$EVIDENCE/source-commit.txt" || exit 1
printf '%s\n' "$SOURCE_TREE" > "$EVIDENCE/source-tree.txt" || exit 1
cd "$WORK" || exit 1
```

这里 mkdir 只作用于确认不存在的新目录，没有 chmod/修改旧 root 权限。用户
明确选择的 NATIVE_ROOT 会传给 harness；计数 fixture 是它下面后来新建的
独占子目录。evidence/source/tmp/cache 都在 counted fixture 之外。
Git archive 只证明提交内的文件与 executable 位，不证明原 POSIX 私有 mode、
owner 或 ACL；新目录实际权限由原生检查核对。核实路径无 symlink、自己拥有
本机及这些目录，且没有并发写入者，再继续。不要把用户日常目录用作 RUNROOT。

下面的白名单只用于当前新隔离环境，不修改任何既有缓存/配置，不传 HOME、
用户 GOFLAGS、SQLITE_ORACLE 或其他 manual gate。完整函数和填入的非秘密
路径保存成 `procedure.txt`；command 日志只记函数名时不能自证白名单。

```bash
# 当前 NOT_RUN。日志文件只允许新建；任何非零即停止。
capture_inventory() {
  local name=$1 rc
  shift
  test ! -e "$EVIDENCE/$name.stdout" || return 1
  printf '%q ' "$@" > "$EVIDENCE/$name.command" || return 1
  printf '\n' >> "$EVIDENCE/$name.command" || return 1
  date -u +%Y-%m-%dT%H:%M:%SZ > "$EVIDENCE/$name.start-utc" || return 1
  "$@" > "$EVIDENCE/$name.stdout" 2> "$EVIDENCE/$name.stderr"
  rc=$?
  printf '%s\n' "$rc" > "$EVIDENCE/$name.exit" || return 1
  date -u +%Y-%m-%dT%H:%M:%SZ > "$EVIDENCE/$name.end-utc" || return 1
  return "$rc"
}
inventory_go() {
  env -i PATH="$TOOLCHAIN_BIN:/usr/bin:/bin" \
    GOPATH="$GOPATH_ROOT" GOMODCACHE="$MODULE_CACHE" GOCACHE="$RUNROOT/build-cache" \
    TMPDIR="$RUNROOT/tmp" GOENV=off GOWORK=off GOTOOLCHAIN=local \
    GOPROXY=off GOSUMDB=off GO111MODULE=on CGO_ENABLED=0 \
    GOOS=darwin GOARCH="$NATIVE_ARCH" GOMAXPROCS=2 \
    GOFLAGS= GOEXPERIMENT= GODEBUG= GORACE= GOCACHEPROG= GOCOVERDIR= GOFIPS140=off \
    LC_ALL=C LANG=C TZ=UTC "$GO_BIN" "$@"
}
capture_inventory version inventory_go version || exit 1
capture_inventory host inventory_go env GOVERSION GOHOSTOS GOHOSTARCH GOOS GOARCH CGO_ENABLED || exit 1
# 人工核对 go1.27.2、真实 GOHOSTOS=darwin、GOHOSTARCH=所填架构、target相同、CGO=0。
# target=darwin 不证明主机是Darwin；主机不符停止，禁止模拟器或跨目标结果冒充原生。
capture_inventory modules inventory_go mod verify || exit 1
```

下面仅用 Python 标准库生成全源清单；它是未来用户模板，不解析或执行 Go。
包含 root 目录的 mode/uid/gid/mtime；文件再记大小与 SHA-256。遇到 symlink
或特殊文件立即停，不静默跟随。保存此函数原文及 Python 版本到 procedure。

```bash
# 当前 NOT_RUN；调用前人工审阅，后清单使用完全相同的函数。
inventory_source() {
  python3 - "$WORK" <<'PY_SOURCE'
from pathlib import Path
import hashlib, json, stat, sys
root = Path(sys.argv[1])
rows = []
for p in [root] + sorted(root.rglob('*')):
    st = p.lstat()
    row = dict(path=p.relative_to(root).as_posix(), mode=oct(stat.S_IMODE(st.st_mode)),
               uid=st.st_uid, gid=st.st_gid, mtime_ns=st.st_mtime_ns)
    if stat.S_ISDIR(st.st_mode):
        row['kind'] = 'directory'
    elif stat.S_ISREG(st.st_mode):
        row.update(kind='file', size=st.st_size, sha256=hashlib.sha256(p.read_bytes()).hexdigest())
    else:
        raise SystemExit('Unreviewed source object: ' + str(p))
    rows.append(row)
print(json.dumps(rows, ensure_ascii=False, sort_keys=True, indent=2))
PY_SOURCE
}
capture_inventory source-before inventory_source || exit 1
capture_inventory source-and-tool-sha256 python3 -c \
  'import hashlib,pathlib,sys; [print(hashlib.sha256(pathlib.Path(p).read_bytes()).hexdigest(),p) for p in sys.argv[1:]]' \
  "$EVIDENCE/source.tar" "$GO_BIN" || exit 1
```

对应完整清单保存在 `source-before.stdout`（JSON），最终生成 source-after.stdout
逐字比较。目录变化也必须说明，不静默忽略源码变动。另保存
`host-and-storage.md`，核对 OS、架构、卷类型/subtype、uid/mode 与空间；
不转储环境，不依赖仅有路径或磁盘标签推断文件系统。

## 6 有限源码门、选择与编译（当前 NOT_RUN）

先完成新七路径的独立人工审阅，特别是所有实际调用和所有权路径。新 guard
只是 exact path/tag/env/entry、no init、source pin 和有限实际调用结构检查；
不会执行 native fixture，也不替代控制流审阅。负向变体仅在内存中 parse，
不启动、写盘或运行变体源码。

```bash
# 分别审批后逐项执行，任何非零立即停。不是整个包/全仓测试。
capture_inventory guard-tests inventory_go test -mod=readonly -buildvcs=false -p=1 \
  -count=1 -json -vet=off -timeout=2m \
  -run '^TestDarwinInventoryManual(SourceContract|DefaultExclusion|GateOrderMutations|NativeCallMutations|IdentityMutations|NoInitMutations)$' \
  ./internal/testscope || exit 1
capture_inventory source-guard inventory_go run -mod=readonly -buildvcs=false ./internal/testscope || exit 1
capture_inventory default-selection inventory_go list -mod=readonly -json ./internal/encstore || exit 1
capture_inventory manual-selection inventory_go list -mod=readonly -tags=manual_darwin_inventory -json ./internal/encstore || exit 1
capture_inventory default-compile inventory_go test -mod=readonly -buildvcs=false -p=1 -vet=off \
  -c -o "$EVIDENCE/ordinary.test" ./internal/encstore || exit 1
capture_inventory manual-compile inventory_go test -mod=readonly -buildvcs=false -p=1 -vet=off \
  -tags=manual_darwin_inventory -c -o "$EVIDENCE/manual.test" ./internal/encstore || exit 1
capture_inventory default-symbols inventory_go tool nm "$EVIDENCE/ordinary.test" || exit 1
capture_inventory manual-symbols inventory_go tool nm "$EVIDENCE/manual.test" || exit 1
capture_inventory manual-build-info inventory_go version -m "$EVIDENCE/manual.test" || exit 1
capture_inventory manual-binary-sha256 python3 -c \
  'import hashlib,pathlib,sys; p=pathlib.Path(sys.argv[1]); print(p.stat().st_size,hashlib.sha256(p.read_bytes()).hexdigest())' \
  "$EVIDENCE/manual.test" || exit 1
```

guard-tests 必须精确六个顶层 RUN/PASS、零子项/FAIL/SKIP、一个正确 package PASS，
exit 0、空 stderr。source-guard 必须明确成功。default-selection 的 TestGoFiles
不得有该文件；manual-selection 必须有，两个旧 namespace manual 文件仍不应
被选入。默认 binary 符号不得有唯一入口；manual binary 必须有精确入口，且
不得把只有 `-run` 零匹配的退出 0 当作证明。上述 binary 均尚未被运行。
固定 `-vet=off` 仅声明本批不是 vet 资格；不覆盖历史独立 vet 限制。

把 manual.test 的大小、SHA-256、build-info 与完整 argv 保存；源码前后相同、
编译来源/依赖全部可追溯才进入下一节。不能修改源码后沿用旧 binary。

## 7 唯一原生运行命令与预期结果（当前 NOT_RUN）

此步需要单独明确的原生执行授权。不得自动补做 gate 缺失、坏路径、换卷或
失败 close 的物理负对照。负变体只属于上一节纯源码 guard。

```bash
# 手工核实全部前置后仅执行一次；已编译二进制，无 go test 隐式重编译。
capture_inventory native-inventory env -i \
  PATH=/usr/bin:/bin TMPDIR="$RUNROOT/tmp" LC_ALL=C LANG=C TZ=UTC GOMAXPROCS=2 \
  GOSQLITE_MANUAL_DARWIN_INVENTORY=1 \
  GOSQLITE_MANUAL_DARWIN_INVENTORY_ROOT="$NATIVE_ROOT" \
  GOSQLITE_MANUAL_DARWIN_INVENTORY_SOURCE_TREE="$SOURCE_TREE" \
  "$EVIDENCE/manual.test" -test.v -test.count=1 -test.parallel=1 -test.timeout=2m \
  -test.run '^TestManualDarwinUnicodeInventory$'
# 无论成功失败均保存 capture 的退出码。失败时停止后续项目代码，不自动重跑。
```

标准输出必须有精确一个 `=== RUN   TestManualDarwinUnicodeInventory`、一个
相应顶层 PASS、最终 `PASS`、零子项/SKIP/FAIL，exit 0、空 stderr。还必须逐项
核对下列 11 个 `ASSERT ... PASS`，每个恰好一次：

1. prerequisites
2. nil-owner-refusal
3. retained-enumerator-classification
4. namespace-exact-bytes
5. namespace-byte-limit
6. namespace-entry-limit
7. selfopen-exact-bytes
8. selfopen-byte-limit
9. selfopen-entry-limit
10. samples-unchanged
11. retirement-complete

四个 SAMPLE 名称各出现两次，前后大小/digest 一致；family 总额 12，sibling 7。
11 个显式 owner 的 RETIRE 必须 attempted=true、complete=true、physical=true，
无 Close 错误；终态 unresolved=0、registry_groups=0、registry_leases=0。
不要把预期四次 ErrLimit 记录当作异常重试理由。所有 fixture 仍在原路径。
只在全部实际命名断言、身份、证据和保全都满足后，才能写“该精确源码/二进制
在该原生 Darwin/架构/卷上有限 inventory harness PASS”。

## 8 失败排查、封存与用户确认后清理

任何编译/依赖失败、缺名/重复名/额外名、SKIP、非零、截断日志、owner 未知、
身份变化或前置不满足，停止，不扩 selector、不替换工具/卷、不放宽断言，
不自动再次 Close/Open/test。一次失败就是该次 FAIL 或前置 BLOCKED，保留它。

- gate/root：检查本次已批准的白名单及绝对路径；不得自动采用 TMPDIR/TempDir。
- uid/mode/symlink：交给用户核实隔离 root；不 chmod 旧目录、删除旧文件或跟随链接。
- ErrUnsupported/Fstatfs：保留实际 type/subtype/local/kernel；不改成所有 MNT_LOCAL
  都通过，也不把 nil/closed owner 改为 path probe。
- count/charge：保留所有四个 ordinary payload、目录清单及日志，不删 sibling 求通过。
- ErrChanged/poison/pending/unknown retirement：保留所有现场、ledger 诊断及 source；
  不重试未知 consuming Close，不释放仍有未退休 child 的 parent，不 kill/恢复数据库。
- 超时/异常退出：记录已看到的最后断言；缺少退休证明就记 UNKNOWN，不推断 OS
  进程结束等于 Close 通过。不得外加 kill、fault 或 child process 场景。

下面的静态后清单在成功或失败之后单独保存，不重新启动项目代码；若源文件
已经变化，保留两个版本的证据并记 FAIL，不把差异隐藏或更新 pin 求通过。

```bash
# 当前 NOT_RUN。只采集该用户新建的 source 副本。
capture_inventory source-after inventory_source || exit 1
cmp "$EVIDENCE/source-before.stdout" "$EVIDENCE/source-after.stdout" || exit 1
```

成功失败都保留整个 RUNROOT。后续只收集无需启动项目代码的静态材料，且不访问
真实秘密；补 source-after 清单、四个新样本的 size/digest/实际权限、日志与完整
command/start/end/exit/stdout/stderr、procedure、工具和 binary 哈希、host/storage
证据、前后源码差异、逐名断言核对表。生成封存文件清单并读回比对 SHA-256。
不要声称当前的源代码手册已经产生任何这些未来运行证据。

用户先确认源代码/日志/失败 fixture 的副本完整可读、没有未处理的 owner 状态，
再单独决定如何清理自己新建的 RUNROOT。本文和 harness 都不给自动删除、强制
终止、secure erase 或未知 handle 重试命令。若用户没有确认，持续保留现场。

本新候选只填补有限目录分类/计费 harness 源码缺口；源码审阅、guard/selection/
compile 的未来结果和 native 的未来结果必须分别记账。目前全部执行层仍
**NOT_RUN**，不得宣称 Darwin/加密/生产安全或 M3/M8/M10 整体验收完成。
