<!-- Original: checkpoint-extension-independent-review.md -->
# Independent source review: finite R3/R4 checkpoint/extension facility

Review date: 2026-10-12 UTC. Scope: the sixteen source/manual files in `checkpoint-extension-overlay`; the complete `checkpoint-extension-source-proposal.md`; the overlay handoff; relevant existing ordinary API/storage/query implementations. This review did not edit the working tree, integrate a patch, or touch a remote.

## Verdict and execution boundary

The finite facility is source-review CLEAR after the original author corrected the three defects listed below. This is a human/source-and-byte review, not a parsed, formatted, compiled, tested, executed, or physically qualified result. The separate testscope/guard integration has its own reviewer and acceptance. Its HOLD pins must not be replaced merely on the strength of an old hash or this review's title.

Go, gofmt, Go parser, compiler, model tests, AST tests, source-selection tools, database construction, checkpoint, native controller, process exit, reopen, and physical validation were all NOT_RUN by this reviewer. Operations used were source reads, grep/sed, Python file/JSON/hash/diff bookkeeping, and this review-file write. No pure predicate was evaluated through another language. The native commands in the Chinese manual were read only. The lead's later authorization for a future-user launch recipe supersedes the proposal's older sentence excluding such a recipe; it does not authorize execution here.

The fixed scope remains ten cases, three profiles, the combined 315-row fixture, twelve pure roots, three native roots, and seven added void production call lines. No additional interruption case, profile/index product, ANN campaign, rekey mode, replay/repair path, or layout-3 requirement was introduced.

## Defects found and corrected by the original author

1. The initial native encrypted family inventory decoded public headers and checked role/FileID naming but did not enforce complete ciphertext extents. A malformed or header-only unselected retained/pending/base/WAL file could satisfy that native inventory even though the conceptual pure-family validator checked extents. Because the native caller truthfully does not evaluate the full conceptual Evidence model, those checks could not be delegated to its unused validator. The author added `manualCheckpointCiphertextExtent`, called for every non-lock encrypted family object. It checks finite 4096-page geometry and sequence, exact role-specific total length, public record role/sequence/plaintext length, and complete end-of-file. Root metadata has the layout-2 512+256*n form; base extent is 704+pages*4136; segment extent is 928+frames*4140. I compared these formulas and record order directly with encstore and encryption writers. It also binds each object's complete public Epoch, including DBID, to the selected family's Epoch. These are public framing/identity checks, explicitly not authentication of retained history.
2. The initial options omitted retention fields and consequently used production defaults of 256 generations, 1 GiB, and 16384 entries. The corrected native options explicitly set MaxGenerations=16, MaxRetainedBytes=64 MiB, and MaxDirectoryEntries=64, preserving the already fixed per-file, WAL, SQL, page, and family bounds. Whether the composed fixture fits them remains NOT_RUN.
3. Initially, an interrupted encrypted writer self-exited before `saveRole`, so its providerResults check was never reached. The corrected writer evaluates and retains its credential lifecycle before the checkpoint-ready artifact and before arming. The controller requires exactly zero plain providers or one encrypted writer-open/trial/OperationOpen provider with one acquisition, one release, and no duplicate before granting checkpoint. The original control/probe checks remain. This closes the abrupt-writer path rather than converting its exit into an orderly DB close.

Only the native source file changed for these corrections. The initial sixteen-file freeze is preserved as `checkpoint-extension-overlay/PRE-REVIEW-FREEZE-e959f814.zip`, SHA-256 `17e16532c95a65f71731e9f0d625684916a7d4a9ed03a7a910fc3f76381e4e37`. The initially reviewed manifest was `e959f81438eb191934cb180cb065030d6f4a615fe516416f2c2c1ed4069f3545`; it is superseded by the final identity section below. The corrected native file reviewed here has SHA-256 `598ae5a3257c0fab5f755f603199245b70e477cd9881031d66aea04e46ebcce0`.

## Default isolation and seven-seam production order

Independently removing only the seven new call lines reproduced the selected baselines byte-for-byte:

- `internal/pager/checkpoint.go`: exactly two `checkpointObserve` lines removed; baseline SHA-256 `34938ce870f397be1f487c816f9817ed2f2f2a82cf1e79e606fbe4332132c811`.
- `internal/pager/shared_checkpoint.go`: exactly two lines removed; baseline SHA-256 `a726cea6df0f1841af5fd28dc72db70d9807e1c83a7d7fe1b56ea5b4b43b6cd5`.
- `internal/encstore/publish.go`: exactly three `checkpointPublicationObserve` lines removed; R5-overlay baseline SHA-256 `d4a3d5f2cfb99e2413dbd271f113cc2b08203a4635ef10638bc43ff022d2fc29`.

Exclusive copy observation follows successful writeFullAt; terminal observation follows successful WAL unlink, after main Sync and WAL Close, before stale-SHM removal and directory Sync. Shared copy follows successful writeFullAt after attempted and WAL Sync; its terminal observation follows successful SetBackfill after main Sync. No operation, error return, poison behavior, lock, or ordinary cleanup was moved.

Publication observations are adjacent to the three existing R5 observers at before-rename, after successful rename, and after final successful namespace Sync. R5 calls and their arguments remain intact. The checkpoint observer uses a separate context key/purpose and tag. In an R5 context the new root adapter returns before decoding; in the ordinary build its empty void stub does nothing. No R5 case, credential purpose, layout-1 bound, or six-case acceptance was broadened.

Both default stubs use `!linux || !manual_checkpoint_restart`; the two native adapters, common bridge, and three role roots use `linux && manual_checkpoint_restart`. The vocabulary files contain scalar enum declarations only. The four pure package files use only descriptor-free standard-library value computation; they import neither SQLite, pager, encstore, encryption, os, io, process APIs, nor native constructors. Default production imports do not acquire the manual bridge. This is source-selection reasoning, not a claim that go list or a non-Linux build was run.

## Committed C, authority, and observer purpose

