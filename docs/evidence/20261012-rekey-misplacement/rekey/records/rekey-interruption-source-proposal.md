# Interrupted rekey: finite source proposal

Status: DESIGN ONLY / SOURCE-READ EVIDENCE. No implementation, Go tool, parser, test, binary, native database operation, process controller, Git operation, or network operation was run for this report. Actual new physical rekey remains **NOT_RUN**. This document does not authorize implementation or execution and does not reopen any ANN V4, fuzz campaign, denied workflow, or unknown interrupted chain.

Read base: `/workspace/scratch/5e06a74e5f65/restart-20261012T2033/gosqlite`, supplied by the parent as current main 6d plus the A8 candidate. Commit identity is parent-supplied; no Git command was used to verify it. File hashes below identify the source actually inspected. A8 and other concurrent changes must be frozen again before implementation review.

## Recommendation

Implement one separate, manually gated Linux layout-1/raw-key/4096-byte-page rekey facility with **six cases in one profile**, not a Cartesian product over the six storage protocols:

1. normal control;
2. before the root rename attempt;
3. after successful root rename, before its directory Sync;
4. after successful post-rename directory Sync, before final root readback/adoption;
5. public `DB.Rekey` returned nil, before external acknowledgement;
6. public `DB.Rekey` returned nil and the controller's acknowledgement file/namespace were synced, then process exit.

Use three private, observation-only publication call sites with a default no-op implementation; put the return/ack cuts in the manual caller. No VFS fault framework, syscall replacement, production rekey refactor, recovery fallback, or public API change is necessary. The future interrupted cases deliberately exit at a protocol barrier without running deferred cleanup. They qualify only controlled process-exit observations, not power loss, failed Sync, SIGKILL, or hardware persistence.

These are five distinct interruption/ack states plus one lifecycle control. Dropping the returned-no-ack row would lose the distinction between absent API completion and absent external ACK; dropping the after-Sync row would conflate namespace persistence calls with successful final checks/adoption. Additional candidate-write, pre-candidate-Sync or in-syscall cuts are deliberately excluded. The old-archive comparison below is a bounded normal-control companion, not a seventh interruption case or an additional storage profile.

Copy the *design* of A5's fixed pipe transport, verified self-image, parent-owned Start/Wait, and causal acknowledgement. Do not import its concrete controller/channel/model or weaken its plain-rollback contracts. A separate shell controller is not smaller here: it would still need reviewed binary identity, framing, real process ownership/Wait, exclusive write/fsync/close helpers, bounded diagnostics, and a new shell-source admission contract. Existing ordinary scripts explicitly exclude these workflows. Neither route may run now or be sent to another executor/CI.

## 1. The original requirement and actual implementation

The original `goal.md` explicitly requires interrupted rekey in §8.3, line 316; full crash/rekey tests in M8, line 361; and failure/restart plus crash/rekey/backup coverage in §11.1, lines 374 and 378. Thus this is a genuine missing caller/controller slice. Ordinary correct-key rekey tests do not close it. The proposal closes only a representative layout-1 **source facility**, not M8 as a whole.

### Exact source chain

| Source | Relevant behavior |
|---|---|
| `sqlite/encryption.go:131–184`, `DB.Rekey` | Validates target; refuses different mode, read-only, active Tx/Rows; reads existing DBID; calls `encstore.Prepare(... OperationRekeyTarget, id)` outside engine mutexes; rechecks state; calls `RekeyPrepared`; changes connection encryption options only after nil. |
| `internal/encstore/format.go:95–130`, `Prepare` | Canonicalizes/preflights; `NewRekeyEpoch` retains DBID; derives target master using a purpose-bearing request; prepared master has explicit ownership. |
| `internal/pager/encrypted_session.go:336–341`, `RekeyPrepared` | Holds session mutex and enters `operationLocked(ctx, true, true, validate, ...)`; invokes `Store.Publish` with a nonnil prepared target. |
| `internal/pager/encrypted_operation.go:46–87`, `operationLocked` | Rejects active session Tx; begins maintenance; verifies complete selected encrypted data and page-one header; calls `validateCopy`; defers maintenance retirement and poison propagation. |
| `sqlite/encryption_copy.go:15–34`, `validateCopy` | Catalog grammar, B-tree/overflow ownership and freelist validation. Explicitly **not** a full index-to-table content audit. |
| `internal/encstore/publish.go:51–178`, `publishBase` | Copies every logical page into a new immutable base under the target; syncs its namespace; prepares a new root; calls `publishRoot`; adopts base/master/options only after nil; clears `target.Master` on adoption; closes obsolete objects/master; cleanup failure can still make the result uncertain. |
| `internal/encstore/publish.go:289–414`, `publishRoot` | Verifies candidate bytes; writes/syncs `.pending`; reopens/verifies/stability-checks it; retains old root bytes; syncs candidate/retained namespace; checks current authority, selected objects, candidate and context; attempts rename; syncs namespace; rereads and checks final root and owners; adopts root; closes old root. |
| `internal/encstore/namespace.go:92–110,134–149` | `sync` checks the pinned directory then calls its managed `Sync`; `write` syncs the newly written file; `rename` uses the pinned-directory helper. There is no function literally named `namespaceSync` in this path. |
| `internal/encstore/platform_linux.go:118–128` | Existing-root replacement calls `unix.Renameat`; absent-root creation uses `Renameat2(... RENAME_NOREPLACE)`. This rekey profile always replaces an existing root. |
| `internal/encstore/store.go:134–225` | Open captures the selected current root, derives a master using `OperationOpen`, then authenticates/opens that selection. Retained roots and abandoned candidates are not fallback authorities. |

