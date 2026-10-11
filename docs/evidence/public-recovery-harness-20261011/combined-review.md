# Independent final-composition review: public rollback manual harness

Result: PASS for source review, exact frozen composition, documentation syntax/links, and the six authorized AST/list/compile/symbol commands. No blocking finding. This is a limited compiler-only qualification, not runtime, recovery, durability, driver, M3/M10, or overall project acceptance. Every physical scenario remains **BLOCKED / NOT_RUN**.

The compiler lease was released to the lead immediately after terminal completion at 2026-10-11 09:32:51 UTC. No further Go command is authorized or was run by this review.

## Exact identity and preservation

- Reviewed frozen candidate: `public-recovery-harness-combined-freeze-v1`.
- Independently reconstructed tree: `0af9b064bebfbec9c8270c4bf2f10ec2055fb186`.
- Complete inventory: 3,104 files, 789 non-docs. SHA-256, byte count, Git blob/mode, every file/directory POSIX mode, and directory paths match the lead's frozen inventory and match before/after this review.
- Base: commit `50b627e11e9dbce7f2fde8df35c4634c52a06d52`, tree `8f2709293287a3517a52b3f989888ded8230ef53`, 3,077 files and 788 non-docs. Independently hashed the exact base archive's file bytes and reconstructed its Git tree without executing the archive or writing Git objects/refs.
- All 788 prior non-docs are preserved in bytes, paths, Git identities and POSIX modes. The sole new non-doc is `sqlite/rollback_recovery_manual_linux_test.go`, 10,657 bytes, SHA-256 `dd5a63e2a8e7d4fdd969ba03273a8f8147d5eff81babf8fce44a5228c5cdaf5e`, Git blob `4251553bd2084c0e2d2e586e1f18084dbae646aa`, mode 100644 / 0644.
- That Go file is byte-identical to independently reviewed V3 (and its reviewed V2 source). Production, existing tests, dependencies, goal and CI sources are unchanged.
- Base delta is exactly nine changed docs (the eight composed docs plus checkpoints.json), 26 added docs/evidence files, and that one added Go file. No deletions.
- The eight composed document hashes match their exact base-blob manifest. `docs/progress.md` preserves the complete base prefix. The 43 pre-existing checkpoint records remain semantically unchanged; the 44th records the namespace checkpoint.
- The 23 selected historical evidence files match their published size/hash manifest. All 67 files in the original V1/V2/V3 review evidence manifest were rehashed successfully and remain unchanged.

No main edit, source correction, permission change, cleanup, remote change, branch action, or CI action was performed. Mutable main is intentionally not claimed equal to this freeze: the lead reported post-freeze documentation-only closure updates. Those require their own final readback and do not alter this exact-tree qualification.

## Independent behavioral source review

Read the full goal and complete new source, including all helpers; no applicable AGENTS.md exists in the workspace ancestors or frozen candidate. Also read the original V1/V2/V3 reviews, documentation delta, external AST checker, text checker, testscope exemption, and the pure journal encoder implementation.

- The exact first line is `//go:build manual_recovery && linux`; the filename is Linux-specific. Default and oracle-only selection exclude the file. Adding both oracle and manual_recovery would select it, so that combination is not an ordinary permission or a permitted execution shortcut.
- The unique test's very first statement compares `GOSQLITE_MANUAL_RECOVERY` against `independently_authorized` and calls Fatal otherwise. No Helper, directory read, fixture construction, pending query, or public Open precedes it. The gate value itself grants no authorization.
- No new package variable initializer, init, TestMain, Skip, Parallel, subprocess, fault hook, forced exit, Remove/RemoveAll, cleanup retry, or direct recovery call is present. The only existing init found across the frozen Go sources is the pre-existing database/sql driver registration.
- Root validation requires a nonempty, absolute, clean, existing, empty, non-symlink directory with no symlink components. Fresh scenario directories are retained; adversarial namespace races are expressly outside coverage.
- The fixture helper makes deterministic prepared bytes for an empty schema plus freelist, an altered three-page working image, and a finite two-record journal. The journal functions only validate/allocate/encode/checksum; they do not perform I/O. This is not a process-crash or power-loss simulation.
- Completed fixture input sets have three backups plus the exact manifest bytes checked in fixed order using nonfatal Errorf, so one failed backup check does not suppress later checks. This cleanup is registered before working files/public operations. Partial fixture creation is not represented as a completed fixture or PASS.
- All four ordered t.Run return values are checked and failure stops later scenarios. The top-level pending cleanup is read-only and runs after subtest cleanup, including Fatal paths. DB ownership closes at most once; its attempted flag is set before Close, the error is retained, and explicit failure returns before a reopen. No retry or automatic deletion occurs.