The seed uses the supported ordinary public Tx APIs. It creates the three literal tables and 312 rows in the first data transaction, commits, runs exactly one unarmed public preparatory Checkpoint, then inserts the three omitted rows and creates grams/HNSW/IVF indexes in the second data transaction. The adjacent original Commit result must be nil before full C verification and the exclusive external committed-head receipt. Its write, file Sync/Close, directory Sync/Close, and identity/hash readback precede seed Close and writer Start. There is no checkpoint acknowledgement in interrupted cases.

All writers independently obtain the full C ledger and exact original authority before parent checkpoint grant. The unique ManualContext call is adjacent to the public DB.Checkpoint call; no direct pager/encstore checkpoint entry substitutes for it. The bridge validates the writer role, channel sequence, path, head and closed case. For plain authority it checks the complete selected-copy progression: strictly increasing pages, first-copy-only barrier, finite count, exact terminal count. It releases its own mutex before pipe exchange and never re-enters DB/session/store. Controls alone can report public nil after every expected point.

Plain C is bound to a clean complete bounded WAL scan, full selected page hashes, salt/checksum/frame/DBSize/end extent, whole main/WAL hashes, and a complete logical page digest. The current fixture uses equal 4096 page/sector values, so sector protection cannot silently append a different generation. Shared selected pages project out prior backfill; safe remains the final frame. Post-Wait exclusive retirement requires no WAL and the exact C main length. Shared cases retain main/WAL/SHM inodes; interrupted cuts directly check attempted and the expected old/new backfill from frozen SHM, while probe reconstruction is allowed to change transient SHM.

Encrypted C is authenticated through public VerifyEncrypted, with layout2/EncryptionWAL, sequence4, one segment, bounded geometry, and the old authenticated HeadCommitID. The observed old root is exactly1320 bytes and candidate1064 bytes. Candidate keeps the complete Epoch and geometry, has a fresh root FileID and sequence5, and ParentCommitID equals the old authenticated WAL head, not a root ciphertext hash. Before-rename selects the exact old root/Info; later cuts and control require the candidate root, sequence5, zero active segment/frame/byte counts and a new authenticated base head. The actual verifier authenticates the selected manifest/base relationship; public header decoding is never labeled authentication. No encrypted plaintext page-hash claim is made.

## Complete extension ledger and actual search paths

I reviewed all thirteen StreamSQL entries, full typed cell copying, explicit SQL/args/column/row identity encoding, EOF/Rows.Err/one-Close receipts, semantic validators, and the complete digest comparison at writer and probe. The SQL shadow streams retain physical row IDs and complete vector/adjacency BLOBs. All ordinary method signatures and result mirrors were compared with the relevant source: extension_api/ann_api/ivf API, plan_grams, similarity_search, vector_search, check, rows, encryption, grams configs, ANN configs/storage, and WAL/index definitions.

- All seven docs rows, NULL/empty distinctions, all180 HNSW rows and all128 IVF rows are checked against independent fixed generators. IVF values preserve six-decimal SQL formatting followed by float32 conversion.
- Complete schema includes twelve tables, nine ordinary shadow indexes and nine guard triggers; three catalogs/fields have exact names/methods/generation/current schema/source hashes and frozen canonical configs.
- Independent ASCII gram enumeration covers both namespaces and all lengths1..3, requiring exactly96 posting rows and68 frequency rows with complete unique token/document membership and exact frequency counts.
- HNSW requires180 live nodes/mappings, all source vector identities, no tombstones, next_node_id181, valid entry/max-level and complete bounded adjacency structure. Exact graph bytes are frozen across C comparisons. Actual CheckExtensionIndex additionally checks full persisted graph integrity; one positive ANN work counter is not used as a whole-graph proof.
- IVF requires ready state, four centroids and all128 memberships. It independently selects each nearest persisted float32 centroid using float64 squared distances and list-order ties, enforces one member per source and exact counts, and excludes HNSW nodes/live records from IVF.
- Six actual public searches are retained. Grams indexed/scan returns IDs1,2 at2/3 with the fixed SEARCH/union and work counters. HNSW indexed/exact results equal independently rescored IDs20,19,18,17,23; the indexed run requires genuine bounded graph work. IVF uses one real list, never Probes=Lists, and requires32 member/base/rescore/identity operations and IDs1..5. Canonical native query mirrors preserve all public plan/stats/hit fields; no EXPLAIN result is substituted.
- All three named CheckExtensionIndex calls and bounded full DB.Check must return nil. Exact check counts are Tables12, Indexes9, ExtensionIndexes3, TableRows979, IndexEntries789, ReservedPages0. Seed Pages/FreePages persist; only permitted adapter-dependent Check.PageReads is excluded from C digest.

The native path directly invokes ValidateLedger/CompareLedgers and ValidateAuthority. `AuthorityModelConsistent` and `LedgerModelConsistent` are distinct from `FullModelEvidenceEvaluated=false` and `SourceModelConsistent=false`. The latter false flags are honest scope declarations and do not guard, short-circuit, or bypass required native assertions. The conceptual complete Evidence trace is not fabricated from constants. PhysicalValidated remains false in both model and native observations.

## Process, transport, ownership, and copies

There are exactly three native test roots. The controller owns at most writer then probe, at most two Starts, one actual Cmd.Wait per concrete started Process, fixed absolute same-inode self-image path, anchored test flags, and allowlisted environment. It checks complete source inventory, compiler/binary/compile receipt and /proc/self/exe identities. There is no shell, PATH spawn, CommandContext, Kill, Signal, PID-based recovery, replacement process, or child-spawning child in this closure.

Control/event descriptors are parent-created pipes3/4. Canonical bounded frames bind source/compiler/compile receipt/binary/trial/fixture/profile/case/role/generation/challenge and monotonic sequence. Grants echo full event hashes and observation identity. Ready, checkpoint-ready, ordered observations, public-return for controls, and result events are checked separately. Post-exit acceptance requires one actual unsignaled ProcessState with exact exit86 or0 plus terminal event-pipe EOF; protocol abort87, panic/nonzero control exit, extra bytes, or diagnostic overflow fail. Pipe retirement is after actual Wait/EOF.

