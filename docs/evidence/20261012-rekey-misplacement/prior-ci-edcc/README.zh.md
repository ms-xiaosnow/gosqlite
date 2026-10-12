# edcc8be：自然完成的 Ubuntu CI 证据

- 仓库：ms-xiaosnow/gosqlite
- 精确提交：edcc8be3e4c0d0dc46066020dc900a5effcaee30
- Git tree：1149df0b8d733e93597acca07d373d029d4df913
- 自然触发 run：38314186578；attempt 1；结论 success
- https://github.com/ms-xiaosnow/gosqlite/actions/runs/38314186578
- oracle job 115026774110：success，2026-10-12 22:07:29 UTC 完成
- ordinary job 115026774139：success，2026-10-12 22:28:51 UTC 完成
- run 最终更新时间：2026-10-12 22:28:52 UTC

## 范围与边界

这两个已有 Ubuntu 工作流验证普通测试、源码范围守卫、vet、构建、race，以及固定官方 SQLite 3.54.0 对照测试。它们没有执行受限 manual crash/fault/process-exit 套件，不能作为断电、原生 I/O 故障、OS Sync 失败、硬件持久性或 M10 验收的通过证据。Native/manual cases：NOT RUN。

保存过程只读取现有 GitHub run/job/log 和精确 Git 对象，没有 rerun、cancel、修改 workflow、运行 Go/SQLite/测试，也没有建立第二个工作树。源码快照来自指定提交，而不是后来可能变动的工作目录。

## 文件

- ci-evidence.json：精简索引、身份校验、日志长度和 SHA-256、步骤状态、来源和验证边界
- terminal-metadata.json：完整终态 run/jobs API 元数据及日志扫描记录
- *.decoded.log.gz：解压后为GitHub connector 返回的完整解码 UTF-8 job 日志，未归一化；不是 GitHub 原始日志 ZIP
- source-snapshot/：该提交的 workflow 和三个检查脚本的精确 blob
- bundle-library.json / bundle-restore-result.json：完整 Git bundle 的 Library 身份、精确 ID 下载及 bare 恢复校验
- bundle-tree-manifest.json：3,436 个跟踪路径的 Git mode、blob SHA、长度、SHA-256
- SHA256SUMS：归档成员校验值（不包含自身）

完整历史 bundle 单独保存为 gosqlite-root-auth-edcc8be.bundle，Library ID libfile_3e0aa6cabf2c819191a12760cfb302d1，7,965,820 字节，SHA-256 b727892ea7ba6e64e61cb2327d23149364002e0951e3553d6ca034bdc7f1f193。精确 ID 下载后以 bare、main 分支恢复；3,436 个路径、mode 与 blob 内容 SHA-256 全部与原始提交一致，共 53,748,202 字节。git fsck --full 和 git bundle verify 均 exit 0。

日志经过常见凭据/签名 URL 模式及相关行检查：未发现意外凭据，保留 GitHub 已有 *** 遮罩。模式检查并非绝对无秘密证明。

本仓库保存的是紧凑副本：大体积 bundle-tree-manifest.json 与完整 SHA256SUMS 见已验证 Library 全包；这里的 gzip 日志解压后与原日志相同，copied-raw-identity.json（若有）给出原件身份。原 README 中全包目录说明保留为历史索引，不表示全部大文件都在本 Git 子目录。
