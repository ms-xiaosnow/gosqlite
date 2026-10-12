# R5 interrupted rekey: independent native/model/observer source review

## Verdict and boundary

**SOURCE_REVIEW_CLEAR_WITH_LIMITS for the eight source files identified below.** Three definite native-controller/evidence defects were reported, corrected by the implementation worker, and independently reread. No remaining definite blocker was identified within the approved six-case, one-profile source boundary.

This is a source-reading verdict, not a compiler, parser, test, process, filesystem-durability, cryptographic-security, or physical acceptance verdict. All Go/gofmt/parser executables, source-selection tools, compiler invocations, model tests, guard tests, native fixtures, wrong-key Opens, child exits, and archive restoration remain **NOT_RUN** in this review. No source was changed in the base checkout. The reviewer only read source, listed paths, compared text, calculated file hashes, coordinated corrections in the separate overlay, and wrote this report.

Scope: `rekey-interruption-overlay` native caller/controller, publication observations/default stub, descriptor-free value model/tests, and manual documentation. The separately maintained `rekey-guard-overlay` and its complete scanner integration require their own independent review. This report does not approve its pins, AST rules, guard execution, or integration into the base checkout.

The profile remains Linux, snapshot/layout 1, public disposable RawKey256 credentials, 4096-byte logical pages, three logical pages, one ordinary table/index. The six cases remain control, before root rename, after rename before its directory Sync, after that directory Sync before adoption, returned nil without ACK, and returned nil with parent-durable ACK. The archive companion is optional only for control; it does not introduce another interruption case or profile.

## Definite findings and corrections

### F1: unresolved-child retention did not survive controller return — corrected

Initial native `receive`/`waitChild` timed out through `t.Fatal`, while `finish` returned if the owned child Wait channel was not immediately ready. Returning from the only selected controller test allows the test binary to exit, so a global slice did not actually preserve process/pipe ownership. This contradicted the explicit CHILD_LIVE/WAIT_UNKNOWN boundary.

Final `finish` (around lines 141–167) blocks on the already-issued, exactly-once Wait channel for a started child whose actual Wait has not been observed. It sends no further grant and performs no Kill, Signal, replacement Start, second Wait, or dependent probe. A later actual process-state result is recorded separately rather than rewriting the initial unknown receipt. The timeout remains a failed/inconclusive observation.

The correction also distinguishes a completed Wait call from an available `ProcessState`. A nil state now enters `manualRekeyHoldUnknown` in both ordinary and deferred failure paths. The hold is a passive timer-backed loop (`time.Sleep(time.Hour)`), not an empty `select {}` that could allow Go's all-goroutines-asleep fatal termination. Thus unknown process ownership cannot be converted into ordinary parent return by either a deadline or a nil-state result. External disposition may still be necessary indefinitely; this facility does not promise successful shutdown of an arbitrary stuck child.

### F2: terminal duplicate/trailing protocol data could be ignored — corrected

Initial `waitChild` validated the expected exit and closed the event reader without checking whether bytes remained after the last accepted event. A trailing duplicate frame or suffix could therefore escape the protocol checks.

Final `waitChild` (around lines 599–634) first observes the owning parent's actual Wait and validates exit code, signal status, error identity, and diagnostic bounds. It then reads one byte and requires exactly zero bytes plus `io.EOF` before pipe retirement. Any trailing byte rejects the terminal transcript. `TerminalEOF` is retained in the process receipt and required by the value model; a finite negative value case was added within the existing test inventory. Under the frozen roles, children do not spawn descendants and parent duplicate pipe endpoints have already been closed, so a genuine terminal EOF has a concrete ownership basis.

### F3: nonce credential labels depended on an arbitrary ancestor name — corrected

Initial `nonceLedger` labeled an object NEW whenever its entire absolute pathname contained `/restore-new/`. An otherwise permitted run parent with that component would mislabel OLD original/history objects.

