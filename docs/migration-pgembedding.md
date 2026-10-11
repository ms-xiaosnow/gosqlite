# pg_embedding 迁移：当前入口与离线合同

核对基点：`1352ae524575e4d6fe8b5434fd930c3b07be4ac8`（2026-10-11）。
M9 已有离线目录检查、受限 SQL AST 转换、持久 CSV/NDJSON 导入与正常续传、
只读校验，以及 PostgreSQL 迁移审阅包生成；**完整 M9 迁移验收尚未完成**。
这些工具不连接 PostgreSQL/Neon，不接收服务器 DSN/凭据，也不执行远端 SQL。
M3/M8 物理持久性、安全审查和独立发布门仍未闭合。

## 当前两条工作流

1. **导出到本地 gosqlite。** 人工审阅并收集离线目录和不可变 CSV/NDJSON
   导出；检查类型/扩展身份、维度与映射后，用 `convert` 转换支持的 SQL 子集，
   或用 `import` 导入显式 manifest 所描述的数据。导入默认只读校验；写入新库
   必须使用 `--apply --output`，匹配已提交作业的续传必须再给 `--resume`。
   `verify` 只读校验完整导出/目标数据；可显式加 `--distances` 做有界范数、
   完整精确距离和 Top-K 校验。普通导入报告的 `exact_distances`、`ann_recall`
   仍为 `not_run`，精确距离扩展也不验证 ANN recall。
2. **PostgreSQL/Neon 原地迁移审阅。** `transition` 已生成保留数组/cast 或
   typed shadow-column 方案；加 `--full-table-evidence` 可生成受限 PG16
   整表复制/换名方案。输出中的修改和安装后 SQL 保持逐行注释，人工门不自动
   解除。成功只表示审阅包生成完成，不表示服务器能力、执行、切换或回滚已验证。
   保留旧数组/表只有在写入仍同步且旧服务可恢复时才有回滚价值。

| 当前入口 | API / 实现 | 详细合同 |
|---|---|---|
| `inspect` | `DecodeCatalog` / `Inspect`；[CLI 分派](../cmd/gosqlite/migrate.go) | 本页下方的目录检查、REAL[]、术语映射和风险 |
| `convert` | [`Convert`](../migration/postgres/convert.go)、`Conversion.SQL`、`BindTransform.ConvertValue` | [AST、类型绑定和不支持语法](migration-converter.md) |
| `import` / `verify` | [`Import`](../migration/postgres/import.go) / [`VerifyImport`](../migration/postgres/import_verify.go)；[CLI 参数](../cmd/gosqlite/migrate_import.go) | [manifest、CSV/NDJSON、事务/续传、校验](import-pgembedding.md) |
| `verify --distances` | [`VerifyImportDistances`](../migration/postgres/import_distances.go) | [采样查询、全候选、误差界与跳过状态](import-distance-verification.md) |
| `transition` | [`GenerateTransition`](../migration/postgres/transition.go)；[CLI](../cmd/gosqlite/migrate_transition.go) | [CLI](migration-transition-cli.md)、[阶段/校验/回滚](migration-transition.md) |
| `transition --full-table-evidence` | [`GenerateFullTableTransition`](../migration/postgres/transition_full.go) | [PG16 依赖证据与整表示例](migration-full-table.md) |

`--apply` / `--resume` 是本地导入参数，`--output` 是本地导入/校验目标；
不能用于 `inspect`、`convert` 或 `transition`。这些只读/生成子命令使用
`--report` 显式新建报告。具体参数、边界与错误由各入口分别检查，不能把
检查器的参数限制推广为整个迁移 CLI 的限制。

### 历史检查点 A

检查点 A 当时只有目录检查和 REAL[] 转换；AST、CSV/NDJSON 导入/续传及
迁移包生成确实属于后续工作。该历史状态不再代表当前 main。原检查器的
合成目录、人工导出器和限定测试证据继续保留；下面仍适用的检查器合同
不构成完整 PostgreSQL 方言或在线迁移支持。

## 检查器示例与报告语义

From the repository root with the pinned Go 1.27.2 toolchain:

```sh
CGO_ENABLED=0 go run ./cmd/gosqlite migrate pgembedding inspect \
  --catalog testdata/pgembedding-0.3.6/catalog.json
CGO_ENABLED=0 go run ./cmd/gosqlite migrate pgembedding \
  --catalog testdata/pgembedding-0.3.6/catalog.json --dry-run
CGO_ENABLED=0 go run ./cmd/gosqlite migrate pgembedding inspect \
  --catalog testdata/pgembedding-0.3.6/catalog.json --format markdown
```

