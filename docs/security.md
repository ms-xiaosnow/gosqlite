# 安全合同与发布门

Test-scope correction: historical full suites from M2 through 7addc493 included two namespace-mutation tests. Their PASS outputs remain factual, but are not clean ordinary-scope evidence. See [the audit and gating correction](test-scope-audit.md).

This is a development project, not production-ready. M2 can modify a supported SQLite subset. It validates supported catalog grammar and whole-file page ownership, holds native Linux locks, bounds dirty pages and refuses layouts it cannot preserve. No crash/recovery durability qualification exists. Never use irreplaceable data with this development build. No security or legal review has been completed.

The encryption package implements experimental memory-only AES-256-GCM authenticated-record primitives with unique writer nonces, bound file/page/frame identity, explicit raw-key versus passphrase modes and bounded Argon2id parameters. The M8 snapshot and dirty-page WAL storage slices connect these primitives to SQL and code-configured driver connections, immutable authenticated roots/bases, rekey, encrypted backup/restore, and explicit plaintext export/import. Set `sqlite.Options.Encryption` to select it; nil keeps the ordinary plaintext engine. Immutable authenticated WAL segments, bounded overlay replay and checkpoint are implemented; Linux EncryptionWALShared implements concurrent readers and one writer; Windows/amd64 native coordination is implemented but unexecuted; Darwin encrypted coordination is implemented but unexecuted, while Darwin Unicode inventory, Windows shared rekey and full M8 acceptance remain open. The independent container is not SQLite/SEE ciphertext compatibility; no SEE private source is accessed. Keys must enter only through code-configured providers, never argv, DSNs, SQL, logs, repository files, or examples with usable static secrets.

AEAD cannot prevent whole-family historical rollback without externally protected freshness state. Under a reused credential, an unrelated complete valid encrypted family can be substituted unless the application also checks an externally protected expected identity. Internal parent hashes, commit sequences and authenticated roots are not that external expectation; the public storage APIs do not accept an expected-root identity/freshness token. Metadata sizes/IDs/KDF costs, access patterns, process memory and Go GC copies remain risks. The library's best-effort clearing does not erase all copies or expanded keys. Publishing a production-security claim requires independent review and the full negative/recovery gate.

Encrypted snapshot publication streams a fresh full base and authenticated root, verifies the complete object and its expected child identity/descriptor/hash, syncs files/directories, retains prior ciphertext and replaces the sole current root. The encrypted path writes no plaintext journal, WAL, staging or spill file. Snapshot mode is O(database size) per changed commit. Explicit WAL mode writes only final dirty page images into a fresh authenticated segment and selects the entire ordered graph in a new root. Checkpoint/rekey materialize a fresh base. Both use memory-only ordinary rollback. Linux OFD leases on a persistent separate `.gse-lock` inode are held for the connection lifetime: EXCLUSIVE for a single writable connection or SHARED for read-only connections. Those legacy modes do not permit a writer beside another connection. Explicit EncryptionWALShared/layout3 instead retains shared lifecycle ownership, a separate writer byte and short root capture/publication gate. Each operation pins one authenticated selection; rekey requires every other connection to be closed. Each connection still permits one local reader/Tx lease.

Physical files are no-follow/nonblocking regular single-link objects, exclusive-created with mode 0600; the library never deletes the stable lock inode. These checks are not a complete hostile-filesystem defense. Live Linux encrypted family operations use a pinned parent-directory descriptor, relative no-follow opens and rename/publication, plus identity checks. Initial path resolution and a hostile filesystem/OS remain outside a complete defense; namespace-race branches have no executed acceptance evidence. Use trusted, access-controlled directories. Publication failures at or after a rename attempt poison the session with commit uncertainty; automatic root fallback, candidate cleanup and recovery selection are absent. Source ordering is not power-loss qualification.

