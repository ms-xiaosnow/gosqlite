# Native encrypted storage on Darwin: bounded implementation

## 2026-10-11 17:02 UTC 开发资格补记

本次 Darwin 有界清单/诊断源码组合 `8a324a8cedeccc8ba46234924c8074890aefa1da`
的 21 项限定普通验证已全部闭合。ordinary/race 各 5,614 PASS、7 个固定预期
SKIP；完整 SQLite 3.54.0 oracle 为 7,246 PASS、零 SKIP；LIMIT oracle/race
394 PASS。Darwin 两架构及 Windows 测试二进制仅编译并核对符号，未运行。
完整身份、原始记录器失败和修正、支持域及缺口见
[Darwin 清单专题的本次补记](darwin-unicode-inventory.md)。

本文形成时的已发布父基点为 `3ea6d3ab15f678fe48bee312921da60828845869`，
其精确 Library 恢复 17 门和 CI [38149634238](https://github.com/ms-xiaosnow/gosqlite/actions/runs/38149634238)
已通过；只属于该父基点。本批仅同步文档，所有 813 个非 docs 且非 README
文件与上述已验证组合逐字节/模式一致。新提交、精确恢复和 CI 要分别读取
其实际终态，不预写自指 SHA 或未来 PASS。历史段落的“未发布”、旧 main、
NOT_RUN、PASS/FAIL 保留各自写作时点，不能混作当前资格。

原生 Darwin/Windows、实际持久性/故障/恢复、安全审查和 M10 均未验收。
本次没有合入另行开发的原生 Unicode 手工入口、strict plaintext backup、
WAL-tail publisher、只读明文 WAL 或 IPK 修正；相关未实现/未合入项仍分别跟踪。

当前两目标编译为 Darwin/amd64 和 Darwin/arm64，各精确 99 Test + 1 Fuzz 符号；没有在 macOS 上执行 initializer 或测试。实际枚举目录/Fstatfs/Unicode sibling 手工入口不在本源码树内。

Date: 2026-10-10. Targets: Darwin/arm64 and Darwin/amd64, CGO disabled.
**Native runtime execution is NOT_RUN.** This source/compile checkpoint does
not qualify M3, M8, M10, security, interoperability or physical durability.

Windows comparison updated 2026-10-11: only the Windows rekey comparison below
is superseded. Other source/compile statements retain this Darwin checkpoint's
original scope; no Darwin backend or native execution is added by this update.

## Capability and remaining work

| Capability | Implementation | Evidence boundary |
| --- | --- | --- |
| Encrypted modes 0/1/2 | Managed descriptors, immutable selection and native byte locks | Native runtime NOT_RUN; target source compiled only |
| Shared-mode rekey and legacy-to-shared upgrade | Atomic POSIX lifecycle conversion with local-owner conflict checks | Memory state laws; no native acceptance |
| Plain/encrypted managed same-inode probes | One cross-package physical-close gate and inode registry | Portable synthetic-resource laws; target test compiled only |
| Relative namespace and publication | openat/fstatat, nofollow, renameatx_np(RENAME_EXCL), renameat replacement | Target source/test compilation only |
| Local filesystem and regular-file policy | fstat/fstatfs, MNT_LOCAL, single-link regular members | No native filesystem qualification |
| User target basename | 可打印 ASCII，最长 177 字节，预留 78 字节生成后缀 | 现有组件策略不变，Unicode 父路径原已允许 |
| Directory contents for inventory | 当前未发布候选只放行可证明无关的有效 UTF-8 同级名称，并核验实际枚举 owner 的本地 APFS 或 hfs subtype 0–3 | 限定源码审阅满足要求；固定源码的 Linux/amd64 精确 20 项模型/AST normal/race PASS；额外编译门及原生验证仍 NOT_RUN，见[清单证明](darwin-unicode-inventory.md) |
| Unicode target basenames | **未实现的一般能力** | 本次不实现一般 Apple Unicode 等价/根名支持；真实目录 Unicode 同级资格 harness 另为 IMPLEMENTATION_MISSING |
| Sync | fsync for regular files and directories | No F_FULLFSYNC/device-cache or power-loss qualification |
| Windows shared-mode rekey comparison | Guarded conversion source now exists in the separate Windows composition | It is not atomic POSIX conversion; post-conversion same-source-key authority check is required; native Windows remains NOT_RUN |

Modes `Snapshot=0`, `WAL=1`, `WALShared=2`, authenticated layouts 1/2/3,
coordination protocol 1, KDF/record framing, content commitments and archives
are unchanged. Existing Linux OFD and Windows native syscall sources are
unchanged. Unsupported operating systems and Windows architectures remain
fail-closed. The existing Go transaction/replay/publication algorithms remain
in use; this is not another database engine or a lock-only fallback.

## Shared physical ownership, separate logical protocols

Darwin POSIX fcntl locks belong to a process. Closing any descriptor for an
inode removes that process's locks, even when the closing descriptor never
acquired a lock. A private encrypted-only registry would therefore be unsafe:
a plain VFS inspection probe could release an encrypted connection's locks.

`internal/darwinfile` owns the common lifecycle gate and physical descriptor
records for both backends. Native open, fstat identity discovery, registration,
all successful kernel-lock ledger changes, and physical close are serialized
under this gate. The identity is `(device, inode)` from the opened descriptor.
Plain VFS and encstore retain their separate per-owner interval arbitration;
SQLite's ranges/DMS and the encrypted three-byte protocol do not change.
Each domain publishes whether any of its inode's kernel locks remain, while
still holding the same gate used for the native lock call.

Every descriptor enters the shared registry before validation or failed-open
cleanup can retire it. A logical close first releases only its owner's logical
holds. Physical close waits until neither domain has a process lock on that
inode. Successful last-hold release drains already logically closed records.
The physical close function strongly retains the actual `os.File`; neither
GC nor a finalizer can close it underneath another owner. The encrypted API
exposes only `managedFile` leases, never `*os.File` aliases or raw handles.

Deferred regular descriptors can be reused by access mode. Encrypted reuse
also requires the same parent directory identity, transfers the directory pin
to the new logical lease's parent, and resets sequential position. Publicly
closed leases remain closed; reuse creates a new logical lease. Plain VFS
preserves its existing access/role reuse policy. The managed inode budget is
256 descriptors and the process budget is 16,384, shared across both domains.
Admission precedes native open. A raced identity may discover one overflow
record; it is retained safely and all further physical opens are refused until
that overage drains. No live descriptor is evicted to satisfy a limit.

An fstat failure after raw open cannot justify closing an unknown inode. The
raw integer, without an `os.File` finalizer, enters quarantine. All further
physical opens stop. Its one close attempt waits until every inode in both
domains has no process lock. A close error is terminal uncertainty: the numeric
descriptor is never retried, the tombstone remains charged, and further opens
remain refused. Known-inode close uncertainty poisons that inode and remains
strongly retained. Failed unlocks keep the unreleased successful ledger entries
and may be retried by internal ordered cleanup; ordinary I/O remains poisoned.

Retirement hooks are private, synchronous, gate-owned bookkeeping. They do not
acquire any per-file mutex, perform application callbacks or create new opens.
Native close is marked attempted before invocation. Parent-close errors and
unexpected hook panics cannot cause retry of a consumed descriptor number.
Ordinary data I/O holds its lease mutex, not the process-wide lifecycle gate.

This containment is not universal resource recovery. A common Store or failed
open may discard the caller's retry route while an exceptional resource remains
retained for process lifetime. There is no background collector. Arbitrary
unmanaged application descriptors or independent SQLite libraries inside the
same process cannot be intercepted; their same-inode closes remain outside the
supported Darwin contract. Official SQLite interoperability requires a separate
process and has not been run on Darwin here.

## Stable three-byte coordination

`.gse-lock` remains stable, empty, regular and single-link:

- Byte 0: shared lifecycle for mode 2; exclusive for creation, legacy writable
  sessions, explicit upgrade and globally quiescent rekey
- Byte 1: exclusive writer reservation
- Byte 2: short shared root-capture or exclusive publication gate

The inode ledger tracks each logical lease's exact byte holds separately from
the aggregate kernel mode. Same-mode repeats are idempotent. Local exclusive
conflicts return BUSY before a syscall. Multiple local readers share one kernel
read lock; closing one cannot release another's hold. Only a successful native
`F_SETLK` changes the aggregate and local ledgers.

Shared-to-exclusive and exclusive-to-shared conversion use native F_SETLK
without an unlock gap. A contended upgrade preserves the old shared hold.
Unexpected acquisition/release errors poison the inode rather than being
reported as safely retryable BUSY. Cleanup releases capture, writer, then
lifecycle; partially successful cleanup retains every remaining exact hold.
Windows uses a separate guarded admission/conversion protocol with a real gap,
not this atomic POSIX conversion. Its post-conversion same-source-key authority
check is documented in [the Windows protocol](windows-shared-rekey.md); native
Windows remains NOT_RUN. This comparison does not change Darwin backend code.

## Namespace, identity and lifetime

Initial absolute/canonical selection uses Darwin's descriptor-free Lstat and
EvalSymlinks. The selected parent then receives a managed O_DIRECTORY handle.
Members are opened with one validated basename, relative to that retained
handle, using `O_NOFOLLOW | O_CLOEXEC | O_NONBLOCK`; no truncating disposition is
accepted. Enumeration opens `.` through the retained parent and has its own
managed directory handle. It uses the exported x/sys `SYS_GETDIRENTRIES64`
call directly, not Go's or x/sys's ReadDir/Getdirentries emulation: those
wrappers create hidden directory descriptors and discard some close errors.
The native ABI is checked against Apple's syscall 344 and pinned Go's Darwin
Syscall6 stubs. A 64 KiB stream buffer and at most 64 returned names bound each
batch. A strict memory-tested parser consumes only the returned byte count,
never the trailing extended-flags word. It validates record/name lengths,
termination and separators, skips zero-inode and dot entries, and performs no
lazy metadata opens. Direct-kernel ABI and native-version support remain
compile-only limitations requiring approved native validation; there is no
weaker enumeration fallback.

Before data access, fstat checks actual type/identity/link count and fstatfs
requires local storage. Named identity uses `fstatat(AT_SYMLINK_NOFOLLOW)` through
the retained directory. Held descriptor identity, original size/time evidence,
single-link count and named parent identity are checked. These metadata probes
open no extra descriptors. `os.SameFile` compares Darwin stat snapshots without
opening a lazy Windows-style metadata handle.

A directory's logical close is permanent, but its actual descriptor stays pinned
until every child physically retires. A retained child can still make relative
identity checks through that pin; public directory operations cannot reopen the
closed lease. Known or unknown child close uncertainty retains the parent pin.

2026-10-11 当前未发布候选补记：ASCII 大小写折叠家族计费仍保守处理 APFS/HFS
普通大小写别名，也可能在大小写敏感卷多计名称。原先所有非 ASCII 同级名称均
拒绝的限制，现只对“有效 UTF-8 且在完整 ASCII 根名内部、遇到 Unicode/控制
字节前已经出现折叠后差异”的名称给出暂定非成员结论。Kelvin sign、long-s、
组合符/忽略字符先于差异、完整根名后才有 Unicode、非法 UTF-8 均不满足证明。
未知 ID/生成后缀继续拒绝；这不是完整 Apple Unicode normalization/case 算法。

生产分类还须使用实际 ReadDir 枚举 owner：持有 lease mutex，经 rawLocked 和
目录标志检查，在同一 descriptor 上一次 Fstatfs，要求 MNT_LOCAL 加精确 NUL
终止类型 apfs，或 hfs subtype 0–3；APFS 不受 HFS subtype 上限限制。未知/非
本地类型不能因 ASCII 前缀差异被跳过，没有第二次 open 或查询失败后的降级。
配额检查仍先于分类；无关名称计扫描项但不打开/计字节，选中成员保留 checked
namespace、owned Close 和物理字节计费。六种批内失败以 readErr 为首保留诊断，
只有字面 io.EOF 清洁终结，原取消与有界清理约束不变。

根名上限仍为 177 字节可打印 ASCII，最长 78 字节生成后缀预留不变，父路径可
含 Unicode。根名 profile 外的既有提前拒绝时机不变；目录内容限制仍在 inventory
执行，不承诺先于每次 provider/credential 或 stable-lock 操作。一般 Unicode
根名能力仍未实现，不能把该缺口与“已有测试但未运行”混为一谈。

限定源码审阅已满足要求。V1 文档的 normal/race NOT_RUN 保留历史身份；固定
源码的 Linux/amd64 精确 20 项模型/源码 normal/race 现已各一次 PASS，额外
编译门和原生 Darwin 执行仍 NOT_RUN。AST 只是结构回归辅助，不证明控制流、锁范围或
绑定。现有两个 nil-directory 分类调用只有 ASCII 名称；真实 managed-directory
加 Unicode-sibling 的资格 harness 为 IMPLEMENTATION_MISSING，另与原生执行
NOT_RUN 分列。上文历史编译证据不转给新候选。完整证明、六项诊断、历史 FAIL/HOLD
和精确身份见[清单专题](darwin-unicode-inventory.md)。

The source and destination of publication remain in the same pinned directory.
The candidate is opened managed/nofollow and its handle/name identity is checked.
Initial publication uses native `renameatx_np(..., RENAME_EXCL)`; replacement
uses `renameat`. The destination is then checked against the candidate inode and
single-link/type constraints. There is no check-then-replace approximation,
link/unlink, copy/delete, cross-volume or weaker fallback. Unsupported native
rename/filesystem capabilities return an error. Existing candidate Sync,
directory Sync, authentication and uncertainty rules are preserved.

Initial path resolution and metadata comparisons are cooperative observations.
They do not prove safety against a hostile actor racing the parent hierarchy,
retargeting names or modifying already-open file bytes. Keep the hierarchy under
trusted application control. None of those hostile operations were executed.

## Verification and sources

The focused command log and exact source hashes are in
`docs/evidence/native-encrypted-darwin-components.txt` and
`docs/evidence/native-encrypted-darwin-source-sha256.txt`. Portable tests use only
synthetic resources/callbacks for exceptional close, lock and quarantine laws.
Encrypted generic test inspections use `internal/testkit/fileview`: Darwin
reads through the managed VFS, while other platforms retain `os.ReadFile`.
This avoids dropping live `.gse-lock` holds during ciphertext-marker checks.
Linux correct-key intact histories and frozen vectors are separate regression
evidence. Target-only ordinary tests are compiled, never launched here.

No native Darwin or Windows execution, native process acceptance, failed native
I/O/lock/close/Sync, short write, disk tamper/wrong-key/missing-selected-file test,
retarget or namespace race, crash/process exit, hot recovery, interrupted
publication/rekey or physical-durability acceptance was run here or elsewhere.
Security/legal review and M3/M8/M10 qualification remain independent open gates.

Authoritative APIs checked on 2026-10-10:

- Apple [fcntl process-lock semantics](https://developer.apple.com/library/archive/documentation/System/Conceptual/ManPages_iPhoneOS/man2/fcntl.2.html)
- Apple [open/openat manual source](https://github.com/apple-oss-distributions/xnu/blob/main/bsd/man/man2/open.2)
- Apple [renameat/renameatx_np and RENAME_EXCL manual source](https://github.com/apple-oss-distributions/xnu/blob/main/bsd/man/man2/rename.2)
- Apple [APFS normalization/case-preservation policy](https://developer.apple.com/library/archive/documentation/FileManagement/Conceptual/APFS_Guide/FAQ/FAQ.html)
  and [Unicode character data](https://www.unicode.org/Public/9.0.0/ucd/UnicodeData.txt)
- Apple [64-bit directory-entry ABI](https://github.com/apple-oss-distributions/xnu/blob/main/bsd/sys/dirent.h),
  [syscall declarations](https://github.com/apple-oss-distributions/xnu/blob/main/bsd/kern/syscalls.master),
  and [getdirentries64 implementation](https://github.com/apple-oss-distributions/xnu/blob/main/bsd/vfs/vfs_syscalls.c)
- Pinned `golang.org/x/sys v0.49.0`: Darwin Openat, Fstat/Fstatat/Fstatfs,
  FcntlFlock, Renameat and RenameatxNp wrappers and constants
- Pinned Go 1.27.2 Darwin `os.File` and `os.SameFile` behavior, and the existing
  plain managed Darwin protocol in ADR 0015

No new dependency, C engine, CGO production path or copied Apple implementation
was introduced. Official sources are API/protocol references only.
