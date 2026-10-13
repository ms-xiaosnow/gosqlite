# c5a8632：自然完成的 Ubuntu CI 证据

- 仓库：ms-xiaosnow/gosqlite
- 精确提交：c5a8632908cc4747a5f581542a15a8532ef42ad9
- Git tree：43569f28b1c8b2d23c913ac61be41622f143a45f
- 自然触发 run：38323357015；attempt 1；结论 success
- https://github.com/ms-xiaosnow/gosqlite/actions/runs/38323357015
- oracle job 115056281413：success，2026-10-12T23:50:33Z 完成
- ordinary job 115056281469：success，2026-10-13T00:09:27Z 完成
- run 最终更新时间：2026-10-13T00:09:28Z

## 范围与边界

这两个已有 Ubuntu 工作流配置普通测试、源码范围守卫、vet、构建、race，以及固定官方 SQLite 3.54.0 对照测试。只有终态成功的 job/step 才能支持对应步骤通过。它们没有执行受限 manual crash/fault/process-exit 套件，不能作为断电、原生 I/O 故障、OS Sync 失败、硬件持久性或 M10 验收的通过证据。Native/manual cases：NOT RUN。

保存过程只读取现有 GitHub run/job/log 和精确 Git 对象，没有 rerun、cancel、修改 workflow、运行 Go/SQLite/测试，也没有建立第二个工作树。源码快照来自指定提交，不是后续可能变动的工作目录。

## 文件

- ci-evidence.json：精简索引、身份校验、日志长度和 SHA-256、步骤状态、来源和验证边界
- terminal-metadata.json：完整终态 run/jobs API 元数据及日志扫描记录
- *.decoded.log：完整解码 UTF-8 job 日志，未归一化；不是 GitHub 原始日志 ZIP
- source-snapshot/：该提交 workflow 和三个检查脚本的精确 blob
- bundle-library.json / bundle-restore-result.json：完整 Git bundle 的 Library 身份、精确 ID 下载及 bare 恢复校验
- bundle-tree-manifest.json：3,902 个跟踪路径的 Git mode、blob SHA、长度、SHA-256
- SHA256SUMS：归档成员校验值（不包含自身）

完整历史 bundle 已单独保存为 gosqlite-checkpoint-extensions-c5a8632.bundle，Library ID libfile_9f5d934f0c7881918aee2869cd749aae，12,734,677 字节，SHA-256 2817259ab2d87259204f901800d5d6b2c020cde04e1791573ee5e1b47d3f02f4。精确 ID 下载后 bare main 恢复验证 3,902 个路径、mode 与 blob 内容 SHA-256 全部一致，共 61,647,557 字节，git fsck exit 0。

日志经过常见凭据/签名 URL 模式及相关行检查：未发现意外凭据，保留 GitHub 已有 *** 遮罩。模式检查并非绝对无秘密证明。

## 精简 Git 证据包

完整终态元数据、路径清单和两个 decoded 日志使用无时间戳 gzip 压缩；解压后的原始字节保持不变。bundle-proof.json 合并 bundle 身份、恢复结果和 Git 校验输出；archive-library-proof.json 保存完整 ZIP 的 Library 身份与独立下载核验。SHA256SUMS 覆盖本目录其他全部文件。此目录只准备给主任务进行后续 docs-only 证据提交，本保存任务未修改仓库工作树或 GitHub。