### The four actual assertion sets

1. `nil-default-preserves-input`: public OpenContext with NoCreate and nil recovery policy; nil DB; public CodeIOErr; errors.Is ErrRecoveryRequired; exact working DB/journal bytes; no pending cleanup owner.
2. `opt-in-restores-and-reopens`: public NoCreate plus empty RollbackRecoveryOptions; two-page/512-byte and read/write-version metadata; empty sqlite_schema; successful one-attempt Close; full original DB bytes and absent journal; public read-only NoCreate reopen; metadata, successful Close, unchanged full bytes, absent journal and no pending owner.
3. `journal-limit-preserves-input`: MaxJournalBytes=1; nil DB; CodeIOErr plus ErrRecoveryRequired and ErrMemoryLimit causes; exact DB/journal bytes and no pending owner.
4. `no-create-preserves-absence`: fresh isolated missing path; NoCreate plus opt-in; nil DB and os.ErrNotExist; absent DB/journal/WAL/SHM before and after; entirely empty scenario directory afterward; no pending owner.

These are source assertions, not observed scenario outcomes. Connector physical recovery, broader journals/default-on policies/platforms, the native WAL-tail publisher, full short-write/Sync/ENOSPC/crash/checkpoint/rekey/cryptographic fault facilities, and remaining project acceptance are still gaps.

## Documentation and historical/current claims

- Published manual SHA-256: `66f5d8b83a95d99d613352e130bcf44f7ace0aee82141849d1f9df69c64b2878`.
- Original V3 manual SHA-256: `a49ae1882c4a5dcb8c58860fa8df3b011f88c9ee99963a5721831049477ec4d2`, retained verbatim as `v3-reviewed-manual.md.txt`. Only the published manual's status paragraph differs; every other byte, command and assertion is preserved.
- All code/command fences in the eight composed docs are byte-identical to their base versions. Their 45 shell fences and the topical manual's three shell fences passed `bash -n`: 48 total. Two embedded manual Python snippets parsed with ast.parse. No shell-fence contents or runtime-evidence collector were executed.
- The manual's exact entrypoint, build tag, gate variables, four names, NoCreate/ReadOnly/MaxJournalBytes assertions, compile-only `-c -p=1 -vet=off`, exact runtime selector/count and retained-evidence criteria match the source. Its selection checker is fail-closed 0/0/1, and its runtime criteria require all four named cases, no skip/failure, and complete identity/exit evidence. The printed future runtime command remains unexecuted.
- All 196 relative links/anchors checked in the eight docs, topical manual and new summary resolve against the frozen tree. This includes all five previously pending links to `docs/evidence/public-recovery-harness-20261011.md`.
- Historical V1 CHANGES_REQUIRED and V2 source PASS remain distinct. V3's 08:58 compiler PASS belongs to tree `1755d67431f9de082449b08fa24b26b2b35eb612` on P0, not to this namespace-based composition. This report supplies the separate composition result.
- The freeze correctly keeps the namespace CI observation as nonterminal (summary last known 09:26: ordinary running, oracle successful). No terminal CI result was inferred here. The lead later reported terminal CI and documentation closure; those are outside this immutable freeze and this report does not independently attest them.
- Earlier implementation-missing statements retain their historical scopes; current sections correctly close only the limited public Open harness source gap and preserve physical BLOCKED / NOT_RUN and genuinely missing driver/full-fault/native-WAL facilities.

## Exact six Go commands and environment