### Three different meanings of publication

1. **Rename attempted:** `publishRoot` sets `attempted = true` immediately before `s.ns.rename` at lines 370–371. Any returned failure from this point is conservatively uncertain, including a rename call that itself failed. Do not label every rename error as OLD.
2. **Namespace Sync returned nil:** `s.ns.sync(s.path)` at lines 374–376 has returned. Final open/read/stability/owner checks and in-memory adoption are still ahead. It is not yet public success.
3. **Public Rekey returned nil:** the whole `publishBase` adoption/old-owner cleanup, deferred maintenance, and public option update completed. This is distinct again from receipt of an external acknowledgement.

There are earlier namespace Sync calls at `publishBase:132–134` and `publishRoot:345–347`. The proposed first cut is after those have returned and final pre-rename authority checks have passed. It is not an interruption while constructing or syncing the candidate. The third cut is specifically after the **post-rename** Sync. Names must encode that distinction; a generic “after sync” occurrence counter is insufficient.

### Errors and ownership are not transaction rollback

`publishRoot:369` preserves the original cause inside `errors.Join(encstore.ErrUncertain, cause)` and poisons the Store. `pager.encryptedError:29–46` adds `ErrPoisoned` and `ErrCommitUncertain`; `sqlite.wrap:93–121` gives these `CodeIOErr` (10). `publishBase:176–178` can also return an uncertain ownership failure after durable root publication. A nil rename, nil directory Sync, or changed on-disk root does not imply nil public Rekey.

Before rename, errors are not automatically `ErrUncertain`; exact cause and cleanup composition still matter. After an uncertain return, never keep using that handle, retry Rekey, reclassify it as rollback, or choose a retained root. Record the original error object and public `errors.Is`/`errors.As` facts before any close. Cleanup may add its own token/poison and must remain a separate observation.

**Exit is not an error return.** Cases 2–4 never return from public Rekey. Their receipt says `NotReturned`, not `ErrCommitUncertain`. The application-level outcome is unacknowledged; the model must not invent an error it did not observe. Real rename/Sync/close error execution is outside this observer-only slice and remains NOT_RUN.

## 2. Fixed fixture and immutable credential purpose

Profile: Linux native process exit, `EncryptionSnapshot`/layout 1, `RawKey256`, 4096-byte logical pages, one table and one ordinary B-tree secondary index, no WAL, no shared readers, no extension/ANN indexes, no password/KDF matrix. Storage/device sector size is not inferred from the logical page size. Record actual filesystem identity separately, without claiming a hardware sector or durability guarantee.

Prepare one tiny public disposable fixture per independently selected case in a new private retained directory:

- `CREATE TABLE t(id INTEGER PRIMARY KEY,k TEXT,v TEXT)`
- `CREATE INDEX by_k ON t(k)`
- `INSERT INTO t VALUES(1,'red','one'),(2,'blue','two'),(3,'red','three')`

Use ordinary source-supported SQL once, then verify and close normally before copying or spawning. Freeze the observed, authenticated baseline header and exact full ledger. The expected geometry is three logical pages; require that observation rather than silently accepting other geometry. With initial creation plus these three autocommit statements the expected root sequence is 4; bind that explicitly after confirming the source construction, not by guessing from the file count. Rekey must preserve all logical pages and page-one logical counters while the encrypted root sequence increases by exactly one.

