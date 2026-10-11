# Integrated-tree source and selected-helper audit

Date: 2026-10-11 UTC. Base commit 63c2ebb10e3a2c4ff2378ee9f33c46133134c70f,
tree d735409ebd1c43d90ee1650c8a7c2a084e270674. Donor V2 tree
5f2766d4b058b7d0b284dd1f34d35edf90c83db4. This report records a fresh audit,
not a new physical recovery result and not reuse of historical V1/V2 PASS.

## Guard integrated first

Before any project code executed, inspected the exact baseline and donor guard,
all three selected guard tests and their inert source inputs. Merged only the
reviewed recovery hunks. main() checks GOFLAGS, walks test source files, reads
source and calls inspectSource; it does not import pager/sqlite or call database
code. inspectSource only parses AST and collects strings. The tests call this
parser on string values; those strings are never compiled or executed.

The current base guard is byte-identical to the donor V1 baseline. The distinct
Rename/rename selector expansion and Windows/Darwin raw namespace gates are not
in this base and are not added in this slice. Existing os.Rename/reference guards
remain intact. Same-file alias/dot-import prevention is conservative, scope-
insensitive and incomplete: inferred literal element types and cross-file aliases
may be missed, shadowed names may false-positive. No selected test uses these
omissions, reflection, hidden function values or hooks to enter physical recovery.

## Exact allowed selectors and full reached helper chains

- internal/testscope: TestScopeSourceReferenceGuard,
  TestScopeRecoveryOptionTypeChains, TestIndependentReviewGuardDirectAndAliasLimit.
  Helpers: inspectSource, stdlib parser/AST/strings/formatting only.
- internal/pager: TestReaderRecoveryMemoryRetiredPathProof,
  TestReaderRecoveryMemoryDiagnosticAndOwnershipRefusal,
  TestReaderRecoverySourceProofBoundary,
  TestReaderAcquisitionSourceSnapshotAfterShared.
  Reached model helpers: plainRetirementRegistry.reserve/snapshot,
  plainMemoryOwner.RetirementComplete/Close, finishReaderProbe,
  plainRetirement.begin/beginClass/adoptLocked/retireLocked/releaseLocked/retry,
  RetryPlaintextCleanup/walkPlainErrors, plainDiagnostic, plainSingleCause and
  vfs.CleanupOwnersBounded. The local registries contain only plainMemoryOwner;
  no vfs.File/plainFile/native owner is installed. The pending-case retry handles
  that exact local memory owner, never the process-wide registry. Descriptor-
  free diagnostics are ordinary errors or memory model wrappers. AST tests read
  pager.go/reader_recovery.go without running the functions they inspect. The
  added typed-nil and panic-on-Is/As/Unwrap witness reaches only the exact private
  type check, proving arbitrary traversal is not used. Paths are equality labels.
- sqlite: TestRollbackRecoveryMemoryPolicy,
  TestRollbackRecoveryMemorySingleAttemptSequence,
  TestRollbackRecoverySourcePublicAndDriverBoundaries,
  TestRollbackRecoveryMemoryFailureClassification.
  Reached helpers: validateRollbackRecovery, RollbackRecoveryOptions.pagerOptions,
  journal.ReplayLimits.WithDefaults, runRollbackRecovery with only local memory
  callbacks, checkContext, wrap/classifyEncryptedCleanup, source parser/printer
  helper recoverySourceFunctions and source-only assertRecoveryOptionCopyOrder.
  os.ErrNotExist is a synthetic error sentinel only. os.ReadDir and ParseFile
  inspect source; no filesystem DB fixture is created. No enabled Options or
  Connector is constructed, and snapshotRollbackRecovery is only inspected.
  No call reaches recoverRollbackOpen, RecoverRollback, pager.Open or public Open.

Repository scan found no TestMain. The only production init is driver registration
(sql.Register), which installs a driver without opening it. Selected-package
variable initializers are sentinels, bounded empty registries and literal API
compatibility values; no database operation. The remaining tests compile but are
not selected. No fuzz mode, manual tag, oracle tag or process helper is used.