`compiler/commands.json` preserves complete argv, cwd, UTC start/finish, stdout/stderr filenames and natural exit codes. It is the V3 command list with only candidate/output-path substitutions. All six Go commands and the separately approved Python text checker exited 0 with empty stderr.

1. External standard-library-only AST inspector: 09:32:42.029498–09:32:42.262437 UTC. Reads the candidate as text; does not import or execute its package. Gate, constraint truth table, unique entrypoint/no eager hook, four fail-stop scenarios, five public Open references, and no cleanup retry/deletion passed.
2. Default go list: 09:32:42.263030–09:32:42.379243 UTC.
3. Oracle-only go list: 09:32:42.380164–09:32:42.443925 UTC.
4. manual_recovery go list: 09:32:42.444661–09:32:42.502162 UTC.
5. Linux/amd64 `go test -c -p=1 -vet=off -tags manual_recovery -o <evidence>/public-recovery.test ./sqlite`: 09:32:42.531900–09:32:51.150822 UTC. Compile/link only.
6. `go tool nm <evidence>/public-recovery.test`: 09:32:51.152314–09:32:51.286834 UTC. Symbol metadata only.

The Python file-selection text checker between stages 4 and 5 required exact counts default=0, oracle=0, manual=1 and passed. Text-only symbol validation found exactly one T symbol named `github.com/ms-xiaosnow/gosqlite/sqlite_test.TestManualPublicRollbackRecovery`.

The binary is 14,761,657 bytes, SHA-256 `cf3db787f6e4cc887783f83c53253a547a3f5ab58aa59cdbb472bc0ab4d4569d`. It was never executed, including unmatched selectors, gate-off, smoke runs or -test.list.

Pinned Go/GOROOT: `toolchains/go1.27.2`; static VERSION and executable SHA-256 verified. GOTOOLCHAIN=local, GOENV=off, GOWORK=off, GOPROXY=off, GOSUMDB=off, GOFLAGS=-buildvcs=false, GOMAXPROCS=2, CGO_ENABLED=0, GOOS=linux, GOARCH=amd64; existing ROOT/go-path and ROOT/go-cache used. All GOSQLITE_MANUAL_* and GOTMPDIR values were unset; default /tmp retained. No alternate cache/tmp path, cleanup or downloads were used.

Capacity/inode/writability checks ran before every Go command. Minimum observed /tmp availability was 1,413,300,224 bytes / 1,243,469 inodes; workspace 10,672,844,800 bytes / 1,140,579 inodes. All exceed the required 1 GiB / 2 GiB and 10,000-inode thresholds. No Go version/env/test/build/vet/guard/oracle/race probe beyond the six exact commands was invoked.

## Evidence index

- `identity-and-static.json`: independently reconstructed base/candidate identity, exact delta, doc hashes, syntax/link counts.
- `pre-inventory.json`, `post-inventory.json`: all frozen bytes, Git identities, exact file/directory modes and paths.
- `source-review-gate-proof.json`: source review completed before any compiler-stage command.
- `shell-syntax.json`, `shell/`: 48 parsed, unexecuted shell blocks.
- `relative-links.json`: 196 resolved relative links/anchors.
- `history-and-qualification.json`: 67 original evidence hashes, prior checkpoint preservation, resource minima and no-runtime boundary.
- `preflight.json`, `compiler/resources.json`, `compiler/commands.json`: static tool/environment evidence and all exact command/precommand resource records.
- `compiler/*.stdout`, `compiler/*.stderr`, `compiler/*-files.txt`, `compiler/symbols.txt`: actual raw outputs.
- `compiler/exact-symbol.json`, `compiler/artifact-identity.json`, `compiler/public-recovery.test`: compile/link artifact identity; binary never executed.
- `result.json`: terminal scoped PASS and complete before/after preservation.
- `review.py`, `run_compiler.py`: external read-only review/compiler orchestration source; not project source.
- `evidence-sha256.json`: final evidence manifest excluding itself.

All physical/recovery/fault/namespace operations, ANN V4, the 41-target campaign, full ordinary aggregate and oracle runtime remain NOT_RUN by this reviewer. This review stops at the explicitly authorized source/selection/compiler boundary.
