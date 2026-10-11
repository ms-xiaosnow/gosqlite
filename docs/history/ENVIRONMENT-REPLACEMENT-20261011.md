# 2026-10-11 执行环境替换与恢复记录

## 已确认的中断事实

- 20:38:31 UTC，固定组合树 `9ca02ee7fee55f0eb4abe83330b37b80d02aa4bd` 的 22 项普通验证启动，执行会话为 `54102`。此后尚未收集到任何一项终态。
- 20:38:50 的一次源码只读操作成功；20:38:59 起，旧 `durable-restart` 目录被报告不存在。两个源码工作任务独立观察到同一现象。
- 旧会话恢复返回 `executor key changed during session recovery`。新的命令环境可用，但旧项目目录、临时工具目录和会话日志不可读。
- 按执行器身份变化处理。具体原因未知；没有证据把它归因于项目删除、数据库故障、测试断言或安全审批拒绝。
- 原 `54102` 的终态保持 **UNKNOWN**。不能宣称旧进程已正常退出，也不能将本次恢复当作旧 22 项通过。新环境中所有组合运行证据必须重新取得。

## 持久对象与恢复结果

远端 main 只读核验仍为 `1e8e7fbb694b9234b7797414ae7422a38e08ab7e`，tree
`4e82f2859975a1d8df11b692f732ed29f6a83e6d`。从已验证 Library bundle 本地重建后，
bundle verify、完整 fsck、HEAD/tree 及干净工作区检查均通过，未修改远端。
原 goal.md 的 43,100 字节与 SHA256
`9ae7b1d776856669a10eb753408c7754d015d266f8783240165450e96e4bac81` 保持。

18 个持久文件共 938,692,019 字节已重新取回；17 个已有原始 SHA 的对象全部匹配，
另一个分卷索引在读取后与四卷的顺序、偏移、长度及 SHA 分别交叉核验。
13 个归档恢复了 26,007 个普通文件、1,201 个目录，文件字节和模式零差异。
原 IPK 记录器审阅的一条绝对路径 symlink fixture 没有在新文件系统物化；其目标
保留在原归档和恢复清单中。没有声称 inode、所有权、ACL、时间戳或全部 POSIX
身份与旧环境相同。历史日志中的原绝对路径不重写。

只读 WAL 源码重新核回 `b19ee07536c83d554f70b40d461b0b96ee4e2872`，
IPK 源码重新核回 `96e039f2a0b0990789579bfa5fe20746b6b1ecdb`。
39 个原输入路径和已保存对话中找回的 20 条精确文档替换重新组成 3,177 文件，
tree 恢复为原 `9ca02ee7fee55f0eb4abe83330b37b80d02aa4bd`。
这证明源码内容身份，不能恢复旧进程结果。

恢复后的组合已另存 Library `libfile_6e270b5803a88191a036503886487c5d`，
12,196,455 字节，SHA256
`10e11be7528d7668daf6701fc974ee52b4d9e60996bb8135595396e250c6c66b`，
3,343 个成员完整读回通过。原 RO 有限 50 PASS、IPK 原始 expected-RED／
修正版 55 PASS／官方 25 PASS 的独立证据仍保留各自树和时间身份。

## 工具引导的普通失败与修正

官方 Go 1.27.2 包重新下载，70,590,635 字节与原固定 SHA256 一致；安装后的
bin/go SHA256 仍为 `548d3c32f83fa81152f543eb1526be7884cfe0526eb5e9ce29d3eca14786e7e0`。
隔离的版本查询返回 `go version go1.27.2 linux/amd64`，exit 0。

第一次 SQLite 构建引导被额外包在空环境中，下载阶段无法解析 www.sqlite.org，
脚本内置重试后 exit 6；没有取得源码或开始构建。第一次固定三个 Go 模块下载
同样因 proxy.golang.org 的 DNS/network unreachable 返回 exit 1，没有取得模块。
这两项是工具引导／网络失败，不能列作数据库安全测试豁免或测试 PASS。

原已审安装脚本只净化 SQLite 子进程，下载器使用已有引导环境。恢复这一契约后，
同一官方 SQLite URL 的单次重试成功，原 14,881,448 字节与 SHA3-256
`a5c29342ca185abcda332448fbb27c536661245be27e20199b46c49099b0e15e` 全部匹配。
未读取或打印凭据、修改网络设置、变更版本或使用未验证镜像。只能据此确认该
引导方式成功，不能推断是哪一个环境变量导致差异。

随后从本地已核验源码包完成相同版本的官方 SQLite 构建，exit 0；生成的
sqlite3.c SHA3-256 仍为
`8efd453d08a7cfc4c79de5923153036cdc4dcaa254d4f1e617be6a2a98828f9e`。
新的二进制为 8,708,688 字节，SHA256
`86169de25c972197f7346d2ecf1176ed449133c09e7f09df3cda19e330350ecd`，
与旧二进制不同，后续验证必须使用新身份。隔离的版本／source ID 查询通过；
没有运行官方源码测试。

三个固定模块 crypto v0.58.0、sys v0.49.0、text v0.43.0 下载会话自然 exit 0，
返回值和独立重算的六个 h1 校验值均与 go.sum 匹配；1,430 个模块文件和 2,284 个
SQLite 文件已对官方归档核验。新环境引导租约已结束；这不改变旧 54102 的 UNKNOWN。
可再分发的官方源码备份共 25,648,768 字节，SHA256
`2a7bedf1b85cc4cec4dc4a53cd0ac13f7a8cbceab20fd6fee42616736f4b4bda`，
包含原源码包、模块 zip/mod/info、许可副本及校验清单，不包含环境变量、凭据或私有缓存。

## 继续验证的边界

使用明确的新恢复目录，不新建旧目录来掩盖丢失，也不把旧会话标为自然释放。
新的普通验证使用固定源码／工具／模块、独立审阅的选择、同一 Go 租约、默认
/tmp 和资源门。只执行既有 21 项普通集合及新增的健康 live-writer 单根预检；
完整 oracle 不省略，41 个 Fuzz 目标仅运行已有固定种子，不开启 campaign。

此前真正受限的 ANN V4、41 目标持续 fuzz、未知安全中断链、物理故障／强杀／
namespace 干扰／真实恢复等限制原样保留。环境替换不提供重试这些链路的许可。
原生外平台、真实持久性、独立安全／法律和 M10 仍未验收。

新环境的 /tmp 为 tmpfs；文件系统容量和挂载标识需按新观测报告，不沿用旧 inode
或旧环境 fsync 结果。普通调用 Sync 的成功不能证明断电持久性。现有实验性
Backup/WAL publisher 的 ext-family、内核能力和协作目录前提没有因此取得资格。

## 证据索引

- `environment-replacement-observations.json`：原会话错误、新路径状态及资源观测。
- `git-recovery-verified.json`：每条 Git 恢复检查及实际退出状态。
- `library-recovery-integrity.json`、`RESTORE-MANIFEST.json`、`restore-proof/`：原对象和解包核验。
- `readonly-ipk-combined-recovery-evidence-v1/`：精确重组、替换和新路径清单。
- `combined-source-library-verified.json`：组合归档读回结果。
- `toolchain-recovery-evidence/`：官方工具下载、原引导失败、重试及实际构建结果。

本记录是开发与恢复说明，不是最终交付或整体验收结论。
