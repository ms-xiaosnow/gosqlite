# R5 interrupted rekey: independent guard and integration source review

## Verdict and boundary

**SOURCE_REVIEW_CLEAR_WITH_LIMITS for the six-file guard freeze identified below.** One definite guard-positive-control defect was reported, corrected by the implementation worker, and independently reread. No remaining definite blocker was identified in this finite source/integration review. All nine new byte pins remain explicit HOLD values; this report does not claim that the held scanner currently passes.

This is source review only. No Go, gofmt, Go parser/scanner executable, source-selection command, compiler, test, native process, fixture, wrong-key Open, interruption, archive, recovery, or physical validation was run. The reviewer used source reads, text comparisons, literal decoding, lexical inventories, hashes and Git identity/status reads, coordinated a source correction, and wrote this report. No source code or pin was changed by the reviewer. Source-review clearance is not execution authorization, a type-check result, or physical/security acceptance.

Reviewed base HEAD: `01c63e6f8fb3d9e196afe2f42a163eb3eac4ad5a`. Other authorized workers may be preparing integration concurrently; this report identifies overlay bytes rather than claiming an unchanged worktree or a committed result.

The facility input is the eight-file `rekey-interruption-source-evidence/manifest.json`, SHA-256 `e78dcdf71c2a6cff108436f16c120982457ad810aa2d39bba487afbfb99f9b99`. Its separate native/model/observer review is `rekey-interruption-independent-review.md`, SHA-256 `1523e55299bbf9c609791e76aa63b43e1844e3166f4d9f7db37852c423fc8ada`. That report's limits continue to apply, especially that native plaintext-page hashes and page-one counter equality are not observed. This guard review does not erase those limits.

## Definite finding and correction

### F1: the approved Wait goroutine was rejected by the generic enclosing-function helper — corrected

The original `rekeyGuardCallContext` used `oracleEnclosingFunction`, which deliberately returns nil upon crossing an `ast.FuncLit`. The frozen native `command.Wait()` is inside the required `go func()` in `start`. Therefore the original `start|command.Wait()` allowance could never match that real call. This was a definite false rejection of the native positive fixture and its source-pinned startup admission, not a hypothetical arbitrary-program bypass.

Final `rekeyGuardWaitCall` narrowly admits only the exact native path and `command.Wait()` expression, directly in `err := command.Wait()` as the first of exactly two closure statements. The closure must have the exact complete goroutine statement and concrete result-channel send, be directly invoked without arguments, and be a direct statement of `start` on receiver `r *manualRekeyRun`. The general `start|command.Wait()` allowance was removed. `oracleEnclosingFunction` itself remains byte-for-byte unchanged. Moving the goroutine into an extra `if` is now a double-repinned mutant within the existing process-ownership test root. The positive fixture and this correction were traced statically; neither was executed.

## Full scanner integration

The complete candidate `main.go` was read. Its diff against the base consists only of:

1. One direct all-Go-source `inspectRekeyInterruptionSource(path, raw)` triplet before the earlier root-negative source triplet, preserving returned errors and appended findings.
2. One direct `inspectRekeyInterruptionContracts(".")` triplet after the earlier root-negative whole contract, preserving error output, finding output and the shared failure bit.

The initial manual-GOFLAGS refusal, complete WalkDir traversal, prior source-hook ordering, ordinary `_test.go` filter, `oracleCurrentGoPath` filter, all prior end-of-walk contracts, traversal-error propagation and final failure exit remain present and ordered. No build-tag shortcut or blanket manual exemption was added.

The complete `inspectSource` body is identical to base, SHA-256 `98341345e43f39b39dbc78eaf49a916a0701f3d7f2f383d4949b9a3ed439d199`. Existing recovery exceptions and the exact A5 scalar Signal exception are unchanged. The new model's `SignalNumber` spelling needs no expansion of those exceptions.

The only `oracle_startup.go` change is one OR predicate at the existing process-reference branch. It requires the exact R5 native pathname, exact Linux AND manual tag, direct unaliased `exec.Command`, original exec import binding, exactly one command constructor, exact self-image/role-flags call context, and a clean full native-file R5 guard result including its byte pin and declaration snapshot. That admission alone is not a companion-file proof: the integrated all-source and end-of-walk planes separately check required companion presence and source contracts. The older native CLI and A5 admissions are unchanged.

