# 有限 checkpoint 中断及扩展索引账本自测

状态：SOURCE_ONLY / NOT_RUN。本文件中的命令是留给用户之后在隔离 Linux 环境中的可复现步骤；编写、审阅或合并本文件不代表执行授权，不代表命令已运行。当前没有创建数据库、运行 Go、编译、解析、执行测试、退出子进程或执行重开。真实断电/设备持久性结论仍为 NOT_RUN。

## 固定范围

只有一个 controller、十个封闭 case、三种 authority、一个 315 行组合 fixture。每个 admission 仅运行一个 case；最多 Start 两个子进程，各自只有一个实际 Cmd.Wait。writer 与 probe 不重叠。没有 profile/index 笛卡尔积，没有旧 A5/R5 扩围，没有 ANNV4/campaign。

- exclusive-control：普通返回，主文件为 C，WAL 删除
- exclusive-after-first-main-copy：首个实际主文件页写完，完整原 WAL 保持，C 可重开
- exclusive-after-wal-unlink：主文件已 Sync 且 WAL 已删除，尚未目录 Sync；当前内核必须观察新主文件
- shared-control：普通返回，主/WAL/SHM inode 保留，完整 backfill
- shared-after-first-main-copy：首个主文件页写完，attempted=safe，backfill 仍是原值
- shared-after-backfill：成功 SetBackfill(safe) 后，主文件已 Sync，WAL 保留
- encrypted-control：普通返回，layout2 新 base/root，原 Epoch/同一密钥，无活动 segment
- encrypted-before-root-rename：已验证及准备 candidate，但仍选原 root
- encrypted-after-root-rename：rename 成功后当前内核必须选新 root；不宣称目录持久性
- encrypted-after-root-dirsync：最终目录 Sync 完成、尚未重新打开/采用 root；必须选新 root

C 是已得到真实公开 Commit(nil) 并在独立 control 目录写入、Sync、关闭、目录 Sync、读回核验的最终事务。B 是更早的 312 行且无扩展索引基线，任何 case 重开为 B 都失败。最终 C=7 docs+180 hitems+128 iitems=315 行。seed 恰好两个数据事务和一个未 armed 的公开 preparatory Checkpoint。加密序列必须是 1/2/3/4，待测 checkpoint 为 5；不在运行时调整预期。

## 一、前置条件和隔离

1. 使用本机一次性的 Linux VM/独立测试文件系统，普通非 root 用户，禁止放真实业务数据库、密钥或其他人的文件。确认没有同目录数据库访问者、后台清理器或自动测试器。
2. 先完成该精确源码的审阅及另行授权的有限 pure/AST/source-selection/compile-only 检查。native 三个 TestManualCheckpointRestart* 入口在这些步骤中绝不能执行，连 `-run=^$` 也不能替代 compile-only。
3. 使用当前源码要求的 Go 工具链；记录 `go env GOVERSION`。不自动切换或下载工具链。source/compiler/binary/receipt 都按完整内容散列绑定；任何源码变化都要重新生成独立的新 freeze，不能只重写 pin 掩盖修改。
4. 路径使用不含空白、`<>&` 或 Unicode 行分隔符的普通 ASCII 名称，避免 JSON 编码/环境歧义。`SOURCE` 为已审阅的完整仓库绝对路径；`OUT` 为全新、用户拥有的 0700 保留目录。不要复用任何既有 run 目录。
5. 下面命令只有操作者明确决定开展真实 native 自测时才运行。`independently_authorized` 只是本地门槛文字，不是授予助手执行权限。

## 二、有限源码检查及只编译

以下选择器只覆盖这次新增的有限模型与八个 AST 守卫。若 guard 文件尚未集成、pin 仍为 HOLD 或任一步失败，停止，保留失败内容，不启动 native。

