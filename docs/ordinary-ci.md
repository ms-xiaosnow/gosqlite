# Ordinary unit and SQLite oracle CI

## 2026-10-11 本地组合与 hosted CI 的分界

冻结树 `cace45621c10259231a2e764cfbefdcdbe312ccf` 的新鲜本地普通门已为
5,758 PASS / 7 个固定 oracle SKIP，另有四个修复普通根 47 PASS、
原 SQL 九进程 oracle 9 PASS、健康 live-writer 7 PASS，以及 guard/vet/CGO0
build PASS。完整 26 项于 **22:05:15 UTC 自然 PASS**，28 次实际 Go 调用；
ordinary/race 各 5,758 PASS / 7 SKIP，完整 pinned oracle 7,431 PASS / 零 SKIP，
LIMIT oracle/race 394 PASS；五目标 build、模块/无 CGO 闭包与六条外平台
编译/符号门均 PASS，外平台镜像未运行。见[精确证据](evidence/readonly-ipk-combined-20261011.md)。

上述是本地执行，未修改本页描述的 workflow、脚本或 CI 范围，也不是该新树
已发布或 hosted CI 成功的证明。原 `9ca02ee7fee55f0eb4abe83330b37b80d02aa4bd`
普通门四根 FAIL 及后续 19 项 NOT_RUN 保留；新树仅四个测试文件不同，
生产字节相同。原有两项 Ubuntu CI、新提交和精确恢复只按实际终态另记。

父基点 `1e8e7fbb694b9234b7797414ae7422a38e08ab7e` 的
[CI 38160811706](https://github.com/ms-xiaosnow/gosqlite/actions/runs/38160811706)
已于 18:02:03 UTC 更新为 SUCCESS，run 47 / attempt 1，ordinary
`114533143796`、oracle `114533143863` 均 SUCCESS；已恢复的原 receipt
与封存哈希核对一致。该历史成功不替代 cace4562 的新 hosted CI。

本次 source-only 文档整理没有运行本页命令或任何 Go/oracle/native/manual
工作流；下面复现步骤仍为用户模板，不授予受限链执行许可。41 Fuzz 目标的
既有 313 固定种子与持续 fuzz campaign 分开，ANN V4、未知中断链及全部
物理/恢复/外平台原生资格保持未闭合。

The workflow `.github/workflows/check.yml` has two independent ordinary jobs:

1. `ordinary` runs `scripts/check.sh` with Go 1.27.2. It guards test scope,
   runs pure-Go unit tests, vet and build, then the separate Go race-tooling gate.
2. `oracle` builds the pinned official SQLite 3.54.0 CLI in the runner's temporary
   directory, then runs `scripts/test-oracle.sh`. The script requires Go 1.27.2
   and an explicit absolute `SQLITE_ORACLE` executable path, runs the scope guard,
   verifies the exact SQLite version/source ID, and executes the complete
   ordinary `go test -count=1 -timeout=30m -tags oracle ./...` suite with
   `CGO_ENABLED=0` and `GOTOOLCHAIN=local`.

Both jobs run on push and pull request. The oracle job has no `-run` filter,
package subset, manual build tag, optional-oracle fallback, or cached test pass.
The C compiler and SQLite CLI are test-only; production remains pure Go and does
not execute or link SQLite C. The builder runs no upstream SQLite tests.

## Reproducing the pinned oracle

The build recipe and upstream pins come from `docs/upstream.md` and the existing
`scripts/install-baselines.sh`; this smaller builder does not download Go.
Use a host with a C compiler/make, Python 3.11 or newer, unzip and curl. The builder fails
explicitly if a required tool is absent; CI does not install system packages.
Choose a new absolute destination whose parent exists; existing destinations are
refused so another toolchain cannot be overwritten.

```sh
./scripts/build-oracle.sh /absolute/new/sqlite-oracle
export SQLITE_ORACLE=/absolute/new/sqlite-oracle/sqlite-build-3540000/sqlite3
./scripts/test-oracle.sh
```

To reuse an already downloaded official archive without a network request:

```sh
./scripts/build-oracle.sh /absolute/new/sqlite-oracle /absolute/sqlite-src-3540000.zip
```

Both routes require the 14,881,448-byte archive and stream its SHA3-256 before extraction, the generated
`sqlite3.c` SHA3-256 after compilation, and the executable's version/source ID:

- Archive: `a5c29342ca185abcda332448fbb27c536661245be27e20199b46c49099b0e15e`
- Generated source: `8efd453d08a7cfc4c79de5923153036cdc4dcaa254d4f1e617be6a2a98828f9e`
- SQLite: `3.54.0`
- Source ID: `2026-10-09 15:46:58 be8d059e9a49089ab2dce5ed26dd87aaf598fdc5fbe0b107c0dd758464bcd5e3`

Configure/build logs remain in the destination's `evidence/` directory. These
checks do not claim byte-identical CLI binaries across compilers/build paths.

## The coverage correction

Before this change, CI ran only `scripts/check.sh`; it neither installed the
pinned oracle nor invoked `scripts/test-oracle.sh`. Historical CI run
[38076830650](https://github.com/ms-xiaosnow/gosqlite/actions/runs/38076830650)
therefore passed its narrower unit/vet/build/race scope. It did not establish a
full tagged-oracle pass. A scoped clean-restore oracle command likewise verifies
only the selected packages/tests, not every tagged test in the repository.

The subsequent complete tagged-oracle run exposed an obsolete assertion in
`TestOracleErrorsAndResources`: an implemented `JOIN ... ON` query was still
expected to return `ErrUnsupported`. The exact historical failure log
`relational-lead-v2-checks.txt` is retained unchanged by the lead. This correction
moves the original query into positive pinned comparison and replaces only that
negative case with unsupported `JOIN ... USING`, retaining the explicit error
assertion. No assertion was relaxed and no test was skipped.

The source/helper audit and local focused/fresh-builder evidence are under
`docs/evidence/oracle-ci/`. Focused checks and a validated workflow definition do
not establish a new hosted CI pass. The lead records full corrected acceptance
and the hosted run for the exact published commit separately.

## Scope boundary

Read `docs/test-scope-audit.md` and review the bodies/helpers of changed tests
before execution. Ordinary scope includes intact, correct-key database access,
normal writes, rollback, BUSY/cancellation, close/reopen, checkpoint, successful
rekey and encrypted copy/backup/restore, and pure-memory negative cases.

Physical corruption/tamper/wrong-key/missing-file tests, hostile namespace
mutation, interrupted publication/rekey, native I/O or resource failures,
short-write/disk-full/permission injection, crash/kill/forced process-exit and
hot-recovery workflows remain outside both jobs. Manual tags are forbidden by
the scope guard. Its static reference checks supplement behavioral review and
are not a sandbox or an exhaustive safety proof.