A timer cannot release authority. Failure while a child is live/unwaited retains the actual Wait channel in the controller's deferred unresolved-child hold; nil ProcessState passively holds forever without a second Wait. A later disposition is recorded as late evidence and does not repair the failed run. Direct Open/Rows/Tx/file capability owners are captured before error decisions; pending registries, dependency ordering and one attempted retirement prevent retry-to-success. An abrupt writer has no deferred DB.Close execution because os.Exit bypasses it. Ordinary seed/control/probe each explicitly close once.

Copies create private0700 directories and exclusive0600 regular single-link files, check exact owner/modes, fd/path identities, sizes/hashes, source before/after equality and cross-family disjoint dev/inode identities. The corrected encrypted inventory includes bases, segments, pending and retained roots, rejects unknown names, wrong roles/IDs/Epoch/extents, symlinks, hardlinks, directories and oversized inventories. Only the new copy destination and its containing namespace are synced. Original, post-Wait trial, and post-exit evidence are never opened as databases or repaired; final complete manifests are rechecked. Probe opens one distinct mutable copy. Plain supported native Open may reconstruct/invalidate SHM; main/WAL bytes remain unchanged. Encrypted probe is same-key ReadOnly/NoCreate/EncryptionWAL. There is no fallback reopen or retained-root promotion.

## Finite affected validation closure (NOT_RUN)

Pure/model package `./internal/testkit/checkpointrestart`, exactly these twelve roots:

1. TestCheckpointRestartTenCases
2. TestCheckpointRestartAuthorityCuts
3. TestCheckpointRestartCommittedHeadCausality
4. TestCheckpointRestartTransportBinding
5. TestCheckpointRestartOwnerWaitAndUnknown
6. TestCheckpointRestartPrivateFamilyCopies
7. TestCheckpointRestartEncryptedSameEpoch
8. TestCheckpointRestartNeverPhysicalPass
9. TestCheckpointExtensionFixtureRows
10. TestCheckpointExtensionGramsLedger
11. TestCheckpointExtensionGraphMembershipLedger
12. TestCheckpointExtensionQueryPathsAndCheck

The finite test source covers all ten synthetic positives and enumerated negatives for lost/reordered cuts, stale/missing C, incorrect copy projection, forged ACK/commit identity, frame replay/canonicality, missing/repeated Wait/EOF, unknown owners, copied aliases/unexpected files, wrong Epoch/root-hash parent, complete postings/graph/membership, false query path, wrong distances and Check totals. These are synthetic-value tests, not evidence that native calls happened.

New testscope AST/source roots, owned and reviewed separately, exactly:

1. TestCheckpointRestartGuardInventoryAndGate
2. TestCheckpointRestartGuardProductionOrder
3. TestCheckpointRestartGuardObserverPurpose
4. TestCheckpointRestartGuardPublicEntryAndCommitReceipt
5. TestCheckpointRestartGuardProcessOwnership
6. TestCheckpointRestartGuardFullExtensionLedger
7. TestCheckpointRestartGuardPrivateCopiesAndProbe
8. TestCheckpointRestartGuardDefaultAndIntegration

The corrected ciphertext helper, retention literals, Ready.Providers field and both pre-arm provider checks belong in that final protected native declaration/call closure. Repinned semantic mutants must exercise the obligations rather than succeed through stale-byte rejection. Any final guard refreeze must use this corrected native identity, not e959f814's old native hash. Changed main/oracle and R5 shared-publication obligations remain part of the separate guard integration review and its exact affected old selectors; this report does not approve an arbitrary whole-repository/native test run.

Three roots that must remain unexecuted in source validation: TestManualCheckpointRestartController, TestManualCheckpointRestartWriter, TestManualCheckpointRestartProbe. A later authorized metadata-selection/compile-only plan may cover Linux default, exact manual tag, non-Linux default, R5-only unchanged selection, and specifically approved combined tags. It must not launch a produced binary, even with -run=^$, or claim a prior execution result.

## Remaining limitations

No parser/compiler result is inferred from visual source review; source formatting and the final old/new guard integration may reveal a narrow issue during separately authorized checks. The combined fixture, exact native counters/Check totals and configured quotas are source-derived expectations, not measurements. Filesystem, same-kernel shared-WAL reconstruction, actual process disposition, encryption checkpoint/reopen and power-loss durability all remain unobserved. The guarded finite closure is not a theorem about arbitrary Go programs, malicious same-UID races, arbitrary filesystems, external SQLite concurrency, layout3 maintenance, general ANN recall/performance, or physical sectors. None of those limits justifies expanding the locked facility.

## Final source identities

The following identities are for the actual bytes independently read. Proposal SHA-256: `74ba6412f77953322b908b49b4e47912b76fa5dfcf265f3001d90bd3d05352c1`.

Final overlay manifest SHA-256: `dd9e49ac002db6d2c57e0cadcc1e1fbdf94d8131ee4fc86c136db448d7523441`. Final overlay handoff SHA-256: `f7f3ff3d1eaa2e2af592523f1a70d7962882ca10c9cea6097c81985f361d5d17`. Overlay patch SHA-256: `562a632c17c1bbc824de608e1bbd3e464c49cbd81a4ad99d52d16771de902755`. All sixteen manifest entries were rechecked against the actual source bytes after the author's corrections and matched.

