# Bounded LEFT JOIN left-row prefilter

## 当前五切片组合状态（2026-10-12）

当前冻结运行树为 `59ce26879c4301babc7648fbddc5cc66c4097263`（3,267 文件、783 个 Go 文件），包含 LEFT/Rows、encrypted inspect、Darwin inventory manual、Connector rollback manual 和 automatic UNIQUE（含 c382 空结果表头 helper 修复）；**不包含 CHECK**。该树于 **2026-10-12 05:21:06 UTC 自然完成完整 26 门 / 28 次 Go 调用 PASS**，lease 已 RELEASED。ordinary/race 各 **8,569 named PASS / 7 个固定预期 SKIP**，完整 pinned oracle **10,301 PASS / 0 SKIP**，LIMIT oracle/race **394 PASS**。新检查点提交、Git bundle、精确恢复及对应 CI 均 **PENDING**；已验证的未发布源包不代替这些步骤。

已发布 main 仍为 `8bc189474bdb2b0a683c6c059992f9acbc426250`，其 bundle、恢复 26 门和两项 CI 已闭合，只资格该精确基点。当前范围、具名输入收据和真实代码/原生余缺以[当前状态](current-status-20261012.md)及[组合证据](evidence/combined-checkpoint-20261012.md)为准。下文带旧日期的 SOURCE_ONLY、NOT_RUN、PENDING、FAIL/HOLD/UNKNOWN 均保留原时点含义；历史失败不回写，手工、物理及 M3/M8/M10 门不因此开放。

## 历史独立源码准备记录（运行前）

Status: **SOURCE_ONLY / NOT_RUN**. This independent candidate is based on the
parent-supplied checkpoint `f189746671aaf4962a699292a84110e2d43387e8`, tree
`0f29261ca50576373069dd8ad23a2bd3377a743c`. No compiler, formatter, parser,
native test, race test or official SQLite process has run for this candidate.
This document is a reviewable contract, not compatibility or execution approval.

## Retained failure and selected correction

The full fourteen-statement historical fixture and original mismatch are still
unchanged in `evidence/boolean-independent/final-oracle-probes.go.txt` and
`evidence/boolean-independent/v2-expanded-oracle-results.txt`. The original query
is `SELECT t.id,u.y FROM t0 t LEFT JOIN u ON abs(t.v) AND t.flag WHERE t.flag=99`.
Historical native execution reported integer overflow; pinned official SQLite
returned a successful empty result. Those observations belong to the old source,
not to this candidate. The new tests preserve that fixture in full, and declare
the no-secondary-index comparison as a separate healthy fixture.

The source now admits one root LEFT JOIN whose two children are ordinary named
table leaves. The entire original bound WHERE must be an AND-tree of `=`/`==`
comparisons between a direct left slot and an ordinary SQL literal, in either
operand order. One direct unary `+`/`-` around a numeric literal is allowed.
Physical-column precedence, result aliases, hidden rowid and IPK binding use the
existing binder. Every slot lookup requires presence and a valid left-side range.

Parameters, right slots, mixed/error-capable WHERE terms, nested/multi-join
relations, HAVING, promoted direct LEFT-ON aliases, typed vector declarations,
nonliteral defaults and nonordinary/nonfinite literal values stay unadmitted.
The schema proof uses the existing parser's canonical ordinary declarations;
it does not evaluate defaults or invoke `ordinarySafeDefaults`. The synthetic
schema table is also excluded. Rejection means the query keeps its prior plan;
it does not classify otherwise-supported SQL as invalid.

## Ordering, ownership and accounting

`sqlite/relational_bind.go` retains the immutable original bound WHERE before
ON/HAVING promotion, predicate cloning and constant propagation. After ordinary
predicate preparation, it compiles a separate program with `FixedColumns=nil`.
The full WHERE program and its existing fixed-column behavior remain intact.

`sqlite/relational.go` evaluates that optional program after a left row is
fetched and before the right source factory is invoked. False or NULL skips the
left row, without an ON evaluation or null extension. A true result continues
the unchanged ON/match/null-extension path, followed by the existing full WHERE
recheck. All binding, startup predicates, LIMIT/OFFSET and EXPLAIN behavior stay
on their existing paths. Evaluation uses the mutable join `rows` pointer, which
`aggregateInput` retargets to the original input scope for global/grouped routes.

The left leaf retains ownership of a skipped borrowed row until its next
Next/Close. Predicate scratch is cleared through `Rows.relationalPredicate` on
true/false/NULL/error returns. All sources still borrow the original root reader,
context and budget. The extra proof, program and scalar work are charged normally;
there is no promise of identical quota thresholds and no limit enlargement.

Plans remain JOIN/nested_loop or their existing AGGREGATE methods. Only admitted
plans append `left-row equality prefilter; full WHERE recheck after join` to their
reason. This is table scanning. No index use, seek, reordering, generalized
pushdown or performance qualification is claimed. SourceOpens, SourceRows,
JoinPairs and NullExtensions continue counting actual work.

## Fixed proposed qualification

New source roots are `TestLeftWherePrefilterAdmission`,
`TestLeftWherePrefilterResults`, `TestLeftWherePrefilterPhaseAndOwnership`,
`TestLeftWherePrefilterAggregateInput`, and driver
`TestLeftWherePrefilterDriver`. The aggregate root has six fixed cases and the
driver root three fixed histories. Fixtures are healthy SQL-created 4096-byte
rollback databases. Logical SQL errors and explicit work/memory/context limits
do not introduce physical fault/recovery hooks.