Inspection always opens only the supplied catalog, never a database. Optional
`--report new-report.json` creates a new report file with mode 0600 and refuses
an existing path; it does not overwrite a catalog, database or previous report.
Duplicate flags, unsupported flags, positional arguments and `--dry-run=false`
are errors. JSON/Markdown reports are deterministic for identical catalog facts
and options. Exit zero means inspection/reporting completed, not that findings
are resolved: callers must check `status`, per-object status and finding codes.

The included `catalog.json` is a complete **synthetic** fixture with invented
OIDs, an extension in the custom `legacy` namespace, three metrics, a custom
index name, an unrelated array column and a claimed exhaustive row profile.
It is not a captured server inventory or observed data profile. Its selected
index parameters were intentionally set to a valid reviewed 16/64 pair so the
fixture can demonstrate a proposed target configuration; tests separately
exercise invalid legacy defaults and published examples.

## 转换、导入/续传与原地迁移审阅示例

下面均使用现有 CLI 参数；本次文档核对没有执行这些命令。转换/审阅例子
读取仓库内明确标注的合成 fixture。导入例子的 `source.ndjson`、`export.json`
由调用者准备并审阅；首次导入的 `imported.db` 必须不存在。

```sh
CGO_ENABLED=0 go run ./cmd/gosqlite migrate pgembedding convert \
  --catalog examples/migrate-pgembedding/converter-catalog.json \
  --input examples/migrate-pgembedding/source.sql \
  --target local --table 30000=documents
CGO_ENABLED=0 go run ./cmd/gosqlite migrate pgembedding import \
  --input source.ndjson --manifest export.json
CGO_ENABLED=0 go run ./cmd/gosqlite migrate pgembedding import \
  --input source.ndjson --manifest export.json --output imported.db --apply
# 仅对相同源和 manifest 的已提交作业，显式正常续传。
CGO_ENABLED=0 go run ./cmd/gosqlite migrate pgembedding import \
  --input source.ndjson --manifest export.json --output imported.db --apply --resume
CGO_ENABLED=0 go run ./cmd/gosqlite migrate pgembedding verify \
  --input source.ndjson --manifest export.json --output imported.db
CGO_ENABLED=0 go run ./cmd/gosqlite migrate pgembedding transition \
  --catalog testdata/pgembedding-0.3.6/catalog.json \
  --options examples/migration-transition/options.json
CGO_ENABLED=0 go run ./cmd/gosqlite migrate pgembedding transition \
  --catalog examples/migration-full-table/catalog.json \
  --options examples/migration-full-table/options.json \
  --full-table-evidence examples/migration-full-table/evidence.json
```

本地导入按批将数据、拒绝记录、来源和游标放入同一原生事务；续传先重放并
核对完整已提交前缀。`--stop-after-batches` 是正常提交后主动返回/关闭，
不是崩溃恢复。未知提交结果返回错误，不自动重插或声称已回滚；后续显式
续传仍可能被尚未支持的公开恢复路径阻塞。默认遇错停止，显式 quarantine
只接纳限定的确定性行错误。已导入数据和源文件不被自动删除，索引失败保留
`data_complete_index_pending`。细节和可运行合成端到端示例见
[导入指南](import-pgembedding.md)。

## 目录检查 API

`migration/postgres` contains:

- `DecodeCatalog(io.Reader, Limits) (CatalogSnapshot, error)`
- `Inspect(context.Context, CatalogSnapshot, InspectOptions) (Report, error)`
- `ParseRealArray(string, dimensions) (vector.Vector, error)`
- `RenderJSON(io.Writer, Report) error` and `RenderMarkdown(io.Writer, Report) error`

A zero `Limits` uses 16 MiB input, 10,000 catalog objects including columns,
100,000 dependency edges, depth 32 and 1,048,576 JSON/work tokens. Custom limits
must be complete, positive and no greater than the defaults. Individual strings
are at most 1 MiB; identifiers are at most PostgreSQL's 63 UTF-8 bytes. Before
inspection allocates maps or canonical JSON, exported programmatic structs are
also checked for aggregate work/bytes, finite profile numbers and valid text.
The caller already owns input memory; these checks bound additional work and
allocation. The preflight byte charge conservatively bounds JSON output, so a
near-limit compact source can be rejected. No production dependency was added.

JSON requires exact field spelling and all schema fields, including explicit
false values, empty arrays and nullable norm fields. It rejects duplicate keys
at every level, unknown or missing fields, null required arrays/scalars, trailing
values, invalid UTF-8/NUL, unpaired Unicode surrogate escapes, wrong scalar
kinds, unsupported versions, duplicate identities and unresolved references.
Errors identify categories/positions without echoing source vector values.