Final native code (around line 867) identifies the restore family by exact directory equality with `filepath.Join(admission.Root, "restore-new")`; target-epoch classification remains separate. Credential-purpose selection for actual Open was already immutable and was unaffected. The correction fixes the retained public derivation-input evidence rather than adding another credential or scenario.

## Source/model alignment corrections

The initial model and native caller used different root basenames (`fixture.db` versus `fixture.gse`) and different role-generation conventions. Final model paths/manifests use `fixture.gse`; native generations are fixed distinct values 1–5 for controller, writer, OLD probe, NEW probe, and archive check, matching the model's distinct-role-generation requirement.

The model initially required the archive in every control trace while native admission exposed an optional `ArchiveControl`. Final `Evidence.ArchiveControl`, trace construction, role/provider/artifact/nonce counts and archive validation consistently support control with or without that companion and reject archive outside control. Finite positive/negative cases were added inside the existing eight model roots. The model's scalar signal field was renamed `SignalNumber` at the separate guard worker's request; this is ordinary inert data, not permission to change old restricted-reference behavior.

A remaining difference is deliberately disclosed rather than hidden: `Generation.LogicalPages` and `PageOneCounters` are synthetic value-model relationships. Native code does not observe plaintext page hashes or byte-for-byte page-one counter equality. The native SQL/index/Check ledger and authenticated encrypted-root identity are useful but cannot stand in for those missing observations. Documentation now states this limitation explicitly. This review does not assert the proposal's stronger logical-page byte-preservation evidence has been implemented or executed.

## Publication observation and default-build review

Text comparison against base `internal/encstore/publish.go` shows exactly three added call lines, with no other textual changes:

1. immediately before `attempted = true`, after final authority/selection/candidate/context checks;
2. immediately after successful root rename, before the post-rename namespace Sync;
3. immediately after that Sync returns nil, before final checked root readback/adoption.

The existing error branches, earlier candidate/retained namespace Sync, uncertainty classification, final readback, owner checks, root adoption and old-root Close stay in order. These are distinct boundaries; no earlier Sync is counted as the final one, and successful rename is not treated as public nil.

The common file contains only a private point type and three constants. The default file has the complementary `!linux || !manual_rekey_interruption` constraint and an exactly empty void observer. It imports only context and owns no descriptor, environment read, registry access, transport, callback or exit capability. This is source-level semantic preservation, not a claim of identical compiled machine code or proven symbol absence.

The manual observer is selected by exact `linux && manual_rekey_interruption`. It receives no Store/master pointer and returns no value that could alter publication. It decodes/hashes bounded public headers and ciphertext, retains scalar values rather than engine slice aliases, validates old/candidate identity and point order, checks completed provider borrows, and releases its private mutex before the pipe barrier. It does not reenter DB/session methods. Existing DB/session locks may remain held, as the documentation correctly says.

`ManualRekeyOwners` holds only the cleanup-registry mutex and reads its counters plus atomic pending pointers. It does not take a group lock in reverse order, retry/reap cleanup, invoke a resource callback, or perform native I/O. The existing cleanup implementation was read to check that lock-order argument.

## Tool, initialization and source/image closure

All five native test roots begin by invoking the independent admission gate. Before native role work, admission requires exact role flags, the manual authorization marker, unprivileged Linux, empty GOFLAGS, canonical binding hash/schema, private root parent, observed filesystem type, source/image checks, empty cleanup bookkeeping, and (for children) a fixed-pipe role-ready exchange granted by the parent.

Source/image checking hashes an explicit complete inventory of Go source plus go.mod/go.sum, rejects omitted/additional inventoried entries, checks the canonical compile-only receipt and fixed package/tag/flags, hashes the compiler and actual `/proc/self/exe`, and binds the configured spawn path to that image by hash and inode. The parent rechecks image identity before each Start. This is receipt/hash/identity admission, not an independent proof that arbitrary compiler output corresponds to source; compilation and its receipt creation are still unperformed and separately authorized steps.

