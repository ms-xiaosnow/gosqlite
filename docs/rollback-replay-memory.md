# 有界 rollback journal 解码与回放计划（仅内存）

## CHECK / EOF 检查点：完整正常门已通过（2026-10-12 07:22 UTC）

冻结运行树 `e06f35a022536a8ebd157731ee9a860521e70eb0` 于 **07:22:18 UTC**
自然完成 **26 门 / 28 次 Go 调用 PASS**，session 80563 exit 0，lease RELEASED。
ordinary/race 各 **8,948 named PASS / 7 个明确 SKIP**，完整 pinned oracle
**10,687 PASS / 0 named SKIP**。本副本只增加状态文档，运行源码与该冻结树一致；
本副本的新提交、bundle、精确恢复运行及对应 CI 均 **PENDING**。

CHECK 仅资格[已记录的有界 scalar 子集](check-constraints.md#精确-partial-profile)。
EOF-count 仍仅为 decode-only、EOFUnproven；native owner 在写入/Sync 前继续
finite-only fail-closed，未开放 EOF 恢复。外平台只有编译/符号检查，原生手工、
物理持久性/恢复、安全及 M3/M8/M10 未验收。后续 Darwin/Windows backend 和
真实只读媒体 harness 未纳入；其 source-only/HOLD 不被本次 PASS 覆盖。

已发布基点 `e092f0d4623cb243eec6053a4b3db7eef598542f` 的 bundle、精确恢复
26 门及 CI 38209370406 两项 SUCCESS 已另行闭合，只资格该基点。
[本检查点中文报告、精确收据与未完成项](evidence/check-eof-checkpoint-20261012.md)。

从下一节起保留各阶段原文；旧“当前”、SOURCE_ONLY、NOT_RUN、PENDING 与
FAIL/HOLD/UNKNOWN 均只指原时点和原树，不能覆盖本节的新执行记录，也不会被回写。

## 组合状态说明（2026-10-12）

本文件的源码合同已合入 CHECK / EOF-count 新组合；组合所有执行仍 NOT_RUN；V1 的旧 sqlite.go
source pin HOLD 已在本 V2 按独立完整源审作唯一 hash 窄修，guard 尚未运行。CHECK 与 EOF 的输入有限结果只绑定各自原树；下面原源码阶段
NOT_RUN 与待审阅叙述保留历史含义。格式化和单片源码审阅已另有记录，不能代替组合
默认闭包复审、运行、恢复/CI 或物理验收。见[当前组合状态](check-eof-integration-20261012.md)。

以下保留各输入与检查点原时点记录；其中“当前”、未合入、NOT_RUN、PENDING 和 PASS 均只绑定原树，不覆盖上面的本组合状态。

## 当前 EOF-count 增量：SOURCE_ONLY / NOT_RUN（2026-10-12）

本批源码增加第一且唯一已发布 `0xffffffff` 记录计数头的有界 EOF-count
解码，并为已有 finite-count 路径增加明确计数来源。它只扩展字节解析；
`RecoverRollback` 同时增加必须的 finite-only 准入门，nil-default 与显式
opt-in 的原生恢复都不接纳 sentinel。Go、gofmt、Go parser、compiler、
source guard、tests、race、oracle 与 native 执行均 **NOT_RUN**；旧树的
源码审阅、有限内存证据和 PASS 不转授本批。物理恢复、故障、持久性和
M3/M8/M10 验收不由这个增量完成。

## 支持的输入与结果合同

`internal/journal.ParseReplay` 仅支持以下两种独立形态：

1. 一个或多个完整、sector-aligned、published、finite-count segments。
   首头提供 page size（2 的幂，512..65536）、sector size（2 的幂，
   32..65536）和 original page count（0..0xfffffffe）；后续头必须逐项
   相同。每段自己的 nonce 和有限计数只作用于该段记录。下一头只在前段
   记录结束处或紧随其后的 sector 边界，不搜索 magic。头内与段间 padding
   可以非零。已发布 count=0 段是零记录，不根据余下字节推断未发布计数。
2. offset 0 处第一且唯一 published 头的 raw count 恰为 `0xffffffff`。
   完整头 sector 必须已包含在输入内，余下全部字节按首头 page size 和 nonce
   解释为记录；不允许后续 segment/header。令剩余字节为输入长度减 sector
   size，必须能被 `pageSize+8` 整除；不能向下取整、忽略部分记录或接纳纯
   padding 尾部。有效计数以 uint64 推导。header-only / 零有效记录可在结构上
   解码，但不证明空事务、主库安全或允许恢复。

两种形态都逐条验证 original page number、nonce-seeded sparse checksum、
全局页号唯一性以及完整输入消费。page 0、locking page
`0x40000000/pageSize+1`、超出 original extent 的页、重复页、错误 checksum、
表观 super/master-journal trailer 均拒绝。相同 nonce 不构成错误，也不提供
认证保证。任何错误返回 nil plan，输入不变；全部验证成功后才复制页图像。

`ReplayPlan` 新增 typed `CountEvidence` 字段；原三字段语义保持：

- `ReplayCountUnknown = 0`：未建立计数来源；零值和未知未来枚举值均不得获准
  原生恢复，不能把零值默认为 finite。
- `ReplayCountFiniteHeaders`：所有已解析段有已发布有限计数。所有成功的既有
  finite-count 输入设置此值；它不表示认证、事务完整或物理持久性。
- `ReplayCountEOFUnproven`：计数只来自本次提供字节的 EOF；不能证明没有截掉
  完整记录。所有成功的首头 sentinel 输入设置此值。
- `ReplayPlan.Header` 始终保留精确首个 raw header，含原 `Records` 和 `Nonce`。
  sentinel 的 `Header.Records` 仍为 `0xffffffff`，不能归一化成有效计数；
  `len(ReplayPlan.Pages)` 才是所有形态的有效页图像数量。
- `Pages` 保持 journal 顺序，每页独立拥有 byte slice；输入、结果和不同页
  图像互不别名。`DatabaseBytes` 与 `ValidateExtent` 仍基于首头 original
  extent。旧 finite 输入的 Header/Pages/DatabaseBytes 与所有权语义不变；
  新增枚举字段是明确的内部表示增量，不声称整个结构体序列化字节不变。

既有 first-sentinel 负例在本批成为具名的 decode-only / EOFUnproven 正例，
不是删除控制；later-sentinel、legacy page-size-zero、super/master 和其他
原拒绝保持。finite journal 在早一段 record end 结束的完整前缀可仍是合法
输入。EOF-count journal 即使被截掉一个或多个完整尾记录，剩余记录也可能
全部验证成功。字节解析不能识别这种完整记录截断，不能将所有 record-boundary
前缀一概称为损坏。

## 配额、算术与安全含义

`ReplayLimits` 的三个字段与零值默认值不变：64 MiB journal、65536 records、
1 TiB original database extent。`MaxJournalBytes` 在解析前限制整个输入。
finite 路径以 uint64 减法预算检查累计 count；EOF 路径先验证完整 sector，
再用减法计算余量，验证精确整除后才得有效 count。两条路径都在分配每记录
metadata 或复制图像前检查 `MaxRecords`、original page count、记录字节界限
及 `MaxDatabaseBytes`，不让溢出、越界或 huge advertised count 先触发分配。

复制总量为 `len(Pages)*first.PageSize`，不超过实际输入、journal 配额、
record 配额所限图像量和 original extent；metadata 与 duplicate set 为
O(有效记录数)。finite 零记录段仍消耗完整 sector，最小 32 bytes，遍历量
受输入界限约束。alignment、乘法与 offset 加法都在相应余量检查后进行。
这些算法边界不是进程 RSS 或 allocator 上限承诺。

`ValidateExtent` 只要求为当前 main extent 缺失或仅部分存在的 original 页
提供图像，unused locking page 除外。它不能证明完整长度的 main 未被更改，
也不能证明所有已修改页都存在 original image。EOF 整除、sparse checksum
通过、健康 header、完整主文件或 safe-append 声称均不能补足此证明。
SQLite sparse checksum 不是密码学认证，并不覆盖每个字节；它不认证 writer
的 sync 策略、transaction generation、database identity、hotness 或 durability。
`0xffffffff` 可能来自 no-sync、memory journal 或 SAFE_APPEND 条件，不能据此
推断实际 writer 来源，更不能声称修复 no-sync 断电风险。

## 原生恢复必须的 finite-only 门

`RecoverRollback` 对同一个新解析 plan 在 ParseReplay 成功分支之后立即且
恰好一次调用 descriptor-free `admitRecoveryPlan`，失败即通过
`recoveryPlanError` 返回。该门位于 `ValidateExtent`、restored-header 验证
以及任何 Sync、写入、truncate、unlink/发布之前，不能由另一份 plan、
替换 header、回调、缓存或配置绕过。

门只接纳非 nil、`CountEvidence == ReplayCountFiniteHeaders` 且 raw first
`Header.Records != 0xffffffff` 的 plan。nil、unknown/未来枚举、EOFUnproven、
以及标为 finite 却持有 raw sentinel 的矛盾 plan 都拒绝。helper 不做 I/O、
不调用回调、不修改 plan，也不产生可复用发布许可；finite plan 仍须通过
原有 extent/header/锁/身份检查。拒绝保留 `journal.ErrReplayUnsupported`
经 `recoveryPlanError` 的 ErrRecoveryRequired/ErrUnsupported cause 路径，
不制造 touched/removed 或 commit uncertainty。非法 sentinel 字节可能先由
parser 报更具体错误，不承诺与旧 blanket refusal 完全相同的错误文本/形状。

nil-default Open/Connector 继续在 journal-presence 处拒绝，根本不进入此
parser；显式 opt-in 也不能恢复 sentinel。公开 bridge、driver policy、
Reader proof、cleanup、源 pin 和手工门不因 decode 成功获得额外权限。
生产 writer 以及 Header.Validate / EncodeHeader / EncodeRecord / ParseHeader /
ParseRecord 编解码 API 仍 finite-count-only；没有 no-sync 写入选项、新公开
flag、DSN、env 或 CLI 恢复模式。详见[公开恢复合同](rollback-recovery-open.md)。

## 当前支持/拒绝矩阵

| 输入或操作 | 结果与边界 |
|---|---|
| Published finite-count，几何一致且页号全局唯一 | 有界 owned memory plan；FiniteHeaders |
| 第一且唯一 published 0xffffffff，完整 sector 与精确记录 EOF | 仅有界解码；EOFUnproven；raw sentinel 保留 |
| EOF-count 完整记录边界截断 | 可能结构合法；不能证明 transaction 完整 |
| 后续 segment 的 0xffffffff | Unsupported；不作后续 EOF 推断 |
| 重复页、later geometry/extent 不一致 | Unsupported；不采用 first/last-wins 或覆盖首头 |
| Legacy first page size zero | Unsupported；不从不可信主头猜测 |
| Unpublished first header、无效 page/checksum/first geometry | 拒绝且无 partial plan |
| 未发布/不识别后续头、partial sector/record、额外尾部 | 拒绝且无 partial plan |
| 表观 super/master-journal trailer，包括 malformed magic trailer | Unsupported；不做外部查找 |
| 任何配额超限 | Resource-limit error；无 partial plan |
| Native sentinel/unknown/矛盾 metadata plan | 必须拒绝；default/opt-in 都不扩大准入 |
| 物理回放、hotness/identity/locking、故障/持久性 | 原有独立资格；本批 NOT_RUN |

## 固定官方来源

沿用 [upstream.md](upstream.md) 已记录的 SQLite 3.54.0 官方源码包，source ID
`be8d059e9a49089ab2dce5ed26dd87aaf598fdc5fbe0b107c0dd758464bcd5e3`；
`src/pager.c` SHA-256 为
`96d7bb9c4a5a6da0909ce0ab4714311e1ffa5f13007dc4c69f028a2760104ba4`。
本批只静态阅读，没有下载、CLI 执行或生成物理 journal/database fixture。

- `journalHdrOffset` 1388..1413：finite 后续头的对齐规则。
- `writeJournalHdr` 1471..1608：28-byte 固定字段、未使用 sector bytes，以及
  no-sync / memory journal / SQLITE_IOCAP_SAFE_APPEND 三种 sentinel 来源。
- `readJournalHdr` 1627..1731：每段计数/nonce，首头 page/sector 几何；SQLite
  的 page-size-zero fallback 需要已知 pager page size，本 parser 不采用。
- `pager_playback` 2927..2935：sentinel 假定第一且唯一头，按首 sector 之后
  的余量推导 count。本 parser 额外要求精确整除，不采用部分记录 floor/skip。
- `pager_cksum` 2289..2297 与 `pager_playback_one_page` 2336..2420：nonce 的
  uint32 wraparound sparse checksum；sentinel 没有 checksum 豁免。
- `syncJournal` 4356..4434：finite count publication 和旧下一头失效；本批
  不从 stale tail 推断 provenance，也不扩展 SQLite 的 super-journal 语义。

## 本批源码验证边界

新回归限定为独立字节 fixture 的具名内存测试，以及 descriptor-free
admission/error/source-AST 检查；既有两个 fuzz property 仅修正 raw sentinel
和显式 CountEvidence 的不变量。它们不执行 RecoverRollback、public bridge
或 recovery-enabled real Open/Connect。新测试体、既有固定 seeds 和 source
检查在本批均 **NOT_RUN**；不启动新 fuzz campaign 或手工/物理链。

下面保留前一 finite-segment 切片的历史验证说明与命令原文，仅供核对来源。
它们不是本 EOF-count 切片的执行提案或授权，尤其不授权重启其中旧 fuzz 命令。
新树所有后续执行须独立审阅准确源码与另行取得范围许可。

## Reproducible selected checks

The fixture builder uses only byte arrays and independent alignment/checksum
math, never the production journal encoder. Coverage includes 96 page/sector
pairs, every prefix of a multi-segment fixture, nonzero padding, zero-count
chains, equal/different nonces, cross-segment duplicates, late errors, cumulative
quota boundaries and uint32 overflow advertisements, independent-copy ownership,
128 deterministic randomized extent/application models, and bounded byte fuzz.

All ordinary test sources and helpers in `internal/journal` were behaviorally
reviewed as memory-only; no `TestMain`, init hook, filesystem or pager path is
present. Before each selected normal/race/fuzz suite, run the source guard:

```sh
go run ./internal/testscope
go test ./internal/journal -run '^(TestReplay.*InMemory|TestSQLiteRollbackFormatInMemory|TestRejectMalformedJournalBytesInMemory|FuzzReplay.*BytesOnly)$' -count=1
# Repeat the source guard before this separate race-tooling gate:
CGO_ENABLED=1 go test -race ./internal/journal -run '^(TestReplay.*InMemory|TestSQLiteRollbackFormatInMemory|TestRejectMalformedJournalBytesInMemory|FuzzReplay.*BytesOnly)$' -count=1
# Repeat the source guard before each bounded pure-memory fuzz gate:
go test ./internal/journal -run '^$' -fuzz '^FuzzReplaySegmentsBytesOnly$' -fuzztime=5s -parallel=2
go test ./internal/journal -run '^$' -fuzz '^FuzzReplayPlanBytesOnly$' -fuzztime=5s -parallel=2
```

Use pinned Go 1.27.2, `GOTOOLCHAIN=local GOPROXY=off GOSUMDB=off
GOFLAGS=-buildvcs=false`, sibling shared Go cache/path, and `GOMAXPROCS=2`.
Normal and fuzz runs use `CGO_ENABLED=0`; only race tooling uses CGO. Build/vet
and untagged pager test compilation are separate non-executing checks. No manual
build tag, restricted physical/native-failure test, or aggregate gate is part of
these commands. The lead owns aggregate validation and independent review.
