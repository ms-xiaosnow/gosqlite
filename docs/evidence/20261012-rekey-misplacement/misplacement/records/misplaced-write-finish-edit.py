from pathlib import Path
import re,hashlib,json
R=Path('/workspace/scratch/5e06a74e5f65/restart-20261012T2033');O=R/'misplaced-write-overlay';W=R/'gosqlite'
def fn(s,name):
 m=re.search(r'^func (?:\([^\n]+?\) )?'+re.escape(name)+r'\([^\n]*',s,re.M);assert m,name
 return s[m.start():s.index('\n}',m.start())+2]
# Do not continue byte reads after a known failed bounded post-image read.
p=O/'sqlite/rollback_fault_sql_manual_linux_test.go';s=p.read_text();old='''			var journalErr error
			journalBytes, _, journalErr = manualRollbackSQLRead(&c.ledger, filepath.Join(c.root, "trial.db-journal"), 16408)
			misplacementErr = errors.Join(misplacementErr, journalErr)
''';new='''			if misplacementErr == nil {
				journalBytes, _, misplacementErr = manualRollbackSQLRead(&c.ledger, filepath.Join(c.root, "trial.db-journal"), 16408)
			}
''';assert old in s;s=s.replace(old,new);p.write_text(s)
# Refresh only the changed closed root semantic decision after that refinement.
p=O/'internal/testscope/rollback_fault_sql.go';s=p.read_text();pat=r'(`func TestManualRollbackFaultSQLLedger\([^`]*?)(`\)\.\.\.\))';s,n=re.subn(pat,lambda m:'`'+fn((O/'sqlite/rollback_fault_sql_manual_linux_test.go').read_text(),'TestManualRollbackFaultSQLLedger')+m.group(2),s);assert n==1;p.write_text(s)
# Fixed additional count and role cases.
p=O/'internal/testkit/rollbackfault/contract_test.go';s=p.read_text();s=s.replace('"success-exposed", "prefix", "short-nil", "unexecuted"','"success-exposed", "prefix", "short-nil", "negative-native", "oversized-native", "unexecuted"');s=s.replace('\t\tcase "unexecuted": result.Executed = false','\t\tcase "negative-native": result.NativeN = -1\n\t\tcase "oversized-native": result.NativeN = 4097\n\t\tcase "unexecuted": result.Executed = false')
needle='\t\tbad = spec\n\t\tbad.Action = Action(value)';assert needle in s;s=s.replace(needle,'''\t	bad = spec
		bad.Target.Role = Role(value)
		if bad.Target.Role != Database {
			if _, problem := NewPlan(bad); problem == NoProblem { t.Fatal("misplacement role accepted", value) }
		}
		bad = spec
		bad.Target.Occurrence = uint8(value)
		if value != 3 {
			if _, problem := NewPlan(bad); problem == NoProblem { t.Fatal("misplacement occurrence accepted", value) }
		}
'''+needle);p.write_text(s)
# Current status additions preserve earlier historical evidence exactly.
manual='''# 2026-10-12 R2：固定错位写入源码后继

状态：**SOURCE_ONLY / INDEPENDENT_REVIEW_PENDING / FORMAT_NOT_RUN / MODEL_GUARD_COMPILE_NOT_RUN / NATIVE_NOT_RUN**。
此后继基于 main `01c63e6f8fb3d9e196afe2f42a163eb3eac4ad5a`，只编辑独立小型 overlay 的十一份既有 Go 源文件与两份中文手册，不改运行工作树。没有运行 Go、gofmt、AST/parser/scanner runner、测试、fixture、Commit、miswrite、recovery、目标二进制或 CI。下方历史已通过记录仍只属于原始字节，不能转授本后继。R2 设施代码终点为独立源审与另行批准的有限模型／guard／编译验证；本稿尚未达到该终点。完整 goal/M3 与真实介质资格没有因此关闭。

## 唯一新增 case 与完整允许结果

在原九格之后仅追加 `index-misplace-minus64-error`（`SQLIndexMisplaceMinus64Error=10`）。旧九个 case 的值、目标及分类不变。新增 `MisplaceIndexPageMinus64`，没有任意 offset 参数、回调、profile 或重试框架。

- 请求仍为 DBPage 第3次、同一个原 managed database owner、offset=8192、length=4096。通用 Call／Result／Event 字段形状不变。
- 真正调用为同一个 owner 的 `WriteAt(b, off-64)`，b 是完整4096字节 payload，native offset=8128。写入区间 `[8128,12224)` 只覆盖原 journal 保护的 page2 尾64字节和 page3 前4032字节，文件仍必须恰12288字节。
- 只有实际 `(4096,nil)` 才追加已有 private injected error，向原 writeFullAt 返回 `(4096,injected)`。实际非nil错误、短计数、无效计数均保留，不覆盖、不补写、不重试，也不能满足该计划。
- public Commit 必须保留原 `*sqlite.Error`、CodeIOErr、ErrPoisoned、ErrRecoveryRequired 与唯一原注入原因。短写、uncertain、pending、额外 Close/cleanup 原因不合格。原 public Commit／立即 Finish 邻接保留。
- 完整允许结果集只有 `{OLD}`：一次 fresh independent replay.db 恢复后，七个 typed SQL 查询、实际 SEARCH/by_k 与 NOT INDEXED 工作量、EOF/Err/一次Close、三页完整 Check 和全文件 baseline hash 都必须相等。NEW、混合行/索引、同count不同成员均不合格。损坏原 trial 从不作为健康 NEW 打开。

## 实际参数、字节与 journal 证据

SQL 专用固定记录保存 writer 入参 offset/len、真正 native 变量 offset/len、调用点累计次数及调用前完整 payload SHA-256。新分支只调用一次原 WriteAt；scope 在独立锁内保存这一观察，锁内无I/O。不会从 plan 反填 native offset，不会把 Returned.Call.Offset 改成8128。其余九格的完整 SQLMisplacementEvidence 必须为零。

arm 额外检查实际 t.info.Size=12288、original/header count=3、dirty与journal originals 恰1/2/3、完整4096字节 buffer及schema/table/index叶种类13/13/10。既有健康独立owner、no session/shared/encryption/keepFile/borrowed cleanup、one-lifetime registry全部保留。

成功完成 Commit/owner检查后，caller 才读原 trial：同 inode、恰12288字节，post[8128:12224] hash 等于调用前 payload hash，且不同于独立 baseline 同区间；post[12224:12288] 与 baseline 尾64字节逐字节相等。不能要求其余所有字节均OLD，因为前两页已正常写入。

原16408字节 journal 在内存中用现有纯 `journal.ParseReplay` 解析一次，限制仍16408/3/12288。要求 finite-header count、Records=3、OriginalPages=3、page/sector=4096、DatabaseBytes=12288、ValidateExtent(12288)成功、pages按1/2/3排序且每个完整original image逐字节等于独立baseline。保留各页hash；通用receipt中的journal body/header/directory发布成功前缀仍不可省略。

`commit-observation.json` 在任何依赖性失败退出之前保留实际 writer/cause/已获字节观察和补充错误文本；原错误对象与diagnostics仍保存在内存，文本不替代identity检查。原件、不可变copy与独立replay的文件inventory、长度、不同inode、retention与一次owner退休规则不变。只恢复副本，原 trial与原 journal始终保留。无extra recovery、Close或cleanup retry。

## 兼容性与后续有限范围

生产 Commit、default bridge、VFS、recovery、A5代码与rollbackFaultBefore/Core、rollbackFaultAfter/Core四个函数保持原字节；A5使用Pass，不产生placement记录。现有SQL manual root名字不变，不增加可运行根或controller。

新增纯值正/负例及双重重钉source hash/full snapshot后仍必须命中独立语义诊断的反例，覆盖真实调用offset、完整payload、一次调用、native错误优先、严格实际geometry、journal/tail字节、public cause、OLD-only与旧九格零证据。旧否定断言和A5 exact-core/mutex-order检查保留；未来unknown case边界从10移动到11，因为10现为唯一明确追加case。

另行授权后的固定范围仅为：rollbackfault／rollbackrestart／testscope三个纯值与source包；原内存WriterAt测试根；ordinary sourceguard；sqlite、pager的manual_fault_rollback编译及sqlite双tag编译；三个准确镜像的nm/hash/根inventory。不得运行tagged image（即使空selector），不得扩展到旧ANN、campaign、全仓或物理测试。所有检查必须对应最终source manifest和固定工具哈希，原FAIL/NOT_RUN保留。

以下历史手册与证据说明完整保留。

'''
for f in ['docs/testing/rollback-fault-sql-manual-zh.md','docs/testing/rollback-fault-adapter-zh.md']:
 p=O/f;p.write_text(manual+p.read_text())
print('Failure-stop refinement, finite count/role negatives and Chinese manual status authored.')
