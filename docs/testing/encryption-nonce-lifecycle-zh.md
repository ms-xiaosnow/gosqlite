> 当前增量验证：3个精确普通健康/纯值根normal与race均PASS，format及sourceguard通过；[命令与原始记录](../evidence/20261012-publication-nonce/README.zh.md)。下文保留源审时说明，实际物理负面仍NOT_RUN。

# R6：普通加密生命周期的 nonce 关联断言

本片只补原目标 §8.2:307 的有限关联断言。复用两个已有健康生命周期，
不新增格式/profile 矩阵，不改变生产 nonce/KDF/writer 实现。

## 精确源码与入口

- `sqlite/encryption_nonce_review_test.go`：只读 inventory helper 与纯值域控制。
- `sqlite/encryption_wal_test.go`：`TestEncryptionWALPasswordOrdinaryReopenAndCancellation`。
- `sqlite/encryption_review_test.go`：`TestEncryptionReviewFreshIdentityAndRetainedBytes`。
- 新增纯值入口：`TestEncryptionPublicNonceDomainIdentity`。

三个入口的有限 selector：

```text
^(TestEncryptionWALPasswordOrdinaryReopenAndCancellation|TestEncryptionReviewFreshIdentityAndRetainedBytes|TestEncryptionPublicNonceDomainIdentity)$
```

以上是待验证源码的选择范围，不是执行记录或运行授权。

## 断言的实际含义

同一个测试始终使用同一 application credential。helper 不取得 credential
或实际 master/file key，只记录公开的符号域：

```text
(Epoch{DatabaseID, EpochID, Kind, KDF, KDFSalt, RootSalt}, FileID, file role, record role)
+ nonce = NoncePrefix[4] || big-endian(record sequence)[8]
```

这个域对应 `encryption/key.go` 的 master 与 file 派生输入；nonce 对应
`encryption/record.go` 的 `recordNonce`。不把 geometry、commit/transaction/
parent identity、page/frame routing、plaintext size 或 ciphertext 当成 key
域，以免不同 AAD 掩盖同 key/nonce 的复用。WAL 页号不是公开 record header
字段，本 helper 不伪造它，也不替代生产认证器。原有数据与
`VerifyEncrypted` 断言完整保留。

公开 header/record 来自测试自身正常生成的 intact 文件，通过既有
`fileview.ReadFile` 只读观察；不绕开 Darwin 的 descriptor-close registry。
扫描只接受当前 root、`.base`、`.wal`、`.retained`，跳过准确名称的
`.gse-lock`。固定上限为 64 个目录成员、每文件 4 MiB、每文件 256 条
record、每条 plaintext 最多 64 KiB、全 inventory 4096 个 pair。
每文件按声明顺序读完所有记录，检查 role/sequence、完整 extent 和 EOF。

每次快照中 pair 必须唯一。后续 publication 必须保留此前所有对象的
header 与 SHA-256，保留所有历史 pair，且正好增加一个新 root 和一个
新 base/WAL 对象。旧 root 改名为 retained 后的再次观察不是新的 Seal，
因此通过对象身份和相同完整 bytes digest 识别历史记录，不误报重读。
新 root 必须 FileID 新鲜、commit sequence 增一；新对象必须使用发布后
的相同 epoch，且除 role/record count 外的完整 descriptor 与 root 一致。
WAL 的公开 ParentCommitID 是逻辑 object-head ID，不能误当旧 root 的
bytes digest；本片不额外解密 manifest 来取 PreviousRootHash。snapshot
测试原有的紧邻 parent-root digest 断言保留，不能推广到 WAL。

1. 既有 password/WAL fixture 在取消 commit 后 rollback，inventory 必须
   完全不变；普通关闭并重新打开后、再次 INSERT 前也必须完全不变。
2. reopen 后 INSERT 在相同 epoch 下增加新 WAL/root pair，与全部 retained
   旧对象的符号 pair 一起检查；两行完整 SQL 数据保持正确。
3. 既有正常 Close checkpoint 增加新 base/root pair，并保持同 epoch。
   最后正确 credential 的只读 reopen 与 `VerifyEncrypted` 保持原断言。
4. 既有 snapshot fixture 的普通 rollback 不增加或改变 nonce-bearing
   对象；同 credential 成功 rekey 使用新 epoch/root salt/root FileID，
   新 base/root 与旧 epoch 的 retained 对象一起检查。后续只读 reopen
   不改变该 inventory。原 retained byte 和 SQL 数据断言均保留。
5. 纯值控制核对 nonce 字节编码、各公开派生域/record role 分离，以及
   改 AAD/长度仍得到同一 pair。该入口无 key 派生、数据库或文件写入。

正常成功路径沿用原有 `Close`、只读 reopen 和最后的延迟 Close；新增
reopen 后断言之前给该 writer 添加延迟 Close，以便断言失败也释放句柄。
helper 只使用会关闭所开句柄的既有 `fileview.ReadFile`，不持有文件句柄。

## 有限闭合与未运行边界

本片交付状态为 **SOURCE_ONLY / NOT_RUN**。编写期间未运行 Go、gofmt、
compiler、测试、数据库、native 流程或 CI；未修改生产源码或现有工作树。
集成后先独立源码审查，再按现有授权对精确源码取得这三个入口的有限
ordinary/race 终态及 sourceguard 结果，保存其源码绑定与原始结果即可
闭合 R6 的新增代码片。不需要新 manual tag/controller 或新增 profile。

inventory 不变证明这里的 rollback 没有发布或改写 nonce-bearing 文件，
不单独声称它观察了所有尚未落盘的 writer 状态；既有
`TestWriterErrorsPoisonCopies` 和 `TestWriterFreshIdentityAndNoResume`
继续承担 writer 失败后不可复用、重复写入的纯内存断言。

本片没有 fault、crash、wrong-key、磁盘损坏/负面认证、恢复、ANN 或
campaign 调用。进程中断 rekey 属于 R5；物理/平台/掉电和密码学外部资格
仍为各自 NOT_RUN/未完成，不能从本片推出生产已完整 VERIFIED。