```sh
export SOURCE=/absolute/path/to/reviewed/gosqlite
export OUT=/absolute/private/new/checkpoint-source-freeze
umask 077
mkdir -m 700 "$OUT"
export COMPILER="$(readlink -f "$(command -v go)")"
export GOFLAGS= GOTOOLCHAIN=local GOENV=off GOWORK=off
cd "$SOURCE"
"$COMPILER" test ./internal/testkit/checkpointrestart -run '^(TestCheckpointRestartTenCases|TestCheckpointRestartAuthorityCuts|TestCheckpointRestartCommittedHeadCausality|TestCheckpointRestartTransportBinding|TestCheckpointRestartOwnerWaitAndUnknown|TestCheckpointRestartPrivateFamilyCopies|TestCheckpointRestartEncryptedSameEpoch|TestCheckpointRestartNeverPhysicalPass|TestCheckpointExtensionFixtureRows|TestCheckpointExtensionGramsLedger|TestCheckpointExtensionGraphMembershipLedger|TestCheckpointExtensionQueryPathsAndCheck)$' -count=1
"$COMPILER" test ./internal/testscope -run '^(TestCheckpointRestartGuardInventoryAndGate|TestCheckpointRestartGuardProductionOrder|TestCheckpointRestartGuardObserverPurpose|TestCheckpointRestartGuardPublicEntryAndCommitReceipt|TestCheckpointRestartGuardProcessOwnership|TestCheckpointRestartGuardFullExtensionLedger|TestCheckpointRestartGuardPrivateCopiesAndProbe|TestCheckpointRestartGuardDefaultAndIntegration)$' -count=1
"$COMPILER" test -c -tags=manual_checkpoint_restart -o "$OUT/checkpoint-restart.test" ./sqlite
export GOVERSION="$("$COMPILER" env GOVERSION)"
```

逐条检查实际退出码，失败就停止。default/non-Linux 的源选择应只包含两个空 stub，不能包含 common manual bridge、两个 native adapters 或三个 native 测试入口。R5-only 源选择必须保持原 scope；只有另行明确授权才做 combined-tag compile-only，不能执行其二进制。

## 三、冻结完整源码和 compile-only 收据

下面 Python 仅给用户生成 admission 所需的规范 JSON。必须在源码静止、上一节 compile-only 实际成功后运行。模型及 source 审阅不等于 compile-only 成功，禁止预填成功收据。

```sh
python3 - <<'PYFREEZE'
import hashlib, json, os, pathlib
S=pathlib.Path(os.environ['SOURCE']).resolve()
O=pathlib.Path(os.environ['OUT']).resolve()
C=pathlib.Path(os.environ['COMPILER']).resolve()
B=O/'checkpoint-restart.test'
def sha(b): return hashlib.sha256(b).hexdigest()
def save(p,obj):
    raw=json.dumps(obj,separators=(',',':'),ensure_ascii=False).encode()
    with p.open('xb') as f:
        os.chmod(p,0o600); f.write(raw); f.flush(); os.fsync(f.fileno())
    return sha(raw)
files={}
for p in sorted(S.rglob('*')):
    if '.git' in p.relative_to(S).parts or not p.is_file(): continue
    rel=p.relative_to(S).as_posix()
    if not (rel.endswith('.go') or rel in ('go.mod','go.sum')): continue
    assert not p.is_symlink()
    raw=p.read_bytes(); files[rel]={'Bytes':len(raw),'SHA256':sha(raw)}
manifest=save(O/'source-manifest.json',{'Files':files})
receipt={'Schema':'checkpoint-restart-compile-v1','SourceManifest':manifest,
 'Compiler':sha(C.read_bytes()),'Binary':sha(B.read_bytes()),'Package':'./sqlite',
 'Tags':'manual_checkpoint_restart','GoVersion':os.environ['GOVERSION'],
 'CompileOnly':True,'Flags':['test','-c','-tags=manual_checkpoint_restart','-o',str(B),'./sqlite']}
save(O/'compile-receipt.json',receipt)
fd=os.open(O,os.O_RDONLY|os.O_DIRECTORY); os.fsync(fd); os.close(fd)
print('source and compile receipt retained:',O)
PYFREEZE
```

二进制必须仍是这次生成的同 inode 文件，不能复制后启动另一个路径。入口会读取 `/proc/self/exe` 并与编译输出精确比较，子进程也只允许执行该路径。controller 参数必须完全一致，不能加 `-test.v`、其他 run selector 或 timeout。