| File | Bytes | SHA-256 |
|---|---:|---|
| `docs/testing/checkpoint-restart-manual-zh.md` | 14201 | `297d6b94908880fb56274e7d743a5c0374892a9b8c63bebf7a0a465e9f5067b7` |
| `internal/encstore/checkpoint_observation.go` | 267 | `53e8e9d5f624dd5c5048451f39d6100d84e552b964b54734e397e0b203418f2b` |
| `internal/encstore/checkpoint_observation_default.go` | 191 | `0f7afd8f55157f2c97818e4ab776b7e71aca8e2d6318371a88153c4f5f05e683` |
| `internal/encstore/checkpoint_restart_manual_linux.go` | 1136 | `a6a3ad7c36ae0a098bc8ff4053f899ff3d8feeed22b700ed7ef34ce246f8e6ea` |
| `internal/encstore/publish.go` | 15081 | `76b24327571e519ed8a59661fd552571d4d4ae6ef19bb9bb2e1c680d86bcc86b` |
| `internal/pager/checkpoint.go` | 5457 | `3b70e940343ccdd2a02bb88d086dc3304364f6c842d6ef985645c5ebc3f2a36f` |
| `internal/pager/checkpoint_observation.go` | 247 | `4283c5c8cda005988f65c018cb79de16887fa9947e01c280ecf4ccd20ba11136` |
| `internal/pager/checkpoint_observation_default.go` | 220 | `88326037f7bc2b9324f22f99eee9e296b26ff0d03e7ccfd5c4c63e9b24a183ea` |
| `internal/pager/checkpoint_restart_manual_linux.go` | 544 | `eb6a14c2ac23c55e2063b3154c5002abb02ec5c879140d2f657e521b2e380a65` |
| `internal/pager/shared_checkpoint.go` | 6372 | `a32ba18f1f7e365cbdbc7aa5bb1a91063254d238016117c2298bce242906940d` |
| `internal/testkit/checkpointrestart/contract.go` | 39475 | `9576b18a43d847b32a5e7103d915755c5cb02286131a64d26cb2da0a78c7f7e7` |
| `internal/testkit/checkpointrestart/contract_test.go` | 18503 | `ab8fae12a7e9e2b77366d8021a9f8731121856a21005dfda36d1e4d73b775546` |
| `internal/testkit/checkpointrestart/extension_ledger.go` | 35305 | `8effc48cef13f6e742ba74f9350d14274ef8b5b9cca6154cc13b03659a7dae6f` |
| `internal/testkit/checkpointrestart/extension_ledger_test.go` | 11548 | `16236dcf77f7ce296a46f4198f3a6489986911d029c35e861c07ce34e958fb8c` |
| `internal/testkit/checkpointrestart/manual_linux.go` | 13536 | `47f23e8061c7016db9096534382579f6a0f52d9756e17918d3b36abc3cd2139a` |
| `sqlite/checkpoint_restart_manual_linux_test.go` | 77912 | `598ae5a3257c0fab5f755f603199245b70e477cd9881031d66aea04e46ebcce0` |


<!-- Original: checkpoint-guard-independent-review.md -->
# Independent R3/R4 checkpoint guard integration review

Date: 2026-10-12 UTC. Verdict: **CLEAR for the exact final guard source overlay and finite source/integration contracts reviewed here.** This is source-review clearance, not a compiler/test/native/persistence PASS and not execution authorization. The separate complete facility review owns the native/model correctness verdict and has also returned source-review CLEAR for the same corrected source manifest. Its report is `checkpoint-extension-independent-review.md`, SHA-256 `23c8f2ce1273991d1ca95decdaddcdb924b0614be1a2c431ddefdafa7f811d14`, independently read and hash-verified here. No guard finding remains unresolved in this freeze.

## Scope and method

Reviewed the complete changed main function and unchanged `inspectSource`, the new checkpoint inspector and all eight source-test roots, oracle process admission, readonly wiring and closed mutation register, changed R5 policy/snapshots/tests, actual public checkpoint/process/bridge closures, source ownership, pin ownership, and default/tag exclusion. Read the source proposal, final worker policy, and final corrected facility source. Coordinated with the independent facility reviewer rather than duplicating its complete ledger/native audit.

Only read-only filesystem/text operations, Python byte/literal/JSON/hash comparisons, and writing this report were performed. No Go, gofmt, Go parser, compiler, test executable, source scanner, checkpoint, database fixture, process controller, native child, process exit, reopen, physical experiment, ANN V4 or campaign was run. No W file was edited. JSON manifest/literal decoding is not execution of Go or the source templates.

## Exact reviewed freeze

- Source proposal: `74ba6412f77953322b908b49b4e47912b76fa5dfcf265f3001d90bd3d05352c1`
- Corrected 16-file facility manifest: `dd9e49ac002db6d2c57e0cadcc1e1fbdf94d8131ee4fc86c136db448d7523441`
- Final eight-file guard manifest: `edaa9bac8613930b4cf0440a0ab9d3d0443864c904c4421739c0ef88d276cf60`
- Guard full diff: `2f8c21e2d0b05c2083383cf1f0b8bcd57ac6f92a3228dc6b0d607cfb2bec550d`
- Guard source policy: `8b250b1c2e9f1667a195f824580101d954d7b67411548eeb35445e3ab14c6139`
- Finite affected-root inventory: `67c93d936f2751f8960e4df7e839121f324874d7abcc0b617d133dd72ea64534`

The corrected facility manifest supersedes `e959f81438eb191934cb180cb065030d6f4a615fe516416f2c2c1ed4069f3545`. Every file length and SHA-256 in both final manifests was independently reproduced. Every replacement baseline hash in the guard manifest matched the current W file. Reconstructing the unified diff from those exact baseline and overlay bytes reproduced `OVERLAY-PATCH.diff` exactly.

### Eight guard files

| Path | Bytes | SHA-256 |
|---|---:|---|
| `internal/testscope/checkpoint_restart.go` | 284543 | `7fd1fe09e2503e18d1269140ad5548c9783be1febd52156ed83cebd32f586eef` |
| `internal/testscope/checkpoint_restart_test.go` | 14045 | `1ed2967e823623e9db960ec1f1b5e216a4563b426f1a55bfc635021e9021e080` |
| `internal/testscope/main.go` | 12044 | `9d29dda532e851dd0fa2211473a5abb24309f798a849483c7efca09a7a3a058f` |
| `internal/testscope/oracle_startup.go` | 11371 | `f7d7f6f02e886dfe057a4494564093f78a7644dffe2fa4c41fafa2929227edda` |
| `internal/testscope/readonly_wal_manual.go` | 84941 | `55c4393ab9af5d372266d521a9985d61aa310d3e7d7b1eb325d63ac61fb7d7a1` |
| `internal/testscope/readonly_wal_manual_test.go` | 51011 | `2a2635e8dc47b7caf2cdc32f4181d367fb7cf998c560872da8ff809232430a57` |
| `internal/testscope/rekey_interruption.go` | 293438 | `f67321ace55091535a35000d8b2c5b74767b39fb58034dbf649d56940a45bdd4` |
| `internal/testscope/rekey_interruption_test.go` | 11950 | `4e58cb737ad468ca0a43861752a6549ebb117e4858c1b7fd1316430bc92d14f0` |