The oracle-tagged `TestLeftWherePrefilterSQLite354` proposes exactly sixteen
explicit comparisons plus one SQLite 3.54.0/source-id identity process. It reuses
the isolated `internal/testoracle.ForTest` factory, fresh `:memory:` official
fixtures, typed int64/NULL decoding, two explicit error families and natural
child exits. Its expectations remain proposals until separately reviewed and
executed. CLI output does not establish C-API prepare-versus-step timing.

The exact finite normal/race selectors, all new subtests/helpers and immutable
file manifests are delivered with the independent source evidence. Independent
source review precedes any runtime approval. No historical Boolean matrix, old
whole planner test, full-package test campaign or new CI is implicitly selected.

## Remaining gaps

Parameterized/mixed WHERE, CAST/general-expression planning, multi-join pushdown,
ordinary OFFSET/projection order and general eager-sort timing remain separate
work. Existing CAST/IPK/LIMIT/alias assertions are unchanged. Storage/UNIQUE
candidates, original goal, historical evidence, runtime limits and all physical
recovery/fault/namespace/ANN/fuzz restrictions are untouched. This slice does not
establish full SQLite parity, durability, physical safety, M3 or M10 completion.

## V2 source-only test strengthening

The independent V1 source review found no blocking production issue and identified
 two test-strength limits. V2 preserves all three production files and all 400
pre-existing test files byte-for-byte. No runtime has been executed.

Known terminal native paths now inspect `Rows.closed`, source/snapshot retirement,
connection/Tx lease counts, CurrentBytes and scalarBytes before explicit Close.
The two intentional early-Close histories retain their original bodies. The
MaxWork/MaxBytes=1 controls now explicitly require preparation failure and nil Rows;
they are not described as errors arising inside the runtime prefilter.

A new descriptor-free root, `TestLeftWherePrefilterRuntimeQuota`, has exactly two
cases. It assembles a valid private-slot equality program, the real Rows/join/
evaluator and root budget, and a memory-only borrowed-row source. For integer
comparison, dispatch reaches 61 work units, holds a 64-byte model row and 28-byte
column scratch, then asks for 640 comparison bytes followed by 512 work units.
The work boundary fails that final Step after a 732-byte peak; the memory boundary
fails the 640-byte request at a 92-byte peak. The source's Close observer records
zero scalar scratch and exactly its 64-byte row ownership before Rows.close can
reset CurrentBytes. Both cases require one left fetch, zero right factory calls,
automatic lease/source retirement and idempotent explicit Close. These are proposed
source-derived assertions, not observed results or native-allocation measurements.

Static follow-up also found a separate existing bookkeeping concern: preparation
can charge scalarBytes, while Rows.close resets CurrentBytes but does not reset
scalarBytes. LIMIT 0, EXPLAIN, cancellation before first fetch and Close-before-Next
can bypass scalar cleanup. V1 already asserts zero scalarBytes on these paths.
Those assertions are preserved; they may expose stale planning bookkeeping when
run. No production fix or weaker assertion is included here, and no actual memory
leak or runtime failure is inferred. This concern needs independent resolution
before claiming that the full finite selection is ready to pass.

The historical raw-string fixture is 737 bytes with no trailing newline:
SHA-256 `1acac981973436c2af836236a4d57bdc051a51b0ac33b5470951894f6968fc67`.
With one trailing LF it is 738 bytes and hashes to
`b05bc233560e7d02084994003ba386789b7e1f74c71a201875207f44527ab4d1`.
Both conventions describe the same unchanged fourteen statements. V2 proposes
26 ordinary/race roots, 312 descendants and 338 named nodes; the oracle remains
one root, 16 descendants and exactly 17 official processes on complete traversal.

## V3 terminal bookkeeping repair (source only)

Independent V2 review confirmed the suppressed-execution scalar state flow. This
is stale conservative bookkeeping, not an observed heap-buffer or snapshot leak.
V3 adds one terminal assignment in `Rows.close`: after source Close and owner
release, scalarBytes and resultBytes are set to zero beside the existing
CurrentBytes reset. The ordinary charging, quotas, runtime clears, failure latch,
Work/PeakBytes history and retirement/error order are unchanged. Direct reset
avoids subtracting stale counters after the terminal total is already retired.

The shared-budget aggregate/grouped child Rows are internal evaluators only.
They call fetch/evaluateSourceRow and close their source, never public Rows.Close
or Rows.close; only the root is registered with DB/Tx. Thus early child-source
retirement still cannot reset a budget while the root or another source owns
charged state. The source tree is closed before the root reset.

The paired resultBytes reset is necessary because fetch retains an emitted
result charge. Explicit early Close or a reached LIMIT can bypass the next
fetch's clearResult even after the root drops its current row. Existing early
Close and terminal tests now require both scratch subcounters to be zero; the
existing named bodies and all 400 original test files remain unchanged. No new
case, fixture, helper, process, or wider SQL admission is added.

The finite proposal stays at 26 ordinary/race roots, 312 descendants and 338
named nodes, with the same 16-case/17-process oracle. All formatting, compilation,
fixture/native/model/race/oracle execution remains NOT_RUN. V1/V2 are preserved;
this narrow production repair still requires independent source review and
separately authorized runtime qualification.
