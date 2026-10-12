# post-DB 检查点：远端普通 CI 与备份回读

检查点 `6d668f80d199b2c49647a3cbc949442d125988ae`，树 `ce11dea8ec544a099692443aef43140f063e699b`。

## 远端结果

[GitHub Actions 38311861065](https://github.com/ms-xiaosnow/gosqlite/actions/runs/38311861065) 于 2026-10-12 21:56:19 UTC 自然结束，attempt 1，结论 SUCCESS。两个 job 的 head SHA、run ID 与检查点一致，均为 ubuntu-latest：

- oracle `115019235397`：SUCCESS，21:44:21 UTC 完成。验证官方 SQLite 3.54.0 固定源包、生成 amalgamation 和运行身份，然后完成带源引用守卫的普通 oracle 全仓套件。
- ordinary `115019235657`：SUCCESS，21:56:18 UTC 完成。`./scripts/check.sh` 的源引用守卫、普通全仓 tests、vet、build 和 race 阶段完成。

命令由同一提交的 source-snapshot 说明；具体 job/step 元数据及时间见 ci-evidence.json。两个 `.decoded.log` 保留 GitHub 已解码的 UTF-8 文本，含 BOM 与原有屏蔽 `***`，未作换行或文本正规化。它们不是原始 GitHub ZIP 的字节副本。检查了凭据相关行与常见密钥模式，未发现需要额外遮盖的内容；该检查不构成无秘密的绝对证明。

## 可恢复备份

`gosqlite-postdb-6d668f8.bundle` 已存 Library，并按精确 Library/file ID 回读。7,862,905 字节，SHA-256 `ed43c8ef6ca710612f4d186230d2f27ed9520f5a451a55a2821dde23507891fe`。

独立 bare restore 的提交、树、全部 3,387 个受跟踪文件（53,417,387 字节）和 Git 模式与检查点一致；`git fsck --full` 退出 0。bundle 包含 `refs/heads/main`，恢复时应明确使用 `--branch main`。详情见 bundle-library.json、bundle-restore-result.json。本次保全未创建第二工作树，未本地执行 Go、测试或 SQLite。

## 结论边界

这是已发布 post-DB 检查点的普通 CI 证据，不代表实际物理故障、恢复、进程退出、OS Sync 返回失败或硬件持久性验收。受限手工套件未在这两个 job 执行；整体未验收，M10 未通过。post-DB 新 caller 的本地影响验证与 NOT_RUN 边界仍见同一提交的 `docs/evidence/20261012-postdb/README.zh.md`。

GitHub 的 Node.js 20 action 运行时弃用提示不影响本次两个 job 成功结论；这是工具链提示，不是被忽略的测试失败。