### Eighteen new held source pins

All 18 checkpoint pins remain the literal `HOLD_PENDING_INDEPENDENT_CHECKPOINT_FULL_SOURCE_REVIEW`. The guard does not self-release them. Seventeen complete Go declaration templates equal their final source bytes exactly; the eighteenth target is the manual document, which is byte-pinned rather than parsed. The precise release targets for this freeze are:

| Path | SHA-256 |
|---|---|
| `docs/testing/checkpoint-restart-manual-zh.md` | `297d6b94908880fb56274e7d743a5c0374892a9b8c63bebf7a0a465e9f5067b7` |
| `internal/encstore/checkpoint_observation.go` | `53e8e9d5f624dd5c5048451f39d6100d84e552b964b54734e397e0b203418f2b` |
| `internal/encstore/checkpoint_observation_default.go` | `0f7afd8f55157f2c97818e4ab776b7e71aca8e2d6318371a88153c4f5f05e683` |
| `internal/encstore/checkpoint_restart_manual_linux.go` | `a6a3ad7c36ae0a098bc8ff4053f899ff3d8feeed22b700ed7ef34ce246f8e6ea` |
| `internal/encstore/publish.go` | `76b24327571e519ed8a59661fd552571d4d4ae6ef19bb9bb2e1c680d86bcc86b` |
| `internal/pager/checkpoint.go` | `3b70e940343ccdd2a02bb88d086dc3304364f6c842d6ef985645c5ebc3f2a36f` |
| `internal/pager/checkpoint_observation.go` | `4283c5c8cda005988f65c018cb79de16887fa9947e01c280ecf4ccd20ba11136` |
| `internal/pager/checkpoint_observation_default.go` | `88326037f7bc2b9324f22f99eee9e296b26ff0d03e7ccfd5c4c63e9b24a183ea` |
| `internal/pager/checkpoint_restart_manual_linux.go` | `eb6a14c2ac23c55e2063b3154c5002abb02ec5c879140d2f657e521b2e380a65` |
| `internal/pager/shared_checkpoint.go` | `a32ba18f1f7e365cbdbc7aa5bb1a91063254d238016117c2298bce242906940d` |
| `internal/testkit/checkpointrestart/contract.go` | `9576b18a43d847b32a5e7103d915755c5cb02286131a64d26cb2da0a78c7f7e7` |
| `internal/testkit/checkpointrestart/contract_test.go` | `ab8fae12a7e9e2b77366d8021a9f8731121856a21005dfda36d1e4d73b775546` |
| `internal/testkit/checkpointrestart/extension_ledger.go` | `8effc48cef13f6e742ba74f9350d14274ef8b5b9cca6154cc13b03659a7dae6f` |
| `internal/testkit/checkpointrestart/extension_ledger_test.go` | `16236dcf77f7ce296a46f4198f3a6489986911d029c35e861c07ce34e958fb8c` |
| `internal/testkit/checkpointrestart/manual_linux.go` | `47f23e8061c7016db9096534382579f6a0f52d9756e17918d3b36abc3cd2139a` |
| `sqlite/checkpoint_restart_manual_linux_test.go` | `598ae5a3257c0fab5f755f603199245b70e477cd9881031d66aea04e46ebcce0` |
| `internal/testscope/main.go` | `9d29dda532e851dd0fa2211473a5abb24309f798a849483c7efca09a7a3a058f` |
| `internal/testscope/oracle_startup.go` | `f7d7f6f02e886dfe057a4494564093f78a7644dffe2fa4c41fafa2929227edda` |

HOLD is deliberate. A valid source is refused by the pin plane before release; that is not an unexplained false positive or a request to bypass the pin. A later formatting/body change requires reviewing the actual changed source and keeping templates, manifests and independently owned pins consistent. This review does not authorize blindly refreshing hashes.

## Correctness and integration findings

### 1. Whole main and prior policies

The complete main preserves the original all-Go WalkDir, error propagation, all prior source inspectors, ordinary-test restriction, current-source oracle/WAL filter, all previous root contracts, failure accumulator, final failure exit and success message. Its only substantive additions are:

1. The direct checkpoint source triplet before R5: inspect, return its original error, append its findings.
2. The direct whole-root checkpoint contract after R5: inspect, report error/set failure, report every finding/set failure.

There is no early return, disabled branch, swallowed finding, weakened tag exclusion, or collector reset. `inspectSource` is byte-identical; its exact 4,519-byte function source SHA-256 remains `98341345e43f39b39dbc78eaf49a916a0701f3d7f2f383d4949b9a3ed439d199`. The old recovery exceptions and scalar Signal forwarding are unchanged.

The final main bytes reviewed here are SHA-256 `9d29dda532e851dd0fa2211473a5abb24309f798a849483c7efca09a7a3a058f`. The older whole-main pin owners remain outside this overlay and must be updated consistently only for the independently accepted final main: `walPublicationFrozenSources`, `formatUpgradePins`, `encryptedRootPins`, and the exact `reviewedMainSHA256`/`reviewedWholeMain` literals in rollback_fault_test.go, rollback_fault_sql_test.go and rollback_restart_test.go. The prior ordinary-function pin must not change.

### 2. Exact source and process closure

The checkpoint policy owns exactly 15 facility Go files, with the two guard files exempted only from their own protected-name scan. It rejects added files in the closed facility path families, unauthorized consumers/import aliases, copied protected capabilities, broadened/relocated tags, unapproved compiler directives, package drift and declaration drift. It inspects excluded-tag source as source; a manual tag is not a global scanner exemption.

The sole new oracle-startup admission is the exact native caller path, first-line Linux AND tag, direct unaliased `exec.Command(r.admission.BinaryPath, manualCheckpointFlags(role)...)`, exact os/exec import binding, one Command occurrence, and successful complete checkpoint source contract including the native pin. A HOLD native pin cannot admit it. The oracle's earlier approved calls, pointer/type restrictions, factory contract and frozen source pins are unchanged; its only change is the additional bounded predicate in the existing decision.