## Fresh correctness reassessment

The exact source dependencies used by the bridge, ownership models, parser limits,
path handling and cleanup/error classifier were compared with the V1 source base:
13 inspected dependencies are unchanged (dependency-source-comparison.json).
This consistency is supporting source provenance, not inherited runtime PASS.

1. Policy validation is descriptor-free and precedes native admission and path
   resolution. Nil leaves encrypted/default behavior and budgets unchanged.
   Readonly/WAL/encrypted/non-Linux combinations refuse; URI flags never upgrade.
2. Both Open and Connector dereference the optional policy into a local copy,
   assign its address before storing Options, and retain all three scalar bounds.
   Connector modes apply first. The public Open snapshot precedes normalization
   and DB storage; no concurrent caller mutation is supported. Added source
   assertions retain existing assertions and check single copy/order/call counts.
3. The reader defer retains its one cleanup call, now via finishReaderProbe.
   Recovery-ready proof requires the exact private journal-presence pointer for
   the resolved path, complete retirement and the sole retained diagnostic.
   Wrappers/joins/generic sentinels/extra errors/pending tokens cannot admit.
   Prior errors remain refusal even when cleanup was consumed; later explicit
   memory cleanup retry cannot retroactively change an old error into proof.
4. Public pre-canonical admission transfers exactly once. A failed constructor
   retires it; public deferred Finish sees an already-transferred root. Physical
   recovery would reserve its own root and independently validate locks/path/
   full journal. A successful recovery is followed by one fresh pager.Open,
   which reserves a new root and never creates. Reprobe failure cannot fall
   through to public creation. Recovery has exactly one production call site.
5. Cancellation checked before recovery and reprobe; after recovery starts
   physical writes the existing implementation completes or reports uncertainty.
   No late context check discards success. Failed callbacks retain initial and
   subsequent errors, return zero metadata, and preserve owner identity. Historical
   ErrRecoveryRequired keeps CodeIOErr precedence over BUSY/cancel/limit; it does
   not assert the current disk still needs recovery. Zero-extent recovery may
   succeed but then fail the ordinary fresh probe, without creation.
6. checkSidecars still fails closed for every journal and its typed diagnostic
   still unwraps ErrRecoveryRequired. No unrelated test checks the old error
   wording. The bridge cannot be reached through later SQL/Reader/Tx operations.
   Native recovery implementation is unchanged except its reachability comment.
   Path identity is cooperative, not an adversarial namespace guarantee.

## Existing-test preservation

All existing tests outside two reviewed files remain byte-identical to 63c2.
main_test.go adds six inert restricted-source assertions without removing or
relaxing controls. reader_acquisition_test.go retains every original ordering and
ownership assertion; its single exact defer string is updated to the new named
retirement bridge. The new reader source test additionally requires that bridge's
first statement to invoke begin, and a call-free proof checker. This one expected
source spelling change is necessary compatibility, not a weakened assertion.
New source/model tests include additive typed-nil/no-traversal and option-copy
ordering assertions beyond the accepted donor. Existing behavior tests are not
rewritten or skipped. goal.md and all unrelated sources are unchanged.

## Boundaries

No actual RecoverRollback, opted real Open, production bridge, manual_recovery,
manual_namespace, physicalfault/crash/kill/forced exit, namespace replacement,
missing selected file, wrong-key/tamper, interrupted publication/rekey, native OS
or CI runner, ANN V4, 41-target fuzz or unknown blocked operation is executed.
No main/Git metadata/remote/CI/Library/donor modification. Compiler execution
waits for lead capacity release and uses original resource thresholds. No cleanup,
relocation, alternative route or hidden bypass is performed by this worker.

Physical recovery acceptance remains BLOCKED / NOT_RUN. The source/model check
log will establish only the exact selected checks for the integrated candidate.