The read-only-WAL/A8 wiring helper has a deliberately closed findings-append inventory and is also called by A5's wiring checker. Adding a new main append without registering it would cause an old-policy false rejection. The two additional files correct exactly that integration:

- `readonly_wal_manual.go` adds the exact R5 source triplet and source ordering, exact whole-contract triplet and ordering, one exact findings append, and once-call inventory entries for both R5 hooks.
- `readonly_wal_manual_test.go` retains the existing 12 matrix rows and all 14 mutation operations; it adds one source row and one whole-contract row. The finite count is now 14 × 14 = 196, with no new Test root or named child.

The old absence/renaming/result substitution/error substitution/dropped error/dead code/duplicate/reset/address-escape/order/moved-triplet mutations remain. No A5 implementation, capability helper, ordinary restricted-reference body, or existing source pin was changed in the frozen six-file overlay.

## R5 source rules and finite mutant review

The new guard uses independent source structure/statement/body contracts plus supplemental per-file byte pins and complete declaration snapshots. Nine snapshots were independently decoded as string literals and compared byte-for-byte with the seven frozen Go facility files and the reviewed main/oracle files; all matched. All 24 native critical-body literals and all 12 bridge critical-body literals were independently compared with the corresponding frozen facility text; all matched. The six finite function inventories agree with the declarations in their frozen source files.

The two scanning planes retain findings and errors and check missing required files. They include exact source paths and build constraints, copied protected capability names, model-consumer placement, allowed import paths, finite native selectors and call counts, function inventories, transport field order/types, and public encryption Header/Epoch/KDF/FileDescriptor shape dependencies. Publication source remains subject to its whole-source pin and to the exact three adjacent void observer calls around unchanged real rename/Sync/readback statements.

The default observer has the exact complementary `!linux || !manual_rekey_interruption` build constraint and exact empty void function body. The vocabulary source has no imports or functions. The manual observer requires a concrete context token, bounded public/ciphertext observation, unlock before transport, exact granted exit, no result substitution, and no key/native reentry, callback, deferred work, or caller-buffer mutation. The reviewed bridge body contracts retain actual protocol, owner snapshot and nil-return behavior.

The native contracts retain first-action role admission, unchanged adjacent capture of the real public Rekey error, nil-only return barrier, parent ACK bound to the fifth actual received writer frame, exclusive complete write and file/directory retirement chain, one fixed self-image Start and the exact Wait goroutine, unresolved-child retention, terminal EOF, one ReadOnly/NoCreate probe Open, the original authentication error taxonomy, literal seven-query ledger, bounded Check, one-time resource close, complete independent source-preserving copies, provider purposes and control-only archive behavior. These source obligations cannot substitute for observations from a later real experiment.

Seven new guard roots parse inert source strings. The rejection helper temporarily replaces BOTH the byte digest and declaration snapshot, restores both with defer, and refuses to count HOLD/declaration-snapshot diagnostics as semantic rejection. The source contains 43 explicit mutation anchors; each anchor is present in its intended frozen fixture. Some anchors intentionally use the first of multiple occurrences, as the helper specifies. The separate copied-path/inventory cases and ordinary/A5 protection probes remain present. Positive controls cover all owned frozen source categories and main, and the startup helper has a positive exact-path/native fixture alongside negative copied-path and shell-target fixtures. The Wait defect above shows why positive controls matter.

This is a finite prevention aid. It does not establish arbitrary Go-program equivalence, type identity, whole-program dataflow, absence of all possible hostile rewrites after a reviewer repins arbitrary bytes, source-to-compiler correctness, or a general native-effect sandbox. Its independent semantic bodies and exact snapshots are reviewed obligations, not a license to refresh pins until a scan becomes green.

## Source-reader and model package closure

The new guard/test files import only standard-library source-reading/AST/hash/string/testing packages; they do not import the native facility, sqlite, encstore, os/exec, syscall, time, or a third-party execution package. Native examples occur as inert source-string data. New package variables are literal maps of pins, source strings, body strings, function names and transport declarations. There is no new init or TestMain. The complete local `internal/testscope` package's actual import blocks were inspected: they are standard-library source-reader/testing dependencies. Existing small map-building initializers remain source-data construction. This is a source-level direct-package assessment, not a newly executed Go dependency graph or a fresh audit of the entire standard library.

The pure model package imports bytes, crypto/sha256, encoding/binary and encoding/json; its test file imports encoding/binary and testing. It has no native-facility import, timer, process/descriptor capability, callback field, channel/goroutine effect, or package-state initializer. `Result.Pass()` and `Result.PhysicalValidated()` remain unconditional false. The eight model roots are synthetic value-model checks, not native database/process tests.

