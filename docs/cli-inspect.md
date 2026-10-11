# 持久化元数据检查

`gosqlite inspect <db>` 保持已有 JSON 对象：`format`、`header`、`schema`。
字段名、顺序和格式说明不变；嵌套的 FileInfo、SchemaEntry 字段仍以大写
开头。打开和检查使用调用者 context，CLI 以 ReadOnly 模式连接。

`gosqlite inspect --extensions <db>` 额外输出 `extension_indexes` 数组；
无扩展索引时为 `[]`。每项包括：

- `id`、`name`、`source_table`
- `method`、`method_version`、`storage_version`
- `config`、`generation`

`config` 是原样持久化的不透明 UTF-8 字符串，JSON 转义后作为字符串输出，
不是内嵌 JSON 对象。合法自定义方法可以保存 `whole-v1` 这样的非 JSON
配置。新暴露的文本若不是合法 UTF-8 会被拒绝，避免 JSON 编码器静默替换
字节。记录顺序为 catalog rowid 顺序。

原生 Go API：

```go
result, err := db.Inspect(ctx, sqlite.InspectOptions{IncludeExtensions: true})
```

InspectResult 的 Header、Schema 和 ExtensionIndexes 来自同一个 reader
快照。连接已有事务时借用该事务 reader，能看到未提交 DDL，检查后事务
继续可用；否则只打开一个 reader，成功返回前关闭。取消、读/解码/配额
错误及 owned-reader 关闭错误都不返回部分结果。CLI 完整取得元数据后才
编码；输出 writer 自身出错仍可能留下半段 JSON，与既有 CLI 行为相同。

## 适用范围和限制

这是持久化元数据呈现，不是索引完整性或可用性判断。只读取 sqlite_schema
和 `__gosqlite_ext_catalog` 的记录；结构性核对已知 schema 声明、名字/根页、
catalog 行形状及整数范围。必需的扩展 schema 须匹配当前存储声明；未知
storage version 被拒绝。可选 ANN schema 家族按整体识别，不读取其数据根。

不会查找方法实现或调用 Configure、Canonical、Keys 回调；不解析 tokenizer/
ANN 配置、source SQL 或索引数据，不核验配置哈希/代际正确性，也不判断索引
是否安全可用。缺少方法注册不妨碍读取其持久化配置。Check API/CLI 的验证
范围另见其文档，不能由本报告替代。

最多保留 1024 个 catalog 记录。连接 Options.MaxSortBytes 默认 64 MiB，
限制 metadata map、折叠名称、结果字符串/记录和临时解码的保守计费。
每条 payload 由 btree.NewBudgetCursor 在 overflow 拼接和解码之前预留；
预留量保留至检查结束，所以最终 JSON 小于该配额也可能被保守拒绝。
既有 schema 上限（32767 项、16 MiB 保留 schema 额度及独立 payload 上限）
和 B-tree 遍历上限另行适用。这不是 Go 总堆、RSS、page cache 或全部 schema
分配的硬上限。默认不带参数的检查不读扩展 catalog，也不应用该可选计费。

没有改动存储格式、pager、注册表、planner、写入路径或恢复逻辑。此选项
没有增加真正只读的 plaintext WAL 或加密 CLI inspect。

## 普通复核方法

先依照 [中文手册](manual-verification-zh.md) 的环境与隔离步骤准备 Go 1.27.2、
依赖、独立源码副本和证据目录。以下命令仅适用于包含本功能的精确提交，
不要对手册较早的固定基点 776b23e 直接假定有该命令。

```sh
export GOTOOLCHAIN=local CGO_ENABLED=0
INSPECT_EVIDENCE=$(mktemp -d "${TMPDIR:-/tmp}/gosqlite-inspect-evidence.XXXXXX") || exit $?
capture() {
    name=$1; shift
    rc=0
    "$@" >"$INSPECT_EVIDENCE/$name.stdout" 2>"$INSPECT_EVIDENCE/$name.stderr" || rc=$?
    printf '%s\n' "$rc" >"$INSPECT_EVIDENCE/$name.exit" || return $?
    return "$rc"
}
capture inspect-go-version go version || exit $?
capture inspect-env go env GOVERSION GOOS GOARCH CGO_ENABLED GOTOOLCHAIN GOPATH GOCACHE || exit $?
test "$(go env GOVERSION)" = go1.27.2 || exit 1
capture inspect-sourceguard go run ./internal/testscope || exit $?
capture inspect-sqlite go test -count=1 -v ./sqlite -run '^TestInspectMetadata(EmptyReadonly|GramsRoundTrip|CustomMethodNoCallbacks|CatalogOnlyReads|Quota|Contexts|TransactionAndFreshHeader)$' || exit $?
capture inspect-cli go test -count=1 -v ./cmd/gosqlite -run '^TestInspect(Metadata(DefaultJSON|ExtensionsJSON|EmptyExtensionsJSON|CanceledOutput|OpaqueConfigJSON)|SourceForwardsCallerContext|ContextReadonlyFixtures|ContextCanceled|ReadonlyFixture)$' || exit $?
printf '证据目录：%s\n' "$INSPECT_EVIDENCE"
```

每项必须 exit 0；保留 stdout、stderr、完整提交 SHA、Go/OS/架构和环境。
新增 metadata 测试以成功的普通 API 创建完整小型 rollback fixture，正常关闭后
只读重开；所选既有回归还包括提交内完整 fixture 和 opener 的 AST 接线检查：

- 默认 JSON 完整字节保持；可选数组正确，无扩展索引时返回 `[]`
- grams 配置/代际精确回读；正常 Reindex 后比较新配置与代际
- 已注册和未注册的自定义方法都能呈现 `whole-v1`；回调计数不增加
- 被动读页记录不改变结果，不读取 source/fields/postings/frequency 根
- 同一完整配置的低预算调用返回 ErrMemoryLimit 和零值结果；预解码配额
  在读取配置 overflow 页前生效，不注入 I/O 错误
- 预取消/过期 context 保留 errors.Is，CLI 不输出内容
- 事务 DDL、header、catalog 同快照，原事务继续正常查询、写入、提交；
  后续连接正常提交后的完整 header 与新检查结果精确一致
- 只读阶段前后整个目录 inventory、SHA 和文件内容不变

失败后保存原始日志再停止。先核对源码版本、工具链、依赖、配置和真实
测试选择；不要改断言、删除或损坏数据库来强行通过。清理只删除用户创建
的隔离源码/构建产物，保留原库和证据。Go 测试的临时 fixture 通常会自动
清理，失败日志并不保证包括完整数据库现场。

## 验证边界

独立实现阶段在 Go 1.27.2 Linux/amd64，以 plaintext rollback、合法完整
grams/custom fixture 和既有默认 inspect fixture，通过上述定名测试、
相同选择的 race、scoped vet 和 CGO0 build。首次新增测试漏传 Commit 的
context，编译 exit 1；单参数修正后通过，原错误与源码哈希保留。
完整组合门、独立审阅及持久检查点的结果应以对应证据记录为准。

这些结果不涵盖加密 inspect、真正只读 WAL、ANN 算法/恢复、中断操作、
物理故障或其他 OS 原生行为，也没有完成 goal §12.2 的全部 CLI 要求。
