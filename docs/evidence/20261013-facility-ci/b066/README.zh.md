# b066060：Ubuntu CI 的紧凑提交材料

对应精确提交 b06606020fdbb5b73dd0e9b000773f3f02c6979d，Git tree 67f4c6d73d3645d272ded287061747764e0f3921。
自然触发 run 38321827127，attempt 1，最终 success：https://github.com/ms-xiaosnow/gosqlite/actions/runs/38321827127

- oracle job 115051446927：success，2026-10-12 23:32:30 UTC 完成
- ordinary job 115051447164：success，2026-10-12 23:41:51 UTC 完成
- run 最终更新时间：2026-10-12 23:41:52 UTC

两个 Ubuntu lane 通过了普通测试、源码范围守卫、vet、构建、race，以及固定官方 SQLite 3.54.0 对照测试。受限 manual crash/fault/process-exit 套件未执行；本记录不证明物理断电、原生 I/O 故障、OS Sync 失败、硬件持久性或 M10 验收。Native/manual cases：NOT RUN。

本目录用于之后提交，未改动当前源码工作树。JSON 和固定来源快照逐字复制；两个 .decoded.log.gz 使用 gzip mtime=0，解压后与 connector 返回的完整 UTF-8 日志逐字匹配，保留原始 BOM 与换行。日志是解码表示，不是 GitHub 原始日志 ZIP。compact-verification.json 记录压缩与解压长度及 SHA-256；copied-raw-identity.json 记录来源内容校验值。

完整证据 ZIP 单独保存：gosqlite-b066060-ci-38321827127-success.zip
Library ID：libfile_688e8fb4e8e08191a31d4f3c476c918e
290,500 字节；SHA-256 c8b2f43abeef00b64333237596f5fc26f462fba7dfc280b6a328fc419fba8e5b。
精确 ID 下载验证通过、ZIP CRC 通过、17 个成员逐字匹配。完整 ZIP 包含未压缩日志和全部 3,868 个路径的 bundle-tree-manifest.json；本紧凑目录不重复存放大型树清单、bundle 或 ZIP。

完整 Git bundle 单独保存：gosqlite-rekey-misplacement-b066060.bundle
Library ID：libfile_9bd8c01766e08191a349b3605a63c9ac
11,107,623 字节；SHA-256 bdacbef29dfe3daa4c678e3ffbaa1ac8f38b740ad6ea77daebe4c25a380fc9ce。
精确 ID 下载后 bare main 恢复，3,868 路径/mode/blob SHA-256 与不可变指定提交全部一致；58,931,205 字节；bundle verify 和 git fsck 通过。保存过程中没有 rerun、cancel、GitHub 写入、源码修改、Go/SQLite/测试执行或第二个工作树。

未发现常见凭据或签名 URL 模式；GitHub 的 *** 遮罩原样保留。模式检查不构成绝对无秘密证明。