Two credential values, OLD and NEW, are predetermined public disposable fixture material, with distinct domain-separated recipes in the manually gated source. Their labels are immutable. This follows the existing A8 public synthetic-key precedent but must clearly say these are public test data, never usable application secrets. Accept no user key, credential file, argv secret, DSN, environment secret, network key lookup, or key chosen from an observed failed Open.

Each provider instance has an immutable **purpose**, allowed operation, and exact independently admitted path:

- OLD seed: `OperationCreate` at the original fixture root;
- OLD writer: `OperationOpen` at the trial root;
- NEW rekey target: `OperationRekeyTarget` at the same trial root;
- OLD probe: only `OperationOpen` on its fixed OLD-probe copy;
- NEW probe: only `OperationOpen` on its fixed NEW-probe copy;
- OLD archive writer/source and NEW restore target: separate instances fixed to `OperationBackupTarget`, archive `OperationOpen`, and `OperationRestoreTarget` for the control-only archive check.

Do not infer key purpose from DBID/EpochID in an unauthenticated disk header. `encryption/key.go:24–28` explicitly warns that these fields are not authenticated until `OpenFile`. Provider checks of purpose/path are admission checks, not authentication evidence. After successful authentication, bind DBID, epoch, salt, root FileID, head hash, sequence, layout and geometry to the frozen source/candidate.

Every Acquire's material is cloned, borrowed only until one Release, cleared by Release, and recorded with purpose/operation/count plus a duplicate-release flag. Do not log credential bytes or hashes intended as secrets. In the interrupted cuts, target acquisition has already completed; provider borrow release counts can be checked while active master schedules still belong to the exiting process. Process exit does not prove secure memory erasure.

The original API already creates fresh target epoch/root salts and file identities (`key.go:98–140`). Require same DBID, different nonzero EpochID and RootSalt, fresh root and base FileIDs, new root sequence `old+1`, and `ParentCommitID = SHA256(old selected root)`. For raw-key mode KDF fields and KDFSalt stay zero. These public identity checks are useful finite evidence, not a complete `(key, nonce)` uniqueness proof across arbitrary crash histories.

Add one bounded **public derivation-input/nonce ledger**, with no secret-key export or new encryption hook. `encryption/key.go:260–303` derives role-separated file ciphers from the master, DBID, EpochID, file role, record role and FileID; `record.go:97–138` burns sequence before Seal; `record.go:333–337` forms the nonce from four-byte NoncePrefix plus big-endian uint64 sequence. For each unique frozen original/candidate root or base ciphertext object, record credential label, epoch/RootSalt, DBID, FileID, file role, record role, physical record offset/extent, sequence, NoncePrefix and resulting 12-byte nonce. Root has one metadata record; this three-page base has metadata, three data records, and footer. Bound parsing by exact declared record counts/extents; no page or frame number is invented from the record header, which stores only role/sequence/plaintext length.

Treat retained roots and independent byte-identical copies as aliases of the same historical encryption event, identified by whole-object hash, not as fresh encryption. Reject a distinct encryption event reusing the same public key-domain tuple and nonce, duplicate sequence or inconsistent bytes for one identity. NEW candidate root/base must use fresh domains even in the pre-rename case. Old archive/restore objects, when present in control, get their actual separate file/DBID domains. This observes the fixed fixture's immutable encrypted objects only; it does not inspect secret derived keys, prove randomness, or close the original goal's complete restart/rollback/failed-write nonce campaign. Neither rejected speculative counterexamples nor parser success is physical nonce evidence.

## 3. Cases, exact cuts, and permissible conclusions

| Case | Exact owned barrier | Public Rekey | Parent acknowledgement | Expected selected root on an unchanged same-kernel process-exit readback |
|---|---|---|---|---|
| `control` | No interruption; all three observers pass; public nil; explicit DB.Close; natural writer exit 0 | Nil | Not required for control | NEW |
| `before-root-rename` | After `ctx.Err` check, immediately before `attempted = true` at `publishRoot:370` | NotReturned | Absent | OLD; candidate `.base`/`.pending` and retained root may already exist |
| `after-root-rename-before-dirsync` | Immediately after `s.ns.rename` returned nil, before `s.ns.sync` at line 374 | NotReturned | Absent | NEW is visible to a fresh process; no final namespace Sync was made by the writer |
| `after-root-dirsync-before-adoption` | Immediately after `s.ns.sync` returned nil, before `s.ns.checked` at line 377 | NotReturned | Absent | NEW; final root checks/adoption and obsolete-owner closure not completed |
| `rekey-returned-no-ack` | Manual caller immediately captures the actual `db.Rekey` result; nil event is received by parent; exit grant before any ACK file is created | Nil | Absent | NEW |
| `acknowledged-exit` | Same public nil capture; controller completes exact ACK write/Sync/Close and separate control-directory Sync/Close; matching ACK and exit grant received | Nil | Present, exact original nil-return frame | NEW, bound to that same acknowledgement |