## Exact pin-release boundary

All nine R5 byte pins remain `HOLD_PENDING_INDEPENDENT_REKEY_FULL_SOURCE_REVIEW`. Every older pin remains unchanged. Consequently ordinary full-contract execution and old whole-main pin tests are expected to remain held until a separate, explicitly reviewed integration release occurs. This state must not be described as a passing guard run.

For the exact reviewed unformatted candidate, the new main SHA-256 is `1d9f41ed857bf64a3d2913f96fee3409a173f036df753efd81a7a9190ee2536a`; the older main digest `6dbab48ff374858bfafe578f0fca99f5176ced7b6bdc008bec04ce6a6d1d0eb2` occurs in six independently owned pin/literal locations:

- `internal/testscope/wal_tail_publication.go`
- `internal/testscope/encryption_upgrade_manual.go`
- `internal/testscope/encrypted_root_negative.go`
- `internal/testscope/rollback_fault_test.go`
- `internal/testscope/rollback_fault_sql_test.go`
- `internal/testscope/rollback_restart_test.go`

Only an integration release of the exact reviewed main bytes may replace those six old-main values. Preserve all non-main pins, independent literal checks, and the unchanged ordinary-body digest. The nine new R5 release targets are their exact independently reviewed source hashes, never a blanket computation from whatever happens to be on disk. A later formatting pass or substantive change creates new byte identities: inspect the resulting diff, keep the declaration snapshots/mutant anchors coherent, independently hash the actual final sources, and record a new release receipt. This report does not pre-approve unseen post-format bytes or edit any pins.

## Finite proposed validation roots — NOT_RUN, not authorization

The primary new scope is eight model roots and seven guard roots. Because the integration changes shared old-policy wiring and whole-main identities, a finite affected regression proposal also includes the 54 existing source-only roots listed below. This makes 61 selected testscope roots and 8 model roots, with no native manual role selected. These names are a source-read inventory, not output from a test binary or Go selector. Any execution still requires the parent's separately scoped approval and released final source identities. No wildcard package campaign or native process case is implied.

### New value-model roots (8)

- `TestRekeyInterruptionSixCases`
- `TestRekeyInterruptionCutOrder`
- `TestRekeyInterruptionCredentialPurpose`
- `TestRekeyInterruptionSelectedGeneration`
- `TestRekeyInterruptionSQLIndexLedger`
- `TestRekeyInterruptionAckCausality`
- `TestRekeyInterruptionOwnersAndArtifacts`
- `TestRekeyInterruptionNeverPhysicalPass`

### New guard roots (7)

- `TestRekeyGuardInventoryGate`
- `TestRekeyGuardPublicationOrder`
- `TestRekeyGuardObserverIsolation`
- `TestRekeyGuardPublicReturnACK`
- `TestRekeyGuardProcessOwnership`
- `TestRekeyGuardProbeLedger`
- `TestRekeyGuardIntegrationDefault`

### Existing readonly_wal_manual_test.go (7)

- `TestReadOnlyWALManualSourceContract`
- `TestReadOnlyWALManualDefaultExclusion`
- `TestReadOnlyWALManualWholeRootInventory`
- `TestReadOnlyWALManualGateAndProfileMutations`
- `TestReadOnlyWALManualEffectBoundaryMutations`
- `TestReadOnlyWALManualOwnershipMutations`
- `TestReadOnlyWALManualPureClosureMutations`

### Existing rollback_restart_test.go (10)

- `TestRollbackRestartOwnedSourceContracts`
- `TestRollbackRestartDoubleRepinnedSelectionAndStubCounterexamples`
- `TestRollbackRestartOutsideConsumersAndAliasesRejected`
- `TestRollbackRestartOldCoreAndMutexWrapperStayExact`
- `TestRollbackRestartWiringPreservesAllPoliciesAndReviewHold`
- `TestRollbackRestartDoubleRepinnedNativeProtocolCounterexamples`
- `TestRollbackRestartDoubleRepinnedModelCausalCounterexamples`
- `TestRollbackRestartOracleExceptionRequiresCompletePinnedNativeClosure`
- `TestRollbackRestartDoubleRepinnedCommandCapabilityClosure`
- `TestRollbackRestartDoubleRepinnedLaunchInputClosure`