New native globals are nil retention slices, not effectful package initializers. The model imports only bytes, SHA-256, binary encoding and JSON; model tests add testing. No descriptor, os/exec, syscall, timer, database, encstore or function-valued native capability exists in the model. No new init/TestMain was found in the reviewed files; the base sqlite tree was also searched for those roots. No executable source selector or parser was used. Guard enforcement of the whole closure remains a separate obligation.

## Controller, protocol and return/ACK causality

The controller alone creates `exec.Cmd` owners. Child roles are closed, the executable is the verified same image, argv contains one anchored role test plus fixed flags, and the child environment is a fixed allowlist. There is no shell/PATH lookup, arbitrary PID, Kill/Signal, CommandContext cancellation, child-spawn entry in writer/probes, scenario retry, or case campaign. Each child starts once, has exactly one actual `command.Wait` call in its owning goroutine, and cannot overlap an un-Waited prior role.

fd3/fd4 are controller-created pipes. The child checks both are distinct pipes. Each frame has a bounded four-byte length prefix and canonical JSON body, bounded sequence, and trial/source/binary/profile/case/role/generation/challenge binding. Canonical re-encoding rejects unknown/duplicate fields, alternate JSON representations, trailing bodies and truncation; parent sequence/state checks reject misplaced/repeated received events. Each grant hashes the exact received raw frame and preserves its binding, sequence, point and observation. Fresh parent challenges and distinct role generations prevent reuse of a previous role's transcript. Terminal EOF now closes the last-frame gap.

The writer invokes public `DB.Rekey` exactly once and immediately assigns the unchanged returned error to `capturedRekeyErr`. Any error fails before the nil-only return barrier. The first three interrupted cases exit inside their target observer and therefore never fabricate a public return/error. Returned-no-ACK checks ACK absence before granting exit. The normal control alone returns through explicit DB.Close and natural process exit.

For acknowledged-exit, only the parent ACK function can write `parent-ack.frame`; it requires the exact fifth received writer frame, a public-nil event equal to the last accepted frame. The write path uses O_EXCL, verifies complete write, file Sync and one Close, then separate directory Sync and one Close, followed by byte/hash/identity readback. The ACK inode is checked against original/trial entries before the causal ACK/exit grant. No child claim, stdout text, successful rename or timer substitutes for this chain. The ACK directory Sync is not claimed as database-directory durability.

Raw diagnostics are separately bounded, hashed and retained. Test output is never parsed into physical PASS. Process records and final observations keep `PhysicalValidated=false`; value-model `Pass()` and `PhysicalValidated()` return false unconditionally.

## Credentials, errors and one-time retirement

Each provider purpose fixes label, exact path and allowed operation before any observed untrusted header: OLD creation/writer/probe/archive purposes and NEW rekey/probe/restore purposes are separate instances. Keys derive from explicitly public disposable recipe strings, not user input, environment secrets, key files, DSNs or observed DBIDs. Acquire checks the fixed operation/path/kind, clones material, and Release clears it once. Duplicate/absent release is rejected; observer snapshots require both writer borrows to be 1/1. No credential bytes or credential-derived secret diagnostics are serialized. Process exit is expressly not a secure-erasure claim.

Each probe is a fresh process with one ReadOnly/NoCreate snapshot Open on its independently copied family. One selected purpose succeeds and authenticates the expected old/candidate header/hash; the other must return nil DB and the original `*sqlite.Error` for operation open, CodeNotADB 26, and authentication identity. The classifier rejects the explicit cleanup/poison/busy/changed/recovery/uncertain/format/limit/provider/cancellation/path/syscall/EOF classes and requires empty public/manual cleanup bookkeeping. No failed Open is followed by Query, Verify, another key, recovery or retained-root fallback.