The actual frozen `start` body restricts spawning to the controller, closed writer/probe roles, no duplicate role, at most two children and no overlap without actual Wait. Admission checks exact role flags, unprivileged Linux, empty GOFLAGS, full source manifest, compile-only receipt/compiler identity, actual `/proc/self/exe` bytes and inode, and the same spawn-path inode/bytes. The spawn rechecks the target. No PATH search, shell, arbitrary selector, CommandContext, replacement, Kill or Signal is admitted.

Start and the two bridge self-exits have exact function/call contexts. The one Wait is admitted only as the exact `go func(){ err := command.Wait(); child.wait <- ... }()` within `if command.Process != nil`, itself owned by `(*manualCheckpointRun).start`. Ordinary enclosing-function resolution still refuses unrelated closure crossings. Complete declaration templates separately freeze receiver identity and surrounding source.

Read the actual writer, controller, process ownership helpers, bridge and adapters: fd3/fd4 roles, fixed environment/flags, source/binary/role/generation/challenge/sequence binding, original error capture, granted cuts, one actual Wait, terminal EOF, failure retention and post-Wait copy ordering are present in the frozen closure. Unknown process disposition cannot manufacture success or permit dependent copies/probes. Writer readiness now includes the checked provider receipt before the public checkpoint grant; the final template contains and protects this correction.

The bridge's unique concrete context belongs to the writer and requires sequence 2. The sole public DB.Checkpoint call is immediately after arming and preserves its returned error. Target self-exit does not run deferred DB.Close. Controls alone traverse the complete observer sequence and nil-return barrier. Probe calls are separate, on a private copied family. This describes source intent; none of those operations occurred during this review.

### 3. Seven production seams and R5 compatibility

Checkpoint production ownership and count are fixed: two exclusive calls in checkpointLocked, two shared calls in checkpointLocked, three encrypted calls in publishRoot. Direct expression-statement calls and actual write/unlink/SetBackfill/rename/Sync neighbors are checked separately from byte and whole-declaration identity.

The three encrypted neighbors retain all three original rekey observers in order. The before-rename pair precedes attempted=true and actual rename; the after-rename pair precedes final namespace Sync; the after-dirsync pair precedes checked root reopen/adoption. The R5 adjacency policy now names exactly those three checkpoint neighbors, without a generic observer allowance, and keeps the old rekey reference count and owner checks.

The two manual context purposes remain independent. A checkpoint context does not arm the R5 observer, and an R5 context returns from the checkpoint adapter before header handling. R5-only/default behavior adds only the independently empty checkpoint stub calls. This is source-semantic isolation, not a machine-code identity assertion.

The R5 publish/main/oracle pins remain HOLD for their independently reviewed changed bytes. All nine R5 declaration snapshots match the composed source exactly. All other current R5 pins and policy bodies are retained, including the nearest-function main collector fix. Its state checks include the main WalkDir closure but do not misclassify the distinct inspectSource-local findings appends.

Two concrete stale-baseline issues were found in the initial guard overlay and corrected before this freeze: an inventory spelling had reverted RekeyQueryText to SQLQueryText; a test had dropped the rollback-model boundary regression loop. The final overlay is rebased from current W, preserves RekeyQueryPlan/RekeyQueryText with no SQLQueryPlan/SQLQueryText occurrences, and retains that regression loop. The final R5 test file differs from current W only by the two new checkpoint-neighbor mutants.

### 4. Readonly closed register and inert counterexamples

The register has exactly 16 rows: the old 14 plus checkpoint per-source and whole-root rows. The old 14 rows are byte-identical, and the old mutation-operation loop is byte-identical. Each row has the same 13 local mutations plus one relocation mutation, giving 16 × 14 = 224. Existing roots and child naming are preserved.

The readonly policy separately requires direct checked placement, source-hook order, complete root-contract order, exact append inventory, error/result propagation, one initial failure state, no reset/address escape and final output-to-failure propagation. It adds only the checkpoint names to those closed inventories.

The eight new guard tests parse inert strings when later authorized. They do not import the manual bridge or invoke native test roots. Mutation rejection temporarily refreshes the byte pin, while the frozen declarations and explicit structural obligations remain independent. Seventy backtick mutation anchors were checked against their final source strings by literal comparison only; no mutant was parsed/executed here. This is a finite source contract, not a claim that arbitrary Go behavior is proven safe. The new header-mirror, exact extent, Epoch, retention-bound and provider-readiness mutations remain within the same eight roots.

Static lexical inspection of the composed current source found no outside protected-name collisions, and no forbidden selector in the owned non-production source. This prevents the definite current-source collisions checked here; it does not substitute for later compiler/AST validation.

## Default and compile-only closure

Source selection follows directly from the exact complementary tags and filenames:

| Selection | Checkpoint source closure |
|---|---|
| Linux, no manual tag | Private vocabulary and the two exact empty void defaults; no bridge, adapter or native caller |
| Non-Linux, no tag or checkpoint tag | Same defaults; Linux-suffixed bridge/adapters/caller remain excluded |
| Linux, checkpoint tag | Two adapters, common manual bridge and exact three native roots; defaults excluded |
| Linux, R5-only tag | Checkpoint defaults remain inert; checkpoint manual closure excluded |
| Linux, both tags | Both independently scoped manual closures; neither context arms the other observer |

The untagged checkpointrestart package contains only contract.go and extension_ledger.go plus their pure tests when selected; no default os/exec/database/native import is introduced by that package. The manual common bridge imports encryption, while the pager/encstore adapters import checkpointrestart; that selected direction introduces no source-level pager/encstore/sqlite back-edge from the bridge. The testscope package contains the source templates as strings and uses ordinary source-inspection dependencies; it does not import the checkpoint native facility.

These are source-level closure conclusions. Default/non-Linux/manual/combined selection commands, compilation, linked symbol counts and test execution all remain NOT_RUN. Later Linux manual qualification must be compile-only for sqlite, preserving the exact three Controller/Writer/Probe roots; a `-run=^$` native test invocation is not a compile-only substitute.

## Finite later validation inventory

The exact inventory is also in AFFECTED-ROOTS.json at the hash above. It contains 8 new guard roots, 30 directly affected existing guard roots, 12 pure roots, and 3 native roots that remain excluded from execution. This is a bounded later-validation inventory, not authorization, a campaign or a new acceptance requirement.