The report's `catalog_sha256` hashes the actual inspected structure serialized
as Go JSON with stable struct-field order (`catalog_hash_kind=canonical_json_v1`).
Collection ordering is part of that identity. `source_bytes_sha256` is included
only if the structure still equals the decoded source facts; mutating exported
fields invalidates that raw-byte provenance. Hashes establish content identity,
not authenticity or proof that a supplied catalog/profile describes a server.

## Catalog collection and identity

Review `examples/migrate-pgembedding/inspect-schema.sql`. It uses a bounded-time
repeatable-read, read-only transaction, trusted `pg_catalog` names and explicit
schema scope. It reads metadata/definitions without evaluating application or
extension distance functions. The query is source-reviewed, **not executed
against PostgreSQL** in this checkpoint. Its target syntax uses PostgreSQL 12+
catalog fields such as `attgenerated`; other server versions need review.

The export query intentionally declares coverage gaps: only HNSW indexes are
selected, dependents are direct rather than transitive, policy bodies are not
fully inventoried, and privileges/RLS may require further review. It supplies
no row profile or application SQL by default. It cannot establish complete data
validity. Full-data profiling must be a separately reviewed read tied to an
immutable export; the sample shape/count profile is supplied evidence, not a
measurement performed by the inspector.

Membership checks combine `embedding` version 0.3.6, exact type OIDs/names,
`pg_depend` ownership, method handler, opclass input type/default identity,
strategy-1 operator, support-1 function, and argument/result signatures. Index
names themselves are irrelevant. An unrelated `real[]` column is a candidate,
not automatically selected. Explicit `InspectOptions.Selected` entries identify
relation OID, attribute number and dimension; they cannot override conflicting
index dimensions. Domains, unknown opclasses, unknown versions or forged
same-name objects are not treated as the known extension.

Automatic eligibility is narrower than inspection: ordinary permanent heaps,
no RLS/inheritance, one non-NULL int4/int8 primary key, selected real[] columns,
no generated/default/identity expression or custom typmod, and single-key, unfiltered,
non-expression HNSW indexes. Dependencies and incomplete coverage require
review. `attndims` does not prove actual shape. Primary-key identity/sequence defaults
require explicit mapping review; they are not silently treated as local rowid
allocation. Exhaustive supplied profiles must
agree on dimensions, bounds, NULL policy and zero/nonfinite status. No dimension
is inferred from the first row.

`safe` on an object means it passes this inspector's supported subset checks,
not production safety or migration authorization. Invalid/review-required
objects do not expose a usable `proposed_target`. Global warnings still require
review even for individually eligible configurations. No report approves remote
execution or certifies target privileges/version.

## REAL[] value conversion

Accepted non-NULL values are one-dimensional `{1,-2.5,3e-4}` arrays, including
quoted numeric elements with decoded PostgreSQL array escapes, ASCII whitespace,
and explicit default bounds such as `[1:3]={1,2,3}`. Decimal tokens round directly
to binary32 using the existing dense parser. Signed zero, finite extrema and
subnormals are preserved; decimal underflow may produce signed zero.

SQL NULL belongs to a row/nullability layer, not `ParseRealArray`. `{}` is not
NULL. Bare NULL elements, quoted `"NULL"`, multidimensional/ragged arrays,
nondefault lower bounds, inconsistent bounds, nondecimal values, NaN/infinity,
binary32 overflow, trailing syntax and inconsistent dimensions are rejected.
No normalization, padding, truncation, flattening or precision conversion is
performed. Text is limited to 2 MiB, components to 16,000, and a decoded numeric
element to 128 bytes. `ValueError` gives stable code, one-based element ordinal
and expected/observed dimensions without embedding contents.

This preserves source real values represented accurately by the export; it
cannot recover binary32 bits already lost by insufficient decimal precision.
Local metric arithmetic is compensated float64 while old extension results are
float32/architecture-sensitive. Cosine zero norms are an explicit local error.
No exact output-bit equivalence is claimed.

## Options and extension-transition hazards

Pinned mappings are `ann_l2_ops` → `vector_l2_ops`, `ann_cos_ops` →
`vector_cosine_ops`, and `ann_manhattan_ops` → `vector_l1_ops`. Legacy Manhattan
`<~>` 在已确认的旧向量类型/扩展身份上下文中，由当前 `convert` 转为 dense
`<+>`；现代 bit `<~>` 仍表示 Hamming。`inspect` 本身仅报告身份，SQL 改写由
[独立 AST 转换入口](migration-converter.md) 完成，绝不全局替换同名运算符。

Effective legacy defaults are m=100, efconstruction=16, efsearch=64. Target HNSW
requires m=2..100, ef_construction=4..1000 and ef_construction >= 2*m. Both old
100/16 defaults and the published 3/5 example are invalid. The report preserves
the old effective values and emits `target_option_constraint`; it never clamps
or silently retunes. Dense values can store 16,000 dimensions, but the target
HNSW cap is 2,000. Unsupported index options are not dropped.