The actual public error object is captured before further work and kept in the run's in-process causes. Durable receipts contain bounded classification fields, not arbitrary Error() strings. This is not a generic proof about every possible future custom error implementation; it is checked against the present native source and fixed public providers.

Owners are registered immediately after acquisition. Public Rows/DB/file closes increment one attempt before invocation, preserve unknown failure state, and are not retried by deferred cleanup. DB retirement refuses live/unknown child Rows. EOF is exhausted and Rows.Err observed before the single public Rows.Close; a failed internal EOF retirement is marked unknown, so public Close cannot repeat it. A pending cleanup token blocks later native retirement. Process Wait and bookkeeping remain distinct facts.

## SQL/index, selected generation, copies and archive

All seven native query texts and frozen int64/string/string rows match the value model: full scan; blue index/scan; red index/scan; and empty green index/scan. Every query compares independently against literals. Index plans require SEARCH/btree/by_k/k, one index/field and recheck, with positive bounded actual index-page reads even for empty green. Candidates, rowid seeks, base fetches and returned rows are checked; scans require no index work and exactly three base fetches. Iterators are exhausted within three rows and record EOF, Err and one public Close. Check uses MaxPages=3/MaxPageReads=512 and exact page/table/index/row counts plus positive bounded page reads.

Successful probe verification binds snapshot/layout 1, page size/count, DBID, complete selected public header, root hash/head hash and sequence to the baseline or the unchanged writer candidate. Candidate publication validates same DBID, fresh epoch/salt/root identity, sequence 5 from baseline 4 and parent hash. SQL success alone cannot choose a different generation. Full plaintext-page byte equality remains outside the native observation, as disclosed above.

Directories are newly created 0700 and outputs O_EXCL 0600. Family inventories include every entry and reject unrecognized names/types, symlinks, hardlink/identity aliases, plaintext sidecars, bad header/name/role bindings, and nonzero lock files, with 64-entry/64-KiB-file/1-MiB-family caps. Copies preserve the root basename, copy complete bounded bytes into distinct inodes, Sync only their own destinations, reread hashes/length/identity, and compare the source manifest before/after. Freeze and probes occur only after actual writer Wait. Original, exited trial, frozen evidence and both probe families are rechecked for unchanged hashes/sizes/dev/inodes. There is no source repair, Sync supplementation, overwrite, cleanup deletion, rename or retained-root promotion.

The nonce inventory covers bounded original and post-exit historical root/base objects, plus archive/restore objects when selected. Whole-object hash deduplicates byte-identical retained/copy aliases. It checks declared counts, record roles, extents and monotonically fixed sequences, records public credential/domain/prefix/nonce inputs, rejects cross-event nonce/sequence reuse in one public domain and inconsistent bytes for a file identity, and requires one fresh target base. OLD/NEW purpose labels now use the exact owned family/target epoch. These are public-input observations, not secret-key export, randomness proof or an arbitrary-history nonce campaign.

The optional control archive is produced before rekey via BackupEncrypted, has its own DBID/file role/manifest, and is restored using separate OLD source/NEW target purposes into an absent new family. Its NEW readonly reopen gets the same complete SQL/index/Check ledger and is closed once. Source/archive identity and bytes are rechecked; restored DBID differs from source/archive and the native source checks new epoch/file identity against those baselines. This does not restore over an uncertain active root, downgrade format, revoke OLD access to retained history, or establish anti-rollback freshness.

## Remaining acceptance boundary

- This finite source review can accompany the separate guard review and a later request for exactly scoped model/guard/selection/compile-only validation. It does not authorize those actions.
- The new source has never been compiled or type-checked here. No source-reading statement is a substitute for that result.
- Source-model consistency and synthetic counterexamples cannot establish actual process, owner, authentication, ACK, SQL, nonce or archive observations.
- The native facility still lacks plaintext-page hash/counter equality observations. Do not report that part of the stronger proposal as closed.
- A physical run needs its own selected environment, approved case, source/compiler/binary receipt, retained private namespace and unresolved-child disposition. Unknown child state remains a live blocker, not a successful stopping state.
- Power loss, failed rename/Sync/Close, other layouts, shared rekey, candidate write errors, torn/misplaced writes, device loss, ENOSPC, arbitrary restart/nonce histories and broader M8/security acceptance remain outside this source slice.