### Eight new guard roots

- `TestCheckpointRestartGuardInventoryAndGate`
- `TestCheckpointRestartGuardProductionOrder`
- `TestCheckpointRestartGuardObserverPurpose`
- `TestCheckpointRestartGuardPublicEntryAndCommitReceipt`
- `TestCheckpointRestartGuardProcessOwnership`
- `TestCheckpointRestartGuardFullExtensionLedger`
- `TestCheckpointRestartGuardPrivateCopiesAndProbe`
- `TestCheckpointRestartGuardDefaultAndIntegration`

### Thirty directly affected existing guard roots

- `TestRekeyGuardInventoryGate`
- `TestRekeyGuardPublicationOrder`
- `TestRekeyGuardObserverIsolation`
- `TestRekeyGuardPublicReturnACK`
- `TestRekeyGuardProcessOwnership`
- `TestRekeyGuardProbeLedger`
- `TestRekeyGuardIntegrationDefault`
- `TestReadOnlyWALManualSourceContract`
- `TestReadOnlyWALManualDefaultExclusion`
- `TestReadOnlyWALManualWholeRootInventory`
- `TestReadOnlyWALManualGateAndProfileMutations`
- `TestReadOnlyWALManualEffectBoundaryMutations`
- `TestReadOnlyWALManualOwnershipMutations`
- `TestReadOnlyWALManualPureClosureMutations`
- `TestOracleSourceConstructorGuard`
- `TestOracleSourceImportAndMutationGuard`
- `TestOracleSourceFactoryLiteralGuard`
- `TestOracleSourceNativeExceptionGuard`
- `TestOracleSourcePathScope`
- `TestScopeSourceReferenceGuard`
- `TestEncryptedRootFiniteStructure`
- `TestEncryptedRootFalsePassSourcesEvenWhenRepinned`
- `TestEncryptedRootWholeSourceAndCapabilityBoundary`
- `TestEncryptedRootPinsRequireIndependentRelease`
- `TestFormatUpgradeMainKeepsExistingPolicies`
- `TestWALPublicationGuardFixedSourceAndWiring`
- `TestRollbackFaultGuardWiringPreservesPriorPolicies`
- `TestRollbackFaultSQLWiringAndMainReviewHold`
- `TestRollbackRestartWiringPreservesAllPoliciesAndReviewHold`
- `TestRollbackRestartOracleExceptionRequiresCompletePinnedNativeClosure`

### Twelve pure model roots

- `TestCheckpointRestartTenCases`
- `TestCheckpointRestartAuthorityCuts`
- `TestCheckpointRestartCommittedHeadCausality`
- `TestCheckpointRestartTransportBinding`
- `TestCheckpointRestartOwnerWaitAndUnknown`
- `TestCheckpointRestartPrivateFamilyCopies`
- `TestCheckpointRestartEncryptedSameEpoch`
- `TestCheckpointRestartNeverPhysicalPass`
- `TestCheckpointExtensionFixtureRows`
- `TestCheckpointExtensionGramsLedger`
- `TestCheckpointExtensionGraphMembershipLedger`
- `TestCheckpointExtensionQueryPathsAndCheck`

### Three native roots: NOT_RUN

- `TestManualCheckpointRestartController`
- `TestManualCheckpointRestartWriter`
- `TestManualCheckpointRestartProbe`

## Final boundary

The facility remains ten fixed cases, three profiles, the 315-row fixture, twelve pure roots, three native roots and seven inserted no-op-default seams. The manual document includes later user-operated steps under the clarified source-only documentation scope; their presence does not imply that they were run or authorize this reviewer to run them.

No pin was released by this reviewer. The independent native/model review is a separate prerequisite for releasing its source pins; its CLEAR verdict for this exact corrected freeze is recorded above. Any subsequent native or guard source change invalidates the corresponding exact-byte part of this review until the changed closure is examined and its snapshot/hash updated explicitly. No extra pin framework, formatting layer, native run, fixture expansion or general-program proof is requested.


<!-- Original: checkpoint-format-independent-review.md -->
# Narrow final checkpoint formatting/pin review

2026-10-12 UTC. Verdict: **CLEAR** for the exact final overlay and one-literal legacy pin plan below. This review extends the prior source CLEAR only across formatting, pin/snapshot synchronization and format-dependent test anchors. No unrelated behavior, test scope or acceptance requirement was reopened or added.

## Exact identity

- FINAL-MANIFEST.json: `112820790a2727e1b8d30f801ab0f7ae0fcb06e37e5f28a0c73d63ab1e7b2b94`
- FINAL-INTEGRATION.patch: `94464cc578aa2dcd2d93cd657801546a85c9fbe17fdb5094886cbf304cb07414`
- Prior facility manifest: `dd9e49ac002db6d2c57e0cadcc1e1fbdf94d8131ee4fc86c136db448d7523441`
- Prior guard manifest: `edaa9bac8613930b4cf0440a0ab9d3d0443864c904c4421739c0ef88d276cf60`
- Prior guard review: `846c309f404b0c7550483f44958e947a3d23cd59b90c8d97962af69f139da663`
- Finite planned test inventory: `9b3b31d013383605d4b0c2f584a3fc87cf3fde306f3b8d5f8571f832bac82921`

Independently verified all 24 final paths, byte lengths and hashes; every preserved before file equals the exact previously reviewed source; every replacement baseline equals current W and each add is absent. All manifest-listed evidence hashes/lengths match. Reconstructing the patch from the current baseline and final bytes, using /dev/null for new files, reproduces FINAL-INTEGRATION.patch exactly. There are 15 additions and nine replacements.

## Formatting and synchronization

Read INTEGRATION.md, format-proof.json, numbered diffs, final-sync-proof.json, both release stages, both anchor proofs, the final manifest and finite-test inventory. Independently repeated the byte/literal comparisons rather than executing the supplied scripts.