`efsearch` becomes a reviewed PostgreSQL session/transaction setting or local
`VectorSearchOptions.EFSearch`, not an index creation option. Conflicting old
per-index values require application-specific handling. Query settings greater
than the target/local 1..1000 bound are rejected.

Both old `embedding` and target `vector` define the database-wide `hnsw` access
method. Different extension schemas do not avoid the collision. In-place
transition requires reviewed dependencies, a maintenance-window or separate
branch/database choice, confirmed target extension version/opclasses and a
rollback plan. No default CASCADE, source-column deletion or cleanup is provided.
Retained arrays help rollback only if later writes keep them current; restoring
old ANN service after a committed extension transition is a separate operation.

## 固定来源与历史检查点证据

Pinned legacy source: `5d48508aeaeb86b9e9c630468ada7a124b905795`, extension 0.3.6.
Pinned target: pgvector v0.8.7, `f37c13f68b57d2c3472b2214fbcff699d6d34876`.
`testdata/pgembedding-0.3.6/manifest.json`, LICENSE and NOTICE preserve exact source
file hashes and licenses. The Go implementation is independently written.

The requested [current Neon page](https://neon.com/docs/extensions/pgvector#migrate-from-pg_embedding-to-pgvector)
was retrieved on 2026-10-10; its migration chapter is absent. The separate
[official historical chapter](https://github.com/neondatabase/website/blob/d4c1cf37f79acbf4c02880eddb97dd9a36cb2d20/content/docs/extensions/pg_embedding.md#migrate-from-pg_embedding-to-pgvector)
is the 2024-01-23 source reference. Historical destructive examples are not
current migration instructions or executable defaults.

Focused tests exercise real CLI dry-run/reporting, strict catalog validation,
ownership/signature drift, invalid option defaults, dimension/profile conflicts,
byte-exact binary32 conversion and source/provenance mutation. Only ordinary
in-memory/parser and intact-file CLI tests run here. No PostgreSQL execution,
remote write, crash/fault/forced-exit, recovery, wrong-key or physical-tamper
workflow was performed. Exact executed commands/results are recorded in the
checkpoint evidence by the integration lead.

## 已有实现证据与尚未完成项

以下均为仓库已保存的限定历史证据，不是本次静态文档核对新跑的测试，
也不将旧源码上的 PASS 改写为本次基点的执行结果。

| 范围 | 精确现有测试入口 | 已保存证据 |
|---|---|---|
| AST 与绑定转换 | [`TestConvertLocalExecuteExactAndNormalReopen`](../migration/postgres/convert_test.go)、[`TestMigrateConverterCLI`](../cmd/gosqlite/migrate_converter_test.go) | [转换器审阅与实际命令](evidence/m9-converter-review.txt) |
| 导入/续传/校验 | [`TestImportNativeCommittedResumeAndVerify`](../migration/postgres/import_test.go)、[`TestMigrateImportDryRunApplyResumeVerifyCLI`](../cmd/gosqlite/migrate_import_test.go) | [导入独立审阅](evidence/m9-import-review.txt) |
| 迁移模板和整表方案 | [`TestMigrateTransitionReviewOnlyArtifacts`](../cmd/gosqlite/migrate_transition_test.go)、[`TestFullTableCopySwapAndContracts`](../migration/postgres/transition_full_test.go) | [CLI 审阅](evidence/m9-transition-cli-independent-review.txt)、[整表独立审阅](evidence/m9-full-table-independent-review.md) |
| 调用者数据精确距离 v2 | [`TestIndependentImportDistanceRationalOracle`](../migration/postgres/import_distance_independent_review_test.go)、[`TestImportDatasetDistancesMappedFormatsAndResume`](../migration/postgres/import_distances_test.go) | [v1 失败和 v2 修正](evidence/import-distance-v2-handoff.md) |

转换器初轮失败、导入 CSV 内存保留[回归失败](evidence/m9-import-retention-before-fix.txt)
及其[修正记录](evidence/m9-import-final-delta.txt)、16,000 维距离 v1 误拒绝均
继续保留。它们与未完成 ANN 报告工具的 FAIL/HOLD 是不同证据边界。
未知触发中断、新 fuzz/ANN V4 暂停链路仍按[中断记录](interruption-20261010.md)
处理，不重试或换路。公开恢复、明文 backup、WAL-tail、planner 与部分 OS
backend 修正候选尚未合入；完整物理故障 harness、原生 OS 验收、真实 PG/Neon
执行和外部独立安全/法律审查仍有缺口。参见[验收矩阵](acceptance-status.md)
及[中文人工验证边界](manual-verification-zh.md)。本页入口修正不关闭这些项目。