## Reviewed identities

Proposal SHA-256: `f1a563a176bd09ed5c3718ed1571b6900ef5e87f787bf35b1ac5fbdfa28a1e1e`.

Renewed source freeze: `rekey-interruption-source-evidence/manifest.json`, SHA-256 `e78dcdf71c2a6cff108436f16c120982457ad810aa2d39bba487afbfb99f9b99`. The retained `rekey-interruption-source-evidence/rekey-interruption-source-only.patch` has SHA-256 `6e6f1744c5db6d49e394f6c30c4c90395ca26d8a63a818b9f3a5f023b36f4d36`. Both were independently hashed after the implementation worker announced the renewed freeze.

Eight-file final source freeze (file paths relative to the overlay):

| File | SHA-256 |
|---|---|
| docs/testing/rekey-interruption-manual-zh.md | 25baafd77286649973669711218f8c6d1ffdc1ce434ae4a4315664ac503e8ce8 |
| internal/encstore/publish.go | d4a3d5f2cfb99e2413dbd271f113cc2b08203a4635ef10638bc43ff022d2fc29 |
| internal/encstore/rekey_interruption_manual_linux.go | c7b8362b2098e9ec046acca468b0dcc698552a827bcc01586bf5b3d80c72be8a |
| internal/encstore/rekey_observation.go | 5328be5bb42fb4207331d24d8c7bc157d37244836c24384a72b01ed26f68fbcc |
| internal/encstore/rekey_observation_default.go | 0c44d0577df486ca116b3f332a71f45e34e58367df64e112dba8387c2dfe51bb |
| internal/testkit/rekeyinterruption/contract.go | fd351a9dc85d574d32d5e67763ae898b732ce30a23949c10f7a7ccdc13ab796a |
| internal/testkit/rekeyinterruption/contract_test.go | 2b7b89d5f367fb6dba1985feaaa9f2d4b52c68c076ec5ed270bd7233b80300df |
| sqlite/rekey_interruption_manual_linux_test.go | f267deaf827ce746dbe6ac70b7be134c6c0418e777a843ce1a11e65f5c7a183f |

Relevant base sources independently read and hashed:

| File | SHA-256 |
|---|---|
| internal/encstore/publish.go | adffe2f3c8c4db58a39e3719a72c5ee69f389eccefde876b4e36285dc663ae93 |
| internal/encstore/cleanup.go | a773c72aac60ab150ce05ae85810902396d9c7190434bcbc9ba876c682e91780 |
| sqlite/encryption.go | 02679a9bc79ce5c61201516e3f8216e92fdb70a7ac9fd9db529ee5218a65caa4 |
| sqlite/encryption_copy.go | 962f54bae02b2c658ad84dfb0c64b1f6211b8deab2ff2e0b16450c19751af953 |
| encryption/format.go | ec712e053e32b7c5d5e323f82fcb9eae1b0da1a3e9668215a3790d4a8a3f86c4 |
| encryption/key.go | bcc56e630a263954a2f01dc8e22a22fd6030d6eed5a58b75411d3d32d5c4ffea |
| sqlite/sqlite.go | 0e34c8d0e8a4bf537cb464d544e4dedde50ab04b19e1fbcb426fcca1931ecc6e |
| sqlite/rows.go | 0e164cf3fa34b2255962f99e263f573ff390895df0831a71f3489f2ba36dd8cd |

Hashes identify the local source bytes reviewed. They do not attest a Git commit, executed binary, compiler result or physical experiment.