## 四、为一个 case 准备 admission，然后由用户启动

先从上方十个 case 中选一个。推荐先分别观察三个普通 control；这不是此文已经执行它们的声明。每个 case 使用全新不存在的 `RUN` 路径，其父目录必须用户拥有且 0700。每个 binding 单独保留，不覆写。

```sh
export CASE=exclusive-control
export RUN="$OUT/run-$CASE"
export FSTYPE="linux-fstype-$(stat -f -c %t "$OUT")"
export BINDING="$OUT/binding-$CASE.json"
python3 - <<'PYBIND'
import hashlib,json,os,pathlib,secrets
O=pathlib.Path(os.environ['OUT']).resolve()
def sha(p): return hashlib.sha256(p.read_bytes()).hexdigest()
run=pathlib.Path(os.environ['RUN'])
assert run.is_absolute() and not run.exists()
a={'Schema':'checkpoint-restart-binding-v1','Trial':secrets.token_hex(16),
 'Case':os.environ['CASE'],'Root':str(run),'Filesystem':os.environ['FSTYPE'],
 'Declaration':'linux-process-exit-no-powerloss-retain-all',
 'SourceRoot':str(pathlib.Path(os.environ['SOURCE']).resolve()),
 'SourceManifestPath':str(O/'source-manifest.json'),'SourceManifest':sha(O/'source-manifest.json'),
 'CompilerPath':str(pathlib.Path(os.environ['COMPILER']).resolve()),
 'Compiler':sha(pathlib.Path(os.environ['COMPILER']).resolve()),
 'BinaryPath':str(O/'checkpoint-restart.test'),'Binary':sha(O/'checkpoint-restart.test'),
 'CompileReceiptPath':str(O/'compile-receipt.json'),'CompileReceipt':sha(O/'compile-receipt.json')}
p=pathlib.Path(os.environ['BINDING']);raw=json.dumps(a,separators=(',',':')).encode()
with p.open('xb') as f:
    os.chmod(p,0o600);f.write(raw);f.flush();os.fsync(f.fileno())
fd=os.open(p.parent,os.O_RDONLY|os.O_DIRECTORY);os.fsync(fd);os.close(fd)
print(hashlib.sha256(raw).hexdigest())
PYBIND
export BINDING_SHA256="$(sha256sum "$BINDING" | cut -d ' ' -f 1)"
env -i GOFLAGS= \
 GOSQLITE_MANUAL_CHECKPOINT_RESTART=independently_authorized \
 GOSQLITE_CHECKPOINT_ROLE=controller \
 GOSQLITE_CHECKPOINT_BINDING="$BINDING" \
 GOSQLITE_CHECKPOINT_BINDING_SHA256="$BINDING_SHA256" \
 "$OUT/checkpoint-restart.test" \
 '-test.run=^TestManualCheckpointRestartController$' \
 -test.count=1 -test.parallel=1 -test.shuffle=off -test.timeout=0
```

必须保留实际 controller 退出码、所有 output 和整个 RUN。不要把 writer/probe 当独立测试启动；fd3/fd4、challenge、精确 source/binary/case/profile/fixture/role/generation/sequence 都由 controller 授予。不得 kill、Signal、再次 Wait、替换 executable、切换 credential/profile、绕过 grant 或根据 stdout 字样推断完成。

要完成十个有限 case，可以逐个改变 CASE 重复第四节；每次都必须生成新 RUN 与新 binding。不得对失败 case 自动重试，也不得扩大行数、改种子、弱化断言或添加运行矩阵。持续阻塞时不能靠退出 controller 伪造清理，见下一节。

## 五、应检查的完整证据

