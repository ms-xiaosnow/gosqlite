# Darwin 只读 SHM 后端准备：SOURCE_ONLY / partial

## 本片完成的源码边界

此片只补 `internal/vfs/native_darwin.go` 的 DMS 接入判定与映射保护。
它是 `goal.md` §4.1 / M3 的一项后端准备，不是完整 Darwin 只读 WAL、
M3 或生产安全验收。源码尚待独立审查；本片没有运行 Go、gofmt、AST 测试、
编译、native 测试或官方 SQLite oracle。所有运行状态均为 **NOT_RUN**。

公共支持契约仍为已有的 Linux live initialized SHM 模式。
`sqlite/readonly_wal_policy.go`、`internal/pager/shared_readonly_policy.go`、
`internal/walindex/readonly_policy.go` 三个 Linux-only 门保持原样，
所以 Darwin 的公共 `ReadOnly + WALShared` 仍在 native 获取之前拒绝。
没有修改 Options/OpenReadOnly 的公开声明，也没有添加新公共选项。

## DMS 的实际接入规则

新增可移植、无 descriptor、无 callback、无 native I/O 的
`readonlyDarwinDMSDecision(writable, peer)`，其中 `peer` 是 uint8 底层的
专用类型：unknown（零值）、unheld、shared、exclusive。

- unknown 及任何未声明的数值：`ErrUnsupported`，不初始化。
- 只读 + unheld：`ErrUnsupported`，不初始化。
- 可写 + unheld：允许走已有的独占 DMS 初始化协议。
- shared：允许随后获取共享 DMS，不初始化。
- exclusive：`ErrBusy`，不初始化。

实际 `nativeAttachDMS` 仍先持有 process registry 的锁并检查 poison。
已有 `node.kernel[darwinDMS] == darwinRead` 的本进程共享锁快路保持原样；
Darwin 的 POSIX 锁属于进程，F_GETLK 不报告本进程自己的锁，不能用外部
查询替代这个 registry 事实。仅在外部查询成功后，原生 F_UNLCK/F_RDLCK/
F_WRLCK 被转换为上述 typed peer；未知值保持 unknown。生产函数把真实的
`f.writable` 和该 peer 交给 helper，拒绝返回必须先于独占锁或 Truncate。

可写初始化仍依次执行：独占 DMS → Truncate(3) → 降为共享 DMS。
初始化抢锁失败立即返回（BUSY 不落到共享路径）；truncate 或初始化后
降级失败仍 poison，并保留已经表示的所有权供既有 Close 路径处理。
只读拒绝不获取额外锁、不截断、不释放/关闭别人持有的 descriptor。

## 映射与所有权

实际 `nativeMap` 将 `PROT_READ` 作为基础，只有 `f.writable` 为真才增加
`PROT_WRITE`，然后把这个值传入实际 `unix.Mmap`。`MAP_SHARED` 不变。
没有新增映射绕路或 backing file 增长。通用 mappingSpan、extent 检查、
managed mapping 注册、nativeUnmap、view → SHM slots → DMS → descriptor
的既有清理顺序均未改动。现有只读 SHM exclusive-lock 拒绝也未改动。

这些源码条件不证明 native macOS 行为、真实非特权环境或只读文件系统，
更不证明硬件写保护或输入不可变。

## 新增的有限测试源码（全部 NOT_RUN）

- `TestDarwinReadOnlyDMSDecisionMemory`：直接调用真实 helper，穷举全部
  256 个 uint8 peer 值 × 2 个 writable 值；核对错误类别、零值拒绝、
  唯一初始化条件和输入不变，不创建 descriptor。
- `TestDarwinReadOnlyDMSProductionSourceBoundary`：解析真实 DMS/查询/
  获取/释放/poison 函数的完整 AST 与可读契约形状，冻结 registry 快路、
  probe→typed peer→helper→拒绝返回→独占/截断的真实控制流。
- `TestDarwinReadOnlyMappingProductionSourceBoundary`：核对真实 Mmap 参数、
  extent 及 retirement 函数的完整 AST；不能靠一个未被使用的保护 helper
  通过检查。
- `TestDarwinReadOnlySourceMutationCounterexamples`：仅在内存中修改真实
  源码副本。删除/反转/忽略判定、伪造可写能力、绕开本地 registry、
  将截断/独占获取前移、丢失 poison、强制可写/私有/额外映射和 helper
  初始化副作用等反例，必须由同一源码边界拒绝。反例不落盘、不编译、
  不执行，也不触发 native 调用。

AST 契约是有限的完整函数结构检查，忽略注释、格式和源码位置；不是
通用跨文件调用图或类型证明。契约字面量本身需要独立审查，不能仅因它与
代码一致就认为安全。源码读取只读取已知 `.go` 文件，不读取数据库。
未新增 init、顶层动态初始化、fixture、子进程或 native 测试入口。

后续只有在固定源码、选择根和完整 package/import/init 闭包独立审查后，
才提出确切的有限 model/AST 运行或 Darwin amd64/arm64 交叉编译命令。
当前文档不授予运行权限，不包含可被误当成已运行的命令/结果。

## 仍缺少的实现和验证

- Darwin 可用的公共集成仍需另行一致更新三层 OS 门、API 注释和策略测试，
  以及真实 Darwin 同进程/跨进程 DMS、锁、映射、driver/CLI 证据。
- 冷启动已有 SHM 尚缺可靠的私有 heap index、unreliable-SHM lease、
  DMS 可靠性重查和 WAL generation/suffix 协议，不能仅放开 unheld。
- 缺 SHM 尚缺明确且可证明的一致快照/不可变输入授权机制。
- Linux 真实非特权/只读文件系统的预备环境 qualification harness 仍缺代码。
  本片既没有实现该 harness，也不能将其标成“只差运行”。

不更改 Linux/Windows 后端、session/index 算法、pager/sqlite/driver/CLI、
恢复、Backup、WAL publisher、module、workflow 或任何已有 manual 门。
不恢复 ANN V4、41-target campaign 或先前不明中断链。源码准备不解除
既有 physical fault/crash/recovery/namespace/tamper 等执行限制。