For cases 2–4 the observer reports the scalar/ciphertext identity snapshot, parent validates its exact event prefix, and only then grants the dedicated exit. The child does not return an injected error, panic, call DB.Close/Rollback, or run defers. Use a fixed expected target exit code (for example A5's existing convention 86), with a distinct protocol-abort outcome. Exit 86 alone is never sufficient; signal, OOM, panic, normal exit 0, missing frame, timeout or target miss is failure/inconclusive. The normal control and probes require actual Wait-observed natural exit 0.

On a genuine power loss near rename/dirsync the on-disk result cannot be inferred from these observations. In particular, absence of ACK does not imply OLD, ACK's own Sync cannot supplement the writer's missing DB directory Sync, and retained source/candidate bytes do not establish which namespace update survived a device failure. A future hardware experiment needs its own declared environment and contract.

## 4. Minimal observer design and normal-build boundary

Proposed private signature, as a design rather than an implementation:

`rekeyPublicationObserve(ctx, point, path, oldRootBytes, candidateRootBytes)`

- Exactly three call sites in `Store.publishRoot`, at the positions above. It returns nothing; no branch, result value, error, write length, or native operation can be replaced.
- The default implementation is an empty function. Normal platform methods, error ordering, file/namespace Syncs, candidate verification, ownership adoption and closes remain in their existing order.
- The manual implementation is selected only by `linux && manual_rekey_interruption`; the default stub by `!linux || !manual_rekey_interruption`. A tiny common declaration file owns the three closed point constants/signature vocabulary. No environment access, descriptor, transport, registry or testkit import exists in the stub.
- An explicitly constructed, single-use manual context token scopes observation to the one public Rekey call. The token contains a closed reviewed plan and concrete fixed channel, never a function callback, arbitrary descriptor, Store pointer, replacement VFS or caller-supplied action. Ordinary seed/backup/Open work has no token.
- In manual code the observer validates the token, exact path, fixed layout/header extents, old authenticated-root hash, candidate identity, point order and single target. It copies/hashes bounded **ciphertext/public metadata** only. It cannot mutate or retain aliases to engine byte slices, reach `s.master`, return an error into publication, or call back into DB/session methods.
- The observer is called after each relevant namespace helper returned, outside its managed-file/native locks. DB and session mutexes remain held; no reentrance is permitted. Release any private observer mutex before pipe I/O/barrier waiting. The context token is not cancelled to simulate a crash.
- A manual-only `ManualRekeyOwners` helper may snapshot existing encrypted cleanup-registry scalar bookkeeping under its own lock: groups, leases and pending tokens, with no native operation, resource status callback, retry, prune/reap, or descriptor escape. Require empty registry before a role starts and after normal close. Counts are bookkeeping evidence; they do not prove that an interrupted child's live descriptors were gracefully retired.

The default source necessarily gains three no-op calls. Claim semantic preservation, not byte-for-byte identical machine code without evidence. A later authorized compile/symbol review must show the normal build contains neither the manual bridge nor process/exit/transport capabilities attributable to it. Existing publication statements must be byte/order-preserved apart from the inserted observers; a source mutant must fail if a real operation is moved across a cut. If strict binary identity rather than no-op source is demanded, this approach needs a different build-only instrumentation design and is an unresolved scope decision, not something this report proves.

## 5. Controller, retained originals, and fresh classification

### No live dependency on A5

`rollbackrestart/contract.go` fixes five plain cases and `rollbackfault.Profile`. Its bridge arms the plain three-page fault scope and captures public **Commit**. `rollbackfault.ValidateSQLSnapshot` explicitly requires `PlainRollback4096` (`sql_ledger.go:510–517`). The native controller and guard freeze exact paths, functions and calls. Therefore importing it for encrypted rekey or relabelling the profile would weaken existing evidence. Leave those model, bridge, native caller and old allowlists unchanged. Reuse reviewed patterns by an independently reviewed, small separately named implementation.

### Fixed future native roles

- One controller.
- One writer calling public `DB.Rekey` exactly once.
- One fresh OLD-probe process with exactly one fixed key-purpose Open.
- One fresh NEW-probe process with exactly one fixed key-purpose Open.
- For `control` only, one archive-restore-check process, described below.

Thus no case loops, retry policy or profile campaign are needed. A selected case is a separately admitted run with a new retained namespace. Each role starts once and is actually waited once by its owning parent process object. Neither probe nor writer spawns children. No PID from a frame is accepted as a process capability.

Copy A5's reviewed structural requirements, not its mutable source or authorization:

- exact source manifest/compiler/compile-only receipt/binary hash and actual `/proc/self/exe` identity;
- exact role-specific test root and fixed flags; `GOFLAGS` empty; clean allowlisted child environment;
- controller-created fd3 control reader/fd4 event writer only; validate that they are distinct pipes;
- canonical bounded frame format, e.g. 4096-byte body plus four-byte length, at most 64 frames per scenario, with trial/source/binary/profile/case/role/generation/challenge/sequence binding;
- no stdout-derived PASS; bounded raw diagnostic captures are separate;
- ACK only from the received exact public-nil frame, O_EXCL full write, file Sync, one Close, separate control-directory Sync and one Close, hash/identity readback, then ACK and exit grants;
- no automatic Kill/Signal, CommandContext kill-on-timeout, shell, arbitrary executable, PATH lookup, or deadline interpreted as Wait success.

If a child stays alive or actual Wait is unavailable, save `CHILD_LIVE`/`WAIT_UNKNOWN`, retain the owned process and pipe objects, and stop dependent probing. A genuinely stuck child may require further user disposition; finite cases do not promise a bounded successful shutdown. Do not silently replace a still-live child or redirect the workflow.

### Retained layout

Use fixed separate families under a new 0700 run directory:

- `inputs/original/`: closed authenticated baseline family, complete immutable manifest;
- `inputs/old-archive.gse`: portable pre-rekey archive, only if preparing the control companion;
- `trial/`: independent complete baseline-family copy, writer's sole mutable family;
- `evidence/post-exit/`: independent frozen complete copy after actual writer Wait;
- `probe-old/`, `probe-new/`: separate distinct-inode copies of the same post-exit family;
- `restore-new/`: absent destination family used only by control's archive check;
- `control/`: binding, protocol, parent ACK, raw diagnostics, process receipts, source/fixture/SQL/artifact and failure records.

Preserve the same root basename inside each family, because object names are derived from it. Copy the full bounded family, not just the main root. Inventory includes root, `.gse-lock`, bases, retained roots, pending candidates and any other observed entry; reject unexpected types, aliases or plaintext sidecars instead of silently filtering them. Proposed explicit caps: 64 entries and 1 MiB total family data, 64 KiB per file, with exact normal fixture extents reviewed before admission. No hardlinks, symlinks, overwrite, rename, remove, cleanup or reuse of originals.

Every copy has unique dev/inode identity, complete byte/length/hash readback and checked one-time owners; copies are O_EXCL and synced only at their own destinations. Freeze trial only after actual writer Wait. Never Open, repair, RecoverRollback, Restore over, rename or add a Sync to the frozen trial/original family. Hash/size/type/identity manifests before and after probes detect changes. A copied destination's Sync is not evidence for durability of its source.

### Independent OLD/NEW key outcome

Each probe is a fresh process with a closed purpose selected by the frozen plan **before observing the root**. It opens its own copy with `ReadOnly:true, NoCreate:true, EncryptionSnapshot` exactly once. There is no “try OLD then fall back to NEW” on the same connection/family, no alternate root, and no retry. Independent evidence from both probes classifies the selected post-exit root:

- OLD: OLD opens/authenticates and meets the complete unchanged ledger; NEW fails exactly as an authentication Open.
- NEW: NEW opens/authenticates and meets that same complete ledger; OLD fails exactly as an authentication Open.
- Both succeed, both fail, mixed/incomplete SQL or metadata, wrong public code, unrelated I/O/cleanup failure, or lost ownership: reject/inconclusive, never success.

An exact expected wrong-key error requires nil DB, original `*sqlite.Error` with operation `open`, `CodeNotADB` (26), and `errors.Is(err, encryption.ErrAuthentication)`. Reject joins with cleanup pending, poison, busy, changed, recovery-required, commit-uncertain, format/corrupt/limit, provider failure, cancellation, path/syscall/EOF I/O diagnostics. Preserve original error objects in-process and bounded classification tokens in the receipt; do not serialize `Error()` strings containing arbitrary path/provider details. No Query/Verify is attempted on the failed Open. A wrong-key nonnil error of any kind is not sufficient.

On the successful probe, call `VerifyEncrypted`, complete SQL/index ledger and bounded `Check`, then explicit DB.Close once. Close and EOF retirement are checked; later cleanup must not repeat failed native retirement. Any pending token or unknown close is failure/owner-pending, not a passed negative. Public `PendingEncryptedCleanups` and a read-only manual registry snapshot support bookkeeping; neither is a substitute for actual process Wait.

## 6. Full SQL and index ledger

Rekey changes encryption authority, not logical table state. Do not use the plain rollback model's different OLD/NEW rows to distinguish generations. Both generations require exactly the baseline rows, with strict int64/string/string types:

`(1, red, one), (2, blue, two), (3, red, three)`.

Use the seven-query pattern already present in `rollbackfault/sql_ledger.go:272–311` and `sqlite/rollback_fault_sql_manual_linux_test.go:709–803`, as copied purpose-specific code rather than invoking its plain-profile validator:

1. all rows `NOT INDEXED ORDER BY id`;
2. blue equality, real secondary index;
3. blue equality `NOT INDEXED`;
4. red equality, real secondary index;
5. red equality `NOT INDEXED`;
6. green equality, real secondary index, expected empty;
7. green equality `NOT INDEXED`, expected empty.

Each must equal independently frozen literal expectations, not just agree with another query. Verify `SEARCH`, `btree`, exact `by_k`/`k`, index count/field count and recheck for indexed queries; `SCAN`/`NOT INDEXED requested` for scans. Capture actual `Rows.Stats`: nonempty index results require real page probes, matching candidates/rowid seeks/base fetches/returned rows; even empty green requires a real index-page probe. Scans fetch the bounded three rows with zero index work. Fully exhaust at most three rows, record EOF and `Rows.Err`, then one explicit public Rows.Close; EOF's internal retirement is not counted as a second public close.

`Check(MaxPages:3, MaxPageReads:512)` must report exact supported scope, three pages, no free/reserved pages, one table, one ordinary index, zero extension indexes, three table rows and three index entries, positive bounded page reads. Capture/close each owner once with identities unique across processes/generations. `VerifyEncrypted` alone authenticates physical bytes; it does not supply this semantic ledger.

All successful probes additionally validate layout 1, mode snapshot, page size/count, DBID, head/root hash and sequence against the appropriate frozen authenticated old/candidate metadata. A correct SQL ledger with the wrong root generation is not an allowed substitute.

## 7. Old archive restore is a separate control, not downgrade/recovery

Keep a portable archive made **before** rekey using `BackupEncrypted` and immutable OLD archive-target purpose. The archive gets its own fresh identity (`sqlite/encryption_copy.go:48–74`); do not call a copied main root an archive.

Only the normal-control companion restores that retained archive into the independently absent `restore-new/` family using source OLD archive purpose and target NEW restore purpose. `RestoreEncrypted:103–131` authenticates archive, validates copy, then calls `Prepare(... OperationRestoreTarget, zero DBID)`, so the restored database must get a new DBID/epoch/file identities. Its DBID must differ from both the source DB and archive identities, while its table/index ledger equals the pre-rekey baseline. Reopen the created target with its NEW credential read-only/no-create, verify, check full ledger, close once, and prove the archive/source manifests remained unchanged.

This is supported old-archive recovery to a **new target**. It does not downgrade the active format/key generation, overwrite an uncertain root, restore the old source DBID, revoke OLD's access to history, or establish anti-rollback freshness. Retained `.retained` files must never be promoted into the current root by the harness. Physical archive execution is also NOT_RUN now.

## 8. Concrete minimal file/hunk plan

All names below are proposals, not existing implemented capabilities.

| File | Smallest proposed change |
|---|---|
| `internal/encstore/publish.go`, `Store.publishRoot` | Exactly three void observer calls: before `attempted=true`, after rename-nil block, after post-rename-sync-nil block. Preserve all existing real operations/branches/order. |
| NEW `internal/encstore/rekey_observation.go` | Three private point constants and minimal shared signature vocabulary; no runtime effects. |
| NEW `internal/encstore/rekey_observation_default.go` | Exact `!linux || !manual_rekey_interruption` build constraint and empty observer. |
| NEW `internal/encstore/rekey_interruption_manual_linux.go` | Exact `linux && manual_rekey_interruption`; concrete context scope, bounded channel, read-only observation/registry snapshots, fixed granted exit, no callback or generic I/O fault action. |
| NEW `internal/testkit/rekeyinterruption/contract.go` | Descriptor-free closed six-case/profile/purpose/role/event/state/evidence model, finite framing and immutable expectation helpers. No `os`, syscall, exec, time, filesystem, sqlite/encstore import or function-valued effects. |
| NEW `internal/testkit/rekeyinterruption/contract_test.go` | Finite value-only positive, missing-evidence and false-success counterexamples. |
| NEW `sqlite/rekey_interruption_manual_linux_test.go` | Exact manual tag; fixed controller/writer/OLD-probe/NEW-probe/control-archive roots; immutable providers, bounded fixture/copy/SQL/owner helpers, single public Rekey call with adjacent unchanged result capture. No production sqlite/pager API change. |
| NEW `internal/testscope/rekey_interruption.go` | Exact paths/tags/import consumers, closed functions/calls/effects, observer adjacency/order, one authority for returned nil and ACK, no reentry/escape, source-manifest/pin and whole-main checks. |
| NEW `internal/testscope/rekey_interruption_test.go` | Finite inert AST/source mutants, including repinned malicious source, not just stale-hash failures. |
| `internal/testscope/main.go` | One per-file new scanner call plus one whole-contract call. The general restricted-reference policy remains; only exact independently reviewed references may be admitted. Whole-main and body pins affected by this integration must be independently reviewed before updating, including A8's current whole-main pin. |
| NEW `docs/testing/rekey-interruption-manual-zh.md` | Exact profile/cases, input/owner/error/ACK contracts, retained layout and code-vs-runtime status. No executable run recipe until separately authorized. |

No changes to `sqlite/encryption.go`, pager rekey/maintenance, native platform methods, key derivation, ordinary scripts/workflows, old rollbackrestart model/channel/caller, A8 native caller, or plain fault profile are required for the suggested instrumentation. The guard integration may require exact new restricted-reference predicates for `os.Exit`/process behavior in the new manual file, but must not add a build-tag blanket exemption or relax old predicates.

The new guard must protect its integration on both planes: per-file AST scan *and* end-of-walk contract checks. The inspected A8 correction expressly checks that both calls exist once; do not repeat a missing-wiring integration error. Do not update old pins merely to make a scanner green.

## 9. Finite descriptor-free validation and review proposal

All items in this section are future proposals, not commands, not authorization, and NOT_RUN for this new facility. Freeze exact test-root inventory/count after implementation before requesting any finite validation.

### Value model roots: proposed finite eight

1. `TestRekeyInterruptionSixCases`: all six exact traces; one profile only; not-returned/nil/ACK distinctions.
2. `TestRekeyInterruptionCutOrder`: missing/duplicate/reordered cuts; earlier candidate Sync cannot stand in for final Sync; rename attempt cannot stand in for successful rename.
3. `TestRekeyInterruptionCredentialPurpose`: immutable OLD/NEW/role/path/operation; key-purpose substitution, authenticated-vs-untrusted identity.
4. `TestRekeyInterruptionSelectedGeneration`: same SQL with different generation, wrong DBID/parent/sequence/epoch/FileID, both keys accepted/rejected, no retained-root fallback.
5. `TestRekeyInterruptionSQLIndexLedger`: wrong typed value, missing/extra row, stale/missing index, empty-index shortcut, absent EOF/Err/close, weak Check counters.
6. `TestRekeyInterruptionAckCausality`: ACK before public nil, rewritten return frame, failed/partial write, missing file/dir Sync/Close, ack/source inode alias, child-manufactured parent evidence.
7. `TestRekeyInterruptionOwnersAndArtifacts`: one Start/Wait, genuine expected exit versus timeout/signal/live child, stale challenge, unknown close, pending cleanup, original/copied family alias or mutation, restore overwrites source.
8. `TestRekeyInterruptionNeverPhysicalPass`: even a fabricated internally consistent complete trace returns evidence-consistent only; `PhysicalValidated()` and `Pass()` stay false; a requested physical claim is rejected.

Frame bounds/canonical encoding/unknown or duplicate fields/trailing bytes/truncated prefix/stale generation belong to these finite roots rather than a new fuzz campaign. No model test may call native code or even open a database.

### Source/AST roots: proposed finite seven

1. exact source inventory/import closure/tags and first-action independent gate;
2. exact three publication observers and preserved real operation order;
3. observer no mutation, callback, native lock reentry, master/key access or result substitution;
4. unique unchanged public Rekey result capture and parent-only ACK;
5. fixed self-image/process/pipe/Wait ownership, no arbitrary PID/Kill/shell/child spawning;
6. fixed readonly/no-create independent probes, literal SQL ledger, no alternate key/recovery/retained-root promotion/retry;
7. complete two-hook guard integration, old contracts still active, default stub and no manual capabilities in ordinary source closure.

For substantive mutants, update the proposed source pins/shape snapshots in the inert test input and still demand semantic rejection. A pin-only rejection does not establish the source rule. Review all affected `inspectSource` and whole `main`, not just one added condition; A5's old exact plain restrictions remain independently enforced.

### Selection and compile-only, if separately allowed later

- Pure source selection: ordinary Linux, Linux with exactly the new manual tag, existing rollback-only tags without the new tag, Linux all-declared manual tags for composition only if specifically admitted, and a non-Linux selection. Default/non-Linux must have the stub and no new entry/exit/channel. Existing A5 source selection must stay identical.
- Compile only the exact tagged sqlite test package and model/scanner dependencies into a separate retained output directory, bind source manifest/compiler/options/binary receipt, and do not execute or list tests by launching that binary.
- Compiler/type checking is needed for context-token ownership, package placement/import cycles, build constraints and alias handling. No assertion here establishes that the proposed code already compiles.
- A later default build symbol review may establish absence of the new manual capabilities. Source selection or compile success cannot make physical evidence true and cannot authorize a native case.

## 10. Unresolved dependencies and stopping boundary

Before source implementation: root approves the one-profile six-case scope, three no-op observer insertion points, separate controller, wrong-key probe pair and control-only old-archive restore. This report makes no action request directly to the user.

Before finite model/AST/selection/compile validation: freeze implementation and independently review complete observer/controller/provider/error/owner paths plus all changed guards. Finalize exact fixture geometry/sequence, source-to-binary receipt, copy bounds, source error taxonomy and manual registry snapshot lock ordering; confirm no old guard is weakened. A8 is a currently inspected candidate, not presumed validated solely because its code exists.

Before any future physical execution: separate actual authorization for each case, selected Linux/FS environment, binary/source binding, retained private directories, process exit and wrong-key Opens, ACK/copy effects, old-archive companion, and unresolved-live-child disposition. No automatic execution, new environment, CI routing or rebuilding a denied/unknown chain is supplied by this design.

Still open after this source slice: layout-2/3 or plain-WAL interruption; candidate creation/write/file-Sync interruptions; actual rename/dir-Sync/close errors; ENOSPC/torn/wrong-offset/device-loss; full nonce reuse/restart proof; extension-index interrupted recovery; concurrent/shared rekey leases; broader archive/upgrade publication; security/cryptographic audit and M3/M8/M10 acceptance. These are bounded separate gaps, not additional cases silently included here.

Stopping condition for this assignment: deliver this source-evidenced proposal only. No source was changed and no proposed verification or native operation was run.

## Inspected source identities

Hashes were obtained by read-only SHA-256 over local files; they are not a Git-tree or binary provenance claim.

| File | SHA-256 |
|---|---|
| `goal.md` | `9ae7b1d776856669a10eb753408c7754d015d266f8783240165450e96e4bac81` |
| `sqlite/encryption.go` | `02679a9bc79ce5c61201516e3f8216e92fdb70a7ac9fd9db529ee5218a65caa4` |
| `sqlite/encryption_copy.go` | `962f54bae02b2c658ad84dfb0c64b1f6211b8deab2ff2e0b16450c19751af953` |
| `internal/pager/encrypted_operation.go` | `b172f3f25ac82bff0da61aa6c5f67a32aa8860b263bf3ec97efead946e3960a6` |
| `internal/pager/encrypted_session.go` | `ed1ce852473d63f5eafc445e1f90062f80e4e1ea17e7c1360c4ee0fcb334494c` |
| `internal/encstore/publish.go` | `adffe2f3c8c4db58a39e3719a72c5ee69f389eccefde876b4e36285dc663ae93` |
| `internal/encstore/namespace.go` | `96e76556ac6277d9090bd0e0eef580d52d85e51af66b6d2f2f3951fa554d3e72` |
| `internal/encstore/store.go` | `0921aa3ccc90849f34f9354bfb206ca9c94be91b6b9c1b7d4980b73f34024d30` |
| `internal/testscope/main.go` | `6dbab48ff374858bfafe578f0fca99f5176ced7b6bdc008bec04ce6a6d1d0eb2` |
| `internal/testscope/rollback_restart.go` | `a6edb3a708ecf81c688c766835b349c26777553613de587673847b451a0e8ab2` |
| `internal/testscope/encrypted_root_negative.go` | `6d78b1b56319ab4743ce0186070bcd125b5f222df2b14cc6fba5a4f29b8a2577` |
| `sqlite/encrypted_root_negative_manual_linux_test.go` | `3e1e3640ff321a7673d9f39c29d32cfe73ec2d4c53858205ee9a61b7738bbbc3` |