- Exactly 23 Go sources were formatted; 14 differ from their reviewed input. Noncomment/nonseparator token spelling and every comment's bytes are preserved. The only import-order difference is the unchanged import multiset in contract.go, with encoding/hex before encoding/json. This is text-level comparison, not Go parser or compiler validation.
- All 18 checkpoint pins equal the final target bytes. All nine R5 pins equal their current composed targets; the release diff changes only the three reviewed shared publish/main/oracle HOLD values. The six other R5 pins remain unchanged.
- All 17 checkpoint templates and nine R5 snapshots equal their complete final target sources exactly. Guard implementation bytes outside the two explicitly allowed pin/snapshot maps in each guard are byte-identical to the preserved formatter output. No policy was widened or disabled.
- All 73 checkpoint and 47 direct R5 mutation anchors are present. Exactly 11 checkpoint source-anchor adaptations preserve their original token spelling. Reverting only those source literals reconstructs the exact post-gofmt test file; all mutant replacement text, call order, roots and other test bytes remain untouched.
- Every other final Go file is byte-identical to its recorded formatter output. The manual document is byte-identical to the reviewed input.

The formatter's recorded single official-gofmt invocation exited 0. This reviewer did not run gofmt or any Go executable and does not infer compiler/test success from that receipt.

## Main, prior policies, ownership and isolation

Main and oracle bytes are unchanged from the already independently reviewed overlay:

- Whole main: `9d29dda532e851dd0fa2211473a5abb24309f798a849483c7efca09a7a3a058f`
- Oracle: `f7d7f6f02e886dfe057a4494564093f78a7644dffe2fa4c41fafa2929227edda`
- Exact 4,519-byte inspectSource function: `98341345e43f39b39dbc78eaf49a916a0701f3d7f2f383d4949b9a3ed439d199`

The earlier whole-main propagation review, exact process admission, lexical collector correction, current RekeyQueryPlan/RekeyQueryText names, rollback-boundary regression, 16-row × 14-operation readonly register, seven production seams and R5 observer adjacency therefore remain in force. Formatting/template synchronization does not add a new native capability or broaden a prior source exemption.

Exact tag comments and source tokens are retained. Ordinary and non-Linux checkpoint builds still select empty default observers and exclude the Linux manual bridge, two adapters and three native role roots. R5-only and combined-tag contexts retain the previously reviewed independent purpose boundaries. These are source-level conclusions; source-selection commands, compiler/linker checks and native entry execution were not run here.

## Finite validation and current-W integration

The final inventory preserves exactly the prior eight new guard roots, 30 affected old guard roots and 12 pure model roots: **50 pure/AST roots**, with exact anchored selectors. The three manual native roots remain separately listed as NOT_RUN.

The additional planned healthy-default roots are exactly:

1. ./internal/pager: TestSharedWALSnapshotsPromotionAndCheckpoint
2. ./sqlite: TestNativeWALConnectionLifecycle
3. ./sqlite: TestReviewEncryptionSharedLatestCheckpointMetadata

Each exists in its recorded current-W source, whose exact SHA-256 was verified. Their default-tag selection remains separate from the manual roles. No new test or matrix was created by this review.

The six legacy pin owners are absent from the 24-file overlay. Each legacy-main-pin-plan.json current-W hash is correct and each file contains exactly one specified old-main literal. The plan substitutes only that literal with the reviewed final main hash; it preserves every other byte, including newer R2 changes. Do not copy older whole-file snapshots for these updates. The six updates remain the lead's later integration step; this reviewer did not apply them or write W.

## Execution boundary

This review used source reads, byte/hash/literal/JSON bookkeeping and this report write only. No Go/gofmt/parser/compiler/test/source-scanner/database/checkpoint/native child/process exit/reopen or campaign was executed by this reviewer. The proposed finite tests and compile-only selections remain NOT_RUN in this review. No additional review or pin layer is requested.


<!-- Original: checkpoint-unused-import-fix-independent-review.md -->
# Narrow checkpoint unused-import correction review

2026-10-12 UTC. Verdict: **CLEAR** for the exact two-file correction. No unrelated hypothesis, test or acceptance scope was added.

- Source tree recorded in SOURCE-IDENTITY.json: `ffc73262e0314f46827b7f4cee1cdac766dbd889`
- SOURCE-IDENTITY.json SHA-256: `5f6e0631b750fc9fba1e9e1e1a8242d590f8bada839aa60b4dbc52eb549d628a`
- change.patch SHA-256: `9756f673f0fedfdca2976daad62ea88b34a6d060d9327a3017ca0b2ff6fdc2a3`
- Final contract.go SHA-256: `42281d899772be55222137744b0638b4c073e7d96772367604a9553eaf1fff8a`
- Final checkpoint_restart.go SHA-256: `f9d6f0843e50a745922b223b6f11e1dd557e0acd17c94cfb53ea2811951811c9`

The reported build failure was the unused bytes import in the first 12-root pure package command. No pure test result is inferred from a build failure. The lead reports the preceding format and three source-selection steps passed; this review did not rerun them.

I independently reconstructed current contract.go by deleting exactly its single bytes import line from the preserved before file. There is no bytes-qualified identifier use in that file. Every other byte is unchanged.

I independently reconstructed the current guard from its preserved before file using exactly three matching changes: model SHA-256 pin, the bytes token removed from that model's exact import inventory, and the same removed import line inside its complete source snapshot. Every other guard byte is unchanged. The patch has exactly these two file targets, and their hashes agree with SOURCE-IDENTITY.json.

All 17 checkpoint declaration snapshots match current W. All 73 checkpoint mutation source anchors are still present, including the model import-expansion anchor. The test file is unchanged from the independently reviewed formatted overlay. No native admission, process role, observer seam, tag, filename, root or caller body changes.

The prior three source-selection PASS results can be retained with an explicit dependency note: this correction changes neither file/tag selection nor package-level dependency selection (bytes remains used/imported by the package's ledger/tests). Those are earlier inventory-specific observations carried across an import-only correction, not a new-tree compiler/test PASS. The failed pure-package command and remaining authorized checks still require their actual results on the corrected source.

This reviewer performed only source/JSON/literal/hash comparisons and wrote this report. No W edits, Go, parser, compiler, source scanner, tests, native child, checkpoint, process exit, reopen or campaign execution occurred.