Rekey rewrites the current snapshot with fresh epoch/file keys while preserving the logical database ID, but old credentials still decrypt retained history. There is no historical revocation, garbage collection, or secure erase. Publication quotas default to 256 successful root generations, 1 GiB physical family bytes and 16,384 scanned directory entries, with hard maxima of 2^20 generations, 1 TiB and 2^20 entries. Inventory includes old roots/bases and abandoned candidates, reserves the next publication, and counts unrelated names toward scan work. That directory bound is not an inode quota: one commit may add entries that make the next scan refuse. A portable encrypted backup/restore into a fresh family resets history accounting without deleting the source.

`ExportPlaintext` deliberately writes a named ordinary plaintext SQLite file; partial plaintext output may remain on error. `ImportEncrypted` keeps a pinned read-only rollback-mode source, refuses native WAL import, and never erases the original plaintext. Rekey/copy validation checks supported catalog grammar, B-tree/overflow page ownership and freelist membership, not a complete index-to-table content proof. `VerifyEncrypted` authenticates the complete selected physical head, including every referenced WAL segment, not all SQL semantics or historical freshness. Copying the main root alone is not a backup; use the portable encrypted backup API.

Password KDF costs are bounded and validated before expensive work. Default/minimum Argon2id costs are 64 MiB and 3 passes (default 4 lanes); hard maxima are 256 MiB, 6 passes, 8 lanes and 1,572,864 KiB-passes. Nil/zero derivation budgets share a process-wide one-derivation semaphore; explicit budgets of 1–8 must be shared across connectors. The synchronous inner Argon2 computation cannot be cancelled mid-derivation; context is checked before/after it and while waiting for a budget slot. Providers are trusted application code and own their own resource behavior, credential security and cleanup. Request IDs received before authentication must not select arbitrary external accounts/endpoints/credentials.

Known platform-restricted crash/fault/process-exit workflows must not run here or be redirected to CI/another executor. Encrypted wrong-key-on-disk, tampered-on-disk, crash/forced-exit, hot recovery, failed sync, short/wrong-offset writes, permission faults and interrupted rekey/publication workflows likewise remain unexecuted and must not be relocated elsewhere. Ordinary deterministic unit/read/write/rollback/BUSY/race and cross-process lock checks run at M2. The encrypted slice adds correct-key/intact ordinary workflow evidence, while primitive/manifest negative cases change only memory. Memory-only malformed-input checks do not exercise corrupt disk files. Manual verification instructions and unexecuted gates remain explicit.


In ordinary plaintext rollback mode, a writable connection resolves its database path once so cwd/final symlink changes do not intentionally redirect later operations. Physical pathname/inode stability is checked while a transaction is open. Shared rollback snapshots are held by Rows/Tx lifetimes, not by idle connections; encrypted snapshot leases instead last for the whole connection as described above. Index maintenance is atomic at statement-overlay level; operational failures during physical publication are conservatively poisoned and remain unqualified without restricted manual evidence. Native rollback journals are plaintext and mode 0600; ordinary M2 storage provides no encryption/confidentiality protection.


M3 exclusive WAL is an explicitly restricted coordination model. The connection owns one exclusive database descriptor until Close; lease operations do not release that lock. A strict WAL scanner bounds retained index entries, total frames/bytes and index-processing work, rejects unknown versions/salts/checksums and refuses unsupported tails. WAL checksums provide accidental-damage detection, not authenticity; a correctly checksummed WAL from another database cannot be cryptographically distinguished. This native SQLite WAL is plaintext; its checksums supply no authenticated history or rollback defense. The separate encrypted container uses its own authenticated segments.

The source-only rollback recovery operation is not automatically reachable through normal Open/Query/Exec. Cold, unknown and ambiguous journal files remain untouched. Its restricted physical replay/error branches have no executed acceptance evidence. Sector-neighbor and sync-order source fixes plus normal checkpoint tests are not a power-loss guarantee. Shared-memory concurrency is implemented and ordinarily tested; plain Darwin/Windows native backends are implemented but their runtime remains unverified.