### Existing main_test.go (1)

- `TestScopeSourceReferenceGuard`

### Existing oracle_startup_test.go (5)

- `TestOracleSourceConstructorGuard`
- `TestOracleSourceImportAndMutationGuard`
- `TestOracleSourceFactoryLiteralGuard`
- `TestOracleSourceNativeExceptionGuard`
- `TestOracleSourcePathScope`

### Existing wal_tail_publication_test.go (11)

- `TestWALPublicationGuardRejectsActivationReferences`
- `TestWALPublicationGuardSafeControls`
- `TestWALPublicationGuardPureFilesRejectNativeAliasesAndVariables`
- `TestWALPublicationGuardExactCallBoundaries`
- `TestWALPublicationGuardFixedSourceAndWiring`
- `TestWALPublicationGuardFakeFactoryIsExactSource`
- `TestWALPublicationGuardProductionBoundariesAreRequired`
- `TestWALPublicationGuardRejectsControlFlowAndFactoryDrift`
- `TestWALPublicationGuardManualSourceIsExactAndDoubleGated`
- `TestWALPublicationGuardElapsedSourceIsInertAndPinned`
- `TestWALPublicationGuardElapsedRejectsTimingAndQualificationDrift`

### Existing encryption_upgrade_manual_test.go (3)

- `TestFormatUpgradeManualStructuralCounterexamples`
- `TestFormatUpgradePureClosureCounterexamples`
- `TestFormatUpgradeMainKeepsExistingPolicies`

### Existing encrypted_root_negative_test.go (4)

- `TestEncryptedRootFiniteStructure`
- `TestEncryptedRootFalsePassSourcesEvenWhenRepinned`
- `TestEncryptedRootWholeSourceAndCapabilityBoundary`
- `TestEncryptedRootPinsRequireIndependentRelease`

### Existing rollback_fault_test.go (4)

- `TestRollbackFaultSourceContracts`
- `TestRollbackFaultRepinnedSemanticCounterexamples`
- `TestRollbackFaultOutsideConsumersRejected`
- `TestRollbackFaultGuardWiringPreservesPriorPolicies`

### Existing rollback_fault_sql_test.go (9)

- `TestRollbackFaultSQLSourceContracts`
- `TestRollbackFaultSQLRepinnedSemanticCounterexamples`
- `TestRollbackFaultSQLOutsideConsumersAndOldEntriesRejected`
- `TestRollbackFaultSQLOrdinaryRecoveryExceptionIsOneReference`
- `TestRollbackFaultSQLWiringAndMainReviewHold`
- `TestRollbackFaultSQLOrdinaryExampleFields`
- `TestRollbackFaultSQLOrdinaryExamplesKeepModelBoundary`
- `TestRollbackFaultSQLOrdinaryFieldBindingsReject`
- `TestRollbackFaultSQLOrdinaryCompanionFailures`

## Reviewed identities

Guard manifest SHA-256: `4ad67d8e663c869fce5622f2c710db2cb48e1b38cdde05e28be9f1e0fc04f42a`.

Guard patch SHA-256: `8586750456c9579495369c2d462207400e3d23a80c14e5284d1462c05c2276db`.

All six manifest identities were independently rehashed after the worker announced the final freeze:

| Overlay path | Bytes | SHA-256 |
|---|---:|---|
| internal/testscope/rekey_interruption.go | 276145 | `8311b7e601d005c89da71b63323efaba668a7d697a61bafdfd315f5390f6bd84` |
| internal/testscope/rekey_interruption_test.go | 10841 | `82aed5860c429d40dadb6ba0b10335a759d12431c93ead88822c717ab59eb0c7` |
| internal/testscope/main.go | 11580 | `1d9f41ed857bf64a3d2913f96fee3409a173f036df753efd81a7a9190ee2536a` |
| internal/testscope/oracle_startup.go | 11306 | `0e480e214568b935c16c34ef9226f7ef2c86fce81151ebe09aa91087a62ec6a4` |
| internal/testscope/readonly_wal_manual.go | 84407 | `2c4c667a1a31e0d8efb341f2b94a03c011189fca44fa2349248814af1b19f389` |
| internal/testscope/readonly_wal_manual_test.go | 49916 | `8a7af77ba419ed25908148ec9e076a249c766b6e33a46b0853e69d54d572f35e` |

These hashes identify source artifacts reviewed, not an executed binary, successful build, test result, source-selector result, published commit or physical acceptance.