- `control/committed-head.json`：实际 C 的 Commit(nil) 来源、完整账本 digest、selected head、source/binary/profile/fixture，先于待测 checkpoint。
- `control/seed.json`：所有 13 个有序完整 SQL 流，包括类型/NULL/完整 BLOB，全部真实 EOF、Rows.Err(nil)、一次 Rows.Close，三种索引完整 schema/catalog/shadow 数据；不能只看 COUNT/hash。
- 7 grams 行产生 96 posting、68 frequency；HNSW 180 个完整 live node/映射，完整 adjacency 原字节；IVF 全部 128 membership/4 centroid，独立 float32→float64 距离判定每个归属。
- 六次真实公开搜索：grams indexed/scan 都返回 1,2（分数 2/3）；HNSW indexed/exact 返回 20,19,18,17,23；IVF 单 list indexed/exact 返回 1..5。精确计划和全部 stats 随账本保存，不能以 EXPLAIN、fallback 或仅正 ANN counter 替代。
- 三次 CheckExtensionIndex(nil) 和完整 Check：Tables=12、Indexes=9、ExtensionIndexes=3、TableRows=979、IndexEntries=789、ReservedPages=0；Pages/FreePages 固定于 C，PageReads 有界。
- `writer.ready.json` 和有界二进制 event transcript：writer 先完整核验 C，再获得唯一 checkpoint grant。只在首个 copy 发送 barrier；之后每页在 bridge 中进行有界严格递增计数，terminal cut 必须已见全部 selected 页。
- `writer.process.json` / `probe.process.json`：每个一个真实 Start、一个 owned Wait、实际 ProcessState、非 signal、正确 exit code，以及 terminal pipe EOF。目标中断 writer 是 86，control/probe 是 0，协议失败是 87。
- 原始 `original`、已退出 `trial`、`evidence/post-exit` 全 family manifest 最终保持不变。probe 是第四套不同 inode 的私有副本。plaintext probe 为正常可写模式 Open，允许合法 SHM 重建/失效；main/WAL 内容保持。加密 probe 为相同密钥 ReadOnly/NoCreate/layout2。
- 加密 selected root old=1320 字节/new=1064 字节；candidate ParentCommitID 必须是已认证旧 HeadCommitID，不能错用 root SHA256；整个 Epoch 不变。NEW probe 认证序列5、零活动 segments/frames/bytes，新的 base HeadCommitID。
- `control/observations.json` 分开记 SourceModelConsistent、PublicCheckpointReturned、ActualWaitObserved、CommittedHeadRecovered、PhysicalValidated。它们不是一个笼统 PASS；`PhysicalValidated` 始终 false。当前 caller 对共享的完整账本与 authority 标量映射实际调用纯谓词，另记 `LedgerModelConsistent` / `AuthorityModelConsistent`；它没有凭空构造完整模型的概念事件历史，因此 `FullModelEvidenceEvaluated=false` / `SourceModelConsistent=false`。这不应被改写成全部模型完成或物理 PASS。这是同内核受控进程退出观察，不是实际掉电、磁盘扇区原子性或目录丢失耐久性认证。

## 六、失败、UNKNOWN 与保留

任何漏 cut、额外 frame、错 grant、错误身份、rows 不完整、少索引、错误 epoch、旧 B、混合账本、非预期 busy/unsupported/corrupt/authentication/Sync 错误都失败。不能改为“Open 成功就算通过”。第一原因和实际错误对象、owner、frame、stdout/stderr 均保留，不能重试 Close/cleanup API 把错误改成成功。

若 Start/transport/Wait 不明确，禁止后续 family copy 和 probe。已经 Start 的 child 若未有实际 Wait，controller 保留原 Wait channel 被动等待；Wait 没有 ProcessState 则维持 UNKNOWN。即使后来有真实 Wait，也只记 late disposition，原失败不恢复为成功。需要操作者在隔离环境外决定如何处置，不能向测试增加 Kill/Signal 或第二次 Wait。

本设施不清理证据、不删除 retained/pending/old ciphertext、不 promotion 旧 root。完成后先备份完整 RUN、binding、manifest、compiler/binary hash、compile receipt 和退出码。仅当用户确认所有进程真实结束且无需继续调查，才可手动删除本次全新创建、所有权已核对的 OUT 子目录；不要对仓库、原业务路径或未知目录使用清理命令。