M5 similarity operations enforce per-text tokenizer limits plus cumulative candidate/input-byte/set-comparison/DP/result limits, propagate live contexts and return no partial successful result on errors. Exact plans never use lossy candidate cutoffs. User-supplied registry handlers, driver.Valuer implementations and custom mathematical callbacks remain trusted application code; their own allocation, blocking or side effects cannot be bounded by the engine. Native SQL filter parsing accepts only one expression and binds parameters through the normal parser. Vector primitives reject nonfinite values and malformed/unknown encodings before unbounded allocation; their BLOB checksum/authentication is intentionally absent and is not an encryption format. These source reviews are not independent release security certification.

See [encryption-primitives.md](encryption-primitives.md) for primitive-specific credential/nonce lifecycle and memory-only evidence, and [encryption-storage.md](encryption-storage.md) for the implemented root/footer/storage semantics, complete API/options/limits, retention policy and remaining gates. Component negative tests change only in-memory byte slices and do not qualify disk tamper/recovery handling. Verified ordinary encrypted runtime is Linux/amd64 only. Native Darwin and Windows backends are implemented but their runtime qualification, independent security review and production acceptance remain outstanding. No ciphertext format is stable or production accepted.

Encrypted WAL bounds active segment/frame/root bytes, overlay entries, open files,
scan/operation I/O, replay work and aggregate adapter memory separately from dirty
transaction pages and retained history. Estimates include overlapping old/new
overlays and minted file-ID registries, including both masters during rekey.
They do not bound application-owned inputs, trusted callbacks or all runtime/GC
retention. New roots authenticate exact ordered descriptors, parent content IDs,
complete extents and footer hashes; this is not an external freshness oracle.
Source review and correct-key ordinary tests are not cryptographic certification.

Migration inspection is local and read-only. Reports include schema/object names
and public statistics, so treat them as application metadata. Input JSON has
strict size/depth/token/object limits; real-array errors omit source values.
The catalog exporter is a review artifact and has not been run against PostgreSQL.
No migration report authorizes destructive remote changes or automatic retuning.

Later Windows source review found metadata handles outside the plain managed
retirement model, including stdlib identity probes with discarded close outcomes.
The subsequent bounded metadata/public-path correction is implemented and source-
reviewed; see native-windows-metadata.md. Its native execution is still NOT_RUN.
Prior cross-builds and
portable ownership tests are retained as scoped evidence, not all-handle proof.
No physical fault was run and no data-loss result is asserted. Linux managed OFD
behavior is unchanged by this source finding.

Windows encrypted namespace operations use managed native relative handles and
strict local fixed-NTFS/filename policy. No Windows runtime, directory persistence
barrier, shared-rekey support or physical failure qualification is implied. The
public Open prelude was included in the metadata audit rather than assuming the
internal backend alone covered every handle. Source/model reviews are not an
independent security/cryptographic/legal review. Uncertain-close tombstones may
retain capacity and poison availability permanently; there is no safe numeric
handle retry or automatic reclamation promise.

Later source review additionally found the plain Windows mapping-object close
path could retry a numeric HANDLE after an uncertain close. The replacement
state machine now retains a terminal unknown-close record and first cause,
including failed MapView setup cleanup, while preserving the still-live view
and DMS/main protections. Native Windows failure/runtime testing remains
NOT_RUN; source and memory proof is not OS qualification. The withdrawn
unpublished candidate containing the earlier mapping path was never main.

Darwin now shares descriptor retirement across plain and encrypted native paths;
unmanaged same-inode closes remain outside that protection. Shared-WAL failed
Open ownership has bounded retention and explicit retry; terminal uncertainty
can exhaust admission until process exit. No cleanup error authorizes repeated
raw OS close. Native WALShared successful Close and driver disposal now have a
separate bounded ownership correction. Encrypted acquisition/selection/copy/session retirement now has bounded scope
ownership and explicit discovery/retry, with native exceptional branches still
unexecuted. Rollback Reader/Tx, exclusive WAL and public metadata adoption now
have bounded ownership corrections. Recovery ownership is source-reviewed; public automatic recovery integration and
physical native-failure qualification remain open. The two earlier source gaps
are reconciled below; complete public-lifecycle safety is not established.


## 当前修正与历史失败的边界（2026-10-11）

本次以 `1352ae524575e4d6fe8b5434fd930c3b07be4ac8` 核对实现与已保存证据，
只修正文档，没有重跑测试，也没有把局部工程复审提升为外部安全审计。

- **shared 初始不存在来源已修正。**
  [`shared_session.go`](../internal/pager/shared_session.go) 的创建和 sidecar
  判断只接受既有 VFS 的初始、尚无 handle 的 absence；
  [`plain_session_cleanup.go`](../internal/pager/plain_session_cleanup.go) 的
  私有 transition probe 同样传递该来源。后续检查/退休错误即使含 ENOENT
  也不能授权创建或忽略。详见[当前合同](shared-absence-provenance.md)、
  [`TestSharedAbsenceSourceInitialProvenance` 与 memory/source 控制](../internal/pager/shared_absence_test.go)，
  以及[独立审阅与精确历史证据](evidence/shared-absence-lead-review/REVIEW.md)。
- **rollback Reader 的 post-SHARED 元数据采集已修正。**
  [`OpenAdmitted`](../internal/pager/pager.go) 在成功取得 SHARED 后立即对同一
  已持有描述符重新 stat，再建立 Reader/header 快照；错误经原退休 owner
  返回，没有把 `ErrChanged` 改成 BUSY 或静默重试。详见
  [Reader 合同](rollback-reader-acquisition.md)、
  [`TestReaderAcquisitionSourceSnapshotAfterShared` / `TestReaderAcquisitionOrdinaryCooperatingWriter`](../internal/pager/reader_acquisition_test.go)
  和[独立审阅](evidence/rollback-reader-acquisition-lead-review/REVIEW.md)。

历史尾段曾把上述两项列为 open，描述的是修正前状态。原 rollback 20 秒
样本在 14.015854093 秒失败、两次 baseline 控制失败、四次修正后 20 秒
样本通过，均保留在[原始修正交接](evidence/rollback-reader-acquisition-evidence/handoff-v1.md)。
修正与 shared 来源处理还进入了[既有组合门](evidence/accepted-tools-combined-checks.txt)
（其记录的精确 tree 为 `ddb957e2a1d804b236a295f7fe6ce383954b852c`，
[exit 0](evidence/accepted-tools-combined-checks.exit)）；这不是本次基点新执行、
长期 soak、原生异常路径或全部 public lifecycle 的验收。

迁移的权限边界也须按当前入口区分：目录检查、转换与 PostgreSQL 审阅包
不会执行数据库 SQL；本地 `import --apply` 则确实会创建并写入原生数据库，
`--resume` 对匹配作业继续写入。精确参数与源/目标约束见
[迁移入口](migration-pgembedding.md) 和[导入合同](import-pgembedding.md)。
正常提交后关闭/续传不证明未知提交结果、崩溃或热恢复安全。

## 仍未关闭的实现与验收门

- 公开 Open 恢复、明文 backup、native WAL-tail 发布和 planner 修正仍有未
  合入/未实现部分；Windows shared rekey、Darwin 名称处理等候选也不能当作
  当前 main 能力。有关源码与状态以[验收矩阵](acceptance-status.md)及
  [未发布候选记录](interruption-20261010.md)为准。
- 未知触发中断、ANN 报告 V3 的 FAIL/HOLD 和被中断 V4/新 fuzz 链路继续
  保留，不因普通测试历史或本文修正解除暂停，也不推断为 ANN 算法已损坏。
- 完整物理故障/断电/短写/Sync/ENOSPC/torn write/checkpoint/rekey 中断
  harness 仍有源码缺口；已有源码与尚无 harness 的项分别列在
  [中文手工验收手册](manual-verification-zh.md)。这里未执行或转移任何受限流程。
- 原生 OS 运行、真实持久性环境和外部独立数据安全/密码学/法律审查仍不完整。
  跨编译、源审查、内存负例和普通正确密钥测试均不构成生产安全或稳定版认证。
